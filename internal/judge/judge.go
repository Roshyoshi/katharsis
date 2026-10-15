package judge

import (
	"context"

	"katharsis/internal/problem"
)

// Event is streamed while judging so the HUD can fill in row by row.
type Event interface{ isEvent() }

type BuildDone struct{ Build Build }

type TestDone struct {
	Index  int
	Result Result
}

// Finished ends a judge pass. Err is set when the pass could not complete
// (missing compiler, unreadable file); verdicts are in the TestDone events.
// Summary is set by Submit and nil from Judge.
type Finished struct {
	Err     error
	Summary *Summary
}

func (BuildDone) isEvent() {}
func (TestDone) isEvent()  {}
func (Finished) isEvent()  {}

// Judge compiles p's source with prof and runs every test in order, sending
// events to emit. Tests run sequentially: samples are few, and parallel runs
// would skew the CPU times shown next to them. Cancelling ctx (a newer save)
// stops the pass without a Finished event.
func Judge(ctx context.Context, p *problem.Problem, prof Profile, lim Limits, emit func(Event)) {
	b, ok := build(ctx, p.Source, prof, emit)
	if !ok {
		return
	}
	if !b.OK {
		emit(Finished{})
		return
	}

	fileIO := problem.FileIO(b.Src)
	for i, t := range p.Tests {
		r, err := Run(ctx, b.Binary, Case{In: t.In, Out: t.Out, FileIO: fileIO}, lim, prof)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			emit(Finished{Err: err})
			return
		}
		emit(TestDone{Index: i, Result: r})
	}
	emit(Finished{})
}

// build is the compile step of Judge and Submit: it emits BuildDone, or
// Finished when the compiler could not run, or nothing once ctx is done. ok
// is false when the pass ends there.
func build(ctx context.Context, source string, prof Profile, emit func(Event)) (b Build, ok bool) {
	b, err := Compile(ctx, source, prof)
	if ctx.Err() != nil {
		return b, false
	}
	if err != nil {
		emit(Finished{Err: err})
		return b, false
	}
	emit(BuildDone{Build: b})
	return b, true
}
