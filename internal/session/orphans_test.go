package session

import (
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// deadPid is the pid of a process that has exited and been reaped.
func deadPid(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func tagAs(t *testing.T, pane string, pid int, role string) {
	t.Helper()
	for opt, v := range map[string]string{gameOption: strconv.Itoa(pid), roleOption: role} {
		if _, err := tmux("set-option", "-p", "-t", pane, opt, v); err != nil {
			t.Fatal(err)
		}
	}
}

// A game killed mid-door leaves its vim, with unsaved edits, and lldb
// (qa/chaos r04). The next game's sweep saves and closes the vim, so the
// resumed door opens the one file without a swap-file warning, and kills
// lldb. Panes of a game still running, and untagged panes, stay.
func TestSweepOrphans(t *testing.T) {
	vim := realVim(t)
	privateTmux(t)
	path := solution(t)
	ed, err := tmux("new-window", "-P", "-F", "#{pane_id}", "--", vim, "-u", "NONE", "-i", "NONE", "-N", path)
	if err != nil {
		t.Fatal(err)
	}
	waitScreen(t, ed, "sol.cpp")
	keys(t, ed, "ounsaved")
	waitScreen(t, ed, "-- INSERT --")
	lldb, err := tmux("new-window", "-P", "-F", "#{pane_id}", "--", "sleep", "300")
	if err != nil {
		t.Fatal(err)
	}
	live, err := tmux("new-window", "-P", "-F", "#{pane_id}", "--", "sleep", "300")
	if err != nil {
		t.Fatal(err)
	}
	// The test binary's own pid would be skipped as this game's; its
	// parent (go test) is a game still running.
	dead := deadPid(t)
	tagAs(t, ed, dead, RoleEditor)
	tagAs(t, lldb, dead, RoleLLDB)
	tagAs(t, live, os.Getppid(), RoleLLDB)

	n, err := SweepOrphans(5 * time.Second)
	if err != nil || n != 2 {
		t.Fatalf("SweepOrphans = %d, %v; want the 2 orphans", n, err)
	}
	all, err := tmux("list-panes", "-a", "-F", "#{pane_id}")
	if err != nil {
		t.Fatal(err)
	}
	have := strings.Fields(all)
	for pane, want := range map[string]bool{ed: false, lldb: false, live: true} {
		if got := slices.Contains(have, pane); got != want {
			t.Errorf("pane %s there = %v after the sweep, want %v", pane, got, want)
		}
	}
	if b, _ := os.ReadFile(path); string(b) != "int main() {}\nunsaved\n" {
		t.Errorf("file = %q, want the orphan's edit saved", b)
	}
}
