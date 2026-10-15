package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLLDBArgs(t *testing.T) {
	env := []string{"env", "-u", "DEVELOPER_DIR", "-u", "SDKROOT"}
	got := LLDBArgs("/usr/bin/lldb", "/c/bin", `/t/my "tests"/3.in`)
	want := append(slices.Clone(env), "/usr/bin/lldb",
		"-o", `settings set target.input-path "/t/my \"tests\"/3.in"`,
		"-o", "breakpoint set --name main", "--", "/c/bin")
	if !slices.Equal(got, want) {
		t.Errorf("LLDBArgs =\n %q\nwant\n %q", got, want)
	}
	got = LLDBArgs("lldb", "/c/bin", "")
	want = append(slices.Clone(env), "lldb", "-o", "breakpoint set --name main", "--", "/c/bin")
	if !slices.Equal(got, want) {
		t.Errorf("LLDBArgs without stdin = %q, want %q", got, want)
	}
}

type geom struct{ left, top, width, height int }

func paneGeom(t *testing.T, pane string) geom {
	t.Helper()
	out, err := tmux("display-message", "-p", "-t", pane, "#{pane_left} #{pane_top} #{pane_width} #{pane_height}")
	if err != nil {
		t.Fatal(err)
	}
	var g geom
	if _, err := fmt.Sscan(out, &g.left, &g.top, &g.width, &g.height); err != nil {
		t.Fatalf("pane geometry %q: %v", out, err)
	}
	return g
}

func TestSplitBelow(t *testing.T) {
	orig := privateTmux(t)
	sh := []string{"sh", "-c", "sleep 30"}
	ed, err := SplitEditor(t.TempDir(), "sol.cpp", sh, 60)
	if err != nil {
		t.Fatal(err)
	}
	dbg, err := SplitBelow(ed, t.TempDir(), sh, 40)
	if err != nil {
		t.Fatal(err)
	}
	e, d, g := paneGeom(t, ed), paneGeom(t, dbg), paneGeom(t, orig)
	if d.left != e.left || d.width != e.width || d.top <= e.top {
		t.Errorf("debugger %+v should sit under the editor %+v", d, e)
	}
	if g.height != 50 || g.left <= e.left {
		t.Errorf("game pane %+v changed; want it full height on the right", g)
	}
	if !panes(t)[dbg].active {
		t.Error("the debugger pane did not take focus")
	}
	if _, err := SplitBelow(ed, t.TempDir(), nil, 40); err == nil {
		t.Error("SplitBelow with no command succeeded")
	}
	if err := KillPane(dbg); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the debugger pane to close", func() bool { return !PaneAlive(dbg) })
	if err := KillPane(dbg); err != nil {
		t.Errorf("KillPane of a closed pane: %v", err)
	}
}

// compile builds a tiny C++ program with -O0 -g, as judge.Debug would.
func compile(t *testing.T, dir, src string) string {
	t.Helper()
	cxx, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("clang++ not on PATH")
	}
	write(t, filepath.Join(dir, "p.cpp"), src)
	bin := filepath.Join(dir, "p")
	cmd := exec.Command(cxx, "-std=gnu++23", "-O0", "-g", "p.cpp", "-o", bin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NIX_HARDENING_ENABLE=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	return bin
}

// TestLLDBPane drives the real lldb the way :Debug does: stopped at main,
// `run` replays the test, the program reads it and finishes.
func TestLLDBPane(t *testing.T) {
	if _, err := os.Stat(LLDB); err != nil {
		t.Skipf("no %s", LLDB)
	}
	for _, tc := range []struct {
		name   string
		src    string
		fileIO bool
	}{
		{"stdin", "#include <cstdio>\nint main() { int a, b; scanf(\"%d %d\", &a, &b); printf(\"sum=%d\\n\", a + b); }\n", false},
		{"file io", "#include <cstdio>\nint main() { freopen(\"x.in\", \"r\", stdin); freopen(\"x.out\", \"w\", stdout);\n" +
			"int a, b; scanf(\"%d %d\", &a, &b); printf(\"sum=%d\\n\", a + b); }\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orig := privateTmux(t)
			dir := t.TempDir()
			bin := compile(t, dir, tc.src)
			cwd := t.TempDir()
			in := filepath.Join(dir, "3.in")
			write(t, in, "1 2\n")
			stdin := in
			if tc.fileIO {
				write(t, filepath.Join(cwd, "x.in"), "1 2\n")
				stdin = ""
			}
			pane, err := SplitBelow(orig, cwd, LLDBArgs(LLDB, bin, stdin), 70)
			if err != nil {
				t.Fatal(err)
			}
			waitScreen(t, pane, "Breakpoint 1: where = p`main")
			keys(t, pane, "run", "Enter")
			waitScreen(t, pane, "stop reason = breakpoint 1.1")
			keys(t, pane, "continue", "Enter")
			waitScreen(t, pane, "exited with status = 0")
			if tc.fileIO {
				if b, err := os.ReadFile(filepath.Join(cwd, "x.out")); err != nil || string(b) != "sum=3\n" {
					t.Errorf("x.out = %q, %v; want sum=3", b, err)
				}
			} else if s := capture(t, pane); !strings.Contains(s, "sum=3") {
				t.Errorf("program output missing:\n%s", s)
			}
			keys(t, pane, "quit", "Enter")
			waitFor(t, "lldb to quit", func() bool { return !PaneAlive(pane) })
		})
	}
}

func TestCloseEditor(t *testing.T) {
	vim := realVim(t)
	for _, tc := range []struct {
		name   string
		typed  []string // keys before CloseEditor; insert mode left open
		answer string   // to vim's save prompt, "" for none
		closed bool
		file   string
	}{
		{"unmodified", nil, "", true, "int main() {}\n"},
		{"modified in insert mode, saved", []string{"ohello"}, "y", true, "int main() {}\nhello\n"},
		{"modified, not saved", []string{"ohello"}, "n", true, "int main() {}\n"},
		{"modified, cancelled", []string{"ohello"}, "c", false, "int main() {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			privateTmux(t)
			path := solution(t)
			pane, err := tmux("new-window", "-P", "-F", "#{pane_id}", "--", vim, "-u", "NONE", "-i", "NONE", "-N", path)
			if err != nil {
				t.Fatal(err)
			}
			waitScreen(t, pane, "sol.cpp")
			if len(tc.typed) > 0 {
				keys(t, pane, tc.typed...)
				waitScreen(t, pane, "-- INSERT --")
			}
			if err := CloseEditor(pane); err != nil {
				t.Fatal(err)
			}
			if tc.answer != "" {
				waitScreen(t, pane, "Save changes to")
				keys(t, pane, tc.answer)
			}
			if tc.closed {
				waitFor(t, "vim to quit", func() bool { return !PaneAlive(pane) })
			} else {
				ex(t, pane, "echo 'modified=' . &modified")
				waitScreen(t, pane, "modified=1")
				ex(t, pane, "qa!")
			}
			if b, _ := os.ReadFile(path); string(b) != tc.file {
				t.Errorf("file = %q, want %q", b, tc.file)
			}
		})
	}
}

func showWindow(t *testing.T, win, name string) (value string, local bool) {
	t.Helper()
	line, err := tmux("show-options", "-w", "-q", "-t", win, name)
	if err != nil {
		t.Fatal(err)
	}
	if line == "" {
		return "", false
	}
	v, err := tmux("show-options", "-w", "-q", "-v", "-t", win, name)
	if err != nil {
		t.Fatal(err)
	}
	return v, true
}

func TestBorder(t *testing.T) {
	pane := privateTmux(t)
	win, err := WindowOf(pane)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WindowOf("%999"); err == nil {
		t.Error("WindowOf a missing pane succeeded")
	}
	// The window has its own pane-border-format; pane-border-status it
	// inherits. Both must come back exactly so.
	const custom = `#{pane_index} "#{pane_title}"`
	if _, err := tmux("set-option", "-w", "-t", win, "pane-border-format", custom); err != nil {
		t.Fatal(err)
	}
	global := func() string {
		out, err := tmux("show-options", "-g", "-w", "pane-border-status")
		if err != nil {
			t.Fatal(err)
		}
		fo, _ := tmux("show-options", "-g", "-w", "pane-border-format")
		return out + "\n" + fo
	}
	before := global()

	b, err := ShowBorder(win)
	if err != nil {
		t.Fatal(err)
	}
	if v, local := showWindow(t, win, "pane-border-status"); !local || v != "top" {
		t.Errorf("pane-border-status = %q (local %v), want top on the window", v, local)
	}
	// The text is a format, stored as given: its style directives style the
	// border, and ## is how it draws a literal # (the caller escapes).
	const format = "#[fg=colour1,reverse]⏱ 01:23#[default] ##3"
	if err := b.Set(pane, format); err != nil {
		t.Fatal(err)
	}
	if got, _ := tmux("display-message", "-p", "-t", pane, "#{E:pane-border-format}"); got != format {
		t.Errorf("border expands to %q, want the format as given", got)
	}
	if global() != before {
		t.Errorf("global options changed:\n%s\nwas\n%s", global(), before)
	}

	if err := b.Restore(); err != nil {
		t.Fatal(err)
	}
	if v, local := showWindow(t, win, "pane-border-status"); local {
		t.Errorf("pane-border-status left set on the window: %q", v)
	}
	if v, local := showWindow(t, win, "pane-border-format"); !local || v != custom {
		t.Errorf("pane-border-format = %q (local %v), want %q back", v, local, custom)
	}
	if global() != before {
		t.Errorf("global options changed after Restore")
	}
}

// A door that ends while the player has unsaved edits must save them and
// leave no prompt behind (QA: the next keystroke answered "Save changes?").
func TestSaveAndCloseEditor(t *testing.T) {
	vim := realVim(t)
	privateTmux(t)
	path := solution(t)
	pane, err := tmux("new-window", "-P", "-F", "#{pane_id}", "--", vim, "-u", "NONE", "-i", "NONE", "-N", path)
	if err != nil {
		t.Fatal(err)
	}
	waitScreen(t, pane, "sol.cpp")
	keys(t, pane, "ohello")
	waitScreen(t, pane, "-- INSERT --")
	if err := SaveAndCloseEditor(pane); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "vim to quit without asking", func() bool { return !PaneAlive(pane) })
	if b, _ := os.ReadFile(path); string(b) != "int main() {}\nhello\n" {
		t.Fatalf("file = %q, want the edit saved", b)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".swp")); err == nil {
		t.Fatal("swap file left behind")
	}
}

func TestZoom(t *testing.T) {
	orig := privateTmux(t)
	if _, err := SplitEditor(t.TempDir(), "sol.cpp", []string{"sh", "-c", "sleep 30"}, 60); err != nil {
		t.Fatal(err)
	}
	zoomed := func() string {
		out, err := tmux("display-message", "-p", "-t", orig, "#{window_zoomed_flag}")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, step := range []struct {
		on   bool
		want string
	}{{true, "1"}, {true, "1"}, {false, "0"}, {false, "0"}} {
		if err := Zoom(orig, step.on); err != nil {
			t.Fatal(err)
		}
		if got := zoomed(); got != step.want {
			t.Errorf("after Zoom(%v): zoomed %q, want %q", step.on, got, step.want)
		}
	}
}
