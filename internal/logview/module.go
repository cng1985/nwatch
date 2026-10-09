package logview

import "go.uber.org/fx"

// Module 提供可查询的进程日志。
var Module = fx.Module("logview",
	fx.Provide(NewStore, BindDB),
	fx.Invoke(func(Bound) {}),
)
