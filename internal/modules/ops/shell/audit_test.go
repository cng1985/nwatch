package shell

import (
	"io"
	"strings"
	"testing"
	"time"
)

type fakeTerm struct{}

func (f *fakeTerm) Read([]byte) (int, error)        { return 0, io.EOF }
func (f *fakeTerm) Write(p []byte) (int, error)     { return len(p), nil }
func (f *fakeTerm) Close() error                    { return nil }
func (f *fakeTerm) SetReadDeadline(time.Time) error { return nil }
func (f *fakeTerm) Resize(int, int) error           { return nil }

func TestAuditTypedHiddenAndHistory(t *testing.T) {
	var got []string
	h := NewHub(Options{Enabled: true})
	h.SetRecorder(func(user, remote, session, command string) {
		if user != "alice" || remote != "10.1.1.1" || session != "s1" {
			t.Errorf("meta %s %s %s", user, remote, session)
		}
		got = append(got, command)
	})
	sess := &Session{
		id: "s1", user: "alice", remote: "10.1.1.1",
		term: &fakeTerm{},
		hub:  h, done: make(chan struct{}),
	}
	if _, err := sess.Write([]byte("echo hello\n")); err != nil {
		t.Fatal(err)
	}
	sess.screen.Write([]byte("\n[sudo] password for ubuntu: "))
	if _, err := sess.Write([]byte("secret\n")); err != nil {
		t.Fatal(err)
	}
	sess.screen.Write([]byte("\ralice@host:~$ uptime\x1b[K"))
	if _, err := sess.Write([]byte("\x1b[A\r")); err != nil {
		t.Fatal(err)
	}
	sess.screen.Write([]byte("\ralice@host:~$ echo password\x1b[K"))
	if _, err := sess.Write([]byte("echo password\n")); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "echo hello|uptime|echo password" {
		t.Fatalf("%q", got)
	}
}
