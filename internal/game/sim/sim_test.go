package sim_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// ideal follows the engine's own model exactly: it clears a door with the
// logistic chance of its rating and nothing else (no wrong submits, no
// ducks, near-constant solve times well inside any timer). It picks doors
// by bytes × P alone (Stake 0), which every plain door ties on when the
// aim is right, so the doors it takes are the doors on offer; weighing the
// stake, it would take the easiest.
func ideal(rating float64) sim.Player {
	p := sim.DefaultPlayer(rating)
	p.WAProb, p.StuckWAs, p.DuckBoost, p.BlindCost, p.Sigma = 0, 0, 0, 0, 0.01
	p.Stake = 0
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
// and cursed doors are the player's risk) must clear at its TargetClear
// (its doors' mean, where MinClear leaves only the window above the floor),
// and the calibration must stay near zero, since there is no shape to fix.
//
// It plays without packages: which daemons and upgrades a boot meets is
// beside the aim, and the default model's test plays with them. The
// rating is the mean over the last 200 boots: Elo at K 40 wanders by more
// than 100 points from boot to boot (seeds 1 to 8 end between 1516 and
// 1776 with packages, 1561 and 1739 without), and the mean does not. So
// are the calibration offsets: one tier's offset learns from a few doors a
// boot and ends anywhere in about ±100.
//
// The mid and boss tiers lean low. This player swaps with kexec exactly
// the doors it would fail; Elo does not rate a swap (close), so it sees
// only the doors kept and settles high, and the offsets, which count a
// swap as a failure (calibrate), take that back. Over seeds 1 to 8 the
// last 200 boots average Elo 1642 and offsets -2, -54 and -71 (Userland,
// mid, boss); rating the swaps as losses gives 1600 and +11, +8, -11.
// The plain doors clear on target either way, so the aim is right; only
// the rating reads high for a player who swaps.

func TestIdealPlayerHitsTargets(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	// On the legacy shape. On the map the plain doors clear on target too,
	// but kexec once an act lands this player's swap on act II's one room
	// after Boss I, and the swaps the offsets count as failures add up to
	// -110 to -173 there over seeds 1, 3 and 5; TestCalibrationHitsTargets
	// holds the map's aim.
	r := rules.Legacy()
	r.Packages = nil
	p := ideal(1600)
	p.Install = nil
	rep := sim.Run(sim.Config{
		Rules:  r,
		Pool:   sim.SyntheticPool(20000, 1),
		Player: p,
		Boots:  500, Warmup: 100, Seed: 3, Block: 50,
	})
	t.Logf("ideal 1600:\n%s", rep)
	for i, b := range r.Biomes {
		want := windowMean(game.Aim(0, b.TargetClear), b.MinClear)
		if got, _ := taught(rep, i); math.Abs(got-want) > 0.05 {
			t.Errorf("biome %d: plain doors clear %.3f, aimed at %.3f", i, got, want)
		}
		off := 0.0
		tail := rep.Blocks[len(rep.Blocks)-4:] // the last 200 boots
		for _, bl := range tail {
			off += (bl.Offsets[game.CalibrationAll] + bl.Offsets[game.CalibrationTier(b)]) / float64(len(tail))
		}
		if math.Abs(off) > 100 {
			t.Errorf("biome %d: calibrated by %+.0f for a logistic player over the last 200 boots", i, off)
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
// higher prestige stays harder by exactly what the offset says. A biome
// with a MinClear offers no door under it (D22), so its doors are the ±100
// window's above the floor, and the aim is their mean (windowMean). The
// aim is read off every plain door on offer, not the ones the model took:
// with a fourth door (fork or tmux) and shufs to spend, the model's pick
// leans to the easier doors by a few points, which is its choice, not the
// aim.
func TestCalibrationHitsTargets(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	r := rules.Standard()
	// The aim machinery, unheld: a tier's band (D63) holds a 1600 player's
	// Bronze aim down to 1300 and lifts its Gold one to 1700 on purpose, so
	// its doors miss the targets there by design (TestGuardrailBand plays
	// players who fit their tier).
	r.TierBands, r.TierWarmup = nil, 0
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
			want := windowMean(aim, b.MinClear)
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

// A preset with fewer biomes than boot #1's shape (rules.FirstBiomes)
// plays: boot #1's later biomes have report entries of their own.
func TestRunShorterPreset(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	r.Biomes, r.Nodes = []rules.Biome{r.Biomes[0], r.Biomes[2]}, nil // a linear shape, off the map
	rep := sim.Run(sim.Config{Rules: r, Pool: sim.SyntheticPool(3000, 1), Player: sim.DefaultPlayer(1300), Boots: 20, Seed: 1})
	crashes := 0
	for _, n := range rep.CrashBiome {
		crashes += n
	}
	if n := len(r.FirstBiomes); len(rep.Attempts) != n || len(rep.CrashBiome) != n {
		t.Errorf("%d attempt and %d crash entries, want boot #1's %d biomes", len(rep.Attempts), len(rep.CrashBiome), n)
	}
	if rep.Boots != 20 || rep.Errors != 0 || rep.FullClears+crashes != rep.Boots {
		t.Errorf("boots %d errors %d clears %d crashes %d", rep.Boots, rep.Errors, rep.FullClears, crashes)
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

// windowMean is the mean clear chance of doors spread evenly over the ±100
// rating window around aim (a player rated 0), leaving out those under
// floor, as the engine's first window does.
func windowMean(aim, floor float64) float64 {
	sum, n := 0.0, 0
	for x := aim - 100; x <= aim+100; x++ {
		if p := game.Expected(0, x); p >= floor {
			sum += p
			n++
		}
	}
	return sum / float64(n)
}

// TestGuardrailG1 holds guardrail G1 (ml-pipeline §1.2, slice ML7) at the
// trial-by-fire band's 10% (D49): the default model players from 900 to
// 1600 crash before the first merge window in at most 10% of their boots
// at prestige 0, on a pool with the Codeforces floor (synthetic, a tenth
// at 800), over boots 51-250 only: past them a player who never improves
// runs the floor dry (D46: the static model is judged there, the learning
// player on the long horizon, TestMapLearn). The union pool's numbers are
// in TestMapReport.
func TestGuardrailG1(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	pool := sim.SyntheticPool(20000, 1)
	for _, rating := range []float64{900, 1000, 1100, 1300, 1600} {
		t.Run(fmt.Sprint(rating), func(t *testing.T) {
			t.Parallel()
			crash, boots := 0, 0
			for _, seed := range []uint64{7, 8} {
				rep := sim.Run(sim.Config{Rules: rules.Standard(), Pool: pool, Player: sim.DefaultPlayer(rating),
					Boots: 200, Warmup: 50, Seed: seed})
				crash += rep.CrashBiome[0]
				boots += rep.Boots
			}
			g1 := float64(crash) / float64(boots)
			t.Logf("model %.0f: G1 %.1f%% over %d boots", rating, 100*g1, boots)
			if g1 > 0.1 {
				t.Errorf("model %.0f crashes in Userland in %.1f%% of boots, over the band's 10%%", rating, 100*g1)
			}
		})
	}
}
