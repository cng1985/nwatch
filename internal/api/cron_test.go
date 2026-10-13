package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/model"
)

func TestCronJobAPI(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)

	rec := do(t, s, http.MethodGet, "/api/cron-jobs", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("auth %d", rec.Code)
	}
	rec = do(t, s, http.MethodPost, "/api/cron-jobs", token, map[string]any{
		"name": "坏表达式", "cron": "99 * * * *", "command": "echo x",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid cron %d %s", rec.Code, rec.Body.String())
	}
	preview := "/api/cron/preview?expr=" + url.QueryEscape("0 2 * * *") + "&count=3"
	rec = do(t, s, http.MethodGet, preview, token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "每天 02:00") || !strings.Contains(rec.Body.String(), "Asia/Shanghai") {
		t.Fatalf("preview %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodPost, "/api/cron-jobs", token, map[string]any{
		"name": "清理临时文件", "cron": "@daily", "command": "echo cron-api",
		"timeout": 5, "description": "每天清理",
	})
	if rec.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data model.CronJob `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	job := created.Data
	if job.Cron != "0 0 * * *" || job.ScheduleText != "每天 00:00" || job.NextRunAt == nil || !job.Enabled {
		t.Fatalf("%+v", job)
	}
	id := strconv.FormatUint(uint64(job.ID), 10)

	rec = do(t, s, http.MethodPost, "/api/cron-jobs/"+id+"/run", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "cron-api") || !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("run %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/cron-jobs/"+id+"/runs", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "cron-api") || !strings.Contains(rec.Body.String(), "manual") {
		t.Fatalf("runs %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/cron-jobs/"+id+"/disable", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("disable %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodGet, "/api/export", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "清理临时文件") || !strings.Contains(rec.Body.String(), `"cron":"0 0 * * *"`) {
		t.Fatalf("export %d %s", rec.Code, rec.Body.String())
	}
	var exported struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	rec = do(t, s, http.MethodDelete, "/api/cron-jobs/"+id, token, nil)
	if rec.Code != 200 {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/import", token, exported.Data)
	if rec.Code != 200 {
		t.Fatalf("import %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/cron-jobs?keyword="+url.QueryEscape("清理"), token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "清理临时文件") || !strings.Contains(rec.Body.String(), "每天 00:00") {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
}

func TestCronHTTPJobAPI(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	var method, ctype, body string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		method, ctype, body = r.Method, r.Header.Get("Content-Type"), string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pong":true}`))
	}))
	defer upstream.Close()

	rec := do(t, s, http.MethodPost, "/api/cron-jobs", token, map[string]any{
		"name": "回调", "cron": "0 3 * * *", "kind": "http", "method": "POST", "bodyType": "json",
		"url": upstream.URL + "/hook", "body": `{"name":"nmonitor"}`, "headers": map[string]string{"X-Job": "cron"},
		"timeout": 5,
	})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "POST JSON") {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data model.CronJob `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatUint(uint64(created.Data.ID), 10)
	rec = do(t, s, http.MethodPost, "/api/cron-jobs/"+id+"/run", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"success":true`) || !strings.Contains(rec.Body.String(), "pong") {
		t.Fatalf("run %d %s", rec.Code, rec.Body.String())
	}
	if method != http.MethodPost || ctype != "application/json" || body != `{"name":"nmonitor"}` {
		t.Fatalf("method=%s type=%s body=%s", method, ctype, body)
	}
	rec = do(t, s, http.MethodPost, "/api/cron-jobs", token, map[string]any{
		"name": "坏 JSON", "cron": "@hourly", "kind": "http", "method": "POST", "bodyType": "json",
		"url": upstream.URL, "body": "{",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid json %d %s", rec.Code, rec.Body.String())
	}
}
