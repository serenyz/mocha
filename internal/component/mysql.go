package component

import (
	"fmt"
	"mmchat/internal/config"
	"mmchat/internal/model"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitMysql(cfg *config.MysqlConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DatabaseName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open mysql failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db failed: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(
		time.Duration(cfg.ConnMaxLifetime) * time.Second,
	)

	err = db.AutoMigrate(
		&model.User{},
		&model.UserProfile{},
		&model.Media{},
		&model.Conversation{},
		&model.ConversationDirect{},
		&model.ConversationGroup{},
		&model.ConversationMember{},
		&model.Message{},
		&model.MessageAttachment{},
	)
	if err != nil {
		return nil, fmt.Errorf("migrate database failed: %w", err)
	}
	return db, nil
}
