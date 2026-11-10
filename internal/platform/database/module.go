package database

import (
	"context"
	"log/slog"

	"github.com/cng1985/nwatch/internal/platform/config"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Module 打开数据库并在进程退出时关闭。
// 构造函数不执行迁移；受控迁移由 app/bootstrap 统一编排。
var Module = fx.Module("database",
	fx.Provide(provideDB),
)

func provideDB(cfg *config.Config, lc fx.Lifecycle) (*gorm.DB, error) {
	db, err := Open(OptionsFromConfig(cfg))
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			slog.Info("关闭数据库")
			return Close(db)
		},
	})
	return db, nil
}
