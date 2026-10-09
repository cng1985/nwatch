package host

import (
	"testing"
)

func TestSnapshotReadsLocalResources(t *testing.T) {
	snap, err := NewCollector().Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.MemTotal == 0 || snap.CPUCount == 0 {
		t.Fatalf("%+v", snap)
	}
	if snap.CPUPercent < 0 || snap.CPUPercent > 100 || snap.MemPercent < 0 || snap.MemPercent > 100 {
		t.Fatalf("percent cpu=%v mem=%v", snap.CPUPercent, snap.MemPercent)
	}
	if len(snap.Disks) == 0 {
		t.Fatal("no disks")
	}
	foundRoot := false
	for _, disk := range snap.Disks {
		if disk.Total == 0 || disk.Percent < 0 || disk.Percent > 100 {
			t.Fatalf("disk %+v", disk)
		}
		if disk.Path == "/" {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Fatalf("missing root: %+v", snap.Disks)
	}
	second, err := NewCollector().Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if second.MemTotal != snap.MemTotal {
		t.Fatalf("mem total changed %d %d", snap.MemTotal, second.MemTotal)
	}
}

func TestUsageOfRoot(t *testing.T) {
	disk, err := Usage("/")
	if err != nil {
		t.Fatal(err)
	}
	if disk.Path != "/" || disk.Total == 0 {
		t.Fatalf("%+v", disk)
	}
	if _, err := Usage("/this/path/does/not/exist"); err == nil {
		t.Fatal("expected missing path")
	}
}
