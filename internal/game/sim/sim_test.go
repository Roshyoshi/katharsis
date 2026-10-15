package sim_test

import (
	"math"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
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

// taught is the calibration's observed and expected clear rate of the
// plain, aimed doors of biome i.
func taught(rep sim.Report, i int) (observed, expected float64) {
	n := float64(max(rep.Taught[i], 1))
	return float64(rep.TaughtClears[i]) / n, rep.TaughtP[i] / n
}

// TestIdealPlayerHitsTargets is the engine's end-to-end aiming check: for
// a player who behaves like the logistic model, Elo must find the true
// rating, the plain doors of each biome (the ones the aim controls; blind
// and cursed doors are the player's risk) must clear at its TargetClear,
// and the calibration must stay near zero, since there is no shape to fix.
func TestIdealPlayerHitsTargets(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	r := rules.Standard()
	rep := sim.Run(sim.Config{
		Rules:  r,
		Pool:   sim.SyntheticPool(20000, 1),
		Player: ideal(1600),
		Boots:  500, Warmup: 100, Seed: 3,
	})
	t.Logf("ideal 1600:\n%s", rep)
	for i, b := range r.Biomes {
		if got, _ := taught(rep, i); math.Abs(got-b.TargetClear) > 0.05 {
			t.Errorf("biome %d: plain doors clear %.3f, target %.2f", i, got, b.TargetClear)
		}
		cal := rep.Meta.Calibration
		if off := cal[game.CalibrationAll].Offset + cal[game.CalibrationTier(b)].Offset; math.Abs(off) > 100 {
			t.Errorf("biome %d: calibrated by %+.0f for a logistic player", i, off)
		}
	}
	if math.Abs(rep.FinalRating-1600) > 120 {
		t.Errorf("engine rating %.0f, true 1600", rep.FinalRating)
	}
}

// TestCalibrationHitsTargets: the default model's solve curve is flatter
// than the logistic and has ceilings (timers, wrong submits, ducks), which
// is what the calibration is for. Its plain doors must clear at what the
// engine expects of them in every tier, within binomial noise (3 standard
// errors, plus 2 points of calibration lag), and the expectation must be
// TargetClear at prestige 0 and, at prestige 2, the logistic chance
// 2·PrestigeOffset above the aim: a higher prestige stays harder by
// exactly what the offset says.
func TestCalibrationHitsTargets(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	r := rules.Standard()
	pool := sim.SyntheticPool(20000, 1)
	for _, prestige := range []int{0, 2} {
		rep := sim.Run(sim.Config{
			Rules:    r,
			Pool:     pool,
			Player:   sim.DefaultPlayer(1600),
			Boots:    500,
			Warmup:   100,
			Seed:     5,
			Prestige: prestige,
		})
		t.Logf("model 1600, prestige %d:\n%s", prestige, rep)
		for i, b := range r.Biomes {
			got, exp := taught(rep, i)
			want := game.Expected(0, game.Aim(0, b.TargetClear)+float64(prestige)*r.PrestigeOffset)
			tol := 3*math.Sqrt(exp*(1-exp)/float64(max(rep.Taught[i], 1))) + 0.02
			if math.Abs(got-exp) > tol || math.Abs(exp-want) > 0.03 {
				t.Errorf("prestige %d biome %d: plain doors clear %.3f (±%.3f), engine expected %.3f, want both near %.3f",
					prestige, i, got, tol, exp, want)
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
	crashes := 0
	for _, n := range a.CrashBiome {
		crashes += n
	}
	if a.Boots != 200 || a.Errors != 0 || a.FullClears+crashes != a.Boots || a.FullClears == 0 {
		t.Errorf("boots %d errors %d clears %d crashes %d", a.Boots, a.Errors, a.FullClears, crashes)
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
