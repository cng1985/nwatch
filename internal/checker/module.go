package checker

import "go.uber.org/fx"

// Module 注册全部检测器。新增检测类型时在这里 Provide，并交给 NewRegistry。
var Module = fx.Module("checker",
	fx.Provide(
		NewHTTPChecker,
		NewTLSChecker,
		NewTCPChecker,
		NewRegistry,
	),
)
