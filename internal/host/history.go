package host

import (
	"encoding/json"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"gorm.io/gorm"
)

// Point 是图表上的一个时间点。
type Point struct {
	Time        time.Time `json:"time"`
	CPUPercent  float64   `json:"cpuPercent"`
	MemPercent  float64   `json:"memPercent"`
	DiskPercent float64   `json:"diskPercent"`
}

func Since(name string) (time.Time, string) {
	switch name {
	case "6h":
		return time.Now().Add(-6 * time.Hour), "6h"
	case "24h":
		return time.Now().Add(-24 * time.Hour), "24h"
	case "7d":
		return time.Now().Add(-7 * 24 * time.Hour), "7d"
	default:
		return time.Now().Add(-time.Hour), "1h"
	}
}

func LoadHistory(db *gorm.DB, from time.Time) ([]Point, error) {
	var rows []model.HostSample
	err := db.Where("sampled_at >= ?", from).Order("sampled_at asc").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return Downsample(rows, 180), nil
}

func Downsample(rows []model.HostSample, max int) []Point {
	if max < 1 {
		max = 180
	}
	if len(rows) == 0 {
		return []Point{}
	}
	if len(rows) <= max {
		return mapPoints(rows)
	}
	size := float64(len(rows)) / float64(max)
	out := make([]Point, 0, max)
	for i := 0; i < max; i++ {
		start := int(float64(i) * size)
		end := int(float64(i+1) * size)
		if end <= start {
			end = start + 1
		}
		if start >= len(rows) {
			break
		}
		if end > len(rows) {
			end = len(rows)
		}
		out = append(out, average(rows[start:end]))
	}
	return out
}

func mapPoints(rows []model.HostSample) []Point {
	out := make([]Point, 0, len(rows))
	for _, row := range rows {
		out = append(out, Point{
			Time:        row.SampledAt,
			CPUPercent:  row.CPUPercent,
			MemPercent:  row.MemPercent,
			DiskPercent: row.DiskPercent,
		})
	}
	return out
}

func average(rows []model.HostSample) Point {
	var cpu, mem, disk float64
	for _, row := range rows {
		cpu += row.CPUPercent
		mem += row.MemPercent
		disk += row.DiskPercent
	}
	n := float64(len(rows))
	return Point{
		Time:        rows[len(rows)-1].SampledAt,
		CPUPercent:  cpu / n,
		MemPercent:  mem / n,
		DiskPercent: disk / n,
	}
}

func SampleFrom(snap Snapshot) model.HostSample {
	raw, _ := json.Marshal(snap.Disks)
	return model.HostSample{
		CPUPercent:  snap.CPUPercent,
		MemPercent:  snap.MemPercent,
		MemTotal:    snap.MemTotal,
		MemUsed:     snap.MemUsed,
		SwapPercent: snap.SwapPercent,
		Load1:       snap.Load1,
		DiskPercent: RootPercent(snap.Disks),
		DisksRaw:    string(raw),
		SampledAt:   snap.SampledAt,
	}
}
