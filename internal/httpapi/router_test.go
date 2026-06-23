package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"hookgram/internal/config"
	"hookgram/internal/database"
	"hookgram/internal/model"
	"hookgram/internal/repository"
	"hookgram/internal/service"
	"hookgram/internal/telegram"

	"github.com/gin-gonic/gin"
)

type httpFakeTelegram struct {
	texts      []string
	parseModes []string
}

func (f *httpFakeTelegram) GetUpdates(context.Context, int64, int) ([]telegram.Update, error) {
	return nil, context.Canceled
}

func (f *httpFakeTelegram) SendMessage(_ context.Context, _ int64, text, parseMode string) (int, error) {
	f.texts = append(f.texts, text)
	f.parseModes = append(f.parseModes, parseMode)
	return 1000 + len(f.texts), nil
}

func TestSetupLoginTokenAndWebhookFlow(t *testing.T) {
	dir := t.TempDir()
	cfgManager, err := config.LoadOrCreate(filepath.Join(dir, "data", "config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := cfgManager.Update(func(cfg *config.Config) {
		cfg.Database.DSN = filepath.Join(dir, "data", "hookgram.db")
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}
	db, err := database.Open(cfgManager.Current())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	defer sqlDB.Close()
	repo := repository.New(db)
	services := service.NewContainer(cfgManager, repo, telegram.NewClient(cfgManager))
	router := NewRouter(cfgManager, services)

	assertJSON(t, router, http.MethodGet, "/api/setup/status", nil, http.StatusOK, `"initialized":false`)
	assertJSON(t, router, http.MethodGet, "/api/version", nil, http.StatusOK, `"version":"v0.1.0-rc.1"`)
	setupPage := httptest.NewRecorder()
	router.ServeHTTP(setupPage, httptest.NewRequest(http.MethodGet, "/setup", nil))
	if setupPage.Code != http.StatusOK || !strings.Contains(setupPage.Body.String(), "Hookgram") {
		t.Fatalf("setup page status = %d, body = %s", setupPage.Code, setupPage.Body.String())
	}

	setupBody := map[string]string{
		"username":  "admin",
		"password":  "secret123",
		"bot_token": "test-token",
		"api_proxy": "http://127.0.0.1:1",
		"base_url":  "http://127.0.0.1:8787",
	}
	assertJSON(t, router, http.MethodPost, "/api/setup", setupBody, http.StatusOK, `"ok":true`)
	assertJSON(t, router, http.MethodGet, "/api/setup/status", nil, http.StatusOK, `"initialized":true`)
	assertJSON(t, router, http.MethodPost, "/api/setup", setupBody, http.StatusConflict, "系统已完成初始化")

	loginRes := doJSON(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "secret123",
	}, "")
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRes.Code, loginRes.Body.String())
	}
	cookie := loginRes.Result().Cookies()[0].String()

	settingsRes := doJSON(t, router, http.MethodPatch, "/api/admin/settings", map[string]string{
		"api_proxy": "http://127.0.0.1:18081/proxy",
	}, cookie)
	if settingsRes.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", settingsRes.Code, settingsRes.Body.String())
	}
	settingsRead := doJSON(t, router, http.MethodGet, "/api/admin/settings", nil, cookie)
	if settingsRead.Code != http.StatusOK || !strings.Contains(settingsRead.Body.String(), "18081/proxy") {
		t.Fatalf("settings read status = %d, body = %s", settingsRead.Code, settingsRead.Body.String())
	}

	user, err := repo.UpsertTelegramUser(&model.TelegramUser{
		TelegramUserID: 10001,
		ChatID:         10001,
		Username:       "tester",
		DisplayName:    "Tester",
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("upsert telegram user: %v", err)
	}

	tokenRes := doJSON(t, router, http.MethodPost, "/api/admin/users/"+uintString(user.ID)+"/tokens", map[string]string{
		"alias": "ci",
	}, cookie)
	if tokenRes.Code != http.StatusOK {
		t.Fatalf("create token status = %d, body = %s", tokenRes.Code, tokenRes.Body.String())
	}
	var tokenPayload struct {
		PlainToken string `json:"plain_token"`
	}
	if err := json.Unmarshal(tokenRes.Body.Bytes(), &tokenPayload); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if tokenPayload.PlainToken == "" {
		t.Fatal("expected one-time plain token")
	}

	assertJSON(t, router, http.MethodGet, "/w/"+tokenPayload.PlainToken+"?title=GET&text=hello&level=info", nil, http.StatusBadGateway, "delivery failed")
	assertJSON(t, router, http.MethodPost, "/w/"+tokenPayload.PlainToken, map[string]any{
		"title":  "POST",
		"text":   "hello json",
		"format": "markdown",
		"fields": map[string]any{"env": "test"},
	}, http.StatusBadGateway, "delivery failed")

	messages, err := repo.ListMessagesByUser(user.ID, 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 webhook messages, got %d", len(messages))
	}
	if messages[0].DeliveryError == "" {
		t.Fatal("expected delivery error to be recorded without real bot token")
	}
}

func assertJSON(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int, contains string) {
	t.Helper()
	res := doJSON(t, handler, method, path, body, "")
	if res.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d, body = %s", method, path, res.Code, wantStatus, res.Body.String())
	}
	if contains != "" && !strings.Contains(res.Body.String(), contains) {
		t.Fatalf("%s %s body = %s, want contains %s", method, path, res.Body.String(), contains)
	}
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func uintString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}

func TestBotRuntimeConfigUpdateDoesNotStartBeforeParentContext(t *testing.T) {
	dir := t.TempDir()
	cfgManager, err := config.LoadOrCreate(filepath.Join(dir, "data", "config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := cfgManager.Update(func(cfg *config.Config) {
		cfg.Database.DSN = filepath.Join(dir, "data", "hookgram.db")
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}
	db, err := database.Open(cfgManager.Current())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	defer sqlDB.Close()
	services := service.NewContainer(cfgManager, repository.New(db), telegram.NewClient(cfgManager))
	if err := cfgManager.Update(func(cfg *config.Config) {
		cfg.Telegram.BotToken = "test-token"
		cfg.Telegram.APIProxy = "http://127.0.0.1:1"
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}
	services.Bot.UpdateConfig()
	if services.Bot.IsRunning() {
		t.Fatal("bot runtime should not start before Start receives parent context")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	services.Bot.Start(ctx)
	if !services.Bot.IsRunning() {
		t.Fatal("bot runtime should start after Start with configured token")
	}
	if err := cfgManager.Update(func(cfg *config.Config) {
		cfg.Telegram.BotToken = ""
	}); err != nil {
		t.Fatalf("clear token: %v", err)
	}
	services.Bot.UpdateConfig()
	if services.Bot.IsRunning() {
		t.Fatal("bot runtime should stop after token is cleared")
	}
}

func TestWebhookGetPostBehavior(t *testing.T) {
	fake := &httpFakeTelegram{}
	router, repo, user, token, closeDB := newWebhookRouterFixture(t, fake)
	defer closeDB()

	var logs bytes.Buffer
	oldWriter := gin.DefaultWriter
	gin.DefaultWriter = &logs
	defer func() { gin.DefaultWriter = oldWriter }()

	assertJSON(t, router, http.MethodGet, "/w/"+token.PlainText+"?title=GETTitle&text=hello-get&level=success&source=browser", nil, http.StatusOK, `"message":"sent"`)
	assertJSON(t, router, http.MethodGet, "/w/"+token.PlainText+"?title=OnlyTitle", nil, http.StatusOK, `"message":"sent"`)
	assertJSON(t, router, http.MethodGet, "/w/"+token.PlainText, nil, http.StatusBadRequest, "消息内容不能为空")

	jsonRes := doJSON(t, router, http.MethodPost, "/w/"+token.PlainText, map[string]any{
		"title":  "JSONTitle",
		"text":   "**hello-json**",
		"format": "markdown",
		"level":  "success",
		"source": "deploy-system",
		"fields": map[string]any{"环境": "prod", "服务": "api-server"},
	}, "")
	if jsonRes.Code != http.StatusOK {
		t.Fatalf("json post status = %d, body = %s", jsonRes.Code, jsonRes.Body.String())
	}
	if len(fake.texts) < 3 || !strings.Contains(fake.texts[len(fake.texts)-1], "环境") {
		t.Fatalf("expected rendered fields in Telegram text, got %#v", fake.texts)
	}

	textRes := doRaw(t, router, http.MethodPost, "/w/"+token.PlainText+"?title=PlainTitle&level=warning&source=script", "text/plain", "plain body")
	if textRes.Code != http.StatusOK {
		t.Fatalf("text post status = %d, body = %s", textRes.Code, textRes.Body.String())
	}

	form := url.Values{}
	form.Set("title", "FormTitle")
	form.Set("message", "message-body")
	form.Set("content", "content-body")
	form.Set("level", "info")
	form.Set("source", "form-system")
	formRes := doRaw(t, router, http.MethodPost, "/w/"+token.PlainText, "application/x-www-form-urlencoded", form.Encode())
	if formRes.Code != http.StatusOK {
		t.Fatalf("form post status = %d, body = %s", formRes.Code, formRes.Body.String())
	}

	messages, err := repo.ListMessagesByUser(user.ID, 20)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 6 {
		t.Fatalf("expected 6 messages including empty failure, got %d", len(messages))
	}
	updatedToken, err := repo.FindTokenByID(token.Token.ID)
	if err != nil {
		t.Fatalf("find token: %v", err)
	}
	if updatedToken.UseCount != 5 {
		t.Fatalf("use_count = %d, want 5", updatedToken.UseCount)
	}
	if messages[0].Content != "message-body" || messages[0].Source != "form-system" {
		t.Fatalf("form parsing failed, latest message = %#v", messages[0])
	}
	if messages[1].Content != "plain body" || messages[1].Source != "script" {
		t.Fatalf("text/plain parsing failed, message = %#v", messages[1])
	}
	if messages[2].Source != "deploy-system" {
		t.Fatalf("json source missing, message = %#v", messages[2])
	}
	if messages[5].Source != "browser" || messages[5].Title != "GETTitle" || messages[5].Content != "hello-get" {
		t.Fatalf("get parsing failed, oldest message = %#v", messages[5])
	}
	if strings.Contains(logs.String(), token.PlainText) {
		t.Fatalf("webhook token leaked in logs: %s", logs.String())
	}
}

func TestWebhookInvalidAndDisabledTokenResponses(t *testing.T) {
	fake := &httpFakeTelegram{}
	router, repo, user, token, closeDB := newWebhookRouterFixture(t, fake)
	defer closeDB()

	assertJSON(t, router, http.MethodGet, "/w/not-a-real-token?text=hello", nil, http.StatusUnauthorized, "invalid token")
	assertJSON(t, router, http.MethodPost, "/w/not-a-real-token", map[string]string{"text": "hello"}, http.StatusUnauthorized, "invalid token")

	if _, err := repo.FindTelegramUserByID(user.ID); err != nil {
		t.Fatalf("find user: %v", err)
	}
	servicesToken := token.Token
	nowDisabled, err := repo.FindTokenByID(servicesToken.ID)
	if err != nil {
		t.Fatalf("find token: %v", err)
	}
	now := time.Now()
	nowDisabled.DisabledAt = &now
	if err := repo.UpdateToken(nowDisabled); err != nil {
		t.Fatalf("disable token: %v", err)
	}

	assertJSON(t, router, http.MethodGet, "/w/"+token.PlainText+"?text=hello", nil, http.StatusForbidden, "token disabled")
	assertJSON(t, router, http.MethodPost, "/w/"+token.PlainText, map[string]string{"text": "hello"}, http.StatusForbidden, "token disabled")
}

func doRaw(t *testing.T, handler http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func newWebhookRouterFixture(t *testing.T, tg *httpFakeTelegram) (http.Handler, *repository.Repository, *model.TelegramUser, *service.TokenResult, func()) {
	t.Helper()
	dir := t.TempDir()
	cfgManager, err := config.LoadOrCreate(filepath.Join(dir, "data", "config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := cfgManager.Update(func(cfg *config.Config) {
		cfg.Database.DSN = filepath.Join(dir, "data", "hookgram.db")
		cfg.App.BaseURL = "http://127.0.0.1:8787"
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}
	db, err := database.Open(cfgManager.Current())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	repo := repository.New(db)
	services := service.NewContainer(cfgManager, repo, tg)
	router := NewRouter(cfgManager, services)
	user, err := repo.UpsertTelegramUser(&model.TelegramUser{
		TelegramUserID: 30001,
		ChatID:         30001,
		Username:       "webhook_tester",
		DisplayName:    "Webhook Tester",
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("upsert telegram user: %v", err)
	}
	token, err := services.Tokens.Create(user.ID, "webhook")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	return router, repo, user, token, func() { _ = sqlDB.Close() }
}
