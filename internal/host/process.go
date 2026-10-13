package host

import (
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultProcessLimit = 80
	maxProcessLimit     = 300
	maxCommandRunes     = 400
	processCacheTTL     = time.Second
)

// Process 是某一时刻的一个本机进程。CPUPercent 按单个核心计算，可以超过 100。
type Process struct {
	PID        int     `json:"pid"`
	PPID       int     `json:"ppid"`
	User       string  `json:"user"`
	Name       string  `json:"name"`
	Command    string  `json:"command"`
	State      string  `json:"state"`
	StateText  string  `json:"stateText"`
	CPUPercent float64 `json:"cpuPercent"`
	MemBytes   uint64  `json:"memBytes"`
	MemPercent float64 `json:"memPercent"`
}

// ProcessList 是按 CPU 或内存排序后的进程列表。
type ProcessList struct {
	Items     []Process `json:"items"`
	Total     int       `json:"total"`
	Sort      string    `json:"sort"`
	SampledAt time.Time `json:"sampledAt"`
}

type rawProc struct {
	PID     int
	PPID    int
	User    string
	Name    string
	Command string
	State   string
	RSS     uint64
	Ticks   uint64
}

type procSample struct {
	Total    uint64
	MemTotal uint64
	At       time.Time
	Procs    []rawProc
}

type procBaseline struct {
	total uint64
	ticks map[int]uint64
}

func (s procSample) baseline() procBaseline {
	ticks := make(map[int]uint64, len(s.Procs))
	for _, proc := range s.Procs {
		ticks[proc.PID] = proc.Ticks
	}
	return procBaseline{total: s.Total, ticks: ticks}
}

// Processes 返回本机进程。sortBy 为 cpu 或 memory，按占用从高到低排列。
func (c *Collector) Processes(sortBy string, limit int) (ProcessList, error) {
	sortBy = normalizeProcessSort(sortBy)
	limit = normalizeProcessLimit(limit)
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.procCached) > 0 && time.Since(c.procCacheAt) < processCacheTTL {
		return finishProcessList(c.procCached, sortBy, limit, c.procCacheAt), nil
	}

	cur, err := readProcSnapshot()
	if err != nil {
		return ProcessList{}, err
	}
	if !c.hasProc || cur.Total < c.procBase.total {
		c.procBase = cur.baseline()
		c.hasProc = true
		c.mu.Unlock()
		time.Sleep(200 * time.Millisecond)
		c.mu.Lock()
		cur, err = readProcSnapshot()
		if err != nil {
			return ProcessList{}, err
		}
	}
	items := assembleProcesses(c.procBase, cur, runtime.NumCPU())
	c.procBase = cur.baseline()
	c.hasProc = true
	c.procCached = items
	c.procCacheAt = cur.At
	return finishProcessList(items, sortBy, limit, cur.At), nil
}

func finishProcessList(src []Process, sortBy string, limit int, at time.Time) ProcessList {
	items := append([]Process(nil), src...)
	sortProcesses(items, sortBy)
	total := len(items)
	if len(items) > limit {
		items = items[:limit]
	}
	if items == nil {
		items = []Process{}
	}
	return ProcessList{Items: items, Total: total, Sort: sortBy, SampledAt: at}
}

func assembleProcesses(base procBaseline, cur procSample, cpuCount int) []Process {
	if cpuCount < 1 {
		cpuCount = 1
	}
	var deltaTotal uint64
	if cur.Total > base.total {
		deltaTotal = cur.Total - base.total
	}
	items := make([]Process, 0, len(cur.Procs))
	for _, proc := range cur.Procs {
		cpu := 0.0
		if deltaTotal > 0 {
			if prev, ok := base.ticks[proc.PID]; ok && proc.Ticks >= prev {
				cpu = float64(proc.Ticks-prev) / float64(deltaTotal) * float64(cpuCount) * 100
			}
		}
		name := proc.Name
		if name == "" {
			name = proc.Command
		}
		if name == "" {
			name = "-"
		}
		items = append(items, Process{
			PID:        proc.PID,
			PPID:       proc.PPID,
			User:       proc.User,
			Name:       name,
			Command:    proc.Command,
			State:      proc.State,
			StateText:  processStateText(proc.State),
			CPUPercent: cpu,
			MemBytes:   proc.RSS,
			MemPercent: percentOf(proc.RSS, cur.MemTotal),
		})
	}
	return items
}

func sortProcesses(items []Process, sortBy string) {
	byMemory := sortBy == "memory"
	sort.SliceStable(items, func(i, j int) bool {
		if byMemory {
			if items[i].MemBytes != items[j].MemBytes {
				return items[i].MemBytes > items[j].MemBytes
			}
			if items[i].CPUPercent != items[j].CPUPercent {
				return items[i].CPUPercent > items[j].CPUPercent
			}
		} else {
			if items[i].CPUPercent != items[j].CPUPercent {
				return items[i].CPUPercent > items[j].CPUPercent
			}
			if items[i].MemBytes != items[j].MemBytes {
				return items[i].MemBytes > items[j].MemBytes
			}
		}
		return items[i].PID < items[j].PID
	})
}

func normalizeProcessSort(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "mem", "memory", "rss":
		return "memory"
	default:
		return "cpu"
	}
}

func normalizeProcessLimit(limit int) int {
	if limit <= 0 {
		return defaultProcessLimit
	}
	if limit > maxProcessLimit {
		return maxProcessLimit
	}
	return limit
}

func processStateText(state string) string {
	switch state {
	case "R":
		return "运行"
	case "S":
		return "睡眠"
	case "D":
		return "不可中断"
	case "Z":
		return "僵尸"
	case "T":
		return "停止"
	case "t":
		return "跟踪停止"
	case "I":
		return "空闲"
	case "X", "x":
		return "退出"
	default:
		if state == "" {
			return "-"
		}
		return state
	}
}

// parseProcStat 解析 /proc/pid/stat。进程名用括号包住，里面可以有空格。
func parseProcStat(text string) (name, state string, ppid int, ticks, rssPages uint64, ok bool) {
	start := strings.IndexByte(text, '(')
	end := strings.LastIndexByte(text, ')')
	if start < 0 || end <= start {
		return
	}
	name = text[start+1 : end]
	rest := strings.Fields(strings.TrimSpace(text[end+1:]))
	if len(rest) < 22 || rest[0] == "" {
		return
	}
	state = rest[0]
	ppid, _ = strconv.Atoi(rest[1])
	utime, err1 := strconv.ParseUint(rest[11], 10, 64)
	stime, err2 := strconv.ParseUint(rest[12], 10, 64)
	if err1 != nil || err2 != nil {
		return
	}
	ticks = utime + stime
	if pages, err := strconv.ParseUint(rest[21], 10, 64); err == nil {
		rssPages = pages
	}
	ok = true
	return
}

func cleanCommand(s string) string {
	s = strings.ReplaceAll(s, "\x00", " ")
	s = strings.Join(strings.Fields(s), " ")
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	return trimRunes(s, maxCommandRunes)
}

func trimRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	if n < 1 {
		return ""
	}
	return string(runes[:n-1]) + "…"
}
