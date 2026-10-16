package monitor

import (
	"runtime"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/model"
)

func TestNormalizeResourceAndScript(t *testing.T) {
	cpu := &Request{Name: "CPU", Type: model.TypeCPU, Interval: 30, Timeout: 5}
	if err := cpu.Normalize(60, 5); err != nil || cpu.Threshold != 90 {
		t.Fatalf("%+v %v", cpu, err)
	}
	disk := &Request{Name: "磁盘", Type: model.TypeDisk, Interval: 30, Timeout: 5, Host: "var"}
	if err := disk.Normalize(60, 5); err == nil {
		t.Fatal("relative disk path")
	}
	absDisk := "/var"
	absWork := "/tmp"
	if runtime.GOOS == "windows" {
		absDisk = `C:\`
		absWork = `C:\nmonitor`
	}
	disk.Host = absDisk
	disk.Threshold = 80
	if err := disk.Normalize(60, 5); err != nil {
		t.Fatal(err)
	}
	script := &Request{Name: "脚本", Type: model.TypeScript, Interval: 30, Timeout: 5, Command: "echo ok", WorkDir: "tmp"}
	if err := script.Normalize(60, 5); err == nil {
		t.Fatal("relative workdir")
	}
	script.WorkDir = absWork
	if err := script.Normalize(60, 5); err != nil {
		t.Fatal(err)
	}
	var saved model.Monitor
	script.Apply(&saved)
	if saved.Command != "echo ok" || saved.WorkDir != absWork {
		t.Fatalf("%+v", saved)
	}
	if err := (&Request{Name: "x", Type: "ping", Interval: 30, Timeout: 5}).Normalize(60, 5); err == nil || !strings.Contains(err.Error(), "script") {
		t.Fatal(err)
	}
}

func TestNormalizeHTTPBody(t *testing.T) {
	req := &Request{
		Name: "API", Type: model.TypeHTTP, URL: "https://example.com/hook", Method: "POST",
		BodyType: "formdata", Body: "tenant=141\nok=1", Interval: 60, Timeout: 5,
	}
	if err := req.Normalize(60, 5); err != nil {
		t.Fatal(err)
	}
	if req.BodyType != "formdata" || req.Body != "ok=1&tenant=141" {
		t.Fatalf("%s %s", req.BodyType, req.Body)
	}
	legacy := &Request{
		Name: "旧请求", Type: model.TypeHTTP, URL: "https://example.com/hook", Method: "POST",
		Body: "hello", Interval: 60, Timeout: 5,
	}
	if err := legacy.Normalize(60, 5); err != nil || legacy.BodyType != "text" || legacy.Body != "hello" {
		t.Fatalf("%+v %v", legacy, err)
	}
	get := &Request{
		Name: "健康检查", Type: model.TypeHTTP, URL: "http://example.com/health", Method: "GET",
		BodyType: "json", Body: `{"ok":true}`, Interval: 60, Timeout: 5,
	}
	if err := get.Normalize(60, 5); err != nil || get.Body != "" || get.BodyType != "" {
		t.Fatalf("%+v %v", get, err)
	}
	bad := &Request{
		Name: "坏 JSON", Type: model.TypeHTTP, URL: "https://example.com", Method: "POST",
		BodyType: "json", Body: "{", Interval: 60, Timeout: 5,
	}
	if err := bad.Normalize(60, 5); err == nil {
		t.Fatal("expected json error")
	}
	var saved model.Monitor
	req.Apply(&saved)
	if saved.BodyType != "formdata" || saved.Body != "ok=1&tenant=141" {
		t.Fatalf("%+v", saved)
	}
}
