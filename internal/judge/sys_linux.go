package judge

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// children and exePath find a dead judge's runs on darwin only: here a
// dead parent's children may go to a subreaper (systemd --user) instead
// of pid 1, so a parent of 1 does not mark them.
func children(ppid int) []int { return nil }

func exePath(pid int) string { return "" }

func osRelease() string {
	b, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return strings.TrimSpace(string(b))
}

// usage reads a live process's resident size from /proc/<pid>/statm, whose
// second field counts resident pages, and its CPU time from
// /proc/<pid>/stat, whose utime and stime fields count USER_HZ ticks (100
// per second on every Linux the kernel ABI still allows).
func usage(pid int) (mem int64, cpu time.Duration, err error) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/statm")
	if err != nil {
		return 0, 0, err
	}
	var size, resident int64
	if _, err := fmt.Sscan(string(b), &size, &resident); err != nil {
		return 0, 0, err
	}
	mem = resident * int64(os.Getpagesize())

	st, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, 0, err
	}
	// The command name in parentheses may hold spaces; fields count from
	// after its closing parenthesis, where field 3 (state) starts.
	i := bytes.LastIndexByte(st, ')')
	if i < 0 {
		return 0, 0, errors.New("malformed /proc stat")
	}
	f := strings.Fields(string(st[i+1:]))
	if len(f) < 13 {
		return 0, 0, errors.New("malformed /proc stat")
	}
	utime, err1 := strconv.ParseInt(f[11], 10, 64) // field 14
	stime, err2 := strconv.ParseInt(f[12], 10, 64) // field 15
	if err1 != nil || err2 != nil {
		return 0, 0, errors.New("malformed /proc stat")
	}
	const userHZ = 100
	return mem, time.Duration(utime+stime) * time.Second / userHZ, nil
}
