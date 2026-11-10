package backup

import (
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册备份与配置导入导出业务。
var Module = fx.Module("backup",
	fx.Provide(
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
)
