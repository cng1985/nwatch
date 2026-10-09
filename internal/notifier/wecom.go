package notifier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
)

type WeComNotifier struct {
	settings *settings.Store
}

func NewWeCom(store *settings.Store) *WeComNotifier {
	return &WeComNotifier{settings: store}
}

func (n *WeComNotifier) Type() string { return model.NotifierWeCom }

func (n *WeComNotifier) Send(ctx context.Context, cfg *model.Notifier, event *model.AlertEvent) (string, error) {
	if cfg.WebhookURL == "" {
		return "", fmt.Errorf("企业微信 Webhook 为空")
	}
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": markdown(event, n.settings.Location()),
		},
	}
	raw, _ := json.Marshal(payload)
	body, err := postJSON(ctx, cfg.WebhookURL, raw)
	if err != nil {
		return body, err
	}
	var parsed struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if json.Unmarshal([]byte(body), &parsed) == nil && parsed.ErrCode != 0 {
		return body, fmt.Errorf("企业微信错误 %d: %s", parsed.ErrCode, parsed.ErrMsg)
	}
	return body, nil
}
