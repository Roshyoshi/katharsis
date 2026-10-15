package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// privateTmux starts a tmux server of the test's own and points tmuxArgs at
// it, returning the id of its only pane. The server is killed at cleanup.
//
// The user may be running tmux right now. Every call here names the private
// socket with -L, and on top of that $TMUX is cleared and $TMUX_TMPDIR moved
// to a scratch dir, so a call that lost its -L would find no server at all
// rather than the user's.
func privateTmux(t *testing.T) (pane string) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}
	// Not t.TempDir: macOS caps socket paths at 104 bytes.
	tmp, err := os.MkdirTemp("/tmp", "dctmux")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", tmp)
	// Panes inherit the server's environment; a pane that misses its -c
	// lands in $HOME, so make that a scratch dir too.
	t.Setenv("HOME", tmp)

	sock := fmt.Sprintf("katharsis-e-%d", os.Getpid())
	old := tmuxArgs
	tmuxArgs = []string{"-L", sock}
	t.Cleanup(func() { tmuxArgs = old })

	// -f /dev/null: none of the user's tmux.conf (plugins, hooks). The
	// first pane runs sleep rather than the user's shell and its rc files.
	start := exec.Command("tmux", "-L", sock, "-f", "/dev/null",
		"new-session", "-d", "-s", "t", "-x", "200", "-y", "50", "sleep 300")
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start tmux: %v\n%s", err, out)
	}
	// Registered after the Setenvs, so it runs before they are undone.
	t.Cleanup(func() {
		if out, err := exec.Command("tmux", "-L", sock, "kill-server").CombinedOutput(); err != nil {
			t.Errorf("kill tmux: %v\n%s", err, out)
		}
	})

	pane, err = tmux("display-message", "-p", "-t", "t:", "#{pane_id}")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX_PANE", pane)
	return pane
}

type paneInfo struct {
	left, width int
	active      bool
}

func panes(t *testing.T) map[string]paneInfo {
	t.Helper()
	out, err := tmux("list-panes", "-t", "t:", "-F", "#{pane_id} #{pane_left} #{pane_width} #{pane_active}")
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]paneInfo{}
	for line := range strings.Lines(out) {
		var id string
		var p paneInfo
		var active int
		if _, err := fmt.Sscan(line, &id, &p.left, &p.width, &active); err != nil {
			t.Fatalf("list-panes line %q: %v", line, err)
		}
		p.active = active == 1
		m[id] = p
	}
	return m
}

// waitFor polls cond until it holds or patience runs out.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(patience); !cond(); {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSplitEditor(t *testing.T) {
	orig := privateTmux(t)
	// "##" checks that -c is escaped: tmux would expand it to "#", miss the
	// directory and start the pane in $HOME instead.
	dir := filepath.Join(t.TempDir(), "door##4")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Stands in for vim: records its cwd and file argument ($0), then idles.
	// The record goes to an absolute path, so a wrong cwd writes nothing
	// anywhere else.
	probe := filepath.Join(t.TempDir(), "probe")
	q := shellQuote(probe)
	editor := []string{"sh", "-c", fmt.Sprintf(`{ pwd -P; echo "$0"; } > %s.tmp && mv %[1]s.tmp %[1]s; exec sleep 30`, q)}
	id, err := SplitEditor(dir, "sol.cpp", editor, 60)
	if err != nil {
		t.Fatal(err)
	}

	ps := panes(t)
	ed, ok := ps[id]
	if !ok {
		t.Fatalf("pane %s not in %v", id, ps)
	}
	if ed.left >= ps[orig].left {
		t.Errorf("editor pane left=%d, game pane left=%d; want editor on the left", ed.left, ps[orig].left)
	}
	if ed.width < 119 || ed.width > 121 {
		t.Errorf("editor pane is %d of 200 columns, want 60%%", ed.width)
	}
	if !ed.active || ps[orig].active {
		t.Errorf("focus: editor active=%v, game active=%v", ed.active, ps[orig].active)
	}

	waitFor(t, "the editor to start", func() bool { _, err := os.Stat(probe); return err == nil })
	got, err := os.ReadFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := filepath.EvalSymlinks(dir) // pwd -P gives /private/var/...
	if err != nil {
		t.Fatal(err)
	}
	if want := cwd + "\nsol.cpp\n"; string(got) != want {
		t.Errorf("editor saw cwd and file %q, want %q", got, want)
	}

	if !PaneAlive(id) || !PaneAlive(orig) {
		t.Fatalf("PaneAlive: editor %v, game %v; want both alive", PaneAlive(id), PaneAlive(orig))
	}
	if _, err := tmux("kill-pane", "-t", id); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "PaneAlive to go false", func() bool { return !PaneAlive(id) })
	if !PaneAlive(orig) {
		t.Error("killing the editor pane took the game pane with it")
	}
}

// With remain-on-exit the pane outlives its command; that is not alive, and
// KillPane still closes it.
func TestDeadPane(t *testing.T) {
	privateTmux(t)
	if _, err := tmux("set-option", "-w", "-t", "t:", "remain-on-exit", "on"); err != nil {
		t.Fatal(err)
	}
	id, err := SplitEditor(t.TempDir(), "sol.cpp", []string{"sh", "-c", "exit 0"}, 50)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "PaneAlive to go false", func() bool { return !PaneAlive(id) })
	if dead, err := tmux("display-message", "-p", "-t", id, "#{pane_id} #{pane_dead}"); err != nil || dead != id+" 1" {
		t.Fatalf("pane %s should remain, dead: got %q, %v", id, dead, err)
	}
	if err := KillPane(id); err != nil {
		t.Fatal(err)
	}
	if n := len(panes(t)); n != 1 {
		t.Errorf("%d panes after KillPane of the dead pane, want 1", n)
	}
}

func TestPaneAliveMissing(t *testing.T) {
	privateTmux(t)
	for _, id := range []string{"", "%999"} {
		if PaneAlive(id) {
			t.Errorf("PaneAlive(%q) = true", id)
		}
	}
}

func TestSplitEditorErrors(t *testing.T) {
	privateTmux(t)
	dir := t.TempDir()
	file := filepath.Join(t.TempDir(), "f")
	write(t, file, "")
	sh := []string{"sh", "-c", "sleep 30"}
	for _, tc := range []struct {
		name   string
		dir    string
		editor []string
		pct    int
	}{
		{"no editor", dir, nil, 60},
		{"width 0", dir, sh, 0},
		{"width 100", dir, sh, 100},
		{"missing dir", filepath.Join(dir, "nope"), sh, 60},
		{"dir is a file", file, sh, 60},
	} {
		if id, err := SplitEditor(tc.dir, "sol.cpp", tc.editor, tc.pct); err == nil {
			t.Errorf("%s: got pane %s, want an error", tc.name, id)
		}
	}
	t.Setenv("TMUX_PANE", "")
	if id, err := SplitEditor(dir, "sol.cpp", sh, 60); err == nil {
		t.Errorf("no TMUX_PANE: got pane %s, want an error", id)
	}
	if n := len(panes(t)); n != 1 {
		t.Errorf("%d panes after failed splits, want 1", n)
	}
}

func TestExecArgs(t *testing.T) {
	old := tmuxArgs
	t.Cleanup(func() { tmuxArgs = old })

	tmuxArgs = nil
	got := execArgs("katharsis", []string{"/bin/katharsis", "-seed", "8f3a"})
	want := []string{"tmux", "-L", "katharsis", "new-session", "-A", "-s", "katharsis", "--", "/bin/katharsis", "-seed", "8f3a"}
	if !slices.Equal(got, want) {
		t.Errorf("execArgs = %q\nwant      %q", got, want)
	}

	// A test's -L comes after ours, and tmux keeps the last.
	tmuxArgs = []string{"-L", "private"}
	got = execArgs("katharsis", []string{"/Applications/My Games/dead cases"})
	want = []string{"tmux", "-L", "katharsis", "-L", "private", "new-session", "-A", "-s", "katharsis", "--", `'/Applications/My Games/dead cases'`}
	if !slices.Equal(got, want) {
		t.Errorf("execArgs = %q\nwant      %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	for _, s := range []string{"plain", "with space", "it's", `$HOME "x" \n`, ""} {
		out, err := exec.Command("sh", "-c", "printf %s "+shellQuote(s)).Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != s {
			t.Errorf("sh read %s as %q, want %q", shellQuote(s), out, s)
		}
	}
}
