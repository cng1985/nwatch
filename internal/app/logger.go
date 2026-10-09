package app

import (
	"log/slog"
	"os"
	"strings"

	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/version"
	"go.uber.org/fx"
)

var loggerModule = fx.Module("logger",
	fx.Provide(newLogger),
	fx.Invoke(logReady),
)

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

func logReady(cfg *config.Config, store *settings.Store) {
	slog.Info("NMonitor 启动", "version", version.Version, "addr", cfg.Addr(), "database", cfg.Database.Path, "workers", cfg.Scheduler.WorkerCount)
	if store.UsingDefaultPassword() {
		slog.Warn("正在使用默认管理员密码 admin，请尽快在系统配置中修改，或通过环境变量 NMONITOR_PASSWORD 设置")
	}
}
