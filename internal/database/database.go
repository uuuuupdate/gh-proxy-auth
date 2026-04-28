package database

import (
	"fmt"

	"github.com/uuuuupdate/gh-proxy-auth/internal/config"
	"github.com/uuuuupdate/gh-proxy-auth/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init() error {
	cfg := config.C
	var dialector gorm.Dialector

	switch cfg.Database.Driver {
	case "sqlite":
		dialector = sqlite.Open(cfg.Database.DSN)
	case "mysql":
		dialector = mysql.Open(cfg.Database.DSN)
	case "postgres":
		dialector = postgres.Open(cfg.Database.DSN)
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Token{},
		&models.DownloadLog{},
		&models.Setting{},
		&models.Passkey{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	DB = db
	return nil
}

func GetSetting(key string) string {
	var setting models.Setting
	if err := DB.Where("key = ?", key).First(&setting).Error; err != nil {
		return ""
	}
	return setting.Value
}

func SetSetting(key, value string) error {
	setting := models.Setting{Key: key, Value: value}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "deleted_at"}),
	}).Create(&setting).Error
}
