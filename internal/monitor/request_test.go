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
