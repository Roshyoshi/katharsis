//go:build !darwin && !linux

package judge

import (
	"errors"
	"time"
)

// usage is unsupported here; the watchdog stops at the first error, and
// only the post-mortem peak (ru_maxrss) and CPU time are checked.
func usage(pid int) (int64, time.Duration, error) { return 0, 0, errors.ErrUnsupported }

func osRelease() string { return "" }
