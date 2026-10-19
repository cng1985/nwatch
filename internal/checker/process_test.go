package checker

import (
	"context"
	"testing"

	"github.com/cng1985/nwatch/internal/host"
	"github.com/cng1985/nwatch/internal/model"
)

type fakeProcs struct {
	items []host.Process
	err   error
}

func (f fakeProcs) AllProcesses() ([]host.Process, error) { return f.items, f.err }

func TestProcessCheckerStopped(t *testing.T) {
	c := &ProcessChecker{host: fakeProcs{items: []host.Process{{PID: 1, Name: "systemd", State: "S"}}}}
	res, err := c.Check(context.Background(), &model.Monitor{Host: "nginx", Match: model.MatchName})
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
	ok := evaluateProcess(&model.Monitor{Host: "nginx", Match: model.MatchName, Threshold: 90, MemThreshold: 512}, items, 3)
	if !ok.Success || ok.Metadata["count"] != 2 {
		t.Fatalf("running %+v", ok)
	}
	if ok.Metadata["cpuPercent"].(float64) != 86 {
		t.Fatalf("cpu %+v", ok.Metadata)
	}

	highCPU := evaluateProcess(&model.Monitor{Host: "nginx", Match: model.MatchName, Threshold: 80}, items, 3)
	if highCPU.Success || highCPU.Status != "HIGH" {
		t.Fatalf("cpu %+v", highCPU)
	}
	highMem := evaluateProcess(&model.Monitor{Host: "nginx", Match: model.MatchName, MemThreshold: 100}, items, 3)
	if highMem.Success || highMem.Status != "HIGH" {
		t.Fatalf("mem %+v", highMem)
	}

	byCommand := evaluateProcess(&model.Monitor{Host: "nginx", Command: "nginx: master", Match: model.MatchCommand, Threshold: 50}, items, 3)
	if !byCommand.Success || byCommand.Metadata["count"] != 1 {
		t.Fatalf("command %+v", byCommand)
	}
}

func TestProcessCheckerReadError(t *testing.T) {
	c := &ProcessChecker{host: fakeProcs{err: errString("无法读取进程")}}
	res, err := c.Check(context.Background(), &model.Monitor{Host: "nginx"})
	if err != nil || res.Success || res.Status != "ERROR" {
		t.Fatalf("%+v %v", res, err)
	}
}
