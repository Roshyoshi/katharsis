package game_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// ps is a prefix-sums problem id in trialPool.
func ps(rating int) catalog.ID { return catalog.ID(fmt.Sprintf("ps:%d", rating)) }

// trialPool has prefix-sums problems 200 apart, so each of silverTrial's
// aims has one within the narrowest window and the draws are determined.
// Off-topic problems on the aims, and filler, must lose to them.
func trialPool() []catalog.Problem {
	var pool []catalog.Problem
	for _, r := range []int{1200, 1400, 1600, 1800, 2400} {
		pool = append(pool, catalog.Problem{ID: ps(r), Title: fmt.Sprint("PS ", r), Topic: "prefix-sums", Rating: r, Judgeable: true})
	}
	pool = append(pool,
		catalog.Problem{ID: "ps:unjudgeable", Topic: "prefix-sums", Rating: 1700},
		catalog.Problem{ID: "dp:1400", Topic: "dp", Rating: 1400, Judgeable: true},
		catalog.Problem{ID: "dp:1700", Topic: "dp", Rating: 1700, Judgeable: true},
	)
	return append(pool, dense([]string{"simulation"}, 800, 2600, 100)...)
}

func silverTrial() game.TrialSpec {
	return game.TrialSpec{
		Module: "prefix-sums", Title: "Introduction to Prefix Sums",
		Topic: "prefix-sums", Band: [2]int{1350, 1650}, Targets: []int{1400, 1600, 1700},
		Capability: game.CapabilityFor("prefix-sums"),
	}
}

func newTrial(t *testing.T, r rules.Rules, m *game.Meta, spec game.TrialSpec, seed uint64) *harness {
	t.Helper()
	if m == nil {
		m = &game.Meta{}
	}
	pool := trialPool()
	s, fx, err := game.NewTrial(r, m, pool, spec, seed, t0)
	if err != nil {
		t.Fatalf("NewTrial: %v", err)
	}
	return &harness{t: t, s: s, m: m, ps: pool, now: t0, fx: fx}
}

// TestNewTrialLayout: one room per target, each door the on-topic problem
// nearest its aim, every door blind.
func TestNewTrialLayout(t *testing.T) {
	t.Parallel()
	for seed := range uint64(10) {
		h := newTrial(t, rules.Trial(), nil, silverTrial(), seed)
		s, m := h.s, h.m
		if got, want := slices.Concat(s.Trial.Plan...), []catalog.ID{ps(1400), ps(1600), ps(1800)}; !slices.Equal(got, want) {
			t.Fatalf("seed %d: doors %v, want %v", seed, got, want)
		}
		if len(s.Biomes) != 1 || s.Biomes[0].Rooms != 3 || s.Biomes[0].Name != "trial" || s.Biomes[0].Topic != "prefix-sums" {
			t.Errorf("biomes %+v", s.Biomes)
		}
		if s.Boot != 1 || m.Boots != 1 || s.Prestige != 0 || s.Phase != game.PhaseDoors || len(s.Doors) != 1 {
			t.Fatalf("boot %d/%d prestige %d phase %s doors %d", s.Boot, m.Boots, s.Prestige, s.Phase, len(s.Doors))
		}
		if d := s.Doors[0]; d.Problem != ps(1400) || !d.Blind || d.Mystery || d.Cursed || d.Tier != "" {
			t.Errorf("first door %+v, want ps:1400, blind, no tier", d)
		}
		if pf, ok := has[game.Prefetch](h.fx); !ok || len(pf.Problems) != 3 {
			t.Errorf("effects %v, want a Prefetch of the whole plan first", h.fx)
		}
	}
}

// TestTrialAims: with no targets the doors spread over the band; an aim
// the topic cannot reach within every window gives up the topic for the
// rating.
func TestTrialAims(t *testing.T) {
	t.Parallel()
	spread := silverTrial()
	spread.Targets = nil
	far := silverTrial()
	far.Targets = []int{1400, 1600, 3000}
	for _, tc := range []struct {
		name string
		spec game.TrialSpec
		want func(ids []catalog.ID) bool
	}{
		// 1350, 1500, 1650.
		{"spread over the band", spread, func(ids []catalog.ID) bool {
			return slices.Equal(ids, []catalog.ID{ps(1400), ps(1600), ps(1800)})
		}},
		// 3000: ps:2400 is 600 off, past the widest window; the filler at
		// 2500 and 2600 is within it.
		{"far aim", far, func(ids []catalog.ID) bool {
			return slices.Equal(ids[:2], []catalog.ID{ps(1400), ps(1600)}) &&
				(ids[2] == "t:simulation:2500" || ids[2] == "t:simulation:2600")
		}},
	} {
		for seed := range uint64(10) {
			got := slices.Concat(newTrial(t, rules.Trial(), nil, tc.spec, seed).s.Trial.Plan...)
			if !tc.want(got) {
				t.Errorf("%s, seed %d: doors %v", tc.name, seed, got)
			}
		}
	}
}

// TestTrialSkipsSolved: a trial door is one the player has not solved,
// with help or without. ps:1600 was solved with the editorial, so its aim
// takes ps:1800, a window out, and the last aim, 1700, passes over it for
// ps:1200, two windows out: a solved door waits behind every unsolved one
// on the topic in reach.
func TestTrialSkipsSolved(t *testing.T) {
	t.Parallel()
	for seed := range uint64(10) {
		m := &game.Meta{Solves: []game.Solve{{Problem: ps(1600), Kind: game.Editorial}}}
		tr := newTrial(t, rules.Trial(), m, silverTrial(), seed).s.Trial
		if got, want := slices.Concat(tr.Plan...), []catalog.ID{ps(1400), ps(1800), ps(1200)}; !slices.Equal(got, want) {
			t.Fatalf("seed %d: doors %v, want %v", seed, got, want)
		}
	}
}

func TestNewTrialErrors(t *testing.T) {
	t.Parallel()
	choice := rules.Trial()
	choice.Biomes[0].Kind, choice.Biomes[0].Choices = rules.Choice, 2
	empty := silverTrial()
	empty.Band = [2]int{1650, 1350}
	tiny := []catalog.Problem{{ID: "only", Topic: "dp", Rating: 1500, Judgeable: true}}
	tests := []struct {
		name string
		r    rules.Rules
		spec game.TrialSpec
		pool []catalog.Problem
		meta *game.Meta
	}{
		{"nil meta", rules.Trial(), silverTrial(), trialPool(), nil},
		{"boot rules", rules.Legacy(), silverTrial(), trialPool(), &game.Meta{}},
		{"choice biome", choice, silverTrial(), trialPool(), &game.Meta{}},
		{"empty band", rules.Trial(), empty, trialPool(), &game.Meta{}},
		{"one problem", rules.Trial(), silverTrial(), tiny, &game.Meta{}},
	}
	for _, tc := range tests {
		before := mustJSON(t, tc.meta)
		if _, _, err := game.NewTrial(tc.r, tc.meta, tc.pool, tc.spec, 1, t0); err == nil {
			t.Errorf("%s: NewTrial succeeded", tc.name)
		}
		if got := mustJSON(t, tc.meta); got != before {
			t.Errorf("%s: meta changed on error", tc.name)
		}
	}
}

// clearTrial ACs every door of a trial: one biome, so no merge window
// comes between them.
func clearTrial(h *harness) {
	h.t.Helper()
	for h.s.Phase != game.PhaseCleared {
		if h.s.Phase != game.PhaseDoors {
			h.t.Fatalf("phase %s in a trial", h.s.Phase)
		}
		if d := h.s.Doors[0]; !d.Blind || d.Mystery || d.Cursed {
			h.t.Fatalf("door %+v, want blind only", d)
		}
		h.clearDoor(0, 10*time.Minute)
	}
}

func TestTrialPass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		caps    []string
		passed  []string
		granted string
	}{
		{"first pass", nil, nil, "CAP_PREFIX"},
		{"capability already held", []string{"CAP_PREFIX"}, []string{"more-prefix-sums"}, ""},
		{"passed before", []string{"CAP_PREFIX"}, []string{"prefix-sums"}, ""},
	}
	for _, tc := range tests {
		m := &game.Meta{Capabilities: slices.Clone(tc.caps), TrialsPassed: slices.Clone(tc.passed)}
		h := newTrial(t, rules.Trial(), m, silverTrial(), 3)
		clearTrial(h)
		s := h.s
		if !s.Trial.Passed || s.Trial.Granted != tc.granted {
			t.Errorf("%s: trial %+v, want passed, granted %q", tc.name, s.Trial, tc.granted)
		}
		if !game.Unlocked(m, "prefix-sums") || !slices.Equal(m.Capabilities, []string{"CAP_PREFIX"}) {
			t.Errorf("%s: capabilities %v", tc.name, m.Capabilities)
		}
		if n := len(slices.DeleteFunc(slices.Clone(m.TrialsPassed), func(x string) bool { return x != "prefix-sums" })); n != 1 {
			t.Errorf("%s: trials passed %v, want prefix-sums once", tc.name, m.TrialsPassed)
		}
		if m.Prestige != 0 {
			t.Errorf("%s: a trial unlocked prestige %d", tc.name, m.Prestige)
		}
		if _, ok := has[game.ShowCleared](h.fx); !ok {
			t.Errorf("%s: effects %v", tc.name, h.fx)
		}
		if len(m.Solves) != 3 || slices.ContainsFunc(m.Solves, func(x game.Solve) bool { return !x.Trial }) {
			t.Errorf("%s: solves %+v, want 3 trial solves", tc.name, m.Solves)
		}
		if s.Committed != s.Earned() || m.Bank != s.Earned() {
			t.Errorf("%s: committed %d bank %d earned %d", tc.name, s.Committed, m.Bank, s.Earned())
		}
	}
}

// TestTrialFail: failing a trial loses its uncommitted bytes like a crash
// but leaves a real boot's core dump alone and schedules no regression.
func TestTrialFail(t *testing.T) {
	t.Parallel()
	wa := game.SubmitResult{Verdict: "WA", Passed: 1, Total: 10}
	tests := []struct {
		name  string
		clear bool // clear the first door, then open the second
		fail  func(h *harness)
		cause game.Outcome
	}{
		// One room lost, the second :q! ends it.
		{"a second :q!", false, func(h *harness) {
			h.open(0)
			h.do(game.GiveUp{})
			h.open(0)
			h.do(game.GiveUp{})
		}, game.GaveUp},
		{"a :q! after a clear and a loss", true, func(h *harness) {
			h.do(game.GiveUp{})
			h.open(0)
			h.do(game.GiveUp{})
		}, game.GaveUp},
		// However many cycles the upgrade picks left.
		{"out of lives", true, func(h *harness) {
			for n := 0; h.s.Phase == game.PhaseProblem && n < 20; n++ {
				h.do(wa)
			}
		}, game.OutOfLives},
	}
	for _, tc := range tests {
		dump := game.CoreDump{Problem: "usaco:1", Bytes: 300, Boot: 7}
		m := &game.Meta{CoreDump: &dump}
		h := newTrial(t, rules.Trial(), m, silverTrial(), 1)
		if tc.clear {
			h.clearDoor(0, 10*time.Minute)
			h.open(0)
		}
		earned := h.s.Earned()
		tc.fail(h)
		s := h.s
		if s.Phase != game.PhaseCrashed || s.Crash.Cause != tc.cause || s.Crash.Lost != earned || s.Crash.DumpLost != 0 {
			t.Fatalf("%s: phase %s crash %+v, want %s losing %d", tc.name, s.Phase, s.Crash, tc.cause, earned)
		}
		if m.CoreDump == nil || *m.CoreDump != dump {
			t.Errorf("%s: core dump %+v, want the boot's %+v untouched", tc.name, m.CoreDump, dump)
		}
		if len(m.Regressions) != 0 || s.Trial.Passed || len(m.Capabilities) != 0 {
			t.Errorf("%s: regressions %v passed %v capabilities %v", tc.name, m.Regressions, s.Trial.Passed, m.Capabilities)
		}
		_, ed := has[game.OpenEditorial](h.fx)
		if ed != (tc.cause == game.GaveUp) {
			t.Errorf("%s: editorial opened %v", tc.name, ed)
		}
	}
}

// TestTrialLosesRooms: a :q! loses its room, paid nothing and rated as
// failed, and the trial goes on to the next room, or passes after the
// last. Losing TrialLostRooms rooms, any of them, still passes.
func TestTrialLosesRooms(t *testing.T) {
	t.Parallel()
	for lose := range 3 {
		m := &game.Meta{}
		h := newTrial(t, rules.Trial(), m, silverTrial(), 1)
		var lost []catalog.ID
		for door := 0; h.s.Phase != game.PhaseCleared; door++ {
			if h.s.Phase != game.PhaseDoors {
				t.Fatalf("door %d: phase %s", lose, h.s.Phase)
			}
			if door != lose {
				h.clearDoor(0, 10*time.Minute)
				continue
			}
			if n := h.s.LostRoomsLeft(); n != 1 {
				t.Fatalf("door %d: %d rooms left to lose, want 1", door, n)
			}
			d := h.open(0)
			rating := m.Rating
			h.do(game.GiveUp{})
			lost = append(lost, d.Problem)
			if m.Rating >= rating {
				t.Errorf("door %d: %s given up, rating %.0f -> %.0f, want it rated as failed", lose, d.Problem, rating, m.Rating)
			}
			if _, ok := has[game.CloseProblem](h.fx); !ok || h.s.Crash != nil {
				t.Fatalf("door %d: :q!: effects %v, crash %+v", lose, h.fx, h.s.Crash)
			}
		}
		s := h.s
		if !s.Trial.Passed || s.Trial.Lost != 1 || s.Trial.Granted != "CAP_PREFIX" {
			t.Errorf("door %d lost: trial %+v, want passed with 1 lost", lose, s.Trial)
		}
		if len(m.Solves) != 2 || len(m.Regressions) != 0 {
			t.Errorf("door %d lost: %d solves, regressions %v; want 2 and none", lose, len(m.Solves), m.Regressions)
		}
		var gaveUp []catalog.ID
		for _, a := range s.History {
			if a.Outcome == game.GaveUp {
				gaveUp = append(gaveUp, a.Door.Problem)
			}
		}
		if !slices.Equal(gaveUp, lost) {
			t.Errorf("door %d lost: history gave up %v, want %v", lose, gaveUp, lost)
		}
	}
}

func TestTrialNoRetreat(t *testing.T) {
	t.Parallel()
	h := newTrial(t, rules.Trial(), nil, silverTrial(), 1)
	h.open(0)
	before, meta := mustJSON(t, h.s), mustJSON(t, h.m)
	if err := h.try(game.Retreat{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("retreat in a trial: %v", err)
	}
	if mustJSON(t, h.s) != before || mustJSON(t, h.m) != meta {
		t.Error("a refused retreat changed the trial")
	}
}

// TestOvertime: under a soft timer the deadline marks the attempt instead
// of crashing it; an overtime clear pays OvertimeBytesMult and scores at
// most OvertimeScore. Boots can run on a soft timer too, so this uses one.
func TestOvertime(t *testing.T) {
	t.Parallel()
	pool := dense(catalog.Topics, 800, 2600, 20)
	r := plain()
	r.OnExpire = rules.ExpireOvertime
	r.RatingColdAttempts = 0 // K 40, so the score is readable off the rating
	ac := game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10}
	tests := []struct {
		name     string
		before   []game.Action // after the deadline (15 min) plus 1 min
		submit   time.Duration // when the AC lands, from the door opening
		quack    bool
		overtime bool
		score    float64
	}{
		{"in time", nil, 10 * time.Minute, false, false, 1},
		{"within grace", nil, 15*time.Minute + 20*time.Second, false, false, 1},
		{"after grace", nil, 15*time.Minute + 30*time.Second, false, true, 0.5},
		{"a tick marks it", []game.Action{game.Tick{}}, 20 * time.Minute, false, true, 0.5},
		{"a late quack is no crash", []game.Action{game.Quack{}}, 20 * time.Minute, true, true, 0.5},
		{"hinted in time", nil, 10 * time.Minute, true, false, 0.7},
	}
	for _, tc := range tests {
		r := r
		r.Ducks, r.BlindAll = 3, tc.quack
		h := newHarness(t, r, &game.Meta{Rating: 1500}, pool, 0, 1)
		d := h.open(0)
		opened := h.s.Current.Opened
		if tc.quack && tc.before == nil {
			h.do(game.Quack{})
		}
		for _, a := range tc.before {
			if _, err := game.Step(h.s, h.m, pool, a, opened.Add(16*time.Minute)); err != nil {
				t.Fatalf("%s: %T: %v", tc.name, a, err)
			}
			if h.s.Phase != game.PhaseProblem {
				t.Fatalf("%s: %T past the deadline left phase %s", tc.name, a, h.s.Phase)
			}
		}
		if len(tc.before) > 0 && tc.before[0] == (game.Tick{}) && !h.s.Current.Overtime {
			t.Errorf("%s: a Tick past the deadline did not mark overtime", tc.name)
		}
		rating := h.m.Rating
		if _, err := game.Step(h.s, h.m, pool, ac, opened.Add(tc.submit)); err != nil {
			t.Fatal(err)
		}
		a := h.s.History[len(h.s.History)-1]
		if a.Outcome != game.Cleared || a.Overtime != tc.overtime || h.s.Phase == game.PhaseCrashed {
			t.Fatalf("%s: attempt %+v phase %s", tc.name, a, h.s.Phase)
		}
		paid := d.Bytes
		if tc.overtime {
			paid = int64(math.Round(float64(d.Bytes) * r.OvertimeBytesMult))
		}
		if got := h.s.Paid(a); got != paid || h.s.Uncommitted != paid || h.s.Earned() != paid {
			t.Errorf("%s: paid %d uncommitted %d earned %d, want %d", tc.name, got, h.s.Uncommitted, h.s.Earned(), paid)
		}
		if want := rating + 40*(tc.score-game.Expected(rating, float64(d.Rating))); math.Abs(h.m.Rating-want) > 1e-9 {
			t.Errorf("%s: rating %v, want %v (score %v)", tc.name, h.m.Rating, want, tc.score)
		}
		if !h.s.SoftTimer() {
			t.Errorf("%s: SoftTimer false", tc.name)
		}
	}
}

func TestShowsFailingInput(t *testing.T) {
	t.Parallel()
	pool := dense(catalog.Topics, 800, 2600, 20)
	open := func(r rules.Rules, daemons ...game.Daemon) *game.State {
		h := newHarness(t, r, nil, pool, 0, 1)
		h.s.Daemons = daemons
		h.open(0)
		return h.s
	}
	ended := func(a game.Action) *game.State {
		h := newHarness(t, plain(), nil, pool, 0, 1)
		h.open(0)
		h.do(a)
		return h.s
	}
	shown := plain()
	shown.ShowFailingInput = true
	doors := newHarness(t, plain(), nil, pool, 0, 1).s
	tests := []struct {
		name string
		s    *game.State
		want bool
	}{
		{"no boot", nil, true},
		{"in a door", open(plain()), false},
		{"other daemons", open(plain(), game.Fork, game.Kexec), false},
		{"strace", open(plain(), game.Strace), true},
		{"rules show it", open(shown), true},
		{"a trial", newTrial(t, rules.Trial(), nil, silverTrial(), 1).s, true},
		{"choosing doors", doors, false},
		{"crashed", ended(game.GiveUp{}), true},
	}
	for _, tc := range tests {
		if got := tc.s.ShowsFailingInput(); got != tc.want {
			t.Errorf("%s: ShowsFailingInput = %v, want %v", tc.name, got, tc.want)
		}
	}
}
