package notifier

import "fmt"

// Registry 按渠道类型查找发送适配。
type Registry struct {
	items map[string]Sender
}

func NewRegistry(ding *DingTalkNotifier, wecom *WeComNotifier, hook *WebhookNotifier) *Registry {
	r := &Registry{items: map[string]Sender{}}
	for _, n := range []Sender{ding, wecom, hook} {
		r.items[n.Type()] = n
	}
	return r
}

func (r *Registry) Get(kind string) (Sender, error) {
	n, ok := r.items[kind]
	if !ok {
		return nil, fmt.Errorf("未注册的通知类型 %s", kind)
	}
	return n, nil
}
