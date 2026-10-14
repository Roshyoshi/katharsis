//go:build !darwin && !linux

package judge

import "errors"

// rss is unsupported here; the memory watchdog stops at the first error and
// only the post-mortem peak (ru_maxrss) is checked.
func rss(pid int) (int64, error) { return 0, errors.ErrUnsupported }

func osRelease() string { return "" }
