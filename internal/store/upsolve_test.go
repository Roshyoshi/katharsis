package store_test

import (
	"reflect"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/store"
)

// Upsolves pairs each upsolve's events into one record, newest first: a
// submit after the AC is listed and the solve stays independent; a glance
// is left out; an editorial read before the AC makes it editorial; an app
// killed mid-upsolve leaves it at its last verdict; verdicts whose open was
// lost are dropped; a solve from before events began is listed once. The
// calendar counts what git log lists.
func TestUpsolves(t *testing.T) {
	t.Parallel()
	s := open(t, tempPath(t))
	m := game.Meta{Rating: 1200, Solves: []game.Solve{
		{Problem: "cf:9Z", Kind: game.Hinted, At: local(10, 10, 20, 12), Elapsed: 12 * time.Minute, Upsolve: true},
		{Problem: "cf:1A", Kind: game.Independent, At: local(10, 14, 12, 3), Elapsed: 3 * time.Minute, Upsolve: true},
	}}
	if err := s.SaveMeta(m); err != nil {
		t.Fatal(err)
	}
	ms := func(d time.Duration) int64 { return d.Milliseconds() }
	var session int64
	emit := func(at time.Time, kind string, problem catalog.ID, data map[string]any) {
		t.Helper()
		id, err := s.Event(store.Event{At: at, Session: session, Kind: kind, Problem: problem, Data: data})
		if err != nil {
			t.Fatal(err)
		}
		if kind == store.EventAppStart {
			session = id
		}
	}
	t0 := local(10, 14, 12, 0)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	emit(t0, store.EventAppStart, "", nil)
	emit(at(0), store.EventUpsolveOpen, "cf:1A", map[string]any{"editorial": false})
	emit(at(1), store.EventUpsolveJudged, "cf:1A", map[string]any{"verdict": "WA", "passed": 2, "total": 10, "editorial": false, "elapsed_ms": ms(time.Minute)})
	emit(at(3), store.EventUpsolveJudged, "cf:1A", map[string]any{"verdict": "AC", "passed": 10, "total": 10, "editorial": false, "elapsed_ms": ms(3 * time.Minute)})
	emit(at(4), store.EventUpsolveJudged, "cf:1A", map[string]any{"verdict": "WA", "passed": 9, "total": 10, "editorial": false, "elapsed_ms": ms(4 * time.Minute)})
	emit(at(5), store.EventUpsolveClose, "cf:1A", map[string]any{"verdict": "WA", "editorial": false, "elapsed_ms": ms(5 * time.Minute), "recovered": 64})
	emit(at(6), store.EventUpsolveOpen, "cf:2B", map[string]any{"editorial": false})
	emit(at(6).Add(20*time.Second), store.EventUpsolveClose, "cf:2B", map[string]any{"verdict": "", "editorial": false, "elapsed_ms": ms(20 * time.Second)})
	emit(at(7), store.EventUpsolveOpen, "cf:3C", map[string]any{"editorial": true})
	emit(at(9), store.EventUpsolveJudged, "cf:3C", map[string]any{"verdict": "AC", "passed": 5, "total": 5, "editorial": true, "elapsed_ms": ms(2 * time.Minute)})
	emit(at(9), store.EventUpsolveClose, "cf:3C", map[string]any{"verdict": "AC", "editorial": true, "elapsed_ms": ms(2 * time.Minute)})
	emit(at(10), store.EventUpsolveOpen, "cf:4D", map[string]any{"editorial": false})
	emit(at(12), store.EventUpsolveJudged, "cf:4D", map[string]any{"verdict": "TLE", "passed": 3, "total": 8, "editorial": false, "elapsed_ms": ms(2 * time.Minute)})
	// The app dies; the next session's first events are a verdict whose
	// open was lost, then an upsolve left after three minutes.
	emit(at(30), store.EventAppStart, "", nil)
	emit(at(31), store.EventUpsolveJudged, "cf:4D", map[string]any{"verdict": "AC", "passed": 8, "total": 8, "editorial": false, "elapsed_ms": ms(21 * time.Minute)})
	emit(at(32), store.EventUpsolveOpen, "cf:5E", map[string]any{"editorial": false})
	emit(at(35), store.EventUpsolveClose, "cf:5E", map[string]any{"verdict": "", "editorial": false, "elapsed_ms": ms(3 * time.Minute)})

	got, err := s.Upsolves()
	if err != nil {
		t.Fatal(err)
	}
	want := []store.UpsolveRecord{
		{Problem: "cf:5E", Opened: at(32), Elapsed: 3 * time.Minute},
		{Problem: "cf:4D", Opened: at(10), Elapsed: 2 * time.Minute,
			Verdicts: []store.UpsolveVerdict{{"TLE", 3, 8, 2 * time.Minute, false}}},
		{Problem: "cf:3C", Opened: at(7), Elapsed: 2 * time.Minute, Kind: game.Editorial, Editorial: true,
			Verdicts: []store.UpsolveVerdict{{"AC", 5, 5, 2 * time.Minute, true}}},
		{Problem: "cf:1A", Opened: at(0), Elapsed: 5 * time.Minute, Kind: game.Independent, Recovered: 64,
			Verdicts: []store.UpsolveVerdict{
				{"WA", 2, 10, time.Minute, false}, {"AC", 10, 10, 3 * time.Minute, false}, {"WA", 9, 10, 4 * time.Minute, false}}},
		{Problem: "cf:9Z", Opened: local(10, 10, 20, 0), Elapsed: 12 * time.Minute, Kind: game.Hinted},
	}
	if len(got) != len(want) {
		t.Fatalf("%d upsolves, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		got[i].Opened = got[i].Opened.In(want[i].Opened.Location())
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("upsolve %d\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}

	// cf:1A, cf:3C and cf:5E closed on 10-14; cf:2B's glance did too, and
	// its 20s are in the day's time, but it is not counted.
	days, err := s.Days(local(10, 14, 0, 0), local(10, 14, 23, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 || days[0].Upsolves != 3 || days[0].Door != (5+2+3)*time.Minute+20*time.Second {
		t.Errorf("Days = %+v, want 3 upsolves and 10m20s", days)
	}
}
