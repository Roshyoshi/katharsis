package sim_test

// The map's balance (docs/design/map.md §(i)): the default against the
// trial-by-fire band, each lever on its own, the boss twists, each prestige
// level, and a learning player's year. A report, not a check: set
// KATHARSIS_MAP=1 and run with -v. TestGuardrailG1 and TestGuardrailBand
// are the checks.

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// acts are the last tier of each of the map's acts.
var acts = []int{rules.TierBossI, rules.TierBossII, rules.TierFinal}

// mapPool is map.md's union pool (USACO + Codeforces from the cache), or
// the synthetic one where the cache has none.
func mapPool(t *testing.T) (string, []catalog.Problem) {
	if os.Getenv("KATHARSIS_MAP_POOL") != "synthetic" {
		usaco, cf := cachedPool(t, "usaco"), cachedPool(t, "codeforces")
		if usaco != nil && cf != nil {
			return fmt.Sprintf("usaco+codeforces %d", len(usaco)+len(cf)), slices.Concat(usaco, cf)
		}
	}
	return "synthetic 20000", sim.SyntheticPool(20000, 1)
}

// mapSum sums what the map report shows over runs of one cell.
type mapSum struct {
	boots, full, g1, fsck, fountain, saves int
	act                                    [3]int // crashes per act
	doorTime, clearTime                    []time.Duration
	bosses                                 map[string]sim.BossTally
}

func (m *mapSum) add(rep sim.Report) {
	m.boots += rep.Boots
	m.full += rep.FullClears
	m.g1 += rep.CrashBiome[0]
	m.fsck += rep.Fscks
	m.fountain += rep.Fountained
	m.saves += rep.BossSaves
	for tier, n := range rep.CrashBiome {
		for a, last := range acts {
			if tier <= last {
				m.act[a] += n
				break
			}
		}
	}
	m.doorTime = append(m.doorTime, rep.DoorTime...)
	m.clearTime = append(m.clearTime, rep.DoorTimeClear...)
	if m.bosses == nil {
		m.bosses = map[string]sim.BossTally{}
	}
	for k, b := range rep.Bosses {
		t := m.bosses[k]
		t.Reached += b.Reached
		t.Crashed += b.Crashed
		t.Retries += b.Retries
		m.bosses[k] = t
	}
}

func pct(a, b int) float64 { return 100 * float64(a) / float64(max(b, 1)) }

// hazard is the crash hazard of each act: crashes there over the boots
// that reached it.
func (m mapSum) hazard() [3]float64 {
	reached := m.boots
	var h [3]float64
	for a, n := range m.act {
		h[a] = pct(n, reached)
		reached -= n
	}
	return h
}

func meanDur(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var s time.Duration
	for _, d := range ds {
		s += d
	}
	return s / time.Duration(len(ds))
}

func quant(ds []time.Duration, q float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	ds = slices.Clone(ds)
	slices.Sort(ds)
	return ds[min(len(ds)-1, int(q*float64(len(ds))))]
}

func hm(d time.Duration) string {
	d = d.Round(time.Minute)
	return fmt.Sprintf("%dh%02d", int(d.Hours()), int(d.Minutes())%60)
}

func (m mapSum) line() string {
	h := m.hazard()
	n := max(m.boots, 1)
	return fmt.Sprintf("full %4.1f%% | hazard I/II/III %4.1f/%4.1f/%4.1f | G1 %3.1f%% | door time %s, clear median %s | fsck %.2f, fountain %.1f, boss retries %.2f a boot",
		pct(m.full, n), h[0], h[1], h[2], pct(m.g1, n), hm(meanDur(m.doorTime)), hm(quant(m.clearTime, 0.5)),
		float64(m.fsck)/float64(n), float64(m.fountain)/float64(n), float64(m.saves)/float64(n))
}

// bossLine is each boss's hazard: the boots it ended over the boots that
// reached it, and its retries a boot that reached it.
func (m mapSum) bossLine() string {
	var parts []string
	for _, k := range append(slices.Clone(game.BossKeys), game.FinalBoss) {
		b := m.bosses[k]
		if b.Reached == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %.1f%% of %d (retries %.2f)", k, pct(b.Crashed, b.Reached), b.Reached,
			float64(b.Retries)/float64(b.Reached)))
	}
	return strings.Join(parts, ", ")
}

type mapCfg struct {
	name string
	r    rules.Rules
}

// mapConfigs are the default and each lever of map.md §(i)1 on its own,
// with the boss twists off as one more.
func mapConfigs() []mapCfg {
	z := func(name string, f func(r *rules.Rules)) mapCfg {
		r := rules.Standard()
		r.Biomes, r.Nodes = slices.Clone(r.Biomes), slices.Clone(r.Nodes)
		f(&r)
		return mapCfg{name, r}
	}
	all := func(r *rules.Rules, f func(b *rules.Biome)) {
		for i := range r.Biomes {
			f(&r.Biomes[i])
		}
		for i := range r.Nodes {
			f(&r.Nodes[i])
		}
	}
	cs := []mapCfg{
		z("default", func(r *rules.Rules) {}),
		z("twists off", func(r *rules.Rules) { r.Twists = rules.Twists{} }),
		z("final retries", func(r *rules.Rules) { r.OneShot = 0 }),
		z("no fountain", func(r *rules.Rules) { r.FountainCycles = 0 }),
		z("kexec once a biome", func(r *rules.Rules) { r.KexecPerAct = false }),
		z("no fsck", func(r *rules.Rules) { r.FsckPrice = 0 }),
		z("fails 10 in every act", func(r *rules.Rules) {
			all(r, func(b *rules.Biome) {
				if b.FailCycles == 12 || b.FailCycles == 15 {
					b.FailCycles = 10
				}
			})
		}),
		z("Overclock free, compounding", func(r *rules.Rules) { r.OverclockMax, r.OverclockCycles = 0, 0 }),
		z("timer_mult 0.8", func(r *rules.Rules) { r.TimerMult = 0.8 }),
	}
	if only := os.Getenv("KATHARSIS_MAP_ONLY"); only != "" {
		cs = slices.DeleteFunc(cs, func(c mapCfg) bool {
			return !slices.ContainsFunc(strings.Split(only, ","), func(p string) bool { return strings.HasPrefix(c.name, p) })
		})
	}
	return cs
}

// cell plays rating's model at prestige p on r over seeds, boots 51-250 of
// each (the static model before the floor runs dry, map.md §(i)).
func cell(r rules.Rules, pool []catalog.Problem, rating float64, p int, boots int, seeds []uint64) mapSum {
	var m mapSum
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, seed := range seeds {
		wg.Go(func() {
			rep := sim.Run(sim.Config{Rules: r, Pool: pool, Player: sim.DefaultPlayer(rating), Boots: boots, Warmup: 50, Seed: seed, Prestige: p})
			mu.Lock()
			m.add(rep)
			mu.Unlock()
		})
	}
	wg.Wait()
	return m
}

func TestMapReport(t *testing.T) {
	if os.Getenv("KATHARSIS_MAP") == "" {
		t.Skip("set KATHARSIS_MAP=1 for the map report")
	}
	name, pool := mapPool(t)
	boots := 200
	if n, err := strconv.Atoi(os.Getenv("KATHARSIS_MAP_BOOTS")); err == nil && n > 0 {
		boots = n
	}
	parts := os.Getenv("KATHARSIS_MAP_PARTS")
	want := func(p string) bool { return parts == "" || strings.Contains(parts, p) }
	ratings := []float64{1000, 1300, 1600}

	if want("p0") {
		cfgs := mapConfigs()
		cells := make([][]mapSum, len(cfgs))
		var wg sync.WaitGroup
		for ci, c := range cfgs {
			cells[ci] = make([]mapSum, len(ratings))
			for ri, rating := range ratings {
				wg.Go(func() { cells[ci][ri] = cell(c.r, pool, rating, 0, boots, []uint64{7, 8}) })
			}
		}
		wg.Wait()
		var w strings.Builder
		fmt.Fprintf(&w, "%s, P0, boots 51-%d of seeds 7 and 8\n", name, boots+50)
		for ci, c := range cfgs {
			for ri, rating := range ratings {
				fmt.Fprintf(&w, "  %-28s %4.0f: %s\n", c.name, rating, cells[ci][ri].line())
				if ci < 2 {
					fmt.Fprintf(&w, "  %-28s       bosses: %s\n", "", cells[ci][ri].bossLine())
				}
			}
		}
		t.Logf("p0:\n%s", w.String())
	}

	if want("levels") {
		lr := []float64{1000, 1600}
		cells := make([][]mapSum, game.MaxPrestige+1)
		var wg sync.WaitGroup
		for p := range game.MaxPrestige + 1 {
			cells[p] = make([]mapSum, len(lr))
			for ri, rating := range lr {
				wg.Go(func() { cells[p][ri] = cell(rules.Standard(), pool, rating, p, boots, []uint64{7}) })
			}
		}
		wg.Wait()
		var w strings.Builder
		fmt.Fprintf(&w, "%s, each prestige for boots 51-%d, seed 7\n", name, boots+50)
		for p := range cells {
			for ri, rating := range lr {
				fmt.Fprintf(&w, "  P%d %4.0f: %s\n", p, rating, cells[p][ri].line())
				fmt.Fprintf(&w, "          bosses: %s\n", cells[p][ri].bossLine())
			}
		}
		t.Logf("levels:\n%s", w.String())
	}
}

// TestMapLearn is the long horizon (map.md §(i)5): 30 learning players
// rated 900, 1100 and 1300, a year each, choosing their level (the highest
// unlocked whose last 10 boots there cleared 30%), with churn and without.
// Set KATHARSIS_MAP=1 and KATHARSIS_MAP_PARTS=learn.
func TestMapLearn(t *testing.T) {
	if os.Getenv("KATHARSIS_MAP") == "" || !strings.Contains(os.Getenv("KATHARSIS_MAP_PARTS"), "learn") {
		t.Skip("set KATHARSIS_MAP=1 KATHARSIS_MAP_PARTS=learn")
	}
	name, pool := mapPool(t)
	players := 30
	if n, err := strconv.Atoi(os.Getenv("KATHARSIS_MAP_PLAYERS")); err == nil && n > 0 {
		players = n
	}
	for _, churn := range []bool{true, false} {
		type sum struct {
			durable, hours           float64
			quit, boots, g1, full, l int
			prestige                 []int
			byP                      [game.MaxPrestige + 1]sim.Tally
		}
		var x sum
		var mu sync.Mutex
		var wg sync.WaitGroup
		for i := range players {
			wg.Go(func() {
				p := sim.DefaultPlayer([]float64{900, 1100, 1300}[i%3])
				p.Learn = sim.DefaultLearning()
				if !churn {
					p.Learn.Churn = nil
				}
				rep := sim.Run(sim.Config{Rules: rules.Standard(), Pool: pool, Player: p, Boots: 2000, Seed: uint64(1000*i + 7),
					Prestige: -1, Days: 365, ClimbFloor: 0.3})
				mu.Lock()
				defer mu.Unlock()
				x.durable += rep.Learn.Durable
				x.hours += rep.Learn.Hours
				x.boots += rep.Boots
				x.full += rep.FullClears
				x.g1 += rep.CrashBiome[0]
				x.l += rep.Learn.LLL
				x.prestige = append(x.prestige, rep.Prestige)
				for l, tl := range rep.ByPrestige {
					x.byP[l].Boots += tl.Boots
					x.byP[l].FullClears += tl.FullClears
				}
				if rep.Learn.Quit {
					x.quit++
				}
			})
		}
		wg.Wait()
		n := float64(players)
		slices.Sort(x.prestige)
		var by []string
		for l, tl := range x.byP {
			if tl.Boots > 0 {
				by = append(by, fmt.Sprintf("P%d %.0f%% of %d", l, pct(tl.FullClears, tl.Boots), tl.Boots))
			}
		}
		top := 0
		for _, p := range x.prestige {
			if p == game.MaxPrestige {
				top++
			}
		}
		t.Logf("%s, %d learning players (900/1100/1300), a year each, churn %v: durable %+.0f, hours %.0f, boots %.0f, quit %.0f%%, "+
			"G1 %.1f%%, full %.1f%%, runs of three crashes %.1f, at P%d %d of %d; by prestige: %s",
			name, players, churn, x.durable/n, x.hours/n, float64(x.boots)/n, pct(x.quit, players), pct(x.g1, x.boots),
			pct(x.full, x.boots), float64(x.l)/n, game.MaxPrestige, top, players, strings.Join(by, ", "))
	}
}

// TestGuardrailBand holds the map default to the trial-by-fire band
// (D49, map.md §(a)) with the boss twists on: at P0 the static model
// players clear 35 to 60% of their boots (the band's 40-55% with two
// seeds' noise), deaths come late (each act's hazard above the last's, act
// III's under 45%), and no one boss ends more than half the boots that
// reach it. Synthetic pool, boots 51-250 of seeds 7 and 8.
func TestGuardrailBand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	pool := sim.SyntheticPool(20000, 1)
	for _, rating := range []float64{1000, 1600} {
		t.Run(fmt.Sprint(rating), func(t *testing.T) {
			t.Parallel()
			m := cell(rules.Standard(), pool, rating, 0, 200, []uint64{7, 8})
			h := m.hazard()
			t.Logf("model %.0f: %s\n  bosses: %s", rating, m.line(), m.bossLine())
			if f := pct(m.full, m.boots); f < 35 || f > 60 {
				t.Errorf("full clears %.1f%%, want 35-60%%", f)
			}
			if !(h[0] < h[2] && h[1] < h[2]) || h[2] > 45 {
				t.Errorf("hazard by act %.1f/%.1f/%.1f%%: deaths should come late, act III under 45%%", h[0], h[1], h[2])
			}
			for _, k := range slices.Sorted(maps.Keys(m.bosses)) {
				if b := m.bosses[k]; b.Reached >= 50 && pct(b.Crashed, b.Reached) > 50 {
					t.Errorf("%s ends %.1f%% of the %d boots that reach it", k, pct(b.Crashed, b.Reached), b.Reached)
				}
			}
		})
	}
}
