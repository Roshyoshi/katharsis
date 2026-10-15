//go:build unix

package judge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// inGroup starts cmd as the leader of a new process group and makes a
// cancel (a limit hit, a newer save) kill the whole group, so a solution
// that forks cannot leave children behind.
func inGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killGroup(cmd.Process.Pid) }
}

// killGroup SIGKILLs the process group pid leads. After Wait has reaped
// the leader this is still safe while any member lives: a process group's
// id is not handed out as a new pid until the group is empty. Once it is
// empty the kill finds nothing (ESRCH), unless in the microseconds since
// the reap the pid counter wrapped to that very id; the only processes it
// could reach are the player's own.
func killGroup(pid int) error {
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// limitCPU has cmd's program run under an RLIMIT_CPU of limit, rounded up
// to the whole seconds the limit counts in. It is a backstop, not the
// judge's CPU check (watch is): a kill -9 of the game, or a panic off the
// main goroutine, skips the watchdog and every deferred kill, and the run
// it leaves would spin on as launchd's child until reboot. At the limit
// the kernel sends SIGXCPU, which ends a program that does not catch it;
// macOS sends nothing more at the hard limit (Linux sends SIGKILL).
//
// Go sets no limits between fork and exec, so a shell sets this one and
// execs the program in its own place, under the same pid. The run's CPU
// time counts the shell: a judged a+b on an M4 read 0.9 ms more CPU and
// 1.0 ms more wall with dash, 2.0 and 2.4 ms with bash (macOS's sh).
// Should the game run under a lower hard limit, ulimit fails and the
// program runs under that one.
func limitCPU(cmd *exec.Cmd, limit time.Duration) {
	secs := (limit + time.Second - 1) / time.Second
	sh := limitShell()
	script := fmt.Sprintf(`ulimit -t %d; exec "$0" "$@"`, secs)
	cmd.Path, cmd.Args = sh, append([]string{sh, "-c", script}, cmd.Args...)
}

// limitShell is dash where there is one, as on macOS since 10.15.
var limitShell = sync.OnceValue(func() string {
	if _, err := os.Stat("/bin/dash"); err == nil {
		return "/bin/dash"
	}
	return "/bin/sh"
})
