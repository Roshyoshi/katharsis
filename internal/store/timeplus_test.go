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

// TestDays splits TIME+ by local day and counts what happened on each.
// The fixture, in EDT; a door's clock is what Attempt.Ran counts:
//
//	boot  door (local)        clock   on
//	1     09-01 18:00-18:25   25m     before the calendar
//	2     10-12 21:00-21:20   20m     10-12
//	      10-12 21:25-21:55   30m     10-12; the boot clears
//	3     10-13 23:40-00:20   40m     20m on 10-13, 20m on 10-14
//	      10-14 00:25-00:40   10m     times out at 00:35, crashing the boot
//	4     10-15 19:00-19:10   10m     10-15
//	      10-15 19:15-        30m     open at the 19:45 checkpoint
//
// and three upsolves: a solve on 10-10 from before events began (12m), an
// event on 10-14 12:30 (15m), and one closing 10-15 00:10 after 20m, 10
// of them on 10-14.
func TestDays(t *testing.T) {
	t.Parallel()
	s := open(t, tempPath(t))
	m := game.Meta{Rating: 1200, Solves: []game.Solve{
		{Problem: "cf:2B", Kind: game.Independent, At: local(10, 10, 20, 0), Elapsed: 12 * time.Minute, Upsolve: true},
	}}
	// A door that times out had a 10-minute timer.
	door := func(open, closed time.Time, out game.Outcome) game.Attempt {
		a := game.Attempt{Door: game.Door{Problem: "cf:1A", Topic: "graphs", Rating: 1200},
			Opened: open, Closed: closed, Outcome: out}
		if out == game.TimedOut {
			a.Deadline = open.Add(10 * time.Minute)
		}
		return a
	}
	boot := func(n int, start time.Time, doors ...game.Attempt) *game.State {
		return &game.State{Boot: n, Seed: uint64(n), Rules: rules.Standard(), Started: start,
			Phase: game.PhaseCleared, History: doors}
	}
	checkpoint(t, s, m, boot(1, local(9, 1, 18, 0), door(local(9, 1, 18, 0), local(9, 1, 18, 25), game.Cleared)))
	checkpoint(t, s, m, boot(2, local(10, 12, 21, 0),
		door(local(10, 12, 21, 0), local(10, 12, 21, 20), game.Cleared),
		door(local(10, 12, 21, 25), local(10, 12, 21, 55), game.Cleared)))
	crashed := boot(3, local(10, 13, 23, 30),
		door(local(10, 13, 23, 40), local(10, 14, 0, 20), game.Cleared),
		door(local(10, 14, 0, 25), local(10, 14, 0, 40), game.TimedOut))
	crashed.Phase = game.PhaseCrashed
	crashed.Crash = &game.Crash{Cause: game.TimedOut, At: local(10, 14, 0, 40), Uptime: 50 * time.Minute}
	checkpoint(t, s, m, crashed)
	running := boot(4, local(10, 15, 19, 0), door(local(10, 15, 19, 0), local(10, 15, 19, 10), game.Cleared))
	running.Phase = game.PhaseProblem
	running.Current = &game.Attempt{Door: game.Door{Problem: "cf:3C", Topic: "dp"},
		Opened: local(10, 15, 19, 15), Deadline: local(10, 15, 19, 45)}
	s.SetClock(func() time.Time { return local(10, 15, 19, 45) })
	checkpoint(t, s, m, running)
	for _, e := range []store.Event{
		{At: local(10, 14, 12, 0), Kind: store.EventAppStart},
		{At: local(10, 14, 12, 30), Problem: "cf:2B", Kind: store.EventUpsolveClose,
			Data: map[string]any{"elapsed_ms": (15 * time.Minute).Milliseconds()}},
		{At: local(10, 15, 0, 10), Problem: "cf:3C", Kind: store.EventUpsolveClose,
			Data: map[string]any{"elapsed_ms": (20 * time.Minute).Milliseconds()}},
	} {
		if _, err := s.Event(e); err != nil {
			t.Fatal(err)
		}
	}

	min := func(n int) time.Duration { return time.Duration(n) * time.Minute }
	want := []store.Day{
		{Date: local(10, 9, 0, 0)},
		{Date: local(10, 10, 0, 0), Door: min(12), Upsolves: 1},
		{Date: local(10, 11, 0, 0)},
		{Date: local(10, 12, 0, 0), Door: min(50), Boots: 1, Clears: 1},
		{Date: local(10, 13, 0, 0), Door: min(20), Boots: 1},
		{Date: local(10, 14, 0, 0), Door: min(20 + 10 + 15 + 10), Crashes: 1, Upsolves: 1},
		{Date: local(10, 15, 0, 0), Door: min(10 + 10 + 30), Boots: 1, Upsolves: 1},
	}
	days := func(from, to time.Time) []store.Day {
		t.Helper()
		got, err := s.Days(from, to)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	check := func(what string, got, want []store.Day) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: %d days, want %d: %+v", what, len(got), len(want), got)
		}
		for i := range want {
			g, w := got[i], want[i]
			if !g.Date.Equal(w.Date) || g.Date.Location() != edt || g.Door != w.Door ||
				g.Boots != w.Boots || g.Clears != w.Clears || g.Crashes != w.Crashes || g.Upsolves != w.Upsolves {
				t.Errorf("%s: day %d = %+v, want %+v", what, i, g, w)
			}
		}
	}
	// Any time of day names its day: the range is whole days.
	check("10-09 to 10-15", days(local(10, 9, 12, 0), retentionNow), want)
	// A range starting at midnight keeps only that day's side of a door
	// open across it.
	check("from 10-14", days(local(10, 14, 0, 0), retentionNow), want[5:])
	if got := days(retentionNow, local(10, 9, 0, 0)); len(got) != 0 {
		t.Errorf("a backwards range = %+v, want no days", got)
	}

	// Over every day played, the days add up to TIME+.
	var sum time.Duration
	for _, d := range days(local(1, 1, 0, 0), retentionNow) {
		sum += d.Door
	}
	if tp, err := s.TimePlus(0); err != nil || sum != tp {
		t.Errorf("the days add up to %v, TIME+ is %v (%v)", sum, tp, err)
	}
}
