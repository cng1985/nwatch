package settings

import "go.uber.org/fx"

// Module 提供可在运行时修改的系统配置。
var Module = fx.Module("settings",
	fx.Provide(New),
)
