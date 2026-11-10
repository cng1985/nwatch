package scheduler

import (
	"context"
	"github.com/cng1985/nwatch/internal/modules/monitoring/alert"
	"github.com/cng1985/nwatch/internal/modules/monitoring/checker"
	"github.com/cng1985/nwatch/internal/modules/monitoring/host"
	"github.com/cng1985/nwatch/internal/modules/monitoring/metric"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/modules/monitoring/state"
	"github.com/cng1985/nwatch/internal/modules/notify/mailer"
	"github.com/cng1985/nwatch/internal/modules/notify/notifier"
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"github.com/cng1985/nwatch/tests/harness"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestDownSchedulesNotifyUntilRecovery(t *testing.T) {
	var failing atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "sched.db")
	db := harness.OpenDB(t, cfg)
	store, err := settings.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	collector := host.NewCollector()
	reg := checker.NewRegistry(
		checker.NewHTTPChecker(), checker.NewTLSChecker(), checker.NewTCPChecker(),
		checker.NewCPUChecker(collector), checker.NewMemoryChecker(collector), checker.NewDiskChecker(), checker.NewScriptChecker(), checker.NewProcessChecker(collector),
	)
	alerts := alert.NewManager(db, notifier.NewRegistry(notifier.NewDingTalk(store), notifier.NewWeCom(store), notifier.NewWebhook()), nil)
	proc := NewProcessor(db, reg, state.NewEngine(), alerts, metric.NewAggregator(), store, mailer.New(db, store, nil))

	mon := shared.Monitor{
		Name: "接口", Type: shared.TypeHTTP, URL: upstream.URL, Method: "GET",
		Interval: 30, Timeout: 3, Enabled: true, Status: shared.StatusUnknown,
		FailureThreshold: 2, RecoveryThreshold: 1,
	}
	if err := db.Create(&mon).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := proc.Process(ctx, mon.ID, false); err != nil {
		t.Fatal(err)
	}
	failing.Store(true)
	if _, err := proc.Process(ctx, mon.ID, false); err != nil {
		t.Fatal(err)
	}
	var mid shared.Monitor
	if err := db.First(&mid, mon.ID).Error; err != nil {
		t.Fatal(err)
	}
	if mid.Status == shared.StatusDown || mid.NextNotifyAt != nil {
		t.Fatalf("below threshold status=%s next=%v", mid.Status, mid.NextNotifyAt)
	}

	out, err := proc.Process(ctx, mon.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Monitor.Status != shared.StatusDown || len(out.Events) != 1 || out.Events[0].EventType != shared.EventDown {
		t.Fatalf("expected down event, status=%s events=%d", out.Monitor.Status, len(out.Events))
	}
	if out.Monitor.IncidentStartedAt == nil || out.Monitor.NextNotifyAt == nil || out.Monitor.NotifyStage != 0 {
		t.Fatalf("schedule missing: incident=%v next=%v stage=%d", out.Monitor.IncidentStartedAt, out.Monitor.NextNotifyAt, out.Monitor.NotifyStage)
	}
	want := out.Monitor.IncidentStartedAt.Add(alert.DueOffset(0))
	if !out.Monitor.NextNotifyAt.Equal(want) {
		t.Fatalf("next %s want %s", out.Monitor.NextNotifyAt, want)
	}
	scheduled := *out.Monitor.NextNotifyAt

	again, err := proc.Process(ctx, mon.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Events) != 0 || again.Monitor.NextNotifyAt == nil || !again.Monitor.NextNotifyAt.Equal(scheduled) || again.Monitor.NotifyStage != 0 {
		t.Fatalf("continued failure changed schedule: events=%d next=%v stage=%d", len(again.Events), again.Monitor.NextNotifyAt, again.Monitor.NotifyStage)
	}

	failing.Store(false)
	recovered, err := proc.Process(ctx, mon.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Monitor.Status != shared.StatusUp || recovered.Monitor.NextNotifyAt != nil || recovered.Monitor.NotifyStage != 0 || recovered.Monitor.IncidentStartedAt != nil {
		t.Fatalf("recovery did not clear schedule: %+v", recovered.Monitor)
	}
	if len(recovered.Events) != 1 || recovered.Events[0].EventType != shared.EventRecovered {
		t.Fatalf("recovery events %+v", recovered.Events)
	}
}
