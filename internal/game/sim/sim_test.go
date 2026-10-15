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
//
// It plays without packages: which daemons and upgrades a boot meets is
// beside the aim, and the default model's test plays with them. The
// rating is the mean over the last 200 boots: Elo at K 40 wanders by more
// than 100 points from boot to boot (seeds 1 to 8 end between 1516 and
// 1776 with packages, 1561 and 1739 without), and the mean does not.
func TestIdealPlayerHitsTargets(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	r := rules.Standard()
	r.Packages = nil
	p := ideal(1600)
	p.Install = nil
	rep := sim.Run(sim.Config{
		Rules:  r,
		Pool:   sim.SyntheticPool(20000, 1),
		Player: p,
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
	tail := rep.Ratings[len(rep.Ratings)-200:]
	mean := 0.0
	for _, x := range tail {
		mean += x / float64(len(tail))
	}
	if math.Abs(mean-1600) > 120 {
		t.Errorf("engine rating %.0f over the last 200 boots, true 1600", mean)
	}
}

// TestCalibrationHitsTargets: the default model's solve curve is flatter
// than the logistic and has ceilings (timers, wrong submits, ducks), which
// is what the calibration is for. Its plain doors must clear at what the
// engine expects of them in every tier, within binomial noise (3 standard
// errors, plus 2 points of calibration lag), and the aim must be
// TargetClear at prestige 0 and, at prestige 2, the logistic chance
// 2·PrestigeOffset above it (or the biome's MinClear, where that binds): a
// higher prestige stays harder by exactly what the offset says. The aim is
// read off every plain door on offer, not the ones the model took: with
// fork's fourth door and shufs to spend, the model's pick leans to the
// easier doors by a few points, which is its choice, not the aim.
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
			aim := game.Aim(0, b.TargetClear) + float64(prestige)*r.PrestigeOffset
			if b.MinClear > 0 {
				aim = min(aim, game.Aim(0, b.MinClear))
			}
			want := game.Expected(0, aim)
			tol := 3*math.Sqrt(exp*(1-exp)/float64(max(rep.Taught[i], 1))) + 0.02
			aimed := rep.OfferedP[i] / float64(max(rep.Offered[i], 1))
			if math.Abs(got-exp) > tol || math.Abs(aimed-want) > 0.03 {
				t.Errorf("prestige %d biome %d: plain doors clear %.3f (±%.3f), engine expected %.3f, aimed %.3f, want near %.3f",
					prestige, i, got, tol, exp, aimed, want)
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
