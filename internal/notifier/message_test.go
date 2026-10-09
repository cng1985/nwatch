package notifier

import (
	"strings"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

func TestFailureHeadline(t *testing.T) {
	first := &model.AlertEvent{
		EventType: model.EventDown, OldStatus: model.StatusDown, NewStatus: model.StatusDown,
		MonitorName: "支付", Duration: 15, OccurredAt: time.Now(),
	}
	if titleOf(first) != "服务异常" || strings.Contains(markdown(first, time.UTC), "仍然异常") {
		t.Fatalf("first title %s", titleOf(first))
	}
	again := *first
	again.Duration = 45
	if titleOf(&again) != "服务仍然异常" || !strings.Contains(markdown(&again, time.UTC), "服务仍然异常") || !strings.Contains(markdown(&again, time.UTC), "故障持续") {
		t.Fatalf("ongoing title %s", titleOf(&again))
	}
	changed := &model.AlertEvent{
		EventType: model.EventDown, OldStatus: model.StatusUp, NewStatus: model.StatusDown, Duration: 90,
	}
	if titleOf(changed) != "服务异常" {
		t.Fatalf("transition title %s", titleOf(changed))
	}
}
