package game_test

import (
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// TestMapClock: the map's base clock per tier (D50: tight, time is
// bought), timer_mult scaling it, and Userland's own clock per prestige
// outside the ascensions' cuts (10:00, 9:00 at P3).
func TestMapClock(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	all := &game.Meta{Prestige: game.MaxPrestige}
	for _, c := range game.TopicCapabilities {
		game.GrantCapability(all, c)
	}
	for _, tc := range []struct {
		prestige       int
		mult           float64
		userland, actI time.Duration
	}{
		{0, 1, 10 * time.Minute, 21 * time.Minute},
		{0, 0.9, 9 * time.Minute, 19 * time.Minute},
		{1, 1, 10 * time.Minute, 20 * time.Minute},
		{3, 1, 9 * time.Minute, 18 * time.Minute},
	} {
		r := mapPlain()
		r.TimerMult = tc.mult
		r.OverclockFrac = 0 // the harness picks whatever is offered first
		m := *all
		h := newHarness(t, r, &m, ps, tc.prestige, 1)
		if got := h.s.Doors[0].Timer; got != tc.userland {
			t.Errorf("P%d x%v: Userland %s, want %s", tc.prestige, tc.mult, got, tc.userland)
		}
		nextTier(h)
		h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "bunker")})
		if got := h.s.Doors[0].Timer; got != tc.actI || h.s.Biomes[1].Exits[0].Timer != tc.actI {
			t.Errorf("P%d x%v: act I %s (card %s), want %s", tc.prestige, tc.mult, got, h.s.Biomes[1].Exits[0].Timer, tc.actI)
		}
	}
}

// TestOverclockBought: Overclock adds 15% a pick, two at most, for 5
// cycles each; it is not offered past the cap or without more than 5
// cycles; doors already on offer get the new clock.
func TestOverclockBought(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, cyclesPlain(), nil, ps, 0, 1)
	base := h.s.Doors[0].Timer
	for n, want := range []float64{1.15, 1.30} {
		h.clearDoor(0, 6*time.Minute) // past -O3
		h.s.UpgradeOffers = []game.Upgrade{game.Overclock}
		c := h.s.Cycles
		h.do(game.PickUpgrade{Upgrade: game.Overclock})
		if h.s.Cycles != c-5 {
			t.Errorf("pick %d: cycles %d -> %d, want -5", n+1, c, h.s.Cycles)
		}
		if h.s.Phase == game.PhaseDoors {
			if got := h.s.Doors[0].Timer; got.Minutes() < float64(base.Minutes())*want-1 || got.Minutes() > float64(base.Minutes())*want+1 {
				t.Errorf("pick %d: door on offer %s, want about %.0f%% of %s", n+1, got, 100*want, base)
			}
		}
		if h.s.Phase != game.PhaseDoors {
			break
		}
	}
	s := h.s
	s.Upgrades = append(s.Upgrades, game.Overclock) // a third
	for range 30 {
		if s.PendingUpgrade && slices.Contains(s.UpgradeOffers, game.Overclock) {
			t.Fatal("Overclock offered past its cap")
		}
		if s.Phase != game.PhaseDoors {
			break
		}
		h.clearDoor(0, 6*time.Minute)
	}

	low := newHarness(t, cyclesPlain(), nil, ps, 0, 2)
	low.s.Cycles = 5
	for range 30 {
		low.clearDoor(0, 6*time.Minute)
		if low.s.PendingUpgrade && slices.Contains(low.s.UpgradeOffers, game.Overclock) {
			t.Fatal("Overclock offered with 5 cycles left")
		}
		if low.s.Phase != game.PhaseDoors {
			break
		}
		low.pick()
	}
}

// TestO3OnBase: -O3 measures the clock before bought time: a door that
// opened with at's 5:00 on top must still be cleared within half its own
// timer.
func TestO3OnBase(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	for _, onBase := range []bool{true, false} {
		r := cyclesPlain()
		r.O3OnBase = onBase
		r.Packages = slices.Clone(r.Packages)
		m := &game.Meta{Installed: []string{"at"}}
		h := newHarness(t, r, m, ps, 0, 1)
		h.s.AtTime = 5 * time.Minute // bought with at
		d := h.open(0)
		if h.s.Current.At != 5*time.Minute || h.s.Current.Door.Timer != d.Timer {
			t.Fatalf("at: attempt %+v", h.s.Current)
		}
		// Half the door's own timer and a minute: -O3 on the bought timer,
		// not on the base.
		h.now = h.s.Current.Opened.Add((d.Timer-5*time.Minute)/2 + time.Minute)
		if err := stepNow(h, game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10}); err != nil {
			t.Fatal(err)
		}
		if got := h.s.History[0].O3; got == onBase {
			t.Errorf("O3OnBase %v: -O3 %v at %s of %s (5:00 bought)", onBase, got, h.now.Sub(h.s.History[0].Opened), d.Timer)
		}
	}
}

// TestKexecPerAct: on the map kexec's free swap is once an act: used in
// act I's biome it stays used at Boss I, and comes back in act II.
func TestKexecPerAct(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, mapPlain(), &game.Meta{}, ps, 0, 1)
	nextTier(h)
	h.s.Daemons = append(h.s.Daemons, game.Kexec)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")})
	h.open(0)
	h.do(game.Retreat{})
	if !h.s.KexecUsed {
		t.Fatal("kexec not used")
	}
	nextTier(h) // to Boss I
	if !h.s.KexecUsed || h.s.Biome != 2 {
		t.Errorf("Boss I: kexec used %v (biome %d), want still used in act I", h.s.KexecUsed, h.s.Biome)
	}
	nextTier(h) // past Boss I
	if h.s.KexecUsed || h.s.Biome != 3 {
		t.Errorf("act II: kexec used %v (biome %d), want fresh", h.s.KexecUsed, h.s.Biome)
	}
	// Before cycles it was once a biome.
	l := newHarness(t, plain(), nil, ps, 0, 1)
	l.s.Daemons = append(l.s.Daemons, game.Kexec)
	l.open(0)
	l.do(game.Retreat{})
	l.toMerge()
	l.pick()
	l.do(game.LeaveMerge{})
	if l.s.KexecUsed {
		t.Error("legacy: kexec stayed used past its biome")
	}
}

// TestFountain: a merge window after a boss restores 5 cycles, free, up to
// the starting pool; the windows after a topic biome have none; a ladder
// that cuts it (P5) leaves none.
func TestFountain(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	for _, tc := range []struct{ from, want, gain int }{{20, 25, 5}, {28, 30, 2}, {30, 30, 0}, {35, 35, 0}} {
		h := newHarness(t, mapPlain(), &game.Meta{}, ps, 0, 1)
		nextTier(h)
		h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")})
		toMergeFast(h)
		if h.s.Fountain != 0 || h.s.Fountained != 0 {
			t.Fatalf("a fountain after act I's biome: %d", h.s.Fountain)
		}
		h.pick()
		h.do(game.LeaveMerge{})
		h.s.Cycles = tc.from
		h.clearDoor(0, 20*time.Minute) // Boss I
		if h.s.Phase != game.PhaseMerge || h.s.Cycles != tc.want || h.s.Fountain != tc.gain || h.s.Fountained != tc.gain {
			t.Errorf("from %d: cycles %d, fountain %d (%d in all)", tc.from, h.s.Cycles, h.s.Fountain, h.s.Fountained)
		}
		h.pick()
		h.do(game.LeaveMerge{})
		if h.s.Fountain != 0 {
			t.Errorf("fountain %d after the window", h.s.Fountain)
		}
	}
	r := mapPlain()
	r.Ascensions = []rules.Ascension{{Level: 1, FountainCycles: -5}}
	h := newHarness(t, r, &game.Meta{Prestige: 1}, ps, 1, 1)
	nextTier(h)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")})
	nextTier(h)
	h.s.Cycles = 20
	h.clearDoor(0, 20*time.Minute)
	if h.s.Cycles != 20 || h.s.Fountained != 0 {
		t.Errorf("a ladder without the fountain: cycles %d, fountain %d", h.s.Cycles, h.s.Fountained)
	}
}
