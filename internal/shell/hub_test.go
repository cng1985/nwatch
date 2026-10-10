package shell

import (
	"errors"
	"testing"
	"time"
)

func TestTicketSingleUseAndExpiry(t *testing.T) {
	if LookShell() == "" {
		t.Skip("没有可用的 shell")
	}
	h := NewHub(Options{Enabled: true, TicketTTL: time.Hour})
	token, err := h.Issue("admin")
	if err != nil || token == "" {
		t.Fatalf("issue: %v %q", err, token)
	}
	user, err := h.Redeem(token)
	if err != nil || user != "admin" {
		t.Fatalf("redeem: %v %q", err, user)
	}
	if _, err := h.Redeem(token); !errors.Is(err, ErrTicket) {
		t.Fatalf("reuse: %v", err)
	}

	short := NewHub(Options{Enabled: true, TicketTTL: 20 * time.Millisecond})
	token, err = short.Issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, err := short.Redeem(token); !errors.Is(err, ErrTicket) {
		t.Fatalf("expired: %v", err)
	}
}

func TestTicketDisabledAndCap(t *testing.T) {
	if LookShell() == "" {
		t.Skip("没有可用的 shell")
	}
	h := NewHub(Options{Enabled: false})
	if _, err := h.Issue("admin"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled: %v", err)
	}

	h = NewHub(Options{Enabled: true, TicketTTL: time.Hour})
	for i := 0; i < maxPendingTickets; i++ {
		if _, err := h.Issue("admin"); err != nil {
			t.Fatalf("issue %d: %v", i, err)
		}
	}
	if _, err := h.Issue("admin"); !errors.Is(err, ErrTooManyTickets) {
		t.Fatalf("cap: %v", err)
	}
}

func TestNormalizeSize(t *testing.T) {
	cols, rows := NormalizeSize(0, 0)
	if cols != 80 || rows != 24 {
		t.Fatalf("default %d %d", cols, rows)
	}
	cols, rows = NormalizeSize(1000, 2)
	if cols != 500 || rows != 24 {
		t.Fatalf("clamp %d %d", cols, rows)
	}
}
