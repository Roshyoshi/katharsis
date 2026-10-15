package game_test

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// noRisk is Standard() with its easing on but no door modifiers, so a test
// can follow a door's timer and the lives.
func noRisk() rules.Rules {
	r := rules.Standard()
	r.BlindChance, r.MysteryChance, r.CursedChance = 0, 0, 0
	r.Ascensions = nil
	return r
}

func easeOf(s *game.State, k game.EaseKind) (game.Ease, bool) {
	i := slices.IndexFunc(s.Ease, func(es game.Ease) bool { return es.Kind == k })
	if i < 0 {
		return game.Ease{}, false
	}
	return s.Ease[i], true
}

// A cold-start player is aimed at 576 in Userland, under the pool's 800
// floor: the engine forecasts a crash there on its own numbers, and eases
// the boot with D34's lives (two, as the forecast is over 0.4), a duck
// and longer timers on the floor doors. A player the pool can reach gets
// none of it.
func TestFloorEase(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, noRisk(), nil, ps, 0, 1)
	s, r := h.s, h.s.Rules
	es, ok := easeOf(s, game.EaseFloor)
	// Userland's first window holds the 800 and 820 doors of every topic.
	door := (game.Expected(1000, 800) + game.Expected(1000, 820)) / 2
	forecast := 1 - door*door
	if !ok || !es.Buyback || es.Lives != 2 || es.Ducks != r.EaseDucks || math.Abs(s.Forecast-forecast) > 1e-9 {
		t.Fatalf("easing %+v, forecast %.3f; want the floor's, forecast %.3f, 2 lives and a duck", s.Ease, s.Forecast, forecast)
	}
	if !strings.Contains(es.Why, "Userland") || !strings.Contains(es.Why, "800") {
		t.Errorf("why %q names neither the biome nor the floor", es.Why)
	}
	if s.Lives != r.Lives+2 || s.MaxLives < s.Lives || s.Ducks != r.Ducks+r.EaseDucks {
		t.Errorf("lives %d/%d ducks %d, want %d and %d", s.Lives, s.MaxLives, s.Ducks, r.Lives+2, r.Ducks+r.EaseDucks)
	}
	for _, d := range s.Doors {
		k := d.TimerBuy
		if !slices.Contains(r.BuybackTimers, k) || d.Timer != (time.Duration(float64(15*time.Minute)*k)).Round(time.Minute) || d.Tier != "" {
			t.Errorf("floor door %+v: want a buy-back multiplier on the 15m timer and no tier", d)
		}
	}
	if !strings.Contains(s.Log[len(s.Log)-1].Text, "room_0.0") {
		t.Errorf("dmesg %v", s.Log)
	}

	strong := newHarness(t, noRisk(), &game.Meta{Rating: 1600}, ps, 0, 1)
	if len(strong.s.Ease) != 0 || strong.s.Lives != r.Lives {
		t.Errorf("1600: easings %+v, lives %d", strong.s.Ease, strong.s.Lives)
	}
	for _, d := range strong.s.Doors {
		if d.TimerBuy != 0 {
			t.Errorf("1600: door %+v bought back", d)
		}
	}
	if f := strong.s.Forecast; f < 0.1 || f > 0.2 {
		t.Errorf("1600: forecast %.3f, want near 1-0.92^2", f)
	}

	off := noRisk()
	off.EaseAbove, off.BuybackTimers = 0, nil
	if h := newHarness(t, off, nil, ps, 0, 1); len(h.s.Ease) != 0 || h.s.Lives != off.Lives {
		t.Errorf("easing off: %+v, lives %d", h.s.Ease, h.s.Lives)
	}
}

// A timed-out Userland door costs a life, not the boot (D44), eased or
// not: it closes as timed out, is rated and comes back as a regression,
// and the room is drawn again. On the last life the timeout crashes, and
// past Userland a timeout crashes as always.
func TestTimeoutLife(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 400, 3000, 10)
	h := newHarness(t, noRisk(), &game.Meta{Rating: 1600}, ps, 0, 1)
	if len(h.s.Ease) != 0 || !h.s.TimeoutCostsLife() {
		t.Fatalf("easings %+v, timeout costs a life %v", h.s.Ease, h.s.TimeoutCostsLife())
	}
	lives := h.s.Lives
	for n := 1; h.s.Phase != game.PhaseCrashed; n++ {
		d := h.open(0)
		rated := h.m.Rated
		h.now = h.s.Current.Deadline
		h.do(game.Tick{})
		if h.s.Phase == game.PhaseCrashed {
			// The last life is no spare one: that timeout ends the boot.
			if n != lives || h.s.Lives != 1 || h.s.Crash.Cause != game.TimedOut {
				t.Errorf("crashed on door %d of %d lives, %d left (%+v)", n, lives, h.s.Lives, h.s.Crash)
			}
			break
		}
		a := h.s.History[len(h.s.History)-1]
		if h.s.Phase != game.PhaseDoors || h.s.Lives != lives-n || a.Outcome != game.TimedOut || h.m.Rated != rated+1 ||
			h.s.Room != 0 || slices.ContainsFunc(h.s.Doors, func(o game.Door) bool { return o.Problem == d.Problem }) {
			t.Fatalf("door %d: phase %s lives %d room %d, attempt %+v, rated %d", n, h.s.Phase, h.s.Lives, h.s.Room, a, h.m.Rated)
		}
		if !slices.ContainsFunc(h.m.Regressions, func(g game.Regression) bool { return g.Problem == d.Problem }) {
			t.Errorf("door %d: %s not scheduled to come back", n, d.Problem)
		}
	}

	h = newHarness(t, noRisk(), &game.Meta{Rating: 1600}, ps, 0, 2)
	h.toMerge()
	h.pick()
	h.do(game.LeaveMerge{})
	if h.s.Phase == game.PhaseRoute {
		h.do(game.ChooseRoute{})
	}
	if h.s.TimeoutCostsLife() {
		t.Error("a mid biome timeout costs a life")
	}
	h.open(0)
	h.now = h.s.Current.Deadline.Add(time.Minute)
	h.do(game.SubmitResult{Verdict: "AC"}) // a verdict too late
	if h.s.Phase != game.PhaseCrashed || h.s.Crash.Cause != game.TimedOut {
		t.Errorf("mid biome timeout: phase %s", h.s.Phase)
	}
}

// The player may decline the easing until the first door opens: the lives
// and the duck it added go, and the room is drawn again without its
// timers. Userland's timeout rule is no easing and stays.
func TestDeclineEase(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	r := noRisk()
	h := newHarness(t, r, nil, ps, 0, 1)
	if len(h.s.Ease) == 0 {
		t.Fatal("no easing to decline")
	}
	fx := h.do(game.DeclineEase{})
	if !h.s.Declined || h.s.Lives != r.Lives || h.s.Ducks != r.Ducks || len(h.s.Ease) == 0 {
		t.Fatalf("declined %v lives %d ducks %d easings %+v", h.s.Declined, h.s.Lives, h.s.Ducks, h.s.Ease)
	}
	if _, ok := has[game.Prefetch](fx); !ok {
		t.Errorf("effects %v: the redrawn room is not fetched", fx)
	}
	for _, d := range h.s.Doors {
		if d.TimerBuy != 0 || d.Timer != 15*time.Minute {
			t.Errorf("door %+v after the decline", d)
		}
	}
	if err := h.try(game.DeclineEase{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("second decline: %v", err)
	}
	if !h.s.TimeoutCostsLife() {
		t.Error("declining took Userland's timeout rule")
	}

	// Not after the first door, and not with nothing to decline.
	h = newHarness(t, r, nil, ps, 0, 1)
	h.clearDoor(0, time.Minute)
	if err := h.try(game.DeclineEase{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("decline after a door: %v", err)
	}
	h = newHarness(t, r, &game.Meta{Rating: 1600}, ps, 0, 1)
	if err := h.try(game.DeclineEase{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("decline of nothing: %v", err)
	}
}

// Two boots in a row that crash in Userland make the next aim every biome
// at its EaseClear, with a life; reaching a merge window starts the count
// over, and a clear counts toward the full clears.
func TestGovernor(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 400, 3000, 10)
	r := noRisk()
	m := &game.Meta{Rating: 1600}
	for want := 1; want <= 2; want++ {
		h := newHarness(t, r, m, ps, 0, uint64(want))
		if len(h.s.Ease) != 0 {
			t.Fatalf("boot %d eased: %+v", want, h.s.Ease)
		}
		h.open(0)
		h.do(game.GiveUp{})
		if m.Governor.EarlyCrashes != want {
			t.Fatalf("governor %+v after %d early crashes", m.Governor, want)
		}
	}
	h := newHarness(t, r, m, ps, 0, 3)
	es, ok := easeOf(h.s, game.EaseGovernor)
	if !ok || es.Lives != r.GovernorLives || !slices.Equal(es.Clear, []float64{0.95, 0.78, 0.55}) ||
		es.Why != "the last 2 boots ended in Userland" {
		t.Fatalf("easing %+v", h.s.Ease)
	}
	if h.s.Lives != r.Lives+r.GovernorLives || h.s.AimClear(0) != 0.95 || h.s.AimClear(1) != 0.78 {
		t.Errorf("lives %d, aims %.2f %.2f", h.s.Lives, h.s.AimClear(0), h.s.AimClear(1))
	}
	want := game.Aim(m.Rating, 0.95)
	if got := h.s.Target(m, "dp"); math.Abs(got-want) > 1e-9 {
		t.Errorf("Target %.1f, want %.1f", got, want)
	}
	h.toMerge()
	if m.Governor.EarlyCrashes != 0 {
		t.Errorf("governor %+v at a merge window", m.Governor)
	}
	// Declined, the aim is the biome's own.
	m2 := &game.Meta{Rating: 1600, Governor: game.Governor{EarlyCrashes: 2}}
	h = newHarness(t, r, m2, ps, 0, 4)
	h.do(game.DeclineEase{})
	if h.s.AimClear(0) != r.Biomes[0].TargetClear || h.s.Lives != r.Lives {
		t.Errorf("declined governor: aim %.2f lives %d", h.s.AimClear(0), h.s.Lives)
	}

	// A full clear counts, and starts the early crashes over.
	m3 := &game.Meta{Rating: 1600, Governor: game.Governor{EarlyCrashes: 1, FullClears: 2}}
	h = newHarness(t, r, m3, ps, 0, 5)
	for h.s.Phase != game.PhaseCleared {
		switch h.s.Phase {
		case game.PhaseMerge:
			h.pick()
			h.do(game.LeaveMerge{})
		case game.PhaseRoute:
			h.do(game.ChooseRoute{})
		default:
			h.clearDoor(0, time.Minute)
		}
	}
	if m3.Governor != (game.Governor{FullClears: 3}) {
		t.Errorf("governor %+v after a clear", m3.Governor)
	}
}

// Userland's easiest door rolls no risk modifier (Biome.Plain), even when
// every door would be cursed; the mid biome has no such door.
func TestPlainDoors(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 400, 3000, 10)
	r := noRisk()
	r.CursedChance = 1
	h := newHarness(t, r, &game.Meta{Rating: 1600}, ps, 0, 1)
	var plain []game.Door
	best := 0.0
	for _, d := range h.s.Doors {
		best = max(best, d.SolveP)
		if !d.Cursed {
			plain = append(plain, d)
		}
	}
	if len(plain) != 1 || plain[0].SolveP != best || plain[0].Bytes != int64(math.Round(float64(r.BaseBytes)*0.5/best)) {
		t.Fatalf("doors %+v: want one plain door, the easiest, paid as plain", h.s.Doors)
	}
	h.toMerge()
	h.pick()
	h.do(game.LeaveMerge{})
	if h.s.Phase == game.PhaseRoute {
		h.do(game.ChooseRoute{})
	}
	for _, d := range h.s.Doors {
		if !d.Cursed && !d.Regression {
			t.Errorf("mid biome door %+v not cursed", d)
		}
	}
}

// Every easing's reason is in task terms: it names the doors or boots
// behind it and never calls the boot easy (retention RT8, GR9).
func TestEaseCopy(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	m := &game.Meta{Governor: game.Governor{EarlyCrashes: 3}}
	h := newHarness(t, noRisk(), m, ps, 0, 1)
	if len(h.s.Ease) != 2 {
		t.Fatalf("easings %+v, want the governor's and the floor's", h.s.Ease)
	}
	for _, es := range h.s.Ease {
		why := strings.ToLower(es.Why)
		if !strings.Contains(why, "door") && !strings.Contains(why, "boot") && !strings.Contains(why, "day") {
			t.Errorf("%s: %q names no doors, boots or days", es.Kind, es.Why)
		}
		for _, bad := range []string{"easy", "assist", "mercy", "you"} {
			if strings.Contains(why, bad) {
				t.Errorf("%s: %q says %q", es.Kind, es.Why, bad)
			}
		}
	}
}
