//go:build !unix

package shell

import (
	"errors"
	"os"
	"os/exec"
)

func startPTY(cmd *exec.Cmd, cols, rows int) (*os.File, error) {
	return nil, errors.New("在线终端仅支持 Linux")
}

func resizePTY(f *os.File, cols, rows int) error {
	return errors.New("在线终端仅支持 Linux")
}

func killProcess(pid int) error { return nil }
