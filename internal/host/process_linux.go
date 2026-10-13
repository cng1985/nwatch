//go:build linux

package host

import (
	"errors"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func readProcSnapshot() (procSample, error) {
	_, total, err := readCPU()
	if err != nil {
		return procSample{}, err
	}
	memTotal, _, _, _, err := readMem()
	if err != nil {
		return procSample{}, err
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return procSample{}, errors.New("无法读取进程列表")
	}
	page := uint64(os.Getpagesize())
	if page == 0 {
		page = 4096
	}
	users := map[uint32]string{}
	procs := make([]rawProc, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, convErr := strconv.Atoi(entry.Name())
		if convErr != nil || pid <= 0 {
			continue
		}
		proc, ok := readOneProc(pid, page, users)
		if !ok {
			continue
		}
		procs = append(procs, proc)
	}
	return procSample{Total: total, MemTotal: memTotal, At: time.Now(), Procs: procs}, nil
}

func readOneProc(pid int, pageSize uint64, users map[uint32]string) (rawProc, bool) {
	id := strconv.Itoa(pid)
	raw, err := os.ReadFile("/proc/" + id + "/stat")
	if err != nil {
		return rawProc{}, false
	}
	name, state, ppid, ticks, rssPages, ok := parseProcStat(string(raw))
	if !ok {
		return rawProc{}, false
	}
	cmd := readCmdline(id)
	if cmd == "" {
		cmd = name
	}
	uid := procUID(id)
	nameUser, seen := users[uid]
	if !seen {
		nameUser = lookupUser(uid)
		users[uid] = nameUser
	}
	return rawProc{
		PID:     pid,
		PPID:    ppid,
		User:    nameUser,
		Name:    strings.TrimSpace(name),
		Command: cmd,
		State:   state,
		RSS:     rssPages * pageSize,
		Ticks:   ticks,
	}, true
}

func readCmdline(pid string) string {
	raw, err := os.ReadFile("/proc/" + pid + "/cmdline")
	if err != nil || len(raw) == 0 {
		return ""
	}
	return cleanCommand(string(raw))
}

func procUID(pid string) uint32 {
	info, err := os.Stat("/proc/" + pid)
	if err != nil {
		return 0
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return st.Uid
}

func lookupUser(uid uint32) string {
	id := strconv.FormatUint(uint64(uid), 10)
	account, err := user.LookupId(id)
	if err != nil || account.Username == "" {
		return id
	}
	return account.Username
}
