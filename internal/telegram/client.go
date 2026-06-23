package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hookgram/internal/config"
)

var ErrNoBotToken = errors.New("未配置 Telegram Bot Token")

type ConfigProvider interface {
	Current() config.Config
}

type Client struct {
	cfg  ConfigProvider
	http *http.Client
}

func NewClient(cfg ConfigProvider) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 70 * time.Second,
		},
	}
}

type TelegramError struct {
	Code        int
	Description string
}

func (e TelegramError) Error() string {
	if e.Description == "" {
		return fmt.Sprintf("Telegram API 错误: %d", e.Code)
	}
	return e.Description
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Type      string `json:"type"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var user User
	if err := c.call(ctx, "getMe", map[string]any{}, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	payload := map[string]any{
		"timeout": timeout,
	}
	if offset > 0 {
		payload["offset"] = offset
	}
	var updates []Update
	if err := c.call(ctx, "getUpdates", payload, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text, parseMode string) (int, error) {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if strings.TrimSpace(parseMode) != "" {
		payload["parse_mode"] = parseMode
	}
	var message Message
	if err := c.call(ctx, "sendMessage", payload, &message); err != nil {
		return 0, err
	}
	return message.MessageID, nil
}

func (c *Client) call(ctx context.Context, method string, payload any, result any) error {
	cfg := c.cfg.Current()
	token := strings.TrimSpace(cfg.Telegram.BotToken)
	if token == "" {
		return ErrNoBotToken
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/bot%s/%s", cfg.TelegramAPIBase(), token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return errors.New("Telegram API 地址无效，请检查代理配置")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("Telegram API 请求失败，请检查网络、Token 或代理配置")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return errors.New("Telegram API 返回格式异常")
	}
	if !envelope.OK {
		code := envelope.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return TelegramError{Code: code, Description: sanitizeDescription(envelope.Description, token)}
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return errors.New("Telegram API 返回内容异常")
	}
	return nil
}

func sanitizeDescription(description, token string) string {
	description = strings.ReplaceAll(description, token, "<bot-token>")
	if len([]rune(description)) > 300 {
		runes := []rune(description)
		description = string(runes[:300]) + "..."
	}
	return description
}
