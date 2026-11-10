package notifier

import "context"

// Sender 是一种通知渠道的发送适配。新增渠道时实现本接口并在 Registry 注册。
type Sender interface {
	Type() string
	Send(ctx context.Context, cfg *Notifier, event *Event) (response string, err error)
}

// 通知渠道类型。
const (
	NotifierDingTalk = "dingtalk"
	NotifierWeCom    = "wecom"
	NotifierWebhook  = "webhook"
)
