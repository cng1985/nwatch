package host

import "time"

// HostSample 是本机 CPU、内存、磁盘的一次采样。
type HostSample struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CPUPercent  float64   `json:"cpuPercent"`
	MemPercent  float64   `json:"memPercent"`
	MemTotal    uint64    `json:"memTotal"`
	MemUsed     uint64    `json:"memUsed"`
	SwapPercent float64   `json:"swapPercent"`
	Load1       float64   `json:"load1"`
	DiskPercent float64   `json:"diskPercent"`
	DisksRaw    string    `gorm:"type:text" json:"-"`
	SampledAt   time.Time `gorm:"index" json:"sampledAt"`
}

func (HostSample) TableName() string { return "host_samples" }
