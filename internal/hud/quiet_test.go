package hud

import (
	"strings"
	"testing"

	"katharsis/internal/judge"
)

// TestQuietSamples: Heisenbug's twist (Options.Quiet, D54) reports the
// sample pass as pass or fail only. No per-sample row, count, diff or
// crash report shows in the pane, the border, the pinned judgement or ctl
// status; the host still gets the counts.
func TestQuietSamples(t *testing.T) {
	wa := judge.TestDone{Index: 1, Result: judge.Result{Verdict: judge.WA, Reason: "line 2",
		Diff: &judge.Diff{Line: 2, Want: "seven", Got: "six"}}}
	re := judge.TestDone{Index: 2, Result: judge.Result{Verdict: judge.RE, Reason: "SIGSEGV", Stderr: "AddressSanitizer: heap-buffer-overflow"}}
	ac := func(i int) judge.TestDone { return judge.TestDone{Index: i, Result: judge.Result{Verdict: judge.AC}} }

	for _, embed := range []bool{false, true} {
		m := fixture(t, 3)
		if embed {
			m = embedded(t, 3, 16)
		}
		m.opt.Quiet = true
		feed(m, judge.BuildDone{Build: judge.Build{OK: true}}, ac(0))
		if s := screen(m); !strings.Contains(s, "samples running…") {
			t.Errorf("embedded %v: mid-pass, no failure yet:\n%s", embed, s)
		}
		feed(m, wa, re, judge.Finished{})
		views := map[string]string{
			"pane": screen(m), "border": m.borderText(), "status": m.statusText(),
			"pinned": strings.Join(m.Pinned(60), "\n"),
		}
		for where, v := range views {
			if !strings.Contains(v, "samples") || !strings.Contains(v, "✗ fail") {
				t.Errorf("embedded %v, %s says no fail:\n%s", embed, where, v)
			}
			for _, leak := range []string{"1/3", "/3", "#2", "WA", "RE", "line 2", "seven", "six", "SIGSEGV", "heap-buffer"} {
				if strings.Contains(v, leak) {
					t.Errorf("embedded %v, %s leaks %q:\n%s", embed, where, leak, v)
				}
			}
		}
		if s := m.Samples(); s.Passed != 1 || s.Failed != 2 || s.Total != 3 {
			t.Errorf("the host's counts: %+v", s)
		}
	}

	m := fixture(t, 2)
	m.opt.Quiet = true
	feed(m, judge.BuildDone{Build: judge.Build{OK: true}}, ac(0), ac(1), judge.Finished{})
	if s := screen(m); !strings.Contains(s, "samples ✓ pass") || strings.Contains(s, "2/2") || strings.Contains(s, "ALL PASS") {
		t.Errorf("all pass, quietly:\n%s", s)
	}
	// A compile error is no sample's: it shows in full, as ever.
	m = fixture(t, 2)
	m.opt.Quiet = true
	feed(m, judge.BuildDone{Build: judge.Build{OK: false, Log: "sol.cpp:3:1: error: expected ';'"}}, judge.Finished{})
	if s := screen(m); !strings.Contains(s, "expected ';'") {
		t.Errorf("CE under quiet samples:\n%s", s)
	}
}
