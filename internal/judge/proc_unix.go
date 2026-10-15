//go:build unix

package judge

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
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
