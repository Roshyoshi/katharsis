package hud

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/judge"
	"katharsis/internal/session"
)

func TestMain(m *testing.M) {
	// judge.Compile caches under os.UserCacheDir; keep the real cache out
	// of it and start every run cold.
	home, err := os.MkdirTemp("", "katharsis-hud-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for k, v := range map[string]string{"HOME": home, "XDG_CACHE_HOME": filepath.Join(home, ".cache")} {
		if err := os.Setenv(k, v); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

func needCompiler(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(judge.Compiler); err != nil {
		t.Skipf("%s not on PATH", judge.Compiler)
	}
}

// pump runs cmd, and the commands its messages lead to, concurrently as a
// tea.Program would, feeding each message to m.Update on this goroutine,
// until done holds; done sees each message after Update has. It reports
// whether a command asked to quit.
func pump(t *testing.T, m *Model, cmd tea.Cmd, done func(tea.Msg) bool) (quit bool) {
	t.Helper()
	msgs := make(chan tea.Msg, 64)
	run := func(c tea.Cmd) {
		if c != nil {
			go func() { msgs <- c() }()
		}
	}
	run(cmd)
	deadline := time.After(30 * time.Second)
	var msg tea.Msg
	for !done(msg) {
		select {
		case msg = <-msgs:
			switch m2 := msg.(type) {
			case nil:
			case tea.BatchMsg:
				for _, c := range m2 {
					run(c)
				}
			case tea.QuitMsg:
				return true
			default:
				_, c := m.Update(msg)
				run(c)
			}
		case <-deadline:
			t.Fatal("pump: timed out")
		}
	}
	return false
}

func ask(m *Model, cmd string, args ...string) (tea.Cmd, <-chan session.Reply) {
	req, ch := session.NewRequest(session.Command{Cmd: cmd, Args: args})
	_, c := m.Update(req)
	return c, ch
}

func answered(ch <-chan session.Reply) func(tea.Msg) bool {
	return func(tea.Msg) bool { return len(ch) > 0 }
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func key(s string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: rune(s[0]), Text: s} }

func TestHPBar(t *testing.T) {
	for _, tc := range []struct {
		passed, total, cells int
		want                 string
	}{
		{0, 0, 8, "░░░░░░░░"},
		{0, 52, 8, "░░░░░░░░"},
		{37, 52, 8, "▓▓▓▓▓░░░"},
		{51, 52, 8, "▓▓▓▓▓▓▓░"}, // full only when all pass
		{52, 52, 8, "▓▓▓▓▓▓▓▓"},
	} {
		if got := hpBar(tc.passed, tc.total, tc.cells); got != tc.want {
			t.Errorf("hpBar(%d, %d, %d) = %q, want %q", tc.passed, tc.total, tc.cells, got, tc.want)
		}
	}
}

func feedSubmit(m *Model, evs ...judge.Event) {
	for _, ev := range evs {
		m.Update(eventMsg{ch: m.sub.ch, ev: ev})
	}
}

func TestSubmitHPBar(t *testing.T) {
	m := fixture(t, 3)
	req, ch := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	m.sub = submission{ch: make(chan judge.Event), req: req, started: true, running: true,
		tests: m.prob.Tests, results: make([]judge.Result, 3)}
	wa := judge.Result{Verdict: judge.WA, Reason: "line 1", Diff: &judge.Diff{Line: 1, Want: "7", Got: "6"}}
	feedSubmit(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 2, Result: wa},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}},
	)
	s := screen(m)
	for _, want := range []string{"submit ▓", "1/3", "judging 2/3"} {
		if !strings.Contains(s, want) {
			t.Errorf("mid-pass: missing %q in:\n%s", want, s)
		}
	}
	if len(ch) > 0 {
		t.Fatal("answered before the pass finished")
	}

	results := []judge.Result{{Verdict: judge.AC}, {Verdict: judge.AC}, wa}
	sum := judge.Summarize(results)
	feedSubmit(m,
		judge.TestDone{Index: 1, Result: judge.Result{Verdict: judge.AC}},
		judge.Finished{Summary: &sum},
	)
	s = screen(m)
	for _, want := range []string{"2/3", "WA #3"} {
		if !strings.Contains(s, want) {
			t.Errorf("finished: missing %q in:\n%s", want, s)
		}
	}
	rep := <-ch
	if want := "WA 2/3 · #3 line 1: want 7, got 6"; rep.OK || rep.Text != want {
		t.Errorf("reply %+v, want refusal %q", rep, want)
	}
	if st := m.statusText(); !strings.Contains(st, "HP 2/3 WA") {
		t.Errorf("status %q, want HP 2/3 WA", st)
	}
}

// writeSol replaces the fixture's solution and expected outputs.
func writeSol(t *testing.T, m *Model, src string, outs ...string) {
	t.Helper()
	must(t, os.WriteFile(m.prob.Source, []byte(src), 0o644))
	for i, out := range outs {
		must(t, os.WriteFile(m.prob.Tests[i].Out, []byte(out), 0o644))
	}
}

// The whole :Submit path with the real compiler: a submit superseded by a
// newer one is told so, the newer one is answered with its verdict, and a
// SubmittedMsg goes out for an embedding program.
func TestSubmitEndToEnd(t *testing.T) {
	needCompiler(t)
	m := fixture(t, 3)
	writeSol(t, m, "#include <cstdio>\nint main() { puts(\"1\"); }\n", "1\n", "1\n", "2\n")

	_, first := ask(m, session.CmdSubmit)
	c, second := ask(m, session.CmdSubmit)
	if rep := <-first; rep.OK || !strings.Contains(rep.Text, "superseded") {
		t.Errorf("first submit got %+v, want superseded", rep)
	}
	var submitted *judge.Summary
	pump(t, m, c, func(msg tea.Msg) bool {
		if s, ok := msg.(SubmittedMsg); ok {
			submitted = &s.Summary
		}
		return submitted != nil && len(second) > 0
	})
	rep := <-second
	if want := "WA 2/3 · #3 line 1: want 2, got 1"; rep.OK || rep.Text != want {
		t.Errorf("reply %+v, want %q", rep, want)
	}
	if s := screen(m); !strings.Contains(s, "2/3") || !strings.Contains(s, "WA #3") {
		t.Errorf("screen lacks the HP bar result:\n%s", s)
	}
	if submitted.Passed != 2 || submitted.Total != 3 || submitted.First != 2 {
		t.Errorf("SubmittedMsg = %+v, want 2/3 passed, first failure 2", *submitted)
	}
}

func TestDebugTestChoice(t *testing.T) {
	m := fixture(t, 3)
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}},
		judge.TestDone{Index: 1, Result: judge.Result{Verdict: judge.WA}},
	)
	for _, tc := range []struct {
		args []string
		want string // test name, or an error substring
	}{
		{nil, "2"}, // the first failing sample
		{[]string{"3"}, "3"},
		{[]string{"#1"}, "1"},
		{[]string{"9"}, `no test "9" (have 1..3)`},
	} {
		got, err := m.debugTest(tc.args)
		if err != nil {
			got.Name = err.Error()
		}
		if got.Name != tc.want {
			t.Errorf("debugTest(%q) = %q, want %q", tc.args, got.Name, tc.want)
		}
	}
	// A failed submit wins over the samples.
	m.sub = submission{started: true, tests: m.prob.Tests,
		results: []judge.Result{{Verdict: judge.AC}, {Verdict: judge.AC}, {Verdict: judge.TLE}}}
	if got, _ := m.debugTest(nil); got.Name != "3" {
		t.Errorf("after a submit failing #3, debugTest = %q", got.Name)
	}
}

func TestDebugOpensTarget(t *testing.T) {
	needCompiler(t)
	for _, tc := range []struct {
		name   string
		src    string
		fileIO bool
	}{
		{"stdin", "#include <cstdio>\nint main() { int a; scanf(\"%d\", &a); printf(\"%d\\n\", a); }\n", false},
		{"file io", "#include <cstdio>\nint main() { freopen(\"milk2.in\", \"r\", stdin); }\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixture(t, 2)
			writeSol(t, m, tc.src)
			must(t, os.WriteFile(m.prob.Tests[1].In, []byte("5 6\n"), 0o644))
			var got DebugTarget
			closed := 0
			m.opt.Debug = func(d DebugTarget) (func(), error) {
				got = d
				return func() { closed++ }, nil
			}
			c, ch := ask(m, session.CmdDebug, "2")
			pump(t, m, c, answered(ch))
			if rep := <-ch; !rep.OK || !strings.Contains(rep.Text, "lldb on #2") {
				t.Fatalf("reply %+v", rep)
			}
			if _, err := os.Stat(got.Binary + ".dSYM"); err != nil {
				t.Errorf("no symbols beside %s: %v", got.Binary, err)
			}
			if tc.fileIO {
				b, err := os.ReadFile(filepath.Join(got.Dir, "milk2.in"))
				if got.Stdin != "" || err != nil || string(b) != "5 6\n" {
					t.Errorf("file io target %+v: milk2.in = %q, %v", got, b, err)
				}
			} else if got.Stdin != m.prob.Tests[1].In {
				t.Errorf("Stdin = %q, want test 2's input", got.Stdin)
			}

			// A second :Debug replaces the first debugger.
			c, ch = ask(m, session.CmdDebug, "1")
			first := got.Dir
			pump(t, m, c, answered(ch))
			<-ch
			if _, err := os.Stat(first); closed != 1 || !os.IsNotExist(err) {
				t.Errorf("after a second :Debug: closed=%d, first dir stat %v", closed, err)
			}
			m.Cleanup()
			if _, err := os.Stat(got.Dir); closed != 2 || !os.IsNotExist(err) {
				t.Errorf("after Cleanup: closed=%d, dir stat %v", closed, err)
			}
		})
	}
}

func TestDebugRefusals(t *testing.T) {
	m := fixture(t, 1)
	_, ch := ask(m, session.CmdDebug)
	if rep := <-ch; rep.OK || !strings.Contains(rep.Text, "no debugger") {
		t.Errorf("without Options.Debug: %+v", rep)
	}

	needCompiler(t)
	m.opt.Debug = func(DebugTarget) (func(), error) { t.Error("opened a debugger on a broken build"); return nil, nil }
	writeSol(t, m, "int main() { return undeclared; }\n")
	c, ch := ask(m, session.CmdDebug, "1")
	pump(t, m, c, answered(ch))
	if rep := <-ch; rep.OK || !strings.Contains(rep.Text, "CE:") || !strings.Contains(rep.Text, "undeclared") {
		t.Errorf("compile error: %+v", rep)
	}
}

func TestPracticeCommands(t *testing.T) {
	m := fixture(t, 2)
	for _, cmd := range []string{session.CmdQuack, session.CmdGiveUp, session.CmdRetreat} {
		_, ch := ask(m, cmd)
		if rep := <-ch; rep.OK || !strings.Contains(rep.Text, "practice") {
			t.Errorf("%s: %+v, want a practice-mode refusal", cmd, rep)
		}
	}
	_, ch := ask(m, session.CmdStatus)
	if rep := <-ch; !rep.OK || rep.Text != "⏱ 00:00 · samples 0/2" {
		t.Errorf("status: %+v", rep)
	}
}

func TestBorderFollowsStatus(t *testing.T) {
	m := fixture(t, 1)
	texts := make(chan string, 4)
	m.opt.Border = func(s string) { texts <- s }
	m.updateBorder()()
	if got := <-texts; got != " ⏱ 00:00 · samples 0/1 " {
		t.Errorf("border %q", got)
	}
	if c := m.updateBorder(); c != nil {
		t.Error("border rewritten without a change")
	}
	// Judge events refresh it as they land.
	feed(m, judge.BuildDone{Build: judge.Build{OK: true}}, judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}})
	if m.border != " ⏱ 00:00 · samples 1/1 " {
		t.Errorf("border %q after a pass", m.border)
	}
}

func TestQuitClosesEditorFirst(t *testing.T) {
	setup := func(t *testing.T) (*Model, *bool, chan struct{}) {
		m := fixture(t, 1)
		alive := true
		asked := make(chan struct{}, 4)
		m.opt.EditorAlive = func() bool { return alive }
		m.opt.CloseEditor = func() error { asked <- struct{}{}; return nil }
		return m, &alive, asked
	}

	t.Run("waits for the editor", func(t *testing.T) {
		m, alive, asked := setup(t)
		_, cmd := m.Update(key("q"))
		if quit := pump(t, m, cmd, func(tea.Msg) bool { return len(asked) > 0 }); quit {
			t.Fatal("quit before the editor closed")
		}
		if s := screen(m); !strings.Contains(s, "closing vim") {
			t.Errorf("no closing notice:\n%s", s)
		}
		_, cmd = m.Update(closeTickMsg{})
		if isQuit(cmd) {
			t.Fatal("quit while the editor is still open")
		}
		*alive = false
		if _, cmd = m.Update(closeTickMsg{}); !isQuit(cmd) {
			t.Error("did not quit once the editor closed")
		}
	})
	t.Run("second q leaves it open", func(t *testing.T) {
		m, _, _ := setup(t)
		m.Update(key("q"))
		if _, cmd := m.Update(key("q")); !isQuit(cmd) {
			t.Error("second q did not quit")
		}
		if !m.EditorLeftOpen() {
			t.Error("EditorLeftOpen = false after the player chose to leave it")
		}
	})
	t.Run("no editor to close", func(t *testing.T) {
		m, alive, asked := setup(t)
		*alive = false
		if _, cmd := m.Update(key("q")); !isQuit(cmd) {
			t.Error("q with the editor gone did not quit")
		}
		if len(asked) > 0 {
			t.Error("asked a closed editor to quit")
		}
	})
}

// Quitting cancels the submit, and a cancelled pass sends nothing more. An
// embedded HUD outlives its quit (the host reopens the editor), so the
// submit must read stopped, not judging, and :Submit gets its answer.
func TestQuitStopsSubmit(t *testing.T) {
	m := fixture(t, 2)
	m.opt.EditorAlive = func() bool { return false }
	cancelled := false
	req, ch := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	m.sub = submission{ch: make(chan judge.Event), cancel: func() { cancelled = true }, req: req,
		started: true, running: true, tests: m.prob.Tests, results: make([]judge.Result, 2)}
	if _, cmd := m.Update(tickMsg{hud: m, at: time.Now()}); !isQuit(cmd) {
		t.Fatal("the editor closed but the HUD did not quit")
	}
	if !cancelled {
		t.Error("quit left the submit running")
	}
	if len(ch) == 0 {
		t.Error(":Submit was never answered")
	}
	s := screen(m)
	if strings.Contains(s, "judging") || strings.Contains(s, "running…") || !strings.Contains(s, "stopped") {
		t.Errorf("after quitting the passes should read stopped:\n%s", s)
	}
}
