package svcctl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

const unitsText = `
systemd.service            loaded active running systemd
● nginx.service            loaded failed failed A high performance web server
cron.service               loaded inactive dead Regular background program processing daemon
sshd.service               loaded active running OpenSSH server daemon
getty@.service             loaded inactive dead getty template
notreal.service            not-found inactive dead gone
UNIT LOAD ACTIVE SUB DESCRIPTION
`

const filesText = `
UNIT FILE                              STATE
nginx.service                          enabled
cron.service                           disabled
sshd.service                           enabled
nginx.timer                            enabled
getty@.service                         indirect
rsyslog.service                        enabled
systemd.service                        static
`

func TestListMergesAndSorts(t *testing.T) {
	run := &fakeRunner{fn: func(name string, args []string) (string, error) {
		if args[0] == "list-units" {
			return unitsText, nil
		}
		return filesText, nil
	}}
	got, err := NewWithRunner(true, run).List(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || len(got.Items) != 4 {
		t.Fatalf("items %+v", got.Items)
	}
	want := []string{"nginx.service", "sshd.service", "cron.service", "rsyslog.service"}
	for i, name := range want {
		if got.Items[i].Name != name {
			t.Fatalf("order %v", names(got.Items))
		}
	}
	if got.Items[0].Active != "failed" || got.Items[0].UnitFile != "enabled" {
		t.Fatalf("nginx %+v", got.Items[0])
	}
	if got.Items[3].Active != "inactive" || got.Items[3].Sub != "dead" || got.Items[3].UnitFile != "enabled" {
		t.Fatalf("rsyslog %+v", got.Items[3])
	}
}

func TestListFilters(t *testing.T) {
	run := &fakeRunner{fn: func(name string, args []string) (string, error) {
		if args[0] == "list-units" {
			return unitsText, nil
		}
		return filesText, nil
	}}
	m := NewWithRunner(true, run)
	failed, err := m.List(context.Background(), "web", "failed")
	if err != nil {
		t.Fatal(err)
	}
	if len(failed.Items) != 1 || failed.Items[0].Name != "nginx.service" {
		t.Fatalf("%+v", failed.Items)
	}
	active, err := m.List(context.Background(), "", "active")
	if err != nil {
		t.Fatal(err)
	}
	if len(active.Items) != 1 || active.Items[0].Name != "sshd.service" {
		t.Fatalf("%+v", names(active.Items))
	}
}

func TestActionRejectsUnsafeNames(t *testing.T) {
	run := &fakeRunner{}
	m := NewWithRunner(true, run)
	for _, name := range []string{"", "../nginx", "nginx;id", "nginx.timer", "getty@.service", "systemd", "init.service", "foo bar.service", "a/b.service"} {
		if _, err := m.Action(context.Background(), name, "stop"); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if len(run.calls) != 0 {
		t.Fatalf("runner called: %v", run.calls)
	}
	unit, err := m.Action(context.Background(), "nginx", "restart")
	if err != nil {
		t.Fatal(err)
	}
	if unit != "nginx.service" || len(run.calls) != 1 || run.calls[0] != "systemctl restart --no-pager nginx.service" {
		t.Fatalf("unit %s calls %v", unit, run.calls)
	}
}

func TestActionSurfacesCommandError(t *testing.T) {
	run := &fakeRunner{fn: func(string, []string) (string, error) {
		return "", errors.New("Access denied")
	}}
	_, err := NewWithRunner(true, run).Action(context.Background(), "nginx.service", "start")
	if err == nil || !strings.Contains(err.Error(), "Access denied") {
		t.Fatal(err)
	}
}

func TestLogsClampAndCommand(t *testing.T) {
	run := &fakeRunner{fn: func(string, []string) (string, error) {
		return "2026-10-13T00:00:00+0000 host nginx[1]: hello\n", nil
	}}
	got, err := NewWithRunner(true, run).Logs(context.Background(), "nginx.service", 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.Lines != minLogLines || !strings.Contains(got.Text, "hello") {
		t.Fatalf("%+v", got)
	}
	if run.calls[0] != "journalctl -u nginx.service -n 20 --no-pager -o short-iso" {
		t.Fatal(run.calls[0])
	}
	empty, err := NewWithRunner(true, &fakeRunner{fn: func(string, []string) (string, error) {
		return "-- No entries --\n", nil
	}}).Logs(context.Background(), "cron", 0)
	if err != nil || empty.Text != "没有日志" || empty.Lines != defaultLogLines {
		t.Fatalf("%+v %v", empty, err)
	}
}

func TestListFallsBackToUnitFiles(t *testing.T) {
	run := &fakeRunner{fn: func(name string, args []string) (string, error) {
		if args[0] == "list-units" {
			return "", errors.New("System has not been booted with systemd as init system (PID 1). Can't operate.")
		}
		return filesText, nil
	}}
	got, err := NewWithRunner(true, run).List(context.Background(), "nginx", "")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || len(got.Items) != 1 || got.Items[0].Name != "nginx.service" || got.Items[0].Active != "inactive" {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(got.Reason, "无法读取运行状态") {
		t.Fatal(got.Reason)
	}
}

func TestDisabledSkipsCommands(t *testing.T) {
	run := &fakeRunner{}
	m := NewWithRunner(false, run)
	list, err := m.List(context.Background(), "", "")
	if err != nil || list.Enabled || list.Available || list.Reason == "" {
		t.Fatalf("%+v %v", list, err)
	}
	if _, err := m.Action(context.Background(), "nginx", "start"); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	if _, err := m.Logs(context.Background(), "nginx", 20); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	if len(run.calls) != 0 {
		t.Fatal(run.calls)
	}
}

func TestFillDescriptionsFromUnitFile(t *testing.T) {
	dir := t.TempDir()
	body := "[Unit]\nDescription=\"Demo bus\"\n\n[Service]\nDescription=ignored\n"
	if err := os.WriteFile(filepath.Join(dir, "dbus.service"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	items := []Unit{{Name: "dbus.service", Active: "inactive"}, {Name: "nginx.service", Description: "already"}}
	fillDescriptions(items, []string{dir})
	if items[0].Description != "Demo bus" || items[1].Description != "already" {
		t.Fatalf("%+v", items)
	}
	if unitDescription([]string{dir}, "../dbus.service") != "" {
		t.Fatal("path escape should be ignored")
	}
}

func TestTrimLogKeepsTail(t *testing.T) {
	text := strings.Repeat("a", maxLogBytes) + "\nTAIL"
	got := trimLog(text)
	if !strings.HasSuffix(got, "TAIL") || len(got) > maxLogBytes {
		t.Fatalf("len %d suffix %q", len(got), got[len(got)-8:])
	}
}

func names(items []Unit) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Name
	}
	return out
}
