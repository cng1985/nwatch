package mailer

import "go.uber.org/fx"

// Module 提供邮件发送。Invoke 保证即使没有其他组件依赖它，后台重试也会启动。
var Module = fx.Module("mailer",
	fx.Provide(New),
	fx.Invoke(func(*Service) {}),
)
