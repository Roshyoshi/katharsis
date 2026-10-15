package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
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

	n, err := SweepOrphans(patience)
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

// A session whose game died is told apart from one whose game runs, so a
// relaunch starts a new game in it instead of attaching to the orphan
// editor alone (qa/chaos r03); newWindow carries this process's
// environment, not the server's.
func TestGoneGame(t *testing.T) {
	privateTmux(t)
	if _, err := tmux("new-session", "-d", "-s", "katharsis", "sleep", "300"); err != nil {
		t.Fatal(err)
	}
	if gone("katharsis") {
		t.Error("a session with no game pid is gone")
	}
	if gone("nosuchsession") {
		t.Error("a missing session is gone")
	}
	mark := func(pid int) {
		t.Helper()
		if _, err := tmux("set-option", "-t", "katharsis:", pidOption, strconv.Itoa(pid)); err != nil {
			t.Fatal(err)
		}
	}
	mark(os.Getppid())
	if gone("katharsis") {
		t.Error("a session whose game runs is gone")
	}
	mark(deadPid(t))
	if !gone("katharsis") {
		t.Error("a session whose game died is not gone")
	}

	out := filepath.Join(t.TempDir(), "env")
	t.Setenv("KATHARSIS_TEST_ENV", "from the relaunch")
	if err := newWindow("katharsis", []string{"sh", "-c", `printf %s "$KATHARSIS_TEST_ENV" >` + shellQuote(out)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the new window's command", func() bool {
		b, _ := os.ReadFile(out)
		return string(b) == "from the relaunch"
	})
}

// MarkGame sets the pid only in a session ExecInTmux made.
func TestMarkGame(t *testing.T) {
	pane := privateTmux(t)
	t.Setenv("TMUX", "/tmp/private,1,0") // InTmux; calls still go by tmuxArgs
	show := func() string {
		t.Helper()
		v, err := tmux("show-options", "-v", "-q", "-t", pane, pidOption)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	t.Setenv(ownSessionEnv, "")
	if err := MarkGame(); err != nil || show() != "" {
		t.Fatalf("MarkGame outside its own session = %v, set %q", err, show())
	}
	t.Setenv(ownSessionEnv, "1")
	if err := MarkGame(); err != nil || show() != strconv.Itoa(os.Getpid()) {
		t.Fatalf("MarkGame = %v, set %q, want %d", err, show(), os.Getpid())
	}
}
