package host

import (
	"os"
	"runtime"
	"sync"
	"time"
)

// Disk 是一个挂载点或盘符的容量。
type Disk struct {
	Path    string  `json:"path"`
	FSType  string  `json:"fsType"`
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Free    uint64  `json:"free"`
	Percent float64 `json:"percent"`
}

// Snapshot 是某一时刻的本机资源。
type Snapshot struct {
	Hostname    string    `json:"hostname"`
	CPUCount    int       `json:"cpuCount"`
	CPUPercent  float64   `json:"cpuPercent"`
	MemTotal    uint64    `json:"memTotal"`
	MemUsed     uint64    `json:"memUsed"`
	MemPercent  float64   `json:"memPercent"`
	SwapTotal   uint64    `json:"swapTotal"`
	SwapUsed    uint64    `json:"swapUsed"`
	SwapPercent float64   `json:"swapPercent"`
	Load1       float64   `json:"load1"`
	Load5       float64   `json:"load5"`
	Load15      float64   `json:"load15"`
	Uptime      float64   `json:"uptime"`
	Disks       []Disk    `json:"disks"`
	SampledAt   time.Time `json:"sampledAt"`
}

// Collector 读取本机 CPU、内存和磁盘。CPU 使用率需要相邻两次采样做差值。
type Collector struct {
	mu        sync.Mutex
	prevIdle  uint64
	prevTotal uint64
	hasPrev   bool
}

func NewCollector() *Collector { return &Collector{} }

func (c *Collector) Snapshot() (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotLocked()
}

func (c *Collector) snapshotLocked() (Snapshot, error) {
	idle, total, err := readCPU()
	if err != nil {
		return Snapshot{}, err
	}
	if !c.hasPrev || total < c.prevTotal {
		c.prevIdle, c.prevTotal, c.hasPrev = idle, total, true
		c.mu.Unlock()
		time.Sleep(200 * time.Millisecond)
		c.mu.Lock()
		idle, total, err = readCPU()
		if err != nil {
			return Snapshot{}, err
		}
	}
	cpu := 0.0
	if total > c.prevTotal {
		idleDelta := float64(idle - c.prevIdle)
		totalDelta := float64(total - c.prevTotal)
		cpu = (1 - idleDelta/totalDelta) * 100
	}
	c.prevIdle, c.prevTotal = idle, total

	memTotal, memUsed, swapTotal, swapUsed, err := readMem()
	if err != nil {
		return Snapshot{}, err
	}
	load1, load5, load15 := readLoad()
	uptime := readUptime()
	disks := readDisks()
	name, _ := os.Hostname()
	return Snapshot{
		Hostname:    name,
		CPUCount:    runtime.NumCPU(),
		CPUPercent:  clampPercent(cpu),
		MemTotal:    memTotal,
		MemUsed:     memUsed,
		MemPercent:  percentOf(memUsed, memTotal),
		SwapTotal:   swapTotal,
		SwapUsed:    swapUsed,
		SwapPercent: percentOf(swapUsed, swapTotal),
		Load1:       load1,
		Load5:       load5,
		Load15:      load15,
		Uptime:      uptime,
		Disks:       disks,
		SampledAt:   time.Now(),
	}, nil
}

func RootPercent(disks []Disk) float64 {
	root := DefaultDiskPath()
	for _, disk := range disks {
		if disk.Path == root || disk.Path == "/" {
			return disk.Percent
		}
	}
	if len(disks) == 0 {
		return 0
	}
	return disks[0].Percent
}

func percentOf(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return clampPercent(float64(used) / float64(total) * 100)
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
