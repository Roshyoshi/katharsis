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

// trialPool has prefix-sums problems at chosen ratings, so the draws of a
// Silver trial (band 1350..1650) are determined: in the band only 1400 and
// 1600 are free once the unit takes 1300 and 1500; past it 1700 is within
// the first window and 1200 within the second. Off-topic problems in the
// band, and filler, must lose to all of them.
func trialPool() []catalog.Problem {
	var pool []catalog.Problem
	for _, r := range []int{1200, 1300, 1400, 1500, 1600, 1700, 2400} {
		pool = append(pool, catalog.Problem{ID: ps(r), Title: fmt.Sprint("PS ", r), Topic: "prefix-sums", Rating: r, Judgeable: true})
	}
	pool = append(pool,
		catalog.Problem{ID: "ps:unjudgeable", Topic: "prefix-sums", Rating: 1450},
		catalog.Problem{ID: "dp:1500", Topic: "dp", Rating: 1500, Judgeable: true},
		catalog.Problem{ID: "dp:1550", Topic: "dp", Rating: 1550, Judgeable: true},
	)
	return append(pool, dense([]string{"simulation"}, 800, 2600, 100)...)
}

func silverTrial() game.TrialSpec {
	return game.TrialSpec{
		Module: "prefix-sums", Title: "Introduction to Prefix Sums",
		// Out of rating order (the module's own order wins), with an id
		// the pool lacks, an unjudgeable one and a repeat.
		Unit:       []catalog.ID{ps(1500), "usaco:404", "ps:unjudgeable", ps(1300), ps(1500)},
		IntegTopic: "prefix-sums", IntegBand: [2]int{1350, 1650},
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

func TestNewTrialLayout(t *testing.T) {
	t.Parallel()
	for seed := range uint64(10) {
		h := newTrial(t, rules.Trial(), nil, silverTrial(), seed)
		s, m := h.s, h.m
		tr := s.Trial
		// Top-ups 1400 and 1600 merge into 1500, 1300 by rating without
		// reordering the module's two.
		if want := []catalog.ID{ps(1400), ps(1500), ps(1300), ps(1600)}; !slices.Equal(tr.Plan[0], want) {
			t.Fatalf("seed %d: unit %v, want %v", seed, tr.Plan[0], want)
		}
		if want := []catalog.ID{ps(1400), ps(1600)}; !slices.Equal(tr.TopUp, want) {
			t.Errorf("top-ups %v, want %v", tr.TopUp, want)
		}
		// On topic beats in band: 1700 and 1200, not dp:1500.
		if got := slices.Sorted(slices.Values(tr.Plan[1])); !slices.Equal(got, []catalog.ID{ps(1200), ps(1700)}) {
			t.Errorf("integration %v, want ps:1200 and ps:1700", tr.Plan[1])
		}
		if len(s.Biomes) != 2 || s.Biomes[0].Rooms != 4 || s.Biomes[1].Rooms != 2 ||
			s.Rules.Biomes[0].Rooms != 4 || s.Rules.Biomes[1].Rooms != 2 ||
			s.Biomes[0].Name != "unit" || s.Biomes[1].Name != "integration" || s.Biomes[0].Topic != "prefix-sums" {
			t.Errorf("biomes %+v, rules biomes %+v", s.Biomes, s.Rules.Biomes)
		}
		if s.Boot != 1 || m.Boots != 1 || s.Prestige != 0 || s.Phase != game.PhaseDoors || len(s.Doors) != 1 {
			t.Fatalf("boot %d/%d prestige %d phase %s doors %d", s.Boot, m.Boots, s.Prestige, s.Phase, len(s.Doors))
		}
		d := s.Doors[0]
		if d.Problem != ps(1400) || d.Blind || d.Mystery || d.Cursed || d.Tier != "" {
			t.Errorf("first door %+v, want ps:1400 with tags shown and no tier", d)
		}
		pf, ok := has[game.Prefetch](h.fx)
		if !ok || len(pf.Problems) != 6 {
			t.Errorf("effects %v, want a Prefetch of the whole plan first", h.fx)
		}
	}
}

func TestNewTrialErrors(t *testing.T) {
	t.Parallel()
	choice := rules.Trial()
	choice.Biomes[1].Kind, choice.Biomes[1].Choices = rules.Choice, 2
	empty := silverTrial()
	empty.IntegBand = [2]int{1650, 1350}
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

// clearTrial ACs every door of a trial, committing at the merge window.
func clearTrial(h *harness) {
	h.t.Helper()
	for h.s.Phase != game.PhaseCleared {
		switch h.s.Phase {
		case game.PhaseDoors:
			h.clearDoor(0, 10*time.Minute)
		case game.PhaseMerge:
			h.pick()
			if len(h.s.DaemonOffers) != 0 {
				h.t.Fatalf("a trial offered daemons %v", h.s.DaemonOffers)
			}
			h.do(game.Commit{})
			h.do(game.LeaveMerge{})
			if d := h.s.Doors[0]; !d.Blind || d.Mystery || d.Cursed {
				h.t.Fatalf("integration door %+v, want blind only", d)
			}
		default:
			h.t.Fatalf("phase %s in a trial", h.s.Phase)
		}
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
		if len(m.Solves) != 6 || slices.ContainsFunc(m.Solves, func(x game.Solve) bool { return !x.Trial }) {
			t.Errorf("%s: solves %+v, want 6 trial solves", tc.name, m.Solves)
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
		fail  func(h *harness)
		cause game.Outcome
	}{
		{":q!", func(h *harness) { h.do(game.GiveUp{}) }, game.GaveUp},
		{"out of lives", func(h *harness) { h.do(wa); h.do(wa); h.do(wa) }, game.OutOfLives},
	}
	for _, tc := range tests {
		dump := game.CoreDump{Problem: "usaco:1", Bytes: 300, Boot: 7}
		m := &game.Meta{CoreDump: &dump}
		h := newTrial(t, rules.Trial(), m, silverTrial(), 1)
		h.clearDoor(0, 10*time.Minute)
		h.open(0)
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
