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
// drop as more cores wake). Programs heavy on system calls fare far worse:
// one flushing a million lines (endl) read 5x its CPU time alone at 5
// workers, kernel time that grows with contention. Half the cores keeps
// compute drift near 15% for a 4x speedup, and JudgeFull reruns a TLE
// alone before it counts.
var Workers = max(1, runtime.NumCPU()/2)

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
// to finish, so it does not depend on scheduling. For the same reason no
// TLE from the parallel pass counts as is: the slowdown there can inflate
// CPU time several times over (see Workers). Every TLE is held back and,
// after the rest, rerun alone in test order until a rerun fails, which
// settles the verdict; only reruns are emitted for the tests rerun, and
// the TLEs after that one are emitted as the parallel pass saw them. A TLE
// after a failure that is not a TLE cannot change the verdict and is not
// rerun either.
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
	var held []int // parallel TLEs, kept in results until rerun
	var failed error
	for d := range out {
		switch {
		case failed != nil || ctx.Err() != nil:
			// Drain: the workers stop once their runs see the cancel.
		case d.err != nil:
			failed = d.err
			stop()
		case workers > 1 && d.r.Verdict == TLE:
			results[d.i] = d.r
			held = append(held, d.i)
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
	slices.Sort(held)
	other := len(tests) // the first failure in test order that is not held
	for i, r := range results {
		if r.Verdict != AC && !slices.Contains(held, i) {
			other = i
			break
		}
	}
	settled := false // a rerun failed
	for _, i := range held {
		if settled || other < i {
			emit(TestDone{Index: i, Result: results[i]})
			continue
		}
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
		settled = r.Verdict != AC
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
