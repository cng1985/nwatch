package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/alert"
	"github.com/cng1985/nwatch/internal/auth"
	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
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
	reg := checker.NewRegistry(checker.NewHTTPChecker(), checker.NewTLSChecker(), checker.NewTCPChecker())
	alerts := alert.NewManager(db, notifier.NewRegistry(notifier.NewDingTalk(store), notifier.NewWeCom(store), notifier.NewWebhook()), nil)
	alerts.Start(1)
	t.Cleanup(func() { alerts.Stop(context.Background()) })
	proc := scheduler.NewProcessor(db, reg, state.NewEngine(), alerts, metric.NewAggregator(), store)
	pool := scheduler.NewPool(2, 8, proc, nil)
	pool.Start()
	t.Cleanup(func() { pool.Stop(context.Background()) })
	sched := scheduler.NewScheduler(time.Hour, db, pool, nil)
	sched.Start()
	t.Cleanup(sched.Stop)
	return NewServer(cfg, db, store, tokens, proc, pool, sched, alerts, nil)
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
