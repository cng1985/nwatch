package k8sctl

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls []string
	fn    func(name string, args []string) (string, error)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if f.fn == nil {
		return "", nil
	}
	return f.fn(name, args)
}

const podJSON = `{
  "items": [
    {"metadata":{"name":"web","namespace":"prod","creationTimestamp":"2020-01-01T00:00:00Z"},
     "spec":{"nodeName":"node-a","containers":[{"name":"app","image":"nginx:1.25"}]},
     "status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":2,"state":{"running":{"startedAt":"2020-01-01T00:00:00Z"}}}]}},
    {"metadata":{"name":"bad","namespace":"prod","creationTimestamp":"2020-01-02T00:00:00Z"},
     "spec":{"containers":[{"name":"app","image":"app:1"},{"name":"sidecar","image":"proxy:1"}]},
     "status":{"phase":"Running","containerStatuses":[
       {"ready":false,"restartCount":5,"state":{"waiting":{"reason":"CrashLoopBackOff"}}},
       {"ready":true,"restartCount":0,"state":{"running":{}}}
     ]}}
  ]
}`

const deployJSON = `{
  "items": [
    {"metadata":{"name":"api","namespace":"prod","creationTimestamp":"2020-01-01T00:00:00Z"},
     "spec":{"replicas":3},"status":{"readyReplicas":2}}
  ]
}`

const svcJSON = `{
  "items": [
    {"metadata":{"name":"api","namespace":"prod","creationTimestamp":"2020-01-01T00:00:00Z"},
     "spec":{"type":"ClusterIP","clusterIP":"10.0.0.8","ports":[{"port":80,"protocol":"TCP"},{"port":443}]}}
  ]
}`

func TestListPods(t *testing.T) {
	run := &fakeRunner{fn: func(string, []string) (string, error) { return podJSON, nil }}
	got, err := NewWithRunner(Options{Enabled: true, Context: "lab"}, run).List(context.Background(), "pods", "prod", "")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.Context != "lab" || len(got.Items) != 2 {
		t.Fatalf("%+v", got)
	}
	if got.Items[0].Name != "bad" || got.Items[0].Status != "CrashLoopBackOff" || got.Items[0].Ready != "1/2" || got.Items[0].Restarts != 5 {
		t.Fatalf("bad %+v", got.Items[0])
	}
	if len(got.Items[0].Containers) != 2 || got.Items[1].Name != "web" || got.Items[1].Node != "node-a" || !strings.HasSuffix(got.Items[1].Age, "d") {
		t.Fatalf("%+v", got.Items)
	}
	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "kubectl --context lab get pods -o json --request-timeout=8s -n prod") {
		t.Fatal(joined)
	}
	filtered, err := NewWithRunner(Options{Enabled: true}, run).List(context.Background(), "pods", "", "nginx")
	if err != nil || len(filtered.Items) != 1 || filtered.Items[0].Name != "web" {
		t.Fatalf("%+v %v", filtered.Items, err)
	}
	if !strings.Contains(strings.Join(run.calls, "\n"), "--all-namespaces") {
		t.Fatal(run.calls)
	}
}

func TestWorkloadsServicesAndActions(t *testing.T) {
	run := &fakeRunner{fn: func(name string, args []string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "get deployments"):
			return deployJSON, nil
		case strings.Contains(joined, "get services"):
			return svcJSON, nil
		case strings.Contains(joined, "get namespaces"):
			return `{"items":[{"metadata":{"name":"prod"},"status":{"phase":"Active"}},{"metadata":{"name":"default"},"status":{"phase":"Active"}}]}`, nil
		case strings.Contains(joined, "logs"):
			return "2026-10-14T00:00:00Z listening\n", nil
		default:
			return "", nil
		}
	}}
	m := NewWithRunner(Options{Enabled: true, Kubeconfig: "/tmp/kube.config"}, run)
	deploys, err := m.List(context.Background(), "deployments", "all", "")
	if err != nil || len(deploys.Items) != 1 || deploys.Items[0].Ready != "2/3" || deploys.Items[0].Status != "Progressing" {
		t.Fatalf("%+v %v", deploys.Items, err)
	}
	svcs, err := m.List(context.Background(), "services", "prod", "")
	if err != nil || svcs.Items[0].Ports != "80/TCP, 443/TCP" || svcs.Items[0].ClusterIP != "10.0.0.8" {
		t.Fatalf("%+v %v", svcs.Items, err)
	}
	ns, err := m.Namespaces(context.Background())
	if err != nil || len(ns.Items) != 2 || ns.Items[0].Name != "default" || ns.Items[1].Name != "prod" {
		t.Fatalf("%+v %v", ns.Items, err)
	}
	if _, _, err := m.Restart(context.Background(), "deployments", "prod", "api"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Scale(context.Background(), "deployments", "prod", "api", 2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.DeletePod(context.Background(), "prod", "web"); err != nil {
		t.Fatal(err)
	}
	logs, err := m.Logs(context.Background(), "prod", "web", "app", 40)
	if err != nil || !strings.Contains(logs.Text, "listening") || logs.Lines != 40 {
		t.Fatalf("%+v %v", logs, err)
	}
	joined := strings.Join(run.calls, "\n")
	for _, want := range []string{
		"kubectl --kubeconfig /tmp/kube.config rollout restart deployments api -n prod",
		"kubectl --kubeconfig /tmp/kube.config scale deployments api -n prod --replicas=2",
		"kubectl --kubeconfig /tmp/kube.config delete pod web -n prod --wait=false",
		"kubectl --kubeconfig /tmp/kube.config logs web -n prod --tail=40 --timestamps=true -c app",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s\n%s", want, joined)
		}
	}
	if _, _, err := m.Restart(context.Background(), "services", "prod", "api"); err != ErrBadKind {
		t.Fatal(err)
	}
	if _, _, err := m.Scale(context.Background(), "deployments", "prod", "api", 65); err != ErrBadScale {
		t.Fatal(err)
	}
	if _, _, err := m.DeletePod(context.Background(), "Prod", "web"); err != ErrBadName {
		t.Fatal(err)
	}
	if _, err := m.List(context.Background(), "configmaps", "", ""); err != ErrBadKind {
		t.Fatal(err)
	}
}

func TestKubeDisabledUnavailableAndConfig(t *testing.T) {
	run := &fakeRunner{}
	got, err := NewWithRunner(Options{}, run).List(context.Background(), "pods", "", "")
	if err != nil || got.Enabled || got.Available || !strings.Contains(got.Reason, "关闭") {
		t.Fatalf("%+v %v", got, err)
	}
	if _, _, err := NewWithRunner(Options{}, run).DeletePod(context.Background(), "prod", "web"); err != ErrDisabled {
		t.Fatal(err)
	}
	if len(run.calls) != 0 {
		t.Fatal(run.calls)
	}
	m := New(Options{Enabled: true})
	m.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	got, err = m.List(context.Background(), "pods", "", "")
	if err != nil || got.Available || !strings.Contains(got.Reason, "kubectl") {
		t.Fatalf("%+v %v", got, err)
	}
	bad := New(Options{Enabled: true, Context: "-evil"})
	got, err = bad.List(context.Background(), "pods", "", "")
	if err != nil || got.Available || !strings.Contains(got.Reason, "context") {
		t.Fatalf("%+v %v", got, err)
	}
	if _, _, err := bad.Restart(context.Background(), "deployments", "prod", "api"); !errors.Is(err, ErrBadConfig) {
		t.Fatal(err)
	}
}

func TestKubeCommandFailure(t *testing.T) {
	run := &fakeRunner{fn: func(string, []string) (string, error) {
		return "", errors.New("The connection to the server was refused")
	}}
	_, err := NewWithRunner(Options{Enabled: true}, run).Namespaces(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatal(err)
	}
}
