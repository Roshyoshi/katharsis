package game_test

import (
	"errors"
	"testing"
	"time"

	"katharsis/internal/game"
)

// TestSamplesRun: a :w on the open door counts on its attempt, in a boot
// that counts them (State.CountsSamples), and is refused between doors; a
// submit that failed a sample is logged as such.
func TestSamplesRun(t *testing.T) {
	h := newHarness(t, plain(), nil, dense([]string{"dp", "graphs"}, 800, 2400, 25), 0, 1)
	if !h.s.CountsSamples {
		t.Fatal("a new boot does not count its sample runs")
	}
	if err := h.try(game.SamplesRun{}); !errors.Is(err, game.ErrInvalid) {
		t.Fatalf("SamplesRun between doors: %v", err)
	}
	h.open(0)
	h.do(game.SamplesRun{})
	h.do(game.SamplesRun{})
	h.do(game.SubmitResult{Verdict: "WA", Passed: 1, Total: 10, SampleFailed: true})
	h.now = h.now.Add(time.Minute)
	h.do(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
	a := h.s.History[0]
	if a.Samples != 2 {
		t.Errorf("Samples %d, want 2", a.Samples)
	}
	if len(a.Events) != 2 || !a.Events[0].SampleFailed || a.Events[1].SampleFailed {
		t.Errorf("events %+v: the first submit failed a sample", a.Events)
	}
}
