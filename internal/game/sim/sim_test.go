package sim_test

import (
	"math"
	"testing"
	"time"

	"deadcases/internal/game"
	"deadcases/internal/game/sim"
	"deadcases/internal/rules"
)

// ideal follows the engine's own model exactly: it clears a door with the
// logistic chance of its rating and nothing else (no wrong submits, no
// ducks, near-constant solve times well inside any timer).
func ideal(rating float64) sim.Player {
	p := sim.DefaultPlayer(rating)
	p.WAProb, p.StuckWAs, p.DuckBoost, p.BlindCost, p.Sigma = 0, 0, 0, 0, 0.01
	p.Median = 10 * time.Minute
	return p
}

// TestIdealPlayerHitsTargets is the engine's end-to-end calibration check:
// for a player who behaves like the logistic model, Elo must find the true
// rating and door selection must land each biome on its TargetClear.
func TestIdealPlayerHitsTargets(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	r := rules.Standard()
	rep := sim.Run(sim.Config{
		Rules:  r,
		Pool:   sim.SyntheticPool(6000, 1),
		Player: ideal(1600),
		Boots:  600, Warmup: 100, Seed: 3,
	})
	t.Logf("ideal 1600:\n%s", rep)
	for i, b := range r.Biomes {
		got := float64(rep.Clears[i]) / float64(rep.Attempts[i])
		if math.Abs(got-b.TargetClear) > 0.06 {
			t.Errorf("biome %d: door clear rate %.3f, target %.2f", i, got, b.TargetClear)
		}
	}
	if math.Abs(rep.FinalRating-1600) > 120 {
		t.Errorf("engine rating %.0f, true 1600", rep.FinalRating)
	}
}

// TestStandardBalance logs rules.Standard() for the two reference players
// (go test -v shows the reports). It asserts accounting only: balance is a
// judgment call made from these numbers, not a test outcome.
func TestStandardBalance(t *testing.T) {
	t.Parallel()
	pool := sim.SyntheticPool(600, 1)
	for _, rating := range []float64{1000, 1600} {
		for _, prestige := range []int{0, -1} {
			rep := sim.Run(sim.Config{
				Rules:    rules.Standard(),
				Pool:     pool,
				Player:   sim.DefaultPlayer(rating),
				Boots:    300,
				Warmup:   20,
				Seed:     7,
				Prestige: prestige,
			})
			t.Logf("player %.0f, prestige %d:\n%s", rating, prestige, rep)
			if rep.Errors != 0 {
				t.Errorf("%d engine errors", rep.Errors)
			}
			crashes := 0
			for _, n := range rep.CrashBiome {
				crashes += n
			}
			if rep.FullClears+crashes != rep.Boots {
				t.Errorf("clears %d + crashes %d != boots %d", rep.FullClears, crashes, rep.Boots)
			}
			if rep.ClearRate <= 0 || rep.ClearRate >= 1 {
				t.Errorf("full-clear rate %.3f", rep.ClearRate)
			}
		}
	}
}

func TestSimulateDeterministic(t *testing.T) {
	t.Parallel()
	pool := sim.SyntheticPool(600, 2)
	a := sim.Simulate(rules.Standard(), pool, 1400, 200, 99)
	b := sim.Simulate(rules.Standard(), pool, 1400, 200, 99)
	if a.String() != b.String() {
		t.Fatalf("same seed, different reports:\n%s\n%s", a, b)
	}
	if a.Boots != 200 || a.Errors != 0 {
		t.Errorf("boots %d errors %d", a.Boots, a.Errors)
	}
}

func TestSyntheticPool(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 1)
	topics := map[string]bool{}
	judgeable := 0
	for _, p := range ps {
		if p.Rating < 800 || p.Rating > 2600 {
			t.Errorf("%s rating %d outside 800..2600", p.ID, p.Rating)
		}
		topics[p.Topic] = true
		if p.Judgeable {
			judgeable++
		}
	}
	if len(topics) != 16 {
		t.Errorf("%d topics, want every catalog topic", len(topics))
	}
	if judgeable != 570 {
		t.Errorf("%d judgeable, want 570", judgeable)
	}
	if len(sim.Simulate(rules.Standard(), ps, 1200, 1, 1).Meta.Capabilities) != len(game.TopicCapabilities) {
		t.Error("the model player should hold every capability")
	}
}
