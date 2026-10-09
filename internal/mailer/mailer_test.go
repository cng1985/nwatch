package mailer

import (
	"context"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/mailer/fakesmtp"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
)

func TestNormalizeRecipients(t *testing.T) {
	cfg := Normalize(Config{To: "a@example.com，b@example.com; a@example.com\nb@example.com", Encryption: "ssl", Port: 0})
	if cfg.Port != 465 || cfg.Encryption != "ssl" {
		t.Fatalf("port/encryption %+v", cfg)
	}
	if len(cfg.Recipients) != 2 {
		t.Fatalf("recipients %#v", cfg.Recipients)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing host and from")
	}
}

func TestFormatOngoing(t *testing.T) {
	subject, body := Format(Notice{
		Ongoing:  true,
		Failures: 4,
		Event: model.AlertEvent{
			EventType: model.EventDown, MonitorName: "ERP", NewStatus: model.StatusDown,
			Message: "timeout", OccurredAt: time.Now(),
		},
	}, time.Local)
	if subject != "服务仍然异常 ERP" || !strings.Contains(body, "连续失败：4 次") || !strings.Contains(body, "持续重试") {
		t.Fatalf("subject %s body %s", subject, body)
	}
}

func TestSendAndRetry(t *testing.T) {
	srv := fakesmtp.Start(t)
	host, portText, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "mail.db")
	cfg.Mail = config.MailConfig{
		Enabled: true, Host: host, Port: port, Username: "mailer", Password: "secret",
		From: "NMonitor <nmonitor@example.com>", To: "ops@example.com", Encryption: "none",
	}
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store, err := settings.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := Send(context.Background(), Config{
		Host: host, Port: port, Username: "mailer", Password: "secret",
		From: cfg.Mail.From, To: cfg.Mail.To, Encryption: "none",
	}, "服务异常 ERP", "第一次"); err != nil {
		t.Fatal(err)
	}
	if len(srv.Messages()) != 1 || !strings.Contains(srv.Messages()[0], "第一次") {
		t.Fatalf("message %#v", srv.Messages())
	}

	srv.FailNext(2)
	svc := New(db, store, nil)
	svc.initialBackoff = 20 * time.Millisecond
	svc.maxBackoff = 40 * time.Millisecond
	svc.Start()
	t.Cleanup(func() { svc.Stop(context.Background()) })
	svc.Notify(context.Background(), Notice{Event: model.AlertEvent{
		MonitorID: 7, MonitorName: "ERP", EventType: model.EventDown, NewStatus: model.StatusDown,
		Message: "仍然失败", OccurredAt: time.Now(),
	}})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(srv.Messages()) >= 2 && strings.Contains(srv.Messages()[len(srv.Messages())-1], "仍然失败") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("retry did not deliver, messages=%#v", srv.Messages())
}
