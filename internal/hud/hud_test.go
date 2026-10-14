package hud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	m.prob, m.rows, m.running = p, make([]row, n), true
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
		m.Update(judgeMsg{gen: m.gen, ev: ev})
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

func TestCompileErrorShowsLog(t *testing.T) {
	m := fixture(t, 1)
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: false, Log: "milk2.cpp:3:1: error: expected ';'"}},
		judge.Finished{},
	)
	if s := screen(m); !strings.Contains(s, "CE") || !strings.Contains(s, "expected ';'") {
		t.Fatalf("want CE and the compiler log, got:\n%s", s)
	}
}

func TestStalePassIsDropped(t *testing.T) {
	m := fixture(t, 1)
	m.Update(judgeMsg{gen: m.gen - 1, ev: judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.WA}}})
	if m.rows[0].done {
		t.Fatal("event from an older pass was applied")
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
