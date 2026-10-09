package state

import "go.uber.org/fx"

// Module 提供监控状态机。
var Module = fx.Module("state",
	fx.Provide(NewEngine),
)
