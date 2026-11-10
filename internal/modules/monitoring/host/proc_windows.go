//go:build windows

package host

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func DefaultDiskPath() string { return `C:\` }

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// Usage 返回路径所在卷的用量。
func Usage(path string) (Disk, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultDiskPath()
	}
	info, err := os.Stat(path)
	if err != nil {
		return Disk{}, errors.New("路径不存在")
	}
	dir := path
	if !info.IsDir() {
		dir = filepath.Dir(path)
	}
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return Disk{}, errors.New("无法读取磁盘用量")
	}
	var avail, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, &free); err != nil {
		return Disk{}, errors.New("无法读取磁盘用量")
	}
	if avail > total {
		avail = total
	}
	used := total - avail
	return Disk{
		Path:    path,
		FSType:  volumeFS(dir),
		Total:   total,
		Used:    used,
		Free:    avail,
		Percent: percentOf(used, total),
	}, nil
}

func readCPU() (idle, total uint64, err error) {
	var idleFT, kernelFT, userFT windows.Filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleFT)),
		uintptr(unsafe.Pointer(&kernelFT)),
		uintptr(unsafe.Pointer(&userFT)),
	)
	if r == 0 {
		return 0, 0, errors.New("无法读取 CPU 信息")
	}
	idle = filetime(idleFT)
	// kernel 时间已经包含 idle。
	total = filetime(kernelFT) + filetime(userFT)
	if total < idle {
		total = idle
	}
	return idle, total, nil
}

func readMem() (total, used, swapTotal, swapUsed uint64, err error) {
	var st memoryStatusEx
	st.Length = uint32(unsafe.Sizeof(st))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&st)))
	if r == 0 || st.TotalPhys == 0 {
		return 0, 0, 0, 0, errors.New("无法读取内存信息")
	}
	total = st.TotalPhys
	avail := st.AvailPhys
	if avail > total {
		avail = total
	}
	used = total - avail
	if st.TotalPageFile > st.TotalPhys {
		swapTotal = st.TotalPageFile - st.TotalPhys
		availSwap := uint64(0)
		if st.AvailPageFile > st.AvailPhys {
			availSwap = st.AvailPageFile - st.AvailPhys
		}
		if availSwap > swapTotal {
			availSwap = swapTotal
		}
		swapUsed = swapTotal - availSwap
	}
	return total, used, swapTotal, swapUsed, nil
}

func readLoad() (float64, float64, float64) { return 0, 0, 0 }

func readUptime() float64 {
	r, _, _ := procGetTickCount64.Call()
	return float64(r) / 1000
}

func readDisks() []Disk {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		if disk, err := Usage(DefaultDiskPath()); err == nil {
			return []Disk{disk}
		}
		return nil
	}
	var disks []Disk
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		switch windows.GetDriveType(p) {
		case windows.DRIVE_FIXED, windows.DRIVE_REMOTE, windows.DRIVE_RAMDISK:
		default:
			continue
		}
		disk, err := Usage(root)
		if err != nil || disk.Total == 0 {
			continue
		}
		disk.Path = root
		disks = append(disks, disk)
		if len(disks) >= 24 {
			break
		}
	}
	if len(disks) == 0 {
		if disk, err := Usage(DefaultDiskPath()); err == nil {
			return []Disk{disk}
		}
	}
	return disks
}

func volumeFS(root string) string {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return ""
	}
	var name [64]uint16
	if err := windows.GetVolumeInformation(p, nil, 0, nil, nil, nil, &name[0], uint32(len(name))); err != nil {
		return ""
	}
	return windows.UTF16ToString(name[:])
}

func filetime(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}
