package judge

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func osRelease() string {
	b, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return strings.TrimSpace(string(b))
}

// rss reads a live process's resident size from /proc/<pid>/statm, whose
// second field counts resident pages.
func rss(pid int) (int64, error) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/statm")
	if err != nil {
		return 0, err
	}
	var size, resident int64
	if _, err := fmt.Sscan(string(b), &size, &resident); err != nil {
		return 0, err
	}
	return resident * int64(os.Getpagesize()), nil
}
