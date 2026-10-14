package flavor

import (
	"fmt"

	"deadcases/internal/game"
)

// PanicReason is the text after "Kernel panic - not syncing: " on the crash
// screen, before ` in "<title>"`. Where a real Linux panic fits the crash
// it is quoted verbatim (kernel/exit.c, kernel/watchdog.c, mm/oom_kill.c,
// arch/x86/kernel/dumpstack.c); :q! is init taking SIGKILL, hence exit
// code 9.
func PanicReason(cause game.Outcome, verdict string) string {
	switch cause {
	case game.TimedOut:
		return "timer expired"
	case game.GaveUp:
		return "Attempted to kill init! exitcode=0x00000009"
	case game.OutOfLives:
		switch normVerdict(verdict) {
		case "TLE":
			return "softlockup: hung tasks"
		case "MLE":
			return "System is deadlocked on memory"
		case "RE":
			return "Fatal exception"
		}
		return "out of lives"
	}
	return "unknown cause " + string(cause)
}

// frames are real Linux symbols, innermost first, from the paths a kernel
// takes for the matching failure: a timer firing, the soft-lockup
// watchdog, the OOM killer, a page fault oops, and exit through a fatal
// signal. They were checked against torvalds/linux master in 2026-10.
var (
	framesTimer = []string{"it_real_fn", "__hrtimer_run_queues", "hrtimer_interrupt"}
	framesTLE   = []string{"watchdog_timer_fn", "__hrtimer_run_queues", "hrtimer_interrupt"}
	framesMLE   = []string{"oom_kill_process", "out_of_memory", "__alloc_pages_slowpath"}
	framesRE    = []string{"page_fault_oops", "do_user_addr_fault", "exc_page_fault"}
	framesQuit  = []string{"panic", "do_exit", "do_group_exit", "get_signal"}
	framesExit  = []string{"do_exit", "do_group_exit", "get_signal"}
)

// CallTrace returns kernel-style frames ("oom_kill_process+0x1d3/0x2a0")
// for the top of the crash screen's Call Trace, above the screen's own
// door and boot frames. The verdict picks the path when it explains the
// crash; a :q! always reads as a kill. Offsets are stable for a seed.
func CallTrace(cause game.Outcome, verdict string, seed uint64) []string {
	var syms []string
	switch v := normVerdict(verdict); {
	case cause == game.GaveUp:
		syms = framesQuit
	case v == "TLE":
		syms = framesTLE
	case v == "MLE":
		syms = framesMLE
	case v == "RE":
		syms = framesRE
	case cause == game.TimedOut:
		syms = framesTimer
	default:
		syms = framesExit
	}
	out := make([]string, len(syms))
	x := seed
	for i, s := range syms {
		x = splitmix64(x)
		size := 0x40 + x%0x3c0      // functions of 64 B to 1 KiB
		off := 1 + (x>>20)%(size-1) // a return address inside it
		out[i] = fmt.Sprintf("%s+0x%x/0x%x", s, off, size)
	}
	return out
}

// splitmix64 is one step of Vigna's SplitMix64: a cheap, well-mixed
// sequence from any seed, including 0.
func splitmix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}
