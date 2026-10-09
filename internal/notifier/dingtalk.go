package notifier

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
)

type DingTalkNotifier struct {
	settings *settings.Store
}

func NewDingTalk(store *settings.Store) *DingTalkNotifier {
	return &DingTalkNotifier{settings: store}
}

func (n *DingTalkNotifier) Type() string { return model.NotifierDingTalk }

func (n *DingTalkNotifier) Send(ctx context.Context, cfg *model.Notifier, event *model.AlertEvent) (string, error) {
	endpoint, err := SignDingTalk(cfg.WebhookURL, cfg.Secret, time.Now())
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": titleOf(event),
			"text":  markdown(event, n.settings.Location()),
		},
	}
	raw, _ := json.Marshal(payload)
	body, err := postJSON(ctx, endpoint, raw)
	if err != nil {
		return body, err
	}
	var parsed struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if json.Unmarshal([]byte(body), &parsed) == nil && parsed.ErrCode != 0 {
		return body, fmt.Errorf("钉钉错误 %d: %s", parsed.ErrCode, parsed.ErrMsg)
	}
	return body, nil
}

func SignDingTalk(webhook, secret string, now time.Time) (string, error) {
	if webhook == "" {
		return "", fmt.Errorf("钉钉 Webhook 为空")
	}
	if secret == "" {
		return webhook, nil
	}
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts + "\n" + secret))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	sep := "?"
	if u, err := url.Parse(webhook); err == nil && u.RawQuery != "" {
		sep = "&"
	}
	return webhook + sep + "timestamp=" + ts + "&sign=" + sign, nil
}
