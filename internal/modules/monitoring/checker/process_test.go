package checker

import (
	"context"
	"github.com/cng1985/nwatch/internal/modules/monitoring/host"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"testing"
)

type fakeProcs struct {
	items []host.Process
	err   error
}

func (f fakeProcs) AllProcesses() ([]host.Process, error) { return f.items, f.err }

func TestProcessCheckerStopped(t *testing.T) {
	c := &ProcessChecker{host: fakeProcs{items: []host.Process{{PID: 1, Name: "systemd", State: "S"}}}}
	res, err := c.Check(context.Background(), &shared.Monitor{Host: "nginx", Match: shared.MatchName})
	if err != nil || res.Success || res.Status != "STOPPED" || res.Message == "" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestProcessCheckerCPUAndMemory(t *testing.T) {
	items := []host.Process{
		{PID: 10, Name: "nginx", Command: "nginx: master", State: "S", CPUPercent: 12, MemBytes: 40 << 20},
		{PID: 11, Name: "nginx", Command: "nginx: worker", State: "S", CPUPercent: 86, MemBytes: 80 << 20},
		{PID: 12, Name: "nginx", Command: "nginx: worker", State: "Z", CPUPercent: 99, MemBytes: 1 << 30},
		{PID: 13, Name: "java", Command: "java -jar other.jar", State: "S", CPUPercent: 99, MemBytes: 1 << 30},
	}
	ok := evaluateProcess(&shared.Monitor{Host: "nginx", Match: shared.MatchName, Threshold: 90, MemThreshold: 512}, items, 3)
	if !ok.Success || ok.Metadata["count"] != 2 {
		t.Fatalf("running %+v", ok)
	}
	if ok.Metadata["cpuPercent"].(float64) != 86 {
		t.Fatalf("cpu %+v", ok.Metadata)
	}

	highCPU := evaluateProcess(&shared.Monitor{Host: "nginx", Match: shared.MatchName, Threshold: 80}, items, 3)
	if highCPU.Success || highCPU.Status != "HIGH" {
		t.Fatalf("cpu %+v", highCPU)
	}
	highMem := evaluateProcess(&shared.Monitor{Host: "nginx", Match: shared.MatchName, MemThreshold: 100}, items, 3)
	if highMem.Success || highMem.Status != "HIGH" {
		t.Fatalf("mem %+v", highMem)
	}

	byCommand := evaluateProcess(&shared.Monitor{Host: "nginx", Command: "nginx: master", Match: shared.MatchCommand, Threshold: 50}, items, 3)
	if !byCommand.Success || byCommand.Metadata["count"] != 1 {
		t.Fatalf("command %+v", byCommand)
	}
}

func TestProcessCheckerReadError(t *testing.T) {
	c := &ProcessChecker{host: fakeProcs{err: errString("无法读取进程")}}
	res, err := c.Check(context.Background(), &shared.Monitor{Host: "nginx"})
	if err != nil || res.Success || res.Status != "ERROR" {
		t.Fatalf("%+v %v", res, err)
	}
}
