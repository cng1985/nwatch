package httpreq

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeBodies(t *testing.T) {
	form, err := mustNormalize(t, TypeForm, "a=1\nb=two", false)
	if form != "a=1&b=two" {
		t.Fatalf("form %s", form)
	}
	data, err := mustNormalize(t, TypeFormData, "tenant=141\nok=1", false)
	if err != nil {
		t.Fatal(err)
	}
	if data != "ok=1&tenant=141" {
		t.Fatalf("formdata %s", data)
	}
	js, err := mustNormalize(t, TypeJSON, "  {\"ok\":true}  ", false)
	if err != nil || js != `{"ok":true}` {
		t.Fatalf("json %q %v", js, err)
	}
	xmlBody, err := mustNormalize(t, TypeXML, " <root>ok</root> ", false)
	if err != nil || xmlBody != "<root>ok</root>" {
		t.Fatalf("xml %q %v", xmlBody, err)
	}
	text, err := mustNormalize(t, TypeText, "  hi  ", false)
	if err != nil || text != "  hi  " {
		t.Fatalf("text %q %v", text, err)
	}
	emptyType, emptyBody, err := Normalize("", "", true)
	if err != nil || emptyType != "" || emptyBody != "" {
		t.Fatalf("empty %q %q %v", emptyType, emptyBody, err)
	}
	legacy, legacyBody, err := Normalize("", "hello", true)
	if err != nil || legacy != TypeText || legacyBody != "hello" {
		t.Fatalf("legacy %q %q %v", legacy, legacyBody, err)
	}
	for _, item := range []struct{ kind, body string }{
		{TypeJSON, "{"},
		{TypeJSON, ""},
		{TypeForm, "no-equals"},
		{TypeFormData, "onlykey"},
		{TypeXML, "<root>"},
		{TypeXML, ""},
		{"yaml", "a: 1"},
	} {
		if _, _, err := Normalize(item.kind, item.body, false); err == nil {
			t.Fatalf("expected error for %s %q", item.kind, item.body)
		}
	}
	if _, _, err := Normalize("", "hello", false); err == nil {
		t.Fatal("expected body type")
	}
}

func TestEncodeFormData(t *testing.T) {
	payload, err := Encode(TypeFormData, "a=1&b=two+words")
	if err != nil {
		t.Fatal(err)
	}
	media, params, err := mime.ParseMediaType(payload.ContentType)
	if err != nil || media != "multipart/form-data" || params["boundary"] == "" {
		t.Fatalf("content type %s %v", payload.ContentType, err)
	}
	reader := multipart.NewReader(payload.Body, params["boundary"])
	form, err := reader.ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if form.Value["a"][0] != "1" || form.Value["b"][0] != "two words" {
		t.Fatalf("%v", form.Value)
	}
	req, err := http.NewRequest(http.MethodPost, "https://example.com", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", payload.ContentType)
	if req.Header.Get("Content-Type") != payload.ContentType {
		t.Fatal(req.Header.Get("Content-Type"))
	}
}

func TestEncodeContentTypes(t *testing.T) {
	cases := []struct {
		kind, body, want string
	}{
		{TypeJSON, `{"ok":true}`, "application/json"},
		{TypeForm, "a=1", "application/x-www-form-urlencoded"},
		{TypeXML, "<root/>", "application/xml; charset=utf-8"},
		{TypeText, "hello", "text/plain; charset=utf-8"},
	}
	for _, item := range cases {
		payload, err := Encode(item.kind, item.body)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(payload.Body)
		if payload.ContentType != item.want || string(raw) != item.body {
			t.Fatalf("%s type=%s body=%s", item.kind, payload.ContentType, raw)
		}
	}
	empty, err := Encode("", "")
	if err != nil || empty.Body != nil || empty.ContentType != "" {
		t.Fatalf("%+v %v", empty, err)
	}
}

func mustNormalize(t *testing.T, kind, body string, allowEmpty bool) (string, error) {
	t.Helper()
	gotKind, gotBody, err := Normalize(kind, body, allowEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if gotKind != kind {
		t.Fatalf("kind %s", gotKind)
	}
	return gotBody, nil
}
