package game_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// noRisk is Standard() with its easing on but no door modifiers and no
// boot #1 rules, so a test can follow a door's timer and the lives.
func noRisk() rules.Rules {
	r := noFirst(rules.Standard())
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

// A cold-start player is aimed at 576 in Userland, under a pool whose
// easiest doors are 800s (rated by no source with a censored floor): the
// engine forecasts a crash there on its own numbers, and eases the boot
// with D34's lives (two, as the forecast is over 0.4), a duck and longer
// timers on the floor doors. A player the pool can reach gets
// none of it.
func TestFloorEase(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	// Every lever on, D34's second life, the duck and the buy-back too.
	rr := noRisk()
	rr.EaseLives, rr.EaseDucks, rr.BuybackTimers = 2, 1, []float64{1.25, 1.5, 2}
	h := newHarness(t, rr, nil, ps, 0, 1)
	s, r := h.s, h.s.Rules
	es, ok := easeOf(s, game.EaseFloor)
	// Userland's first window holds the 800 and 820 doors of every topic.
	door := (game.Expected(1000, 800) + game.Expected(1000, 820)) / 2
	forecast := 1 - door*door
	if !ok || !es.Buyback || es.Cycles != 2 || es.Ducks != r.EaseDucks || math.Abs(s.Forecast-forecast) > 1e-9 {
		t.Fatalf("easing %+v, forecast %.3f; want the floor's, forecast %.3f, 2 lives and a duck", s.Ease, s.Forecast, forecast)
	}
	if !strings.Contains(es.Why, "Userland") || !strings.Contains(es.Why, "800") {
		t.Errorf("why %q names neither the biome nor the floor", es.Why)
	}
	if s.Cycles != r.Cycles+2 || s.MaxCycles < s.Cycles || s.Ducks != r.Ducks+r.EaseDucks {
		t.Errorf("lives %d/%d ducks %d, want %d and %d", s.Cycles, s.MaxCycles, s.Ducks, r.Cycles+2, r.Ducks+r.EaseDucks)
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
	if len(strong.s.Ease) != 0 || strong.s.Cycles != r.Cycles {
		t.Errorf("1600: easings %+v, lives %d", strong.s.Ease, strong.s.Cycles)
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
	if h := newHarness(t, off, nil, ps, 0, 1); len(h.s.Ease) != 0 || h.s.Cycles != off.Cycles {
		t.Errorf("easing off: %+v, lives %d", h.s.Ease, h.s.Cycles)
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
	lives := h.s.Cycles
	for n := 1; h.s.Phase != game.PhaseCrashed; n++ {
		d := h.open(0)
		rated := h.m.Rated
		h.now = h.s.Current.Deadline
		h.do(game.Tick{})
		if h.s.Phase == game.PhaseCrashed {
			// The last life is no spare one: that timeout ends the boot.
			if n != lives || h.s.Cycles != 1 || h.s.Crash.Cause != game.TimedOut {
				t.Errorf("crashed on door %d of %d lives, %d left (%+v)", n, lives, h.s.Cycles, h.s.Crash)
			}
			break
		}
		a := h.s.History[len(h.s.History)-1]
		if h.s.Phase != game.PhaseDoors || h.s.Cycles != lives-n || a.Outcome != game.TimedOut || h.m.Rated != rated+1 ||
			h.s.Room != 0 || slices.ContainsFunc(h.s.Doors, func(o game.Door) bool { return o.Problem == d.Problem }) {
			t.Fatalf("door %d: phase %s lives %d room %d, attempt %+v, rated %d", n, h.s.Phase, h.s.Cycles, h.s.Room, a, h.m.Rated)
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
	if !h.s.Declined || h.s.Cycles != r.Cycles || h.s.Ducks != r.Ducks || len(h.s.Ease) == 0 {
		t.Fatalf("declined %v lives %d ducks %d easings %+v", h.s.Declined, h.s.Cycles, h.s.Ducks, h.s.Ease)
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
	if !ok || es.Cycles != r.GovernorLives || !slices.Equal(es.Clear, []float64{0.95, 0.78, 0.55}) ||
		es.Why != "the last 2 boots ended in Userland" {
		t.Fatalf("easing %+v", h.s.Ease)
	}
	if h.s.Cycles != r.Cycles+r.GovernorLives || h.s.AimClear(0) != 0.95 || h.s.AimClear(1) != 0.78 {
		t.Errorf("lives %d, aims %.2f %.2f", h.s.Cycles, h.s.AimClear(0), h.s.AimClear(1))
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
	if h.s.AimClear(0) != r.Biomes[0].TargetClear || h.s.Cycles != r.Cycles {
		t.Errorf("declined governor: aim %.2f lives %d", h.s.AimClear(0), h.s.Cycles)
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

// cfPool is dense's pool rated by Codeforces, which rates nothing under
// 800: its 800s are the floor.
func cfPool(lo, hi, step int) []catalog.Problem {
	ps := dense(catalog.Topics, lo, hi, step)
	for i := range ps {
		ps[i].RatingSource = "cf"
	}
	return ps
}

// A Codeforces 800 is read as easier than 800 (rules.FloorDrop): for a
// cold-start 1000 player Userland's aim (576) meets the floor's mean, so
// its doors are the floor's, on target and teaching the calibration, at
// the censored clear chance.
func TestFloorReading(t *testing.T) {
	t.Parallel()
	ps := cfPool(800, 2600, 20)
	r := calm(noRisk())
	h := newHarness(t, r, nil, ps, 0, 1)
	for _, d := range h.s.Doors {
		if !d.Floor || d.Rating != 800 || d.Tier == "" || d.Difficulty(&r) != 800-r.FloorDrop {
			t.Errorf("door %+v: want a floor door on target", d)
		}
		if lo, hi := game.Expected(1000, 800), game.Expected(1000, 800-r.FloorDrop); d.SolveP <= lo || d.SolveP >= hi {
			t.Errorf("door %+v: SolveP %.3f outside %.3f..%.3f", d, d.SolveP, lo, hi)
		}
	}
	// The same pool rated by anyone else is read at its ratings.
	h = newHarness(t, r, nil, dense(catalog.Topics, 800, 2600, 20), 0, 1)
	for _, d := range h.s.Doors {
		if d.Floor || d.Tier != "" || d.SolveP != game.Expected(1000, float64(d.Rating)) {
			t.Errorf("door %+v read as a floor door", d)
		}
	}
}

// A player back after rules.ComebackDays or more aims the first biome at
// its EaseClear and says why in days (RT8); a shorter gap, a first boot
// or a decline leaves the aim alone. A save from before Meta.LastPlayed
// counts from its latest solve.
func TestComeback(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 400, 3000, 10)
	r := noRisk()
	day := 24 * time.Hour
	for _, tc := range []struct {
		name string
		meta game.Meta
		days int // 0: no comeback
	}{
		{"four days away", game.Meta{Boots: 5, Rating: 1600, LastPlayed: t0.Add(-4*day - time.Hour)}, 4},
		{"two days away", game.Meta{Boots: 5, Rating: 1600, LastPlayed: t0.Add(-2 * day)}, 0},
		{"first boot", game.Meta{Rating: 1600, LastPlayed: t0.Add(-9 * day)}, 0},
		{"old save", game.Meta{Boots: 5, Rating: 1600, Solves: []game.Solve{{Problem: "x", At: t0.Add(-6 * day)}}}, 6},
	} {
		m := tc.meta
		// What GRUB says before the boot is what the boot then does.
		if got := game.Comeback(&r, &m, t0); got != tc.days {
			t.Errorf("%s: Comeback %d, want %d", tc.name, got, tc.days)
		}
		h := newHarness(t, r, &m, ps, 0, 1)
		es, ok := easeOf(h.s, game.EaseComeback)
		if ok != (tc.days > 0) {
			t.Errorf("%s: easings %+v", tc.name, h.s.Ease)
			continue
		}
		if !m.LastPlayed.Equal(t0) {
			t.Errorf("%s: last played %v, want the boot's start", tc.name, m.LastPlayed)
		}
		if !ok {
			if h.s.AimClear(0) != r.Biomes[0].TargetClear {
				t.Errorf("%s: aim %.2f", tc.name, h.s.AimClear(0))
			}
			continue
		}
		if es.Why != fmt.Sprintf("first boot in %d days", tc.days) || h.s.AimClear(0) != r.Biomes[0].EaseClear ||
			h.s.AimClear(1) != r.Biomes[1].TargetClear || es.Cycles != 0 {
			t.Errorf("%s: easing %+v, aims %.2f %.2f", tc.name, es, h.s.AimClear(0), h.s.AimClear(1))
		}
		h.do(game.DeclineEase{})
		if h.s.AimClear(0) != r.Biomes[0].TargetClear {
			t.Errorf("%s: declined, aim %.2f", tc.name, h.s.AimClear(0))
		}
	}
	// Upsolving and ending a boot both count as playing.
	m := game.Meta{Boots: 5, Rating: 1600, LastPlayed: t0.Add(-9 * day)}
	game.Upsolve(&m, "x", game.Independent, time.Hour, t0.Add(-day))
	if !m.LastPlayed.Equal(t0.Add(-day)) {
		t.Errorf("upsolve: last played %v", m.LastPlayed)
	}
	h := newHarness(t, r, &m, ps, 0, 1)
	h.open(0)
	h.do(game.GiveUp{})
	if !m.LastPlayed.Equal(h.now) {
		t.Errorf("crash: last played %v, want %v", m.LastPlayed, h.now)
	}
}

// A new player's first boot (RT11) plays the short shape, its first room
// offers the pool's easiest doors with tags shown and no risk modifier,
// every door it offers has an editorial, and all of it is said as boot
// #1's; the next room and the next boot are as usual.
func TestFirstBoot(t *testing.T) {
	t.Parallel()
	ps := cfPool(800, 2600, 20)
	for i := range ps {
		ps[i].HasEditorial = i%3 != 0
	}
	r := rules.Standard()
	r.BlindChance, r.MysteryChance, r.CursedChance = 1, 0.5, 1 // every door would be risky
	r.MasteryAttempts = 0
	m := &game.Meta{}
	h := newHarness(t, r, m, ps, 0, 1)
	s := h.s
	es, ok := easeOf(s, game.EaseFirst)
	if !ok || !es.FirstDoor || !es.Editorial || es.Why != "boot #1" {
		t.Fatalf("easings %+v", s.Ease)
	}
	if mustJSON(t, s.Rules.Biomes) != mustJSON(t, rules.Short().Biomes) {
		t.Errorf("boot #1 plays %+v, want the short shape", s.Rules.Biomes)
	}
	editorial := map[catalog.ID]bool{}
	for _, p := range ps {
		editorial[p.ID] = p.HasEditorial
	}
	for _, d := range s.Doors {
		if d.Blind || d.Mystery || d.Cursed || d.Mastered || d.Rating != 800 || !editorial[d.Problem] {
			t.Errorf("first door %+v: want a plain floor door with tags and an editorial", d)
		}
	}
	h.clearDoor(0, time.Minute)
	h.pick()
	risky := 0
	for _, d := range h.s.Doors {
		if !editorial[d.Problem] {
			t.Errorf("boot #1 door %+v has no editorial", d)
		}
		if d.Cursed || d.Blind {
			risky++
		}
	}
	if risky < len(h.s.Doors)-1 {
		t.Errorf("second room %+v: only the safe door should be plain", h.s.Doors)
	}
	h2 := newHarness(t, r, m, ps, 0, 2)
	if _, ok := easeOf(h2.s, game.EaseFirst); ok || len(h2.s.Rules.Biomes[1:2]) != 1 || h2.s.Rules.Biomes[0].Timer.Duration != 15*time.Minute {
		t.Errorf("boot #2: easings %+v, biomes %+v", h2.s.Ease, h2.s.Rules.Biomes)
	}
}

// A trial takes a boot number, but a new player who opens RTFM before
// ever booting still plays boot #1's rules (RT11) on their first boot,
// days later or not, and only on it.
func TestFirstBootAfterTrial(t *testing.T) {
	t.Parallel()
	m := &game.Meta{}
	pool := trialPool()
	if _, _, err := game.NewTrial(rules.Trial(), m, pool, silverTrial(), 1, t0); err != nil {
		t.Fatal(err)
	}
	later := t0.Add(5 * 24 * time.Hour)
	s, _, err := game.NewBoot(rules.Standard(), m, pool, 0, 2, later)
	if err != nil {
		t.Fatal(err)
	}
	_, first := easeOf(s, game.EaseFirst)
	_, back := easeOf(s, game.EaseComeback)
	if !first || back || mustJSON(t, s.Rules.Biomes) != mustJSON(t, rules.Short().Biomes) {
		t.Errorf("first boot after a trial: easings %+v, biomes %+v", s.Ease, s.Rules.Biomes)
	}
	s, _, err = game.NewBoot(rules.Standard(), m, pool, 0, 3, later)
	if _, again := easeOf(s, game.EaseFirst); err != nil || again {
		t.Errorf("second boot: easings %+v, %v", s.Ease, err)
	}
}
