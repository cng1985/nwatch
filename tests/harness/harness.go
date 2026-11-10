// Package harness 为跨模块测试提供数据库与配置装配。
// 测试环境隔离，不指向生产数据。
package harness

import (
	"path/filepath"
	"testing"

	"github.com/cng1985/nwatch/internal/modules/migrations"
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/database"
	"gorm.io/gorm"
)

// NewConfig 返回使用临时数据目录的测试配置。
func NewConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.App.DataDir = dir
	cfg.Database.Path = filepath.Join(dir, "nwatch-test.db")
	cfg.Security.Username = "admin"
	cfg.Security.Password = "test-pass"
	cfg.Security.JWTSecret = "test-secret"
	cfg.Backup.Enabled = false
	cfg.Backup.Dir = filepath.Join(dir, "backups")
	return cfg
}

// OpenDB 打开临时数据库并完成迁移，测试结束后自动关闭。
func OpenDB(t *testing.T, cfg *config.Config) *gorm.DB {
	t.Helper()
	db, err := database.Open(database.OptionsFromConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(migrations.Models()...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	return db
}
