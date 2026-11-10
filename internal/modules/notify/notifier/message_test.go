package notifier

import (
	"strings"
	"testing"
	"time"
)

func TestFailureHeadline(t *testing.T) {
	first := &Event{
		Kind: KindDown, OldStatus: "DOWN", NewStatus: "DOWN",
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
	changed := &Event{
		Kind: KindDown, OldStatus: "UP", NewStatus: "DOWN", Duration: 90,
	}
	if titleOf(changed) != "服务异常" {
		t.Fatalf("transition title %s", titleOf(changed))
	}
}
