package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/cng1985/nwatch/internal/host"
	"github.com/cng1985/nwatch/internal/model"
)

type metrics interface {
	Snapshot() (host.Snapshot, error)
}

type CPUChecker struct{ host metrics }

func NewCPUChecker(collector *host.Collector) *CPUChecker {
	return &CPUChecker{host: collector}
}

func (c *CPUChecker) Type() string { return model.TypeCPU }

func (c *CPUChecker) Check(ctx context.Context, m *model.Monitor) (*Result, error) {
	start := time.Now()
	snap, err := c.host.Snapshot()
	elapsed := int(time.Since(start).Milliseconds())
	if err != nil {
		return failResult("ERROR", err.Error()), nil
	}
	threshold := thresholdOf(m)
	meta := map[string]any{
		"cpuPercent": round1(snap.CPUPercent),
		"threshold":  threshold,
		"load1":      snap.Load1,
		"load5":      snap.Load5,
		"load15":     snap.Load15,
	}
	return percentResult("CPU", snap.CPUPercent, threshold, elapsed, meta), nil
}

type MemoryChecker struct{ host metrics }

func NewMemoryChecker(collector *host.Collector) *MemoryChecker {
	return &MemoryChecker{host: collector}
}

func (c *MemoryChecker) Type() string { return model.TypeMemory }

func (c *MemoryChecker) Check(ctx context.Context, m *model.Monitor) (*Result, error) {
	start := time.Now()
	snap, err := c.host.Snapshot()
	elapsed := int(time.Since(start).Milliseconds())
	if err != nil {
		return failResult("ERROR", err.Error()), nil
	}
	threshold := thresholdOf(m)
	meta := map[string]any{
		"memPercent": round1(snap.MemPercent),
		"memUsed":    snap.MemUsed,
		"memTotal":   snap.MemTotal,
		"threshold":  threshold,
	}
	return percentResult("内存", snap.MemPercent, threshold, elapsed, meta), nil
}

type DiskChecker struct{}

func NewDiskChecker() *DiskChecker { return &DiskChecker{} }

func (c *DiskChecker) Type() string { return model.TypeDisk }

func (c *DiskChecker) Check(ctx context.Context, m *model.Monitor) (*Result, error) {
	path := m.Host
	if path == "" {
		path = host.DefaultDiskPath()
	}
	start := time.Now()
	disk, err := host.Usage(path)
	elapsed := int(time.Since(start).Milliseconds())
	if err != nil {
		return failResult("ERROR", err.Error()), nil
	}
	threshold := thresholdOf(m)
	meta := map[string]any{
		"path":      path,
		"percent":   round1(disk.Percent),
		"used":      disk.Used,
		"total":     disk.Total,
		"threshold": threshold,
	}
	res := percentResult("磁盘 "+path, disk.Percent, threshold, elapsed, meta)
	return res, nil
}

func thresholdOf(m *model.Monitor) float64 {
	if m == nil || m.Threshold <= 0 {
		return 90
	}
	if m.Threshold > 100 {
		return 100
	}
	return m.Threshold
}

func percentResult(label string, value, threshold float64, elapsed int, meta map[string]any) *Result {
	value = round1(value)
	success := value <= threshold
	status := "OK"
	message := fmt.Sprintf("%s %.1f%%，阈值 %.1f%%", label, value, threshold)
	if !success {
		status = "HIGH"
		message = fmt.Sprintf("%s %.1f%%，超过阈值 %.1f%%", label, value, threshold)
	}
	return &Result{
		Success:      success,
		Status:       status,
		ResponseTime: elapsed,
		Message:      message,
		CheckedAt:    time.Now(),
		Metadata:     meta,
	}
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
