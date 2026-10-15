//go:build qa

// Package judgeqa attacks internal/judge with crafted programs. It is a
// report harness, not a regression suite: a FAIL marks a place where the
// local verdict differs from what a real judge (Codeforces, USACO) would
// give, or where the report is not what a player needs.
//
//	nix develop -c go test -tags qa -v ./qa/judge/...
//
// Outside the dev shell, build the binary inside it and run it outside:
//
//	nix develop -c go test -tags qa -c -o /tmp/judgeqa.test ./qa/judge
//	/tmp/judgeqa.test -test.v
//
// HOME points at a scratch dir, so the build cache and the probe cache start
// cold and the player's own caches are never touched.
package judgeqa

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/judge"
)

var (
	haveCompiler bool
	sample       judge.Profile
	probeTook    time.Duration
	// profiles is every sample-ish profile that runs here, plus Full.
	profiles []judge.Profile
	home     string
)

// helperMode is set when the binary was started as a subprocess by one of
// the probe tests; TestMain then runs that helper instead of the suite.
const helperEnv = "JUDGEQA_HELPER"

func TestMain(m *testing.M) {
	if h := os.Getenv(helperEnv); h != "" {
		os.Exit(runHelper(h))
	}
	var err error
	home, err = os.MkdirTemp("", "judgeqa-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	os.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	_, err = exec.LookPath(judge.Compiler)
	haveCompiler = err == nil
	if haveCompiler {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		start := time.Now()
		sample = judge.SampleProfile(ctx)
		probeTook = time.Since(start)
		cancel()
		profiles = discoverProfiles()
	}
	fmt.Fprintf(os.Stderr, "judgeqa: compiler %s, SampleProfile %q (probe %v), profiles %v\n",
		resolved(judge.Compiler), sample.Name, probeTook.Round(time.Millisecond), names(profiles))
	code := m.Run()
	printMatrix()
	os.RemoveAll(home)
	os.Exit(code)
}

func resolved(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return name + " (not found)"
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func names(ps []judge.Profile) []string {
	var s []string
	for _, p := range ps {
		s = append(s, p.Name)
	}
	return s
}

// discoverProfiles returns Full, the chosen sample profile, and every other
// candidate whose hello world runs within 3 s (as probe.go's runs does).
func discoverProfiles() []judge.Profile {
	ps := []judge.Profile{judge.Full, sample}
	for _, p := range []judge.Profile{judge.Samples, judge.AppleSamples, judge.HardenedSamples} {
		if p.Name == sample.Name {
			continue
		}
		if helloRuns(p) {
			ps = append(ps, p)
		} else {
			fmt.Fprintf(os.Stderr, "judgeqa: profile %q does not run here; skipped\n", p.Name)
		}
	}
	return ps
}

func helloRuns(p judge.Profile) bool {
	dir, err := os.MkdirTemp("", "judgeqa-hello-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "hello.cpp")
	os.WriteFile(src, []byte("#include <cstdio>\nint main(){puts(\"ok\");}\n"), 0o644)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	b, err := judge.Compile(ctx, src, p)
	if err != nil || !b.OK {
		return false
	}
	rctx, rcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer rcancel()
	out, err := exec.CommandContext(rctx, b.Binary).Output()
	return err == nil && strings.TrimSpace(string(out)) == "ok"
}

func needCompiler(t *testing.T) {
	t.Helper()
	if !haveCompiler {
		t.Skipf("%s not on PATH", judge.Compiler)
	}
}

func profileNamed(name string) (judge.Profile, bool) {
	for _, p := range profiles {
		if p.Name == name {
			return p, true
		}
	}
	return judge.Profile{}, false
}

func sanitizes(p judge.Profile, what string) bool {
	return slices.ContainsFunc(p.Flags, func(f string) bool {
		return strings.HasPrefix(f, "-fsanitize=") && strings.Contains(f, what)
	})
}

// ---- programs and builds --------------------------------------------------

var (
	buildMu sync.Mutex
	builds  = map[string]*buildMemo{}
	srcDir  string
	srcOnce sync.Once
)

type buildMemo struct {
	once sync.Once
	b    judge.Build
	err  error
}

// source writes body as <name>.cpp in a shared scratch dir (one per
// process) and returns its path.
func source(t testing.TB, name, body string) string {
	t.Helper()
	srcOnce.Do(func() {
		d, err := os.MkdirTemp("", "judgeqa-src-")
		if err != nil {
			panic(err)
		}
		srcDir = d
	})
	dir := filepath.Join(srcDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "sol.cpp")
	if old, err := os.ReadFile(p); err != nil || string(old) != body {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// build compiles body once per (name, profile) per process.
func build(t testing.TB, name, body string, p judge.Profile) judge.Build {
	t.Helper()
	needCompilerTB(t)
	key := name + "\x00" + p.Name
	buildMu.Lock()
	m, ok := builds[key]
	if !ok {
		m = new(buildMemo)
		builds[key] = m
	}
	buildMu.Unlock()
	src := source(t, name, body)
	m.once.Do(func() { m.b, m.err = judge.Compile(context.Background(), src, p) })
	if m.err != nil {
		t.Fatalf("compile %s [%s]: %v", name, p.Name, m.err)
	}
	return m.b
}

func needCompilerTB(t testing.TB) {
	t.Helper()
	if !haveCompiler {
		t.Skipf("%s not on PATH", judge.Compiler)
	}
}

func writeCase(t testing.TB, in, out string) judge.Case {
	t.Helper()
	dir := t.TempDir()
	c := judge.Case{In: filepath.Join(dir, "1.in"), Out: filepath.Join(dir, "1.out")}
	if err := os.WriteFile(c.In, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.Out, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	return c
}

// ---- the result matrix ----------------------------------------------------

type cell struct {
	row, col string
	text     string
}

var (
	matrixMu sync.Mutex
	matrix   []cell
)

func record(row, col string, r judge.Result, err error) {
	text := ""
	switch {
	case err != nil:
		text = "ERR " + err.Error()
	default:
		text = fmt.Sprintf("%s %q cpu=%dms wall=%dms rss=%s", r.Verdict, r.Reason,
			r.CPU.Milliseconds(), r.Wall.Milliseconds(), judge.Bytes(r.MaxRSS))
		if r.Diff != nil {
			text += fmt.Sprintf(" diff{line %d want %q got %q}", r.Diff.Line, r.Diff.Want, r.Diff.Got)
		}
	}
	note(row, col, text)
}

func note(row, col, text string) {
	matrixMu.Lock()
	defer matrixMu.Unlock()
	matrix = append(matrix, cell{row, col, text})
}

func printMatrix() {
	matrixMu.Lock()
	defer matrixMu.Unlock()
	if len(matrix) == 0 {
		return
	}
	sort.SliceStable(matrix, func(i, j int) bool {
		if matrix[i].row != matrix[j].row {
			return matrix[i].row < matrix[j].row
		}
		return matrix[i].col < matrix[j].col
	})
	fmt.Fprintln(os.Stderr, "\n==== judgeqa matrix ====")
	last := ""
	for _, c := range matrix {
		if c.row != last {
			fmt.Fprintf(os.Stderr, "%s\n", c.row)
			last = c.row
		}
		fmt.Fprintf(os.Stderr, "    %-22s %s\n", c.col, c.text)
	}
}
