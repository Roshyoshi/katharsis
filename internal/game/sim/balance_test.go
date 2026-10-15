package sim_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// cachedPool is the judgeable problems of a content source's cached index
// (source "usaco" or "codeforces"), read straight from the cache file (no
// network, no guide snapshot), or nil when there is no cache.
// KATHARSIS_<SOURCE>_INDEX overrides the path.
func cachedPool(t *testing.T, source string) []catalog.Problem {
	t.Helper()
	path := os.Getenv("KATHARSIS_" + strings.ToUpper(source) + "_INDEX")
	if path == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return nil
		}
		path = filepath.Join(dir, "katharsis", source, "index.json")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var idx struct{ Entries []catalog.Problem }
	if err := json.Unmarshal(b, &idx); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var ps []catalog.Problem
	for _, p := range idx.Entries {
		if p.Judgeable {
			ps = append(ps, p)
		}
	}
	return ps
}

// coldBoots is how many boots the engine's rating took to come within 100
// of where it settled (the mean of the last half of the run).
func coldBoots(rs []float64) int {
	if len(rs) == 0 {
		return 0
	}
	tail := rs[len(rs)/2:]
	sum := 0.0
	for _, r := range tail {
		sum += r
	}
	settled := sum / float64(len(tail))
	for i, r := range rs {
		if math.Abs(r-settled) < 100 {
			return i + 1
		}
	}
	return len(rs)
}

// blocks renders a run's time series, one line per block.
func blocks(rep sim.Report) string {
	var w strings.Builder
	at := 0
	for _, b := range rep.Blocks {
		var offs []string
		for _, k := range slices.Sorted(maps.Keys(b.Offsets)) {
			offs = append(offs, fmt.Sprintf("%s %+.0f", k, b.Offsets[k]))
		}
		fmt.Fprintf(&w, "  boots %3d-%3d: full clears %4.1f%%, off target %3.0f%%, rating %.0f, offsets [%s]\n",
			at+1, at+b.Boots, 100*float64(b.FullClears)/float64(b.Boots),
			100*float64(b.OffTarget)/float64(max(b.Opened, 1)), b.Rating, strings.Join(offs, ", "))
		at += b.Boots
	}
	return w.String()
}

// TestBalanceReport prints the numbers balance decisions are made from, on
// a synthetic pool large enough never to run dry and on the cached USACO
// pool: full clears, crashes and door clear rates per biome for the two
// reference players at prestige 0 and climbing, with the run's time series
// (cold start, calibration, pool exhaustion), then a table of what each
// prestige level does to a boot. It is a report, not a check; set
// KATHARSIS_BALANCE=1 and run with -v.
func TestBalanceReport(t *testing.T) {
	if os.Getenv("KATHARSIS_BALANCE") == "" {
		t.Skip("set KATHARSIS_BALANCE=1 for the balance report")
	}
	type pool struct {
		name string
		ps   []catalog.Problem
	}
	pools := []pool{{"synthetic 20000", sim.SyntheticPool(20000, 1)}}
	usaco, cf := cachedPool(t, "usaco"), cachedPool(t, "codeforces")
	if usaco != nil {
		pools = append(pools, pool{fmt.Sprintf("usaco %d", len(usaco)), usaco})
	}
	if usaco != nil && cf != nil {
		pools = append(pools, pool{fmt.Sprintf("usaco+codeforces %d", len(usaco)+len(cf)), slices.Concat(usaco, cf)})
	}
	if only := os.Getenv("KATHARSIS_BALANCE_POOL"); only != "" {
		pools = slices.DeleteFunc(pools, func(p pool) bool { return !strings.HasPrefix(p.name, only) })
	}
	// KATHARSIS_BALANCE_PRESET plays a preset file instead of the standard
	// rules, to try numbers without editing them in.
	r := rules.Standard()
	if path := os.Getenv("KATHARSIS_BALANCE_PRESET"); path != "" {
		var err error
		if r, err = rules.Load(path); err != nil {
			t.Fatal(err)
		}
	}
	// KATHARSIS_BALANCE_BOOTS lengthens the run, to see how long a pool
	// lasts; blocks grow with it.
	boots := 400
	if n, err := strconv.Atoi(os.Getenv("KATHARSIS_BALANCE_BOOTS")); err == nil && n > 0 {
		boots = n
	}
	t.Run("runs", func(t *testing.T) {
		for _, pool := range pools {
			for _, rating := range []float64{1000, 1600} {
				for _, prestige := range []int{0, -1} {
					rep := sim.Run(sim.Config{
						Rules:    r,
						Pool:     pool.ps,
						Player:   sim.DefaultPlayer(rating),
						Boots:    boots,
						Warmup:   50,
						Seed:     7,
						Prestige: prestige,
						Block:    max(50, (boots+50)/9),
					})
					t.Logf("%s, player %.0f, prestige %d (boots 51-%d; rating settled after %d boots):\n%s%s",
						pool.name, rating, prestige, boots+50, coldBoots(rep.Ratings), rep, blocks(rep))
				}
			}
		}
	})
	t.Run("guardrails", func(t *testing.T) {
		for _, pool := range pools {
			t.Logf("%s, P0, boots 51-%d of seeds 7 and 8:\n%s", pool.name, boots+50, guardTable(r, pool.ps, boots))
		}
	})
	t.Run("prestige", func(t *testing.T) {
		for _, pool := range pools {
			for _, rating := range []float64{1000, 1600} {
				t.Logf("%s, player %.0f, each prestige played for boots 51-%d:\n%s",
					pool.name, rating, 2*boots+50, prestigeTable(r, pool.ps, rating, 2*boots))
			}
		}
	})
}

// guardTable is guardrail G1 (P(crash before the first merge window) <=
// 0.2, ml-pipeline §1.2) for the model players from 900 to 1600 at
// prestige 0: with the rules' easing, with the easing off (no forecast
// easing, buy-back or governor), and with the easing but the myopic
// bytes × P chooser (Player.Stake 0). Each cell is G1 and the full-clear
// rate over two seeds; the eased run also shows the engine's mean
// forecast and the share of boots the floor easing took.
func guardTable(r rules.Rules, pool []catalog.Problem, boots int) string {
	off := r
	off.EaseAbove, off.BuybackTimers, off.GovernorAfter = 0, nil, 0
	ratings := []float64{900, 1000, 1100, 1300, 1600}
	type cell struct{ g1, full, forecast, floor float64 }
	configs := []struct {
		name  string
		rules rules.Rules
		stake float64
	}{{"eased", r, 1}, {"easing off", off, 1}, {"eased, myopic", r, 0}}
	cells := make([][]cell, len(configs))
	var wg sync.WaitGroup
	for ci, c := range configs {
		cells[ci] = make([]cell, len(ratings))
		for ri, rating := range ratings {
			wg.Go(func() {
				var n, crash, full, floor int
				var fc float64
				for _, seed := range []uint64{7, 8} {
					p := sim.DefaultPlayer(rating)
					p.Stake = c.stake
					rep := sim.Run(sim.Config{Rules: c.rules, Pool: pool, Player: p, Boots: boots, Warmup: 50, Seed: seed})
					n += rep.Boots
					crash += rep.CrashBiome[0]
					full += rep.FullClears
					floor += rep.Eased[game.EaseFloor]
					fc += rep.Forecast
				}
				cells[ci][ri] = cell{float64(crash) / float64(n), float64(full) / float64(n), fc / float64(n), float64(floor) / float64(n)}
			})
		}
	}
	wg.Wait()
	var w strings.Builder
	fmt.Fprintf(&w, "  %-14s", "G1 / full")
	for _, rt := range ratings {
		fmt.Fprintf(&w, " %13.0f", rt)
	}
	for ci, c := range configs {
		fmt.Fprintf(&w, "\n  %-14s", c.name)
		for _, x := range cells[ci] {
			fmt.Fprintf(&w, "   %4.1f / %4.1f", 100*x.g1, 100*x.full)
		}
	}
	fmt.Fprintf(&w, "\n  %-14s", "forecast")
	for _, x := range cells[0] {
		fmt.Fprintf(&w, " %12.1f%%", 100*x.forecast)
	}
	fmt.Fprintf(&w, "\n  %-14s", "floor-eased")
	for _, x := range cells[0] {
		fmt.Fprintf(&w, " %12.0f%%", 100*x.floor)
	}
	return w.String() + "\n"
}

// prestigeTable plays each prestige level from a fresh player and
// tabulates what the level does to a boot: full clears, where boots crash,
// every door's clear rate per biome, the mid biome's plain doors (the ones
// the aim controls) cleared against what the engine expected, the clear
// chance it aimed every plain mid door on offer at (D22 holds it at 0.6),
// and the aim correction the calibration ended on for the mid biome, which
// shows how much of a level's difficulty it learned away.
func prestigeTable(r rules.Rules, pool []catalog.Problem, rating float64, boots int) string {
	var w strings.Builder
	fmt.Fprintf(&w, "  p  full   crash b0/b1/b2  doors b0/b1/b2  mid plain obs/exp (n)  mid offered  mid calib\n")
	for p := range game.MaxPrestige + 1 {
		rep := sim.Run(sim.Config{
			Rules:    r,
			Pool:     pool,
			Player:   sim.DefaultPlayer(rating),
			Boots:    boots,
			Warmup:   50,
			Seed:     7,
			Prestige: p,
		})
		pct := func(a, b int) float64 { return 100 * float64(a) / float64(max(b, 1)) }
		var crash, doors []string
		for i := range rep.Attempts {
			crash = append(crash, fmt.Sprintf("%2.0f", pct(rep.CrashBiome[i], rep.Boots)))
			doors = append(doors, fmt.Sprintf("%2.0f", pct(rep.Clears[i], rep.Attempts[i])))
		}
		mid := len(rep.Taught) / 2
		exp := rep.TaughtP[mid] / float64(max(rep.Taught[mid], 1))
		cal := rep.Meta.Calibration
		off := cal[game.CalibrationAll].Offset + cal[game.CalibrationTier(r.Biomes[mid])].Offset
		offered := rep.OfferedP[mid] / float64(max(rep.Offered[mid], 1))
		fmt.Fprintf(&w, "  %d  %4.1f%%  %-14s  %-14s  %4.1f%% / %4.1f%% (%4d)  %4.1f%% (%4d)  %+4.0f\n", p, 100*rep.ClearRate,
			strings.Join(crash, "/"), strings.Join(doors, "/"),
			pct(rep.TaughtClears[mid], rep.Taught[mid]), 100*exp, rep.Taught[mid], 100*offered, rep.Offered[mid], off)
	}
	return w.String()
}
