package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/shell"
	"github.com/gorilla/websocket"
)

func TestShellTicketAndSocket(t *testing.T) {
	if shell.LookShell() == "" {
		t.Skip("没有可用的 shell")
	}
	s := newTestServer(t)
	token := login(t, s)

	rec := do(t, s, http.MethodGet, "/api/shell", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth info %d", rec.Code)
	}
	rec = do(t, s, http.MethodGet, "/api/shell", token, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Fatalf("info %d %s", rec.Code, rec.Body.String())
	}

	ticket := issueShellTicket(t, s, token)
	originReq := httptest.NewRequest(http.MethodGet, "/api/shell/ws?ticket="+ticket, nil)
	originReq.Host = "nmonitor.local"
	originReq.Header.Set("Origin", "https://evil.example")
	originRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(originRec, originReq)
	if originRec.Code != http.StatusForbidden {
		t.Fatalf("origin %d %s", originRec.Code, originRec.Body.String())
	}
	if _, err := s.shellHub.Redeem(ticket); err != nil {
		t.Fatalf("origin consumed ticket: %v", err)
	}

	ticket = issueShellTicket(t, s, token)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/shell/ws?ticket=" + ticket + "&cols=100&rows=30"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		body := ""
		if resp != nil {
			raw, _ := io.ReadAll(resp.Body)
			body = string(raw)
		}
		t.Fatalf("dial: %v %s", err, body)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":110,"rows":28}`)); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("printf nmonitor-shell-ok\n")); err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(got.String(), "nmonitor-shell-ok") {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		kind, data, readErr := conn.ReadMessage()
		if kind == websocket.BinaryMessage {
			got.Write(data)
		}
		if readErr != nil && !strings.Contains(readErr.Error(), "timeout") {
			break
		}
	}
	if !strings.Contains(got.String(), "nmonitor-shell-ok") {
		t.Fatalf("socket output %q", got.String())
	}
	_ = conn.Close()
	waitUntil(t, time.Second, func() bool { return s.shellHub.Active() == 0 })

	reuse := httptest.NewServer(s.Handler())
	defer reuse.Close()
	again := "ws" + strings.TrimPrefix(reuse.URL, "http") + "/api/shell/ws?ticket=" + ticket
	if _, resp, err := websocket.DefaultDialer.Dial(again, nil); err == nil {
		t.Fatal("ticket reused")
	} else if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("reuse status %d %s", resp.StatusCode, raw)
	}
}

func TestShellDisabled(t *testing.T) {
	s := newTestServer(t)
	s.shellHub = shell.NewHub(shell.Options{Enabled: false})
	token := login(t, s)
	rec := do(t, s, http.MethodPost, "/api/shell/ticket", token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("disabled %d %s", rec.Code, rec.Body.String())
	}
}

func issueShellTicket(t *testing.T, s *Server, token string) string {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/shell/ticket", token, nil)
	if rec.Code != 200 {
		t.Fatalf("ticket %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Ticket == "" {
		t.Fatal("empty ticket")
	}
	return body.Data.Ticket
}

func waitUntil(t *testing.T, limit time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met")
}
