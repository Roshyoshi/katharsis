package hud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/judge"
)

// An Unjudged source (still the template) is not judged on open: the
// samples say they have not run and how to run them, the border says so,
// and the first save judges them (J13).
func TestUnjudgedWaitsForASave(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	must(t, os.MkdirAll(filepath.Join(dir, "tests"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "p.cpp"), []byte("int main(){}"), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "tests", "1.in"), nil, 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "tests", "1.out"), []byte("1\n"), 0o644))
	m := New(Options{Dir: dir, Profile: judge.Samples, Embedded: true, Unjudged: true})
	m.Init()
	if m.running || m.pass != nil {
		t.Fatal("an unjudged template started a sample pass on open")
	}
	s := screen(m)
	if !strings.Contains(s, "samples not run · :w judges") || strings.Contains(s, "WA") {
		t.Errorf("unjudged view:\n%s", s)
	}
	if st := m.statusText(); !strings.Contains(st, "samples not run") {
		t.Errorf("border %q, want samples not run", st)
	}
	if got := m.Samples(); got.Done {
		t.Errorf("Samples %+v before any pass, want not done", got)
	}
	m.Update(SavedMsg{})
	if !m.running || m.waiting {
		t.Error("a save did not judge the samples")
	}
	m.Cleanup()
}

// Pinned is the HP bar and the samples in one line, the first failure's
// detail folded in.
func TestPinned(t *testing.T) {
	m := embedded(t, 2, 10)
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.AC}},
		judge.TestDone{Index: 1, Result: judge.Result{Verdict: judge.WA, Reason: "line 3",
			Diff: &judge.Diff{Want: "NO", Got: "YES"}}},
		judge.Finished{},
	)
	p := m.Pinned(60)
	if len(p) != 2 || !strings.HasPrefix(ansi.Strip(p[0]), "HP ") {
		t.Fatalf("Pinned = %q", p)
	}
	if got := ansi.Strip(p[1]); got != "samples 1/2 · #2 WA line 3 · want NO got YES" {
		t.Errorf("samples line %q", got)
	}
}

// Samples counts a failure as soon as it is judged, the pass still running,
// so a host can ask before a submit that follows a :w closely.
func TestSamplesFailedMidPass(t *testing.T) {
	m := embedded(t, 2, 10)
	feed(m,
		judge.BuildDone{Build: judge.Build{OK: true}},
		judge.TestDone{Index: 0, Result: judge.Result{Verdict: judge.WA}},
	)
	if s := m.Samples(); s.Done || !s.Compiled || s.Failed != 1 || s.Passed != 0 || s.Total != 2 {
		t.Errorf("Samples mid-pass = %+v, want running, compiled, 1 failed of 2", s)
	}
}
