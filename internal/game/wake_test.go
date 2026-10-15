package game_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// The Trial (docs/design/achievements.md §(f)): a hidden exit of act II's
// nodes into Boss II's tier, open to a full cycle pool; one cursed,
// one-shot door; and the boot ends awake.

// toBossII plays a map boot through The Scheduler, Off-by-One and The ALU
// to the merge window before Boss II.
func toBossII(t *testing.T, m *game.Meta) *harness {
	t.Helper()
	h := newHarness(t, mapPlain(), m, dense(catalog.Topics, 800, 2600, 20), 0, 4)
	nextTier(h)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")})
	nextTier(h)
	nextTier(h) // Off-by-One; its one open exit, The ALU, is taken at once
	if x := h.s.Biomes[3].Exits[exitIndex(t, h.s, "alu")]; !slices.Equal(x.Bosses, []string{"heisenbug", "race-condition"}) {
		t.Errorf("The ALU's card names %v: the Trial is no boss it names", x.Bosses)
	}
	if h.s.Biomes[3].Node != "alu" {
		t.Fatalf("act II plays %q", h.s.Biomes[3].Node)
	}
	toMergeFast(h)
	h.pick()
	return h
}

// trialExit is the Trial's exit at Boss II's tier, ok false when the route
// does not show it.
func trialExit(s *game.State) (game.Exit, int, bool) {
	ex := s.Biomes[rules.TierBossII].Exits
	i := slices.IndexFunc(ex, func(x game.Exit) bool { return x.Node == rules.TrialNode })
	if i < 0 {
		return game.Exit{}, -1, false
	}
	return ex[i], i, true
}

// fill fscks the merge window's pool to its cap.
func fill(h *harness) {
	h.t.Helper()
	h.s.Uncommitted += 1024
	for h.s.Cycles < h.s.MaxCycles {
		h.do(game.Spend{Item: game.Fsck})
	}
}

// TestTrialGate: below the cap the route never names the Trial; once a
// pool has hit the cap (Meta.Capped) it shows it locked; at the cap it is
// open, and the hint is set by the fsck that reached it.
func TestTrialGate(t *testing.T) {
	t.Parallel()
	h := toBossII(t, &game.Meta{})
	h.do(game.LeaveMerge{})
	if _, _, ok := trialExit(h.s); ok || h.s.Capped || h.m.Capped != 0 {
		t.Errorf("below the cap with no hint the route shows the Trial: %+v", h.s.Biomes[4].Exits)
	}

	h = toBossII(t, &game.Meta{Capped: 1})
	h.do(game.LeaveMerge{})
	x, i, ok := trialExit(h.s)
	if !ok || !x.Hidden || x.Locked != game.LockedFull || !x.Gate.Full || x.Ending != game.EndingWake || x.Boss != game.TrialBoss {
		t.Fatalf("hinted, below the cap: %+v (shown %v)", x, ok)
	}
	if h.s.Phase != game.PhaseRoute {
		t.Errorf("phase %s: two bosses on offer", h.s.Phase)
	}
	if err := h.try(game.ChooseRoute{Index: i}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("a locked Trial taken: %v", err)
	}
	if o := h.s.Offers[len(h.s.Offers)-1]; o.Items[i].Slot != "locked" || o.Items[i].Propensity != 1 {
		t.Errorf("logged %+v", o.Items[i])
	}

	m := &game.Meta{}
	h = toBossII(t, m)
	fill(h)
	if !h.s.Capped || m.Capped != h.s.Boot {
		t.Fatalf("at the cap: state %v, meta %d", h.s.Capped, m.Capped)
	}
	rng := h.s.RNG
	h.do(game.LeaveMerge{})
	if x, _, ok := trialExit(h.s); !ok || x.Locked != "" || h.s.Phase != game.PhaseRoute {
		t.Fatalf("at the cap: %+v, phase %s", x, h.s.Phase)
	}
	if h.s.RNG != rng {
		t.Error("the exits drew a random number")
	}
}

// TestTrialDoor: the Trial's door is cursed and one shot: no duck, and a
// failed submit, a timeout or :q! ends the boot.
func TestTrialDoor(t *testing.T) {
	t.Parallel()
	enter := func() *harness {
		h := toBossII(t, &game.Meta{})
		fill(h)
		h.do(game.LeaveMerge{})
		_, i, _ := trialExit(h.s)
		h.do(game.ChooseRoute{Index: i})
		return h
	}
	h := enter()
	s := h.s
	if s.Biomes[4].Node != rules.TrialNode || s.Phase != game.PhaseDoors || len(s.Doors) != 1 {
		t.Fatalf("took the Trial: %+v, phase %s, doors %+v", s.Biomes[4], s.Phase, s.Doors)
	}
	d := s.Doors[0]
	if !d.Cursed || d.Boss != game.TrialBoss || d.Twist != "" || d.Tier != "" || d.Timer <= 0 || !s.OneShot() {
		t.Errorf("the Trial's door %+v (one shot %v)", d, s.OneShot())
	}
	if c, ok := s.FailCost(game.TimedOut); c != 0 || ok {
		t.Errorf("a timeout costs %d (survives %v)", c, ok)
	}
	h.open(0)
	if err := h.try(game.Quack{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("a duck on the Trial: %v", err)
	}
	h.do(game.SubmitResult{Verdict: "WA", Passed: 9, Total: 10})
	if s.Phase != game.PhaseCrashed || !s.Crash.Cursed || s.Ending != "" {
		t.Errorf("one WA: phase %s, crash %+v", s.Phase, s.Crash)
	}

	h = enter()
	h.open(0)
	h.now = h.s.Current.Deadline
	h.do(game.Tick{})
	if h.s.Phase != game.PhaseCrashed || h.s.Crash.Cause != game.TimedOut {
		t.Errorf("a timeout: phase %s", h.s.Phase)
	}
	h = enter()
	h.open(0)
	h.do(game.GiveUp{})
	if h.s.Phase != game.PhaseCrashed {
		t.Errorf(":q!: phase %s", h.s.Phase)
	}
}

// TestTrialWake: clearing the Trial ends the boot awake: cleared, what is
// uncommitted banked, no prestige unlocked though the gate is open, no
// full clear for the governor, and the cleared screen told the ending.
func TestTrialWake(t *testing.T) {
	t.Parallel()
	m := &game.Meta{Governor: game.Governor{FullClears: 2}}
	h := toBossII(t, m)
	m.Rated = 1000 // the prestige gate is open
	fill(h)
	h.do(game.LeaveMerge{})
	_, i, _ := trialExit(h.s)
	h.do(game.ChooseRoute{Index: i})
	h.open(0)
	h.s.Uncommitted = 200
	bank := m.Bank
	h.now = h.now.Add(20 * time.Minute)
	fx := h.do(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
	s := h.s
	if s.Phase != game.PhaseCleared || s.Ending != game.EndingWake || s.Biome != rules.TierBossII {
		t.Fatalf("after the Trial: phase %s, ending %q, biome %d", s.Phase, s.Ending, s.Biome)
	}
	if s.Unlocked != 0 || m.Prestige != 0 {
		t.Errorf("unlocked %d, prestige %d: a wake unlocks nothing", s.Unlocked, m.Prestige)
	}
	if m.Governor != (game.Governor{FullClears: 2}) {
		t.Errorf("governor %+v: a wake is no crash and no full clear", m.Governor)
	}
	if s.Uncommitted != 0 || m.Bank <= bank+200 {
		t.Errorf("uncommitted %d, bank %d from %d", s.Uncommitted, m.Bank, bank)
	}
	if c, ok := has[game.ShowCleared](fx); !ok || c.Ending != game.EndingWake {
		t.Errorf("effects %v", fx)
	}
	if b := game.Debrief(s, m); !b.Cleared || b.Ending != game.EndingWake || b.Unlocked != 0 || b.Top {
		t.Errorf("debrief %+v", b)
	}
	if err := h.try(game.ChooseRoute{Index: 0}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("a woken boot played on: %v", err)
	}
}
