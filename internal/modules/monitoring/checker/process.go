package checker

import (
	"context"
	"fmt"
	"github.com/cng1985/nwatch/internal/modules/monitoring/host"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"strings"
	"time"
)

type processLister interface {
	AllProcesses() ([]host.Process, error)
}

// ProcessChecker 检查所选进程是否仍在运行，以及 CPU、内存是否超过阈值。
type ProcessChecker struct{ host processLister }

func NewProcessChecker(collector *host.Collector) *ProcessChecker {
	return &ProcessChecker{host: collector}
}

func (c *ProcessChecker) Type() string { return shared.TypeProcess }

func (c *ProcessChecker) Check(ctx context.Context, m *shared.Monitor) (*Result, error) {
	start := time.Now()
	items, err := c.host.AllProcesses()
	elapsed := int(time.Since(start).Milliseconds())
	if err != nil {
		return failResult("ERROR", err.Error()), nil
	}
	return evaluateProcess(m, items, elapsed), nil
}

func evaluateProcess(m *shared.Monitor, items []host.Process, elapsed int) *Result {
	if m == nil {
		return failResult("ERROR", "未配置进程")
	}
	name := strings.TrimSpace(m.Host)
	if name == "" {
		return failResult("ERROR", "未配置进程名")
	}
	command := strings.TrimSpace(m.Command)
	mode := processMatch(m.Match, command)
	matched := matchProcesses(items, name, command, mode)

	cpuThreshold := m.Threshold
	if cpuThreshold < 0 {
		cpuThreshold = 0
	}
	memThreshold := m.MemThreshold
	if memThreshold < 0 {
		memThreshold = 0
	}

	var cpuMax, cpuSum, memPercent float64
	var memBytes uint64
	pids := make([]int, 0, len(matched))
	sampleCommand := command
	for _, proc := range matched {
		if proc.CPUPercent > cpuMax {
			cpuMax = proc.CPUPercent
		}
		cpuSum += proc.CPUPercent
		memBytes += proc.MemBytes
		memPercent += proc.MemPercent
		pids = append(pids, proc.PID)
		if sampleCommand == "" {
			sampleCommand = proc.Command
		}
	}
	if len(pids) > 20 {
		pids = pids[:20]
	}
	cpuMax = round1(cpuMax)
	cpuSum = round1(cpuSum)
	memPercent = round1(memPercent)
	memMB := float64(memBytes) / (1024 * 1024)

	meta := map[string]any{
		"name":           name,
		"command":        sampleCommand,
		"match":          mode,
		"count":          len(matched),
		"pids":           pids,
		"cpuPercent":     cpuMax,
		"cpuSum":         cpuSum,
		"memBytes":       memBytes,
		"memMB":          round1(memMB),
		"memPercent":     memPercent,
		"cpuThreshold":   cpuThreshold,
		"memThresholdMB": memThreshold,
	}

	if len(matched) == 0 {
		res := failResult("STOPPED", fmt.Sprintf("进程 %s 未运行", name))
		res.ResponseTime = elapsed
		res.Metadata = meta
		res.CheckedAt = time.Now()
		return res
	}

	cpuHigh := cpuThreshold > 0 && cpuMax > cpuThreshold
	memHigh := memThreshold > 0 && memMB > memThreshold
	usage := processUsageText(len(matched), cpuMax, memMB)
	message := fmt.Sprintf("进程 %s 正在运行，%s", name, usage)
	status := "OK"
	success := true
	if cpuHigh && memHigh {
		success = false
		status = "HIGH"
		message = fmt.Sprintf("进程 %s CPU %.1f%% 超过 %.1f%%，内存 %.1f MB 超过 %.1f MB", name, cpuMax, cpuThreshold, memMB, memThreshold)
	} else if cpuHigh {
		success = false
		status = "HIGH"
		message = fmt.Sprintf("进程 %s CPU %.1f%%，超过阈值 %.1f%%（%s）", name, cpuMax, cpuThreshold, usage)
	} else if memHigh {
		success = false
		status = "HIGH"
		message = fmt.Sprintf("进程 %s 内存 %.1f MB，超过阈值 %.1f MB（%s）", name, memMB, memThreshold, usage)
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

func processMatch(match, command string) string {
	switch strings.ToLower(strings.TrimSpace(match)) {
	case shared.MatchCommand:
		return shared.MatchCommand
	case shared.MatchName:
		return shared.MatchName
	default:
		if strings.TrimSpace(command) != "" {
			return shared.MatchCommand
		}
		return shared.MatchName
	}
}

func matchProcesses(items []host.Process, name, command, mode string) []host.Process {
	out := make([]host.Process, 0)
	for _, proc := range items {
		if !processAlive(proc) || proc.Name != name {
			continue
		}
		if mode == shared.MatchCommand && !commandMatches(proc.Command, command) {
			continue
		}
		out = append(out, proc)
	}
	return out
}

func processAlive(proc host.Process) bool {
	switch proc.State {
	case "Z", "X", "x":
		return false
	default:
		return true
	}
}

func commandMatches(live, want string) bool {
	live = strings.TrimSpace(live)
	want = strings.TrimSpace(want)
	if want == "" || live == "" {
		return false
	}
	return strings.Contains(live, want) || strings.Contains(want, live)
}

func processUsageText(count int, cpu, memMB float64) string {
	if count > 1 {
		return fmt.Sprintf("%d 个实例，CPU 最高 %.1f%%，内存合计 %.1f MB", count, cpu, memMB)
	}
	return fmt.Sprintf("1 个实例，CPU %.1f%%，内存 %.1f MB", cpu, memMB)
}
