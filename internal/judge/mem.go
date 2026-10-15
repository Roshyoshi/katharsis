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

// memCap is the size past which the watchdog kills a run held to mem bytes
// (none when mem <= 0). macOS rejects RLIMIT_AS and RLIMIT_DATA, so nothing
// else stops a program that allocates without bound. The 1.25x margin keeps
// the watchdog from deciding a close call: a run just over its limit
// finishes and is judged MLE on its peak (ru_maxrss), like on a real judge.
func memCap(mem int64) int64 {
	if mem <= 0 {
		return HardMemCap
	}
	return mem + mem/4
}

// kill says why the watchdog killed a run: Mem is the size that crossed
// the memory cap, CPU is set when the run used up its CPU allowance. Both
// zero means it did not kill.
type kill struct {
	Mem int64
	CPU bool
}

// watch samples proc every memPoll and kills it once its memory grows past
// memLimit or its CPU time past cpuLimit (no CPU limit when <= 0). Killing
// on CPU rather than waiting for the wall clock ends an infinite loop as
// soon as it is over budget instead of at 2x budget plus slack in wall
// time, which on a busy machine is later still.
//
// stop ends the watch and returns what it killed for. Call stop after
// Wait: os.Process refuses to signal a process Wait has reaped
// (ErrProcessDone), which keeps the kill off a reused pid. On darwin the
// refusal starts just after wait4 returns rather than before it, a window
// of a few instructions.
func watch(proc *os.Process, memLimit int64, cpuLimit time.Duration) (stop func() kill) {
	done := make(chan struct{})
	res := make(chan kill, 1)
	go func() {
		t := time.NewTicker(memPoll)
		defer t.Stop()
		for {
			// An error means the process has exited (ESRCH) or the
			// platform cannot tell; either way there is nothing to watch.
			n, cpu, err := usage(proc.Pid)
			if err != nil {
				res <- kill{}
				return
			}
			var k kill
			switch {
			case n > memLimit:
				k.Mem = n
			case cpuLimit > 0 && cpu > cpuLimit:
				k.CPU = true
			}
			if k != (kill{}) {
				_ = proc.Kill() // ErrProcessDone if it exited meanwhile
				res <- k
				return
			}
			select {
			case <-done:
				res <- kill{}
				return
			case <-t.C:
			}
		}
	}()
	return func() kill {
		close(done)
		return <-res
	}
}
