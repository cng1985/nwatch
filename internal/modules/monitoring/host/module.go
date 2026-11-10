package host

import (
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 采集本机 CPU、内存、磁盘和进程，并注册主机资源查询路由。
var Module = fx.Module("host",
	fx.Provide(
		NewCollector,
		NewSampler,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
	fx.Invoke(func(*Sampler) {}),
)
