package notifier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cng1985/nwatch/internal/model"
)

type WebhookNotifier struct{}

func NewWebhook() *WebhookNotifier { return &WebhookNotifier{} }

func (n *WebhookNotifier) Type() string { return model.NotifierWebhook }

func (n *WebhookNotifier) Send(ctx context.Context, cfg *model.Notifier, event *model.AlertEvent) (string, error) {
	if cfg.WebhookURL == "" {
		return "", fmt.Errorf("Webhook 地址为空")
	}
	status := event.NewStatus
	if status == "" {
		status = event.OldStatus
	}
	payload := map[string]any{
		"eventType":    event.EventType,
		"monitorId":    event.MonitorID,
		"monitorName":  event.MonitorName,
		"monitorType":  event.MonitorType,
		"status":       status,
		"oldStatus":    event.OldStatus,
		"message":      event.Message,
		"target":       event.Target,
		"responseTime": event.ResponseTime,
		"occurredAt":   event.OccurredAt.Format(timeRFC3339),
	}
	if event.CertExpireAt != nil {
		payload["certificateExpireAt"] = event.CertExpireAt.Format(timeRFC3339)
	}
	if event.CertDaysLeft != nil {
		payload["certificateDaysRemaining"] = *event.CertDaysLeft
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return postJSON(ctx, cfg.WebhookURL, raw)
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
