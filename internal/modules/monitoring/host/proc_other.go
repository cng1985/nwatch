//go:build !linux && !windows

package host

import "errors"

func DefaultDiskPath() string { return "/" }

func Usage(string) (Disk, error) {
	return Disk{}, errors.New("当前系统不支持磁盘监控")
}

func readCPU() (uint64, uint64, error) {
	return 0, 0, errors.New("当前系统不支持 CPU 监控")
}

func readMem() (uint64, uint64, uint64, uint64, error) {
	return 0, 0, 0, 0, errors.New("当前系统不支持内存监控")
}

func readLoad() (float64, float64, float64) { return 0, 0, 0 }

func readUptime() float64 { return 0 }

func readDisks() []Disk { return nil }
