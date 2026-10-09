package config

import "go.uber.org/fx"

// Module 提供进程配置。新增配置项放在本包，不要写进启动入口。
var Module = fx.Module("config",
	fx.Provide(Load),
)
