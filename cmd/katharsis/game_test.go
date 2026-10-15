package main

import (
	"strings"
	"testing"
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
