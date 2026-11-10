package shell

import (
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册在线终端业务与会话管理。
var Module = fx.Module("shell",
	fx.Provide(
		newHub,
		NewRepository,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
	fx.Invoke(func(*Hub) {}),
)

// newHub 按配置构造终端会话中心。
func newHub(cfg *config.Config) *Hub {
	return NewHub(Options{
		Enabled:     cfg.Shell.Enabled,
		IdleTimeout: cfg.Shell.IdleTimeout,
		MaxLifetime: cfg.Shell.MaxLifetime,
		MaxSessions: cfg.Shell.MaxSessions,
	})
}
