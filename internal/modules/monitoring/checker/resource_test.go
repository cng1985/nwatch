package checker

import (
	"context"
	"github.com/cng1985/nwatch/internal/modules/monitoring/host"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"testing"
)

type fakeHost struct {
	snap host.Snapshot
	err  error
}

func (f fakeHost) Snapshot() (host.Snapshot, error) { return f.snap, f.err }

func TestCPUCheckerThreshold(t *testing.T) {
	high := &CPUChecker{host: fakeHost{snap: host.Snapshot{CPUPercent: 95}}}
	res, err := high.Check(context.Background(), &shared.Monitor{Threshold: 90})
	if err != nil || res.Success {
		t.Fatalf("high %+v %v", res, err)
	}
	low := &CPUChecker{host: fakeHost{snap: host.Snapshot{CPUPercent: 12.2}}}
	res, err = low.Check(context.Background(), &shared.Monitor{Threshold: 90})
	if err != nil || !res.Success || res.Message == "" {
		t.Fatalf("low %+v %v", res, err)
	}
}

func TestMemoryCheckerError(t *testing.T) {
	c := &MemoryChecker{host: fakeHost{err: errHost}}
	res, err := c.Check(context.Background(), &shared.Monitor{})
	if err != nil || res.Success || res.Status != "ERROR" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestDiskCheckerOnTempDir(t *testing.T) {
	res, err := NewDiskChecker().Check(context.Background(), &shared.Monitor{Host: t.TempDir(), Threshold: 100})
	if err != nil || !res.Success {
		t.Fatalf("%+v %v", res, err)
	}
	missing, err := NewDiskChecker().Check(context.Background(), &shared.Monitor{Host: "/this/path/does/not/exist", Threshold: 50})
	if err != nil || missing.Success {
		t.Fatalf("%+v %v", missing, err)
	}
}

var errHost = errString("无法读取")

type errString string

func (e errString) Error() string { return string(e) }
