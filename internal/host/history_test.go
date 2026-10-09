package host

import (
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

func TestDownsampleAveragesBuckets(t *testing.T) {
	start := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	rows := make([]model.HostSample, 4)
	for i := range rows {
		rows[i] = model.HostSample{
			CPUPercent:  float64(i + 1),
			MemPercent:  float64((i + 1) * 10),
			DiskPercent: 50,
			SampledAt:   start.Add(time.Duration(i) * time.Minute),
		}
	}
	got := Downsample(rows, 2)
	if len(got) != 2 {
		t.Fatalf("points %d", len(got))
	}
	if got[0].CPUPercent != 1.5 || got[1].CPUPercent != 3.5 {
		t.Fatalf("cpu avg %+v", got)
	}
	if got[0].MemPercent != 15 || got[1].MemPercent != 35 {
		t.Fatalf("mem avg %+v", got)
	}
	if !got[1].Time.Equal(rows[3].SampledAt) {
		t.Fatalf("time %s", got[1].Time)
	}
}

func TestDownsampleKeepsShortSeries(t *testing.T) {
	rows := []model.HostSample{{CPUPercent: 12, SampledAt: time.Now()}}
	got := Downsample(rows, 180)
	if len(got) != 1 || got[0].CPUPercent != 12 {
		t.Fatalf("%+v", got)
	}
	if len(Downsample(nil, 10)) != 0 {
		t.Fatal("empty")
	}
}
