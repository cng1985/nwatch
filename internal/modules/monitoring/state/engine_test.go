package state

import (
	"github.com/cng1985/nwatch/internal/modules/monitoring/checker"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"testing"
	"time"
)

func TestFailureThresholdThenDown(t *testing.T) {
	m := &shared.Monitor{Status: shared.StatusUp, FailureThreshold: 3, RecoveryThreshold: 1, Type: shared.TypeHTTP}
	now := time.Now()
	for i := 1; i <= 2; i++ {
		d := NewEngine().Decide(m, &checker.Result{Success: false, Message: "timeout"}, now)
		if d.Status != shared.StatusUp || len(d.Events) != 0 || d.Failures != i {
			t.Fatalf("failure %d: %+v", i, d)
		}
		m.Status = d.Status
		m.ConsecutiveFailures = d.Failures
		m.ConsecutiveSuccesses = d.Successes
	}
	d := NewEngine().Decide(m, &checker.Result{Success: false, Message: "timeout"}, now)
	if d.Status != shared.StatusDown || len(d.Events) != 1 || d.Events[0].Type != shared.EventDown {
		t.Fatalf("expected down event, got %+v", d)
	}
	m.Status = d.Status
	m.ConsecutiveFailures = d.Failures
	again := NewEngine().Decide(m, &checker.Result{Success: false, Message: "timeout"}, now)
	if again.Status != shared.StatusDown || len(again.Events) != 0 {
		t.Fatalf("down should not renotify: %+v", again)
	}
}

func TestRecoveryThreshold(t *testing.T) {
	m := &shared.Monitor{Status: shared.StatusDown, FailureThreshold: 3, RecoveryThreshold: 2, Type: shared.TypeHTTP}
	now := time.Now()
	started := now.Add(-time.Minute)
	m.IncidentStartedAt = &started
	d := NewEngine().Decide(m, &checker.Result{Success: true, Message: "ok"}, now)
	if d.Status != shared.StatusDown || len(d.Events) != 0 {
		t.Fatalf("one success should stay down: %+v", d)
	}
	m.ConsecutiveSuccesses = d.Successes
	d = NewEngine().Decide(m, &checker.Result{Success: true, Message: "ok"}, now)
	if d.Status != shared.StatusUp || len(d.Events) != 1 || d.Events[0].Type != shared.EventRecovered {
		t.Fatalf("expected recovery, got %+v", d)
	}
	if !d.ClearIncident {
		t.Fatal("expected incident cleared")
	}
}

func TestUnknownBecomesUpWithoutRecoveryEvent(t *testing.T) {
	m := &shared.Monitor{Status: shared.StatusUnknown, FailureThreshold: 3, RecoveryThreshold: 1}
	d := NewEngine().Decide(m, &checker.Result{Success: true}, time.Now())
	if d.Status != shared.StatusUp || len(d.Events) != 0 {
		t.Fatalf("unexpected %+v", d)
	}
}

func TestSuccessResetsFailures(t *testing.T) {
	m := &shared.Monitor{Status: shared.StatusUp, FailureThreshold: 3, RecoveryThreshold: 1, ConsecutiveFailures: 2}
	d := NewEngine().Decide(m, &checker.Result{Success: true}, time.Now())
	if d.Failures != 0 || d.Status != shared.StatusUp {
		t.Fatalf("%+v", d)
	}
}

func TestTLSThresholdAlertsOnce(t *testing.T) {
	m := &shared.Monitor{
		Type: shared.TypeTLS, Status: shared.StatusUp, FailureThreshold: 3, RecoveryThreshold: 1,
		TLSWarningDays: 14, TLSCriticalDays: 7, TLSNotifyDays: "30,14,7,3,1,0",
	}
	info := &checker.TLSInfo{Status: shared.TLSWarning, DaysRemaining: 20, NotAfter: time.Now().AddDate(0, 0, 20)}
	d := NewEngine().Decide(m, &checker.Result{Success: true, TLS: info, Message: "warn"}, time.Now())
	if len(d.Events) != 1 || d.Events[0].Type != shared.EventTLSWarn || d.LastTLSNotified != "30" {
		t.Fatalf("first warning: %+v", d)
	}
	m.LastTLSNotified = d.LastTLSNotified
	m.TLSStatus = d.TLSStatus
	d = NewEngine().Decide(m, &checker.Result{Success: true, TLS: info}, time.Now())
	if len(d.Events) != 0 {
		t.Fatalf("repeated warning: %+v", d.Events)
	}
	info.DaysRemaining = 6
	info.Status = shared.TLSCritical
	d = NewEngine().Decide(m, &checker.Result{Success: true, TLS: info, Message: "critical"}, time.Now())
	if len(d.Events) != 1 || d.Events[0].Type != shared.EventTLSCrit || d.LastTLSNotified != "7" {
		t.Fatalf("critical: %+v", d)
	}
}

func TestTLSExpiredIsImmediateAndNotDuplicated(t *testing.T) {
	m := &shared.Monitor{Type: shared.TypeTLS, Status: shared.StatusUp, FailureThreshold: 3, RecoveryThreshold: 1, TLSNotifyDays: "30,14,7,0"}
	info := &checker.TLSInfo{Status: shared.TLSExpired, DaysRemaining: 0}
	d := NewEngine().Decide(m, &checker.Result{Success: false, Immediate: true, TLS: info, Message: "证书已过期"}, time.Now())
	if d.Status != shared.StatusDown {
		t.Fatalf("status %+v", d)
	}
	if len(d.Events) != 1 || d.Events[0].Type != shared.EventTLSExpire {
		t.Fatalf("events %+v", d.Events)
	}
}

func TestTLSRecovered(t *testing.T) {
	m := &shared.Monitor{Type: shared.TypeTLS, Status: shared.StatusUp, LastTLSNotified: "14", TLSStatus: shared.TLSWarning, FailureThreshold: 3, RecoveryThreshold: 1}
	info := &checker.TLSInfo{Status: shared.TLSNormal, DaysRemaining: 40}
	d := NewEngine().Decide(m, &checker.Result{Success: true, TLS: info}, time.Now())
	if len(d.Events) != 1 || d.Events[0].Type != shared.EventTLSRecover || d.LastTLSNotified != "" {
		t.Fatalf("%+v", d)
	}
}
