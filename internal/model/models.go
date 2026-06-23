package model

import "time"

type AdminUser struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	Username     string     `json:"username" gorm:"size:80;uniqueIndex;not null"`
	PasswordHash string     `json:"-" gorm:"size:255;not null"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

type TelegramUser struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	TelegramUserID int64      `json:"telegram_user_id" gorm:"uniqueIndex;not null"`
	ChatID         int64      `json:"chat_id" gorm:"index;not null"`
	Username       string     `json:"username" gorm:"size:120"`
	FirstName      string     `json:"first_name" gorm:"size:120"`
	LastName       string     `json:"last_name" gorm:"size:120"`
	DisplayName    string     `json:"display_name" gorm:"size:240"`
	Status         string     `json:"status" gorm:"size:30;index;not null;default:active"`
	LastSeenAt     *time.Time `json:"last_seen_at" gorm:"index"`
	BlockedAt      *time.Time `json:"blocked_at" gorm:"index"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"index"`
}

type WebhookToken struct {
	ID             uint         `json:"id" gorm:"primaryKey"`
	TelegramUserID uint         `json:"telegram_user_id" gorm:"index;not null;uniqueIndex:idx_tg_alias"`
	TelegramUser   TelegramUser `json:"-"`
	Alias          string       `json:"alias" gorm:"size:120;not null;uniqueIndex:idx_tg_alias"`
	TokenHash      string       `json:"-" gorm:"size:128;uniqueIndex;not null"`
	TokenPrefix    string       `json:"token_prefix" gorm:"size:32;index;not null"`
	UseCount       int64        `json:"use_count" gorm:"not null;default:0"`
	LastUsedAt     *time.Time   `json:"last_used_at" gorm:"index"`
	DisabledAt     *time.Time   `json:"disabled_at" gorm:"index"`
	CreatedAt      time.Time    `json:"created_at" gorm:"index"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type WebhookMessage struct {
	ID                uint         `json:"id" gorm:"primaryKey"`
	WebhookTokenID    uint         `json:"webhook_token_id" gorm:"index"`
	WebhookToken      WebhookToken `json:"token,omitempty"`
	TelegramUserID    uint         `json:"telegram_user_id" gorm:"index"`
	TelegramUser      TelegramUser `json:"telegram_user,omitempty"`
	Title             string       `json:"title" gorm:"size:240"`
	Content           string       `json:"content" gorm:"type:text"`
	RawPayload        string       `json:"raw_payload" gorm:"type:text"`
	Format            string       `json:"format" gorm:"size:30"`
	Level             string       `json:"level" gorm:"size:30;index"`
	Source            string       `json:"source" gorm:"size:180;index"`
	SourceIP          string       `json:"source_ip" gorm:"size:80"`
	UserAgent         string       `json:"user_agent" gorm:"size:300"`
	DeliveryStatus    string       `json:"delivery_status" gorm:"size:30;index"`
	DeliveryError     string       `json:"delivery_error" gorm:"type:text"`
	TelegramMessageID int          `json:"telegram_message_id"`
	CreatedAt         time.Time    `json:"created_at" gorm:"index"`
}

type AppSetting struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"size:120;uniqueIndex;not null"`
	Value     string    `json:"value" gorm:"type:text"`
	ValueType string    `json:"value_type" gorm:"size:30;not null;default:string"`
	Remark    string    `json:"remark" gorm:"size:300"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
