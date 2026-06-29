package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"hookgram/internal/auth"
	"hookgram/internal/config"
	"hookgram/internal/model"
	"hookgram/internal/service"
	"hookgram/internal/telegram"
	appversion "hookgram/internal/version"
	"hookgram/web"

	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg      *config.Manager
	services *service.Container
	assets   fs.FS
}

var webhookTokenPath = regexp.MustCompile(`^/w/[^/?#]+`)

func NewRouter(cfg *config.Manager, services *service.Container) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	dist, _ := fs.Sub(web.Dist, "dist")
	s := &Server{cfg: cfg, services: services, assets: dist}
	r := gin.New()
	r.Use(safeLogger(), safeRecovery())

	r.GET("/api/version", s.version)
	r.GET("/api/setup/status", s.setupStatus)
	r.POST("/api/setup", s.setup)

	r.POST("/api/auth/login", s.login)
	r.POST("/api/auth/logout", s.logout)
	r.GET("/api/auth/me", s.requireAuth(), s.me)
	r.POST("/api/auth/change-password", s.requireAuth(), s.changePassword)

	admin := r.Group("/api/admin", s.requireAuth())
	admin.GET("/dashboard", s.dashboard)
	admin.GET("/users", s.users)
	admin.GET("/users/:id", s.userDetail)
	admin.GET("/users/:id/tokens", s.userTokens)
	admin.POST("/users/:id/tokens", s.createUserToken)
	admin.PATCH("/users/:id/tokens/:tokenId", s.updateUserToken)
	admin.DELETE("/users/:id/tokens/:tokenId", s.deleteUserToken)
	admin.GET("/users/:id/messages", s.userMessages)
	admin.GET("/messages", s.messages)
	admin.GET("/messages/:id", s.messageDetail)
	admin.GET("/settings", s.settings)
	admin.PATCH("/settings", s.updateSettings)
	admin.POST("/settings/telegram/test", s.testTelegram)

	r.GET("/w/:token", s.webhook)
	r.POST("/w/:token", s.webhook)

	r.NoRoute(s.spa)
	return r
}

func (s *Server) version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":   appversion.Version,
		"commit":    "unknown",
		"buildDate": "unknown",
		"platform":  runtime.GOOS + "/" + runtime.GOARCH,
	})
}

func (s *Server) setupStatus(c *gin.Context) {
	initialized, err := s.services.Auth.SetupComplete()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "读取初始化状态失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"initialized": initialized})
}

func (s *Server) setup(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		BotToken string `json:"bot_token"`
		APIProxy string `json:"api_proxy"`
		BaseURL  string `json:"base_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.services.Auth.Setup(req.Username, req.Password, req.BotToken, req.APIProxy, req.BaseURL); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrAlreadySetup) {
			status = http.StatusConflict
		}
		errorJSON(c, status, err.Error())
		return
	}
	s.services.Bot.UpdateConfig()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	admin, session, err := s.services.Auth.Login(req.Username, req.Password)
	if err != nil {
		errorJSON(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	setSessionCookie(c, session.Token, session.ExpiresAt)
	c.JSON(http.StatusOK, gin.H{"ok": true, "admin": admin})
}

func (s *Server) logout(c *gin.Context) {
	if cookie, err := c.Cookie(auth.CookieName); err == nil {
		s.services.Sessions.Delete(cookie)
	}
	clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) me(c *gin.Context) {
	adminID := c.GetUint("admin_id")
	admin, err := s.services.Repo.FindAdminByID(adminID)
	if err != nil {
		errorJSON(c, http.StatusUnauthorized, "登录已失效")
		return
	}
	c.JSON(http.StatusOK, gin.H{"admin": admin})
}

func (s *Server) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.services.Auth.ChangePassword(c.GetUint("admin_id"), req.OldPassword, req.NewPassword); err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) dashboard(c *gin.Context) {
	users, err := s.services.Repo.CountTelegramUsers()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "读取统计失败")
		return
	}
	tokens, _ := s.services.Repo.CountTokens()
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today, _ := s.services.Repo.CountMessagesSince(start, "")
	sent, _ := s.services.Repo.CountMessagesSince(start, "sent")
	failed, _ := s.services.Repo.CountMessagesSince(start, "failed")
	recent, _ := s.services.Repo.ListRecentMessages(8)
	c.JSON(http.StatusOK, gin.H{
		"bot_users":      users,
		"tokens":         tokens,
		"today_messages": today,
		"sent":           sent,
		"failed":         failed,
		"recent":         recent,
	})
}

func (s *Server) users(c *gin.Context) {
	users, err := s.services.Repo.ListTelegramUsers()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "读取用户失败")
		return
	}
	type row struct {
		model.TelegramUser
		TokenCount   int64 `json:"token_count"`
		MessageCount int64 `json:"message_count"`
	}
	rows := make([]row, 0, len(users))
	for _, user := range users {
		tokenCount, _ := s.services.Repo.CountTokensByUser(user.ID)
		messageCount, _ := s.services.Repo.CountMessagesByUser(user.ID)
		rows = append(rows, row{TelegramUser: user, TokenCount: tokenCount, MessageCount: messageCount})
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) userDetail(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	user, err := s.services.Repo.FindTelegramUserByID(id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, "用户不存在")
		return
	}
	tokenCount, _ := s.services.Repo.CountTokensByUser(user.ID)
	messageCount, _ := s.services.Repo.CountMessagesByUser(user.ID)
	c.JSON(http.StatusOK, gin.H{"user": user, "token_count": tokenCount, "message_count": messageCount})
}

func (s *Server) userTokens(c *gin.Context) {
	userID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	tokens, err := s.services.Repo.ListTokensByUser(userID)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "读取 Token 失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": tokens})
}

func (s *Server) createUserToken(c *gin.Context) {
	userID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		errorJSON(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	result, err := s.services.Tokens.Create(userID, req.Alias)
	if err != nil {
		if errors.Is(err, service.ErrAliasExists) {
			errorJSON(c, http.StatusConflict, "别名已存在")
			return
		}
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) updateUserToken(c *gin.Context) {
	userID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	tokenID, ok := parseUintParam(c, "tokenId")
	if !ok {
		return
	}
	var req struct {
		Alias    *string `json:"alias"`
		Disabled *bool   `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	var token *model.WebhookToken
	var err error
	if req.Alias != nil {
		token, err = s.services.Tokens.Rename(userID, tokenID, *req.Alias)
		if err != nil {
			if errors.Is(err, service.ErrAliasExists) {
				errorJSON(c, http.StatusConflict, "别名已存在")
				return
			}
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	if req.Disabled != nil {
		token, err = s.services.Tokens.SetDisabled(userID, tokenID, *req.Disabled)
		if err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	if token == nil {
		found, err := s.services.Repo.FindTokenByID(tokenID)
		if err != nil {
			errorJSON(c, http.StatusNotFound, "Token 不存在")
			return
		}
		token = found
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (s *Server) deleteUserToken(c *gin.Context) {
	userID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	tokenID, ok := parseUintParam(c, "tokenId")
	if !ok {
		return
	}
	if err := s.services.Tokens.Delete(userID, tokenID); err != nil {
		errorJSON(c, http.StatusNotFound, "Token 不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) userMessages(c *gin.Context) {
	userID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	messages, err := s.services.Repo.ListMessagesByUser(userID, 100)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "读取推送记录失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": messages})
}

func (s *Server) messages(c *gin.Context) {
	messages, err := s.services.Repo.ListRecentMessages(100)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "读取推送记录失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": messages})
}

func (s *Server) messageDetail(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	message, err := s.services.Repo.FindMessageByID(id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, "记录不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": message})
}

func (s *Server) settings(c *gin.Context) {
	cfg := s.cfg.Current()
	c.JSON(http.StatusOK, gin.H{
		"config_path":        s.cfg.Path(),
		"database_driver":    cfg.Database.Driver,
		"database_dsn":       cfg.Database.DSN,
		"base_url":           cfg.App.BaseURL,
		"effective_base_url": cfg.BaseURL(),
		"telegram_api_proxy": cfg.Telegram.APIProxy,
		"telegram_bot_token": maskSecret(cfg.Telegram.BotToken),
		"has_bot_token":      strings.TrimSpace(cfg.Telegram.BotToken) != "",
		"bot_polling":        s.services.Bot.IsRunning(),
	})
}

func (s *Server) updateSettings(c *gin.Context) {
	var req struct {
		BaseURL  *string `json:"base_url"`
		BotToken *string `json:"bot_token"`
		APIProxy *string `json:"api_proxy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.cfg.Update(func(cfg *config.Config) {
		if req.BaseURL != nil {
			cfg.App.BaseURL = strings.TrimRight(strings.TrimSpace(*req.BaseURL), "/")
		}
		if req.BotToken != nil {
			cfg.Telegram.BotToken = strings.TrimSpace(*req.BotToken)
		}
		if req.APIProxy != nil {
			cfg.Telegram.APIProxy = strings.TrimRight(strings.TrimSpace(*req.APIProxy), "/")
		}
	}); err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存配置失败")
		return
	}
	s.services.Bot.UpdateConfig()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) testTelegram(c *gin.Context) {
	var req struct {
		BotToken *string `json:"bot_token"`
		APIProxy *string `json:"api_proxy"`
	}
	_ = c.ShouldBindJSON(&req)
	cfg := s.cfg.Current()
	if req.BotToken != nil {
		cfg.Telegram.BotToken = strings.TrimSpace(*req.BotToken)
	}
	if req.APIProxy != nil {
		cfg.Telegram.APIProxy = strings.TrimRight(strings.TrimSpace(*req.APIProxy), "/")
	}
	client := telegram.NewClient(staticConfigProvider{cfg: cfg})
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	me, err := client.GetMe(ctx)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Telegram Bot API 测试失败："+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "bot": me})
}

func (s *Server) webhook(c *gin.Context) {
	input, err := parseWebhookInput(c)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	message, err := s.services.Webhook.Deliver(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "invalid token"})
			return
		}
		if errors.Is(err, service.ErrTokenDisabled) {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "token disabled"})
			return
		}
		if errors.Is(err, service.ErrEmptyMessage) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "消息内容不能为空", "message_id": messageID(message)})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": "delivery failed", "message_id": messageID(message)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "sent", "message_id": message.ID})
}

func parseWebhookInput(c *gin.Context) (service.WebhookInput, error) {
	input := service.WebhookInput{
		Token:     c.Param("token"),
		SourceIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Format:    c.Query("format"),
		Level:     c.Query("level"),
		Fields:    map[string]string{},
	}
	if c.Request.Method == http.MethodGet {
		if len(c.Request.URL.RawQuery) > 8192 {
			return input, errors.New("GET URL 过长，请改用 POST")
		}
		input.Title = c.Query("title")
		input.Content = firstNonEmpty(c.Query("text"), c.Query("message"), c.Query("content"))
		input.Source = c.Query("source")
		input.RawPayload = c.Request.URL.RawQuery
		return input, nil
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return input, errors.New("读取请求体失败")
	}
	input.RawPayload = string(raw)
	contentType := strings.ToLower(c.ContentType())
	switch contentType {
	case "application/json":
		var payload map[string]any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &payload); err != nil {
				return input, errors.New("JSON 格式错误")
			}
		}
		input.Title = stringValue(payload["title"])
		input.Content = firstNonEmpty(stringValue(payload["text"]), stringValue(payload["message"]), stringValue(payload["content"]))
		input.Format = firstNonEmpty(stringValue(payload["format"]), input.Format)
		input.Level = firstNonEmpty(stringValue(payload["level"]), input.Level)
		input.Source = stringValue(payload["source"])
		input.Fields = service.FieldsFromAny(payload["fields"])
	case "application/x-www-form-urlencoded":
		values, err := urlValuesFromRaw(raw)
		if err != nil {
			return input, errors.New("表单格式错误")
		}
		input.Title = values.Get("title")
		input.Content = firstNonEmpty(values.Get("text"), values.Get("message"), values.Get("content"))
		input.Format = firstNonEmpty(values.Get("format"), input.Format)
		input.Level = firstNonEmpty(values.Get("level"), input.Level)
		input.Source = values.Get("source")
	default:
		input.Title = c.Query("title")
		input.Level = firstNonEmpty(c.Query("level"), input.Level)
		input.Source = c.Query("source")
		input.Content = string(raw)
		if input.Format == "" {
			input.Format = "plain"
		}
	}
	return input, nil
}

func (s *Server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(auth.CookieName)
		if err != nil || token == "" {
			errorJSON(c, http.StatusUnauthorized, "请先登录")
			c.Abort()
			return
		}
		session, ok := s.services.Sessions.Get(token)
		if !ok {
			clearSessionCookie(c)
			errorJSON(c, http.StatusUnauthorized, "登录已失效")
			c.Abort()
			return
		}
		c.Set("admin_id", session.AdminID)
		c.Next()
	}
}

func (s *Server) spa(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/w/") {
		errorJSON(c, http.StatusNotFound, "资源不存在")
		return
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		errorJSON(c, http.StatusNotFound, "资源不存在")
		return
	}
	name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	if data, info, err := readAsset(s.assets, name); err == nil && !info.IsDir() {
		serveBytes(c, name, data, info.ModTime())
		return
	}
	if filepath.Ext(name) != "" {
		errorJSON(c, http.StatusNotFound, "资源不存在")
		return
	}
	data, info, err := readAsset(s.assets, "index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "前端资源不存在，请先构建 web/dist")
		return
	}
	serveBytes(c, "index.html", data, info.ModTime())
}

func readAsset(assets fs.FS, name string) ([]byte, fs.FileInfo, error) {
	file, err := assets.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	data, err := io.ReadAll(file)
	return data, info, err
}

func serveBytes(c *gin.Context, name string, data []byte, modTime time.Time) {
	http.ServeContent(c.Writer, c.Request, name, modTime, bytes.NewReader(data))
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	raw := c.Param(name)
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		errorJSON(c, http.StatusBadRequest, "参数错误")
		return 0, false
	}
	return uint(value), true
}

func errorJSON(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"ok": false, "error": message})
}

func setSessionCookie(c *gin.Context, token string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return strings.TrimSpace(strings.Trim(fmt.Sprint(typed), "\""))
	}
}

func urlValuesFromRaw(raw []byte) (url.Values, error) {
	return url.ParseQuery(string(raw))
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 10 {
		return "****"
	}
	return value[:6] + "..." + value[len(value)-4:]
}

func messageID(message *model.WebhookMessage) uint {
	if message == nil {
		return 0
	}
	return message.ID
}

func safeLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		maskedPath := maskRequestPath(c.Request.URL.Path)
		if c.Request.URL.RawQuery != "" && !strings.HasPrefix(maskedPath, "/w/<token>") {
			maskedPath += "?" + c.Request.URL.RawQuery
		}
		gin.DefaultWriter.Write([]byte(fmt.Sprintf("[GIN] %3d | %13v | %15s | %-7s %s\n",
			c.Writer.Status(),
			latency,
			c.ClientIP(),
			c.Request.Method,
			maskedPath,
		)))
	}
}

func safeRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("HTTP 请求异常: path=%s error=%v", maskRequestPath(c.Request.URL.Path), recovered)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "服务器内部错误"})
	})
}

func maskRequestPath(path string) string {
	return webhookTokenPath.ReplaceAllString(path, "/w/<token>")
}

type staticConfigProvider struct {
	cfg config.Config
}

func (p staticConfigProvider) Current() config.Config {
	return p.cfg
}
