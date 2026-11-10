// Package app 是组合根：配置、日志、Fx 应用启动与退出。
package app

import (
	"log/slog"

	"github.com/cng1985/nwatch/internal/modules/migrations"
	"github.com/cng1985/nwatch/internal/platform/auth"
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// bootstrapModule 编排受控迁移与幂等初始化。
// Migrate 必须是整个应用最先执行的 Invoke；Initialize 随后触发需要表结构的构造。
var bootstrapModule = fx.Module("bootstrap",
	fx.Invoke(Migrate),
	fx.Invoke(Initialize),
)

// Migrate 按模型清单执行受控迁移。
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(migrations.Models()...); err != nil {
		return err
	}
	slog.Info("数据库迁移完成")
	return nil
}

// Initialize 幂等初始化：内置角色与管理员账号。更新不重置已有数据。
func Initialize(cfg *config.Config, db *gorm.DB, store *settings.Store, users *auth.Store) error {
	if err := users.Ensure(cfg, store); err != nil {
		return err
	}
	slog.Info("初始化完成", "admin", auth.ConfigUsername(cfg))
	return nil
}
