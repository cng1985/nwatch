package alert

import (
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/modules/notify/notifier"
)

// EventView 把监控域的告警事件转换成通知域的事件视图，保持分组间依赖单向。
func EventView(event *shared.AlertEvent) *notifier.Event {
	if event == nil {
		return nil
	}
	view := &notifier.Event{
		EventType:    event.EventType,
		MonitorID:    event.MonitorID,
		MonitorName:  event.MonitorName,
		MonitorType:  event.MonitorType,
		OldStatus:    event.OldStatus,
		NewStatus:    event.NewStatus,
		Target:       event.Target,
		Message:      event.Message,
		ResponseTime: event.ResponseTime,
		OccurredAt:   event.OccurredAt,
		RecoveredAt:  event.RecoveredAt,
		Duration:     event.Duration,
		CertExpireAt: event.CertExpireAt,
		CertDaysLeft: event.CertDaysLeft,
	}
	switch event.EventType {
	case "TEST":
		view.Kind = notifier.KindTest
	case shared.EventDown:
		view.Kind = notifier.KindDown
	case shared.EventRecovered:
		view.Kind = notifier.KindRecover
	case shared.EventTLSWarn:
		view.Kind = notifier.KindTLSWarn
	case shared.EventTLSCrit:
		view.Kind = notifier.KindTLSCrit
	case shared.EventTLSExpire:
		view.Kind = notifier.KindTLSExpire
	case shared.EventTLSInvalid:
		view.Kind = notifier.KindTLSInvalid
	case shared.EventTLSRecover:
		view.Kind = notifier.KindTLSRecover
	default:
		view.Kind = event.EventType
	}
	return view
}
