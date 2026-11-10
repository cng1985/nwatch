package monitor

import (
	"runtime"
	"strings"
	"testing"

	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
)

func TestNormalizeResourceAndScript(t *testing.T) {
	cpu := &MonitorRequest{Name: "CPU", Type: shared.TypeCPU, Interval: 30, Timeout: 5}
	if err := cpu.Normalize(60, 5); err != nil || cpu.Threshold != 90 {
		t.Fatalf("%+v %v", cpu, err)
	}
	disk := &MonitorRequest{Name: "磁盘", Type: shared.TypeDisk, Interval: 30, Timeout: 5, Host: "var"}
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
	script := &MonitorRequest{Name: "脚本", Type: shared.TypeScript, Interval: 30, Timeout: 5, Command: "echo ok", WorkDir: "tmp"}
	if err := script.Normalize(60, 5); err == nil {
		t.Fatal("relative workdir")
	}
	script.WorkDir = absWork
	if err := script.Normalize(60, 5); err != nil {
		t.Fatal(err)
	}
	var saved shared.Monitor
	script.Apply(&saved)
	if saved.Command != "echo ok" || saved.WorkDir != absWork {
		t.Fatalf("%+v", saved)
	}
	if err := (&MonitorRequest{Name: "x", Type: "ping", Interval: 30, Timeout: 5}).Normalize(60, 5); err == nil || !strings.Contains(err.Error(), "script") {
		t.Fatal(err)
	}
}

func TestNormalizeProcess(t *testing.T) {
	missing := &MonitorRequest{Name: "Nginx", Type: shared.TypeProcess, Interval: 30, Timeout: 5}
	if err := missing.Normalize(60, 5); err == nil {
		t.Fatal("expected process name")
	}
	req := &MonitorRequest{
		Name: "Nginx", Type: shared.TypeProcess, Interval: 30, Timeout: 5,
		Host: " nginx ", Command: " nginx: master ", Threshold: 80, MemThreshold: 512,
	}
	if err := req.Normalize(60, 5); err != nil {
		t.Fatal(err)
	}
	if req.Host != "nginx" || req.Match != shared.MatchCommand || req.Threshold != 80 || req.MemThreshold != 512 {
		t.Fatalf("%+v", req)
	}
	var saved shared.Monitor
	req.Apply(&saved)
	if saved.Type != shared.TypeProcess || saved.Match != shared.MatchCommand || saved.MemThreshold != 512 || saved.Command != "nginx: master" {
		t.Fatalf("%+v", saved)
	}
	nameOnly := &MonitorRequest{Name: "Nginx", Type: shared.TypeProcess, Interval: 30, Timeout: 5, Host: "nginx", Match: shared.MatchName, Threshold: 0}
	if err := nameOnly.Normalize(60, 5); err != nil || nameOnly.Match != shared.MatchName || nameOnly.Threshold != 0 {
		t.Fatalf("%+v %v", nameOnly, err)
	}
	bad := &MonitorRequest{Name: "Nginx", Type: shared.TypeProcess, Interval: 30, Timeout: 5, Host: "nginx", Match: shared.MatchCommand}
	if err := bad.Normalize(60, 5); err == nil {
		t.Fatal("expected command")
	}
}

func TestNormalizeHTTPBody(t *testing.T) {
	req := &MonitorRequest{
		Name: "API", Type: shared.TypeHTTP, URL: "https://example.com/hook", Method: "POST",
		BodyType: "formdata", Body: "tenant=141\nok=1", Interval: 60, Timeout: 5,
	}
	if err := req.Normalize(60, 5); err != nil {
		t.Fatal(err)
	}
	if req.BodyType != "formdata" || req.Body != "ok=1&tenant=141" {
		t.Fatalf("%s %s", req.BodyType, req.Body)
	}
	legacy := &MonitorRequest{
		Name: "旧请求", Type: shared.TypeHTTP, URL: "https://example.com/hook", Method: "POST",
		Body: "hello", Interval: 60, Timeout: 5,
	}
	if err := legacy.Normalize(60, 5); err != nil || legacy.BodyType != "text" || legacy.Body != "hello" {
		t.Fatalf("%+v %v", legacy, err)
	}
	get := &MonitorRequest{
		Name: "健康检查", Type: shared.TypeHTTP, URL: "http://example.com/health", Method: "GET",
		BodyType: "json", Body: `{"ok":true}`, Interval: 60, Timeout: 5,
	}
	if err := get.Normalize(60, 5); err != nil || get.Body != "" || get.BodyType != "" {
		t.Fatalf("%+v %v", get, err)
	}
	bad := &MonitorRequest{
		Name: "坏 JSON", Type: shared.TypeHTTP, URL: "https://example.com", Method: "POST",
		BodyType: "json", Body: "{", Interval: 60, Timeout: 5,
	}
	if err := bad.Normalize(60, 5); err == nil {
		t.Fatal("expected json error")
	}
	var saved shared.Monitor
	req.Apply(&saved)
	if saved.BodyType != "formdata" || saved.Body != "ok=1&tenant=141" {
		t.Fatalf("%+v", saved)
	}
}
