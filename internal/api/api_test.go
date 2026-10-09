package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/mailer/fakesmtp"

	"github.com/cng1985/nwatch/internal/alert"
	"github.com/cng1985/nwatch/internal/auth"
	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/host"
	"github.com/cng1985/nwatch/internal/logview"
	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/metric"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/notifier"
	"github.com/cng1985/nwatch/internal/scheduler"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/state"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(dir, "nmonitor.db")
	cfg.Security.Username = "admin"
	cfg.Security.Password = "test-pass"
	cfg.Security.JWTSecret = "test-secret"
	cfg.Backup.Enabled = false
	cfg.Backup.Dir = filepath.Join(dir, "backups")
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
	tokens, err := auth.New(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	collector := host.NewCollector()
	logs := logview.NewStore()
	logs.Bind(db)
	reg := checker.NewRegistry(
		checker.NewHTTPChecker(), checker.NewTLSChecker(), checker.NewTCPChecker(),
		checker.NewCPUChecker(collector), checker.NewMemoryChecker(collector), checker.NewDiskChecker(), checker.NewScriptChecker(),
	)
	alerts := alert.NewManager(db, notifier.NewRegistry(notifier.NewDingTalk(store), notifier.NewWeCom(store), notifier.NewWebhook()), nil)
	alerts.Start(1)
	t.Cleanup(func() { alerts.Stop(context.Background()) })
	mail := mailer.New(db, store, nil)
	mail.Start()
	t.Cleanup(func() { mail.Stop(context.Background()) })
	proc := scheduler.NewProcessor(db, reg, state.NewEngine(), alerts, metric.NewAggregator(), store, mail)
	pool := scheduler.NewPool(2, 8, proc, nil)
	pool.Start()
	t.Cleanup(func() { pool.Stop(context.Background()) })
	sched := scheduler.NewScheduler(time.Hour, db, pool, nil)
	sched.Start()
	t.Cleanup(sched.Stop)
	return NewServer(cfg, db, store, tokens, proc, pool, sched, alerts, mail, collector, logs, nil)
}

func TestLoginAndMonitorCheckFlow(t *testing.T) {
	s := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("UP"))
	}))
	defer upstream.Close()

	token := login(t, s)
	body := map[string]any{
		"name": "demo", "type": "http", "url": upstream.URL, "method": "GET",
		"interval": 30, "timeout": 3, "failureThreshold": 2, "recoveryThreshold": 1,
		"expectedStatusCodes": "200", "bodyContains": "UP",
	}
	rec := do(t, s, http.MethodPost, "/api/monitors", token, body)
	if rec.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data model.Monitor `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	rec = do(t, s, http.MethodPost, "/api/monitors/"+strconv.FormatUint(uint64(created.Data.ID), 10)+"/check", token, nil)
	if rec.Code != 200 {
		t.Fatalf("check %d %s", rec.Code, rec.Body.String())
	}
	var checked struct {
		Data struct {
			Monitor model.Monitor  `json:"monitor"`
			Result  checker.Result `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &checked); err != nil {
		t.Fatal(err)
	}
	if checked.Data.Monitor.Status != model.StatusUp || !checked.Data.Result.Success {
		t.Fatalf("expected up: %+v", checked.Data)
	}

	upstream.Close()
	rec = do(t, s, http.MethodPost, "/api/monitors/"+strconv.FormatUint(uint64(created.Data.ID), 10)+"/check", token, nil)
	rec = do(t, s, http.MethodPost, "/api/monitors/"+strconv.FormatUint(uint64(created.Data.ID), 10)+"/check", token, nil)
	if rec.Code != 200 {
		t.Fatalf("failing check %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &checked); err != nil {
		t.Fatal(err)
	}
	if checked.Data.Monitor.Status != model.StatusDown {
		t.Fatalf("expected down, got %s msg=%s", checked.Data.Monitor.Status, checked.Data.Result.Message)
	}

	dash := do(t, s, http.MethodGet, "/api/dashboard", token, nil)
	if dash.Code != 200 {
		t.Fatalf("dashboard %s", dash.Body.String())
	}
	unauth := do(t, s, http.MethodGet, "/api/monitors", "", nil)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("auth %d", unauth.Code)
	}
}

func TestNotifierMaskAndWebhook(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	var got []byte
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		got = buf.Bytes()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer hook.Close()
	rec := do(t, s, http.MethodPost, "/api/notifiers", token, map[string]any{
		"name": "hook", "type": "webhook", "webhookUrl": hook.URL, "enabled": true,
	})
	if rec.Code != 200 || bytes.Contains(rec.Body.Bytes(), []byte(hook.URL)) {
		t.Fatalf("mask failed %s", rec.Body.String())
	}
	var created struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	rec = do(t, s, http.MethodPost, "/api/notifiers/"+strconv.FormatUint(uint64(created.Data.ID), 10)+"/test", token, nil)
	if rec.Code != 200 {
		t.Fatalf("test %d %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(got, []byte("TEST")) {
		t.Fatalf("payload %s", got)
	}
}

func TestMailOnContinuedFailure(t *testing.T) {
	smtpSrv := fakesmtp.Start(t)
	host, portText, err := net.SplitHostPort(smtpSrv.Addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	s := newTestServer(t)
	token := login(t, s)
	const secret = "smtp-secret-xyz"
	rec := do(t, s, http.MethodPut, "/api/settings/mail", token, map[string]any{
		"enabled": true, "host": host, "port": port, "username": "mailer",
		"password": secret, "from": "nmonitor@example.com", "to": "ops@example.com, duty@example.com",
		"encryption": "none",
	})
	if rec.Code != 200 {
		t.Fatalf("save mail %d %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(secret)) {
		t.Fatalf("password leaked: %s", rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/settings", token, nil)
	if rec.Code != 200 || bytes.Contains(rec.Body.Bytes(), []byte(secret)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"passwordSet":true`)) {
		t.Fatalf("settings %d %s", rec.Code, rec.Body.String())
	}

	var failing atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	rec = do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
		"name": "邮件探针", "type": "http", "url": upstream.URL, "method": "GET",
		"interval": 30, "timeout": 3, "failureThreshold": 2, "recoveryThreshold": 1,
	})
	if rec.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data model.Monitor `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	check := "/api/monitors/" + strconv.FormatUint(uint64(created.Data.ID), 10) + "/check"
	if rec = do(t, s, http.MethodPost, check, token, nil); rec.Code != 200 {
		t.Fatalf("up check %d %s", rec.Code, rec.Body.String())
	}
	failing.Store(true)
	if rec = do(t, s, http.MethodPost, check, token, nil); rec.Code != 200 {
		t.Fatalf("first failure %d %s", rec.Code, rec.Body.String())
	}
	if len(smtpSrv.Messages()) != 0 {
		t.Fatalf("threshold not reached, got %d mails", len(smtpSrv.Messages()))
	}
	if rec = do(t, s, http.MethodPost, check, token, nil); rec.Code != 200 {
		t.Fatalf("second failure %d %s", rec.Code, rec.Body.String())
	}
	var downed struct {
		Data struct {
			Monitor model.Monitor `json:"monitor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &downed); err != nil {
		t.Fatal(err)
	}
	if downed.Data.Monitor.Status != model.StatusDown || downed.Data.Monitor.IncidentStartedAt == nil {
		t.Fatalf("expected incident, status=%s", downed.Data.Monitor.Status)
	}
	if len(smtpSrv.Messages()) != 0 {
		t.Fatalf("down should wait for the 15s notice, got %#v", smtpSrv.Messages())
	}
	if rec = do(t, s, http.MethodPost, check, token, nil); rec.Code != 200 {
		t.Fatalf("third failure %d %s", rec.Code, rec.Body.String())
	}
	if len(smtpSrv.Messages()) != 0 {
		t.Fatalf("continued failure should not mail on every check, got %#v", smtpSrv.Messages())
	}

	failing.Store(false)
	if rec = do(t, s, http.MethodPost, check, token, nil); rec.Code != 200 {
		t.Fatalf("recovery %d %s", rec.Code, rec.Body.String())
	}
	if len(smtpSrv.Messages()) != 1 || !strings.Contains(smtpSrv.Messages()[0], "服务已恢复") || !strings.Contains(smtpSrv.Messages()[0], "邮件探针") {
		t.Fatalf("recovery mail %#v", smtpSrv.Messages())
	}

	rec = do(t, s, http.MethodPost, "/api/settings/mail/test", token, map[string]any{
		"enabled": true, "host": host, "port": port, "from": "nmonitor@example.com", "to": "ops@example.com",
		"encryption": "none",
	})
	if rec.Code != 200 {
		t.Fatalf("test mail %d %s", rec.Code, rec.Body.String())
	}
	if len(smtpSrv.Messages()) != 2 || !strings.Contains(smtpSrv.Messages()[1], "测试邮件") {
		t.Fatalf("test body %#v", smtpSrv.Messages())
	}
}

func TestHostScriptAndLogs(t *testing.T) {
	s := newTestServer(t)
	s.logs.Add("INFO", "采集完成", "kind=host")
	token := login(t, s)

	rec := do(t, s, http.MethodGet, "/api/host?range=1h", token, nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("cpuPercent")) {
		t.Fatalf("host %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
		"name": "磁盘根分区", "type": "disk", "host": "/", "threshold": 99,
		"interval": 60, "timeout": 5, "failureThreshold": 1, "recoveryThreshold": 1,
	})
	if rec.Code != 200 {
		t.Fatalf("disk monitor %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodPost, "/api/scripts/run", token, map[string]any{
		"name": "探测", "command": "echo nmonitor-script-ok", "timeout": 5,
	})
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("nmonitor-script-ok")) {
		t.Fatalf("run %d %s", rec.Code, rec.Body.String())
	}
	var ran struct {
		Data model.ScriptRun `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ran); err != nil {
		t.Fatal(err)
	}
	if !ran.Data.Success || ran.Data.ExitCode != 0 {
		t.Fatalf("%+v", ran.Data)
	}

	rec = do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
		"name": "脚本监控", "type": "script", "command": "echo scheduled-ok",
		"interval": 60, "timeout": 5, "failureThreshold": 1, "recoveryThreshold": 1,
	})
	if rec.Code != 200 {
		t.Fatalf("script monitor %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data model.Monitor `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	rec = do(t, s, http.MethodPost, "/api/monitors/"+strconv.FormatUint(uint64(created.Data.ID), 10)+"/check", token, nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("scheduled-ok")) {
		t.Fatalf("check %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodGet, "/api/scripts/runs?keyword=scheduled-ok", token, nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("scheduled-ok")) {
		t.Fatalf("runs %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/logs?keyword=采集完成", token, nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("采集完成")) {
		t.Fatalf("logs %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/scripts/run", token, map[string]any{"command": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty script %d", rec.Code)
	}
}

func login(t *testing.T, s *Server) string {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/auth/login", "", map[string]string{"username": "admin", "password": "test-pass"})
	if rec.Code != 200 {
		t.Fatalf("login %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Data.Token
}

func do(t *testing.T, s *Server, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}
