package script

import (
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册手动脚本执行业务。
var Module = fx.Module("script",
	fx.Provide(
		NewRepository,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
)
