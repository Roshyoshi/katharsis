package sim_test

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// trialSum sums the Trial's check over runs of one cell.
type trialSum struct {
	boots, full, reached, capped, open, taken, wakes int
	earned, spent, banked                            float64
	doors                                            time.Duration
}

func (x *trialSum) add(rep sim.Report) {
	x.boots += rep.Boots
	x.full += rep.FullClears
	x.capped += rep.Capped
	x.open += rep.TrialOpen
	x.taken += rep.TrialTaken
	x.wakes += rep.Wakes
	x.earned += rep.MeanEarned * float64(rep.Boots)
	x.spent += rep.MeanSpent * float64(rep.Boots)
	for _, b := range rep.Banked[len(rep.Banked)-rep.Boots:] {
		x.banked += float64(b)
	}
	for _, d := range rep.DoorTime {
		x.doors += d
	}
	reached := rep.Boots
	for tier, n := range rep.CrashBiome {
		if tier < rules.TierBossII {
			reached -= n
		}
	}
	x.reached += reached
}

// perHour is what the doors paid per hour inside them, less what merge
// windows spent of it (the seekers' fscks); banked is what reached the
// bank, commits and dumps upsolved back.
func (x trialSum) perHour() float64       { return (x.earned - x.spent) / max(x.doors.Hours(), 1e-9) }
func (x trialSum) bankedPerHour() float64 { return x.banked / max(x.doors.Hours(), 1e-9) }

func (x trialSum) line() string {
	return fmt.Sprintf("full %4.1f%% | at Boss II %4.1f%%, capped %4.1f%%, Trial open %4.1f%% of arrivals, taken %d, woke %d (%4.1f%% of takers) | %5.1f B a door-hour net of spending (gross %5.1f, banked %5.1f), %5.1f B a boot net",
		pct(x.full, x.boots), pct(x.reached, x.boots), pct(x.capped, x.boots), pct(x.open, x.reached), x.taken, x.wakes,
		pct(x.wakes, x.taken), x.perHour(), x.earned/max(x.doors.Hours(), 1e-9), x.bankedPerHour(),
		(x.earned-x.spent)/float64(max(x.boots, 1)))
}

func trialCell(r rules.Rules, pool []catalog.Problem, p sim.Player, prestige, boots int) trialSum {
	var x trialSum
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, seed := range []uint64{7, 8} {
		wg.Go(func() {
			rep := sim.Run(sim.Config{Rules: r, Pool: pool, Player: p, Boots: boots, Warmup: 50, Seed: seed, Prestige: prestige})
			mu.Lock()
			x.add(rep)
			mu.Unlock()
		})
	}
	wg.Wait()
	return x
}

// noTrial is the standard map without the Trial's node.
func noTrial() rules.Rules {
	r := rules.Standard()
	r.Nodes = slices.DeleteFunc(slices.Clone(r.Nodes), func(n rules.Biome) bool { return n.ID == rules.TrialNode })
	for i := range r.Nodes {
		r.Nodes[i].Next = slices.DeleteFunc(slices.Clone(r.Nodes[i].Next), func(id string) bool { return id == rules.TrialNode })
	}
	return r
}

// TestTrialCheck is achievements.md §(f)'s check before merge, on the
// synthetic pool, seeds 7 and 8. The model set out for the
// Trial hoards its bytes to the window before Boss II, fscks to the cap
// there when it can, and takes the Trial whenever it opens; against the
// default:
//
//   - non-takers play as without the Trial: the default player's full
//     clears and pay match the map without its node;
//   - bytes per door-hour, net of what merge windows spent (the seekers'
//     fscks), no higher for the seekers: the Trial is no farm. Its gross
//     pay a door-hour can run higher, as a cursed boss door's does;
//   - the Trial's clear rate on arrival, against the design's 35-45% band
//     (Boss I's 0.55, to offset the curse), reported, and held to 20-70%.
//
// By default P0, boots 51-170; KATHARSIS_TRIAL=1 adds P4 and plays boots
// 51-250, and KATHARSIS_TRIAL_AIM=0.5 plays the seekers on another aim.
func TestTrialCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	pool := sim.SyntheticPool(20000, 1)
	levels, boots := []int{0}, 120
	if os.Getenv("KATHARSIS_TRIAL") != "" {
		levels, boots = []int{0, 4}, 200
	}
	for _, prestige := range levels {
		for _, rating := range []float64{1000, 1300, 1600} {
			t.Run(fmt.Sprintf("P%d/%.0f", prestige, rating), func(t *testing.T) {
				t.Parallel()
				seeker := sim.DefaultPlayer(rating)
				seeker.Trial = true
				var def, bare, seek trialSum
				var wg sync.WaitGroup
				wg.Go(func() { def = trialCell(rules.Standard(), pool, sim.DefaultPlayer(rating), prestige, boots) })
				// One cell plays the map without the Trial too. A default boot
				// that never reaches the cap meets no Trial exit and no hint, so
				// its every draw is as without the node (the report's capped
				// share says how often that held).
				compare := prestige == 0 && rating == 1300
				if compare {
					wg.Go(func() { bare = trialCell(noTrial(), pool, sim.DefaultPlayer(rating), prestige, boots) })
				}
				aim, _ := strconv.ParseFloat(os.Getenv("KATHARSIS_TRIAL_AIM"), 64)
				wg.Go(func() { seek = trialCell(withTrialAim(aim), pool, seeker, prestige, boots) })
				wg.Wait()
				t.Logf("default  %s", def.line())
				if compare {
					t.Logf("no Trial %s", bare.line())
				}
				t.Logf("seeker   %s", seek.line())
				if def.taken != 0 || def.wakes != 0 {
					t.Errorf("the default model took the Trial %d times", def.taken)
				}
				if d := pct(def.full, def.boots) - pct(bare.full, bare.boots); compare && (d < -1 || d > 1 || def.perHour() > bare.perHour()*1.01 ||
					def.perHour() < bare.perHour()*0.99) {
					t.Errorf("non-takers moved: full %+.1f points, %.1f against %.1f B a door-hour", d, def.perHour(), bare.perHour())
				}
				if seek.perHour() > def.perHour() {
					t.Errorf("seekers earn %.1f B a door-hour against the default's %.1f: a farm", seek.perHour(), def.perHour())
				}
				if seek.taken < 30 {
					t.Fatalf("only %d boots took the Trial", seek.taken)
				}
				if rate := pct(seek.wakes, seek.taken); rate < 20 || rate > 70 {
					t.Errorf("the Trial cleared %.1f%% on arrival, want 35-45%% (held to 20-70)", rate)
				}
			})
		}
	}
}

// withTrialAim is the standard map with the Trial aimed at aim (0: as it
// is), for KATHARSIS_TRIAL_AIM.
func withTrialAim(aim float64) rules.Rules {
	r := rules.Standard()
	if aim <= 0 {
		return r
	}
	r.Nodes = slices.Clone(r.Nodes)
	for i := range r.Nodes {
		if r.Nodes[i].ID == rules.TrialNode {
			r.Nodes[i].TargetClear, r.Nodes[i].MinClear = aim, min(r.Nodes[i].MinClear, aim)
		}
	}
	return r
}
