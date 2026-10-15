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

func TestNewBootLayout(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, calm(rules.Standard()), nil, ps, 0, 1)
	s, m := h.s, h.m

	if m.Boots != 1 || s.Boot != 1 || m.Rating != 1000 {
		t.Errorf("Boots %d, Boot %d, Rating %v; want 1, 1, cold start 1000", m.Boots, s.Boot, m.Rating)
	}
	if len(s.Biomes) != 3 {
		t.Fatalf("%d biomes, want 3", len(s.Biomes))
	}
	b0, b1, b2 := s.Biomes[0], s.Biomes[1], s.Biomes[2]
	if b0.Kind != rules.Fixed || b0.Topic != game.Mixed || b0.Name != "Userland" || b0.Rooms != 2 {
		t.Errorf("biome 0 = %+v, want fixed mixed Userland with 2 rooms", b0)
	}
	if b1.Kind != rules.Choice || b1.Topic != "" || len(b1.Offers) != 2 {
		t.Errorf("biome 1 = %+v, want an unchosen choice of 2", b1)
	}
	for _, o := range b1.Offers {
		if game.CapabilityFor(o) != "" {
			t.Errorf("offer %q needs %s, which a new player lacks", o, game.CapabilityFor(o))
		}
	}
	if b2.Kind != rules.Boss || b2.Rooms != 1 || !slices.Contains(game.BossKeys, b2.Boss) {
		t.Errorf("biome 2 = %+v, want a one-room boss from BossKeys", b2)
	}
	if s.Phase != game.PhaseDoors || len(s.Doors) != 3 {
		t.Fatalf("phase %s with %d doors, want doors with 3", s.Phase, len(s.Doors))
	}
	if s.Cycles != 3 || s.MaxCycles != 5 || s.Ducks != 1 {
		t.Errorf("lives %d/%d ducks %d, want 3/5 and 1", s.Cycles, s.MaxCycles, s.Ducks)
	}
	pf, ok := has[game.Prefetch](h.fx)
	if !ok || len(pf.Problems) != 3 {
		t.Fatalf("effects %v, want a Prefetch of the 3 doors", h.fx)
	}
	for i, d := range s.Doors {
		if pf.Problems[i] != d.Problem {
			t.Errorf("prefetch %d = %s, door is %s", i, pf.Problems[i], d.Problem)
		}
	}
	if len(s.Log) == 0 {
		t.Error("no dmesg lines")
	}
}

func TestNewBootErrors(t *testing.T) {
	t.Parallel()
	ps := dense([]string{"simulation"}, 800, 1600, 50)
	unjudgeable := slices.Clone(ps)
	for i := range unjudgeable {
		unjudgeable[i].Judgeable = false
	}
	broken := rules.Standard()
	broken.Cycles = 0
	tests := []struct {
		name     string
		r        rules.Rules
		pool     []catalog.Problem
		unlocked int
		prestige int
	}{
		{"locked prestige", rules.Standard(), ps, 0, 1},
		{"negative prestige", rules.Standard(), ps, 0, -1},
		{"past max prestige", rules.Standard(), ps, 4, 5},
		{"empty pool", rules.Standard(), nil, 0, 0},
		{"nothing judgeable", rules.Standard(), unjudgeable, 0, 0},
		{"invalid rules", broken, ps, 0, 0},
	}
	for _, tc := range tests {
		m := &game.Meta{Prestige: tc.unlocked}
		before := mustJSON(t, m)
		if _, _, err := game.NewBoot(tc.r, m, tc.pool, tc.prestige, 1, t0); err == nil {
			t.Errorf("%s: NewBoot succeeded", tc.name)
		}
		if got := mustJSON(t, m); got != before {
			t.Errorf("%s: meta changed on error:\n%s\n%s", tc.name, before, got)
		}
	}
}

func TestBossKeys(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 50)
	// A linear boot meets The Halting Problem at the top of its rules'
	// ladder (P4 for the legacy rules), and from there to MaxPrestige.
	top := rules.Legacy().TopPrestige(game.MaxPrestige)
	for p := 0; p <= game.MaxPrestige; p++ {
		h := newHarness(t, rules.Legacy(), &game.Meta{Prestige: p}, ps, p, uint64(p))
		got := h.s.Biomes[2].Boss
		if p >= top && got != game.FinalBoss {
			t.Errorf("prestige %d boss %q, want %q", p, got, game.FinalBoss)
		}
		if p < top && !slices.Contains(game.BossKeys, got) {
			t.Errorf("prestige %d boss %q, want one of BossKeys", p, got)
		}
	}

	// Two boss biomes get different named bugs.
	r := plain()
	r.Biomes = []rules.Biome{
		{Kind: rules.Boss, Rooms: 1, TargetClear: 0.5, Timer: rules.Duration{Duration: 50 * time.Minute}},
		{Kind: rules.Boss, Rooms: 1, TargetClear: 0.5, Timer: rules.Duration{Duration: 50 * time.Minute}},
	}
	r.CursedChance = 1
	h := newHarness(t, r, nil, ps, 0, 3)
	if a, b := h.s.Biomes[0].Boss, h.s.Biomes[1].Boss; a == b || a == "" {
		t.Errorf("boss keys %q and %q, want two distinct", a, b)
	}
	if len(h.s.Doors) != 1 || h.s.Doors[0].Boss != h.s.Biomes[0].Boss || h.s.Doors[0].Cursed {
		t.Errorf("boss room doors %+v, want one uncursed door of the biome's boss", h.s.Doors)
	}
	if !slices.Contains(sounds(h.fx), "boss") {
		t.Errorf("sounds %v, want boss on entering the boss room", sounds(h.fx))
	}
}

// routeFirst is a boot whose first biome is a route choice.
func routeFirst() rules.Rules {
	r := plain()
	r.Biomes = []rules.Biome{
		{Kind: rules.Choice, Rooms: 1, Choices: 2, TargetClear: 0.7, Timer: rules.Duration{Duration: 30 * time.Minute}},
		{Kind: rules.Boss, Rooms: 1, TargetClear: 0.5, Timer: rules.Duration{Duration: 50 * time.Minute}},
	}
	return r
}

func TestRouteOffers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		caps []string
		pool []catalog.Problem
		ok   func([]string) bool
	}{
		{"new player gets un-gated topics", nil, dense(catalog.Topics, 800, 2600, 20), func(o []string) bool {
			return len(o) == 2 && game.CapabilityFor(o[0]) == "" && game.CapabilityFor(o[1]) == ""
		}},
		{"capability unlocks its topic", []string{"CAP_DP"}, dense([]string{"dp", "graphs"}, 800, 2600, 20), func(o []string) bool {
			return slices.Equal(o, []string{"dp"})
		}},
		{"nothing unlocked falls back to mixed", nil, dense([]string{"dp", "graphs"}, 800, 2600, 20), func(o []string) bool {
			return slices.Equal(o, []string{game.Mixed})
		}},
		{"too few problems near the player", nil, dense([]string{"simulation"}, 2400, 2600, 10), func(o []string) bool {
			return slices.Equal(o, []string{game.Mixed})
		}},
	}
	for _, tc := range tests {
		h := newHarness(t, routeFirst(), &game.Meta{Capabilities: tc.caps}, tc.pool, 0, 5)
		if h.s.Phase != game.PhaseRoute {
			t.Fatalf("%s: phase %s, want route", tc.name, h.s.Phase)
		}
		offers := h.s.Biomes[0].Offers
		if !tc.ok(offers) {
			t.Errorf("%s: offers %v", tc.name, offers)
		}
		h.do(game.ChooseRoute{Index: 0})
		if h.s.Phase != game.PhaseDoors || h.s.Biomes[0].Topic != offers[0] || len(h.s.Doors) == 0 {
			t.Errorf("%s: after the route: phase %s topic %q doors %d", tc.name, h.s.Phase, h.s.Biomes[0].Topic, len(h.s.Doors))
		}
		for _, d := range h.s.Doors {
			if offers[0] != game.Mixed && d.Topic != offers[0] {
				t.Errorf("%s: door topic %q in a %q biome", tc.name, d.Topic, offers[0])
			}
		}
	}
}

func TestDoorTargets(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 400, 3000, 10)
	r := plain()
	tier := game.CalibrationTier(r.Biomes[0])
	cal := func(all, own float64) map[string]game.Calibration {
		return map[string]game.Calibration{game.CalibrationAll: {Offset: all}, tier: {Offset: own}}
	}
	tests := []struct {
		rating   float64
		prestige int
		cal      map[string]game.Calibration
		offset   float64 // the calibration the doors should carry
	}{
		{1500, 0, nil, 0}, {1500, 2, nil, 0}, {2000, 0, nil, 0}, {1000, 4, nil, 0},
		{1500, 0, cal(80, -30), 50},
		{1500, 2, cal(-60, 0), -60},
		{1500, 0, cal(400, 100), r.CalibMax},                             // clamped
		{1500, 0, map[string]game.Calibration{"0.50": {Offset: 200}}, 0}, // another tier's
	}
	for _, tc := range tests {
		for seed := range uint64(5) {
			m := &game.Meta{Rating: tc.rating, Prestige: tc.prestige, Calibration: tc.cal}
			h := newHarness(t, r, m, ps, tc.prestige, seed)
			target := tc.rating + 400*math.Log10(1/r.Biomes[0].TargetClear-1) + r.PrestigeOffset*float64(tc.prestige) + tc.offset
			if got := h.s.Target(m, "dp"); math.Abs(got-target) > 1e-9 {
				t.Errorf("rating %v prestige %d: Target %.1f, want %.1f", tc.rating, tc.prestige, got, target)
			}
			for _, d := range h.s.Doors {
				if math.Abs(float64(d.Rating)-target) > 100 {
					t.Errorf("rating %v prestige %d: door %d, target %.0f±100", tc.rating, tc.prestige, d.Rating, target)
				}
				// SolveP takes the calibration back out of the rating.
				if want := game.Expected(tc.rating, float64(d.Rating)-tc.offset); math.Abs(d.SolveP-want) > 1e-12 {
					t.Errorf("SolveP %v, want %v", d.SolveP, want)
				}
				if d.Tier != tier {
					t.Errorf("plain door on target has tier %q, want %q", d.Tier, tier)
				}
			}
		}
	}
}

// The prestige offset raises a biome's doors until they reach its
// MinClear and no further (D22); a biome without a floor keeps rising.
// Doors and their SolveP follow the clamped aim, and none is under the
// floor: its window keeps only the half above it.
func TestPrestigeFloor(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 400, 3000, 10)
	const rating = 1500
	for _, floor := range []float64{0, 0.6} {
		for p := range game.MaxPrestige + 1 {
			r := plain()
			r.Biomes[0].TargetClear, r.Biomes[0].MinClear = 0.7, floor
			m := &game.Meta{Rating: rating, Prestige: game.MaxPrestige}
			h := newHarness(t, r, m, ps, p, 1)
			want := game.Aim(rating, 0.7) + float64(p)*r.PrestigeOffset
			if floor > 0 {
				want = min(want, game.Aim(rating, floor))
			}
			got := h.s.Target(m, "dp")
			if math.Abs(got-want) > 1e-9 {
				t.Errorf("floor %v prestige %d: Target %.1f, want %.1f", floor, p, got, want)
			}
			if floor > 0 && game.Expected(rating, got) < floor-1e-9 {
				t.Errorf("prestige %d aims at %.3f, under the %.2f floor", p, game.Expected(rating, got), floor)
			}
			for _, d := range h.s.Doors {
				if math.Abs(float64(d.Rating)-want) > 100 || math.Abs(d.SolveP-game.Expected(rating, float64(d.Rating))) > 1e-12 {
					t.Errorf("floor %v prestige %d: door %d (SolveP %.3f) for target %.0f", floor, p, d.Rating, d.SolveP, want)
				}
				// No door under the floor while the pool has doors above it.
				if d.SolveP < floor-1e-12 {
					t.Errorf("floor %v prestige %d: door %d clears %.3f, under the floor", floor, p, d.Rating, d.SolveP)
				}
			}
		}
	}
}

func TestDoorWindows(t *testing.T) {
	t.Parallel()
	at := func(topic string, rating, n int) []catalog.Problem {
		var ps []catalog.Problem
		for i := range n {
			ps = append(ps, catalog.Problem{ID: catalog.ID(topic + string(rune('a'+i)) + string(rune('0'+rating/100))),
				Topic: topic, Rating: rating, Judgeable: true})
		}
		return ps
	}
	// Player 1500 in the 0.92 biome aims at 1076.
	tests := []struct {
		name string
		pool []catalog.Problem
		want []int // door ratings, sorted
	}{
		{"widens to 250", slices.Concat(at("math", 1000, 5), at("math", 2000, 5)), []int{1000, 1000, 1000}},
		{"closest window first", slices.Concat(at("math", 1100, 1), at("math", 1300, 1), at("math", 1600, 5)), []int{1100, 1300, 1600}},
		{"any rating at last", at("math", 2500, 3), []int{2500, 2500, 2500}},
		{"nearest leftovers past the windows", slices.Concat(at("math", 2600, 20), at("math", 1700, 6)), []int{1700, 1700, 1700}},
		{"short pool offers what exists", at("math", 1100, 2), []int{1100, 1100}},
	}
	for _, tc := range tests {
		h := newHarness(t, plain(), &game.Meta{Rating: 1500}, tc.pool, 0, 9)
		var got []int
		for _, d := range h.s.Doors {
			got = append(got, d.Rating)
		}
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: doors %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCrossTopicFallback(t *testing.T) {
	t.Parallel()
	r := routeFirst()
	r.RouteMinPool = 1
	// Player 1500 in the 0.7 biome aims at 1353. One simulation problem is
	// on target and one far off; complete-search has plenty on target.
	ps := []catalog.Problem{
		{ID: "sim-near", Topic: "simulation", Rating: 1350, Judgeable: true},
		{ID: "sim-far", Topic: "simulation", Rating: 2500, Judgeable: true},
	}
	for i := range 10 {
		ps = append(ps, catalog.Problem{ID: catalog.ID("cs" + string(rune('a'+i))), Topic: "complete-search", Rating: 1350, Judgeable: true})
	}
	h := newHarness(t, r, &game.Meta{Rating: 1500}, ps, 0, 2)
	i := slices.Index(h.s.Biomes[0].Offers, "simulation")
	if i < 0 {
		t.Fatalf("offers %v, want simulation among them", h.s.Biomes[0].Offers)
	}
	h.do(game.ChooseRoute{Index: i})
	var topics []string
	for _, d := range h.s.Doors {
		if d.Problem == "sim-far" {
			t.Errorf("offered the off-rating door before an off-topic one: %+v", h.s.Doors)
		}
		topics = append(topics, d.Topic)
	}
	slices.Sort(topics)
	if !slices.Equal(topics, []string{"complete-search", "complete-search", "simulation"}) {
		t.Errorf("door topics %v", topics)
	}
}

func TestDoorEligibility(t *testing.T) {
	t.Parallel()
	mk := func(id string, judgeable bool) catalog.Problem {
		return catalog.Problem{ID: catalog.ID(id), Topic: "math", Rating: 1100, Judgeable: judgeable}
	}
	ps := []catalog.Problem{
		mk("solved1", true), mk("solved2", true), mk("hinted1", true),
		mk("hinted2", true), mk("fresh", true), mk("nojudge1", false), mk("nojudge2", false),
	}
	m := &game.Meta{Rating: 1500, Solves: []game.Solve{
		{Problem: "solved1", Kind: game.Independent}, {Problem: "solved2", Kind: game.Independent},
		{Problem: "hinted1", Kind: game.Hinted}, {Problem: "hinted2", Kind: game.Editorial},
	}}
	for seed := range uint64(10) {
		mm := *m
		h := newHarness(t, plain(), &mm, ps, 0, seed)
		var got []string
		for _, d := range h.s.Doors {
			got = append(got, string(d.Problem))
		}
		slices.Sort(got)
		if want := []string{"fresh", "hinted1", "hinted2"}; !slices.Equal(got, want) {
			t.Fatalf("seed %d: doors %v, want %v", seed, got, want)
		}
		// Retreat: the opened door can't come back this boot.
		d := h.open(0)
		h.do(game.Retreat{})
		for _, nd := range h.s.Doors {
			if nd.Problem == d.Problem {
				t.Fatalf("retreated door %s offered again", d.Problem)
			}
		}
		if len(h.s.Doors) != 2 {
			t.Fatalf("re-roll offered %d doors, want the 2 left", len(h.s.Doors))
		}
	}
}

func TestDoorModifiers(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	price := func(d game.Door, mult float64) int64 {
		base := min(max(0.5/d.SolveP, 0.25), 4)
		return int64(math.Round(64 * base * mult))
	}
	tests := []struct {
		name  string
		edit  func(*rules.Rules)
		check func(game.Door) bool
		mult  float64
	}{
		{"plain", func(r *rules.Rules) {}, func(d game.Door) bool { return !d.Blind && !d.Mystery && !d.Cursed }, 1},
		{"blind all", func(r *rules.Rules) { r.BlindAll = true }, func(d game.Door) bool { return d.Blind && !d.Mystery }, 1.5},
		{"blind", func(r *rules.Rules) { r.BlindChance = 1 }, func(d game.Door) bool { return d.Blind && !d.Mystery }, 1.5},
		{"mystery implies blind, pays 2x not 3x", func(r *rules.Rules) { r.MysteryChance = 1 }, func(d game.Door) bool { return d.Blind && d.Mystery }, 2},
		{"cursed", func(r *rules.Rules) { r.CursedChance = 1 }, func(d game.Door) bool { return d.Cursed && !d.Blind }, 2},
		{"blind cursed", func(r *rules.Rules) { r.CursedChance, r.BlindChance = 1, 1 }, func(d game.Door) bool { return d.Cursed && d.Blind }, 3},
		{"ascension adds chance", func(r *rules.Rules) { r.Ascensions = []rules.Ascension{{Level: 1, BlindChance: 1}} }, func(d game.Door) bool { return d.Blind }, 1.5},
	}
	for _, tc := range tests {
		r := plain()
		tc.edit(&r)
		h := newHarness(t, r, &game.Meta{Prestige: 1}, ps, 1, 4)
		for _, d := range h.s.Doors {
			if !tc.check(d) {
				t.Errorf("%s: door %+v", tc.name, d)
			}
			if want := price(d, tc.mult); d.Bytes != want {
				t.Errorf("%s: %d bytes, want %d", tc.name, d.Bytes, want)
			}
		}
	}

	// Boss doors are never cursed and pay BossMult.
	r := plain()
	r.CursedChance = 1
	r.Biomes = r.Biomes[2:]
	h := newHarness(t, r, nil, ps, 0, 4)
	d := h.s.Doors[0]
	if d.Cursed || d.Boss == "" || d.Bytes != price(d, 3) {
		t.Errorf("boss door %+v, want uncursed at 3x", d)
	}
}

func TestTimers(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)

	h := newHarness(t, plain(), nil, ps, 0, 1)
	for _, d := range h.s.Doors {
		if d.Timer != 15*time.Minute {
			t.Errorf("default timer %v, want 15m", d.Timer)
		}
	}
	h.clearDoor(0, 10*time.Minute) // past O3: one pick
	if !slices.Contains(h.s.UpgradeOffers, game.Overclock) {
		t.Fatalf("offers %v lack overclock", h.s.UpgradeOffers)
	}
	h.do(game.PickUpgrade{Upgrade: game.Overclock})
	for _, d := range h.s.Doors {
		if d.Timer != 17*time.Minute { // 15m × 1.15, on screen at once
			t.Errorf("overclocked door on offer %v, want 17m", d.Timer)
		}
	}

	// nice +20% and a second overclock compound.
	h.s.Daemons = []game.Daemon{game.Nice}
	h.open(0)
	h.do(game.Retreat{})
	for _, d := range h.s.Doors {
		if d.Timer != 21*time.Minute { // 15m × 1.15 × 1.2 = 20.7m
			t.Errorf("nice + overclock %v, want 21m", d.Timer)
		}
	}

	// Ascension timer multiplier.
	r := plain()
	r.Ascensions = []rules.Ascension{{Level: 1, TimerMult: 0.9}, {Level: 2, TimerMult: 0.5}}
	h = newHarness(t, r, &game.Meta{Prestige: 2}, ps, 1, 1)
	if d := h.s.Doors[0]; d.Timer != 14*time.Minute { // 13.5m rounds up
		t.Errorf("prestige 1 timer %v, want 14m (level 2 must not apply)", d.Timer)
	}
}

// TestTimerFit: past TimerFitMin solves a door's timer is the fit, clamped
// to [TimerFitLo, TimerFitHi] of the biome's default. Doors aimed at 0.92
// sit about 420 points below the player, where the fit is under the floor;
// doors at 0.5 straddle the fit's crossing of the ceiling.
func TestTimerFit(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	var solves []game.Solve
	for range 40 {
		solves = append(solves, game.Solve{Problem: "x", Kind: game.Independent, Elapsed: 20 * time.Minute, Gap: 0})
	}
	clamped := map[string]int{}
	for _, aim := range []float64{0.92, 0.5} {
		r := plain()
		r.Biomes = []rules.Biome{{Kind: rules.Fixed, Rooms: 1, TargetClear: aim, Timer: rules.Duration{Duration: 20 * time.Minute}}}
		for seed := range uint64(10) {
			h := newHarness(t, r, &game.Meta{Rating: 1500, Solves: slices.Clone(solves)}, ps, 0, seed)
			for _, d := range h.s.Doors {
				// q85 of 20m is 20m, ×1.25 headroom, ×2^(gap/200).
				fit := 25 * math.Exp2((float64(d.Rating)-1500)/200)
				lo, hi := 20*r.TimerFitLo, 20*r.TimerFitHi
				switch {
				case fit < lo:
					clamped["lo"]++
				case fit > hi:
					clamped["hi"]++
				default:
					clamped["no"]++
				}
				want := time.Duration(min(max(fit, lo), hi) * float64(time.Minute)).Round(time.Minute)
				if d.Timer != want {
					t.Errorf("aim %v door %d: fitted timer %v, want %v", aim, d.Rating, d.Timer, want)
				}
			}
		}
	}
	if len(clamped) != 3 {
		t.Errorf("doors by clamp %v, want some below, inside and above", clamped)
	}
}

func TestSubmit(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	wa := game.SubmitResult{Verdict: "WA", Passed: 3, Total: 10}

	t.Run("wrong answer costs a life", func(t *testing.T) {
		h := newHarness(t, plain(), nil, ps, 0, 1)
		h.open(0)
		fx := h.do(wa)
		a := h.s.Current
		if h.s.Cycles != 2 || a.Failed != 1 || a.Submits != 1 || a.BestPass != 3 || a.Total != 10 || a.LastVerdict != "WA" {
			t.Errorf("lives %d attempt %+v", h.s.Cycles, a)
		}
		if !slices.Equal(sounds(fx), []string{"wa"}) {
			t.Errorf("sounds %v", sounds(fx))
		}
		h.do(game.SubmitResult{Verdict: "TLE", Passed: 1, Total: 10})
		if h.s.Current.BestPass != 3 {
			t.Errorf("BestPass %d, want the best of all submits", h.s.Current.BestPass)
		}
	})
	t.Run("compile error is free", func(t *testing.T) {
		h := newHarness(t, plain(), nil, ps, 0, 1)
		h.open(0)
		h.do(game.SubmitResult{Verdict: "CE"})
		if h.s.Cycles != 3 || h.s.Phase != game.PhaseProblem {
			t.Errorf("lives %d phase %s after CE", h.s.Cycles, h.s.Phase)
		}
	})
	t.Run("nice charges the normal life", func(t *testing.T) {
		h := newHarness(t, plain(), nil, ps, 0, 1)
		h.s.Daemons = []game.Daemon{game.Nice}
		h.open(0)
		h.do(wa)
		if h.s.Cycles != 2 {
			t.Errorf("lives %d, want 2", h.s.Cycles)
		}
	})
	t.Run("a preset's nice can cost two", func(t *testing.T) {
		r := plain()
		r.NiceLifeCost = 2
		h := newHarness(t, r, nil, ps, 0, 1)
		h.s.Daemons = []game.Daemon{game.Nice}
		h.open(0)
		h.do(wa)
		if h.s.Cycles != 1 {
			t.Errorf("lives %d, want 1", h.s.Cycles)
		}
		h.do(wa)
		if h.s.Phase != game.PhaseCrashed || h.s.Cycles != 0 || h.s.Crash.Cause != game.OutOfLives {
			t.Errorf("phase %s lives %d crash %+v", h.s.Phase, h.s.Cycles, h.s.Crash)
		}
	})
	t.Run("last life crashes", func(t *testing.T) {
		h := newHarness(t, plain(), nil, ps, 0, 1)
		h.open(0)
		h.do(wa)
		h.do(wa)
		fx := h.do(wa)
		c := h.s.Crash
		if h.s.Phase != game.PhaseCrashed || c.Cause != game.OutOfLives || c.Verdict != "WA" || c.Cursed {
			t.Fatalf("phase %s crash %+v", h.s.Phase, c)
		}
		if _, ok := has[game.ShowCrash](fx); !ok || !slices.Contains(sounds(fx), "crash") {
			t.Errorf("effects %v, want ShowCrash and the crash sound", fx)
		}
		if last := h.s.Log[len(h.s.Log)-1].Text; !strings.Contains(last, "out of cycles") {
			t.Errorf("last dmesg line %q", last)
		}
	})
	t.Run("cursed crashes on the first failure", func(t *testing.T) {
		r := plain()
		r.CursedChance = 1
		h := newHarness(t, r, nil, ps, 0, 1)
		h.open(0)
		h.do(wa)
		// The curse ends it with lives to spare, and says so.
		if h.s.Phase != game.PhaseCrashed || h.s.Crash.Cause != game.OutOfLives || !h.s.Crash.Cursed || h.s.Cycles != 3 {
			t.Errorf("phase %s lives %d crash %+v", h.s.Phase, h.s.Cycles, h.s.Crash)
		}
		if last := h.s.Log[len(h.s.Log)-1].Text; !strings.Contains(last, "curse triggered") {
			t.Errorf("last dmesg line %q", last)
		}
	})
	t.Run("accepted", func(t *testing.T) {
		h := newHarness(t, plain(), nil, ps, 0, 1)
		d := h.open(0)
		h.now = h.now.Add(10 * time.Minute)
		fx := h.do(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
		s, m := h.s, h.m
		if s.Phase != game.PhaseDoors || s.Room != 1 || len(s.Doors) != 3 {
			t.Fatalf("phase %s room %d doors %d", s.Phase, s.Room, len(s.Doors))
		}
		if s.Uncommitted != d.Bytes || m.LifetimeBytes != d.Bytes || s.Earned() != d.Bytes {
			t.Errorf("uncommitted %d lifetime %d earned %d, want %d", s.Uncommitted, m.LifetimeBytes, s.Earned(), d.Bytes)
		}
		if !s.PendingUpgrade || s.UpgradePicks != 1 || len(s.UpgradeOffers) != 3 {
			t.Errorf("pending %v picks %d offers %v", s.PendingUpgrade, s.UpgradePicks, s.UpgradeOffers)
		}
		if got := s.History[0]; got.Outcome != game.Cleared || got.O3 {
			t.Errorf("history %+v", got)
		}
		if len(m.Solves) != 1 || m.Solves[0].Kind != game.Independent || m.Solves[0].Elapsed != 11*time.Minute ||
			m.Solves[0].Rating != d.Rating || math.Abs(m.Solves[0].Gap-float64(d.Rating-1000)) > 1e-9 {
			t.Errorf("solves %+v", m.Solves)
		}
		if m.Rating <= 1000 {
			t.Errorf("rating %v did not rise on a clear", m.Rating)
		}
		if _, ok := has[game.CloseProblem](fx); !ok || !slices.Contains(sounds(fx), "ac") {
			t.Errorf("effects %v", fx)
		}
		if _, ok := has[game.Prefetch](fx); !ok {
			t.Errorf("effects %v lack the next room's prefetch", fx)
		}
	})
	t.Run("-O3 owes a second pick", func(t *testing.T) {
		h := newHarness(t, plain(), nil, ps, 0, 1)
		h.clearDoor(0, 5*time.Minute) // 6m of 15m
		if h.s.UpgradePicks != 2 || !h.s.History[0].O3 {
			t.Fatalf("picks %d O3 %v", h.s.UpgradePicks, h.s.History[0].O3)
		}
		h.do(game.PickUpgrade{Upgrade: h.s.UpgradeOffers[0]})
		if !h.s.PendingUpgrade || len(h.s.UpgradeOffers) == 0 {
			t.Fatalf("second pick missing: %+v", h.s.UpgradeOffers)
		}
		h.do(game.PickUpgrade{Upgrade: h.s.UpgradeOffers[0]})
		if h.s.PendingUpgrade || h.s.UpgradeOffers != nil || len(h.s.Upgrades) != 2 {
			t.Errorf("after two picks: pending %v offers %v upgrades %v", h.s.PendingUpgrade, h.s.UpgradeOffers, h.s.Upgrades)
		}
	})
	t.Run("-O3 on the last door promises nothing", func(t *testing.T) {
		r := plain()
		r.Biomes = r.Biomes[2:]
		h := newHarness(t, r, nil, ps, 0, 1)
		h.clearDoor(0, time.Minute)
		if h.s.Phase != game.PhaseCleared || !h.s.History[0].O3 || h.s.UpgradePicks != 0 {
			t.Fatalf("phase %s O3 %v picks %d", h.s.Phase, h.s.History[0].O3, h.s.UpgradePicks)
		}
		for _, l := range h.s.Log {
			if strings.Contains(l.Text, "free upgrade") {
				t.Errorf("dmesg %q after the boot's last door", l.Text)
			}
		}
	})
}

func TestUpgrades(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, plain(), nil, ps, 0, 1)
	h.open(0)
	h.do(game.SubmitResult{Verdict: "WA"})
	h.now = h.now.Add(10 * time.Minute)
	h.do(game.SubmitResult{Verdict: "AC"})
	h.do(game.PickUpgrade{Upgrade: game.ECC})
	if h.s.Cycles != 3 {
		t.Errorf("ECC: lives %d, want 3", h.s.Cycles)
	}
	h.clearDoor(0, 10*time.Minute)
	h.do(game.PickUpgrade{Upgrade: game.Duck})
	if h.s.Ducks != 2 {
		t.Errorf("Duck: ducks %d, want 2", h.s.Ducks)
	}
	if err := h.try(game.PickUpgrade{Upgrade: game.Duck}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("a pick with none owed: %v", err)
	}

	// ECC is not offered at full lives.
	h = newHarness(t, plain(), nil, ps, 0, 1)
	h.s.Cycles = h.s.MaxCycles
	h.clearDoor(0, 10*time.Minute)
	if slices.Contains(h.s.UpgradeOffers, game.ECC) || len(h.s.UpgradeOffers) != 2 {
		t.Errorf("offers %v at full lives", h.s.UpgradeOffers)
	}
}

func TestQuack(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	tests := []struct {
		name   string
		edit   func(*rules.Rules)
		levels []int // RevealTags levels of successive quacks; then one fails
	}{
		{"blind: tags then technique", func(r *rules.Rules) { r.BlindAll, r.Ducks = true, 3 }, []int{1, 2}},
		{"tags shown: straight to technique", func(r *rules.Rules) { r.Ducks = 3 }, []int{2}},
		{"out of ducks", func(r *rules.Rules) { r.BlindAll, r.Ducks = true, 1 }, []int{1}},
		{"no ducks at all", func(r *rules.Rules) { r.Ducks = 0 }, nil},
		{"cursed", func(r *rules.Rules) { r.CursedChance, r.Ducks = 1, 3 }, nil},
	}
	for _, tc := range tests {
		r := plain()
		tc.edit(&r)
		h := newHarness(t, r, nil, ps, 0, 1)
		d := h.open(0)
		for i, lvl := range tc.levels {
			fx := h.do(game.Quack{})
			rt, ok := has[game.RevealTags](fx)
			if !ok || rt.Level != lvl || rt.Problem != d.Problem {
				t.Errorf("%s: quack %d effects %v, want level %d", tc.name, i+1, fx, lvl)
			}
		}
		if err := h.try(game.Quack{}); !errors.Is(err, game.ErrInvalid) {
			t.Errorf("%s: quack %d succeeded", tc.name, len(tc.levels)+1)
		}
		if h.s.Current.Quacks != len(tc.levels) {
			t.Errorf("%s: Quacks %d", tc.name, h.s.Current.Quacks)
		}
	}

	// A hinted clear records a Hinted solve and rates 0.7 (at the
	// steady-state K; TestColdStart covers the cold start's).
	r := plain()
	r.RatingColdAttempts = 0
	h := newHarness(t, r, nil, ps, 0, 1)
	d := h.open(0)
	h.do(game.Quack{})
	h.do(game.SubmitResult{Verdict: "AC"})
	want := 1000 + 40*(0.7-game.Expected(1000, float64(d.Rating)))
	if h.m.Solves[0].Kind != game.Hinted || math.Abs(h.m.Rating-want) > 1e-9 {
		t.Errorf("solve %+v rating %v, want hinted and %v", h.m.Solves[0], h.m.Rating, want)
	}
}

func TestRetreat(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)

	h := newHarness(t, plain(), nil, ps, 0, 1)
	d := h.open(1)
	fx := h.do(game.Retreat{})
	if h.s.Cycles != 2 || h.s.Phase != game.PhaseDoors || len(h.s.Doors) != 3 {
		t.Fatalf("lives %d phase %s doors %d", h.s.Cycles, h.s.Phase, len(h.s.Doors))
	}
	if got := h.s.History[0]; got.Outcome != game.Retreated || got.Door.Problem != d.Problem {
		t.Errorf("history %+v", got)
	}
	if _, ok := has[game.CloseProblem](fx); !ok {
		t.Errorf("effects %v", fx)
	}
	// A swap is a choice of door, not a loss: no rating moves.
	if h.m.Rating != 1000 || h.m.Rated != 0 || h.m.TopicAttempts[d.Topic] != 0 {
		t.Errorf("a retreat rated: rating %v, rated %d, topic attempts %v", h.m.Rating, h.m.Rated, h.m.TopicAttempts)
	}
	h.open(0)
	h.do(game.Retreat{})
	h.open(0)
	if err := h.try(game.Retreat{}); !errors.Is(err, game.ErrInvalid) || h.s.Cycles != 1 {
		t.Errorf("retreat on the last life: %v, lives %d", err, h.s.Cycles)
	}

	// kexec: the first per biome is free, and it resets at the next biome.
	h = newHarness(t, plain(), nil, ps, 0, 1)
	h.s.Daemons = []game.Daemon{game.Kexec}
	h.open(0)
	h.do(game.Retreat{})
	h.open(0)
	h.do(game.Retreat{})
	if h.s.Cycles != 2 || !h.s.KexecUsed {
		t.Errorf("kexec: lives %d used %v, want 2 and true", h.s.Cycles, h.s.KexecUsed)
	}
	h.toMerge()
	h.pick()
	h.do(game.LeaveMerge{})
	if h.s.KexecUsed {
		t.Error("kexec still used in the next biome")
	}
}

func TestDeadline(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	ac := game.SubmitResult{Verdict: "AC"}
	tests := []struct {
		name  string
		after time.Duration // since the door opened
		a     game.Action
		phase game.Phase
	}{
		{"tick before the deadline", 14 * time.Minute, game.Tick{}, game.PhaseProblem},
		{"tick at the deadline", 15 * time.Minute, game.Tick{}, game.PhaseCrashed},
		{"judge result within grace", 15*time.Minute + 29*time.Second, ac, game.PhaseDoors},
		{"judge result after grace", 15*time.Minute + 30*time.Second, ac, game.PhaseCrashed},
		{"retreat after the deadline", 15 * time.Minute, game.Retreat{}, game.PhaseCrashed},
		{"quack after the deadline", 16 * time.Minute, game.Quack{}, game.PhaseCrashed},
		{"retreat before it", 14 * time.Minute, game.Retreat{}, game.PhaseDoors},
	}
	for _, tc := range tests {
		h := newHarness(t, plain(), nil, ps, 0, 1)
		h.open(0)
		opened := h.s.Current.Opened
		if _, err := game.Step(h.s, h.m, ps, tc.a, opened.Add(tc.after)); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if h.s.Phase != tc.phase {
			t.Errorf("%s: phase %s, want %s", tc.name, h.s.Phase, tc.phase)
		}
		if tc.phase == game.PhaseCrashed && h.s.Crash.Cause != game.TimedOut {
			t.Errorf("%s: cause %s", tc.name, h.s.Crash.Cause)
		}
	}
}

func TestGiveUp(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, plain(), nil, ps, 0, 1)
	d := h.open(0)
	h.now = h.now.Add(9 * time.Minute)
	fx := h.do(game.GiveUp{})
	c := h.s.Crash
	if h.s.Phase != game.PhaseCrashed || c == nil || c.Cause != game.GaveUp || c.Door.Problem != d.Problem {
		t.Fatalf("phase %s crash %+v", h.s.Phase, c)
	}
	if c.Uptime != 10*time.Minute { // open at +1m, wait 9m, :q! at +1m
		t.Errorf("uptime %v, want the 10m the door was open", c.Uptime)
	}
	if ed, ok := has[game.OpenEditorial](fx); !ok || ed.Problem != d.Problem {
		t.Errorf("effects %v, want the editorial", fx)
	}
	want := []game.Regression{{Problem: d.Problem, Due: h.now.AddDate(0, 0, 3), Stage: 0, Days: []int{3, 17}}}
	if mustJSON(t, h.m.Regressions) != mustJSON(t, want) {
		t.Errorf("regressions %+v, want %+v", h.m.Regressions, want)
	}
	if h.m.CoreDump != nil {
		t.Errorf("core dump %+v from a boot with nothing uncommitted", h.m.CoreDump)
	}
}

// Uptime counts time inside doors only: it stands still while the player
// sits between doors, and a door that timed out counts its timer however
// late the tick that closed it came (a game closed over the door).
func TestUptimeCountsDoors(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, plain(), nil, ps, 0, 1)
	h.clearDoor(0, 10*time.Minute) // open at +1m, AC at +12m
	if got := h.s.Uptime(t0.Add(6 * time.Minute)); got != 5*time.Minute {
		t.Errorf("uptime 5m into the first door = %v", got)
	}
	h.now = h.now.Add(time.Hour) // an hour on the upgrade pick
	if got := h.s.Uptime(h.now); got != 11*time.Minute {
		t.Errorf("uptime between doors = %v, want the first door's 11m", got)
	}
	d := h.open(0)
	if got := h.s.Uptime(h.now.Add(4 * time.Minute)); got != 15*time.Minute {
		t.Errorf("uptime 4m into the second door = %v, want 15m", got)
	}
	if _, err := game.Step(h.s, h.m, ps, game.Tick{}, h.s.Current.Opened.Add(72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if c := h.s.Crash; c == nil || c.Cause != game.TimedOut || c.Uptime != 11*time.Minute+d.Timer {
		t.Errorf("crash %+v, want a timeout at uptime %v", c, 11*time.Minute+d.Timer)
	}
}

// nice pays for its time in bytes: the same doors, drawn while it runs,
// pay NiceBytesFrac less on a longer timer.
func TestNicePaysInBytes(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	redraw := func(daemons []game.Daemon) []game.Door {
		h := newHarness(t, plain(), nil, ps, 0, 1)
		h.s.Daemons = daemons
		h.open(0)
		h.do(game.Retreat{})
		return h.s.Doors
	}
	without, with := redraw(nil), redraw([]game.Daemon{game.Nice})
	cut := 1 - plain().NiceBytesFrac
	for i, d := range with {
		p := without[i]
		if d.Problem != p.Problem {
			t.Fatalf("door %d: %s with nice, %s without; nice must not move the draw", i, d.Problem, p.Problem)
		}
		if want := float64(p.Bytes) * cut; math.Abs(float64(d.Bytes)-want) > 1 {
			t.Errorf("door %d pays %d B with nice, want about %.1f (%d × %.1f)", i, d.Bytes, want, p.Bytes, cut)
		}
		if d.Timer <= p.Timer {
			t.Errorf("door %d timer %v with nice, %v without", i, d.Timer, p.Timer)
		}
	}
}

// TestCoreDumpOrder pins the order of a crash: the earlier boot's dump dies
// first, then this boot's uncommitted bytes (if any) become the new one.
func TestCoreDumpOrder(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	old := &game.CoreDump{Problem: "old", Bytes: 100, Boot: 7}
	tests := []struct {
		name     string
		dump     *game.CoreDump
		clears   int  // doors cleared before crashing on the next
		commit   bool // commit at the merge window (needs clears >= 2)
		dumpLost int64
		newDump  bool
	}{
		{"first crash with bytes", nil, 1, false, 0, true},
		{"first crash without bytes", nil, 0, false, 0, false},
		{"second crash replaces the dump", old, 1, false, 100, true},
		{"second crash without bytes still loses it", old, 0, false, 100, false},
		{"committed bytes are safe", old, 2, true, 100, false},
		{"uncommitted past a merge are not", old, 2, false, 100, true},
	}
	for _, tc := range tests {
		m := &game.Meta{}
		if tc.dump != nil {
			d := *tc.dump
			m.CoreDump = &d
		}
		h := newHarness(t, plain(), m, ps, 0, 1)
		for range tc.clears {
			h.clearDoor(0, 10*time.Minute)
		}
		if h.s.Phase == game.PhaseMerge {
			if tc.commit {
				h.do(game.Commit{})
			}
			h.pick()
			h.do(game.LeaveMerge{})
			h.do(game.ChooseRoute{Index: 0})
		}
		earned, committed := h.s.Earned(), h.s.Committed
		d := h.open(0)
		h.do(game.GiveUp{})
		c := h.s.Crash
		if c.DumpLost != tc.dumpLost {
			t.Errorf("%s: DumpLost %d, want %d", tc.name, c.DumpLost, tc.dumpLost)
		}
		if c.Lost != earned-committed || h.s.Uncommitted != 0 {
			t.Errorf("%s: Lost %d uncommitted %d, want %d and 0", tc.name, c.Lost, h.s.Uncommitted, earned-committed)
		}
		switch {
		case tc.newDump && (m.CoreDump == nil || m.CoreDump.Problem != d.Problem || m.CoreDump.Bytes != c.Lost || m.CoreDump.Boot != h.s.Boot):
			t.Errorf("%s: core dump %+v, want %d B on %s", tc.name, m.CoreDump, c.Lost, d.Problem)
		case !tc.newDump && m.CoreDump != nil:
			t.Errorf("%s: core dump %+v, want none", tc.name, m.CoreDump)
		}
		if h.s.Earned() != h.s.Committed+c.Lost {
			t.Errorf("%s: earned %d != committed %d + lost %d", tc.name, h.s.Earned(), h.s.Committed, c.Lost)
		}
	}

	// A boot that clears keeps an earlier dump.
	m := &game.Meta{CoreDump: &game.CoreDump{Problem: "old", Bytes: 100}}
	h := newHarness(t, plain(), m, ps, 0, 1)
	fullClear(h)
	if m.CoreDump == nil || m.CoreDump.Bytes != 100 {
		t.Errorf("core dump %+v after a clean boot", m.CoreDump)
	}
}

// fullClear ACs every door, leaving bytes uncommitted at the merges.
func fullClear(h *harness) {
	h.t.Helper()
	for h.s.Phase != game.PhaseCleared {
		switch h.s.Phase {
		case game.PhaseDoors:
			h.clearDoor(0, 10*time.Minute)
		case game.PhaseMerge:
			h.pick()
			h.do(game.LeaveMerge{})
		case game.PhaseRoute:
			h.do(game.ChooseRoute{Index: 0})
		default:
			h.t.Fatalf("phase %s", h.s.Phase)
		}
	}
}

func TestMergeWindow(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, plain(), nil, ps, 0, 1)
	h.open(0)
	h.do(game.Quack{})                  // spend the only duck
	h.now = h.now.Add(10 * time.Minute) // no -O3: one pick
	h.do(game.SubmitResult{Verdict: "AC"})
	h.do(game.PickUpgrade{Upgrade: game.Duck}) // ducks 1, refill level 2
	h.clearDoor(0, 10*time.Minute)
	s, m := h.s, h.m
	if s.Phase != game.PhaseMerge || s.Ducks != 2 {
		t.Fatalf("phase %s ducks %d, want merge with ducks refilled to 2", s.Phase, s.Ducks)
	}
	if len(s.DaemonOffers) != 3 || len(slices.Compact(slices.Sorted(slices.Values(s.DaemonOffers)))) != 3 {
		t.Errorf("daemon offers %v, want 3 distinct", s.DaemonOffers)
	}
	if err := h.try(game.LeaveMerge{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("left with an upgrade owed: %v", err)
	}
	h.pick()

	earned := s.Earned()
	fx := h.do(game.Commit{})
	if s.Committed != earned || m.Bank != earned || s.Uncommitted != 0 || !slices.Equal(sounds(fx), []string{"commit"}) {
		t.Errorf("commit: committed %d bank %d uncommitted %d sounds %v", s.Committed, m.Bank, s.Uncommitted, sounds(fx))
	}
	if fx := h.do(game.Commit{}); len(fx) != 0 {
		t.Errorf("empty commit effects %v", fx)
	}

	missing := game.Daemon("")
	for _, d := range game.AllDaemons {
		if !slices.Contains(s.DaemonOffers, d) {
			missing = d
		}
	}
	if err := h.try(game.PickDaemon{Daemon: missing}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("picked %q, not on offer: %v", missing, err)
	}
	pick := s.DaemonOffers[1]
	h.do(game.PickDaemon{Daemon: pick})
	if !slices.Equal(s.Daemons, []game.Daemon{pick}) || s.DaemonOffers != nil {
		t.Errorf("daemons %v offers %v", s.Daemons, s.DaemonOffers)
	}
	if err := h.try(game.PickDaemon{Daemon: pick}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("second daemon pick in one window: %v", err)
	}
	h.do(game.LeaveMerge{})
	if s.Phase != game.PhaseRoute || s.Biome != 1 || s.Room != 0 {
		t.Errorf("after the window: phase %s biome %d room %d", s.Phase, s.Biome, s.Room)
	}
}

// A merge window offers daemons not yet running, never one a package not
// installed locks (strace and /dev/random in the standard rules), and one
// more with procps. Nor fork once tmux is installed: its rooms already
// hold fork's four doors, and the two do not stack (roomSize).
func TestDaemonOffers(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	daemons := []string{"rng-tools", "strace"}
	tests := []struct {
		installed []string
		held      []game.Daemon
		want      int
	}{
		{nil, nil, 3},
		{nil, []game.Daemon{game.Fork, game.Kexec}, 1},
		{daemons, []game.Daemon{game.Fork, game.Kexec}, 3},
		{append(daemons, "procps"), nil, 4},
		{append(daemons, "procps"), []game.Daemon{game.Fork, game.Kexec, game.Nice}, 0}, // MaxDaemons
		{[]string{"tmux"}, nil, 2},
		{[]string{"tmux"}, []game.Daemon{game.Kexec, game.Nice}, 0},
	}
	for _, tc := range tests {
		h := newHarness(t, plain(), &game.Meta{Installed: tc.installed}, ps, 0, 1)
		h.s.Daemons = tc.held
		h.toMerge()
		if len(h.s.DaemonOffers) != tc.want {
			t.Errorf("%v held %v: offers %v, want %d", tc.installed, tc.held, h.s.DaemonOffers, tc.want)
		}
		for _, d := range h.s.DaemonOffers {
			if slices.Contains(tc.held, d) || h.s.DaemonLocked(d) || d == game.Fork && slices.Contains(tc.installed, "tmux") {
				t.Errorf("%v held %v: offered %q", tc.installed, tc.held, d)
			}
		}
	}
}

func TestForkAndDevRandom(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, plain(), nil, ps, 0, 1)
	h.s.Daemons = []game.Daemon{game.Fork}
	h.clearDoor(0, 10*time.Minute)
	if len(h.s.Doors) != 4 {
		t.Errorf("fork: %d doors after an AC, want 4", len(h.s.Doors))
	}
	h.open(0)
	h.do(game.Retreat{})
	if len(h.s.Doors) != 3 {
		t.Errorf("fork: %d doors after a retreat, want 3", len(h.s.Doors))
	}

	r := plain()
	r.MysteryChance = 1
	h = newHarness(t, r, nil, ps, 0, 1)
	h.s.Daemons = []game.Daemon{game.DevRandom}
	h.open(0)
	h.do(game.Retreat{})
	for _, d := range h.s.Doors {
		want := int64(math.Round(64 * min(max(0.5/d.SolveP, 0.25), 4) * 2 * 2))
		if d.Bytes != want {
			t.Errorf("dev-random mystery door %d bytes, want %d", d.Bytes, want)
		}
	}
}

func TestFullClear(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	// rated is the player's rated attempts before the boot: no prestige
	// unlocks until calibration has CalibAfter of them.
	tests := []struct {
		unlocked, prestige, rated, want int
	}{
		{0, 0, 100, 1}, {1, 0, 100, 1}, {1, 1, 100, 2}, {4, 4, 100, min(5, game.MaxPrestige)}, {game.MaxPrestige, game.MaxPrestige, 100, game.MaxPrestige},
		{0, 0, 0, 0}, // a newcomer clears but stays at prestige 0
	}
	for _, tc := range tests {
		m := &game.Meta{Prestige: tc.unlocked, Rated: tc.rated}
		h := newHarness(t, plain(), m, ps, tc.prestige, 1)
		fullClear(h)
		s := h.s
		if _, ok := has[game.ShowCleared](h.fx); !ok || !slices.Contains(sounds(h.fx), "cleared") {
			t.Errorf("final effects %v", h.fx)
		}
		if s.Uncommitted != 0 || s.Committed != s.Earned() || m.Bank != s.Committed || s.PendingUpgrade {
			t.Errorf("uncommitted %d committed %d earned %d bank %d pending %v",
				s.Uncommitted, s.Committed, s.Earned(), m.Bank, s.PendingUpgrade)
		}
		if m.Prestige != tc.want {
			t.Errorf("unlocked %d, cleared prestige %d: now %d, want %d", tc.unlocked, tc.prestige, m.Prestige, tc.want)
		}
		if len(s.History) != 5 {
			t.Errorf("%d attempts, want 5", len(s.History))
		}
	}
}

func TestPrestigeAscension(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	r := calm(rules.Standard()) // the floor easing adds a life
	tests := []struct {
		prestige, lives, ducks int
	}{
		{0, 3, 1}, {1, 3, 1}, {2, 3, 0}, {3, 3, 0}, {4, 2, 0},
	}
	for _, tc := range tests {
		h := newHarness(t, r, &game.Meta{Prestige: 4}, ps, tc.prestige, 1)
		if h.s.Cycles != tc.lives || h.s.Ducks != tc.ducks {
			t.Errorf("prestige %d: lives %d ducks %d, want %d and %d", tc.prestige, h.s.Cycles, h.s.Ducks, tc.lives, tc.ducks)
		}
	}
}

func TestRegressionDoor(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	target := ps[40].ID
	tests := []struct {
		name string
		due  time.Time
		id   catalog.ID
		want bool
	}{
		{"due", t0.Add(-time.Hour), target, true},
		{"not yet due", t0.Add(24 * time.Hour), target, false},
		{"not in the pool", t0.Add(-time.Hour), "gone:1", false},
	}
	for _, tc := range tests {
		m := &game.Meta{Regressions: []game.Regression{{Problem: tc.id, Due: tc.due, Days: []int{3, 17}}}}
		h := newHarness(t, plain(), m, ps, 0, 1)
		count := func() int {
			n := 0
			for _, d := range h.s.Doors {
				if d.Regression {
					n++
				}
			}
			return n
		}
		for h.s.Biome == 0 && h.s.Phase == game.PhaseDoors {
			if count() != 0 {
				t.Fatalf("%s: regression door in biome 0", tc.name)
			}
			h.clearDoor(0, 10*time.Minute)
		}
		h.pick()
		h.do(game.LeaveMerge{})
		h.do(game.ChooseRoute{Index: 0})
		if got := count(); got != map[bool]int{true: 1, false: 0}[tc.want] {
			t.Fatalf("%s: %d regression doors in biome 1 room 0", tc.name, got)
		}
		if !tc.want {
			continue
		}
		last := h.s.Doors[len(h.s.Doors)-1]
		if !last.Regression || last.Problem != target || last.Cursed {
			t.Fatalf("%s: last door %+v, want the uncursed regression", tc.name, last)
		}
		h.clearDoor(len(h.s.Doors)-1, 10*time.Minute)
		if count() != 0 {
			t.Errorf("%s: a second regression door in one boot", tc.name)
		}
		g := h.m.Regressions[0]
		if g.Stage != 1 || !g.Due.Equal(h.now.AddDate(0, 0, 17)) {
			t.Errorf("%s: after the re-solve %+v, want stage 1 due in 17 days", tc.name, g)
		}
	}
}

func TestElo(t *testing.T) {
	t.Parallel()
	ps := []catalog.Problem{
		{ID: "a", Topic: "dp", Rating: 1000, Judgeable: true},
		{ID: "b", Topic: "dp", Rating: 1000, Judgeable: true},
		{ID: "c", Topic: game.Mixed, Rating: 1000, Judgeable: true},
	}
	r := plain()
	r.DoorsPerRoom = 1
	r.Biomes = []rules.Biome{{Kind: rules.Fixed, Rooms: 3, TargetClear: 0.5, Timer: rules.Duration{Duration: 30 * time.Minute}}}
	r.RatingColdAttempts = 0 // the steady-state K; see TestColdStart

	h := newHarness(t, r, nil, ps, 0, 1)
	first := h.open(0)
	h.do(game.SubmitResult{Verdict: "AC"})
	if h.m.Rating != 1020 {
		t.Errorf("clear at an even door: rating %v, want 1020", h.m.Rating)
	}
	if first.Topic != game.Mixed {
		if got := h.m.TopicRating[first.Topic]; got != 1020 || h.m.TopicAttempts[first.Topic] != 1 {
			t.Errorf("topic %s: rating %v attempts %d", first.Topic, got, h.m.TopicAttempts[first.Topic])
		}
	}
	h.pick()
	h.open(0)
	h.do(game.GiveUp{})
	want := 1020 + 40*(0-game.Expected(1020, 1000))
	if math.Abs(h.m.Rating-want) > 1e-9 {
		t.Errorf("crash: rating %v, want %v", h.m.Rating, want)
	}
	if _, ok := h.m.TopicRating[game.Mixed]; ok {
		t.Error("mixed got a topic rating")
	}

	// A topic's own rating aims doors only after TopicRatingMin attempts.
	m := &game.Meta{Rating: 1500, TopicRating: map[string]float64{"dp": 2000}, TopicAttempts: map[string]int{"dp": 4}}
	if got := game.PlayerRating(&r, m, "dp"); got != 1500 {
		t.Errorf("4 attempts: %v, want the overall 1500", got)
	}
	m.TopicAttempts["dp"] = 5
	if got := game.PlayerRating(&r, m, "dp"); got != 2000 {
		t.Errorf("5 attempts: %v, want the topic's 2000", got)
	}
	if got := game.PlayerRating(&r, &game.Meta{}, "dp"); got != 1000 {
		t.Errorf("cold start: %v", got)
	}
}

// TestColdStart: the first RatingColdAttempts rated attempts move the
// overall and topic ratings with RatingKCold, later ones with RatingK, and
// a save from before Meta.Rated (topic attempts only) is not put back into
// a cold start.
func TestColdStart(t *testing.T) {
	t.Parallel()
	ps := []catalog.Problem{{ID: "a", Topic: "dp", Rating: 1000, Judgeable: true}}
	r := plain()
	r.DoorsPerRoom = 1
	r.Biomes = []rules.Biome{{Kind: rules.Fixed, Rooms: 1, TargetClear: 0.5, Timer: rules.Duration{Duration: 30 * time.Minute}}}
	off := r
	off.RatingColdAttempts = 0
	tests := []struct {
		name string
		r    rules.Rules
		meta game.Meta
		k    float64
	}{
		{"new player", r, game.Meta{}, 100},
		{"last cold attempt", r, game.Meta{Rated: 19}, 100},
		{"warm", r, game.Meta{Rated: 20}, 40},
		{"old save", r, game.Meta{TopicAttempts: map[string]int{"dp": 12, "graphs": 8}}, 40},
		{"old save, few attempts", r, game.Meta{TopicAttempts: map[string]int{"dp": 3}}, 100},
		{"turned off", off, game.Meta{}, 40},
	}
	for _, tc := range tests {
		m := tc.meta
		m.Rating = 1000
		h := newHarness(t, tc.r, &m, ps, 0, 1)
		h.open(0)
		h.do(game.SubmitResult{Verdict: "AC"})
		want := 1000 + tc.k*0.5
		if m.Rating != want || m.TopicRating["dp"] != want || m.Rated != tc.meta.Rated+1 {
			t.Errorf("%s: rating %v topic %v rated %d, want %v, %v and %d",
				tc.name, m.Rating, m.TopicRating["dp"], m.Rated, want, want, tc.meta.Rated+1)
		}
	}
}

func TestInvalidActions(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	all := []game.Action{
		game.ChooseRoute{Index: 0}, game.ChooseRoute{Index: 9}, game.ChooseDoor{Index: 0},
		game.ChooseDoor{Index: -1}, game.ChooseDoor{Index: 9}, game.Retreat{}, game.Quack{},
		game.GiveUp{}, game.SubmitResult{Verdict: "AC"}, game.SubmitResult{Verdict: "WA"},
		game.Commit{}, game.PickUpgrade{Upgrade: game.Overclock}, game.PickUpgrade{Upgrade: "bogus"},
		game.PickDaemon{Daemon: game.Fork}, game.PickDaemon{Daemon: "bogus"}, game.LeaveMerge{},
		game.Link{}, game.Link{Libs: []string{"nope"}}, game.Spend{Item: game.Dup}, game.Spend{Item: game.Fsck},
		game.Spend{Item: "bogus"}, game.Reroll{}, nil,
	}
	setups := []struct {
		phase string
		setup func(h *harness)
		valid func(a game.Action, h *harness) bool // which of all are allowed
	}{
		{"doors", func(h *harness) {}, func(a game.Action, h *harness) bool {
			switch a := a.(type) {
			case game.ChooseDoor:
				return a.Index >= 0 && a.Index < len(h.s.Doors)
			case game.Link:
				return len(a.Libs) == 0 // the first door has not opened
			}
			return false
		}},
		{"doors with a pick owed", func(h *harness) { h.clearDoor(0, 10*time.Minute) }, func(a game.Action, h *harness) bool {
			p, ok := a.(game.PickUpgrade)
			return ok && slices.Contains(h.s.UpgradeOffers, p.Upgrade)
		}},
		{"problem", func(h *harness) { h.open(0) }, func(a game.Action, h *harness) bool {
			switch a.(type) {
			case game.Retreat, game.Quack, game.GiveUp, game.SubmitResult:
				return true
			}
			return false
		}},
		{"merge", func(h *harness) { h.toMerge(); h.pick() }, func(a game.Action, h *harness) bool {
			switch a := a.(type) {
			case game.Commit, game.LeaveMerge:
				return true
			case game.PickDaemon:
				return slices.Contains(h.s.DaemonOffers, a.Daemon)
			case game.Spend:
				return h.s.CanSpend(a.Item) == nil
			}
			return false
		}},
		{"route", func(h *harness) { h.toMerge(); h.pick(); h.do(game.LeaveMerge{}) }, func(a game.Action, h *harness) bool {
			c, ok := a.(game.ChooseRoute)
			return ok && c.Index < len(h.s.Biomes[h.s.Biome].Offers)
		}},
		{"crashed", func(h *harness) { h.open(0); h.do(game.GiveUp{}) }, func(game.Action, *harness) bool { return false }},
		{"cleared", fullClear, func(game.Action, *harness) bool { return false }},
	}
	for _, st := range setups {
		for _, a := range all {
			h := newHarness(t, plain(), nil, ps, 0, 1)
			st.setup(h)
			if st.valid(a, h) {
				continue
			}
			before, meta := mustJSON(t, h.s), mustJSON(t, h.m)
			err := h.try(a)
			if !errors.Is(err, game.ErrInvalid) {
				t.Errorf("%s: %T%+v = %v, want ErrInvalid", st.phase, a, a, err)
			}
			if mustJSON(t, h.s) != before || mustJSON(t, h.m) != meta {
				t.Errorf("%s: %T%+v changed state on error", st.phase, a, a)
			}
		}
		// Tick is fine anywhere and changes nothing outside a problem.
		h := newHarness(t, plain(), nil, ps, 0, 1)
		st.setup(h)
		if h.s.Phase == game.PhaseProblem {
			continue
		}
		before := mustJSON(t, h.s)
		if err := h.try(game.Tick{}); err != nil || mustJSON(t, h.s) != before {
			t.Errorf("%s: Tick = %v, changed %v", st.phase, err, mustJSON(t, h.s) != before)
		}
	}
}

// A topic with MasteryAttempts rated attempts and a rating MasteryMargin
// above the door hides the door's tags (D24) without making it a blind
// door: it pays and teaches like a plain one, and the first quack shows
// the tags. A topic still being learned, or a door near the player's level,
// keeps its tags.
func TestMasteredTopicHidesTags(t *testing.T) {
	t.Parallel()
	ps := dense([]string{"dp"}, 800, 2600, 10)
	meta := func(attempts int) *game.Meta {
		return &game.Meta{
			Rating:        1500,
			TopicRating:   map[string]float64{"dp": 1500},
			TopicAttempts: map[string]int{"dp": attempts},
			Rated:         200,
		}
	}
	tests := []struct {
		name     string
		attempts int
		edit     func(*rules.Rules)
		mastered bool
	}{
		{"mastered", 10, func(*rules.Rules) {}, true},
		{"still learning", 9, func(*rules.Rules) {}, false},
		// Userland aims about 424 below the player.
		{"door near the player", 10, func(r *rules.Rules) { r.MasteryMargin = 450 }, false},
		{"off", 50, func(r *rules.Rules) { r.MasteryAttempts = 0 }, false},
	}
	for _, tc := range tests {
		r := plain()
		r.Ducks = 3
		tc.edit(&r)
		h := newHarness(t, r, meta(tc.attempts), ps, 0, 1)
		for _, d := range h.s.Doors {
			if d.Mastered != tc.mastered || d.Blind {
				t.Errorf("%s: door %d mastered %v blind %v", tc.name, d.Rating, d.Mastered, d.Blind)
			}
			if d.Tier == "" && math.Abs(float64(d.Rating)-h.s.Target(h.m, "dp")) <= 100 {
				t.Errorf("%s: an aimed door teaches no tier", tc.name)
			}
		}
		// The same seed rolls the same doors either way, so the prices
		// must match those of a boot with mastery off.
		off := r
		off.MasteryAttempts = 0
		ref := newHarness(t, off, meta(tc.attempts), ps, 0, 1)
		for i, d := range h.s.Doors {
			if d.Bytes != ref.s.Doors[i].Bytes || d.Problem != ref.s.Doors[i].Problem {
				t.Errorf("%s: door %d pays %d, %d with mastery off", tc.name, i, d.Bytes, ref.s.Doors[i].Bytes)
			}
		}
		h.open(0)
		fx := h.do(game.Quack{})
		want := 2
		if tc.mastered {
			want = 1
		}
		if rv, _ := has[game.RevealTags](fx); rv.Level != want {
			t.Errorf("%s: first quack reveals level %d, want %d", tc.name, rv.Level, want)
		}
	}
}

// The prestige gate (D31) counts rated doors against CalibAfter and names
// the next level, none past the highest.
func TestNextPrestige(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	tests := []struct {
		prestige, rated int
		next, left      int
		open            bool
	}{
		{0, r.CalibAfter - 26, 1, 26, false},
		{0, r.CalibAfter, 1, 0, true},
		{2, r.CalibAfter + 5, 3, 0, true},
		{game.MaxPrestige, r.CalibAfter + 5, 0, 0, false},
	}
	for _, tc := range tests {
		g := game.NextPrestige(r, &game.Meta{Prestige: tc.prestige, Rated: tc.rated})
		if g.Next != tc.next || g.Left() != tc.left || g.Open() != tc.open || g.Rated != tc.rated || g.Need != r.CalibAfter {
			t.Errorf("prestige %d, %d rated: %+v (left %d, open %v), want next %d left %d open %v",
				tc.prestige, tc.rated, g, g.Left(), g.Open(), tc.next, tc.left, tc.open)
		}
	}
}

// A route card's timer (State.RouteTimer) is the one the route's doors
// get, every boot multiplier applied: two Overclocks and nice here (UX
// J24). Without a fitted timer every door gets it.
func TestRouteTimer(t *testing.T) {
	t.Parallel()
	h := newHarness(t, routeFirst(), nil, dense(catalog.Topics, 800, 2600, 20), 0, 5)
	if h.s.Phase != game.PhaseRoute {
		t.Fatalf("phase %s, want route", h.s.Phase)
	}
	h.s.Upgrades = []game.Upgrade{game.Overclock, game.Overclock}
	h.s.Daemons = []game.Daemon{game.Nice}
	topic := h.s.Biomes[0].Offers[0]
	want := h.s.RouteTimer(h.m, topic)
	if want != 48*time.Minute { // 30m × 1.15² × 1.2 = 47.6m
		t.Errorf("route timer %v, want 48m", want)
	}
	h.do(game.ChooseRoute{Index: 0})
	for _, d := range h.s.Doors {
		if d.Timer != want {
			t.Errorf("door %s timer %v, route said %v", d.Problem, d.Timer, want)
		}
	}
}
