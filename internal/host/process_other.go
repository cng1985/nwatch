//go:build !linux

package host

import "errors"

func readProcSnapshot() (procSample, error) {
	return procSample{}, errors.New("当前系统不支持进程查看")
}
