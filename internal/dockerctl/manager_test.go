package dockerctl

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

const psText = "" +
	"abc123def456\tweb\tnginx:1.25\tUp 2 hours\t0.0.0.0:80->80/tcp\n" +
	"fff999aaa111\tworker\tapp:1\tExited (1) 3 minutes ago\t\n" +
	"111222333444\tbatch\treq:1\tRestarting (1) 4 seconds ago\t\n" +
	"555666777888\t/paused_job\tjob:1\tUp 5 minutes (Paused)\t\n" +
	"bad name\tnope:1\tUp 1 second\t\n"

func TestContainersParseFilterAndSort(t *testing.T) {
	run := &fakeRunner{fn: func(string, []string) (string, error) { return psText, nil }}
	got, err := NewWithRunner(true, run).Containers(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || len(got.Items) != 4 {
		t.Fatalf("%+v", got.Items)
	}
	want := []string{"batch", "paused_job", "web", "worker"}
	for i, name := range want {
		if got.Items[i].Name != name {
			t.Fatalf("order %v", names(got.Items))
		}
	}
	if got.Items[2].State != "running" || got.Items[2].Ports == "" || got.Items[1].State != "paused" {
		t.Fatalf("%+v", got.Items)
	}
	filtered, err := NewWithRunner(true, run).Containers(context.Background(), "nginx", "running")
	if err != nil || len(filtered.Items) != 1 || filtered.Items[0].Name != "web" {
		t.Fatalf("%+v %v", filtered.Items, err)
	}
}

func TestContainerActionsAndLogs(t *testing.T) {
	run := &fakeRunner{fn: func(name string, args []string) (string, error) {
		if args[0] == "logs" {
			return "2026-10-14T00:00:00Z started\n", nil
		}
		return "", nil
	}}
	m := NewWithRunner(true, run)
	if _, err := m.Action(context.Background(), "web", "stop"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Action(context.Background(), "/web", "restart"); err != nil {
		t.Fatal(err)
	}
	logs, err := m.Logs(context.Background(), "web", 10)
	if err != nil || logs.Lines != minLogLines || !strings.Contains(logs.Text, "started") {
		t.Fatalf("%+v %v", logs, err)
	}
	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "docker stop -- web") || !strings.Contains(joined, "docker restart -- web") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "docker logs --tail 20 --timestamps -- web") {
		t.Fatal(joined)
	}
	if _, err := m.Action(context.Background(), "web;rm", "start"); err != ErrBadName {
		t.Fatalf("bad name %v", err)
	}
	if _, err := m.Action(context.Background(), "-web", "start"); err != ErrBadName {
		t.Fatalf("flag %v", err)
	}
}

func TestImages(t *testing.T) {
	run := &fakeRunner{fn: func(string, []string) (string, error) {
		return "aaa\tnginx\t1.25\t20MB\t2 days ago\nbbb\t<none>\t<none>\t5MB\t3 weeks ago\n", nil
	}}
	got, err := NewWithRunner(true, run).Images(context.Background(), "nginx")
	if err != nil || len(got.Items) != 1 || got.Items[0].Repository != "nginx" || got.Items[0].Tag != "1.25" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestDockerDisabledAndUnavailable(t *testing.T) {
	run := &fakeRunner{}
	got, err := NewWithRunner(false, run).Containers(context.Background(), "", "")
	if err != nil || got.Enabled || got.Available || !strings.Contains(got.Reason, "关闭") {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := NewWithRunner(false, run).Action(context.Background(), "web", "start"); err != ErrDisabled {
		t.Fatal(err)
	}
	if len(run.calls) != 0 {
		t.Fatal(run.calls)
	}
	m := New(true)
	m.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	got, err = m.Containers(context.Background(), "", "")
	if err != nil || got.Available || !strings.Contains(got.Reason, "docker") {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := m.Logs(context.Background(), "web", 20); err != ErrUnavailable {
		t.Fatal(err)
	}
}

func TestDockerCommandFailure(t *testing.T) {
	run := &fakeRunner{fn: func(string, []string) (string, error) {
		return "", errors.New("Cannot connect to the Docker daemon")
	}}
	_, err := NewWithRunner(true, run).Containers(context.Background(), "", "")
	if err == nil || !strings.Contains(err.Error(), "Docker daemon") {
		t.Fatal(err)
	}
}

func names(items []Container) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Name
	}
	return out
}
