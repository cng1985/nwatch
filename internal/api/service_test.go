package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/svcctl"
)

func TestServiceAPI(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	run := &svcctlRunner{fn: func(name string, args []string) (string, error) {
		switch {
		case name == "systemctl" && len(args) > 0 && args[0] == "list-units":
			return "nginx.service loaded active running web server\n", nil
		case name == "systemctl" && len(args) > 0 && args[0] == "list-unit-files":
			return "nginx.service enabled\ncron.service disabled\n", nil
		case name == "journalctl":
			return "2026-10-13T00:00:00+0000 host nginx[1]: started\n", nil
		default:
			return "", nil
		}
	}}
	s.services = svcctl.NewWithRunner(true, run)

	rec := do(t, s, http.MethodGet, "/api/services?state=active", token, nil)
	if rec.Code != 200 {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Data svcctl.ListResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if !listed.Data.Available || len(listed.Data.Items) != 1 || listed.Data.Items[0].Name != "nginx.service" {
		t.Fatalf("%+v", listed.Data)
	}

	rec = do(t, s, http.MethodPost, "/api/services/nginx.service/stop", token, nil)
	if rec.Code != 200 {
		t.Fatalf("stop %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/services/cron/start", token, nil)
	if rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "systemctl stop --no-pager nginx.service") || !strings.Contains(joined, "systemctl start --no-pager cron.service") {
		t.Fatal(joined)
	}

	rec = do(t, s, http.MethodGet, "/api/services/nginx.service/logs?lines=40", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "started") {
		t.Fatalf("logs %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodPost, "/api/services/nginx.timer/restart", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("timer %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/services/systemd.service/stop", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("protected %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/services?state=nope", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("state %d", rec.Code)
	}
	rec = do(t, s, http.MethodGet, "/api/services", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("auth %d", rec.Code)
	}
}

func TestServiceAPIDisabled(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	s.services = svcctl.NewWithRunner(false, &svcctlRunner{})
	rec := do(t, s, http.MethodGet, "/api/services", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/services/nginx/start", token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/services/nginx/logs", token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("logs %d %s", rec.Code, rec.Body.String())
	}
}

func TestServiceAPICommandFailure(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	s.services = svcctl.NewWithRunner(true, &svcctlRunner{fn: func(string, []string) (string, error) {
		return "", errors.New("Unit nginx.service not found")
	}})
	rec := do(t, s, http.MethodPost, "/api/services/nginx.service/start", token, nil)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "not found") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

type svcctlRunner struct {
	calls []string
	fn    func(name string, args []string) (string, error)
}

func (r *svcctlRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if r.fn == nil {
		return "", nil
	}
	return r.fn(name, args)
}
