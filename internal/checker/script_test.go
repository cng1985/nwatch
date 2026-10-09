//go:build unix

package checker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

func TestScriptSuccessAndFailure(t *testing.T) {
	ok := Execute(context.Background(), "echo hello-script", "", 5*time.Second)
	success, msg := ok.Judge("", "")
	if !success || !strings.Contains(msg, "hello-script") || ok.ExitCode != 0 {
		t.Fatalf("%+v %s", ok, msg)
	}
	bad := Execute(context.Background(), "echo missing-token", "", 5*time.Second)
	success, msg = bad.Judge("expected-token", "")
	if success || msg != "输出未包含指定内容" {
		t.Fatalf("%+v %s", bad, msg)
	}
	failed := Execute(context.Background(), "echo boom >&2; exit 3", "", 5*time.Second)
	success, msg = failed.Judge("", "")
	if success || failed.ExitCode != 3 || !strings.Contains(msg, "boom") {
		t.Fatalf("%+v %s", failed, msg)
	}
}

func TestScriptTimeoutKillsProcess(t *testing.T) {
	start := time.Now()
	out := Execute(context.Background(), "sleep 30", "", time.Second)
	if !out.TimedOut || time.Since(start) > 5*time.Second {
		t.Fatalf("elapsed %s %+v", time.Since(start), out)
	}
	_, msg := out.Judge("", "")
	if msg != "脚本执行超时" {
		t.Fatal(msg)
	}
}

func TestScriptCheckerRecordsOutput(t *testing.T) {
	res, err := NewScriptChecker().Check(context.Background(), &model.Monitor{
		Command: "echo line", Timeout: 5, BodyContains: "line",
	})
	if err != nil || !res.Success {
		t.Fatalf("%+v %v", res, err)
	}
	if !strings.Contains(res.Metadata["stdout"].(string), "line") {
		t.Fatalf("stdout %#v", res.Metadata["stdout"])
	}
}

func TestValidateScript(t *testing.T) {
	if _, err := ValidateScript("echo ok", "/tmp", 5, true); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateScript("echo ok", "relative", 5, false); err == nil {
		t.Fatal("relative dir")
	}
	if _, err := ValidateScript("  ", "", 5, false); err == nil {
		t.Fatal("empty")
	}
}
