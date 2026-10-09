package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(path string) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建数据目录: %w", err)
		}
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(time.Hour)
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if err := db.Exec(p).Error; err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.Group{},
		&model.Monitor{},
		&model.Notifier{},
		&model.AlertEvent{},
		&model.NotificationLog{},
		&model.MonitorCheck{},
		&model.MonitorMetric{},
		&model.Setting{},
		&model.HostSample{},
		&model.ScriptRun{},
		&model.AppLog{},
	); err != nil {
		return err
	}
	slog.Info("数据库迁移完成")
	return nil
}

func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
