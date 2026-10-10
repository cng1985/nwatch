//go:build unix

package shell

import (
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

type unixTerm struct {
	file *os.File
}

func (t *unixTerm) Read(p []byte) (int, error)  { return t.file.Read(p) }
func (t *unixTerm) Write(p []byte) (int, error) { return t.file.Write(p) }
func (t *unixTerm) Close() error                { return t.file.Close() }
func (t *unixTerm) SetReadDeadline(deadline time.Time) error {
	return t.file.SetReadDeadline(deadline)
}
func (t *unixTerm) Resize(cols, rows int) error {
	return pty.Setsize(t.file, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func startPTY(cmd *exec.Cmd, cols, rows int) (terminal, error) {
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	return &unixTerm{file: file}, nil
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
