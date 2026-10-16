package checker

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

func TestHTTPCheckerStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	}))
	defer srv.Close()

	c := NewHTTPChecker()
	m := &model.Monitor{
		Type: model.TypeHTTP, URL: srv.URL, Method: "GET", Timeout: 3,
		ExpectedStatusCodes: "200", BodyContains: "UP", FollowRedirects: true,
		Headers: map[string]string{"X-Token": "secret"},
	}
	res, err := c.Check(context.Background(), m)
	if err != nil || !res.Success || res.StatusCode != 200 || res.ResponseTime < 0 {
		t.Fatalf("success result=%+v err=%v", res, err)
	}

	m.BodyContains = "DOWN"
	res, err = c.Check(context.Background(), m)
	if err != nil || res.Success || res.Status != "BODY_MISMATCH" {
		t.Fatalf("body mismatch result=%+v err=%v", res, err)
	}

	m.Headers = nil
	m.BodyContains = ""
	res, err = c.Check(context.Background(), m)
	if err != nil || res.Success || res.StatusCode != 401 {
		t.Fatalf("status mismatch result=%+v err=%v", res, err)
	}
}

func TestHTTPCheckerPostBody(t *testing.T) {
	var method, ctype, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		method, ctype, body = r.Method, r.Header.Get("Content-Type"), string(raw)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := NewHTTPChecker()
	m := &model.Monitor{
		URL: srv.URL, Method: "POST", Timeout: 3, ExpectedStatusCodes: "200", FollowRedirects: true,
		BodyType: "json", Body: `{"tenant":141}`, Headers: map[string]string{"X-Token": "secret"},
	}
	res, err := c.Check(context.Background(), m)
	if err != nil || !res.Success || method != http.MethodPost || ctype != "application/json" || body != `{"tenant":141}` {
		t.Fatalf("result=%+v err=%v method=%s type=%s body=%s", res, err, method, ctype, body)
	}
	m.Method = "GET"
	m.BodyType = "json"
	m.Body = `{"tenant":141}`
	res, err = c.Check(context.Background(), m)
	if err != nil || !res.Success || method != http.MethodGet || body != "" {
		t.Fatalf("get result=%+v err=%v method=%s body=%s", res, err, method, body)
	}
}

func TestHTTPCheckerTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := NewHTTPChecker()
	m := &model.Monitor{URL: srv.URL, Method: "GET", Timeout: 1, ExpectedStatusCodes: "200"}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	res, err := c.Check(ctx, m)
	if err != nil || res.Success {
		t.Fatalf("expected timeout failure, result=%+v err=%v", res, err)
	}
}
