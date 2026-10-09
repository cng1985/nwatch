package alert

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/mailer/fakesmtp"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/notifier"
	"github.com/cng1985/nwatch/internal/settings"
	"gorm.io/gorm"
)

func TestReminderCadence(t *testing.T) {
	smtpSrv := fakesmtp.Start(t)
	host, portText, err := net.SplitHostPort(smtpSrv.Addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)

	var mu sync.Mutex
	var hooks []string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		mu.Lock()
		hooks = append(hooks, string(buf))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer hook.Close()

	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "remind.db")
	cfg.Mail = config.MailConfig{
		Enabled: true, Host: host, Port: port, Username: "mailer", Password: "secret",
		From: "nmonitor@example.com", To: "ops@example.com", Encryption: "none",
	}
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
	mail := mailer.New(db, store, nil)
	alerts := NewManager(db, notifier.NewRegistry(notifier.NewDingTalk(store), notifier.NewWeCom(store), notifier.NewWebhook()), nil)
	alerts.Start(1)
	t.Cleanup(func() { alerts.Stop(context.Background()) })
	reminder := NewReminder(db, alerts, mail, nil)

	start := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	next := start.Add(15 * time.Second)
	mon := model.Monitor{
		Name: "支付", Type: model.TypeHTTP, URL: "https://pay.example", Method: "GET",
		Interval: 60, Timeout: 3, Enabled: true, Status: model.StatusDown,
		FailureThreshold: 1, RecoveryThreshold: 1,
		LastMessage: "connection timeout", ConsecutiveFailures: 3, LastResponseTime: 12,
		IncidentStartedAt: &start, NextNotifyAt: &next, NotifyStage: 0,
	}
	if err := db.Create(&mon).Error; err != nil {
		t.Fatal(err)
	}
	channel := model.Notifier{Name: "hook", Type: model.NotifierWebhook, WebhookURL: hook.URL, Enabled: true}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&mon).Association("Notifiers").Append(&channel); err != nil {
		t.Fatal(err)
	}

	reminder.Scan(start.Add(14 * time.Second))
	if mails, n := smtpSrv.Messages(), hookCount(&mu, &hooks); len(mails) != 0 || n != 0 {
		t.Fatalf("too early mails=%d hooks=%d", len(mails), n)
	}

	reminder.Scan(start.Add(15 * time.Second))
	waitHooks(t, &mu, &hooks, 1)
	mails := smtpSrv.Messages()
	if len(mails) != 1 || strings.Contains(mails[0], "仍然异常") || !strings.Contains(mails[0], "服务异常") || !strings.Contains(mails[0], "支付") {
		t.Fatalf("first notice %#v", mails)
	}
	assertStage(t, db, mon.ID, 1, start.Add(45*time.Second))

	reminder.Scan(start.Add(16 * time.Second))
	if len(smtpSrv.Messages()) != 1 || hookCount(&mu, &hooks) != 1 {
		t.Fatalf("duplicate after first notice mails=%d hooks=%d", len(smtpSrv.Messages()), hookCount(&mu, &hooks))
	}

	reminder.Scan(start.Add(45 * time.Second))
	waitHooks(t, &mu, &hooks, 2)
	mails = smtpSrv.Messages()
	if len(mails) != 2 || !strings.Contains(mails[1], "服务仍然异常") {
		t.Fatalf("second notice %#v", mails)
	}
	assertStage(t, db, mon.ID, 2, start.Add(105*time.Second))

	reminder.Scan(start.Add(105 * time.Second))
	waitHooks(t, &mu, &hooks, 3)
	if len(smtpSrv.Messages()) != 3 {
		t.Fatalf("third notice %#v", smtpSrv.Messages())
	}
	assertStage(t, db, mon.ID, 3, start.Add(165*time.Second))

	if err := db.Model(&model.Monitor{}).Where("id = ?", mon.ID).Updates(map[string]any{
		"status": model.StatusUp, "notify_stage": 0, "next_notify_at": gorm.Expr("NULL"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	reminder.Scan(start.Add(10 * time.Minute))
	if len(smtpSrv.Messages()) != 3 || hookCount(&mu, &hooks) != 3 {
		t.Fatalf("recovered still notified mails=%d hooks=%d", len(smtpSrv.Messages()), hookCount(&mu, &hooks))
	}
}

func TestReminderCatchUpAndBackfill(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "remind.db")
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
	reminder := NewReminder(db, nil, mailer.New(db, store, nil), nil)

	start := time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)
	late := model.Monitor{
		Name: "迟到", Type: model.TypeTCP, Host: "10.0.0.8", Port: 443,
		Interval: 60, Timeout: 3, Enabled: true, Status: model.StatusDown,
		FailureThreshold: 1, RecoveryThreshold: 1,
		IncidentStartedAt: &start, NotifyStage: 0,
	}
	if err := db.Create(&late).Error; err != nil {
		t.Fatal(err)
	}
	reminder.Scan(start.Add(200 * time.Second))
	assertStage(t, db, late.ID, 4, start.Add(225*time.Second))
	reminder.Scan(start.Add(201 * time.Second))
	assertStage(t, db, late.ID, 4, start.Add(225*time.Second))
	if err := db.Model(&model.Monitor{}).Where("id = ?", late.ID).Update("status", model.StatusUp).Error; err != nil {
		t.Fatal(err)
	}

	earlyStart := start.Add(time.Hour)
	early := model.Monitor{
		Name: "旧数据", Type: model.TypeHTTP, URL: "https://old.example",
		Interval: 60, Timeout: 3, Enabled: true, Status: model.StatusDown,
		FailureThreshold: 1, RecoveryThreshold: 1,
		IncidentStartedAt: &earlyStart,
	}
	if err := db.Create(&early).Error; err != nil {
		t.Fatal(err)
	}
	reminder.Scan(earlyStart.Add(5 * time.Second))
	assertStage(t, db, early.ID, 0, earlyStart.Add(15*time.Second))
}

func assertStage(t *testing.T, db *gorm.DB, id uint, stage int, next time.Time) {
	t.Helper()
	var fresh model.Monitor
	if err := db.First(&fresh, id).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.NotifyStage != stage || fresh.NextNotifyAt == nil || !fresh.NextNotifyAt.Equal(next) {
		got := "<nil>"
		if fresh.NextNotifyAt != nil {
			got = fresh.NextNotifyAt.Format(time.RFC3339Nano)
		}
		t.Fatalf("stage=%d next=%s want stage=%d next=%s", fresh.NotifyStage, got, stage, next.Format(time.RFC3339Nano))
	}
}

func hookCount(mu *sync.Mutex, hooks *[]string) int {
	mu.Lock()
	defer mu.Unlock()
	return len(*hooks)
}

func waitHooks(t *testing.T, mu *sync.Mutex, hooks *[]string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hookCount(mu, hooks) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("hooks=%d want %d", hookCount(mu, hooks), want)
}
