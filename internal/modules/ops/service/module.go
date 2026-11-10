package service

import (
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册系统服务控制业务。
var Module = fx.Module("service",
	fx.Provide(
		newManager,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
)

func newManager(cfg *config.Config) *Manager {
	return New(cfg.Service.Enabled)
}
