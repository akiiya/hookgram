package repository

import (
	"errors"
	"time"

	"hookgram/internal/model"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("记录不存在")

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) CountAdmins() (int64, error) {
	var count int64
	err := r.db.Model(&model.AdminUser{}).Count(&count).Error
	return count, err
}

func (r *Repository) CreateAdmin(admin *model.AdminUser) error {
	return r.db.Create(admin).Error
}

func (r *Repository) FindAdminByUsername(username string) (*model.AdminUser, error) {
	var admin model.AdminUser
	err := r.db.Where("username = ?", username).First(&admin).Error
	return ptrOrNotFound(&admin, err)
}

func (r *Repository) FindAdminByID(id uint) (*model.AdminUser, error) {
	var admin model.AdminUser
	err := r.db.First(&admin, id).Error
	return ptrOrNotFound(&admin, err)
}

func (r *Repository) UpdateAdminLoginAt(id uint, at time.Time) error {
	return r.db.Model(&model.AdminUser{}).Where("id = ?", id).Update("last_login_at", at).Error
}

func (r *Repository) UpdateAdminPassword(id uint, hash string) error {
	return r.db.Model(&model.AdminUser{}).Where("id = ?", id).Update("password_hash", hash).Error
}

func (r *Repository) UpsertTelegramUser(user *model.TelegramUser) (*model.TelegramUser, error) {
	var existing model.TelegramUser
	err := r.db.Where("telegram_user_id = ?", user.TelegramUserID).First(&existing).Error
	now := time.Now()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user.LastSeenAt = &now
		if user.Status == "" {
			user.Status = "active"
		}
		if err := r.db.Create(user).Error; err != nil {
			return nil, err
		}
		return user, nil
	}
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"chat_id":      user.ChatID,
		"username":     user.Username,
		"first_name":   user.FirstName,
		"last_name":    user.LastName,
		"display_name": user.DisplayName,
		"status":       "active",
		"last_seen_at": &now,
	}
	if err := r.db.Model(&existing).Updates(updates).Error; err != nil {
		return nil, err
	}
	existing.ChatID = user.ChatID
	existing.Username = user.Username
	existing.FirstName = user.FirstName
	existing.LastName = user.LastName
	existing.DisplayName = user.DisplayName
	existing.Status = "active"
	existing.LastSeenAt = &now
	return &existing, nil
}

func (r *Repository) FindTelegramUserByExternalID(id int64) (*model.TelegramUser, error) {
	var user model.TelegramUser
	err := r.db.Where("telegram_user_id = ?", id).First(&user).Error
	return ptrOrNotFound(&user, err)
}

func (r *Repository) FindTelegramUserByID(id uint) (*model.TelegramUser, error) {
	var user model.TelegramUser
	err := r.db.First(&user, id).Error
	return ptrOrNotFound(&user, err)
}

func (r *Repository) ListTelegramUsers() ([]model.TelegramUser, error) {
	var users []model.TelegramUser
	err := r.db.Order("updated_at desc").Find(&users).Error
	return users, err
}

func (r *Repository) CountTelegramUsers() (int64, error) {
	var count int64
	err := r.db.Model(&model.TelegramUser{}).Count(&count).Error
	return count, err
}

func (r *Repository) MarkTelegramBlocked(id uint, at time.Time) error {
	return r.db.Model(&model.TelegramUser{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": "blocked", "blocked_at": &at}).Error
}

func (r *Repository) CreateToken(token *model.WebhookToken) error {
	return r.db.Create(token).Error
}

func (r *Repository) UpdateToken(token *model.WebhookToken) error {
	return r.db.Save(token).Error
}

func (r *Repository) DeleteToken(id uint) error {
	return r.db.Delete(&model.WebhookToken{}, id).Error
}

func (r *Repository) ListTokensByUser(userID uint) ([]model.WebhookToken, error) {
	var tokens []model.WebhookToken
	err := r.db.Where("telegram_user_id = ?", userID).Order("created_at desc").Find(&tokens).Error
	return tokens, err
}

func (r *Repository) CountTokens() (int64, error) {
	var count int64
	err := r.db.Model(&model.WebhookToken{}).Count(&count).Error
	return count, err
}

func (r *Repository) CountTokensByUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.WebhookToken{}).Where("telegram_user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *Repository) FindTokenByID(id uint) (*model.WebhookToken, error) {
	var token model.WebhookToken
	err := r.db.First(&token, id).Error
	return ptrOrNotFound(&token, err)
}

func (r *Repository) FindTokenByHash(hash string) (*model.WebhookToken, error) {
	var token model.WebhookToken
	err := r.db.Preload("TelegramUser").Where("token_hash = ?", hash).First(&token).Error
	return ptrOrNotFound(&token, err)
}

func (r *Repository) FindTokenByAlias(userID uint, alias string) (*model.WebhookToken, error) {
	var token model.WebhookToken
	err := r.db.Where("telegram_user_id = ? AND alias = ?", userID, alias).First(&token).Error
	return ptrOrNotFound(&token, err)
}

func (r *Repository) FindTokenByAliasOrPrefix(userID uint, value string) (*model.WebhookToken, error) {
	var token model.WebhookToken
	err := r.db.Where("telegram_user_id = ? AND (alias = ? OR token_prefix = ?)", userID, value, value).First(&token).Error
	return ptrOrNotFound(&token, err)
}

func (r *Repository) TouchTokenUsage(id uint, at time.Time) error {
	var token model.WebhookToken
	if err := r.db.First(&token, id).Error; err != nil {
		return err
	}
	token.UseCount++
	token.LastUsedAt = &at
	return r.db.Save(&token).Error
}

func (r *Repository) CreateMessage(message *model.WebhookMessage) error {
	return r.db.Create(message).Error
}

func (r *Repository) CountMessages() (int64, error) {
	var count int64
	err := r.db.Model(&model.WebhookMessage{}).Count(&count).Error
	return count, err
}

func (r *Repository) CountMessagesSince(start time.Time, status string) (int64, error) {
	query := r.db.Model(&model.WebhookMessage{}).Where("created_at >= ?", start)
	if status != "" {
		query = query.Where("delivery_status = ?", status)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

func (r *Repository) CountMessagesByUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.WebhookMessage{}).Where("telegram_user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *Repository) ListRecentMessages(limit int) ([]model.WebhookMessage, error) {
	var messages []model.WebhookMessage
	err := r.db.Preload("WebhookToken").Preload("TelegramUser").
		Order("created_at desc").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func (r *Repository) ListMessagesByUser(userID uint, limit int) ([]model.WebhookMessage, error) {
	var messages []model.WebhookMessage
	err := r.db.Preload("WebhookToken").
		Where("telegram_user_id = ?", userID).
		Order("created_at desc").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func (r *Repository) ListMessagesByToken(tokenID uint, limit int) ([]model.WebhookMessage, error) {
	var messages []model.WebhookMessage
	err := r.db.Where("webhook_token_id = ?", tokenID).
		Order("created_at desc").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func (r *Repository) FindMessageByID(id uint) (*model.WebhookMessage, error) {
	var message model.WebhookMessage
	err := r.db.Preload("WebhookToken").Preload("TelegramUser").First(&message, id).Error
	return ptrOrNotFound(&message, err)
}

func (r *Repository) UpsertSetting(setting *model.AppSetting) error {
	var existing model.AppSetting
	err := r.db.Where("key = ?", setting.Key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.Create(setting).Error
	}
	if err != nil {
		return err
	}
	return r.db.Model(&existing).Updates(map[string]any{
		"value":      setting.Value,
		"value_type": setting.ValueType,
		"remark":     setting.Remark,
	}).Error
}

func ptrOrNotFound[T any](value *T, err error) (*T, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}
