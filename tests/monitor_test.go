package tests

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/modules/notify/mailer/fakesmtp"
)

// login 用默认管理员登录并返回令牌。
func login(t *testing.T, s *testServer) string {
	t.Helper()
	return loginAs(t, s, "admin", "test-pass")
}

func loginAs(t *testing.T, s *testServer, username, password string) string {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/auth/login", "", map[string]string{"username": username, "password": password})
	var body struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("login parse %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	if body.Code != 200 || body.Data.Token == "" {
		t.Fatalf("login %d code=%d %s", rec.Code, body.Code, rec.Body.String())
	}
	return body.Data.Token
}

// do 发起一次 HTTP 请求。应用错误返回 HTTP 200 + code，测试统一断言外壳。
func do(t *testing.T, s *testServer, method, path, token string, payload any) *httptest.ResponseRecorder {
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
	s.serve(rec, req)
	return rec
}

// assertCode 断言应用错误外壳：HTTP 200 且 code 等于期望值。
func assertCode(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	if rec.Code != 200 || body.Code != want {
		t.Fatalf("want code %d got http=%d code=%d msg=%s", want, rec.Code, body.Code, body.Msg)
	}
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
	var created struct {
		Code int `json:"code"`
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	id := strconv.FormatUint(uint64(created.Data.ID), 10)

	rec = do(t, s, http.MethodPost, "/api/monitors/"+id+"/check", token, nil)
	var checked struct {
		Data struct {
			Monitor shared.Monitor `json:"monitor"`
			Result  struct {
				Success bool `json:"success"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &checked); err != nil {
		t.Fatal(err)
	}
	if checked.Data.Monitor.Status != shared.StatusUp || !checked.Data.Result.Success {
		t.Fatalf("expected up: %+v", checked.Data)
	}

	upstream.Close()
	_ = do(t, s, http.MethodPost, "/api/monitors/"+id+"/check", token, nil)
	rec = do(t, s, http.MethodPost, "/api/monitors/"+id+"/check", token, nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &checked); err != nil {
		t.Fatal(err)
	}
	if checked.Data.Monitor.Status != shared.StatusDown {
		t.Fatalf("expected down, got %s", checked.Data.Monitor.Status)
	}

	dash := do(t, s, http.MethodGet, "/api/dashboard", token, nil)
	if dash.Code != 200 {
		t.Fatalf("dashboard %s", dash.Body.String())
	}
	unauth := do(t, s, http.MethodGet, "/api/monitors", "", nil)
	assertCode(t, unauth, 401)
}

func TestMonitorListPagination(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	for i := 0; i < 3; i++ {
		rec := do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
			"name": "m" + strconv.Itoa(i), "type": "http", "url": "http://127.0.0.1:1", "interval": 60, "timeout": 3,
		})
		if rec.Code != 200 {
			t.Fatalf("create %d %s", rec.Code, rec.Body.String())
		}
	}
	rec := do(t, s, http.MethodGet, "/api/monitors?page=1&pageSize=2", token, nil)
	var page struct {
		Data  []shared.Monitor `json:"data"`
		Total int64            `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 2 || page.Total != 3 {
		t.Fatalf("page %+v", page)
	}
	bad := do(t, s, http.MethodGet, "/api/monitors?page=1&pageSize=999", token, nil)
	assertCode(t, bad, 400)
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
		Data shared.Monitor `json:"data"`
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
			Monitor shared.Monitor `json:"monitor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &downed); err != nil {
		t.Fatal(err)
	}
	if downed.Data.Monitor.Status != shared.StatusDown || downed.Data.Monitor.IncidentStartedAt == nil {
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
	rec = do(t, s, http.MethodGet, "/api/host/processes?sort=memory&limit=5", token, nil)
	if runtime.GOOS == "linux" {
		if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"sort":"memory"`)) {
			t.Fatalf("processes %d %s", rec.Code, rec.Body.String())
		}
	} else {
		assertCode(t, rec, 503)
	}
	rec = do(t, s, http.MethodGet, "/api/host/processes?limit=0", token, nil)
	assertCode(t, rec, 400)

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
		Data shared.ScriptRun `json:"data"`
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
		Data shared.Monitor `json:"data"`
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
	assertCode(t, rec, 400)
}

func TestProcessMonitorCreate(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	rec := do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
		"name": "Nginx", "type": "process", "interval": 30, "timeout": 5,
	})
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodPost, "/api/monitors", token, map[string]any{
		"name": "Nginx", "type": "process", "host": "nginx", "match": "name",
		"threshold": 80, "memThreshold": 256,
		"interval": 30, "timeout": 5, "failureThreshold": 1, "recoveryThreshold": 1,
	})
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"type":"process"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"memThreshold":256`)) {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data shared.Monitor `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.Match != shared.MatchName || created.Data.Threshold != 80 {
		t.Fatalf("%+v", created.Data)
	}
	rec = do(t, s, http.MethodPost, "/api/monitors/"+strconv.FormatUint(uint64(created.Data.ID), 10)+"/check", token, nil)
	if rec.Code != 200 {
		t.Fatalf("check %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/monitors/processes?limit=5", token, nil)
	if rec.Code != 200 {
		t.Fatalf("processes %d %s", rec.Code, rec.Body.String())
	}
}
