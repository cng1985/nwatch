//go:build windows

package checker

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

func startScript(ctx context.Context, command, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "cmd", "/C", command)
	cmd.Dir = workDir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	cmd.WaitDelay = time.Second
	return cmd
}
