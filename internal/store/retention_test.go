package store_test

import (
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/store"
)

// The retention fixture is a player's last few months in EDT, built from
// boots, upsolves and worth-it answers; every expectation below is counted
// by hand from this table.
//
//	boot  start (local)   length  ends     outcome   notes
//	1     07-01 18:00     30m              cleared   before the 90 days
//	2     08-01 18:00     30m              crashed
//	3     09-20 18:00     3h30m   21:30    cleared   Sunday
//	4     10-10 14:00     5m      14:05    crashed   replayed by boot 5
//	5     10-10 14:08     12m              cleared
//	6     10-12 20:00     10m     20:10    crashed   upsolved 20:30 (12m)
//	7     10-13 23:50     80m     01:10    crashed   ends late
//	8     10-14 23:30     70m     00:40    crashed   ends late
//	9     10-15 19:00     45m     19:45    cleared   today (Thursday)
var edt = time.FixedZone("EDT", -4*3600)

func local(m time.Month, d, h, mi int) time.Time { return time.Date(2026, m, d, h, mi, 0, 0, edt) }

var retentionNow = local(10, 15, 21, 0)

// endedBoot is boot n, one door long, crashed or cleared after d.
func endedBoot(n int, start time.Time, d time.Duration, crashed bool) *game.State {
	door := game.Door{Problem: catalog.ID("cf:" + string(rune('0'+n))), Topic: "graphs", Rating: 1200, SolveP: 0.8}
	a := game.Attempt{Door: door, Opened: start, Closed: start.Add(d)}
	st := &game.State{Boot: n, Seed: uint64(n), Rules: rules.Standard(), Started: start}
	if crashed {
		a.Outcome, st.Phase = game.TimedOut, game.PhaseCrashed
		st.Crash = &game.Crash{Cause: game.TimedOut, Door: door, At: a.Closed, Uptime: d}
	} else {
		a.Outcome, st.Phase = game.Cleared, game.PhaseCleared
	}
	st.History = []game.Attempt{a}
	return st
}

func event(t *testing.T, s *store.Store, at time.Time, kind string, data map[string]any) {
	t.Helper()
	if _, err := s.Event(store.Event{At: at, Kind: kind, Data: data}); err != nil {
		t.Fatal(err)
	}
}

func retentionFixture(t *testing.T) *store.Store {
	t.Helper()
	s := open(t, tempPath(t))
	m := game.Meta{Rating: 1200, Solves: []game.Solve{
		{Problem: "cf:6", Kind: game.Independent, At: local(10, 12, 20, 30), Upsolve: true},
		{Problem: "cf:7", Kind: game.Independent, At: local(10, 14, 12, 0), Upsolve: true}, // boot 7 is under 48 h old
		{Problem: "cf:4", Kind: game.Hinted, At: local(10, 10, 14, 4)},                     // in the boot: not an upsolve
	}}
	for _, b := range []struct {
		start   time.Time
		d       time.Duration
		crashed bool
	}{
		{local(7, 1, 18, 0), 30 * time.Minute, false},
		{local(8, 1, 18, 0), 30 * time.Minute, true},
		{local(9, 20, 18, 0), 210 * time.Minute, false},
		{local(10, 10, 14, 0), 5 * time.Minute, true},
		{local(10, 10, 14, 8), 12 * time.Minute, false},
		{local(10, 12, 20, 0), 10 * time.Minute, true},
		{local(10, 13, 23, 50), 80 * time.Minute, true},
		{local(10, 14, 23, 30), 70 * time.Minute, true},
		{local(10, 15, 19, 0), 45 * time.Minute, false},
	} {
		n, _ := s.Boots(0)
		checkpoint(t, s, m, endedBoot(len(n)+1, b.start, b.d, b.crashed))
	}
	event(t, s, local(10, 12, 20, 30), store.EventUpsolveClose, map[string]any{"elapsed_ms": int64(12 * time.Minute / time.Millisecond)})
	for _, w := range []struct {
		at     time.Time
		answer string
	}{
		{local(9, 20, 21, 40), "yes"}, {local(9, 25, 20, 0), "yes"}, // the fortnight before
		{local(10, 10, 14, 30), "yes"}, {local(10, 12, 20, 40), "no"}, {local(10, 13, 23, 0), "yes"},
		{local(10, 14, 1, 20), "no"}, {local(10, 15, 19, 50), "skip"},
	} {
		event(t, s, w.at, store.EventWorthIt, map[string]any{"answer": w.answer})
	}
	event(t, s, local(10, 13, 12, 0), store.EventAlarmShown, map[string]any{"alarm": store.AlarmLateNights})
	return s
}

func TestRetention(t *testing.T) {
	t.Parallel()
	s := retentionFixture(t)
	r, err := s.Retention(retentionNow, store.DefaultRetention())
	if err != nil {
		t.Fatal(err)
	}
	if want := [3]float64{5.0 / 7, 6.0 / 30, 7.0 / 90}; r.Load != want {
		t.Errorf("Load = %v, want %v", r.Load, want)
	}
	if len(r.Days) != 91 {
		t.Fatalf("%d days, want 91", len(r.Days))
	}
	practiced := 0
	for _, d := range r.Days {
		if d {
			practiced++
		}
	}
	// Today last: 10-15, 10-14, 10-13, 10-12 practiced, 10-11 not, 10-10 yes.
	if tail := r.Days[85:]; practiced != 7 || !tail[0] || tail[1] || !tail[2] || !tail[3] || !tail[4] || !tail[5] {
		t.Errorf("days: %d practiced, last six %v", practiced, tail)
	}

	mins := func(n float64) time.Duration { return time.Duration(n * float64(time.Minute)) }
	monday := func(m time.Month, d int) time.Time { return local(m, d, 0, 0) }
	wantWeeks := []store.Week{
		{Start: monday(8, 24)}, {Start: monday(8, 31)}, {Start: monday(9, 7)},
		{Start: monday(9, 14), Days: 1, Sessions: 1, Play: mins(210)},
		{Start: monday(9, 21)}, {Start: monday(9, 28)},
		{Start: monday(10, 5), Days: 1, Sessions: 1, Play: mins(17)},
		{Start: monday(10, 12), Days: 4, Sessions: 4, Play: mins(217)},
	}
	same(t, "weeks", r.Weeks, wantWeeks)

	// Sessions in 30 days: 210, 20, 10, 80, 70, 45 min; the gaps before
	// them 49d23h30m, 19d16h30m, 53h40m, 27h40m, 22h20m, 18h20m.
	if r.MedianSession != mins(57.5) {
		t.Errorf("median session %v, want 57m30s", r.MedianSession)
	}
	if want := 40*time.Hour + 40*time.Minute; r.MedianReturn != want {
		t.Errorf("median return %v, want %v", r.MedianReturn, want)
	}
	same(t, "replay", r.Replay, store.Ratio{N: 1, Of: 4})
	same(t, "upsolve", r.Upsolve, store.Ratio{N: 1, Of: 2})
	same(t, "late", r.Late, store.Ratio{N: 2, Of: 7})
	if r.LateWeek != 2 || r.OverBudgetWeek != 0 {
		t.Errorf("week: %d late, %d over budget; want 2, 0", r.LateWeek, r.OverBudgetWeek)
	}
	same(t, "worth", r.Worth, store.WorthIt{Yes: 2, No: 2, Skip: 1})
	same(t, "fortnights", r.Fortnights, [2]store.Fortnight{
		{Play: mins(210), Worth: store.WorthIt{Yes: 2}},
		{Play: mins(234), Worth: store.WorthIt{Yes: 2, No: 2, Skip: 1}},
	})
	same(t, "alarms", r.Alarms, []store.Alarm{
		{Name: store.AlarmWorthIt, Fresh: true},
		{Name: store.AlarmLateNights, Fresh: false}, // shown two days ago
		{Name: store.AlarmHoursVsWorth, Fresh: true},
	})
}

// TestLoadAverage: GRUB's line matches the panel, does not drop the morning
// after a practice day, and is absent for a player with none.
func TestLoadAverage(t *testing.T) {
	t.Parallel()
	s := retentionFixture(t)
	want := [3]float64{5.0 / 7, 6.0 / 30, 7.0 / 90}
	for _, now := range []time.Time{retentionNow, local(10, 16, 9, 0)} {
		got, ok, err := s.LoadAverage(now, store.DefaultRetention())
		if err != nil || !ok || got != want {
			t.Errorf("at %v: %v, %v, %v; want %v", now, got, ok, err, want)
		}
	}
	// Two days on with 10-16 unplayed: 10-16 is a day missed (the window
	// ends yesterday), and 10-09, unplayed too, leaves the 7 days.
	if got, _, _ := s.LoadAverage(local(10, 17, 9, 0), store.DefaultRetention()); got[0] != 5.0/7 {
		t.Errorf("after a missed day: %v", got)
	}
	if got, _, _ := s.LoadAverage(local(10, 18, 9, 0), store.DefaultRetention()); got[0] != 4.0/7 {
		t.Errorf("two missed days on: %v, want 4/7", got)
	}
	empty := open(t, tempPath(t))
	if _, ok, err := empty.LoadAverage(retentionNow, store.DefaultRetention()); ok || err != nil {
		t.Errorf("empty store: ok %v, %v", ok, err)
	}
}

// TestRetentionOverBudget trips the budget alarm with three long sessions
// in a week; an abandoned boot counts until its last checkpoint, not until
// the app marked it a day later. Two "no" answers are too few to alarm.
func TestRetentionOverBudget(t *testing.T) {
	t.Parallel()
	s := open(t, tempPath(t))
	now := retentionNow
	s.SetClock(func() time.Time { return now })
	m := game.Meta{Rating: 1200}
	for i, b := range []struct {
		start time.Time
		d     time.Duration
	}{
		{local(10, 9, 18, 0), 190 * time.Minute},
		{local(10, 11, 18, 0), 210 * time.Minute},
		{local(10, 13, 18, 0), 4 * time.Hour},
	} {
		checkpoint(t, s, m, endedBoot(i+1, b.start, b.d, false))
	}
	// Boot 4 is left running at 10-14 10:20, 20 min in; boot 5 abandons
	// it the next day.
	run := endedBoot(4, local(10, 14, 10, 0), 0, false)
	run.Phase, run.History = game.PhaseDoors, nil
	now = local(10, 14, 10, 20)
	checkpoint(t, s, m, run)
	now = local(10, 15, 18, 0)
	checkpoint(t, s, m, endedBoot(5, local(10, 15, 18, 0), 30*time.Minute, false))
	event(t, s, local(10, 14, 9, 0), store.EventWorthIt, map[string]any{"answer": "no"})
	event(t, s, local(10, 15, 9, 0), store.EventWorthIt, map[string]any{"answer": "no"})

	r, err := s.Retention(retentionNow, store.DefaultRetention())
	if err != nil {
		t.Fatal(err)
	}
	if r.OverBudgetWeek != 3 {
		t.Errorf("%d sessions over budget, want 3", r.OverBudgetWeek)
	}
	same(t, "alarms", r.Alarms, []store.Alarm{{Name: store.AlarmOverBudget, Fresh: true}})
	// 190, 210, 240, 20 (the abandoned boot), 30 min.
	if r.MedianSession != 190*time.Minute {
		t.Errorf("median session %v, want 3h10m", r.MedianSession)
	}
}
