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
		_, _ = io.ReadAll(r.Body)
		mu.Lock()
		hooks = append(hooks, r.URL.Path)
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
	bound := model.Notifier{Name: "bound", Type: model.NotifierWebhook, WebhookURL: hook.URL + "/bound", Enabled: true}
	free := model.Notifier{Name: "free", Type: model.NotifierWebhook, WebhookURL: hook.URL + "/free", Enabled: true}
	off := model.Notifier{Name: "off", Type: model.NotifierWebhook, WebhookURL: hook.URL + "/off", Enabled: false}
	if err := db.Create(&bound).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&free).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&off).Error; err != nil {
		t.Fatal(err)
	}
	// 只绑定其中一个，未绑定的已启用渠道也必须收到。
	if err := db.Model(&mon).Association("Notifiers").Append(&bound); err != nil {
		t.Fatal(err)
	}

	reminder.Scan(start.Add(14 * time.Second))
	if mails, n := smtpSrv.Messages(), hookCount(&mu, &hooks); len(mails) != 0 || n != 0 {
		t.Fatalf("too early mails=%d hooks=%d", len(mails), n)
	}

	reminder.Scan(start.Add(15 * time.Second))
	waitHooks(t, &mu, &hooks, 2)
	assertChannels(t, &mu, &hooks, 1)
	mails := smtpSrv.Messages()
	if len(mails) != 1 || strings.Contains(mails[0], "仍然异常") || !strings.Contains(mails[0], "服务异常") || !strings.Contains(mails[0], "支付") {
		t.Fatalf("first notice %#v", mails)
	}
	assertStage(t, db, mon.ID, 1, start.Add(45*time.Second))

	reminder.Scan(start.Add(16 * time.Second))
	if len(smtpSrv.Messages()) != 1 || hookCount(&mu, &hooks) != 2 {
		t.Fatalf("duplicate after first notice mails=%d hooks=%d", len(smtpSrv.Messages()), hookCount(&mu, &hooks))
	}

	reminder.Scan(start.Add(45 * time.Second))
	waitHooks(t, &mu, &hooks, 4)
	assertChannels(t, &mu, &hooks, 2)
	mails = smtpSrv.Messages()
	if len(mails) != 2 || !strings.Contains(mails[1], "服务仍然异常") {
		t.Fatalf("second notice %#v", mails)
	}
	assertStage(t, db, mon.ID, 2, start.Add(105*time.Second))

	reminder.Scan(start.Add(105 * time.Second))
	waitHooks(t, &mu, &hooks, 6)
	assertChannels(t, &mu, &hooks, 3)
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
	if len(smtpSrv.Messages()) != 3 || hookCount(&mu, &hooks) != 6 {
		t.Fatalf("recovered still notified mails=%d hooks=%d", len(smtpSrv.Messages()), hookCount(&mu, &hooks))
	}
	assertChannels(t, &mu, &hooks, 3)
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

func TestReminderLegacyDownNotifiesWeCom(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer hook.Close()

	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "legacy.db")
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
	alerts := NewManager(db, notifier.NewRegistry(notifier.NewDingTalk(store), notifier.NewWeCom(store), notifier.NewWebhook()), nil)
	alerts.Start(1)
	t.Cleanup(func() { alerts.Stop(context.Background()) })
	reminder := NewReminder(db, alerts, nil, nil)

	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 10, 9, 15, 30, 0, 0, loc)
	oldStart := now.Add(-3 * time.Minute)
	recentFail := now.Add(-5 * time.Second)

	old := model.Monitor{
		Name: "失败测试", Type: model.TypeHTTP, URL: "https://api.example.com/a", Method: "GET",
		Interval: 60, Timeout: 5, Enabled: true, Status: model.StatusDown,
		FailureThreshold: 3, RecoveryThreshold: 1,
		LastMessage: "HTTP 状态码 404，期望 200", ConsecutiveFailures: 8,
		IncidentStartedAt: &oldStart, NotifyStage: 0,
	}
	missing := model.Monitor{
		Name: "demo", Type: model.TypeHTTP, URL: "https://api.example.com/demo", Method: "GET",
		Interval: 60, Timeout: 5, Enabled: true, Status: model.StatusDown,
		FailureThreshold: 3, RecoveryThreshold: 1,
		LastMessage: "HTTP 状态码 404，期望 200", ConsecutiveFailures: 6,
		LastFailureAt: &oldStart,
	}
	fresh := model.Monitor{
		Name: "刚失败", Type: model.TypeHTTP, URL: "https://api.example.com/new", Method: "GET",
		Interval: 60, Timeout: 5, Enabled: true, Status: model.StatusDown,
		FailureThreshold: 3, RecoveryThreshold: 1,
		LastMessage: "HTTP 状态码 404，期望 200",
		LastFailureAt: &recentFail,
	}
	for _, row := range []*model.Monitor{&old, &missing, &fresh} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 升级前已经异常的行，新列是 NULL，不是 0。监控也没有绑定任何渠道。
	ids := []uint{old.ID, missing.ID, fresh.ID}
	if err := db.Model(&model.Monitor{}).Where("id IN ?", ids).Updates(map[string]any{
		"notify_stage":   gorm.Expr("NULL"),
		"next_notify_at": gorm.Expr("NULL"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Monitor{}).Where("id IN ?", []uint{missing.ID, fresh.ID}).Update("incident_started_at", gorm.Expr("NULL")).Error; err != nil {
		t.Fatal(err)
	}
	wecom := model.Notifier{Name: "企业微信", Type: model.NotifierWeCom, WebhookURL: hook.URL, Enabled: true}
	if err := db.Create(&wecom).Error; err != nil {
		t.Fatal(err)
	}

	reminder.Scan(now)
	waitHooks(t, &mu, &bodies, 2)
	mu.Lock()
	got := strings.Join(bodies, "\n")
	mu.Unlock()
	if !strings.Contains(got, "失败测试") || !strings.Contains(got, "demo") || strings.Contains(got, "刚失败") {
		t.Fatalf("bodies=%s", got)
	}
	if strings.Contains(got, "通知测试") {
		t.Fatalf("sent a test message: %s", got)
	}
	assertStage(t, db, fresh.ID, 0, recentFail.Add(15*time.Second))

	reminder.Scan(now.Add(time.Second))
	if hookCount(&mu, &bodies) != 2 {
		t.Fatalf("duplicate sends: %d", hookCount(&mu, &bodies))
	}
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

func assertChannels(t *testing.T, mu *sync.Mutex, hooks *[]string, each int) {
	t.Helper()
	got := map[string]int{}
	mu.Lock()
	for _, path := range *hooks {
		got[path]++
	}
	mu.Unlock()
	if got["/bound"] != each || got["/free"] != each || got["/off"] != 0 {
		t.Fatalf("channels %#v want bound=%d free=%d off=0", got, each, each)
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
