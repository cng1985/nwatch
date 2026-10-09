package notifier

import "go.uber.org/fx"

// Module 注册通知渠道。新增渠道时在这里 Provide，并交给 NewRegistry。
var Module = fx.Module("notifier",
	fx.Provide(
		NewDingTalk,
		NewWeCom,
		NewWebhook,
		NewRegistry,
	),
)
