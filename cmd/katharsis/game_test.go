package main

import (
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A positional argument used to be ignored: `katharsis pratice dir` booted
// the game (and downloaded the catalogs) instead of failing.
func TestRunGameRejectsArgs(t *testing.T) {
	// Should the check regress, fail at the tmux lookup instead of
	// exec'ing tmux over the test.
	t.Setenv("TMUX", "")
	t.Setenv("PATH", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	err := runGame([]string{"pratice", "dir"})
	if err == nil || !strings.Contains(err.Error(), `"pratice"`) {
		t.Fatalf("runGame(pratice dir) = %v, want an unknown command error", err)
	}
}

// Sounds used to be started and never waited on, leaving a zombie per
// sound until the game exited.
func TestStartReaps(t *testing.T) {
	cmd := exec.Command("true")
	start(cmd)
	if cmd.Process == nil {
		t.Fatal("start did not start the command")
	}
	// A zombie still answers signal 0; only a reaped child is gone.
	pid := cmd.Process.Pid
	for deadline := time.Now().Add(5 * time.Second); syscall.Kill(pid, 0) == nil; {
		if time.Now().After(deadline) {
			t.Fatalf("child %d exited but was never reaped", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
