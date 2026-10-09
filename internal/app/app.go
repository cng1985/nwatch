package app

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/cng1985/nwatch/internal/alert"
	"github.com/cng1985/nwatch/internal/api"
	"github.com/cng1985/nwatch/internal/auth"
	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/maintenance"
	"github.com/cng1985/nwatch/internal/metric"
	"github.com/cng1985/nwatch/internal/notifier"
	"github.com/cng1985/nwatch/internal/scheduler"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/state"
	"github.com/cng1985/nwatch/internal/version"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

func New() *fx.App {
	return fx.New(
		fx.NopLogger,
		fx.Provide(
			config.Load,
			newLogger,
			newDB,
			settings.New,
			checker.NewHTTPChecker,
			checker.NewTLSChecker,
			checker.NewTCPChecker,
			checker.NewRegistry,
			state.NewEngine,
			notifier.NewDingTalk,
			notifier.NewWeCom,
			notifier.NewWebhook,
			notifier.NewRegistry,
			alert.NewManager,
			metric.NewAggregator,
			scheduler.NewProcessor,
			newPool,
			newScheduler,
			maintenance.New,
			auth.New,
			api.NewServer,
		),
		fx.Invoke(logReady, startRuntime),
	)
}

// startRuntime forces the long-running components to be constructed so their lifecycle hooks run.
func startRuntime(*api.Server, *scheduler.Scheduler, *scheduler.Pool, *alert.Manager, *maintenance.Service) {
}

func newLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Log.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return logger
}

func newDB(cfg *config.Config, _ *slog.Logger, lc fx.Lifecycle) (*gorm.DB, error) {
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(db); err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			slog.Info("关闭数据库")
			return database.Close(db)
		},
	})
	return db, nil
}

func newPool(cfg *config.Config, proc *scheduler.Processor, lc fx.Lifecycle) *scheduler.Pool {
	return scheduler.NewPool(cfg.Scheduler.WorkerCount, cfg.Scheduler.QueueSize, proc, lc)
}

func newScheduler(cfg *config.Config, db *gorm.DB, pool *scheduler.Pool, lc fx.Lifecycle) *scheduler.Scheduler {
	return scheduler.NewScheduler(cfg.Scheduler.ScanInterval, db, pool, lc)
}

func logReady(cfg *config.Config, store *settings.Store) {
	slog.Info("NMonitor 启动", "version", version.Version, "addr", cfg.Addr(), "database", cfg.Database.Path, "workers", cfg.Scheduler.WorkerCount)
	if store.UsingDefaultPassword() {
		slog.Warn("正在使用默认管理员密码 admin，请尽快在系统配置中修改，或通过环境变量 NMONITOR_PASSWORD 设置")
	}
}
