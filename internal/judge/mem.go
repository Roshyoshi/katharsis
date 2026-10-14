package judge

import (
	"os"
	"time"
)

// HardMemCap bounds every run that has no memory limit of its own: sanitizer
// builds (their shadow memory makes the problem's limit meaningless) and
// runs with Limits.Mem unset. It is there so a runaway allocation cannot eat
// the machine, not to judge anything.
var HardMemCap int64 = 4 << 30

// memPoll is how often the watchdog samples a run. A fresh page faults in at
// a few GB/s at most, so a runaway is killed within a few hundred MB of its
// cap.
const memPoll = 20 * time.Millisecond

// memCap is the size past which the watchdog kills a run. macOS rejects
// RLIMIT_AS and RLIMIT_DATA, so nothing else stops a program that allocates
// without bound. The 1.25x margin keeps the watchdog from deciding a close
// call: a run just over its limit finishes and is judged MLE on its peak
// (ru_maxrss), like on a real judge.
func memCap(lim Limits, p Profile) int64 {
	if p.Sanitized || lim.Mem <= 0 {
		return HardMemCap
	}
	return lim.Mem + lim.Mem/4
}

// watchMemory samples proc's memory every memPoll and kills it once it
// grows past limit. stop ends the watch and returns the size that triggered
// the kill, or 0 if there was none. Call stop after Wait: a kill after the
// process is reaped is refused by os.Process (ErrProcessDone), so a reused
// pid is never signalled.
func watchMemory(proc *os.Process, limit int64) (stop func() (killedAt int64)) {
	done := make(chan struct{})
	res := make(chan int64, 1)
	go func() {
		t := time.NewTicker(memPoll)
		defer t.Stop()
		for {
			// An error means the process has exited (ESRCH) or the
			// platform cannot tell; either way there is nothing to watch.
			n, err := rss(proc.Pid)
			if err != nil {
				res <- 0
				return
			}
			if n > limit {
				proc.Kill()
				res <- n
				return
			}
			select {
			case <-done:
				res <- 0
				return
			case <-t.C:
			}
		}
	}()
	return func() int64 {
		close(done)
		return <-res
	}
}
