//go:build linux

package host

import (
	"bufio"
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

func DefaultDiskPath() string { return "/" }

// Usage 返回路径所在文件系统的用量。路径可以不是挂载点。
func Usage(path string) (Disk, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultDiskPath()
	}
	if _, err := os.Stat(path); err != nil {
		return Disk{}, errors.New("路径不存在")
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Disk{}, errors.New("无法读取磁盘用量")
	}
	return diskFromStat(path, "", st), nil
}

func readCPU() (idle, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, errors.New("无法读取 CPU 信息")
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, errors.New("无法读取 CPU 信息")
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, errors.New("无法读取 CPU 信息")
	}
	nums := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		n, convErr := strconv.ParseUint(field, 10, 64)
		if convErr != nil {
			return 0, 0, errors.New("无法读取 CPU 信息")
		}
		nums = append(nums, n)
	}
	for i, n := range nums {
		// guest 和 guest_nice 已经计入 user、nice。
		if i == 8 || i == 9 {
			continue
		}
		total += n
	}
	idle = nums[3]
	if len(nums) > 4 {
		idle += nums[4]
	}
	return idle, total, nil
}

func readMem() (total, used, swapTotal, swapUsed uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, 0, errors.New("无法读取内存信息")
	}
	defer f.Close()
	vals := map[string]uint64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		n, convErr := strconv.ParseUint(fields[1], 10, 64)
		if convErr != nil {
			continue
		}
		vals[strings.TrimSuffix(fields[0], ":")] = n * 1024
	}
	total = vals["MemTotal"]
	if total == 0 {
		return 0, 0, 0, 0, errors.New("无法读取内存信息")
	}
	avail := vals["MemAvailable"]
	if avail == 0 {
		avail = vals["MemFree"] + vals["Buffers"] + vals["Cached"]
	}
	if avail > total {
		avail = total
	}
	used = total - avail
	swapTotal = vals["SwapTotal"]
	swapFree := vals["SwapFree"]
	if swapFree > swapTotal {
		swapFree = swapTotal
	}
	swapUsed = swapTotal - swapFree
	return total, used, swapTotal, swapUsed, nil
}

func readLoad() (float64, float64, float64) {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15
}

func readUptime() float64 {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func readDisks() []Disk {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		if disk, err := Usage(DefaultDiskPath()); err == nil {
			return []Disk{disk}
		}
		return nil
	}
	defer f.Close()
	seen := map[string]Disk{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		fsType := fields[2]
		if skipFS[fsType] {
			continue
		}
		mount := unescapeMount(fields[1])
		if skipMount(mount) {
			continue
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err != nil {
			continue
		}
		disk := diskFromStat(mount, fsType, st)
		if disk.Total == 0 {
			continue
		}
		if prev, ok := seen[mount]; ok && prev.Total >= disk.Total {
			continue
		}
		seen[mount] = disk
	}
	disks := make([]Disk, 0, len(seen))
	for _, disk := range seen {
		disks = append(disks, disk)
	}
	sort.Slice(disks, func(i, j int) bool {
		if disks[i].Path == "/" {
			return true
		}
		if disks[j].Path == "/" {
			return false
		}
		return disks[i].Path < disks[j].Path
	})
	if len(disks) > 24 {
		disks = disks[:24]
	}
	if len(disks) == 0 {
		if disk, err := Usage(DefaultDiskPath()); err == nil {
			return []Disk{disk}
		}
	}
	return disks
}

func diskFromStat(path, fsType string, st syscall.Statfs_t) Disk {
	bsize := uint64(st.Bsize)
	if st.Bsize <= 0 {
		bsize = 1
	}
	total := st.Blocks * bsize
	free := st.Bavail * bsize
	if free > total {
		free = total
	}
	used := total - free
	return Disk{
		Path:    path,
		FSType:  fsType,
		Total:   total,
		Used:    used,
		Free:    free,
		Percent: percentOf(used, total),
	}
}

var skipFS = map[string]bool{
	"proc": true, "sysfs": true, "devtmpfs": true, "devpts": true, "tmpfs": true,
	"cgroup": true, "cgroup2": true, "pstore": true, "bpf": true, "tracefs": true,
	"debugfs": true, "securityfs": true, "configfs": true, "fusectl": true,
	"mqueue": true, "hugetlbfs": true, "autofs": true, "binfmt_misc": true,
	"nsfs": true, "ramfs": true, "rpc_pipefs": true, "nfsd": true, "efivarfs": true,
	"selinuxfs": true,
}

func skipMount(path string) bool {
	switch {
	case path == "/dev" || strings.HasPrefix(path, "/dev/"):
		return true
	case strings.HasPrefix(path, "/proc"), strings.HasPrefix(path, "/sys"):
		return true
	case strings.HasPrefix(path, "/run"):
		return true
	case strings.HasPrefix(path, "/snap"):
		return true
	default:
		return false
	}
}

func unescapeMount(s string) string {
	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return replacer.Replace(s)
}
