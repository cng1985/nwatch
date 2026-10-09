package database

import (
	"context"
	"log/slog"

	"github.com/cng1985/nwatch/internal/config"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Module 打开数据库并在进程退出时关闭。
var Module = fx.Module("database",
	fx.Provide(provide),
)

func provide(cfg *config.Config, _ *slog.Logger, lc fx.Lifecycle) (*gorm.DB, error) {
	db, err := Open(cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	if err := Migrate(db); err != nil {
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
