//go:build unix

package shell

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSessionEchoResizeAndClose(t *testing.T) {
	h := NewHub(Options{Enabled: true, IdleTimeout: time.Minute, MaxLifetime: time.Hour, MaxSessions: 2})
	t.Cleanup(h.Close)
	sess, err := h.Open("admin", "127.0.0.1", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.Resize(100, 32); err != nil {
		t.Fatal(err)
	}
	if err := sess.Resize(1, 1); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	_ = sess.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _ = sess.Read(buf)
	if _, err := sess.Write([]byte("printf nmonitor-shell-ok\n")); err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(got.String(), "nmonitor-shell-ok") {
		_ = sess.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, readErr := sess.Read(buf)
		if n > 0 {
			got.Write(buf[:n])
		}
		if readErr != nil && !errors.Is(readErr, os.ErrDeadlineExceeded) {
			break
		}
	}
	if !strings.Contains(got.String(), "nmonitor-shell-ok") {
		t.Fatalf("output %q", got.String())
	}
	pid := sess.Pid()
	sess.CloseWith("client")
	deadline = time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat("/proc/" + strconv.Itoa(pid)); err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid %d still present", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
	sess.CloseWith("exit")
	if sess.Reason() != "client" {
		t.Fatalf("reason %s", sess.Reason())
	}
}

func TestSessionLimits(t *testing.T) {
	h := NewHub(Options{Enabled: true, IdleTimeout: 180 * time.Millisecond, MaxLifetime: time.Hour, MaxSessions: 1})
	t.Cleanup(h.Close)
	first, err := h.Open("admin", "127.0.0.1", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Open("admin", "127.0.0.1", 80, 24); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy: %v", err)
	}
	select {
	case <-first.Done():
		t.Fatal("touched session closed early")
	case <-time.After(120 * time.Millisecond):
	}
	first.Touch()
	time.Sleep(250 * time.Millisecond)
	select {
	case <-first.Done():
	default:
		t.Fatal("expected idle close")
	}
	if first.Reason() != "idle" {
		t.Fatalf("reason %s", first.Reason())
	}

	life := NewHub(Options{Enabled: true, IdleTimeout: time.Hour, MaxLifetime: 180 * time.Millisecond, MaxSessions: 1})
	t.Cleanup(life.Close)
	sess, err := life.Open("admin", "127.0.0.1", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-sess.Done():
	case <-time.After(time.Second):
		t.Fatal("expected lifetime close")
	}
	if sess.Reason() != "lifetime" {
		t.Fatalf("reason %s", sess.Reason())
	}
}
