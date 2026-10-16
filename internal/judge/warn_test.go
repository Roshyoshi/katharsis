package judge_test

import (
	"context"
	"strings"
	"testing"

	"katharsis/internal/judge"
)

// The sample builds' warnings (SampleWarnings), on this machine's sample
// profiles (the precompiled-header one boots use among them) and on
// libc++'s, which force-includes the bits/stdc++.h shim: an
// uninitialized read fails the build on its line, a shadowed variable is
// reported and builds, and a clean source reports nothing, the headers,
// libc++'s shim and the driver included.
func TestSampleWarnings(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	tc := judge.Profiles(context.Background())
	for _, p := range []judge.Profile{sample, tc.Samples, hardened, judge.Samples} {
		p := p.WithWarnings(judge.SampleWarnings)
		t.Run(p.Name+pchName(p), func(t *testing.T) {
			t.Parallel()
			b, err := compileOnce("uninit.cpp", p)
			if err != nil {
				t.Fatal(err)
			}
			if b.OK || !strings.Contains(b.Log, "uninit.cpp:8:") || !strings.Contains(b.Log, "uninitialized") {
				t.Errorf("an uninitialized loop variable built (ok %v):\n%s", b.OK, b.Log)
			}
			b = build(t, "shadow.cpp", p)
			if w := judge.Warnings(b.Log); len(w) != 1 || !strings.Contains(w[0], "shadow.cpp:6:") || !strings.Contains(w[0], "shadow") {
				t.Errorf("shadow.cpp warnings = %q, want the one on line 6; log:\n%s", w, b.Log)
			}
			b = build(t, "template.cpp", p)
			if w := judge.Warnings(b.Log); len(w) > 0 {
				t.Errorf("a clean source warned %q", w)
			}
		})
	}
}

func pchName(p judge.Profile) string {
	if p.PCH {
		return " pch"
	}
	return ""
}

func TestWarnings(t *testing.T) {
	t.Parallel()
	log := "clang++: warning: argument unused during compilation: '-L/x' [-Wunused-command-line-argument]\n" +
		"/tmp/a.cpp:47:17: warning: variable 'i' is uninitialized when used here [-Wuninitialized]\n" +
		"   47 |     for (int i; i < t; i++) {\n" +
		"      |                 ^\n" +
		"/tmp/a.cpp:47:15: note: initialize the variable 'i' to silence this warning\n" +
		"ld: warning: ignoring duplicate libraries: '-lc++'\n" +
		"b.cpp:3: warning: no column\n"
	want := []string{
		"/tmp/a.cpp:47:17: warning: variable 'i' is uninitialized when used here [-Wuninitialized]",
		"b.cpp:3: warning: no column",
	}
	if got := judge.Warnings(log); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Warnings = %q, want %q", got, want)
	}
}
