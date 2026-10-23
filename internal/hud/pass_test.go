package hud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"katharsis/internal/judge"
)

// A build that warned says so in the pane and the pinned judgement, ahead
// of the failure's detail, and names the first warning by its place in
// the pane; the warning's file and the compiler's own lines are cut.
func TestWarningsShown(t *testing.T) {
	m := fixture(t, 1)
	src := m.prob.Source
	log := "clang++: warning: argument unused during compilation: '-L/x'\n" +
		src + ":9:13: warning: declaration shadows a local variable [-Wshadow]\n" +
		"    9 |     { int n = 2; }\n" +
		src + ":12:9: warning: unused variable 'k' [-Wunused-variable]\n"
	feed(m, judge.BuildDone{Build: judge.Build{OK: true, Log: log}},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.WA, Reason: "line 1",
			Diff: &judge.Diff{Line: 1, Want: "YES", Got: judge.EOF, GotLine: 1}}},
		judge.Finished{})
	if s := screen(m); !strings.Contains(s, "2 warnings · 9:13 declaration shadows a local variable") ||
		strings.Contains(s, "argument unused") || strings.Contains(s, src) {
		t.Errorf("pane:\n%s", s)
	}
	pinned := strings.Join(m.Pinned(80), "\n")
	if i, j := strings.Index(pinned, "2 warnings"), strings.Index(pinned, "#1 WA"); i < 0 || j < 0 || i > j {
		t.Errorf("pinned, warnings first:\n%s", pinned)
	}

	clean := fixture(t, 1)
	feed(clean, judge.BuildDone{Build: judge.Build{OK: true}})
	if s := screen(clean) + strings.Join(clean.Pinned(80), "\n"); strings.Contains(s, "warning") {
		t.Errorf("a clean build warned:\n%s", s)
	}
}

// Pass carries each sample's files and result, the program's answer among
// it, and the build log as the pane shows it, paths cut; a file is read
// once per problem load, and a long one only in part.
func TestPass(t *testing.T) {
	m := fixture(t, 2)
	tests := m.prob.Tests
	must(t, os.WriteFile(tests[0].In, []byte("7\n1 1 4 2 3\n"), 0o644))
	must(t, os.WriteFile(tests[0].Out, []byte("YES\n"), 0o644))
	must(t, os.WriteFile(tests[1].In, []byte(strings.Repeat("9 ", passFileCap)), 0o644))
	wa := judge.Result{Verdict: judge.WA, Stdout: "", Diff: &judge.Diff{Line: 1, Want: "YES", Got: judge.EOF, GotLine: 1}}
	feed(m, judge.BuildDone{Build: judge.Build{OK: true, Log: m.prob.Source + ":3:1: warning: x\n"}},
		judge.TestDone{Index: 0, Result: wa})

	p := m.Pass()
	if p.Name != m.prob.Name || p.Profile != judge.Samples.Name || !p.Running || p.Build == nil ||
		!strings.HasPrefix(p.Build.Log, "milk2.cpp:3:1:") || len(p.Tests) != 2 {
		t.Fatalf("pass: %+v", p)
	}
	if s := p.Tests[0]; s.Name != "1" || s.In != "7\n1 1 4 2 3\n" || s.Want != "YES\n" || !s.Done || s.Result.Diff == nil {
		t.Errorf("sample 1: %+v", s)
	}
	if s := p.Tests[1]; s.Done || len(s.In) > passFileCap+40 || !strings.Contains(s.In, "more") {
		t.Errorf("sample 2, long and pending: done %v, %d bytes", s.Done, len(s.In))
	}
	// Read once: a file changed under the HUD is the same until a reload.
	must(t, os.WriteFile(tests[0].In, []byte("changed\n"), 0o644))
	if m.Pass().Tests[0].In != "7\n1 1 4 2 3\n" {
		t.Error("Pass read the input again")
	}
	if !m.load() || m.Pass().Tests[0].In != "changed\n" {
		t.Error("a reload kept the old input")
	}
}

// A case of the player's own with no answer (:Case) loads with no Want,
// and once run reads "ran" with the first line it printed; it counts as
// passing, so it is no failure for :Submit to ask about.
func TestOwnCaseRuns(t *testing.T) {
	m := fixture(t, 1)
	must(t, os.WriteFile(filepath.Join(filepath.Dir(m.prob.Tests[0].In), "c1.in"), []byte("5\n"), 0o644))
	if !m.load() || len(m.prob.Tests) != 2 {
		t.Fatalf("load: %v, tests %+v", m.loadErr, m.prob.Tests)
	}
	if s := m.Pass().Tests[1]; s.Name != "c1" || !s.NoWant || s.Want != "" || s.In != "5\n" {
		t.Errorf("own case: %+v", s)
	}
	feed(m, judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}},
		judge.TestDone{Index: 1, Result: judge.Result{Verdict: judge.AC, Unchecked: true, Stdout: "NO\nmore\n"}},
		judge.Finished{})
	m.sel = 1
	if s := screen(m); !strings.Contains(s, "c1  ran") || !strings.Contains(s, "out  NO …") ||
		!strings.Contains(s, "no answer to compare") {
		t.Errorf("pane:\n%s", s)
	}
	if got := m.Samples(); got.Failed != 0 || got.Passed != 2 {
		t.Errorf("samples: %+v", got)
	}
	// Given an answer it is judged, and its failure is told apart from a
	// sample's: no submit is free for it.
	feed(m, judge.TestDone{Index: 1, Result: judge.Result{Verdict: judge.WA}})
	if got := m.Samples(); got.Failed != 1 || got.OwnFailed != 1 {
		t.Errorf("samples after c1's WA: %+v", got)
	}
	feed(m, judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.WA}})
	if got := m.Samples(); got.Failed != 2 || got.OwnFailed != 1 {
		t.Errorf("samples after both fail: %+v", got)
	}
}

func TestShortWarning(t *testing.T) {
	for in, want := range map[string]string{
		"a.cpp:47:17: warning: variable 'i' is uninitialized when used here [-Wuninitialized]": "47:17 variable 'i' is uninitialized when used here [-Wuninitialized]",
		"a.cpp:3: warning: no column": "3 no column",
		"not a warning":               "not a warning",
	} {
		if got := ShortWarning(in); got != want {
			t.Errorf("ShortWarning(%q) = %q, want %q", in, got, want)
		}
	}
}
