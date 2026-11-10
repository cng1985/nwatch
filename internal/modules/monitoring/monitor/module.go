package monitor

import (
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册监控项业务：CRUD、启停、手动检测、检测记录、事件、指标、证书与表格导入导出。
var Module = fx.Module("monitor",
	fx.Provide(
		NewRepository,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
)
