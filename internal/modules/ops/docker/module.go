package docker

import (
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册 Docker 控制业务。
var Module = fx.Module("docker",
	fx.Provide(
		newManager,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
)

func newManager(cfg *config.Config) *Manager {
	return New(cfg.Docker.Enabled)
}
