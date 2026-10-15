package hud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// replied is the reply ch holds already, if any.
func replied(ch <-chan session.Reply) (session.Reply, bool) {
	select {
	case r := <-ch:
		return r, true
	default:
		return session.Reply{}, false
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func key(s string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: rune(s[0]), Text: s} }

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
	for _, want := range []string{"HP ▓", "1/3", "judging 2/3"} {
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
	// The border is a tmux format: the verdict takes its token, and the
	// plain status stays plain.
	if b := m.borderText(); !strings.Contains(b, "HP 2/3 #[fg=colour1]WA#[default]") {
		t.Errorf("border %q, want the WA toned", b)
	}

	// An earlier submit that passed more shows beside this one; before
	// this HUD's first submit, the host's best is the bar.
	m.opt.Best = func() (int, int) { return 3, 3 }
	if s := screen(m); !strings.Contains(s, "HP ") || !strings.Contains(s, "2/3  WA #3 line 1 · best 3/3") {
		t.Errorf("missing the best submit:\n%s", s)
	}
	m.sub = submission{}
	if s := screen(m); !strings.Contains(s, "3/3  best") {
		t.Errorf("a fresh HUD should show the host's best:\n%s", s)
	}
}

// A host's clock is a format of its own (a toned countdown); the HUD
// escapes everything else it puts on the border, which session.Border.Set
// no longer does.
func TestBorderTextEscapes(t *testing.T) {
	m := fixture(t, 1)
	m.opt.Clock = func() string { return "23:41 #1" }
	if got := m.borderText(); got != " ⏱ 23:41 ##1 · samples 0/1 " {
		t.Errorf("border %q, want the clock escaped", got)
	}
	m.opt.BorderClock = func() string { return "#[fg=colour2]⏱ 23:41#[default]" }
	if got := m.borderText(); got != " #[fg=colour2]⏱ 23:41#[default] · samples 0/1 " {
		t.Errorf("border %q, want the host's format as is", got)
	}
	if got := m.statusText(); got != "⏱ 23:41 #1 · samples 0/1" {
		t.Errorf("status %q stays plain", got)
	}
}

// A host that charges verdicts (Options.HostAnswers) answers :Submit
// itself, once, with the cost the HUD cannot know. A submit that reached
// no verdict costs nothing, and the HUD answers it at once.
func TestHostAnswersWithTheCost(t *testing.T) {
	m := fixture(t, 3)
	m.opt.HostAnswers, m.opt.Conceal = true, func() bool { return true }
	submitting := func() <-chan session.Reply {
		req, ch := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
		m.sub = submission{ch: make(chan judge.Event), req: req, started: true, running: true,
			tests: m.prob.Tests, results: make([]judge.Result, 3)}
		return ch
	}

	ch := submitting()
	wa := judge.Result{Verdict: judge.WA, Reason: "line 1", Diff: &judge.Diff{Line: 1, Want: "7", Got: "6"}}
	m.sub.results = []judge.Result{{Verdict: judge.AC}, {Verdict: judge.AC}, wa}
	sum := judge.Summarize(m.sub.results)
	msg := m.applySubmit(judge.Finished{Summary: &sum})().(SubmittedMsg)
	if rep, ok := replied(ch); ok {
		t.Fatalf("answered %+v before the host charged the verdict", rep)
	}
	if msg.Summary != sum {
		t.Errorf("SubmittedMsg summary %+v, want %+v", msg.Summary, sum)
	}
	msg.Answer("-1 ♥, 2 left")
	msg.Answer("again")
	if rep := <-ch; rep.OK || rep.Text != "WA 2/3 · #3 line 1 · -1 ♥, 2 left" {
		t.Errorf("reply %+v, want the concealed verdict and its cost", rep)
	}
	if rep, ok := replied(ch); ok {
		t.Errorf("answered twice: %+v", rep)
	}

	ch = submitting()
	msg = m.applySubmit(judge.Finished{Err: errors.New("no compiler")})().(SubmittedMsg)
	if rep, ok := replied(ch); !ok || rep.Text != "judge error: no compiler" {
		t.Errorf("a submit with no verdict: answered %v with %+v, want it at once", ok, rep)
	}
	if msg.Err == nil {
		t.Error("SubmittedMsg without its error")
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

// The whole :Submit path with the real compiler. A second submit while the
// first is judging is refused at once, and leaves the first's copy alone;
// the first is answered with its verdict, and a SubmittedMsg goes out for
// an embedding program. The second used to replace the first, so a WA
// followed by a fixed submit was never charged.
func TestSubmitEndToEnd(t *testing.T) {
	needCompiler(t)
	m := fixture(t, 3)
	m.opt.SubmitCopy = filepath.Join(m.opt.Dir, ".submit", "milk2.cpp")
	wa := "#include <cstdio>\nint main() { puts(\"1\"); }\n"
	writeSol(t, m, wa, "1\n", "1\n", "2\n")
	for i, in := range []string{"1\n", "1\n", "2\n"} {
		must(t, os.WriteFile(m.prob.Tests[i].In, []byte(in), 0o644))
	}

	c, first := ask(m, session.CmdSubmit)
	writeSol(t, m, "#include <cstdio>\nint main() { int x; scanf(\"%d\", &x); printf(\"%d\\n\", x); }\n")
	c2, second := ask(m, session.CmdSubmit)
	if rep, ok := replied(second); !ok || rep.OK || rep.Text != SubmitBusy {
		t.Errorf("second submit answered %v with %+v, want %q at once", ok, rep, SubmitBusy)
	}
	if c2 != nil {
		t.Error("the refused submit returned a command; it must end nothing for the host")
	}
	if b, err := os.ReadFile(m.opt.SubmitCopy); err != nil || string(b) != wa {
		t.Errorf("copy = %q, %v; want the first submit's source", b, err)
	}
	var submitted *judge.Summary
	pump(t, m, tea.Batch(c, c2), func(msg tea.Msg) bool {
		if s, ok := msg.(SubmittedMsg); ok {
			submitted = &s.Summary
		}
		return submitted != nil && len(first) > 0
	})
	rep := <-first
	if want := "WA 2/3 · #3 line 1: want 2, got 1"; rep.OK || rep.Text != want {
		t.Errorf("first submit got %+v, want %q", rep, want)
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
			// A file-I/O program gets <stem>.in and stdin both, as the judge
			// feeds them, since detecting the stem can guess wrong.
			if got.Stdin != m.prob.Tests[1].In {
				t.Errorf("Stdin = %q, want test 2's input", got.Stdin)
			}
			if tc.fileIO {
				b, err := os.ReadFile(filepath.Join(got.Dir, "milk2.in"))
				if err != nil || string(b) != "5 6\n" {
					t.Errorf("file io target %+v: milk2.in = %q, %v", got, b, err)
				}
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
	if rep := <-ch; !rep.OK || rep.Text != "up 00:00 · samples 0/2" {
		t.Errorf("status: %+v", rep)
	}
}

func TestBorderFollowsStatus(t *testing.T) {
	m := fixture(t, 1)
	texts := make(chan string, 4)
	m.opt.Border = func(s string) { texts <- s }
	m.updateBorder()()
	if got := <-texts; got != " up 00:00 · samples 0/1 " {
		t.Errorf("border %q", got)
	}
	if c := m.updateBorder(); c != nil {
		t.Error("border rewritten without a change")
	}
	// Judge events refresh it as they land.
	feed(m, judge.BuildDone{Build: judge.Build{OK: true}}, judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}})
	if m.border != " up 00:00 · samples 1/1 " {
		t.Errorf("border %q after a pass", m.border)
	}
}

// Each border write runs as a command on a goroutine of its own, so an
// older one can reach tmux after a newer one. It must not undo it: with a
// clock that does not move (an upsolve's "untimed") the stale text stayed
// for the rest of the door.
func TestBorderWritesKeepOrder(t *testing.T) {
	m := fixture(t, 1)
	var got []string
	m.opt.Border = func(s string) { got = append(got, s) }
	older := m.updateBorder()
	m.rows[0] = row{done: true, res: judge.Result{Verdict: judge.AC}}
	newer := m.updateBorder()
	newer()
	older()
	if want := " up 00:00 · samples 1/1 "; len(got) == 0 || got[len(got)-1] != want {
		t.Errorf("border writes %q; the last must be the newest, %q", got, want)
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

// running is a submit in flight on m's tests whose cancel sets *cancelled.
func running(m *Model, cancelled *bool) (session.Request, <-chan session.Reply) {
	req, ch := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	m.sub = submission{ch: make(chan judge.Event), cancel: func() { *cancelled = true }, req: req,
		started: true, running: true, tests: m.prob.Tests, results: make([]judge.Result, len(m.prob.Tests)),
		build: &judge.Build{OK: true}}
	return req, ch
}

// The HUD quits when the editor closes, and an embedded one outlives its
// quit. Quitting cancelled the submit with no verdict, so closing vim
// mid-judge dodged a failing submit (qa/chaos r05). It now runs on, and
// only Cleanup, when the door ends, stops it: answered, reading stopped.
func TestQuitKeepsSubmit(t *testing.T) {
	m := fixture(t, 2)
	m.opt.EditorAlive = func() bool { return false }
	cancelled := false
	_, ch := running(m, &cancelled)
	if _, cmd := m.Update(tickMsg{hud: m, at: time.Now()}); !isQuit(cmd) {
		t.Fatal("the editor closed but the HUD did not quit")
	}
	if cancelled || len(ch) != 0 {
		t.Fatalf("quit cancelled %v, answered %d: the submit should run on", cancelled, len(ch))
	}
	if s := screen(m); !strings.Contains(s, "judging") {
		t.Errorf("after quitting the submit should read judging:\n%s", s)
	}
	m.Cleanup()
	if !cancelled || len(ch) == 0 {
		t.Errorf("Cleanup cancelled %v, answered %d: the door is over", cancelled, len(ch))
	}
	if s := screen(m); strings.Contains(s, "judging") || !strings.Contains(s, "stopped") {
		t.Errorf("after Cleanup the submit should read stopped:\n%s", s)
	}
}

// The editor reopened makes a new HUD; it takes over the submit, whose
// events then apply to it, and the old HUD's Cleanup leaves it running.
func TestAdoptMovesTheSubmit(t *testing.T) {
	prev, next := fixture(t, 2), fixture(t, 2)
	cancelled := false
	_, answer := running(prev, &cancelled)
	pass := prev.sub.ch
	next.Adopt(prev)
	prev.Cleanup()
	if cancelled {
		t.Fatal("the old HUD's Cleanup cancelled the adopted submit")
	}
	next.Update(eventMsg{ch: pass, ev: judge.BuildDone{Build: judge.Build{OK: true}}})
	next.Update(eventMsg{ch: pass, ev: judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.WA}}})
	if s := screen(next); !strings.Contains(s, "judging 1/2") {
		t.Errorf("the new HUD should show the adopted pass:\n%s", s)
	}
	next.Update(eventMsg{ch: pass, ev: judge.Finished{Summary: &judge.Summary{Verdict: judge.WA, Total: 2, Done: 1, First: 0}}})
	if s := screen(next); !strings.Contains(s, "WA #1") {
		t.Errorf("the new HUD should show the verdict:\n%s", s)
	}
	if len(answer) == 0 {
		t.Error(":Submit was never answered")
	}
}

// Cleanup returns once the passes it cancelled have ended, a sample pass a
// save replaced included. Each pass's compiler or solution runs in a
// process group of its own, past tmux's SIGHUP, and only a goroutine of
// this process kills it: a game that exited right after the cancel left
// a compile running.
func TestCleanupWaitsForThePasses(t *testing.T) {
	m := fixture(t, 1)
	var ended atomic.Int32
	lagging := func(ctx context.Context, _ func(judge.Event)) {
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond) // the kill and the reap
		ended.Add(1)
	}
	for range 2 { // the second replaces the first, as rejudge does
		var done <-chan struct{}
		if m.cancel != nil {
			m.cancel()
		}
		m.pass, m.cancel, done = start(lagging)
		m.passes = append(m.passes, done)
	}
	req, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	ch, cancel, done := start(lagging)
	m.sub = submission{ch: ch, cancel: cancel, done: done, req: req, started: true, running: true}

	m.Cleanup()
	if n := ended.Load(); n != 3 {
		t.Errorf("%d of 3 passes had ended when Cleanup returned", n)
	}
}

// With SubmitCopy the submit judges a copy of the source made when it was
// sent, which Resubmit judges again as it stands: later edits to the
// source change neither.
func TestSubmitCopy(t *testing.T) {
	m := fixture(t, 1)
	m.opt.SubmitCopy = filepath.Join(m.opt.Dir, ".submit", "milk2.cpp")
	if cmd := m.Resubmit(); cmd == nil {
		t.Fatal("Resubmit with no copy started nothing")
	} else if msg, ok := cmd().(SubmittedMsg); !ok || msg.Err == nil {
		t.Fatalf("Resubmit with no copy = %#v, want an error", msg)
	}
	req, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	m.submit(req)
	m.stop() // the game quit mid-judge
	must(t, os.WriteFile(m.prob.Source, []byte("edited after the submit"), 0o644))
	if b, err := os.ReadFile(m.opt.SubmitCopy); err != nil || string(b) != "int main(){}" {
		t.Fatalf("copy = %q, %v; want the source as submitted", b, err)
	}
	if m.sub.src != m.opt.SubmitCopy {
		t.Errorf("judged %s, want the copy", m.sub.src)
	}
	m.Resubmit()
	m.sub.cancel()
	if m.sub.src != m.opt.SubmitCopy || !m.sub.running {
		t.Errorf("Resubmit judged %s (running %v), want the copy", m.sub.src, m.sub.running)
	}
}

// A submit judged again on resume runs to its verdict: a :Submit sent
// meanwhile is refused, and neither cancels the pass nor touches the copy
// it judges. It used to replace it, so resubmitting right after a resume
// dodged the failure the resume was there to charge (qa/chaos r05).
func TestResubmitCannotBeReplaced(t *testing.T) {
	m := fixture(t, 1)
	m.opt.SubmitCopy = filepath.Join(m.opt.Dir, ".submit", "milk2.cpp")
	m.opt.SubmitProfile = judge.Samples // no probe for a pass never read
	must(t, os.MkdirAll(filepath.Dir(m.opt.SubmitCopy), 0o755))
	must(t, copyFile(m.prob.Source, m.opt.SubmitCopy)) // what the quit game left
	m.Resubmit()
	t.Cleanup(m.Cleanup)
	pass, cancel, cancelled := m.sub.ch, m.sub.cancel, false
	m.sub.cancel = func() { cancelled = true; cancel() }

	must(t, os.WriteFile(m.prob.Source, []byte("edited after the resume"), 0o644))
	_, ch := ask(m, session.CmdSubmit)
	if rep, ok := replied(ch); !ok || rep.OK || rep.Text != SubmitBusy {
		t.Errorf("submit during the resumed judge answered %v with %+v, want %q at once", ok, rep, SubmitBusy)
	}
	if cancelled || m.sub.ch != pass || !m.sub.running {
		t.Errorf("cancelled %v, same pass %v, running %v: the resumed judge must run on",
			cancelled, m.sub.ch == pass, m.sub.running)
	}
	if b, err := os.ReadFile(m.opt.SubmitCopy); err != nil || string(b) != "int main(){}" {
		t.Errorf("copy = %q, %v; want what was submitted before the quit", b, err)
	}
}
