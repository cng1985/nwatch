package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/k8sctl"
)

func TestKubernetesAPI(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	run := &kubeRunner{fn: func(name string, args []string) (string, error) {
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
	s.kube = k8sctl.NewWithRunner(k8sctl.Options{Enabled: true, Context: "lab"}, run)

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
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("kind %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/kubernetes/workloads/deployments/prod/api/scale", token, map[string]int{"replicas": 99})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("scale %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/kubernetes/resources?kind=secrets", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("secrets %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/api/kubernetes/resources", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("auth %d", rec.Code)
	}
}

func TestKubernetesAPIDisabled(t *testing.T) {
	s := newTestServer(t)
	token := login(t, s)
	s.kube = k8sctl.NewWithRunner(k8sctl.Options{}, &kubeRunner{})
	rec := do(t, s, http.MethodGet, "/api/kubernetes/resources?kind=pods", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/kubernetes/pods/prod/web/delete", token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
}

type kubeRunner struct {
	calls []string
	fn    func(name string, args []string) (string, error)
}

func (r *kubeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if r.fn == nil {
		return "", nil
	}
	return r.fn(name, args)
}
