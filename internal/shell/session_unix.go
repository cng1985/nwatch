//go:build unix

package shell

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

func startPTY(cmd *exec.Cmd, cols, rows int) (*os.File, error) {
	return pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func resizePTY(f *os.File, cols, rows int) error {
	return pty.Setsize(f, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func killProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if err != nil {
		return syscall.Kill(pid, syscall.SIGKILL)
	}
	return nil
}
