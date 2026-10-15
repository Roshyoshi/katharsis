package game_test

import (
	"encoding/json"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// crashOn opens door 0 and crashes it by cause: a timer run out, a wrong
// answer on the last life, or :q!.
func crashOn(h *harness, cause game.Outcome) game.Door {
	h.t.Helper()
	d := h.open(0)
	switch cause {
	case game.TimedOut:
		h.now = h.s.Current.Deadline
		h.do(game.Tick{})
	case game.OutOfLives:
		for h.s.Phase == game.PhaseProblem {
			h.do(game.SubmitResult{Verdict: "WA", Passed: 7, Total: 10})
		}
	default:
		h.do(game.GiveUp{})
	}
	if h.s.Phase != game.PhaseCrashed {
		h.t.Fatalf("phase %s after %s, want crashed", h.s.Phase, cause)
	}
	return d
}

func TestDebriefCrash(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	for _, cause := range []game.Outcome{game.TimedOut, game.OutOfLives, game.GaveUp} {
		old := &game.CoreDump{Problem: "old", Bytes: 100, Boot: 7, Mult: 2}
		m := &game.Meta{Rating: 1200, Bank: 50, CoreDump: old}
		h := newHarness(t, plain(), m, ps, 0, 1)
		h.clearDoor(0, 10*time.Minute)
		earned := h.s.Earned()
		d := crashOn(h, cause)
		b := game.Debrief(h.s, m)

		want := map[game.Outcome]game.LeverKind{game.TimedOut: game.LeverTimer, game.OutOfLives: game.LeverLives, game.GaveUp: game.LeverGaveUp}[cause]
		if b.Cleared || b.Lever.Kind != want || b.Lever.Door.Problem != d.Problem || b.Lever.Predicted != d.SolveP || b.Lever.Timer != d.Timer {
			t.Errorf("%s: lever %+v, want %s on %s", cause, b.Lever, want, d.Problem)
		}
		if cause == game.OutOfLives && (b.Lever.Verdict != "WA" || b.Lever.Passed != 7 || b.Lever.Total != 10 || b.Lever.Failed != 3) {
			t.Errorf("%s: lever tests %+v", cause, b.Lever)
		}
		// kept: the rating's before and after, the bank, the regression.
		if !b.Rating.Known || b.Rating.Before != 1200 || b.Rating.After != m.Rating {
			t.Errorf("%s: rating %+v, meta %v", cause, b.Rating, m.Rating)
		}
		if b.Topic != d.Topic || !b.TopicRating.Known || b.TopicRating.After != m.TopicRating[d.Topic] {
			t.Errorf("%s: topic %q %+v", cause, b.Topic, b.TopicRating)
		}
		if b.Bank != m.Bank || b.Regression == nil || b.Regression.Problem != d.Problem {
			t.Errorf("%s: bank %d regression %+v", cause, b.Bank, b.Regression)
		}
		// The dump: the old one named as lost, the new one live, and the
		// next step upsolves it at the independent rate (standard 1.5x).
		dm := b.Dump
		if dm.Lost == nil || dm.Lost.Problem != "old" || dm.Lost.Bytes != 100 {
			t.Errorf("%s: lost dump %+v", cause, dm.Lost)
		}
		if dm.Kind != game.DumpLive || dm.Bytes != earned || dm.Independent != (earned*3+1)/2 {
			t.Errorf("%s: dump %+v, want %d B live", cause, dm, earned)
		}
		if b.Next != (game.Next{Kind: game.NextUpsolve, Problem: d.Problem, Bytes: dm.Independent}) {
			t.Errorf("%s: next %+v", cause, b.Next)
		}
		if b.Gate != game.NextPrestige(h.s.Rules, m) {
			t.Errorf("%s: gate %+v", cause, b.Gate)
		}

		// After an upsolve the dump reads as recovered and the next step
		// is a reboot.
		game.Upsolve(m, d.Problem, game.Editorial, time.Minute, h.now.Add(time.Hour))
		if b := game.Debrief(h.s, m); b.Dump.Kind != game.DumpRecovered || b.Dump.Recovered != earned || b.Next.Kind != game.NextReboot {
			t.Errorf("%s: after upsolve: dump %+v next %+v", cause, b.Dump, b.Next)
		}
	}
}

func TestDebriefCrashWithoutBytes(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	m := &game.Meta{}
	h := newHarness(t, plain(), m, ps, 0, 1)
	crashOn(h, game.GaveUp)
	b := game.Debrief(h.s, m)
	if b.Dump.Kind != game.DumpNone || b.Dump.Lost != nil || b.Next.Kind != game.NextReboot {
		t.Errorf("dump %+v next %+v", b.Dump, b.Next)
	}
}

func TestDebriefCursed(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	r := plain()
	r.CursedChance = 1
	m := &game.Meta{}
	h := newHarness(t, r, m, ps, 0, 1)
	d := h.open(0)
	if !d.Cursed {
		t.Fatal("door not cursed")
	}
	h.do(game.SubmitResult{Verdict: "WA", Passed: 3, Total: 10})
	if b := game.Debrief(h.s, m); b.Lever.Kind != game.LeverCursed || b.Lever.Verdict != "WA" {
		t.Errorf("lever %+v", b.Lever)
	}
}

func TestDebriefClear(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	m := &game.Meta{Rated: 100, Rating: 1300}
	h := newHarness(t, plain(), m, ps, 0, 1)
	fullClear(h)
	b := game.Debrief(h.s, m)
	last := h.s.History[len(h.s.History)-1]
	if !b.Cleared || b.Lever.Kind != game.LeverCleared || b.Lever.Door.Problem != last.Door.Problem || b.Lever.Elapsed != 11*time.Minute {
		t.Errorf("lever %+v, want the boss %s", b.Lever, last.Door.Problem)
	}
	if b.Unlocked != 1 || b.Top || b.Committed != h.s.Committed || b.Bank != m.Bank {
		t.Errorf("unlocked %d top %v committed %d bank %d", b.Unlocked, b.Top, b.Committed, b.Bank)
	}
	if b.Dump.Kind != game.DumpNone || b.Next.Kind != game.NextReboot || b.Regression != nil || b.Topic != "" {
		t.Errorf("clear has dump %+v next %+v regression %+v topic %q", b.Dump, b.Next, b.Regression, b.Topic)
	}
	if !b.Rating.Known || b.Rating.Before != 1300 {
		t.Errorf("rating %+v", b.Rating)
	}
}

// A boot saved before AtStart existed debriefs without the befores; one
// still running debriefs as nothing.
func TestDebriefWithoutStart(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	m := &game.Meta{CoreDump: &game.CoreDump{Problem: "old", Bytes: 40}}
	h := newHarness(t, plain(), m, ps, 0, 1)
	if b := game.Debrief(h.s, m); b != (game.Brief{}) {
		t.Errorf("running boot debriefs as %+v", b)
	}
	h.s.AtStart = nil
	h.clearDoor(0, 10*time.Minute)
	crashOn(h, game.GaveUp)
	b := game.Debrief(h.s, m)
	if b.Rating.Known || b.TopicRating.Known {
		t.Errorf("rating %+v topic %+v known without a start", b.Rating, b.TopicRating)
	}
	if b.Dump.Lost == nil || b.Dump.Lost.Bytes != 40 || b.Dump.Lost.Problem != "" {
		t.Errorf("lost dump %+v, want 40 B with no name", b.Dump.Lost)
	}
}

// AtStart snapshots the player: later rating changes do not reach it, and
// a trial puts no dump at risk.
func TestAtStart(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	m := &game.Meta{Rating: 1100, Bank: 9, TopicRating: map[string]float64{"dp": 1000},
		CoreDump: &game.CoreDump{Problem: "old", Bytes: 40}}
	h := newHarness(t, plain(), m, ps, 0, 1)
	m.TopicRating["dp"] = 5
	a := h.s.AtStart
	if a == nil || a.Rating != 1100 || a.Bank != 9 || a.TopicRating["dp"] != 1000 || a.Dump == nil || a.Dump.Bytes != 40 {
		t.Fatalf("AtStart %+v", a)
	}
	raw, err := json.Marshal(h.s)
	if err != nil {
		t.Fatal(err)
	}
	var back game.State
	if err := json.Unmarshal(raw, &back); err != nil || back.AtStart == nil || back.AtStart.Dump.Problem != "old" {
		t.Errorf("AtStart round trip %+v: %v", back.AtStart, err)
	}
	tr := newTrial(t, rules.Trial(), m, silverTrial(), 1).s
	if tr.AtStart == nil || tr.AtStart.Dump != nil {
		t.Errorf("trial AtStart %+v", tr.AtStart)
	}
}

func TestUpsolveIndependentPaysMore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		dump      game.CoreDump
		kind      game.SolveKind
		recovered int64
	}{
		{"independent, standard 1.5x", game.CoreDump{Problem: "x", Bytes: 120}, game.Independent, 180},
		{"independent, the dump's own 2x", game.CoreDump{Problem: "x", Bytes: 120, Mult: 2}, game.Independent, 240},
		{"independent, 1x pays the same", game.CoreDump{Problem: "x", Bytes: 120, Mult: 1}, game.Independent, 120},
		{"independent, a preset's 0.5x", game.CoreDump{Problem: "x", Bytes: 120, Mult: 0.5}, game.Independent, 60},
		{"rounds to the byte", game.CoreDump{Problem: "x", Bytes: 37, Mult: 1.5}, game.Independent, 56},
		{"editorial", game.CoreDump{Problem: "x", Bytes: 120, Mult: 2}, game.Editorial, 120},
		{"hinted", game.CoreDump{Problem: "x", Bytes: 120, Mult: 2}, game.Hinted, 120},
	}
	for _, tc := range tests {
		d := tc.dump
		m := game.Meta{Bank: 10, LifetimeBytes: 1000, CoreDump: &d}
		got := game.Upsolve(&m, "x", tc.kind, time.Minute, t0)
		if got != tc.recovered || m.Bank != 10+tc.recovered || m.LifetimeBytes != 1000+max(tc.recovered-tc.dump.Bytes, 0) {
			t.Errorf("%s: recovered %d bank %d lifetime %d, want %d", tc.name, got, m.Bank, m.LifetimeBytes, tc.recovered)
		}
		if r := tc.dump.Recovers(tc.kind); r != tc.recovered {
			t.Errorf("%s: Recovers %d, want %d", tc.name, r, tc.recovered)
		}
	}
}

// The crash stamps the boot's multiplier on the dump it writes, so a
// preset's setting holds for the upsolve days later (D2).
func TestCrashStampsDumpMult(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	r := plain()
	r.DumpIndependentMult = 3
	m := &game.Meta{}
	h := newHarness(t, r, m, ps, 0, 1)
	h.clearDoor(0, 10*time.Minute)
	crashOn(h, game.GaveUp)
	if m.CoreDump == nil || m.CoreDump.Mult != 3 {
		t.Fatalf("dump %+v, want Mult 3", m.CoreDump)
	}
}
