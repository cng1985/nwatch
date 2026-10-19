package host

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestParseProcStat(t *testing.T) {
	text := "42 (nginx worker) S 1 42 42 0 -1 4194560 10 0 0 0 100 50 0 0 20 0 3 0 999 1000000 200"
	name, state, ppid, ticks, rss, ok := parseProcStat(text)
	if !ok || name != "nginx worker" || state != "S" || ppid != 1 || ticks != 150 || rss != 200 {
		t.Fatalf("name=%q state=%q ppid=%d ticks=%d rss=%d ok=%v", name, state, ppid, ticks, rss, ok)
	}
	nested := "7 (my (weird) proc) R 2 0 0 0 0 0 0 0 0 0 4 6 0 0 20 0 1 0 1 1 9"
	name, state, ppid, ticks, rss, ok = parseProcStat(nested)
	if !ok || name != "my (weird) proc" || state != "R" || ppid != 2 || ticks != 10 || rss != 9 {
		t.Fatalf("nested name=%q state=%q ppid=%d ticks=%d rss=%d ok=%v", name, state, ppid, ticks, rss, ok)
	}
	if _, _, _, _, _, ok = parseProcStat("nope"); ok {
		t.Fatal("expected invalid stat")
	}
}

func TestSortProcesses(t *testing.T) {
	items := []Process{
		{PID: 3, CPUPercent: 1, MemBytes: 30},
		{PID: 2, CPUPercent: 9, MemBytes: 10},
		{PID: 1, CPUPercent: 9, MemBytes: 20},
		{PID: 4, CPUPercent: 4, MemBytes: 40},
	}
	sortProcesses(items, "cpu")
	got := []int{items[0].PID, items[1].PID, items[2].PID, items[3].PID}
	if got[0] != 1 || got[1] != 2 || got[2] != 4 || got[3] != 3 {
		t.Fatalf("cpu order %v", got)
	}
	sortProcesses(items, "memory")
	got = []int{items[0].PID, items[1].PID, items[2].PID, items[3].PID}
	if got[0] != 4 || got[1] != 3 || got[2] != 1 || got[3] != 2 {
		t.Fatalf("memory order %v", got)
	}
	if normalizeProcessSort("MEM") != "memory" || normalizeProcessSort("") != "cpu" {
		t.Fatal("sort name")
	}
	if normalizeProcessLimit(0) != defaultProcessLimit || normalizeProcessLimit(1000) != maxProcessLimit || normalizeProcessLimit(5) != 5 {
		t.Fatal("limit")
	}
}

func TestProcessesReadsLocal(t *testing.T) {
	c := NewCollector()
	if runtime.GOOS != "linux" {
		if _, err := c.Processes("cpu", 10); err == nil {
			t.Fatal("expected unsupported")
		}
		return
	}
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
		}
	}()
	defer close(stop)
	time.Sleep(20 * time.Millisecond)

	list, err := c.Processes("cpu", 500)
	if err != nil {
		t.Fatal(err)
	}
	if list.Sort != "cpu" || list.Total < 1 || len(list.Items) == 0 || len(list.Items) > list.Total {
		t.Fatalf("total=%d items=%d sort=%s", list.Total, len(list.Items), list.Sort)
	}
	self := os.Getpid()
	found := false
	for i, item := range list.Items {
		if i > 0 && item.CPUPercent > list.Items[i-1].CPUPercent {
			t.Fatalf("cpu not sorted at %d", i)
		}
		if item.PID == self {
			found = true
			if item.CPUPercent <= 0 || item.MemBytes == 0 || item.User == "" || item.Name == "" {
				t.Fatalf("self %+v", item)
			}
		}
		if item.MemPercent < 0 || item.MemPercent > 100 || item.CPUPercent < 0 {
			t.Fatalf("percent %+v", item)
		}
	}
	if !found {
		t.Fatal("missing self")
	}
	mem, err := c.Processes("memory", 1)
	if err != nil {
		t.Fatal(err)
	}
	if mem.Sort != "memory" || mem.Total < 1 || len(mem.Items) != 1 || mem.Items[0].MemBytes == 0 {
		t.Fatalf("%+v", mem)
	}
	if !mem.SampledAt.Equal(list.SampledAt) {
		t.Fatalf("sort should reuse the sample, cpu=%s memory=%s", list.SampledAt, mem.SampledAt)
	}
	all, err := c.AllProcesses()
	if err != nil || len(all) != list.Total {
		t.Fatalf("all=%d total=%d err=%v", len(all), list.Total, err)
	}
}

func TestParseProcStatMatchesLive(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux proc")
	}
	raw, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Fatal(err)
	}
	name, state, ppid, ticks, rssPages, ok := parseProcStat(string(raw))
	if !ok || name == "" || state == "" || ppid <= 0 || rssPages == 0 {
		t.Fatalf("name=%q state=%q ppid=%d ticks=%d rss=%d", name, state, ppid, ticks, rssPages)
	}
}
