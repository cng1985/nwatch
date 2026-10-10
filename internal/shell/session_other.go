//go:build !unix && !windows

package shell

import (
	"os/exec"
)

func startPTY(cmd *exec.Cmd, cols, rows int) (terminal, error) {
	return nil, ErrUnsupported
}

func killProcess(pid int) error { return nil }
