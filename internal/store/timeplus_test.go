package store_test

import (
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/store"
)

// TIME+ sums the time inside doors: boots (a trial among them) as their
// uptime, upsolves as their events, or as their solves from before events
// began. A door left open across a restart counts to its deadline only.
func TestTimePlus(t *testing.T) {
	t.Parallel()
	now := t0
	s := open(t, tempPath(t))
	s.SetClock(func() time.Time { return now })
	m := sampleMeta()
	day := func(n int) time.Time { return t0.Add(time.Duration(n) * 24 * time.Hour) }

	if got, err := s.TimePlus(0); err != nil || got != 0 {
		t.Fatalf("empty TimePlus = %v, %v; want 0", got, err)
	}

	// Doors of 9 and 14 min, then the third: 20 min on a crash, 25 on a
	// clear. The trial is a boot like any other.
	checkpoint(t, s, m, crash(midBoot(1, day(1)), game.TimedOut)) // 43
	checkpoint(t, s, m, clearBoot(midBoot(2, day(2))))            // 48
	trial := midBoot(3, day(3))
	trial.Rules, trial.Trial = rules.Trial(), &game.TrialState{TrialSpec: game.TrialSpec{Module: "prefix-sums-intro"}}
	checkpoint(t, s, m, clearBoot(trial)) // 48

	// Boot 4's third door opens at 40 min with a 30-min timer, and the game
	// closes 10 min into it: TIME+ has the 33 min the boot has had so far.
	open4 := midBoot(4, day(4))
	now = day(4).Add(50 * time.Minute)
	checkpoint(t, s, m, open4)
	want := (43 + 48 + 48 + 33) * time.Minute
	if got, err := s.TimePlus(0); err != nil || got != want {
		t.Errorf("TimePlus with a door open = %v, %v; want %v", got, err, want)
	}

	// Two days later the game resumes and times the door out: it counts
	// to its deadline, 30 min, not the days it sat closed.
	now = day(6)
	a := *open4.Current
	a.Closed, a.Outcome = now, game.TimedOut
	open4.History, open4.Current = append(open4.History, a), nil
	open4.Phase = game.PhaseCrashed
	open4.Crash = &game.Crash{Cause: game.TimedOut, Door: a.Door, At: now, Uptime: open4.Uptime(now)}
	checkpoint(t, s, m, open4) // 9 + 14 + 30 = 53

	// Upsolves. Events began with a session on day 7: an upsolve solved
	// before it counts its solve (12 min on usaco:987); one after counts
	// its event (15 min on usaco:1060), its solve not again; so does an
	// upsolve left unsolved (5 min, on a problem no boot met). A solve in
	// a boot is in that boot's uptime already.
	session := day(7)
	m.Solves = append(m.Solves,
		game.Solve{Problem: "usaco:987", Kind: game.Independent, At: session.Add(-time.Hour), Elapsed: 12 * time.Minute, Upsolve: true},
		game.Solve{Problem: "usaco:1060", Kind: game.Editorial, At: session.Add(30 * time.Minute), Elapsed: 15 * time.Minute, Upsolve: true},
	)
	if err := s.SaveMeta(m); err != nil {
		t.Fatal(err)
	}
	for _, e := range []store.Event{
		{At: session, Kind: store.EventAppStart},
		{At: session.Add(30 * time.Minute), Problem: "usaco:1060", Kind: store.EventUpsolveClose,
			Data: map[string]any{"elapsed_ms": (15 * time.Minute).Milliseconds()}},
		{At: session.Add(40 * time.Minute), Problem: "cf:1A", Kind: store.EventUpsolveClose,
			Data: map[string]any{"elapsed_ms": (5 * time.Minute).Milliseconds()}},
	} {
		if _, err := s.Event(e); err != nil {
			t.Fatal(err)
		}
	}

	want = (43 + 48 + 48 + 53 + 12 + 15 + 5) * time.Minute
	if got, err := s.TimePlus(0); err != nil || got != want {
		t.Errorf("TimePlus = %v, %v; want %v", got, err, want)
	}
	if got, err := s.TimePlus(4); err != nil || got != want-53*time.Minute {
		t.Errorf("TimePlus without boot 4 = %v, %v; want %v", got, err, want-53*time.Minute)
	}

	// By topic: simulation's 9 + 14 in four boots and the old upsolve;
	// prefix sums' third doors (20, 25, 25, 30) and the new one. The
	// upsolve on a problem no boot met is in no topic.
	st, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.TimePlus != want {
		t.Errorf("Stats.TimePlus = %v, want %v", st.TimePlus, want)
	}
	for topic, w := range map[string]time.Duration{
		"simulation":  (4*23 + 12) * time.Minute,
		"prefix-sums": (20 + 25 + 25 + 30 + 15) * time.Minute,
	} {
		if got := st.Topics[topic].Time; got != w {
			t.Errorf("%s TIME+ = %v, want %v", topic, got, w)
		}
	}
}
