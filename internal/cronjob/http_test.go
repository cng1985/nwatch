package cronjob

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

func TestNormalizeHTTPModes(t *testing.T) {
	form := &Request{Name: "表单", Cron: "@daily", Kind: model.CronKindHTTP, URL: "https://example.com/hook", Method: "post", BodyType: "form", Body: "a=1\nb=two"}
	if err := form.Normalize(); err != nil {
		t.Fatal(err)
	}
	if form.Body != "a=1&b=two" || form.Command != "" {
		t.Fatalf("%+v", form)
	}
	js := &Request{Name: "JSON", Cron: "0 1 * * *", Kind: "http", URL: "http://example.com/api", Method: "POST", BodyType: "json", Body: `{"ok":true}`}
	if err := js.Normalize(); err != nil {
		t.Fatal(err)
	}
	get := &Request{Name: "读取", Cron: "0 * * * *", Kind: "http", URL: "http://example.com/health", Method: "GET", Body: "ignored"}
	if err := get.Normalize(); err != nil {
		t.Fatal(err)
	}
	if get.Body != "" || get.BodyType != "" {
		t.Fatalf("%+v", get)
	}
	for _, req := range []*Request{
		{Name: "坏地址", Cron: "@daily", Kind: "http", URL: "ftp://example.com", Method: "GET"},
		{Name: "坏 JSON", Cron: "@daily", Kind: "http", URL: "https://example.com", Method: "POST", BodyType: "json", Body: `{`},
		{Name: "坏表单", Cron: "@daily", Kind: "http", URL: "https://example.com", Method: "POST", BodyType: "form", Body: "no-equals"},
		{Name: "坏 XML", Cron: "@daily", Kind: "http", URL: "https://example.com", Method: "POST", BodyType: "xml", Body: "<root>"},
		{Name: "未选类型", Cron: "@daily", Kind: "http", URL: "https://example.com", Method: "POST", Body: "hello"},
	} {
		if err := req.Normalize(); err == nil {
			t.Fatalf("expected error for %s", req.Name)
		}
	}
}

func TestExecuteHTTP(t *testing.T) {
	var method, ctype, body, auth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		method = r.Method
		ctype = r.Header.Get("Content-Type")
		auth = r.Header.Get("Authorization")
		body = string(raw)
		if r.URL.Path == "/fail" {
			http.Error(w, "nope", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	}))
	defer upstream.Close()

	get := executeHTTP(context.Background(), &model.CronJob{Kind: model.CronKindHTTP, Method: "GET", URL: upstream.URL + "/health"}, 5*time.Second)
	if !get.Success || get.ExitCode != 200 || get.Stdout != "pong" || get.Message != "HTTP 200" {
		t.Fatalf("%+v", get)
	}
	postJSON := executeHTTP(context.Background(), &model.CronJob{
		Kind: model.CronKindHTTP, Method: "POST", BodyType: model.CronBodyJSON,
		URL: upstream.URL + "/hook", Body: `{"ok":true}`, Headers: map[string]string{"Authorization": "Bearer t"},
	}, 5*time.Second)
	if !postJSON.Success || method != http.MethodPost || ctype != "application/json" || body != `{"ok":true}` || auth != "Bearer t" {
		t.Fatalf("json %+v method=%s type=%s body=%s auth=%s", postJSON, method, ctype, body, auth)
	}
	postForm := executeHTTP(context.Background(), &model.CronJob{
		Kind: model.CronKindHTTP, Method: "POST", BodyType: model.CronBodyForm,
		URL: upstream.URL, Body: "a=1&b=two",
	}, 5*time.Second)
	if !postForm.Success || ctype != "application/x-www-form-urlencoded" || body != "a=1&b=two" {
		t.Fatalf("form %+v type=%s body=%s", postForm, ctype, body)
	}
	postXML := executeHTTP(context.Background(), &model.CronJob{
		Kind: model.CronKindHTTP, Method: "POST", BodyType: model.CronBodyXML,
		URL: upstream.URL, Body: "<root/>",
	}, 5*time.Second)
	if !postXML.Success || ctype != "application/xml; charset=utf-8" || body != "<root/>" {
		t.Fatalf("xml %+v type=%s body=%s", postXML, ctype, body)
	}
	postText := executeHTTP(context.Background(), &model.CronJob{
		Kind: model.CronKindHTTP, Method: "POST", BodyType: model.CronBodyText,
		URL: upstream.URL, Body: "hello",
	}, 5*time.Second)
	if !postText.Success || ctype != "text/plain; charset=utf-8" || body != "hello" {
		t.Fatalf("text %+v type=%s body=%s", postText, ctype, body)
	}
	failed := executeHTTP(context.Background(), &model.CronJob{Kind: model.CronKindHTTP, Method: "GET", URL: upstream.URL + "/fail"}, 5*time.Second)
	if failed.Success || failed.ExitCode != http.StatusBadGateway || !strings.Contains(failed.Message, "HTTP 502") {
		t.Fatalf("%+v", failed)
	}
}
