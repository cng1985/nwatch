package api

import "go.uber.org/fx"

// Module 提供 HTTP 服务。Invoke 触发生命周期，从而把依赖图里的后台组件一起建出来。
var Module = fx.Module("api",
	fx.Provide(NewServer),
	fx.Invoke(func(*Server) {}),
)
