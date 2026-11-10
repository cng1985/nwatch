package logging

import (
	"log/slog"
	"strings"

	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"github.com/cng1985/nwatch/internal/platform/version"
	"go.uber.org/fx"
)

// Module 提供结构化日志：标准输出 + 可查询的数据库记录。
var Module = fx.Module("logging",
	fx.Provide(NewStore, BindDB, NewLogger),
	fx.Invoke(func(Bound) {}),
	fx.Invoke(logReady),
)

// NewLogger 按配置装配默认 slog.Logger。
func NewLogger(cfg *config.Config, logs *Store) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(logs.Handler(level))
	slog.SetDefault(logger)
	return logger
}

func logReady(cfg *config.Config, store *settings.Store, _ Bound) {
	slog.Info("NWatch 启动", "version", version.Version, "addr", cfg.Addr(), "database", cfg.Database.Path, "workers", cfg.Scheduler.WorkerCount)
	if store.UsingDefaultPassword() {
		slog.Warn("正在使用默认管理员密码 admin，请尽快在系统配置中修改，或通过环境变量设置")
	}
}
