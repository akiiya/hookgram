package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"hookgram/internal/auth"
	"hookgram/internal/config"
	"hookgram/internal/model"
	"hookgram/internal/render"
	"hookgram/internal/repository"
	"hookgram/internal/telegram"
)

var (
	ErrInvalidCredentials   = errors.New("用户名或密码错误")
	ErrAlreadySetup         = errors.New("系统已完成初始化")
	ErrInvalidToken         = errors.New("invalid token")
	ErrAliasExists          = errors.New("别名已存在")
	ErrTokenDisabled        = errors.New("Token 已禁用")
	ErrEmptyMessage         = errors.New("消息内容不能为空")
	ErrTelegramUserInactive = errors.New("Telegram 用户不可用")
)

type Container struct {
	Config   *config.Manager
	Repo     *repository.Repository
	Sessions *auth.SessionManager
	Auth     *AuthService
	Tokens   *TokenService
	Webhook  *WebhookService
	Bot      *BotService
	Telegram telegramAPI
}

type telegramAPI interface {
	GetUpdates(ctx context.Context, offset int64, timeout int) ([]telegram.Update, error)
	SendMessage(ctx context.Context, chatID int64, text, parseMode string) (int, error)
}

func NewContainer(cfg *config.Manager, repo *repository.Repository, tg telegramAPI) *Container {
	sessions := auth.NewSessionManager(24*time.Hour, cfg.Current().Security.SessionSecret)
	tokens := &TokenService{cfg: cfg, repo: repo}
	container := &Container{
		Config:   cfg,
		Repo:     repo,
		Sessions: sessions,
		Telegram: tg,
	}
	container.Auth = &AuthService{cfg: cfg, repo: repo, sessions: sessions}
	container.Tokens = tokens
	container.Webhook = &WebhookService{cfg: cfg, repo: repo, tokens: tokens, telegram: tg}
	container.Bot = &BotService{cfg: cfg, repo: repo, tokens: tokens, telegram: tg}
	return container
}

type AuthService struct {
	cfg      *config.Manager
	repo     *repository.Repository
	sessions *auth.SessionManager
}

func (s *AuthService) SetupComplete() (bool, error) {
	count, err := s.repo.CountAdmins()
	return count > 0, err
}

func (s *AuthService) Setup(username, password, botToken, apiProxy, baseURL string) error {
	complete, err := s.SetupComplete()
	if err != nil {
		return err
	}
	if complete {
		return ErrAlreadySetup
	}
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 6 {
		return errors.New("管理员用户名不能为空，密码至少 6 位")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	admin := &model.AdminUser{
		Username:     username,
		PasswordHash: hash,
	}
	if err := s.repo.CreateAdmin(admin); err != nil {
		return err
	}
	return s.cfg.Update(func(cfg *config.Config) {
		cfg.Telegram.BotToken = strings.TrimSpace(botToken)
		cfg.Telegram.APIProxy = strings.TrimRight(strings.TrimSpace(apiProxy), "/")
		cfg.App.BaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	})
}

func (s *AuthService) Login(username, password string) (*model.AdminUser, auth.Session, error) {
	admin, err := s.repo.FindAdminByUsername(strings.TrimSpace(username))
	if err != nil {
		return nil, auth.Session{}, ErrInvalidCredentials
	}
	if !auth.CheckPassword(admin.PasswordHash, password) {
		return nil, auth.Session{}, ErrInvalidCredentials
	}
	now := time.Now()
	if err := s.repo.UpdateAdminLoginAt(admin.ID, now); err != nil {
		return nil, auth.Session{}, err
	}
	admin.LastLoginAt = &now
	session, err := s.sessions.Create(admin.ID)
	if err != nil {
		return nil, auth.Session{}, err
	}
	return admin, session, nil
}

func (s *AuthService) ChangePassword(adminID uint, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("新密码至少 6 位")
	}
	admin, err := s.repo.FindAdminByID(adminID)
	if err != nil {
		return err
	}
	if !auth.CheckPassword(admin.PasswordHash, oldPassword) {
		return errors.New("当前密码错误")
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateAdminPassword(adminID, hash); err != nil {
		return err
	}
	s.sessions.DeleteByAdmin(adminID)
	return nil
}

type TokenResult struct {
	Token      model.WebhookToken `json:"token"`
	PlainText  string             `json:"plain_token"`
	WebhookURL string             `json:"webhook_url"`
}

type TokenService struct {
	cfg  *config.Manager
	repo *repository.Repository
}

func (s *TokenService) Create(userID uint, alias string) (*TokenResult, error) {
	if _, err := s.repo.FindTelegramUserByID(userID); err != nil {
		return nil, err
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		generated, err := s.nextAlias(userID)
		if err != nil {
			return nil, err
		}
		alias = generated
	}
	if _, err := s.repo.FindTokenByAlias(userID, alias); err == nil {
		return nil, ErrAliasExists
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	plain, hash, prefix, err := s.generateUniqueToken()
	if err != nil {
		return nil, err
	}
	token := &model.WebhookToken{
		TelegramUserID: userID,
		Alias:          alias,
		TokenHash:      hash,
		TokenPrefix:    prefix,
	}
	if err := s.repo.CreateToken(token); err != nil {
		return nil, err
	}
	return &TokenResult{
		Token:      *token,
		PlainText:  plain,
		WebhookURL: s.webhookURL(plain),
	}, nil
}

func (s *TokenService) Validate(plain string) (*model.WebhookToken, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil, ErrInvalidToken
	}
	hash := s.Hash(plain)
	token, err := s.repo.FindTokenByHash(hash)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !hmac.Equal([]byte(token.TokenHash), []byte(hash)) {
		return nil, ErrInvalidToken
	}
	if token.DisabledAt != nil {
		return token, ErrTokenDisabled
	}
	return token, nil
}

func (s *TokenService) Rename(userID, tokenID uint, alias string) (*model.WebhookToken, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return nil, errors.New("别名不能为空")
	}
	token, err := s.repo.FindTokenByID(tokenID)
	if err != nil {
		return nil, err
	}
	if token.TelegramUserID != userID {
		return nil, repository.ErrNotFound
	}
	if existing, err := s.repo.FindTokenByAlias(userID, alias); err == nil && existing.ID != token.ID {
		return nil, ErrAliasExists
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	token.Alias = alias
	if err := s.repo.UpdateToken(token); err != nil {
		return nil, err
	}
	return token, nil
}

func (s *TokenService) SetDisabled(userID, tokenID uint, disabled bool) (*model.WebhookToken, error) {
	token, err := s.repo.FindTokenByID(tokenID)
	if err != nil {
		return nil, err
	}
	if token.TelegramUserID != userID {
		return nil, repository.ErrNotFound
	}
	if disabled {
		now := time.Now()
		token.DisabledAt = &now
	} else {
		token.DisabledAt = nil
	}
	if err := s.repo.UpdateToken(token); err != nil {
		return nil, err
	}
	return token, nil
}

func (s *TokenService) Delete(userID, tokenID uint) error {
	token, err := s.repo.FindTokenByID(tokenID)
	if err != nil {
		return err
	}
	if token.TelegramUserID != userID {
		return repository.ErrNotFound
	}
	return s.repo.DeleteToken(tokenID)
}

func (s *TokenService) DeleteByAliasOrPrefix(userID uint, value string) (*model.WebhookToken, error) {
	token, err := s.repo.FindTokenByAliasOrPrefix(userID, strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteToken(token.ID); err != nil {
		return nil, err
	}
	return token, nil
}

func (s *TokenService) Hash(plain string) string {
	secret := []byte(s.cfg.Current().Security.TokenHashSecret)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(plain))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *TokenService) webhookURL(plain string) string {
	return fmt.Sprintf("%s/w/%s", s.cfg.Current().BaseURL(), plain)
}

func (s *TokenService) generateUniqueToken() (plain, hash, prefix string, err error) {
	for i := 0; i < 5; i++ {
		raw := make([]byte, 24)
		if _, err := rand.Read(raw); err != nil {
			return "", "", "", err
		}
		plain = "wh_" + base64.RawURLEncoding.EncodeToString(raw)
		hash = s.Hash(plain)
		if len(plain) > 14 {
			prefix = plain[:14]
		} else {
			prefix = plain
		}
		if _, err := s.repo.FindTokenByHash(hash); errors.Is(err, repository.ErrNotFound) {
			return plain, hash, prefix, nil
		} else if err != nil {
			return "", "", "", err
		}
	}
	return "", "", "", errors.New("生成 Token 失败，请重试")
}

func (s *TokenService) nextAlias(userID uint) (string, error) {
	for i := 1; i < 10000; i++ {
		alias := fmt.Sprintf("webhook-%d", i)
		if _, err := s.repo.FindTokenByAlias(userID, alias); errors.Is(err, repository.ErrNotFound) {
			return alias, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("无法生成可用别名")
}

type WebhookInput struct {
	Token      string
	Title      string
	Content    string
	RawPayload string
	Format     string
	Level      string
	Fields     map[string]string
	Source     string
	SourceIP   string
	UserAgent  string
}

type WebhookService struct {
	cfg      *config.Manager
	repo     *repository.Repository
	tokens   *TokenService
	telegram telegramAPI
}

func (s *WebhookService) Deliver(ctx context.Context, input WebhookInput) (*model.WebhookMessage, error) {
	now := time.Now()
	input.Format = normalizeFormat(input.Format)
	input.Level = normalizeLevel(input.Level)

	token, err := s.tokens.Validate(input.Token)
	if err != nil {
		deliveryError := "invalid token"
		returnErr := ErrInvalidToken
		if errors.Is(err, ErrTokenDisabled) {
			deliveryError = "token disabled"
			returnErr = ErrTokenDisabled
		}
		message := &model.WebhookMessage{
			Format:         input.Format,
			Level:          input.Level,
			Title:          "无效 Token",
			Content:        "收到无效 Webhook Token 调用。",
			RawPayload:     input.RawPayload,
			Source:         input.Source,
			SourceIP:       input.SourceIP,
			UserAgent:      input.UserAgent,
			DeliveryStatus: "failed",
			DeliveryError:  deliveryError,
			CreatedAt:      now,
		}
		if token != nil {
			message.WebhookTokenID = token.ID
			message.TelegramUserID = token.TelegramUserID
			message.Title = input.Title
			message.Content = input.Content
		}
		_ = s.repo.CreateMessage(message)
		return message, returnErr
	}

	if strings.TrimSpace(input.Title) == "" && strings.TrimSpace(input.Content) == "" && len(input.Fields) == 0 {
		message := &model.WebhookMessage{
			WebhookTokenID: token.ID,
			TelegramUserID: token.TelegramUserID,
			Title:          input.Title,
			Content:        input.Content,
			RawPayload:     input.RawPayload,
			Format:         input.Format,
			Level:          input.Level,
			Source:         input.Source,
			SourceIP:       input.SourceIP,
			UserAgent:      input.UserAgent,
			DeliveryStatus: "failed",
			DeliveryError:  ErrEmptyMessage.Error(),
			CreatedAt:      now,
		}
		if err := s.repo.CreateMessage(message); err != nil {
			return nil, err
		}
		return message, ErrEmptyMessage
	}

	if err := s.repo.TouchTokenUsage(token.ID, now); err != nil {
		return nil, err
	}

	if strings.ToLower(strings.TrimSpace(token.TelegramUser.Status)) != "active" {
		message := &model.WebhookMessage{
			WebhookTokenID: token.ID,
			TelegramUserID: token.TelegramUserID,
			Title:          input.Title,
			Content:        input.Content,
			RawPayload:     input.RawPayload,
			Format:         input.Format,
			Level:          input.Level,
			Source:         input.Source,
			SourceIP:       input.SourceIP,
			UserAgent:      input.UserAgent,
			DeliveryStatus: "failed",
			DeliveryError:  "telegram user is not active",
			CreatedAt:      now,
		}
		if err := s.repo.CreateMessage(message); err != nil {
			return nil, err
		}
		return message, ErrTelegramUserInactive
	}

	view := render.WebhookView{
		Title:      input.Title,
		Content:    input.Content,
		Format:     input.Format,
		Level:      input.Level,
		Source:     input.Source,
		TokenAlias: token.Alias,
		Fields:     input.Fields,
		CreatedAt:  now,
	}
	rendered := render.ConstrainTelegramMessage(render.WebhookMessage(view))
	messageID, deliveryNote, deliveryErr := s.sendRendered(ctx, token.TelegramUser, rendered)
	status := "sent"
	deliveryError := deliveryNote
	if deliveryErr != nil {
		status = "failed"
		deliveryError = deliveryErr.Error()
		var tgErr telegram.TelegramError
		if errors.As(deliveryErr, &tgErr) && tgErr.Code == httpStatusForbidden {
			_ = s.repo.MarkTelegramBlocked(token.TelegramUser.ID, now)
		}
	}

	message := &model.WebhookMessage{
		WebhookTokenID:    token.ID,
		TelegramUserID:    token.TelegramUserID,
		Title:             input.Title,
		Content:           input.Content,
		RawPayload:        input.RawPayload,
		Format:            input.Format,
		Level:             input.Level,
		Source:            input.Source,
		SourceIP:          input.SourceIP,
		UserAgent:         input.UserAgent,
		DeliveryStatus:    status,
		DeliveryError:     deliveryError,
		TelegramMessageID: messageID,
		CreatedAt:         now,
	}
	if err := s.repo.CreateMessage(message); err != nil {
		return nil, err
	}
	if deliveryErr != nil {
		return message, deliveryErr
	}
	return message, nil
}

const httpStatusForbidden = 403

func (s *WebhookService) sendRendered(ctx context.Context, user model.TelegramUser, msg render.Message) (int, string, error) {
	if msg.ParseMode == "" {
		messageID, err := s.telegram.SendMessage(ctx, user.ChatID, msg.Text, "")
		return messageID, "", err
	}
	messageID, err := s.telegram.SendMessage(ctx, user.ChatID, msg.Text, msg.ParseMode)
	if err == nil {
		return messageID, "", nil
	}
	fallbackID, fallbackErr := s.telegram.SendMessage(ctx, user.ChatID, msg.Fallback, "")
	if fallbackErr != nil {
		return 0, "", fallbackErr
	}
	return fallbackID, fmt.Sprintf("%s 发送失败，已降级为纯文本: %v", msg.ParseMode, err), nil
}

type BotService struct {
	cfg      *config.Manager
	repo     *repository.Repository
	tokens   *TokenService
	telegram telegramAPI
	mu       sync.Mutex
	parent   context.Context
	cancel   context.CancelFunc
	running  bool
	lastKey  string
}

// Start 启动 Bot 运行时。配置为空时不会启动 polling。
func (s *BotService) Start(ctx context.Context) {
	s.mu.Lock()
	s.parent = ctx
	s.mu.Unlock()
	s.UpdateConfig()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
}

func (s *BotService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *BotService) Restart() {
	s.Stop()
	s.UpdateConfig()
}

func (s *BotService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// UpdateConfig 在配置变更后调用，必要时重启 polling，避免重复 goroutine。
func (s *BotService) UpdateConfig() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.cfg.Current()
	key := strings.Join([]string{
		strings.TrimSpace(cfg.Telegram.BotToken),
		strings.TrimRight(strings.TrimSpace(cfg.Telegram.APIProxy), "/"),
		strings.ToLower(strings.TrimSpace(cfg.Telegram.CommandMode)),
	}, "|")
	shouldRun := strings.TrimSpace(cfg.Telegram.BotToken) != "" && strings.ToLower(strings.TrimSpace(cfg.Telegram.CommandMode)) == "polling"
	if !shouldRun {
		s.stopLocked()
		s.lastKey = key
		return
	}
	if s.running && s.lastKey == key {
		return
	}
	s.stopLocked()
	parent := s.parent
	if parent == nil {
		s.lastKey = key
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.running = true
	s.lastKey = key
	go s.poll(ctx)
}

func (s *BotService) stopLocked() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.running = false
}

func (s *BotService) poll(ctx context.Context) {
	var offset int64
	defer func() {
		s.mu.Lock()
		if s.cancel == nil {
			s.running = false
		}
		s.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := s.telegram.GetUpdates(ctx, offset, 25)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, telegram.ErrNoBotToken) {
				log.Printf("Telegram polling 暂时不可用: %v", err)
			}
			sleepContext(ctx, 5*time.Second)
			continue
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if update.Message != nil && strings.HasPrefix(strings.TrimSpace(update.Message.Text), "/") {
				s.handleCommand(ctx, update.Message)
			}
		}
	}
}

func (s *BotService) handleCommand(ctx context.Context, msg *telegram.Message) {
	user, err := s.upsertUser(msg)
	if err != nil {
		_, _ = s.telegram.SendMessage(ctx, msg.Chat.ID, "记录用户信息失败，请稍后重试。", "")
		return
	}
	parts := strings.Fields(strings.TrimSpace(msg.Text))
	if len(parts) == 0 {
		return
	}
	command := strings.Split(parts[0], "@")[0]
	switch command {
	case "/start":
		s.reply(ctx, msg.Chat.ID, fmt.Sprintf("欢迎使用 Hookgram。\n\n你可以发送 /add 创建 Webhook Token，或发送 /help 查看完整命令。\n\n当前服务地址：%s", s.cfg.Current().BaseURL()))
	case "/help":
		s.reply(ctx, msg.Chat.ID, render.CommandHelp(s.cfg.Current().BaseURL()))
	case "/list":
		s.handleList(ctx, msg.Chat.ID, user.ID)
	case "/add":
		alias := ""
		if len(parts) > 1 {
			alias = parts[1]
		}
		s.handleAdd(ctx, msg.Chat.ID, user.ID, alias)
	case "/del":
		if len(parts) < 2 {
			s.reply(ctx, msg.Chat.ID, "请提供要删除的别名或 Token 前缀。\n示例：/del production")
			return
		}
		s.handleDelete(ctx, msg.Chat.ID, user.ID, parts[1])
	case "/rename":
		if len(parts) < 3 {
			s.reply(ctx, msg.Chat.ID, "请提供旧别名和新别名。\n示例：/rename old new")
			return
		}
		s.handleRename(ctx, msg.Chat.ID, user.ID, parts[1], parts[2])
	case "/url":
		if len(parts) < 2 {
			s.reply(ctx, msg.Chat.ID, "请提供 Token 别名。\n示例：/url production")
			return
		}
		s.handleURL(ctx, msg.Chat.ID, user.ID, parts[1])
	case "/usage":
		if len(parts) < 2 {
			s.reply(ctx, msg.Chat.ID, "请提供 Token 别名。\n示例：/usage production")
			return
		}
		s.handleUsage(ctx, msg.Chat.ID, user.ID, parts[1])
	default:
		s.reply(ctx, msg.Chat.ID, "未知命令。发送 /help 查看可用命令。")
	}
}

func (s *BotService) upsertUser(msg *telegram.Message) (*model.TelegramUser, error) {
	from := msg.From
	if from == nil {
		return nil, errors.New("缺少 Telegram 用户信息")
	}
	display := strings.TrimSpace(strings.Join([]string{from.FirstName, from.LastName}, " "))
	if display == "" && from.Username != "" {
		display = "@" + from.Username
	}
	if display == "" {
		display = fmt.Sprintf("%d", from.ID)
	}
	return s.repo.UpsertTelegramUser(&model.TelegramUser{
		TelegramUserID: from.ID,
		ChatID:         msg.Chat.ID,
		Username:       from.Username,
		FirstName:      from.FirstName,
		LastName:       from.LastName,
		DisplayName:    display,
		Status:         "active",
	})
}

func (s *BotService) handleList(ctx context.Context, chatID int64, userID uint) {
	tokens, err := s.repo.ListTokensByUser(userID)
	if err != nil {
		s.reply(ctx, chatID, "读取 Token 列表失败，请稍后重试。")
		return
	}
	if len(tokens) == 0 {
		s.reply(ctx, chatID, "你还没有 Webhook Token。\n发送 /add 创建一个。")
		return
	}
	var b strings.Builder
	b.WriteString("你的 Webhook Token\n\n")
	for _, token := range tokens {
		status := "启用"
		if token.DisabledAt != nil {
			status = "禁用"
		}
		b.WriteString(fmt.Sprintf("别名：%s\n前缀：%s\n状态：%s\n使用次数：%d\n最近使用：%s\n\n",
			token.Alias, token.TokenPrefix, status, token.UseCount, formatTimePtr(token.LastUsedAt)))
	}
	s.reply(ctx, chatID, strings.TrimSpace(b.String()))
}

func (s *BotService) handleAdd(ctx context.Context, chatID int64, userID uint, alias string) {
	result, err := s.tokens.Create(userID, alias)
	if err != nil {
		if errors.Is(err, ErrAliasExists) {
			s.reply(ctx, chatID, "这个别名已经存在，请换一个。")
			return
		}
		s.reply(ctx, chatID, "创建 Token 失败，请稍后重试。")
		return
	}
	curlPayload := `{"title":"测试消息","text":"Hello from Hookgram","format":"markdown"}`
	text := fmt.Sprintf(`Webhook Token 创建成功

别名：%s
Token：%s

你的 Webhook 地址：

%s

快速测试：

curl -X POST "%s" -H "Content-Type: application/json" -d '%s'

说明：
Token 只会完整显示这一次。
请妥善保存。
丢失后可以删除并重新创建。`,
		result.Token.Alias,
		result.PlainText,
		result.WebhookURL,
		result.WebhookURL,
		curlPayload,
	)
	s.reply(ctx, chatID, text)
}

func (s *BotService) handleDelete(ctx context.Context, chatID int64, userID uint, value string) {
	token, err := s.tokens.DeleteByAliasOrPrefix(userID, value)
	if err != nil {
		s.reply(ctx, chatID, "没有找到这个 Token。")
		return
	}
	s.reply(ctx, chatID, fmt.Sprintf("已删除 Token：%s（%s）", token.Alias, token.TokenPrefix))
}

func (s *BotService) handleRename(ctx context.Context, chatID int64, userID uint, oldAlias, newAlias string) {
	token, err := s.repo.FindTokenByAlias(userID, oldAlias)
	if err != nil {
		s.reply(ctx, chatID, "没有找到旧别名对应的 Token。")
		return
	}
	updated, err := s.tokens.Rename(userID, token.ID, newAlias)
	if err != nil {
		if errors.Is(err, ErrAliasExists) {
			s.reply(ctx, chatID, "新别名已经存在，请换一个。")
			return
		}
		s.reply(ctx, chatID, "修改别名失败，请稍后重试。")
		return
	}
	s.reply(ctx, chatID, fmt.Sprintf("别名已更新：%s -> %s", oldAlias, updated.Alias))
}

func (s *BotService) handleURL(ctx context.Context, chatID int64, userID uint, alias string) {
	token, err := s.repo.FindTokenByAlias(userID, alias)
	if err != nil {
		s.reply(ctx, chatID, "没有找到这个别名。")
		return
	}
	s.reply(ctx, chatID, fmt.Sprintf(`Token：%s
前缀：%s

系统不会保存完整 Token 明文，因此无法重新展示完整 Webhook URL。
如果你已经丢失 Token，请删除后重新创建。

当前服务地址：%s/w/<你的完整Token>`, token.Alias, token.TokenPrefix, s.cfg.Current().BaseURL()))
}

func (s *BotService) handleUsage(ctx context.Context, chatID int64, userID uint, alias string) {
	token, err := s.repo.FindTokenByAlias(userID, alias)
	if err != nil {
		s.reply(ctx, chatID, "没有找到这个别名。")
		return
	}
	messages, err := s.repo.ListMessagesByToken(token.ID, 5)
	if err != nil {
		s.reply(ctx, chatID, "读取使用记录失败，请稍后重试。")
		return
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Token 使用情况\n\n别名：%s\n使用次数：%d\n最近使用：%s\n", token.Alias, token.UseCount, formatTimePtr(token.LastUsedAt)))
	if len(messages) > 0 {
		b.WriteString("\n最近推送：\n")
		for _, msg := range messages {
			title := msg.Title
			if title == "" {
				title = "无标题"
			}
			b.WriteString(fmt.Sprintf("%s｜%s｜%s\n", msg.CreatedAt.Format("01-02 15:04"), msg.DeliveryStatus, title))
		}
	}
	s.reply(ctx, chatID, b.String())
}

func (s *BotService) reply(ctx context.Context, chatID int64, text string) {
	_, err := s.telegram.SendMessage(ctx, chatID, text, "")
	if err != nil {
		log.Printf("发送 Bot 回复失败: %v", err)
	}
}

func sleepContext(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func normalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "plain", "markdown", "html":
		return strings.ToLower(strings.TrimSpace(format))
	default:
		return "plain"
	}
}

func normalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "info", "success", "warning", "error":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return "info"
	}
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "暂无"
	}
	return t.Format("2006-01-02 15:04:05")
}

func FieldsFromAny(value any) map[string]string {
	fields := map[string]string{}
	switch typed := value.(type) {
	case map[string]any:
		for key, val := range typed {
			fields[key] = fieldValue(val)
		}
	case map[string]string:
		for key, val := range typed {
			fields[key] = val
		}
	}
	return fields
}

func fieldValue(value any) string {
	switch value.(type) {
	case map[string]any, []any, map[string]string:
		data, err := json.Marshal(value)
		if err == nil {
			return string(data)
		}
	}
	return fmt.Sprint(value)
}

func RawJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func SortedKeys(value map[string]string) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
