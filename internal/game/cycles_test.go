package game_test

import (
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// cyclesPlain is cyclesLinear without easing, door modifiers or boot #1's
// rules, so a test can predict every price.
func cyclesPlain() rules.Rules {
	r := calm(cyclesLinear())
	r.BlindChance, r.MysteryChance, r.CursedChance = 0, 0, 0
	return r
}

var wa = game.SubmitResult{Verdict: "WA", Passed: 3, Total: 10}

func lastOffer(s *game.State) game.Offer { return s.Offers[len(s.Offers)-1] }

// TestCyclesSubmit: a failed hidden-test submit burns 2 (3 at P1), a CE and
// a submit that failed a sample burn nothing, and the submit that would
// leave none ends the boot.
func TestCyclesSubmit(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, cyclesPlain(), nil, ps, 0, 1)
	if h.s.Cycles != 30 || h.s.MaxCycles != 50 {
		t.Fatalf("start %d/%d, want 30/50", h.s.Cycles, h.s.MaxCycles)
	}
	h.open(0)
	if c, k := h.s.SubmitCost(1, h.now); c != 2 || k != 0 {
		t.Errorf("SubmitCost %d %s", c, k)
	}
	h.do(wa)
	h.do(game.SubmitResult{Verdict: "CE"})
	h.do(game.SubmitResult{Verdict: "WA", Passed: 1, Total: 10, SampleFailed: true})
	if h.s.Cycles != 28 || h.s.Phase != game.PhaseProblem || h.s.Current.Failed != 3 {
		t.Fatalf("cycles %d phase %s: a WA burns 2, a CE and a sample failure nothing", h.s.Cycles, h.s.Phase)
	}
	for h.s.Phase == game.PhaseProblem {
		if err := stepNow(h, wa); err != nil {
			t.Fatal(err)
		}
	}
	if h.s.Cycles != 0 || h.s.Crash == nil || h.s.Crash.Cause != game.OutOfLives || h.s.Current != nil {
		t.Errorf("after 15 WAs: cycles %d crash %+v", h.s.Cycles, h.s.Crash)
	}

	p1 := newHarness(t, cyclesPlain(), &game.Meta{Prestige: 1}, ps, 1, 1)
	p1.open(0)
	p1.do(wa)
	if p1.s.Cycles != 27 {
		t.Errorf("P1: a WA leaves %d of 30, want 27", p1.s.Cycles)
	}
}

// TestCyclesFail: a timeout or give-up burns its biome's price; the room is
// lost and the boot goes on; the door is rated failed and comes back as a
// regression. The give-up that would leave none ends the boot with the
// editorial, as :q! always did.
func TestCyclesFail(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, cyclesPlain(), nil, ps, 0, 1)
	d := h.open(0)
	if c, ok := h.s.FailCost(game.TimedOut); c != 10 || !ok || !h.s.TimeoutCostsLife() {
		t.Errorf("Userland timeout costs %d (%v)", c, ok)
	}
	rated := h.m.Rated
	h.now = h.s.Current.Deadline
	fx := h.do(game.Tick{})
	a := h.s.History[0]
	if h.s.Cycles != 20 || a.Outcome != game.TimedOut || h.s.Phase != game.PhaseDoors || h.s.Room != 1 || h.m.Rated != rated+1 ||
		h.s.PendingUpgrade || lastOffer(h.s).Reason != "fresh" {
		t.Fatalf("timeout: cycles %d attempt %+v phase %s room %d pending %v", h.s.Cycles, a.Outcome, h.s.Phase, h.s.Room, h.s.PendingUpgrade)
	}
	if !slices.ContainsFunc(h.m.Regressions, func(g game.Regression) bool { return g.Problem == d.Problem }) {
		t.Error("the timed-out door is not scheduled to come back")
	}
	if _, ok := has[game.CloseProblem](fx); !ok {
		t.Errorf("effects %v", fx)
	}

	// The give-up of Userland's last room is the biome's end: the merge
	// window, with nothing owed for the lost room.
	h.open(0)
	fx = h.do(game.GiveUp{})
	if h.s.Cycles != 10 || h.s.Phase != game.PhaseMerge || h.s.PendingUpgrade || h.s.Uncommitted != 0 {
		t.Fatalf("give-up: cycles %d phase %s pending %v", h.s.Cycles, h.s.Phase, h.s.PendingUpgrade)
	}
	if _, ok := has[game.OpenEditorial](fx); ok {
		t.Error("a give-up that cost cycles opened the editorial; it waits for the upsolve")
	}

	h.do(game.LeaveMerge{})
	h.do(game.ChooseRoute{Index: 0})
	h.open(0)
	if c, ok := h.s.FailCost(game.GaveUp); c != 10 || ok {
		t.Errorf("with 10 left a give-up costs %d (%v), want the boot", c, ok)
	}
	fx = h.do(game.GiveUp{})
	if h.s.Crash == nil || h.s.Crash.Cause != game.GaveUp || h.s.Cycles != 10 {
		t.Fatalf("give-up on the last cycles: crash %+v cycles %d", h.s.Crash, h.s.Cycles)
	}
	if _, ok := has[game.OpenEditorial](fx); !ok {
		t.Error("a give-up that ended the boot did not open the editorial")
	}
}

// TestBossRetry: Bosses I and II burn 20 and draw a new boss door, the same
// boss; the final is one shot; with 20 left a boss failure ends the boot.
func TestBossRetry(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	r := cyclesPlain()
	boss := r.Biomes[2]
	r.Biomes = []rules.Biome{r.Biomes[0], boss, boss}
	r.Biomes[0].Rooms = 1
	h := newHarness(t, r, nil, ps, 0, 1)
	toMergeFast(h)
	h.pick()
	h.do(game.LeaveMerge{})
	if h.s.OneShot() {
		t.Fatal("Boss I is one-shot")
	}
	key := h.s.Biomes[1].Boss
	d := h.open(0)
	c0 := h.s.Cycles // an Overclock pick may have spent some
	h.now = h.s.Current.Deadline
	h.do(game.Tick{})
	if h.s.Cycles != c0-20 || h.s.Phase != game.PhaseDoors || h.s.Room != 0 || len(h.s.Doors) != 1 || h.s.Doors[0].Problem == d.Problem ||
		h.s.Doors[0].Boss != key || lastOffer(h.s).Reason != "retry" {
		t.Fatalf("boss timeout: cycles %d phase %s room %d doors %+v", h.s.Cycles, h.s.Phase, h.s.Room, h.s.Doors)
	}
	h.open(0)
	h.s.Cycles = 20
	h.do(game.GiveUp{})
	if h.s.Crash == nil || h.s.Crash.Cause != game.GaveUp {
		t.Fatalf("a boss failure with 20 left: crash %+v", h.s.Crash)
	}

	// The final is one shot, whatever the cycles.
	h = newHarness(t, r, nil, ps, 0, 2)
	toMergeFast(h)
	h.pick()
	h.do(game.LeaveMerge{})
	h.clearDoor(0, time.Minute)
	toMergeFast(h)
	h.pick()
	h.do(game.LeaveMerge{})
	if !h.s.OneShot() {
		t.Fatal("the final is not one-shot")
	}
	h.open(0)
	if c, ok := h.s.FailCost(game.TimedOut); c != 0 || ok {
		t.Errorf("the final's timeout costs %d (%v)", c, ok)
	}
	c0 = h.s.Cycles
	h.now = h.s.Current.Deadline
	h.do(game.Tick{})
	if h.s.Crash == nil || h.s.Crash.Cause != game.TimedOut || h.s.Cycles != c0 {
		t.Errorf("the final's timeout: crash %+v cycles %d", h.s.Crash, h.s.Cycles)
	}
}

// TestRetreatCycles: a retreat burns 5, is refused when it would leave
// none, and locks 2:30 into a door, kexec's free swap too.
func TestRetreatCycles(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, cyclesPlain(), nil, ps, 0, 1)
	h.open(0)
	if at := h.s.RetreatLockAt(); !at.Equal(h.s.Current.Opened.Add(150 * time.Second)) {
		t.Errorf("locks at %v", at)
	}
	h.do(game.Retreat{})
	if h.s.Cycles != 25 || h.s.Phase != game.PhaseDoors || lastOffer(h.s).Reason != "retreat" {
		t.Fatalf("retreat: cycles %d phase %s", h.s.Cycles, h.s.Phase)
	}
	h.open(0)
	h.now = h.s.Current.Opened.Add(150 * time.Second)
	if err := stepNow(h, game.Retreat{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("retreat at 2:30: %v", err)
	}
	h.s.Daemons = append(h.s.Daemons, game.Kexec)
	if err := stepNow(h, game.Retreat{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("kexec at 2:30: %v", err)
	}
	h.now = h.s.Current.Opened.Add(149 * time.Second)
	if err := stepNow(h, game.Retreat{}); err != nil || h.s.Cycles != 25 || !h.s.KexecUsed {
		t.Errorf("kexec at 2:29: %v, cycles %d", err, h.s.Cycles)
	}
	h.s.Cycles = 5
	h.open(0)
	if err := h.try(game.Retreat{}); !errors.Is(err, game.ErrInvalid) || h.s.Cycles != 5 {
		t.Errorf("retreat with 5 left: %v", err)
	}
}

// TestECCConversion: under cycles ECC is offered once a boot, and from its
// pick a failed submit costs 5:00 of the door's clock, not 2 cycles, while
// more than 5:00 is left; with less it burns the 2 cycles.
func TestECCConversion(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, cyclesPlain(), nil, ps, 0, 1)
	h.clearDoor(0, time.Minute)
	if !h.s.PendingUpgrade {
		t.Fatal("no pick")
	}
	h.s.UpgradeOffers = []game.Upgrade{game.ECC}
	h.do(game.PickUpgrade{Upgrade: game.ECC})
	if h.s.Cycles != 30 || !h.s.ECCConverts() {
		t.Fatalf("ECC healed to %d (converts %v)", h.s.Cycles, h.s.ECCConverts())
	}
	h.open(0)
	c0 := h.s.Cycles // the -O3 pick may have been an Overclock
	deadline := h.s.Current.Deadline
	if c, k := h.s.SubmitCost(2, h.now); c != 0 || k != 5*time.Minute {
		t.Errorf("SubmitCost under ECC %d %s", c, k)
	}
	h.do(wa)
	if h.s.Cycles != c0 || !h.s.Current.Deadline.Equal(deadline.Add(-5*time.Minute)) {
		t.Fatalf("ECC WA: cycles %d, deadline moved %s", h.s.Cycles, deadline.Sub(h.s.Current.Deadline))
	}
	// With 5:00 or less left, the 5:00 would run the clock out, and a
	// timeout burns 10: the WA burns its 2 cycles instead, and the clock
	// is left alone.
	h.now = h.s.Current.Deadline.Add(-6 * time.Minute) // judged at 05:00 left
	deadline = h.s.Current.Deadline
	h.do(wa)
	if h.s.Cycles != c0-2 || h.s.Phase != game.PhaseProblem {
		t.Fatalf("a WA with 05:00 left: cycles %d (want %d), phase %s", h.s.Cycles, c0-2, h.s.Phase)
	}
	if !h.s.Current.Deadline.Equal(deadline) {
		t.Errorf("a WA with 05:00 left moved the deadline %s", deadline.Sub(h.s.Current.Deadline))
	}
	h.now = h.s.Current.Deadline.Add(-time.Minute)
	h.do(wa) // judged as the clock runs out
	if h.s.Cycles != c0-4 || h.s.Phase != game.PhaseProblem {
		t.Fatalf("a WA at 00:00: cycles %d (want %d), phase %s", h.s.Cycles, c0-4, h.s.Phase)
	}
	for range 50 {
		if h.s.Phase == game.PhaseDoors && h.s.PendingUpgrade && slices.Contains(h.s.UpgradeOffers, game.ECC) {
			t.Fatal("ECC offered twice")
		}
		if h.s.Phase != game.PhaseDoors {
			break
		}
		h.clearDoor(0, time.Minute)
	}
}

// TestFsckCycles: fsck restores 10 (5 from P2), up to MaxCycles.
func TestFsckCycles(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	for _, tc := range []struct{ prestige, from, want int }{{0, 15, 25}, {0, 45, 50}, {2, 15, 20}} {
		h := newHarness(t, cyclesPlain(), &game.Meta{Prestige: tc.prestige}, ps, tc.prestige, 1)
		toMergeFast(h)
		h.s.Cycles, h.s.Uncommitted = tc.from, 1000
		h.do(game.Spend{Item: game.Fsck})
		if h.s.Cycles != tc.want {
			t.Errorf("P%d: fsck from %d to %d, want %d", tc.prestige, tc.from, h.s.Cycles, tc.want)
		}
	}
}

// stepNow steps a at the harness's clock without moving it.
func stepNow(h *harness, a game.Action) error {
	_, err := game.Step(h.s, h.m, h.ps, a, h.now)
	return err
}

// toMergeFast clears every room of the current biome a minute in: the
// cycles clocks are tight.
func toMergeFast(h *harness) {
	h.t.Helper()
	b := h.s.Biome
	for h.s.Biome == b && h.s.Phase == game.PhaseDoors {
		h.clearDoor(0, time.Minute)
	}
	if h.s.Phase != game.PhaseMerge {
		h.t.Fatalf("phase %s after clearing biome %d, want merge", h.s.Phase, b)
	}
}

// TestTwists: each boss bends its one rule where the rules turn it on, and
// none where they do not (a boot saved before twists).
func TestTwists(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	bossOnly := func(r rules.Rules) rules.Rules {
		r = calm(r)
		r.Biomes = []rules.Biome{r.Biomes[len(r.Biomes)-1]}
		return r
	}
	on, off := bossOnly(cyclesPlain()), bossOnly(rules.Legacy())
	// door finds a boot whose boss is key and returns its door under both.
	door := func(key string) (twisted, plain game.Door) {
		prestige := 0
		if key == game.FinalBoss {
			prestige = on.TopPrestige(game.MaxPrestige)
		}
		for seed := range uint64(500) {
			m := &game.Meta{Prestige: game.MaxPrestige}
			h := newHarness(t, on, m, ps, prestige, seed)
			if h.s.Biomes[0].Boss != key {
				continue
			}
			p := newHarness(t, off, &game.Meta{Prestige: game.MaxPrestige}, ps, prestige, seed)
			return h.s.Doors[0], p.s.Doors[0]
		}
		t.Fatalf("no seed draws %s", key)
		return
	}
	for _, key := range append(slices.Clone(game.BossKeys), game.FinalBoss) {
		d, p := door(key)
		if d.Twist != key || p.Twist != "" || p.MemMult != 0 || p.QuietSamples || p.NoSanitizers || p.HiddenClock {
			t.Errorf("%s: twist %q, legacy %+v", key, d.Twist, p)
		}
		switch key {
		case "oom-killer":
			if d.MemMult != 0.5 {
				t.Errorf("oom-killer: memory x%v", d.MemMult)
			}
		case "heisenbug":
			if !d.QuietSamples {
				t.Error("heisenbug: samples tell")
			}
		case "undefined-behavior":
			if !d.NoSanitizers {
				t.Error("undefined-behavior: sanitizers on")
			}
		case game.FinalBoss:
			if !d.HiddenClock {
				t.Error("halting-problem: clock shown")
			}
		case "race-condition":
			// x0.8 the clock for x4 the pay instead of x3, against the same
			// door without the twist.
			base := on.Biomes[0].Timer.Duration
			want := math.Round(float64(on.BaseBytes) * min(max(0.5/d.SolveP, on.MinBytesMult), on.MaxBytesMult) * 4)
			if d.Timer != (base*4/5).Round(time.Minute) || float64(d.Bytes) != want {
				t.Errorf("race-condition: timer %s (base %s), %d B, want %.0f", d.Timer, base, d.Bytes, want)
			}
		}
	}

	// Off-by-One burns double for a submit one test short.
	for seed := range uint64(500) {
		h := newHarness(t, on, nil, ps, 0, seed)
		if h.s.Biomes[0].Boss != "off-by-one" {
			continue
		}
		h.open(0)
		if c, _ := h.s.SubmitCost(1, h.now); c != 4 {
			t.Errorf("off-by-one, one test failing: %d", c)
		}
		if c, _ := h.s.SubmitCost(2, h.now); c != 2 {
			t.Errorf("off-by-one, two failing: %d", c)
		}
		if err := stepNow(h, game.SubmitResult{Verdict: "WA", Passed: 9, Total: 10}); err != nil || h.s.Cycles != 26 {
			t.Errorf("off-by-one WA 9/10: %v, cycles %d", err, h.s.Cycles)
		}
		// Under ECC the double is 10:00 of the clock, and with 7:00 left
		// the near miss burns its 4 cycles instead; two short still pay
		// their 5:00.
		h.s.Upgrades = append(h.s.Upgrades, game.ECC)
		h.now = h.s.Current.Deadline.Add(-7 * time.Minute)
		deadline := h.s.Current.Deadline
		for _, c := range []struct {
			passed, cycles int
			moved          time.Duration
		}{{9, 22, 0}, {8, 22, 5 * time.Minute}} {
			err := stepNow(h, game.SubmitResult{Verdict: "WA", Passed: c.passed, Total: 10})
			if err != nil || h.s.Current == nil {
				t.Fatalf("off-by-one WA %d/10 under ECC, 07:00 left: %v, phase %s", c.passed, err, h.s.Phase)
			}
			if moved := deadline.Sub(h.s.Current.Deadline); h.s.Cycles != c.cycles || moved != c.moved {
				t.Errorf("off-by-one WA %d/10 under ECC, 07:00 left: cycles %d, clock -%s; want %d, -%s",
					c.passed, h.s.Cycles, moved, c.cycles, c.moved)
			}
		}
		return
	}
	t.Fatal("no seed draws off-by-one")
}

// TestLadderBitesLate: the map's prestige cuts land on acts II and III and
// on the pool, not on Userland (map.md §(d)): at P3 a fail after Boss I
// burns 5 more, P4 starts at 25, P6 makes Boss II one shot and P8 every
// boss. P6 and P8 are read off the state, as MaxPrestige may stop short of
// them.
func TestLadderBitesLate(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	m := &game.Meta{Prestige: 4}
	h := newHarness(t, mapPlain(), m, ps, 3, 1)
	h.open(0)
	if c, _ := h.s.FailCost(game.TimedOut); c != 10 {
		t.Errorf("P3 Userland fail %d, want 10", c)
	}
	h.do(game.Retreat{})
	nextTier(h)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")})
	h.open(0)
	if c, _ := h.s.FailCost(game.GaveUp); c != 10 {
		t.Errorf("P3 act I fail %d, want 10", c)
	}
	h.do(game.Retreat{})
	nextTier(h)
	h.open(0)
	if c, _ := h.s.FailCost(game.GaveUp); c != 20 || h.s.OneShot() {
		t.Errorf("P3 Boss I fail %d (one shot %v), want 20", c, h.s.OneShot())
	}
	h.s.Prestige = 8
	if !h.s.OneShot() {
		t.Error("P8: Boss I retries")
	}
	h.s.Prestige = 3
	h.do(game.Retreat{})
	h.s.Cycles = 50
	nextTier(h) // Off-by-One cleared
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "alu")})
	h.open(0)
	if c, _ := h.s.FailCost(game.GaveUp); c != 17 {
		t.Errorf("P3 act II fail %d, want 12+5", c)
	}
	h.do(game.Retreat{})
	nextTier(h)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "heisenbug")})
	for p, one := range map[int]bool{5: false, 6: true} {
		h.s.Prestige = p
		if h.s.OneShot() != one {
			t.Errorf("P%d: Boss II one shot %v, want %v", p, h.s.OneShot(), one)
		}
	}
	h.s.Prestige = 3
	if c, _ := h.s.FailCost(game.TimedOut); c != 25 {
		t.Errorf("P3 Boss II fail %d, want 20+5", c)
	}

	p4 := newHarness(t, mapPlain(), &game.Meta{Prestige: 4}, ps, 4, 1)
	if p4.s.Cycles != 25 || p4.s.StartCycles() != 25 {
		t.Errorf("P4 starts at %d (start %d), want 25", p4.s.Cycles, p4.s.StartCycles())
	}
}

// TestForecastCycles: under cycles D34's forecast is the chance Userland's
// failures burn the pool (meta.md §(b)), not the chance of one failure: at
// 30 cycles two 10-cycle rooms cannot crash a boot, at 20 both must fail.
// D34's easing, when it fires, adds a life of 10 cycles, as the governor
// does.
func TestForecastCycles(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	low := &game.Meta{Rating: 700, Boots: 3} // aimed under the pool's floor
	r := calm(cyclesLinear())
	r.EaseAbove, r.EaseLives = 0.2, 1
	h := newHarness(t, r, low, ps, 0, 1)
	if h.s.Forecast != 0 || len(h.s.Ease) != 0 {
		t.Errorf("30 cycles: forecast %v easings %+v, want none", h.s.Forecast, h.s.Ease)
	}
	lives := r
	lives.Biomes = slices.Clone(r.Biomes)
	lives.Biomes[0].FailCycles = 0
	l := newHarness(t, lives, &game.Meta{Rating: 700, Boots: 3}, ps, 0, 1)
	p := math.Sqrt(1 - l.s.Forecast) // the door's clear chance the old forecast used
	twenty := r
	twenty.Cycles, twenty.EaseAbove = 20, 0.01
	h = newHarness(t, twenty, &game.Meta{Rating: 700, Boots: 3}, ps, 0, 1)
	if want := (1 - p) * (1 - p); math.Abs(h.s.Forecast-want) > 1e-9 {
		t.Errorf("20 cycles: forecast %v, want both rooms failed, %v", h.s.Forecast, want)
	}
	if es, ok := active(h.s, game.EaseFloor); !ok || es.Cycles != 10 || h.s.Cycles != 30 {
		t.Errorf("D34 past 1%%: eased %+v (forecast %v), cycles %d, want +10", es, h.s.Forecast, h.s.Cycles)
	}
	gov := &game.Meta{Boots: 3, Governor: game.Governor{EarlyCrashes: 2}}
	r.GovernorAfter, r.GovernorLives = 2, 1
	h = newHarness(t, r, gov, ps, 0, 1)
	if es, ok := active(h.s, game.EaseGovernor); !ok || es.Cycles != 10 || h.s.Cycles != 40 {
		t.Errorf("governor %+v, cycles %d, want +10", es, h.s.Cycles)
	}
}
