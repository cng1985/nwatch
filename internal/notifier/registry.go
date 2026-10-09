package notifier

import "fmt"

type Registry struct {
	items map[string]Notifier
}

func NewRegistry(ding *DingTalkNotifier, wecom *WeComNotifier, hook *WebhookNotifier) *Registry {
	r := &Registry{items: map[string]Notifier{}}
	for _, n := range []Notifier{ding, wecom, hook} {
		r.items[n.Type()] = n
	}
	return r
}

func (r *Registry) Get(kind string) (Notifier, error) {
	n, ok := r.items[kind]
	if !ok {
		return nil, fmt.Errorf("未注册的通知类型 %s", kind)
	}
	return n, nil
}
