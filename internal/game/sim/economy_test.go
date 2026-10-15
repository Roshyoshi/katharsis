package sim_test

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"katharsis/internal/catalog"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

var economyPool = sync.OnceValue(func() []catalog.Problem { return sim.SyntheticPool(20000, 1) })

// outcome is what a spend policy earns a model player over many boots:
// how often a boot clears, and what reaches the bank per hour of uptime
// (commits and upsolved core dumps).
type outcome struct {
	full, bankH float64
	boots       int
}

func (o outcome) String() string {
	return fmt.Sprintf("full %4.1f%% · bank %5.1f B/h", 100*o.full, o.bankH)
}

// se is the standard error of the full-clear rate.
func (o outcome) se() float64 { return math.Sqrt(o.full * (1 - o.full) / float64(max(o.boots, 1))) }

// play runs p at prestige 0 over seeds and sums what comes after the
// warm-up.
func play(p sim.Player, seeds []uint64, boots, warm int) outcome {
	var full, n int
	var bank, hours float64
	for _, seed := range seeds {
		rep := sim.Run(sim.Config{
			Rules: rules.Standard(), Pool: economyPool(), Player: p,
			Boots: boots, Warmup: warm, Seed: seed,
		})
		full += rep.FullClears
		n += rep.Boots
		for _, b := range rep.Banked[warm:] {
			bank += float64(b)
		}
		hours += float64(rep.Boots) * rep.MeanWall.Hours()
	}
	return outcome{full: float64(full) / float64(n), bankH: bank / hours, boots: n}
}

// TestSpendFork is retention RT2: at a merge window, spending uncommitted
// bytes (lives, ducks, re-rolls for this boot) or committing them (the
// bank, for packages) must be a real choice. A fork one fixed policy wins
// is a chore with more keys, so at both reference ratings
//
//   - no fixed policy (commit everything, or spend a fixed share each
//     window: a quarter, half, all) beats the model's adaptive one (buy
//     what this boot lacks, commit the rest) on both the full-clear rate
//     and the bank per hour, by more than one standard error of the rate;
//   - committing everything costs clears that some spending buys, and
//     spending everything costs bank that committing keeps.
//
// The players install no packages, so the shelf is the base one. The
// test takes about three CPU-minutes (with fewer boots the 1600-rated
// player's policies fall within noise of each other), so it runs with the
// balance report: KATHARSIS_BALANCE=1.
func TestSpendFork(t *testing.T) {
	if os.Getenv("KATHARSIS_BALANCE") == "" {
		t.Skip("set KATHARSIS_BALANCE=1 for the spend fork")
	}
	t.Parallel()
	policies := []struct {
		name  string
		spend bool
		share float64
	}{
		{"commit all", false, 0},
		{"adaptive", true, 0},
		{"spend 25%", false, 0.25},
		{"spend 50%", false, 0.5},
		{"spend all", false, 1},
	}
	for _, rating := range []float64{1000, 1600} {
		t.Run(fmt.Sprint(rating), func(t *testing.T) {
			t.Parallel()
			got := make([]outcome, len(policies))
			var wg sync.WaitGroup
			for i, pol := range policies {
				wg.Go(func() {
					p := sim.DefaultPlayer(rating)
					p.Spend, p.SpendShare, p.Install = pol.spend, pol.share, nil
					got[i] = play(p, []uint64{1, 2}, 300, 50)
				})
			}
			wg.Wait()
			var table strings.Builder
			for i, pol := range policies {
				fmt.Fprintf(&table, "\n  %-10s %s", pol.name, got[i])
			}
			t.Logf("model %.0f, prestige 0, %d boots each:%s", rating, got[0].boots, table.String())

			adaptive := got[1]
			bestSpend := 0.0
			for i, o := range got {
				if i >= 1 {
					bestSpend = max(bestSpend, o.full)
				}
				if i != 1 && o.full > adaptive.full+o.se() && o.bankH > adaptive.bankH {
					t.Errorf("%s beats adaptive on both: %s vs %s", policies[i].name, o, adaptive)
				}
			}
			if commit := got[0]; commit.full >= bestSpend {
				t.Errorf("committing everything clears as often as any spending (%s): spending buys nothing", commit)
			}
			if all, commit := got[len(got)-1], got[0]; all.bankH >= commit.bankH {
				t.Errorf("spending everything banks as much as committing (%s vs %s)", all, commit)
			}
		})
	}
}

// TestAptLadder is retention RT3 against the model players: from a fresh
// save at prestige 0, buying the standard packages cheapest first at GRUB
// between boots (the default model does), with the rate a player banks
// at measured over its first 40 boots,
//
//   - no package before the 40th boot of the 1000-rated player's bank
//     costs more than three boots of it (the goal gradient);
//   - the catalog costs at least 40 boots of either player's bank, and
//     the median run has not bought it all by boot 40;
//   - the 1600-rated player can buy a package after its first boot from
//     at least 80% of fresh saves.
//
// The 1000-rated player is held to the last only as far as its first
// boot banks anything: most of its first boots crash in Userland before
// the first merge window (the pool has little near its Userland target),
// and a crash banks only what an upsolve recovers. Of those that bank
// anything, 80% must afford the first package.
func TestAptLadder(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	t.Parallel()
	r := rules.Standard()
	ladder := r.Ladder()
	var total int64
	for _, p := range ladder {
		total += p.Price
	}
	const boots, horizon = 45, 40
	seeds := []uint64{1, 2, 3, 4, 5, 6, 7, 8}
	for _, rating := range []float64{1000, 1600} {
		t.Run(fmt.Sprint(rating), func(t *testing.T) {
			t.Parallel()
			reps := make([]sim.Report, len(seeds))
			var wg sync.WaitGroup
			for i, seed := range seeds {
				wg.Go(func() {
					reps[i] = sim.Run(sim.Config{
						Rules: r, Pool: economyPool(), Player: sim.DefaultPlayer(rating),
						Boots: boots, Seed: seed,
					})
				})
			}
			wg.Wait()
			var banked int64
			var lasts []int
			for i, rep := range reps {
				for _, b := range rep.Banked[:horizon] {
					banked += b
				}
				last := rep.InstalledAt[len(rep.InstalledAt)-1]
				if last == 0 {
					last = boots + 1 // not bought within the run
				}
				lasts = append(lasts, last)
				t.Logf("seed %d: installed after boots %v", seeds[i], rep.InstalledAt)
			}
			rate := float64(banked) / float64(horizon*len(reps))
			// What the first boot banks varies far more than the run's rate,
			// so it gets a sample of its own: one boot from each of many
			// fresh saves.
			first, firstAny := 0, 0
			const firsts = 60
			b1 := make([]int64, firsts)
			for i := range firsts {
				wg.Go(func() {
					rep := sim.Run(sim.Config{
						Rules: r, Pool: economyPool(), Player: sim.DefaultPlayer(rating),
						Boots: 1, Seed: uint64(1000 + i),
					})
					b1[i] = rep.Banked[0]
				})
			}
			wg.Wait()
			for _, b := range b1 {
				if b >= ladder[0].Price {
					first++
				}
				if b > 0 {
					firstAny++
				}
			}
			slices.Sort(lasts)
			t.Logf("model %.0f: banks %.0f B a boot over boots 1-%d; catalog %d B is %.1f boots; "+
				"a package after boot 1 in %d of %d saves (%d banked anything); catalog bought by boot %v",
				rating, rate, horizon, total, float64(total)/rate, first, firsts, firstAny, lasts)

			if rating == 1000 {
				var sum int64
				for _, p := range ladder {
					if float64(sum) < horizon*rate && float64(p.Price) > 3*rate {
						t.Errorf("%s costs %d B, more than three boots (%.0f B) %.1f boots into the ladder",
							p.Name, p.Price, 3*rate, float64(sum)/rate)
					}
					sum += p.Price
				}
			}
			if float64(total) < horizon*rate {
				t.Errorf("the catalog is %.1f boots of bank, want at least %d", float64(total)/rate, horizon)
			}
			if median := lasts[len(lasts)/2]; median < horizon {
				t.Errorf("the median run bought every package by boot %d, want after %d", median, horizon)
			}
			switch {
			case rating == 1600 && 5*first < 4*firsts:
				t.Errorf("a package after boot 1 in %d of %d saves, want 80%%", first, firsts)
			case first < firstAny*4/5:
				t.Errorf("a package after boot 1 in %d of the %d runs that banked: the first is priced too high", first, firstAny)
			}
		})
	}
}
