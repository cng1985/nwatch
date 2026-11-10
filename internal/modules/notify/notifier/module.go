package notifier

import (
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册通知渠道业务与发送适配。
// 发送适配（dingtalk/wecom/webhook）供告警引擎调用；渠道 CRUD 与测试是本模块业务。
var Module = fx.Module("notifier",
	fx.Provide(
		NewDingTalk,
		NewWeCom,
		NewWebhook,
		NewRegistry,
		NewRepository,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
)
