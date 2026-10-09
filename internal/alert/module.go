package alert

import "go.uber.org/fx"

// Module 提供异步通知队列，以及故障期间的定时提醒。
var Module = fx.Module("alert",
	fx.Provide(NewManager, NewReminder),
	fx.Invoke(func(*Manager) {}),
	fx.Invoke(func(*Reminder) {}),
)
