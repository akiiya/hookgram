package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"hookgram/internal/config"
	"hookgram/internal/database"
	"hookgram/internal/model"
	"hookgram/internal/repository"
	"hookgram/internal/telegram"
)

type fakeTelegram struct {
	failFirst error
	failAll   error
	calls     []string
}

func (f *fakeTelegram) GetUpdates(context.Context, int64, int) ([]telegram.Update, error) {
	return nil, context.Canceled
}

func (f *fakeTelegram) SendMessage(_ context.Context, _ int64, _ string, parseMode string) (int, error) {
	f.calls = append(f.calls, parseMode)
	if f.failAll != nil {
		return 0, f.failAll
	}
	if f.failFirst != nil && len(f.calls) == 1 {
		return 0, f.failFirst
	}
	return 900 + len(f.calls), nil
}

func TestWebhookDeliverFallsBackToPlainText(t *testing.T) {
	services, repo, user, plainToken, closeDB := newWebhookTestServices(t, &fakeTelegram{
		failFirst: telegram.TelegramError{Code: 400, Description: "bad markdown"},
	})
	defer closeDB()

	message, err := services.Webhook.Deliver(context.Background(), WebhookInput{
		Token:   plainToken,
		Title:   "Markdown",
		Content: "**hello**",
		Format:  "markdown",
		Level:   "success",
		Fields:  map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatalf("deliver failed: %v", err)
	}
	if message.DeliveryStatus != "sent" {
		t.Fatalf("status = %s, want sent", message.DeliveryStatus)
	}
	if !strings.Contains(message.DeliveryError, "已降级为纯文本") {
		t.Fatalf("expected fallback note, got %q", message.DeliveryError)
	}
	messages, err := repo.ListMessagesByUser(user.ID, 10)
	if err != nil || len(messages) != 1 {
		t.Fatalf("messages len = %d, err = %v", len(messages), err)
	}
}

func TestWebhookDeliverMarksBlockedOnForbidden(t *testing.T) {
	services, repo, user, plainToken, closeDB := newWebhookTestServices(t, &fakeTelegram{
		failAll: telegram.TelegramError{Code: 403, Description: "Forbidden: bot was blocked by the user"},
	})
	defer closeDB()

	message, err := services.Webhook.Deliver(context.Background(), WebhookInput{
		Token:   plainToken,
		Title:   "Blocked",
		Content: "hello",
		Format:  "plain",
		Level:   "error",
	})
	if err == nil {
		t.Fatal("expected delivery error")
	}
	if message.DeliveryStatus != "failed" {
		t.Fatalf("status = %s, want failed", message.DeliveryStatus)
	}
	updated, err := repo.FindTelegramUserByID(user.ID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if updated.Status != "blocked" || updated.BlockedAt == nil {
		t.Fatalf("expected blocked user, got status=%s blocked_at=%v", updated.Status, updated.BlockedAt)
	}
}

func newWebhookTestServices(t *testing.T, tg *fakeTelegram) (*Container, *repository.Repository, *model.TelegramUser, string, func()) {
	t.Helper()
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
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	repo := repository.New(db)
	services := NewContainer(cfgManager, repo, tg)
	user, err := repo.UpsertTelegramUser(&model.TelegramUser{
		TelegramUserID: 20001,
		ChatID:         20001,
		Username:       "service_test",
		DisplayName:    "Service Test",
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	token, err := services.Tokens.Create(user.ID, "service")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	return services, repo, user, token.PlainText, func() { _ = sqlDB.Close() }
}
