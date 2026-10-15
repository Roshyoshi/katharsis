package judge

import (
	"context"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"katharsis/internal/problem"
)

// Workers is how many tests JudgeFull runs at once.
//
// What is judged is each process's own CPU time, so parallel runs do not
// share a budget, but they do slow each other down. Measured on an M4 (4
// performance + 6 efficiency cores), CPU time per run rose over sequential
// runs of the same binary by 11% at 4 workers, 14-15% at 5 and 19-25% at 8,
// for compute-bound and cache-missing programs alike (shared L2, and clocks
// drop as more cores wake). Half the cores keeps the drift near 15% for a
// 4x speedup; JudgeFull reruns alone the TLEs that the drift could explain.
var Workers = max(1, runtime.NumCPU()/2)

// retrySlack: a TLE in a parallel pass whose CPU time is under
// budget*retrySlack is rerun alone before it counts. 1.3 covers the drift
// measured at up to 8 workers.
const retrySlack = 1.3

// Summary is a full pass as the HP bar shows it: ▓▓▓▓░░ 37/52.
type Summary struct {
	Total  int // tests in the set
	Done   int // tests judged so far
	Passed int // of those, AC

	// First is the index of the first failing test in test order, -1 if
	// none has failed. Mid-pass a failure can still turn up before it.
	First int
	// Verdict is the verdict of tests[First]; AC once every test passed;
	// CE when the build failed; "" while the pass is clean but unfinished.
	Verdict Verdict
	Reason  string // tests[First]'s Reason

	MaxCPU time.Duration
	MaxRSS int64
}

// Summarize tallies results indexed like the tests. A result with no
// Verdict is a test that has not finished.
func Summarize(rs []Result) Summary {
	s := Summary{Total: len(rs), First: -1}
	for i, r := range rs {
		if r.Verdict == "" {
			continue
		}
		s.Done++
		s.MaxCPU, s.MaxRSS = max(s.MaxCPU, r.CPU), max(s.MaxRSS, r.MaxRSS)
		if r.Verdict == AC {
			s.Passed++
		} else if s.First < 0 {
			s.First, s.Verdict, s.Reason = i, r.Verdict, r.Reason
		}
	}
	if s.First < 0 && s.Done == s.Total {
		s.Verdict = AC
	}
	return s
}

// JudgeFull runs the built binary on every test, Workers at a time, and
// returns the tally. It emits one TestDone per test in completion order,
// always from the calling goroutine, so emit need not be safe for concurrent
// use.
//
// It keeps going after a failure, because the HP bar counts every test. The
// verdict is the first failure in test order (Summary.First), not the first
// to finish, so it does not depend on scheduling. For the same reason a TLE
// that the parallel slowdown could explain (see Workers) is held back and
// rerun alone after the rest, in test order; only the rerun is emitted.
//
// Cancelling ctx stops the pass: JudgeFull returns ctx.Err() and emits
// nothing more. An error running a test (an unreadable .out) stops it too.
func JudgeFull(ctx context.Context, tests []problem.Test, fileIO string, b Build, lim Limits, prof Profile, emit func(Event)) (Summary, error) {
	run := func(ctx context.Context, i int) (Result, error) {
		t := tests[i]
		return Run(ctx, b.Binary, Case{In: t.In, Out: t.Out, FileIO: fileIO}, lim, prof)
	}

	type done struct {
		i   int
		r   Result
		err error
	}
	// pass also ends when a run fails, so the other workers stop early.
	pass, stop := context.WithCancel(ctx)
	defer stop()
	out := make(chan done)
	var next atomic.Int64
	var wg sync.WaitGroup
	workers := min(Workers, len(tests))
	for range workers {
		wg.Go(func() {
			for pass.Err() == nil {
				i := int(next.Add(1) - 1)
				if i >= len(tests) {
					return
				}
				r, err := run(pass, i)
				out <- done{i, r, err}
			}
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()

	results := make([]Result, len(tests))
	budget := lim.budget(prof)
	var retry []int
	var failed error
	for d := range out {
		switch {
		case failed != nil || ctx.Err() != nil:
			// Drain: the workers stop once their runs see the cancel.
		case d.err != nil:
			failed = d.err
			stop()
		case workers > 1 && d.r.Verdict == TLE && float64(d.r.CPU) < float64(budget)*retrySlack:
			retry = append(retry, d.i)
		default:
			results[d.i] = d.r
			emit(TestDone{Index: d.i, Result: d.r})
		}
	}
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	if failed != nil {
		return Summarize(results), failed
	}
	slices.Sort(retry)
	for _, i := range retry {
		r, err := run(ctx, i)
		if ctx.Err() != nil {
			return Summary{}, ctx.Err()
		}
		if err != nil {
			return Summarize(results), err
		}
		r.Rerun = true
		results[i] = r
		emit(TestDone{Index: i, Result: r})
	}
	return Summarize(results), nil
}

// Submit compiles source with prof (Full, normally) and judges it on every
// test with JudgeFull. Events: BuildDone, a TestDone per test in completion
// order, then Finished carrying the Summary, whose Verdict is CE when the
// build failed. Cancelling ctx stops it without a Finished event, like Judge.
func Submit(ctx context.Context, source string, tests []problem.Test, prof Profile, lim Limits, emit func(Event)) {
	b, ok := build(ctx, source, prof, emit)
	if !ok {
		return
	}
	if !b.OK {
		emit(Finished{Summary: &Summary{Total: len(tests), First: -1, Verdict: CE}})
		return
	}
	s, err := JudgeFull(ctx, tests, problem.FileIO(b.Src), b, lim, prof, emit)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		emit(Finished{Err: err})
		return
	}
	emit(Finished{Summary: &s})
}
