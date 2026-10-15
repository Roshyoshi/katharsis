package game_test

import (
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
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

// TestBitsSteerNothing: the engine never reads the setuid bits or their
// record (GR7). The same fuzzed boots, on a Meta that holds every bit and
// a full record, play out byte for byte as on one that holds none.
func TestBitsSteerNothing(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 5)
	run := func(m *game.Meta) []string {
		f := &fuzzer{rng: rand.New(rand.NewPCG(7, 8)), now: t0}
		var out []string
		for range 40 {
			s, _ := fuzzBoot(t, rules.Map(), m, ps, f, false)
			out = append(out, mustJSON(t, s))
		}
		m.Setuid, m.Record = nil, game.Record{}
		return append(out, mustJSON(t, m))
	}
	bare := &game.Meta{Prestige: game.MaxPrestige}
	full := &game.Meta{Prestige: game.MaxPrestige}
	full.Record = game.Record{Folded: 1 << 20, Nodes: []string{"alu"}, Bosses: []string{"heisenbug"}, Verdicts: []game.Verdict{"WA"},
		Daemons: []game.Daemon{game.Fork}, Unlocked: []int{1}, Cleared: []int{0}, Endings: []string{game.EndingWake},
		Skips: 3, Nightly: 2, BootTime: 100 * time.Hour, Recovered: 4, RecoveredFree: 2, Backfilled: true}
	for _, id := range []string{"hello-world", "snicker-snack", ".two-miles-high", ".dull-reality", "hundred"} {
		full.Setuid = append(full.Setuid, game.SetuidBit{ID: id, At: t0, Boot: 1})
	}
	a, b := run(bare), run(full)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("boot %d played differently with bits set", i)
		}
	}
}
