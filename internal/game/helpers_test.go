package game_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// plain is Standard() without randomness in the doors or ascensions, and
// without easing, so a test can predict every door's modifiers and timer.
func plain() rules.Rules {
	r := calm(rules.Standard())
	r.BlindChance, r.MysteryChance, r.CursedChance = 0, 0, 0
	r.Ascensions = nil
	return r
}

// calm turns r's easing off (the floor easing, the governor, the
// comeback), boot #1's rules, its safe doors (Biome.Plain) and Userland's
// timeout rule (Biome.TimeoutLife): most tests start a new player (boot
// #1) on a pool whose floor sits above its Userland aim, which the floor
// easing answers with lives and longer timers, some roll every door
// cursed, and some crash on a timeout.
func calm(r rules.Rules) rules.Rules {
	r = noFirst(r)
	r.EaseAbove, r.BuybackTimers, r.GovernorAfter, r.ComebackDays = 0, nil, 0, 0
	r.Biomes = slices.Clone(r.Biomes)
	for i := range r.Biomes {
		r.Biomes[i].Plain, r.Biomes[i].TimeoutLife = 0, false
	}
	return r
}

// dense has a problem every step rating points in lo..hi for each topic.
func dense(topics []string, lo, hi, step int) []catalog.Problem {
	var ps []catalog.Problem
	for _, t := range topics {
		for r := lo; r <= hi; r += step {
			ps = append(ps, catalog.Problem{
				ID:        catalog.ID(fmt.Sprintf("t:%s:%d", t, r)),
				Title:     fmt.Sprintf("%s %d", t, r),
				Topic:     t,
				Rating:    r,
				Judgeable: true,
			})
		}
	}
	return ps
}

// harness drives one boot with a clock that moves a minute per action.
type harness struct {
	t   *testing.T
	s   *game.State
	m   *game.Meta
	ps  []catalog.Problem
	now time.Time
	fx  []game.Effect
}

func newHarness(t *testing.T, r rules.Rules, m *game.Meta, ps []catalog.Problem, prestige int, seed uint64) *harness {
	t.Helper()
	if m == nil {
		m = &game.Meta{}
	}
	s, fx, err := game.NewBoot(r, m, ps, prestige, seed, t0)
	if err != nil {
		t.Fatalf("NewBoot: %v", err)
	}
	return &harness{t: t, s: s, m: m, ps: ps, now: t0, fx: fx}
}

func (h *harness) try(a game.Action) error {
	h.now = h.now.Add(time.Minute)
	fx, err := game.Step(h.s, h.m, h.ps, a, h.now)
	h.fx = fx
	return err
}

func (h *harness) do(a game.Action) []game.Effect {
	h.t.Helper()
	if err := h.try(a); err != nil {
		h.t.Fatalf("Step(%T%+v) in %s: %v", a, a, h.s.Phase, err)
	}
	return h.fx
}

// pick takes every owed upgrade, first offer first.
func (h *harness) pick() {
	h.t.Helper()
	for h.s.PendingUpgrade {
		h.do(game.PickUpgrade{Upgrade: h.s.UpgradeOffers[0]})
	}
}

// clearDoor opens door i and ACs it after d.
func (h *harness) clearDoor(i int, d time.Duration) {
	h.t.Helper()
	h.open(i)
	h.now = h.now.Add(d)
	h.do(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
}

// toMerge clears every room of the current biome.
func (h *harness) toMerge() {
	h.t.Helper()
	b := h.s.Biome
	for h.s.Biome == b && h.s.Phase == game.PhaseDoors {
		h.clearDoor(0, 10*time.Minute)
	}
	if h.s.Phase != game.PhaseMerge {
		h.t.Fatalf("phase %s after clearing biome %d, want merge", h.s.Phase, b)
	}
}

// open opens door i (after taking any owed pick).
func (h *harness) open(i int) game.Door {
	h.t.Helper()
	h.pick()
	h.do(game.ChooseDoor{Index: i})
	return h.s.Current.Door
}

func has[T game.Effect](fx []game.Effect) (T, bool) {
	for _, f := range fx {
		if v, ok := f.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func sounds(fx []game.Effect) []string {
	var s []string
	for _, f := range fx {
		if v, ok := f.(game.Sound); ok {
			s = append(s, v.Name)
		}
	}
	return s
}

func mustJSON(t testing.TB, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// noFirst turns boot #1's rules off (rules.FirstBiomes, FirstDoor,
// FirstEditorial), so a new player's first boot plays like any other.
func noFirst(r rules.Rules) rules.Rules {
	r.FirstBiomes, r.FirstDoor, r.FirstEditorial = nil, false, false
	return r
}

// cyclesLinear is the cycles meta (rules.Map's prices, ladder and twists)
// on the short shape, off the map: Userland, one mid biome and the boss.
func cyclesLinear() rules.Rules {
	r := rules.Map()
	r.Name = "cycles"
	r.Biomes, r.Nodes = rules.ShortBiomes(), nil
	return r
}

// presets are the rules the property tests play: a boot saved before
// cycles, the cycles meta on a linear shape, and the map.
func presets() []rules.Rules {
	m := rules.Map()
	m.Name = "map"
	return []rules.Rules{rules.Legacy(), cyclesLinear(), m}
}
