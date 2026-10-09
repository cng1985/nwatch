package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/cng1985/nwatch/internal/alert"
	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/host"
	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/metric"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/notifier"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/state"
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
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store, err := settings.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	collector := host.NewCollector()
	reg := checker.NewRegistry(
		checker.NewHTTPChecker(), checker.NewTLSChecker(), checker.NewTCPChecker(),
		checker.NewCPUChecker(collector), checker.NewMemoryChecker(collector), checker.NewDiskChecker(), checker.NewScriptChecker(),
	)
	alerts := alert.NewManager(db, notifier.NewRegistry(notifier.NewDingTalk(store), notifier.NewWeCom(store), notifier.NewWebhook()), nil)
	proc := NewProcessor(db, reg, state.NewEngine(), alerts, metric.NewAggregator(), store, mailer.New(db, store, nil))

	mon := model.Monitor{
		Name: "接口", Type: model.TypeHTTP, URL: upstream.URL, Method: "GET",
		Interval: 30, Timeout: 3, Enabled: true, Status: model.StatusUnknown,
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
	var mid model.Monitor
	if err := db.First(&mid, mon.ID).Error; err != nil {
		t.Fatal(err)
	}
	if mid.Status == model.StatusDown || mid.NextNotifyAt != nil {
		t.Fatalf("below threshold status=%s next=%v", mid.Status, mid.NextNotifyAt)
	}

	out, err := proc.Process(ctx, mon.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Monitor.Status != model.StatusDown || len(out.Events) != 1 || out.Events[0].EventType != model.EventDown {
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
	if recovered.Monitor.Status != model.StatusUp || recovered.Monitor.NextNotifyAt != nil || recovered.Monitor.NotifyStage != 0 || recovered.Monitor.IncidentStartedAt != nil {
		t.Fatalf("recovery did not clear schedule: %+v", recovered.Monitor)
	}
	if len(recovered.Events) != 1 || recovered.Events[0].EventType != model.EventRecovered {
		t.Fatalf("recovery events %+v", recovered.Events)
	}
}
