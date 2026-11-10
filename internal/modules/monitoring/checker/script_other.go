//go:build !unix && !windows

package checker

import (
	"context"
	"os/exec"
)

func startScript(ctx context.Context, command, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	return cmd
}
