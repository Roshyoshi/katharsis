package hud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/judge"
	"katharsis/internal/problem"
)

// fixture builds a problem dir with n empty tests and a model already
// pointed at it, without starting a judge pass.
func fixture(t *testing.T, n int) *Model {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "milk2")
	must(t, os.MkdirAll(filepath.Join(dir, "tests"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "milk2.cpp"), []byte("int main(){}"), 0o644))
	for i := 1; i <= n; i++ {
		base := filepath.Join(dir, "tests", string(rune('0'+i)))
		must(t, os.WriteFile(base+".in", nil, 0o644))
		must(t, os.WriteFile(base+".out", nil, 0o644))
	}
	m := New(Options{Dir: dir, Profile: judge.Samples})
	p, err := problem.Load(dir)
	must(t, err)
	m.prob, m.rows, m.running, m.pass = p, make([]row, n), true, make(chan judge.Event)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	return m
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func screen(m *Model) string { return ansi.Strip(m.View().Content) }

func feed(m *Model, evs ...judge.Event) {
	for _, ev := range evs {
		m.Update(eventMsg{ch: m.pass, ev: ev})
	}
}

func TestAllPass(t *testing.T) {
	m := fixture(t, 2)
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}},
		judge.TestDone{Index: 1, Result: judge.Result{Verdict: judge.AC}},
		judge.Finished{},
	)
	if s := screen(m); !strings.Contains(s, "2/2") || !strings.Contains(s, "ALL PASS") {
		t.Fatalf("want 2/2 ALL PASS, got:\n%s", s)
	}
}

func TestWrongAnswerShowsDiffAndSelectsIt(t *testing.T) {
	m := fixture(t, 3)
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}},
		judge.TestDone{Index: 1, Result: judge.Result{
			Verdict: judge.WA, Reason: "line 2",
			Diff: &judge.Diff{Line: 2, Want: "7", Got: "6"},
		}},
	)
	s := screen(m)
	for _, want := range []string{"▸ 2", "#2 WA", "want 7", "got  6", "running…"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "ALL PASS") {
		t.Errorf("ALL PASS shown with a WA:\n%s", s)
	}
}

// The compiler names the source by its absolute path, which in a boot is a
// deep temp directory; every place a CE shows must cut it, or a narrow pane
// shows only the path.
func TestCompileErrorDropsSourceDir(t *testing.T) {
	ceLog := func(m *Model) (log, dir string) {
		dir = filepath.Dir(m.prob.Source) + string(filepath.Separator)
		return dir + "milk2.cpp:3:1: error: expected ';' after expression\n" +
			dir + "milk2.cpp:7:9: note: in instantiation\n" +
			"/usr/include/c++/v1/vector:12:1: note: declared here\n", dir
	}
	for _, tc := range []struct {
		name string
		show func(m *Model, log string) string
		want []string
	}{
		{"sample CE", func(m *Model, log string) string {
			feed(m, judge.BuildDone{Build: judge.Build{OK: false, Log: log}}, judge.Finished{})
			return screen(m)
		}, []string{
			"milk2.cpp:3:1: error: expected ';'",
			// Paths outside the source's directory are kept.
			"/usr/include/c++/v1/vector:12:1",
		}},
		{"submit CE line", func(m *Model, log string) string {
			m.sub = submission{started: true, src: m.prob.Source, build: &judge.Build{OK: false, Log: log}}
			return screen(m)
		}, []string{"submit CE  milk2.cpp:3:1: error: expected ';'"}},
		{"submit CE reply", func(m *Model, log string) string {
			sum := judge.Summary{Verdict: judge.CE}
			m.sub = submission{started: true, src: m.prob.Source, build: &judge.Build{OK: false, Log: log}, sum: &sum}
			return m.submitReply().Text
		}, []string{"CE: milk2.cpp:3:1: error: expected ';' after expression"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixture(t, 1)
			log, dir := ceLog(m)
			got := tc.show(m, log)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("want %q in:\n%s", w, got)
				}
			}
			if strings.Contains(got, dir) {
				t.Errorf("still shows the source directory %q:\n%s", dir, got)
			}
		})
	}
}

// The standalone header puts the clock flush with the right edge. A boot's
// clock carries ♥ and 🦆, which are wider in bytes than in cells.
func TestHeaderClockFlushRight(t *testing.T) {
	for _, clock := range []string{"12:34", "23:41  ♥♥♡  🦆1  96 B"} {
		t.Run(clock, func(t *testing.T) {
			m := fixture(t, 1)
			m.opt.Clock = func() string { return clock }
			head := strings.Split(screen(m), "\n")[0]
			if !strings.HasSuffix(head, "⏱ "+clock) || ansi.StringWidth(head) != 60 {
				t.Errorf("header %q is %d cells, want 60 ending in the clock", head, ansi.StringWidth(head))
			}
		})
	}
}

// A host opens a HUD per door and forwards every message to the current
// one, so the last door's late events and ticks reach the next door's HUD.
// Its own older passes are just as stale.
func TestStaleMessagesAreDropped(t *testing.T) {
	old, m := fixture(t, 3), fixture(t, 1)
	m.sub = submission{ch: make(chan judge.Event), started: true, running: true,
		tests: m.prob.Tests, results: make([]judge.Result, 1)}
	old.sub = submission{ch: make(chan judge.Event)}
	wa := judge.TestDone{Index: 2, Result: judge.Result{Verdict: judge.WA}}
	for name, msg := range map[string]tea.Msg{
		"older pass":       eventMsg{ch: make(chan judge.Event), ev: wa},
		"other HUD's pass": eventMsg{ch: old.pass, ev: wa},
		"other HUD's sub":  eventMsg{ch: old.sub.ch, ev: wa},
		"other HUD's tick": tickMsg{hud: old, at: time.Now()},
	} {
		if _, cmd := m.Update(msg); cmd != nil {
			t.Errorf("%s: answered with a command, so its chain lives on", name)
		}
	}
	if m.rows[0].done || m.sub.results[0].Verdict != "" {
		t.Errorf("a stale event was applied: rows %+v, submit %+v", m.rows, m.sub.results)
	}
}

// A standalone pane's view is exactly its height, with the key help on the
// bottom row, whatever sits above the rule: a submit's HP bar over a
// compile error used to push the help a row below the pane.
func TestStandaloneFitsHeight(t *testing.T) {
	m := fixture(t, 2)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	m.sub = submission{started: true, tests: m.prob.Tests, results: make([]judge.Result, 2), build: &judge.Build{OK: true}}
	m.closing = true
	feed(m, judge.BuildDone{Build: judge.Build{OK: false, Log: strings.Repeat("sol.cpp:1:1: error: x\n", 40)}})
	lines := strings.Split(screen(m), "\n")
	if len(lines) != 12 || !strings.Contains(lines[11], "q quit") || !strings.Contains(lines[0], "milk2") {
		t.Errorf("view is %d lines (pane is 12), want the header first and the key help last:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
	if !strings.Contains(screen(m), "more lines") {
		t.Errorf("the cut log should say how much it hides:\n%s", screen(m))
	}
}

// A full test set is taller than the pane: the rows scroll with the
// selection and the detail and key help stay on screen.
func TestRowsScroll(t *testing.T) {
	m := fixture(t, 9)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	evs := []judge.Event{judge.BuildDone{Build: judge.Build{OK: true}}}
	for i := range 9 {
		v := judge.AC
		if i == 7 {
			v = judge.WA
		}
		evs = append(evs, judge.TestDone{Index: i, Result: judge.Result{Verdict: v, Reason: "line 1",
			Diff: &judge.Diff{Line: 1, Want: "7", Got: "6"}}})
	}
	feed(m, evs...)
	s := screen(m)
	if n := strings.Count(s, "\n") + 1; n != 12 {
		t.Errorf("screen is %d lines, pane is 12:\n%s", n, s)
	}
	for _, want := range []string{"▸ 8", "#8 WA", "want 7", "q quit", "of 9"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "  1   AC") {
		t.Errorf("row 1 shown though the window follows row 8:\n%s", s)
	}
}

func TestScroll(t *testing.T) {
	for _, tc := range []struct{ n, sel, room, lo, hi int }{
		{9, 0, 4, 0, 4},
		{9, 7, 4, 5, 9},
		{9, 4, 4, 2, 6},
		{3, 1, 5, 0, 3},
		{9, 8, 0, 8, 9},
	} {
		if lo, hi := scroll(tc.n, tc.sel, tc.room); lo != tc.lo || hi != tc.hi {
			t.Errorf("scroll(%d, %d, %d) = %d, %d; want %d, %d", tc.n, tc.sel, tc.room, lo, hi, tc.lo, tc.hi)
		}
	}
}

func TestLinesFitWidth(t *testing.T) {
	m := fixture(t, 1)
	m.Update(tea.WindowSizeMsg{Width: 24, Height: 12})
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{
			Verdict: judge.RE, Reason: "AddressSanitizer: heap-buffer-overflow in main",
		}},
	)
	for _, l := range strings.Split(screen(m), "\n") {
		if w := ansi.StringWidth(l); w > 24 {
			t.Errorf("line %q is %d wide, pane is 24", l, w)
		}
	}
}

// embedded is fixture with the HUD set up as a boot embeds it: under the
// boot's HUD bar, which shows the title and the countdown, and over the
// door's own key help.
func embedded(t *testing.T, n, height int) *Model {
	t.Helper()
	m := fixture(t, n)
	m.opt.Embedded = true
	m.opt.Clock = func() string { return "12:34" }
	m.Update(tea.WindowSizeMsg{Width: 60, Height: height})
	return m
}

func TestEmbeddedDropsChrome(t *testing.T) {
	m := embedded(t, 2, 10)
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}},
		judge.TestDone{Index: 1, Result: judge.Result{Verdict: judge.AC}},
		judge.Finished{},
	)
	s := screen(m)
	lines := strings.Split(s, "\n")
	if len(lines) != 10 {
		t.Errorf("view is %d lines, pane is 10:\n%s", len(lines), s)
	}
	if !strings.Contains(lines[0], "2/2") || !strings.Contains(lines[0], "ALL PASS") {
		t.Errorf("first line should be the judge status, got %q", lines[0])
	}
	for _, gone := range []string{"milk2", "12:34", "⏱", "q quit", ":Submit"} {
		if strings.Contains(s, gone) {
			t.Errorf("embedded view repeats the host's %q:\n%s", gone, s)
		}
	}
	// The border is on the editor pane, not the host's: it keeps the clock.
	if st := m.statusText(); !strings.Contains(st, "12:34") {
		t.Errorf("border text lost the clock: %q", st)
	}

	m.opt.Embedded = false
	if s := screen(m); !strings.Contains(s, "milk2") || !strings.Contains(s, "q quit") {
		t.Errorf("standalone view lost its header or key help:\n%s", s)
	}
}

// A sanitizer report taller than the pane is cut, not allowed to push the
// host's lines above it off screen.
func TestEmbeddedFitsHeight(t *testing.T) {
	m := embedded(t, 1, 8)
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{
			Verdict: judge.RE, Reason: "AddressSanitizer: heap-buffer-overflow",
			Stderr: strings.Repeat("    #0 0x0001 in main sol.cpp:3\n", 40),
		}},
	)
	if n := strings.Count(screen(m), "\n") + 1; n != 8 {
		t.Errorf("view is %d lines, pane is 8:\n%s", n, screen(m))
	}
	feed(m, judge.BuildDone{Build: judge.Build{OK: false, Log: strings.Repeat("sol.cpp:1:1: error: x\n", 40)}})
	s := screen(m)
	if n := strings.Count(s, "\n") + 1; n != 8 || !strings.Contains(s, "compile error") {
		t.Errorf("CE view is %d lines (pane is 8) or lost the CE line:\n%s", n, s)
	}
}

func TestEmbeddedNoSamplesHint(t *testing.T) {
	m := embedded(t, 0, 10)
	s := screen(m)
	if strings.Contains(s, "add tests/1.in") || !strings.Contains(s, ":Submit judges the full tests") {
		t.Errorf("a door without samples got the practice hint:\n%s", s)
	}
}
