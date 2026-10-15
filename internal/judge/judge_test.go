package judge_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/judge"
	"katharsis/internal/problem"
	"katharsis/internal/units"
)

var (
	haveCompiler bool
	// sample is judge.SampleProfile: the sanitizer profile whose binaries
	// actually run on this machine.
	sample        judge.Profile
	both, samples []judge.Profile
	full          []judge.Profile
)

func TestMain(m *testing.M) {
	// Compile caches binaries (and SampleProfile its probe) under
	// os.UserCacheDir, which is $HOME/Library/Caches on macOS. Point HOME at a
	// scratch dir so every run starts cold and the real cache is left alone.
	home, err := os.MkdirTemp("", "katharsis-judge-test-")
	if err == nil {
		err = errors.Join(os.Setenv("HOME", home), os.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache")))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, err = exec.LookPath(judge.Compiler)
	haveCompiler = err == nil
	if haveCompiler {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		tc := judge.Profiles(ctx)
		cancel()
		sample = noPCH(tc.Samples)
		// Where g++ judges, clang's Full is the fallback elsewhere: keep
		// judging with it too.
		full = []judge.Profile{noPCH(tc.Full)}
		if tc.Full.Name != judge.Full.Name {
			full = append(full, judge.Full)
		}
		both = append([]judge.Profile{sample}, full...)
		samples = []judge.Profile{sample}
	}
	code := m.Run()
	judge.WaitPCH()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

// noPCH is p without precompiled headers. With them, which of a test's
// builds loads one would depend on which background build finished first;
// TestPCH covers them.
func noPCH(p judge.Profile) judge.Profile {
	p.PCH = false
	return p
}

func needCompiler(t *testing.T) {
	t.Helper()
	if !haveCompiler {
		t.Skipf("%s not on PATH", judge.Compiler)
	}
}

func fixture(name string) string {
	p, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		panic(err)
	}
	return p
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type memo struct {
	once sync.Once
	b    judge.Build
	err  error
}

var builds sync.Map // fixture + profile -> *memo

// compileOnce compiles testdata/<src> with p once per test binary, so
// parallel tests share binaries instead of all invoking the compiler.
func compileOnce(src string, p judge.Profile) (judge.Build, error) {
	key := strings.Join(append([]string{src, p.Name, p.Compiler}, p.Flags...), "\x00")
	v, _ := builds.LoadOrStore(key, new(memo))
	m := v.(*memo)
	m.once.Do(func() { m.b, m.err = judge.Compile(context.Background(), fixture(src), p) })
	return m.b, m.err
}

func build(t *testing.T, src string, p judge.Profile) judge.Build {
	t.Helper()
	needCompiler(t)
	b, err := compileOnce(src, p)
	if err != nil {
		t.Fatalf("compile %s [%s]: %v", src, p.Name, err)
	}
	if !b.OK {
		t.Fatalf("compile %s [%s] failed:\n%s", src, p.Name, b.Log)
	}
	return b
}

// unique is a source comment no other compile in this process shares, so a
// test that needs a cold cache gets one even under -count=N.
func unique(t *testing.T) string {
	return fmt.Sprintf("// %s %d\n", t.Name(), time.Now().UnixNano())
}

func sanitizes(p judge.Profile, what string) bool {
	return slices.ContainsFunc(p.Flags, func(f string) bool {
		return strings.HasPrefix(f, "-fsanitize=") && strings.Contains(f, what)
	})
}

// Every save is judged with the sample profile, so it must run a+b, and it
// must sanitize something.
func TestSampleProfile(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	t.Logf("SampleProfile = %q (compiler %q, flags %v)", sample.Name, sample.Compiler, sample.Flags)
	known := []string{judge.Samples.Name, judge.AppleSamples.Name, judge.HardenedSamples.Name,
		judge.LibstdcxxSamples.Name, judge.GCCSamples.Name}
	if !slices.Contains(known, sample.Name) {
		t.Errorf("SampleProfile = %q, want one of %v", sample.Name, known)
	}
	if !sanitizes(sample, "undefined") {
		t.Errorf("SampleProfile %q has no UBSan", sample.Name)
	}
	b := build(t, "ab.cpp", sample)
	c := writeCase(t, "1 2\n", "3\n")
	r, err := judge.Run(context.Background(), b.Binary, c, judge.Limits{Time: time.Second, Output: 1 << 20}, sample)
	if err != nil || r.Verdict != judge.AC {
		t.Fatalf("a+b under %s: %v %s %q\n%s", sample.Name, err, r.Verdict, r.Reason, r.Stderr)
	}
}

func writeCase(t *testing.T, in, out string) judge.Case {
	t.Helper()
	dir := t.TempDir()
	c := judge.Case{In: filepath.Join(dir, "1.in"), Out: filepath.Join(dir, "1.out")}
	write(t, c.In, in)
	write(t, c.Out, out)
	return c
}

// defaultStack drops the -stack_size link flag from each profile, as a
// control for the deep recursion fixture.
func defaultStack(ps ...judge.Profile) []judge.Profile {
	var out []judge.Profile
	for _, p := range ps {
		p.Name += ",8M-stack"
		p.Flags = slices.DeleteFunc(slices.Clone(p.Flags), func(f string) bool {
			return strings.Contains(f, "stack_size")
		})
		out = append(out, p)
	}
	return out
}

// judged reports whether p is a Full profile, clang's or g++'s.
func judged(p judge.Profile) bool {
	return p.Name == judge.Full.Name || p.Name == judge.GCCFull.Name
}

// trapsUB reports whether p traps on UB instead of reporting it
// (judge.GCCSamples).
func trapsUB(p judge.Profile) bool {
	return slices.Contains(p.Flags, "-fsanitize-trap=undefined")
}

// withHardened adds the no-ASan fallbacks, clang's and, where g++ judges,
// g++'s, unless the sample profile already is one.
func withHardened(ps []judge.Profile) []judge.Profile {
	out := slices.Clone(ps)
	fallbacks := []judge.Profile{judge.HardenedSamples}
	if sample.Libstdcxx {
		fallbacks = append(fallbacks, noPCH(judge.GCCSamples))
	}
	for _, f := range fallbacks {
		if !slices.ContainsFunc(out, func(p judge.Profile) bool { return p.Name == f.Name }) {
			out = append(out, f)
		}
	}
	return out
}

func anyOf(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestRun(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	const tl = 200 * time.Millisecond
	tests := []struct {
		name    string
		src     string
		in, out string
		lim     judge.Limits // zero fields take the defaults below
		profs   []judge.Profile
		want    judge.Verdict
		wantFor func(judge.Profile) judge.Verdict // overrides want
		check   func(t *testing.T, p judge.Profile, r judge.Result)
	}{
		{name: "AC a+b", src: "ab.cpp", in: "1 2\n", out: "3\n", profs: both, want: judge.AC},
		{name: "WA on line 2", src: "wa.cpp", in: "2 3\n", out: "5\n6\n", profs: both, want: judge.WA,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Diff == nil || *r.Diff != (judge.Diff{Line: 2, Want: "6", Got: "7"}) {
					t.Errorf("Diff = %+v, want line 2: 6 vs 7", r.Diff)
				}
				if r.Reason != "line 2" {
					t.Errorf("Reason = %q, want %q", r.Reason, "line 2")
				}
			}},
		{name: "RE null deref", src: "deref.cpp", in: "0\n", profs: both, want: judge.RE,
			check: func(t *testing.T, p judge.Profile, r judge.Result) {
				// UBSan's null check fires before the fault does.
				if sanitizes(p, "undefined") && !strings.Contains(r.Reason, "null pointer") {
					t.Errorf("Reason = %q, want UBSan's null pointer report", r.Reason)
				}
				if !sanitizes(p, "undefined") && !anyOf(r.Reason, "segmentation fault", "bus error") {
					t.Errorf("Reason = %q, want a fault signal", r.Reason)
				}
			}},
		{name: "RE wild deref", src: "deref.cpp", in: "16\n", profs: samples, want: judge.RE,
			check: func(t *testing.T, p judge.Profile, r judge.Result) {
				if !sanitizes(p, "address") {
					if !anyOf(r.Reason, "segmentation fault", "bus error") {
						t.Errorf("Reason = %q, want a fault signal", r.Reason)
					}
					return
				}
				// deref.cpp:9 is the store; a line number proves the
				// binary's dSYM was found.
				if r.Reason != "SEGV at deref.cpp:9" {
					t.Errorf("Reason = %q, want the symbolized ASan SEGV summary at deref.cpp:9", r.Reason)
				}
			}},
		{name: "RE heap overflow", src: "heapoob.cpp", in: "10\n", profs: withHardened(samples), want: judge.RE,
			check: func(t *testing.T, p judge.Profile, r judge.Result) {
				if sanitizes(p, "address") {
					// The library's bounds check fires before the store
					// reaches ASan, and ASan's abort report places it.
					want := "libc++: vector[] index out of bounds at heapoob.cpp:9"
					if p.Libstdcxx {
						want = "libstdc++: __n < this->size() at heapoob.cpp:9"
					}
					if r.Reason != want {
						t.Errorf("Reason = %q, want %q", r.Reason, want)
					}
					return
				}
				// Without ASan, the library's bounds check must catch it.
				if !anyOf(r.Stderr, "out of bounds", "__n < this->size()") {
					t.Errorf("Reason = %q, stderr = %q; want libc++'s or libstdc++'s bounds assertion", r.Reason, r.Stderr)
				}
			}},
		{name: "UB signed overflow", src: "overflow.cpp", in: "2147483647\n", profs: withHardened(samples), want: judge.RE,
			check: func(t *testing.T, p judge.Profile, r judge.Result) {
				if trapsUB(p) {
					if r.Reason != "undefined behavior (trap)" {
						t.Errorf("Reason = %q, want the UB trap", r.Reason)
					}
					return
				}
				if !strings.HasPrefix(r.Reason, "signed integer overflow") || !strings.HasSuffix(r.Reason, " at overflow.cpp:7") {
					t.Errorf("Reason = %q, want signed integer overflow ... at overflow.cpp:7", r.Reason)
				}
			}},
		{name: "RE exit status", src: "exit3.cpp", out: "3\n", profs: both, want: judge.RE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Reason != "exit 3" {
					t.Errorf("Reason = %q, want %q", r.Reason, "exit 3")
				}
			}},
		{name: "TLE infinite loop", src: "spin.cpp", lim: judge.Limits{Time: tl}, profs: both, want: judge.TLE,
			check: func(t *testing.T, p judge.Profile, r judge.Result) {
				budget := time.Duration(float64(tl) * p.Slack)
				// Killed on CPU time: at the budget under a sample
				// profile, at twice it under the judged one, within a
				// poll or two. The wall clock is no measure of that: on a
				// busy machine CPU time accrues slower than it (1.89 s of
				// wall for 0.8 s of CPU at load 23). spin.cpp never ends,
				// so short of the kill only the wall clock stops it, which
				// a machine too busy to give it the CPU by then cannot help.
				kill := 2 * budget
				if p.Slack > 1 {
					kill = budget
				}
				wallKill := 2*budget + 500*time.Millisecond // as Run sets it
				if r.CPU < kill && r.Wall >= wallKill-100*time.Millisecond {
					t.Skipf("starved: cpu %v by the wall-clock kill at %v", r.CPU, wallKill)
				}
				if !strings.HasPrefix(r.Reason, "cpu ") {
					t.Errorf("Reason = %q, want cpu ...", r.Reason)
				}
				if r.CPU < kill || r.CPU > kill+300*time.Millisecond {
					t.Errorf("CPU = %v, want killed just past %v of CPU", r.CPU, kill)
				}
			}},
		// burn.cpp uses 500 ms of CPU: over 400 ms, within any sanitizer slack.
		{name: "TLE slow but finishes", src: "burn.cpp", in: "1 2\n", out: "3\n", lim: judge.Limits{Time: 2 * tl},
			profs: full, want: judge.TLE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				// The wall-clock kill is at 1.3 s; finishing earlier shows
				// the verdict came from CPU time. A machine too busy to give
				// it 500 ms of CPU by then cannot show that.
				if r.Wall >= 1300*time.Millisecond {
					if r.CPU < 500*time.Millisecond {
						t.Skipf("starved: cpu %v by the kill", r.CPU)
					}
					t.Errorf("Wall = %v, want under the 1.3s kill", r.Wall)
				}
			}},
		// A blocked run only the wall-clock kill stops: its reason must not
		// claim it used more CPU than the budget.
		{name: "TLE idle", src: "idle.cpp", lim: judge.Limits{Time: tl}, profs: full, want: judge.TLE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if !strings.HasPrefix(r.Reason, "wall ") {
					t.Errorf("Reason = %q, want wall ...", r.Reason)
				}
			}},
		{name: "AC slow within sanitizer slack", src: "burn.cpp", in: "1 2\n", out: "3\n",
			lim: judge.Limits{Time: 2 * tl}, profs: samples, want: judge.AC},
		{name: "OLE infinite print", src: "spam.cpp", profs: both, want: judge.OLE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Reason != "output over 1M" {
					t.Errorf("Reason = %q, want %q", r.Reason, "output over 1M")
				}
				if r.Wall > 2*time.Second {
					t.Errorf("Wall = %v, want killed at the cap, not the time limit", r.Wall)
				}
			}},
		{name: "OLE output file", src: "fileio_big.cpp", profs: full, want: judge.OLE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Reason != "x.out over 1M" {
					t.Errorf("Reason = %q, want %q", r.Reason, "x.out over 1M")
				}
			}},
		{name: "AC file io", src: "fileio.cpp", in: "1 2\n", out: "3\n", profs: both, want: judge.AC},
		// The answer is judged, not the I/O style: reading x.in and
		// printing to stdout is still an answer.
		{name: "AC file io answering on stdout", src: "fileio_noout.cpp", in: "1 2\n", out: "3\n", profs: both, want: judge.AC},
		{name: "WA file io answering nowhere", src: "fileio_silent.cpp", in: "1 2\n", out: "3\n", profs: full, want: judge.WA,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Reason != "no x.out written" {
					t.Errorf("Reason = %q, want %q", r.Reason, "no x.out written")
				}
			}},
		// usaco:567 with ifstream fin("paint.in") was WA 0/10.
		{name: "AC file io with fstreams", src: "fileio_fstream.cpp", in: "1 2\n", out: "3\n", profs: both, want: judge.AC},
		{name: "AC file io named at run time, problem stem known", src: "fileio_hidden.cpp", in: "1 2\n", out: "3\n",
			lim: judge.Limits{FileIO: "x"}, profs: full, want: judge.AC},
		{name: "RE file io named at run time, stem unknown", src: "fileio_hidden.cpp", in: "1 2\n", out: "3\n",
			profs: full, want: judge.RE},
		{name: "AC stdio program on a file io problem", src: "ab.cpp", in: "1 2\n", out: "3\n",
			lim: judge.Limits{FileIO: "x"}, profs: full, want: judge.AC},
		{name: "RE division by zero", src: "divzero.cpp", in: "1 0\n", profs: withHardened(both), want: judge.RE,
			check: func(t *testing.T, p judge.Profile, r judge.Result) {
				want := "division by zero at divzero.cpp:7"
				switch {
				case judged(p):
					want = "integer division by zero" // a trap: no runtime to say where
				case trapsUB(p):
					want = "undefined behavior (trap)"
				}
				if r.Reason != want {
					t.Errorf("Reason = %q, want %q", r.Reason, want)
				}
			}},
		{name: "RE uncaught exception", src: "throw.cpp", in: "1\n", profs: both, want: judge.RE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Reason != "uncaught std::runtime_error: boom" {
					t.Errorf("Reason = %q, want the exception's type and what()", r.Reason)
				}
			}},
		{name: "RE assert", src: "assert.cpp", in: "7\n", profs: both, want: judge.RE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Reason != "assertion x != 7 failed at assert.cpp:8" {
					t.Errorf("Reason = %q, want the assertion and its line", r.Reason)
				}
			}},
		{name: "RE report after debug output", src: "noisy_oob.cpp", in: "10\n", profs: samples, want: judge.RE,
			check: func(t *testing.T, p judge.Profile, r judge.Result) {
				want := "libc++: vector[] index out of bounds at noisy_oob.cpp:11"
				if p.Libstdcxx {
					want = "libstdc++: __n < this->size() at noisy_oob.cpp:11"
				}
				if sanitizes(p, "address") && r.Reason != want {
					t.Errorf("Reason = %q, want %q, from the report after 64K of stderr", r.Reason, want)
				}
				if !strings.Contains(r.Stderr, "of stderr cut") {
					t.Errorf("Stderr is %d bytes with no cut; want head and tail", len(r.Stderr))
				}
			}},
		{name: "AC forked child left running", src: "fork.cpp", in: "1 2\n", out: "3\n", profs: full, want: judge.AC,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Wall > 500*time.Millisecond {
					t.Errorf("Wall = %v, want the run over when the program exits", r.Wall)
				}
				// The child is in the run's process group, killed with it.
				if out, _ := exec.Command("pgrep", "-f", "sleep 31.41592").Output(); len(out) > 0 {
					_ = exec.Command("pkill", "-f", "sleep 31.41592").Run()
					t.Errorf("forked child still running after Run returned (pids %q)", out)
				}
			}},
		{name: "MLE static data", src: "static.cpp", in: "5 5\n", out: "1\n", profs: full, want: judge.MLE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Reason != "static data 1.2G > 256M" {
					t.Errorf("Reason = %q, want the static size against the limit", r.Reason)
				}
			}},
		{name: "AC static data within the limit", src: "static.cpp", in: "5 5\n", out: "1\n",
			lim: judge.Limits{Mem: 2 << 30}, profs: full, want: judge.AC},
		{name: "AC GNU-isms", src: "gnu.cpp", in: "12 18\n", out: "6 10 12\n", profs: withHardened(both), want: judge.AC},
		{name: "AC deep recursion", src: "recurse.cpp", in: "1000000\n", out: "1000000\n", profs: both, want: judge.AC},
		{name: "RE deep recursion on 8M stack", src: "recurse.cpp", in: "1000000\n", out: "1000000\n",
			profs: defaultStack(both...), want: judge.RE,
			check: func(t *testing.T, p judge.Profile, r judge.Result) {
				if sanitizes(p, "address") && !strings.Contains(r.Reason, "stack-overflow") {
					t.Errorf("Reason = %q, want ASan's stack-overflow", r.Reason)
				}
			}},
		// 300M is over the 256M limit but under the watchdog's 320M, so the
		// run finishes and is judged on its peak.
		{name: "MLE on peak", src: "mle.cpp", in: "1\n", out: "ok\n", lim: judge.Limits{Mem: 256 << 20}, profs: full, want: judge.MLE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if !strings.HasPrefix(r.Reason, "peak ") || r.MaxRSS < 300<<20 {
					t.Errorf("Reason = %q, MaxRSS = %s; want peak over 300M", r.Reason, units.Mem(r.MaxRSS))
				}
			}},
		{name: "MLE runaway killed", src: "hog.cpp", lim: judge.Limits{Mem: 64 << 20}, profs: full, want: judge.MLE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if !strings.HasPrefix(r.Reason, "killed at ") || !strings.HasSuffix(r.Reason, "> 64M") {
					t.Errorf("Reason = %q, want killed at ... > 64M", r.Reason)
				}
				// The cap is 80M; a poll every 20 ms lets it overshoot by
				// a chunk or two, not run on to the fixture's 2G.
				if r.MaxRSS > 512<<20 {
					t.Errorf("MaxRSS = %s, want the kill near 80M", units.Mem(r.MaxRSS))
				}
			}},
		{name: "TLE under a stricter Scale", src: "burn.cpp", in: "1 2\n", out: "3\n",
			lim: judge.Limits{Time: time.Second, Scale: 0.25}, profs: full, want: judge.TLE,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if !strings.HasSuffix(r.Reason, "> 250ms") {
					t.Errorf("Reason = %q, want a 250ms budget", r.Reason)
				}
			}},
		{name: "AC under a looser Scale", src: "burn.cpp", in: "1 2\n", out: "3\n",
			lim: judge.Limits{Time: 300 * time.Millisecond, Scale: 2}, profs: full, want: judge.AC},
		// float.cpp prints 0.333333333; the answer file says 0.3333333.
		{name: "AC float within tolerance", src: "float.cpp", in: "3\n", out: "0.3333333 ok\n",
			lim: judge.Limits{FloatTolerance: 1e-6}, profs: full, want: judge.AC},
		{name: "WA float without tolerance", src: "float.cpp", in: "3\n", out: "0.3333333 ok\n", profs: full, want: judge.WA},
		{name: "WA float word mismatch", src: "float.cpp", in: "4\n", out: "0.25 OK\n",
			lim: judge.Limits{FloatTolerance: 1e-6}, profs: full, want: judge.WA,
			check: func(t *testing.T, _ judge.Profile, r judge.Result) {
				if r.Diff == nil || *r.Diff != (judge.Diff{Line: 1, Want: "OK", Got: "ok"}) {
					t.Errorf("Diff = %+v, want OK vs ok on line 1", r.Diff)
				}
			}},
		{name: "memory unchecked only when sanitized", src: "mle.cpp", in: "1\n", out: "ok\n",
			lim: judge.Limits{Mem: 64 << 20}, profs: samples,
			wantFor: func(p judge.Profile) judge.Verdict {
				if p.Sanitized {
					return judge.AC
				}
				return judge.MLE
			}},
	}
	for _, tc := range tests {
		lim := tc.lim
		if lim.Time == 0 {
			lim.Time = time.Second
		}
		if lim.Mem == 0 {
			lim.Mem = 256 << 20
		}
		if lim.Output == 0 {
			lim.Output = 1 << 20
		}
		for _, p := range tc.profs {
			t.Run(tc.name+"/"+p.Name, func(t *testing.T) {
				t.Parallel()
				b := build(t, tc.src, p)
				c := writeCase(t, tc.in, tc.out)
				c.FileIO = problem.FileIO(b.Src)
				r, err := judge.Run(context.Background(), b.Binary, c, lim, p)
				if err != nil {
					t.Fatalf("Run: %v", err)
				}
				t.Logf("%s %q cpu=%v wall=%v rss=%s", r.Verdict, r.Reason,
					r.CPU.Round(time.Millisecond), r.Wall.Round(time.Millisecond), units.Mem(r.MaxRSS))
				want := tc.want
				if tc.wantFor != nil {
					want = tc.wantFor(p)
				}
				if r.Verdict != want {
					t.Fatalf("Verdict = %s (%q), want %s\nstderr:\n%s", r.Verdict, r.Reason, want, r.Stderr)
				}
				if tc.check != nil {
					tc.check(t, p, r)
				}
			})
		}
	}
}

// The clang + libc++ fallback's sample profiles check vector, string and
// std::array indexing with libc++'s debug hardening; ASan alone misses the
// last two. ASan's abort report says where.
func TestLibcxxHardening(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	for _, p := range []judge.Profile{judge.Samples, judge.AppleSamples, judge.HardenedSamples} {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			if b, err := judge.Compile(context.Background(), fixture("ab.cpp"), p); err != nil || !b.OK {
				t.Skipf("%s does not build here: %v", p.Name, err)
			} else if r, err := judge.Run(context.Background(), b.Binary, writeCase(t, "1 2\n", "3\n"), quick, p); err != nil || r.Verdict != judge.AC {
				t.Skipf("%s does not run here: %v %s %q", p.Name, err, r.Verdict, r.Reason)
			}
			for _, tc := range []struct{ src, in, want, at string }{
				{"heapoob.cpp", "10\n", "libc++: vector[] index out of bounds", "heapoob.cpp:9"},
				{"string_oob.cpp", "5\n", "libc++: string index out of bounds", "string_oob.cpp:10"},
				{"array_oob.cpp", "4\n", "libc++: out-of-bounds access in std::array<T, N>", "array_oob.cpp:16"},
			} {
				b := build(t, tc.src, p)
				r, err := judge.Run(context.Background(), b.Binary, writeCase(t, tc.in, "0\n"), quick, p)
				if err != nil {
					t.Fatal(err)
				}
				want := tc.want
				if sanitizes(p, "address") {
					want += " at " + tc.at
				}
				if r.Verdict != judge.RE || r.Reason != want {
					t.Errorf("%s: %s %q, want RE %q\n%s", tc.src, r.Verdict, r.Reason, want, r.Stderr)
				}
			}
		})
	}
}

func TestCompileError(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	for _, p := range both {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			b, err := judge.Compile(context.Background(), fixture("ce.cpp"), p)
			if err != nil {
				t.Fatalf("Compile: %v (a compile error is not a Go error)", err)
			}
			if b.OK {
				t.Fatal("OK = true for a broken source")
			}
			if !strings.Contains(b.Log, "error") {
				t.Errorf("Log = %q, want the compiler's error", b.Log)
			}
			if _, err := os.Stat(b.Binary); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("binary %s exists after a failed build (stat: %v)", b.Binary, err)
			}
		})
	}
}

func TestCompileCache(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	ctx := context.Background()
	src := filepath.Join(t.TempDir(), "cache.cpp")
	tag := unique(t)
	write(t, src, "int main() { return 0; }\n"+tag)

	first, err := judge.Compile(ctx, src, judge.Full)
	if err != nil || !first.OK {
		t.Fatalf("Compile: %v, OK=%v\n%s", err, first.OK, first.Log)
	}
	if first.Cached || first.Duration <= 0 {
		t.Errorf("first build: Cached=%v Duration=%v, want a real compile", first.Cached, first.Duration)
	}

	again, err := judge.Compile(ctx, src, judge.Full)
	if err != nil || !again.OK {
		t.Fatalf("Compile again: %v, OK=%v", err, again.OK)
	}
	if !again.Cached || again.Binary != first.Binary || string(again.Src) != string(first.Src) {
		t.Errorf("second build: Cached=%v Binary=%s; want cache hit on %s", again.Cached, again.Binary, first.Binary)
	}

	other, err := judge.Compile(ctx, src, sample)
	if err != nil || !other.OK {
		t.Fatalf("Compile %s: %v, OK=%v\n%s", sample.Name, err, other.OK, other.Log)
	}
	if other.Cached || other.Binary == first.Binary {
		t.Errorf("other profile: Cached=%v Binary=%s; want a separate build", other.Cached, other.Binary)
	}
	// clang -g on Darwin writes a dSYM; the symbolizer looks for it next to
	// the binary under the binary's own name.
	if runtime.GOOS == "darwin" && slices.Contains(sample.Flags, "-g") {
		if _, err := os.Stat(other.Binary + ".dSYM"); err != nil {
			t.Errorf("no dSYM beside %s: %v", other.Binary, err)
		}
	}

	write(t, src, "int main() { return 1; }\n"+tag)
	edited, err := judge.Compile(ctx, src, judge.Full)
	if err != nil || !edited.OK {
		t.Fatalf("Compile edited: %v, OK=%v", err, edited.OK)
	}
	if edited.Cached || edited.Binary == first.Binary {
		t.Errorf("edited source: Cached=%v Binary=%s; want a new build", edited.Cached, edited.Binary)
	}
}

// Two judge passes on the same unchanged file (":w" twice) compile the same
// source with the same profile; both must get a usable binary.
func TestCompileConcurrentSameSource(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	for _, p := range both {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join(t.TempDir(), "race.cpp")
			write(t, src, "#include <cstdio>\nint main() { puts(\"race\"); }\n"+unique(t))
			const n = 6
			var wg sync.WaitGroup
			errs := make([]error, n)
			for i := range n {
				wg.Go(func() {
					b, err := judge.Compile(context.Background(), src, p)
					switch {
					case err != nil:
						errs[i] = err
					case !b.OK:
						errs[i] = fmt.Errorf("OK=false: %s", b.Log)
					default:
						if _, err := os.Stat(b.Binary); err != nil {
							errs[i] = err
						}
					}
				})
			}
			wg.Wait()
			for i, err := range errs {
				if err != nil {
					t.Errorf("compile %d: %v", i, err)
				}
			}
		})
	}
}

func TestCompileCancelled(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	src := filepath.Join(t.TempDir(), "cancel.cpp")
	write(t, src, "int main() {}\n"+unique(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, err := judge.Compile(ctx, src, judge.Full)
	if err == nil || b.OK {
		t.Fatalf("Compile on a cancelled context: err=%v OK=%v, want an error", err, b.OK)
	}
	if _, err := os.Stat(b.Binary); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cancelled build left %s (stat: %v)", b.Binary, err)
	}
}

type testFile struct{ name, in, out string }

// load builds a problem dir from the fixture and tests, then loads it.
func load(t *testing.T, src string, tests []testFile) *problem.Problem {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "p")
	body, err := os.ReadFile(fixture(src))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "sol.cpp"), string(body))
	for _, f := range tests {
		write(t, filepath.Join(dir, "tests", f.name+".in"), f.in)
		write(t, filepath.Join(dir, "tests", f.name+".out"), f.out)
	}
	p, err := problem.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// kinds renders an event stream as one line, e.g.
// "BuildDone(ok) Test0(AC) Finished".
func kinds(evs []judge.Event) string {
	var s []string
	for _, e := range evs {
		switch e := e.(type) {
		case judge.BuildDone:
			if e.Build.OK {
				s = append(s, "BuildDone(ok)")
			} else {
				s = append(s, "BuildDone(ce)")
			}
		case judge.TestDone:
			s = append(s, fmt.Sprintf("Test%d(%s)", e.Index, e.Result.Verdict))
		case judge.Finished:
			if e.Err != nil {
				s = append(s, "Finished(err)")
			} else {
				s = append(s, "Finished")
			}
		default:
			s = append(s, fmt.Sprintf("%T", e))
		}
	}
	return strings.Join(s, " ")
}

var quick = judge.Limits{Time: time.Second, Mem: 256 << 20, Output: 1 << 20}

func TestJudgeEvents(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	tests := []struct {
		name  string
		src   string
		files []testFile
		want  string
	}{
		{"tests in natural order", "ab.cpp",
			[]testFile{{"10", "5 5\n", "10\n"}, {"2", "2 2\n", "5\n"}, {"1", "1 2\n", "3\n"}},
			"BuildDone(ok) Test0(AC) Test1(WA) Test2(AC) Finished"},
		{"no tests", "ab.cpp", nil, "BuildDone(ok) Finished"},
		{"compile error", "ce.cpp", []testFile{{"1", "1 2\n", "3\n"}}, "BuildDone(ce) Finished"},
		{"file io", "fileio.cpp", []testFile{{"1", "1 2\n", "3\n"}, {"2", "4 4\n", "8\n"}},
			"BuildDone(ok) Test0(AC) Test1(AC) Finished"},
	}
	for _, tc := range tests {
		for _, p := range both {
			t.Run(tc.name+"/"+p.Name, func(t *testing.T) {
				t.Parallel()
				if tc.src != "ce.cpp" {
					build(t, tc.src, p) // warm the cache; see compileOnce
				}
				var evs []judge.Event
				judge.Judge(context.Background(), load(t, tc.src, tc.files), p, quick, func(e judge.Event) {
					evs = append(evs, e)
				})
				if got := kinds(evs); got != tc.want {
					t.Errorf("events:\n got %s\nwant %s", got, tc.want)
				}
			})
		}
	}
}

func TestJudgeCancelled(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	files := []testFile{{"1", "1 2\n", "3\n"}, {"2", "-1 0\n", "-1\n"}} // ab.cpp spins on test 2

	t.Run("before start", func(t *testing.T) {
		t.Parallel()
		build(t, "ab.cpp", judge.Full)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var evs []judge.Event
		judge.Judge(ctx, load(t, "ab.cpp", files), judge.Full, quick, func(e judge.Event) { evs = append(evs, e) })
		if len(evs) != 0 {
			t.Errorf("events = %s, want none", kinds(evs))
		}
	})

	t.Run("during a run", func(t *testing.T) {
		t.Parallel()
		build(t, "ab.cpp", judge.Full)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		lim := quick
		lim.Time = 10 * time.Second // the wall-clock kill would come at 20.5s
		var evs []judge.Event
		start := time.Now()
		judge.Judge(ctx, load(t, "ab.cpp", files), judge.Full, lim, func(e judge.Event) {
			evs = append(evs, e)
			if td, ok := e.(judge.TestDone); ok && td.Index == 0 {
				time.AfterFunc(300*time.Millisecond, cancel)
			}
		})
		if el := time.Since(start); el > 5*time.Second {
			t.Errorf("Judge returned after %v, want soon after the cancel", el)
		}
		if got, want := kinds(evs), "BuildDone(ok) Test0(AC)"; got != want {
			t.Errorf("events:\n got %s\nwant %s", got, want)
		}
	})
}

// Not parallel: it swaps the package-level Compiler while parallel tests are
// still parked.
func TestJudgeMissingCompiler(t *testing.T) {
	old := judge.Compiler
	judge.Compiler = "katharsis-no-such-compiler"
	defer func() { judge.Compiler = old }()
	var evs []judge.Event
	judge.Judge(context.Background(), load(t, "ab.cpp", []testFile{{"1", "1 2\n", "3\n"}}), judge.Full, quick,
		func(e judge.Event) { evs = append(evs, e) })
	if got := kinds(evs); got != "Finished(err)" {
		t.Errorf("events = %s, want Finished(err)", got)
	}
}
