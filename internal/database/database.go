package database

import (
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hookgram/internal/config"
	"hookgram/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if driver == "" {
		driver = "sqlite"
	}

	var dialector gorm.Dialector
	switch driver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(cfg.Database.DSN), 0755); err != nil {
			return nil, err
		}
		dialector = sqlite.Open(cfg.Database.DSN)
	case "mysql", "mariadb":
		dialector = mysql.Open(cfg.Database.DSN)
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.Database.DSN)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", cfg.Database.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.New(stdlog.New(os.Stdout, "", stdlog.LstdFlags), logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&model.AdminUser{},
		&model.TelegramUser{},
		&model.WebhookToken{},
		&model.WebhookMessage{},
		&model.AppSetting{},
	); err != nil {
		return nil, err
	}
	return db, nil
}
