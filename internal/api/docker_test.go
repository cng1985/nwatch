package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/dockerctl"
)

func TestDockerAPI(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	run := &dockerRunner{fn: func(name string, args []string) (string, error) {
		switch {
		case name == "docker" && len(args) > 0 && args[0] == "ps":
			return "abc\tweb\tnginx:1.25\tUp 2 hours\t0.0.0.0:80->80/tcp\n", nil
		case name == "docker" && len(args) > 0 && args[0] == "images":
			return "img\tnginx\t1.25\t20MB\t2 days ago\n", nil
		case name == "docker" && len(args) > 0 && args[0] == "logs":
			return "2026-10-14T00:00:00Z started\n", nil
		default:
			return "", nil
		}
	}}
	s.docker = dockerctl.NewWithRunner(true, run)

	rec := do(t, s, http.MethodGet, "/api/docker/containers?state=running", token, nil)
	if rec.Code != 200 {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Data dockerctl.ContainerList `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if !listed.Data.Available || len(listed.Data.Items) != 1 || listed.Data.Items[0].Name != "web" {
		t.Fatalf("%+v", listed.Data)
	}

	rec = do(t, s, http.MethodPost, "/api/docker/containers/web/stop", token, nil)
	if rec.Code != 200 {
		t.Fatalf("stop %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/docker/containers/web/logs?lines=40", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "started") {
		t.Fatalf("logs %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/docker/images?q=nginx", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "nginx") {
		t.Fatalf("images %d %s", rec.Code, rec.Body.String())
	}
	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "docker stop -- web") || !strings.Contains(joined, "docker logs --tail 40 --timestamps -- web") {
		t.Fatal(joined)
	}

	rec = do(t, s, http.MethodPost, "/api/docker/containers/bad%20name/start", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("name %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/docker/containers?state=nope", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("state %d", rec.Code)
	}
	rec = do(t, s, http.MethodGet, "/api/docker/containers", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("auth %d", rec.Code)
	}
}

func TestDockerAPIDisabled(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	s.docker = dockerctl.NewWithRunner(false, &dockerRunner{})
	rec := do(t, s, http.MethodGet, "/api/docker/containers", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/docker/containers/web/start", token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/docker/images", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("images %d %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAPICommandFailure(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	s.docker = dockerctl.NewWithRunner(true, &dockerRunner{fn: func(string, []string) (string, error) {
		return "", errors.New("No such container: web")
	}})
	rec := do(t, s, http.MethodPost, "/api/docker/containers/web/start", token, nil)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "No such container") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

type dockerRunner struct {
	calls []string
	fn    func(name string, args []string) (string, error)
}

func (r *dockerRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if r.fn == nil {
		return "", nil
	}
	return r.fn(name, args)
}
