package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/modules/ops/docker"
	"github.com/cng1985/nwatch/internal/modules/ops/service"
)

type recordingRunner struct {
	calls []string
	fn    func(name string, args []string) (string, error)
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if r.fn == nil {
		return "", nil
	}
	return r.fn(name, args)
}

func TestServiceAPI(t *testing.T) {
	run := &recordingRunner{fn: func(name string, args []string) (string, error) {
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
	s := newTestServerWithOptions(t, testOptions{serviceRunner: run})
	token := login(t, s)

	rec := do(t, s, http.MethodGet, "/api/services?state=active", token, nil)
	if rec.Code != 200 {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Data service.ListResult `json:"data"`
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
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodPost, "/api/services/systemd.service/stop", token, nil)
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodGet, "/api/services?state=nope", token, nil)
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodGet, "/api/services", "", nil)
	assertCode(t, rec, 401)
}

func TestServiceAPIDisabled(t *testing.T) {
	s := newTestServerWithOptions(t, testOptions{serviceDisabled: true, serviceRunner: &recordingRunner{}})
	token := login(t, s)
	rec := do(t, s, http.MethodGet, "/api/services", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/services/nginx/start", token, nil)
	assertCode(t, rec, 403)
	rec = do(t, s, http.MethodGet, "/api/services/nginx/logs", token, nil)
	assertCode(t, rec, 403)
}

func TestServiceAPICommandFailure(t *testing.T) {
	s := newTestServerWithOptions(t, testOptions{serviceRunner: &recordingRunner{fn: func(string, []string) (string, error) {
		return "", errString("Unit nginx.service not found")
	}}})
	token := login(t, s)
	rec := do(t, s, http.MethodPost, "/api/services/nginx.service/start", token, nil)
	assertCode(t, rec, 503)
	if !strings.Contains(rec.Body.String(), "not found") {
		t.Fatalf("%s", rec.Body.String())
	}
}

func TestDockerAPI(t *testing.T) {
	run := &recordingRunner{fn: func(name string, args []string) (string, error) {
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
	s := newTestServerWithOptions(t, testOptions{dockerRunner: run})
	token := login(t, s)

	rec := do(t, s, http.MethodGet, "/api/docker/containers?state=running", token, nil)
	if rec.Code != 200 {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Data docker.ContainerList `json:"data"`
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
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodGet, "/api/docker/containers?state=nope", token, nil)
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodGet, "/api/docker/containers", "", nil)
	assertCode(t, rec, 401)
}

func TestDockerAPIDisabled(t *testing.T) {
	s := newTestServerWithOptions(t, testOptions{dockerDisabled: true, dockerRunner: &recordingRunner{}})
	token := login(t, s)
	rec := do(t, s, http.MethodGet, "/api/docker/containers", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/docker/containers/web/start", token, nil)
	assertCode(t, rec, 403)
	rec = do(t, s, http.MethodGet, "/api/docker/images", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("images %d %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAPICommandFailure(t *testing.T) {
	s := newTestServerWithOptions(t, testOptions{dockerRunner: &recordingRunner{fn: func(string, []string) (string, error) {
		return "", errString("No such container: web")
	}}})
	token := login(t, s)
	rec := do(t, s, http.MethodPost, "/api/docker/containers/web/start", token, nil)
	assertCode(t, rec, 503)
	if !strings.Contains(rec.Body.String(), "No such container") {
		t.Fatalf("%s", rec.Body.String())
	}
}

func TestKubernetesAPI(t *testing.T) {
	run := &recordingRunner{fn: func(name string, args []string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "get namespaces"):
			return `{"items":[{"metadata":{"name":"prod"},"status":{"phase":"Active"}}]}`, nil
		case strings.Contains(joined, "get pods"):
			return `{"items":[{"metadata":{"name":"web","namespace":"prod","creationTimestamp":"2020-01-01T00:00:00Z"},"spec":{"containers":[{"name":"app","image":"nginx:1.25"}]},"status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":1}]}}]}`, nil
		case strings.Contains(joined, "logs"):
			return "2026-10-14T00:00:00Z listening\n", nil
		default:
			return "", nil
		}
	}}
	s := newTestServerWithOptions(t, testOptions{kubeRunner: run})
	token := login(t, s)

	rec := do(t, s, http.MethodGet, "/api/kubernetes/namespaces", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "prod") {
		t.Fatalf("ns %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/kubernetes/resources?kind=pods&namespace=prod", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"name":"web"`) {
		t.Fatalf("pods %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/kubernetes/workloads/deployments/prod/api/restart", token, nil)
	if rec.Code != 200 {
		t.Fatalf("restart %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/kubernetes/workloads/deployments/prod/api/scale", token, map[string]int{"replicas": 2})
	if rec.Code != 200 {
		t.Fatalf("scale %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/kubernetes/pods/prod/web/delete", token, nil)
	if rec.Code != 200 {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/kubernetes/pods/prod/web/logs?lines=40&container=app", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "listening") {
		t.Fatalf("logs %d %s", rec.Code, rec.Body.String())
	}
	joined := strings.Join(run.calls, "\n")
	for _, want := range []string{
		"kubectl --context lab rollout restart deployments api -n prod",
		"kubectl --context lab scale deployments api -n prod --replicas=2",
		"kubectl --context lab delete pod web -n prod --wait=false",
		"kubectl --context lab logs web -n prod --tail=40 --timestamps=true -c app",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s\n%s", want, joined)
		}
	}

	rec = do(t, s, http.MethodPost, "/api/kubernetes/workloads/services/prod/api/restart", token, nil)
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodPost, "/api/kubernetes/workloads/deployments/prod/api/scale", token, map[string]int{"replicas": 99})
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodGet, "/api/kubernetes/resources?kind=secrets", token, nil)
	assertCode(t, rec, 400)
	rec = do(t, s, http.MethodGet, "/api/kubernetes/resources", "", nil)
	assertCode(t, rec, 401)
}

func TestKubernetesAPIDisabled(t *testing.T) {
	s := newTestServerWithOptions(t, testOptions{kubeDisabled: true, kubeRunner: &recordingRunner{}})
	token := login(t, s)
	rec := do(t, s, http.MethodGet, "/api/kubernetes/resources?kind=pods", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/kubernetes/pods/prod/web/delete", token, nil)
	assertCode(t, rec, 403)
}

// errString 用最小错误实现适配 Runner 签名。
type errString string

func (e errString) Error() string { return string(e) }
