package alert

import "go.uber.org/fx"

// Module 提供异步通知队列。
var Module = fx.Module("alert",
	fx.Provide(NewManager),
	fx.Invoke(func(*Manager) {}),
)
