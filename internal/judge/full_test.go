package judge_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"deadcases/internal/judge"
	"deadcases/internal/problem"
)

func TestSummarize(t *testing.T) {
	t.Parallel()
	r := func(v judge.Verdict, cpu time.Duration) judge.Result {
		return judge.Result{Verdict: v, CPU: cpu, Reason: string(v) + " reason"}
	}
	var pending judge.Result
	ms := time.Millisecond
	tests := []struct {
		name string
		rs   []judge.Result
		want judge.Summary
	}{
		{"no tests", nil, judge.Summary{First: -1, Verdict: judge.AC}},
		{"nothing done", []judge.Result{pending, pending},
			judge.Summary{Total: 2, First: -1}},
		{"clean so far", []judge.Result{r(judge.AC, 3*ms), pending},
			judge.Summary{Total: 2, Done: 1, Passed: 1, First: -1, MaxCPU: 3 * ms}},
		{"all pass", []judge.Result{r(judge.AC, 3*ms), r(judge.AC, 9*ms)},
			judge.Summary{Total: 2, Done: 2, Passed: 2, First: -1, Verdict: judge.AC, MaxCPU: 9 * ms}},
		{"failure behind a pending test", []judge.Result{pending, r(judge.WA, ms)},
			judge.Summary{Total: 2, Done: 1, First: 1, Verdict: judge.WA, Reason: "WA reason", MaxCPU: ms}},
		{"first in test order wins", []judge.Result{r(judge.AC, ms), r(judge.TLE, 2*ms), r(judge.WA, ms)},
			judge.Summary{Total: 3, Done: 3, Passed: 1, First: 1, Verdict: judge.TLE, Reason: "TLE reason", MaxCPU: 2 * ms}},
	}
	for _, tc := range tests {
		if got := judge.Summarize(tc.rs); got != tc.want {
			t.Errorf("%s: Summarize =\n %+v\nwant\n %+v", tc.name, got, tc.want)
		}
	}
}

// fullPass runs JudgeFull on src over files and returns the summary and the
// TestDone events in emit order.
func fullPass(t *testing.T, ctx context.Context, src string, files []testFile, lim judge.Limits) (judge.Summary, []judge.TestDone, error) {
	t.Helper()
	b := build(t, src, judge.Full)
	p := load(t, src, files)
	var evs []judge.TestDone
	s, err := judge.JudgeFull(ctx, p.Tests, problem.FileIO(b.Src), b, lim, judge.Full, func(e judge.Event) {
		td, ok := e.(judge.TestDone)
		if !ok {
			t.Errorf("JudgeFull emitted %T, want TestDone only", e)
			return
		}
		evs = append(evs, td)
	})
	return s, evs, err
}

func indices(evs []judge.TestDone) []int {
	var is []int
	for _, e := range evs {
		is = append(is, e.Index)
	}
	return is
}

func TestJudgeFull(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	// ab.cpp: test 2 spins (a < 0) until the wall-clock kill at 0.9s, while
	// tests 4 and 7 are wrong and finish at once. The verdict must be test
	// 2's TLE, the first in test order, though both WAs finish first.
	var files []testFile
	for i := 1; i <= 8; i++ {
		in, out := fmt.Sprintf("%d %d\n", i, i), fmt.Sprintf("%d\n", 2*i)
		switch i {
		case 2:
			in = "-1 0\n"
		case 4, 7:
			out = "0\n"
		}
		files = append(files, testFile{fmt.Sprint(i), in, out})
	}
	lim := quick
	lim.Time = 200 * time.Millisecond
	s, evs, err := fullPass(t, context.Background(), "ab.cpp", files, lim)
	if err != nil {
		t.Fatal(err)
	}
	got := indices(evs)
	t.Logf("completion order %v (Workers=%d)", got, judge.Workers)
	if sorted := slices.Sorted(slices.Values(got)); !slices.Equal(sorted, []int{0, 1, 2, 3, 4, 5, 6, 7}) {
		t.Fatalf("TestDone indices %v, want each of 0..7 once", got)
	}
	want := judge.Summary{Total: 8, Done: 8, Passed: 5, First: 1, Verdict: judge.TLE}
	if s.Total != want.Total || s.Done != want.Done || s.Passed != want.Passed || s.First != want.First || s.Verdict != want.Verdict {
		t.Errorf("Summary = %+v, want %+v", s, want)
	}
	for _, e := range evs {
		if e.Result.Rerun {
			t.Errorf("test %d was rerun; only borderline TLEs are", e.Index)
		}
	}
	if judge.Workers > 1 && got[len(got)-1] != 1 {
		t.Errorf("completion order %v: the spinning test should finish last", got)
	}
}

// burn.cpp uses 500 ms of CPU. Against a 450 ms budget that TLE is within
// the parallel drift, so JudgeFull reruns it alone; it stays TLE because
// burn.cpp burns CPU time, not a fixed amount of work.
func TestJudgeFullRerunsBorderlineTLE(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	if judge.Workers < 2 {
		t.Skip("one worker: nothing runs in parallel, nothing to rerun")
	}
	files := []testFile{{"1", "1 2\n", "3\n"}, {"2", "2 2\n", "4\n"}, {"3", "3 3\n", "6\n"}}
	lim := quick
	lim.Time = 450 * time.Millisecond
	s, evs, err := fullPass(t, context.Background(), "burn.cpp", files, lim)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 3 {
		t.Fatalf("got %d TestDone events, want 3", len(evs))
	}
	for _, e := range evs {
		if e.Result.Verdict != judge.TLE || !e.Result.Rerun {
			t.Errorf("test %d: %s rerun=%v cpu=%v; want a rerun TLE", e.Index, e.Result.Verdict, e.Result.Rerun, e.Result.CPU)
		}
	}
	// Reruns are sequential and in test order.
	if got := indices(evs); !slices.Equal(got, []int{0, 1, 2}) {
		t.Errorf("rerun order %v, want [0 1 2]", got)
	}
	if s.First != 0 || s.Verdict != judge.TLE {
		t.Errorf("Summary = %+v, want first failure 0 TLE", s)
	}
}

func TestJudgeFullCancelled(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	files := []testFile{{"1", "1 2\n", "3\n"}}
	for i := 2; i <= 12; i++ {
		files = append(files, testFile{fmt.Sprint(i), "-1 0\n", "-1\n"}) // spins
	}
	lim := quick
	lim.Time = 10 * time.Second // the wall-clock kill would come at 20.5s
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(500*time.Millisecond, cancel)
	start := time.Now()
	_, evs, err := fullPass(t, ctx, "ab.cpp", files, lim)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if el := time.Since(start); el > 5*time.Second {
		t.Errorf("JudgeFull returned after %v, want soon after the cancel", el)
	}
	if got := indices(evs); !slices.Equal(got, []int{0}) {
		t.Errorf("events for %v, want only the AC test 0", got)
	}
}

func TestJudgeFullRunError(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	b := build(t, "ab.cpp", judge.Full)
	p := load(t, "ab.cpp", []testFile{{"1", "1 2\n", "3\n"}, {"2", "2 2\n", "4\n"}})
	if err := os.Remove(p.Tests[1].Out); err != nil {
		t.Fatal(err)
	}
	_, err := judge.JudgeFull(context.Background(), p.Tests, "", b, quick, judge.Full, func(judge.Event) {})
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want the missing .out", err)
	}
}

// sortTests orders the TestDone events within a kinds() line, since a full
// pass emits them in completion order.
func sortTests(s string) string {
	f := strings.Fields(s)
	var tests []string
	var rest []string
	for _, w := range f {
		if strings.HasPrefix(w, "Test") {
			tests = append(tests, w)
		} else {
			rest = append(rest, w)
		}
	}
	slices.Sort(tests)
	if len(rest) < 2 {
		return strings.Join(append(rest, tests...), " ")
	}
	return strings.Join(slices.Concat(rest[:2], tests, rest[2:]), " ")
}

func TestSubmit(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	tests := []struct {
		name    string
		src     string
		files   []testFile
		want    string
		verdict judge.Verdict
		passed  int
	}{
		{"all pass", "ab.cpp", []testFile{{"1", "1 2\n", "3\n"}, {"2", "2 2\n", "4\n"}, {"3", "3 3\n", "6\n"}},
			"BuildStarted BuildDone(ok) Test0(AC) Test1(AC) Test2(AC) Finished", judge.AC, 3},
		{"one wrong", "ab.cpp", []testFile{{"1", "1 2\n", "3\n"}, {"2", "2 2\n", "5\n"}},
			"BuildStarted BuildDone(ok) Test0(AC) Test1(WA) Finished", judge.WA, 1},
		{"file io", "fileio.cpp", []testFile{{"1", "1 2\n", "3\n"}, {"2", "4 4\n", "8\n"}},
			"BuildStarted BuildDone(ok) Test0(AC) Test1(AC) Finished", judge.AC, 2},
		{"compile error", "ce.cpp", []testFile{{"1", "1 2\n", "3\n"}},
			"BuildStarted BuildDone(ce) Finished", judge.CE, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := load(t, tc.src, tc.files)
			var evs []judge.Event
			judge.Submit(context.Background(), p.Source, p.Tests, judge.Full, quick, func(e judge.Event) {
				evs = append(evs, e)
			})
			if got := sortTests(kinds(evs)); got != tc.want {
				t.Errorf("events:\n got %s\nwant %s", got, tc.want)
			}
			fin, ok := evs[len(evs)-1].(judge.Finished)
			if !ok || fin.Summary == nil {
				t.Fatalf("last event %#v, want Finished with a Summary", evs[len(evs)-1])
			}
			s := *fin.Summary
			if s.Verdict != tc.verdict || s.Passed != tc.passed || s.Total != len(tc.files) {
				t.Errorf("Summary = %+v, want %s %d/%d", s, tc.verdict, tc.passed, len(tc.files))
			}
		})
	}
}

// Not parallel: it lowers the package-level HardMemCap while the parallel
// tests are still parked. Runs with no memory limit of their own, and
// sanitizer builds, are held to the hard cap only.
func TestHardMemCap(t *testing.T) {
	needCompiler(t)
	old := judge.HardMemCap
	judge.HardMemCap = 192 << 20
	defer func() { judge.HardMemCap = old }()
	for _, tc := range []struct {
		name string
		p    judge.Profile
		mem  int64
	}{
		{"no limit", judge.Full, 0},
		{"sanitized", sample, 64 << 20},
	} {
		if tc.name == "sanitized" && !sample.Sanitized {
			t.Logf("%s: sample profile %s is not sanitized; skipped", tc.name, sample.Name)
			continue
		}
		b := build(t, "hog.cpp", tc.p)
		c := writeCase(t, "", "survived\n")
		r, err := judge.Run(context.Background(), b.Binary, c, judge.Limits{Time: 10 * time.Second, Mem: tc.mem}, tc.p)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		t.Logf("%s: %s %q rss=%s wall=%v", tc.name, r.Verdict, r.Reason, judge.Bytes(r.MaxRSS), r.Wall.Round(time.Millisecond))
		if r.Verdict != judge.MLE || !strings.HasSuffix(r.Reason, "> 192M cap") {
			t.Errorf("%s: %s %q, want MLE killed at the 192M cap\nstderr:\n%s", tc.name, r.Verdict, r.Reason, r.Stderr)
		}
	}
}

// TestParallelDrift measures how much CPU time a fixed amount of work reads
// when JudgeFull runs it Workers-wide instead of alone. It is a measurement,
// not a check: DEADCASES_DRIFT=1 go test -run ParallelDrift -v.
func TestParallelDrift(t *testing.T) {
	if os.Getenv("DEADCASES_DRIFT") == "" {
		t.Skip("set DEADCASES_DRIFT=1 to measure")
	}
	needCompiler(t)
	b := build(t, "work.cpp", judge.Full)
	n := 4 * judge.Workers
	dir := t.TempDir()
	var tests []problem.Test
	for i := range n {
		in, out := filepath.Join(dir, fmt.Sprint(i)+".in"), filepath.Join(dir, fmt.Sprint(i)+".out")
		write(t, in, "")
		write(t, out, "1\n")
		tests = append(tests, problem.Test{Name: fmt.Sprint(i), In: in, Out: out})
	}
	lim := judge.Limits{Time: 10 * time.Second}
	ctx := context.Background()
	median := func(ds []time.Duration) time.Duration {
		s := slices.Sorted(slices.Values(ds))
		return s[len(s)/2]
	}

	var seq []time.Duration
	start := time.Now()
	for _, tc := range tests {
		r, err := judge.Run(ctx, b.Binary, judge.Case{In: tc.In, Out: tc.Out}, lim, judge.Full)
		if err != nil || r.Verdict != judge.AC {
			t.Fatalf("sequential: %v %s %q", err, r.Verdict, r.Reason)
		}
		seq = append(seq, r.CPU)
	}
	seqWall := time.Since(start)

	var par []time.Duration
	start = time.Now()
	_, err := judge.JudgeFull(ctx, tests, "", b, lim, judge.Full, func(e judge.Event) {
		par = append(par, e.(judge.TestDone).Result.CPU)
	})
	if err != nil {
		t.Fatal(err)
	}
	parWall := time.Since(start)
	ms, mp := median(seq), median(par)
	t.Logf("%d runs, Workers=%d: sequential median cpu %v (wall %v), parallel median cpu %v (wall %v): cpu %+.1f%%, %.1fx faster",
		n, judge.Workers, ms.Round(time.Millisecond), seqWall.Round(time.Millisecond),
		mp.Round(time.Millisecond), parWall.Round(time.Millisecond),
		100*(float64(mp)/float64(ms)-1), float64(seqWall)/float64(parWall))
}
