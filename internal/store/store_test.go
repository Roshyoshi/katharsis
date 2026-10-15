package store_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/store"
)

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	tests := []struct {
		name, xdg, want string
	}{
		{"xdg set", "/xdg/data", "/xdg/data/katharsis/katharsis.db"},
		{"xdg unset", "", filepath.Join(home, ".local/share/katharsis/katharsis.db")},
		{"relative xdg ignored", "rel/data", filepath.Join(home, ".local/share/katharsis/katharsis.db")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", tc.xdg)
			got, err := store.DefaultPath()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("DefaultPath = %s, want %s", got, tc.want)
			}
		})
	}
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func hasTable(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, name).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func TestMigrations(t *testing.T) {
	t.Parallel()

	t.Run("from empty", func(t *testing.T) {
		t.Parallel()
		s := open(t, tempPath(t))
		if v := userVersion(t, s.DB()); v != len(store.Migrations) {
			t.Errorf("user_version = %d, want %d", v, len(store.Migrations))
		}
		for _, name := range []string{"meta", "boots", "attempts", "rating_history",
			"attempts_problem", "attempts_closed", "rating_history_at"} {
			if !hasTable(t, s.DB(), name) {
				t.Errorf("schema lacks %s", name)
			}
		}
		var mode string
		if err := s.DB().QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
			t.Errorf("journal_mode = %q, %v; want wal", mode, err)
		}
		var fk int
		if err := s.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
			t.Errorf("foreign_keys = %d, %v; want 1", fk, err)
		}
	})

	t.Run("reopen is idempotent", func(t *testing.T) {
		t.Parallel()
		path := tempPath(t)
		s := open(t, path)
		checkpoint(t, s, sampleMeta(), midBoot(3, t0))
		mustClose(t, s)
		for range 2 {
			s := open(t, path)
			if v := userVersion(t, s.DB()); v != len(store.Migrations) {
				t.Errorf("user_version = %d after reopen", v)
			}
			m, err := s.LoadMeta()
			if err != nil {
				t.Fatal(err)
			}
			same(t, "meta after reopen", m, sampleMeta())
			mustClose(t, s)
		}
	})

	t.Run("newer schema refused", func(t *testing.T) {
		t.Parallel()
		path := tempPath(t)
		s := open(t, path)
		if _, err := s.DB().Exec(`PRAGMA user_version = 99`); err != nil {
			t.Fatal(err)
		}
		mustClose(t, s)
		if s, err := store.Open(path); !errors.Is(err, store.ErrNewerSchema) {
			if s != nil {
				mustClose(t, s)
			}
			t.Fatalf("Open = %v, want ErrNewerSchema", err)
		}
	})

	t.Run("failed step rolls back whole", func(t *testing.T) {
		t.Parallel()
		path := tempPath(t)
		mustClose(t, open(t, path))
		db, err := sql.Open("sqlite", store.DSN(path))
		if err != nil {
			t.Fatal(err)
		}
		defer mustClose(t, db)
		// The second statement fails after the first has run: the step's
		// transaction must take the CREATE back with it.
		steps := append(slices.Clone(store.Migrations),
			`CREATE TABLE extra (x INTEGER); INSERT INTO missing VALUES (1);`)
		if err := store.Migrate(db, steps); err == nil {
			t.Fatal("Migrate succeeded on a broken step")
		}
		if v := userVersion(t, db); v != len(store.Migrations) {
			t.Errorf("user_version = %d, want %d", v, len(store.Migrations))
		}
		if hasTable(t, db, "extra") {
			t.Error("table from the failed step survived")
		}
	})
}

// TestOpenOddPath: the DSN is a file: URI, so characters that mean something
// in a URI must reach SQLite escaped, not split the path or start a query.
func TestOpenOddPath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "a b?c#d%20e", "dead cases.db")
	s := open(t, path)
	checkpoint(t, s, sampleMeta(), midBoot(1, t0))
	mustClose(t, s)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not at the requested path: %v", err)
	}
	if st, err := open(t, path).ActiveBoot(); err != nil || st == nil {
		t.Fatalf("reopen: ActiveBoot = %v, %v", st, err)
	}
}

// TestOpenPrivateDirs: XDG asks for 0700 on the data directories it has to
// create, and the game leaves creating them to Open.
func TestOpenPrivateDirs(t *testing.T) {
	t.Parallel()
	path := tempPath(t) // .../data/katharsis/katharsis.db, neither dir there
	open(t, path)
	for _, dir := range []string{filepath.Dir(path), filepath.Dir(filepath.Dir(path))} {
		fi, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if perm := fi.Mode().Perm(); perm != 0o700 {
			t.Errorf("%s is %o, want 700", dir, perm)
		}
	}
}

func TestLoadMetaEmpty(t *testing.T) {
	t.Parallel()
	s := open(t, tempPath(t))
	m, err := s.LoadMeta()
	if err != nil {
		t.Fatal(err)
	}
	want := game.Meta{Rating: rules.Standard().ColdStartRating}
	same(t, "empty meta", m, want)
	if st, err := s.ActiveBoot(); st != nil || err != nil {
		t.Errorf("ActiveBoot = %v, %v; want nil, nil", st, err)
	}
}

func TestCheckpointRoundTrip(t *testing.T) {
	t.Parallel()
	path := tempPath(t)
	s := openClocked(t, path, t0.Add(45*time.Minute))
	meta, st := sampleMeta(), midBoot(3, t0)
	checkpoint(t, s, meta, st)
	mustClose(t, s)

	// A fresh process resumes from the file alone.
	s = open(t, path)
	gotMeta, err := s.LoadMeta()
	if err != nil {
		t.Fatal(err)
	}
	same(t, "meta", gotMeta, meta)
	gotState, err := s.ActiveBoot()
	if err != nil {
		t.Fatal(err)
	}
	same(t, "state", gotState, st)

	recs, err := s.Attempts(store.AttemptFilter{Boot: 3})
	if err != nil {
		t.Fatal(err)
	}
	wantRecs := []store.AttemptRecord{
		{Boot: 3, Index: 1, Attempt: st.History[1]},
		{Boot: 3, Index: 0, Attempt: st.History[0]},
	}
	same(t, "attempts", recs, wantRecs)

	boots, err := s.Boots(0)
	if err != nil {
		t.Fatal(err)
	}
	wantBoots := []store.BootSummary{{
		Boot: 3, Seed: st.Seed, Prestige: 1, Rules: "standard", Started: t0,
		Outcome: store.BootRunning,
		// The clock's first reading: 9 + 14 min of closed doors, 5m1s into
		// the open one.
		Uptime:   28*time.Minute + time.Second,
		LastDoor: "usaco:1060",
	}}
	same(t, "boots", boots, wantBoots)
}

func TestAttemptsRecordedOnce(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0)
	meta, st := sampleMeta(), midBoot(1, t0)
	for range 3 {
		checkpoint(t, s, meta, st)
	}
	crash(st, game.OutOfLives)
	for range 3 {
		checkpoint(t, s, meta, st)
	}
	recs, err := s.Attempts(store.AttemptFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var idx []int
	for _, r := range recs {
		idx = append(idx, r.Index)
	}
	if !slices.Equal(idx, []int{2, 1, 0}) {
		t.Fatalf("attempt indexes = %v, want [2 1 0]", idx)
	}
	same(t, "crashing attempt", recs[0].Attempt, st.History[2])
}

func TestAttemptFilter(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0)
	meta := sampleMeta()
	b1 := crash(midBoot(1, t0), game.TimedOut)
	b2 := midBoot(2, t0.Add(24*time.Hour))
	checkpoint(t, s, meta, b1)
	checkpoint(t, s, meta, b2)

	type key struct{ boot, idx int }
	tests := []struct {
		name string
		f    store.AttemptFilter
		want []key
	}{
		{"all, newest first", store.AttemptFilter{}, []key{{2, 1}, {2, 0}, {1, 2}, {1, 1}, {1, 0}}},
		{"one boot", store.AttemptFilter{Boot: 1}, []key{{1, 2}, {1, 1}, {1, 0}}},
		{"one problem", store.AttemptFilter{Problem: "usaco:987"}, []key{{2, 0}, {1, 0}}},
		{"limit", store.AttemptFilter{Limit: 2}, []key{{2, 1}, {2, 0}}},
		{"combined", store.AttemptFilter{Boot: 1, Problem: catalog.ID("usaco:1060")}, []key{{1, 2}}},
		{"no match", store.AttemptFilter{Problem: "cses:1"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recs, err := s.Attempts(tc.f)
			if err != nil {
				t.Fatal(err)
			}
			var got []key
			for _, r := range recs {
				got = append(got, key{r.Boot, r.Index})
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("Attempts(%+v) = %v, want %v", tc.f, got, tc.want)
			}
		})
	}
}

func TestActiveBoot(t *testing.T) {
	t.Parallel()
	meta := sampleMeta()
	day := func(n int) time.Time { return t0.Add(time.Duration(n) * 24 * time.Hour) }
	tests := []struct {
		name     string
		run      func(s *store.Store) error
		want     int // boot to resume, 0 for none
		outcomes []store.BootOutcome
	}{
		{"none", func(s *store.Store) error { return nil }, 0, nil},
		{"running", func(s *store.Store) error {
			return s.Checkpoint(meta, midBoot(1, day(1)))
		}, 1, []store.BootOutcome{store.BootRunning}},
		{"crashed", func(s *store.Store) error {
			return s.Checkpoint(meta, crash(midBoot(1, day(1)), game.TimedOut))
		}, 0, []store.BootOutcome{store.BootCrashed}},
		{"cleared", func(s *store.Store) error {
			return s.Checkpoint(meta, clearBoot(midBoot(1, day(1))))
		}, 0, []store.BootOutcome{store.BootCleared}},
		{"abandoned", func(s *store.Store) error {
			if err := s.Checkpoint(meta, midBoot(1, day(1))); err != nil {
				return err
			}
			return s.AbandonBoot(1)
		}, 0, []store.BootOutcome{store.BootAbandoned}},
		{"newer boot supersedes a running one", func(s *store.Store) error {
			if err := s.Checkpoint(meta, midBoot(1, day(1))); err != nil {
				return err
			}
			return s.Checkpoint(meta, midBoot(2, day(2)))
		}, 2, []store.BootOutcome{store.BootRunning, store.BootAbandoned}},
		{"crash after an earlier clear", func(s *store.Store) error {
			if err := s.Checkpoint(meta, clearBoot(midBoot(1, day(1)))); err != nil {
				return err
			}
			if err := s.Checkpoint(meta, midBoot(2, day(2))); err != nil {
				return err
			}
			return s.Checkpoint(meta, crash(midBoot(2, day(2)), game.GaveUp))
		}, 0, []store.BootOutcome{store.BootCrashed, store.BootCleared}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := openClocked(t, tempPath(t), t0)
			if err := tc.run(s); err != nil {
				t.Fatal(err)
			}
			st, err := s.ActiveBoot()
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tc.want == 0 && st != nil:
				t.Errorf("ActiveBoot = boot %d, want none", st.Boot)
			case tc.want != 0 && (st == nil || st.Boot != tc.want):
				t.Errorf("ActiveBoot = %v, want boot %d", st, tc.want)
			}
			boots, err := s.Boots(0)
			if err != nil {
				t.Fatal(err)
			}
			ends := endedAt(t, s)
			var got []store.BootOutcome
			for _, b := range boots {
				got = append(got, b.Outcome)
				if (b.Outcome == store.BootRunning) != (ends[b.Boot] == 0) {
					t.Errorf("boot %d: outcome %q but ended %d", b.Boot, b.Outcome, ends[b.Boot])
				}
			}
			if !slices.Equal(got, tc.outcomes) {
				t.Errorf("outcomes newest first = %q, want %q", got, tc.outcomes)
			}
		})
	}
}

func TestBootGuards(t *testing.T) {
	t.Parallel()
	meta := sampleMeta()
	crashed := func() *game.State { return crash(midBoot(1, t0), game.TimedOut) }
	tests := []struct {
		name    string
		setup   *game.State // checkpointed first, unless nil
		abandon bool        // then boot 1 is abandoned
		act     func(s *store.Store) error
		want    error // nil: must succeed
	}{
		{"abandon unknown boot", nil, false,
			func(s *store.Store) error { return s.AbandonBoot(7) }, store.ErrNoBoot},
		{"abandon twice", midBoot(1, t0), true,
			func(s *store.Store) error { return s.AbandonBoot(1) }, nil},
		{"abandon crashed", crashed(), false,
			func(s *store.Store) error { return s.AbandonBoot(1) }, nil},
		{"resume abandoned", midBoot(1, t0), true,
			func(s *store.Store) error { return s.Checkpoint(meta, midBoot(1, t0)) }, store.ErrBootEnded},
		{"resume crashed", crashed(), false,
			func(s *store.Store) error { return s.Checkpoint(meta, midBoot(1, t0)) }, store.ErrBootEnded},
		{"clear crashed", crashed(), false,
			func(s *store.Store) error { return s.Checkpoint(meta, clearBoot(midBoot(1, t0))) }, store.ErrBootEnded},
		{"re-checkpoint crashed", crashed(), false,
			func(s *store.Store) error { return s.Checkpoint(meta, crashed()) }, nil},
		{"boot id reused by another seed", crashed(), false,
			func(s *store.Store) error {
				st := midBoot(1, t0)
				st.Seed++
				return s.Checkpoint(meta, st)
			}, store.ErrBootReused},
		{"boot id reused later", crashed(), false,
			func(s *store.Store) error { return s.Checkpoint(meta, midBoot(1, t0.Add(time.Hour))) },
			store.ErrBootReused},
		{"boot 0", nil, false,
			func(s *store.Store) error { return s.Checkpoint(meta, midBoot(0, t0)) }, store.ErrInvalidBoot},
		{"nil state", nil, false,
			func(s *store.Store) error { return s.Checkpoint(meta, nil) }, store.ErrInvalidBoot},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := openClocked(t, tempPath(t), t0)
			if tc.setup != nil {
				checkpoint(t, s, meta, tc.setup)
			}
			if tc.abandon {
				if err := s.AbandonBoot(1); err != nil {
					t.Fatal(err)
				}
			}
			if err := tc.act(s); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("abandon keeps how a crash ended", func(t *testing.T) {
		t.Parallel()
		s := openClocked(t, tempPath(t), t0)
		checkpoint(t, s, meta, crashed())
		if err := s.AbandonBoot(1); err != nil {
			t.Fatal(err)
		}
		boots, err := s.Boots(1)
		if err != nil {
			t.Fatal(err)
		}
		if boots[0].Outcome != store.BootCrashed || boots[0].Cause != game.TimedOut {
			t.Errorf("boot 1 = %s/%s after abandon, want crashed/timed-out",
				boots[0].Outcome, boots[0].Cause)
		}
	})
}

func TestBootSummaryEnds(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0.Add(48*time.Hour))
	meta := sampleMeta()
	crashed := crash(midBoot(1, t0), game.OutOfLives)
	cleared := clearBoot(midBoot(2, t0.Add(24*time.Hour)))
	checkpoint(t, s, meta, crashed)
	checkpoint(t, s, meta, cleared)
	// A later checkpoint of an ended boot must not move its end.
	crashed.Log = append(crashed.Log, game.LogLine{At: t0.Add(72 * time.Hour), Text: "core dumped"})
	checkpoint(t, s, meta, crashed)

	boots, err := s.Boots(0)
	if err != nil {
		t.Fatal(err)
	}
	clearedEnd := cleared.History[len(cleared.History)-1].Closed
	want := []store.BootSummary{
		{
			Boot: 2, Seed: cleared.Seed, Prestige: 1, Rules: "standard",
			Started: cleared.Started, Outcome: store.BootCleared,
			Uptime:    48 * time.Minute, // 9 + 14 + 25 min of doors
			Committed: 120, LastDoor: "usaco:1060",
		},
		{
			Boot: 1, Seed: crashed.Seed, Prestige: 1, Rules: "standard",
			Started: t0, Outcome: store.BootCrashed, Cause: game.OutOfLives,
			Uptime: crashed.Crash.Uptime, Dumped: 120, LastDoor: "usaco:1060",
		},
	}
	same(t, "boots", boots, want)
	same(t, "ended", endedAt(t, s), map[int]int64{
		1: crashed.Crash.At.UnixNano(),
		2: clearedEnd.UnixNano(),
	})
	if one, _ := s.Boots(1); len(one) != 1 || one[0].Boot != 2 {
		t.Errorf("Boots(1) = %+v, want only boot 2", one)
	}
}

// A crashed trial loses its bytes without a core dump, so git log shows
// nothing dumped; a crashed boot shows what went into the dump.
func TestBootSummaryDumped(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0.Add(48*time.Hour))
	meta := sampleMeta()
	boot := crash(midBoot(1, t0), game.TimedOut)
	trial := midBoot(2, t0.Add(time.Hour))
	trial.Trial = &game.TrialState{TrialSpec: game.TrialSpec{Module: "prefix-sums"}}
	crash(trial, game.TimedOut)
	checkpoint(t, s, meta, boot)
	checkpoint(t, s, meta, trial)

	boots, err := s.Boots(0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]int64{}
	for _, b := range boots {
		got[b.Boot] = b.Dumped
	}
	same(t, "dumped", got, map[int]int64{1: 120, 2: 0})
}

// endedAt reads each boot's ended column, 0 while it runs. No query
// returns it, but the file keeps it as the boot's record.
func endedAt(t *testing.T, s *store.Store) map[int]int64 {
	t.Helper()
	rows, err := s.DB().Query(`SELECT id, COALESCE(ended, 0) FROM boots`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ends := map[int]int64{}
	for rows.Next() {
		var id int
		var at int64
		if err := rows.Scan(&id, &at); err != nil {
			t.Fatal(err)
		}
		ends[id] = at
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ends
}

func TestRatingHistory(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0)
	meta, st := sampleMeta(), midBoot(1, t0)
	var wantAt []time.Time
	save := func(r float64, inBoot bool) time.Time {
		meta.Rating = r
		var err error
		if inBoot {
			err = s.Checkpoint(meta, st)
		} else {
			err = s.SaveMeta(meta)
		}
		if err != nil {
			t.Fatal(err)
		}
		pts, _ := s.RatingHistory(time.Time{})
		return pts[len(pts)-1].At
	}
	for _, step := range []struct {
		rating float64
		inBoot bool
		moved  bool
	}{
		{1000, true, true},
		{1000, true, false},
		{1040, true, true},
		{1040, false, false},
		{1012.5, false, true},
		{1012.5, true, false},
	} {
		at := save(step.rating, step.inBoot)
		if step.moved {
			wantAt = append(wantAt, at)
		}
	}
	pts, err := s.RatingHistory(time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := []store.RatingPoint{{wantAt[0], 1000}, {wantAt[1], 1040}, {wantAt[2], 1012.5}}
	same(t, "rating history", pts, want)

	tail, err := s.RatingHistory(wantAt[1])
	if err != nil {
		t.Fatal(err)
	}
	same(t, "rating history since", tail, want[1:])
}

func TestStats(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0.Add(10*24*time.Hour))
	meta := sampleMeta()
	day := func(n int) time.Time { return t0.Add(time.Duration(n) * 24 * time.Hour) }
	checkpoint(t, s, meta, crash(midBoot(1, day(1)), game.TimedOut))
	checkpoint(t, s, meta, crash(midBoot(2, day(2)), game.OutOfLives))
	checkpoint(t, s, meta, clearBoot(midBoot(3, day(3))))
	checkpoint(t, s, meta, crash(midBoot(4, day(4)), game.TimedOut))
	checkpoint(t, s, meta, midBoot(5, day(5)))
	if err := s.AbandonBoot(5); err != nil {
		t.Fatal(err)
	}
	checkpoint(t, s, meta, midBoot(6, day(6)))

	got, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	boots, _ := s.Boots(0)
	var uptime time.Duration
	for _, b := range boots {
		uptime += b.Uptime
	}
	want := store.Stats{
		Boots:      6,
		FullClears: 1,
		Crashes:    map[game.Outcome]int{game.TimedOut: 2, game.OutOfLives: 1},
		Solves:     map[game.SolveKind]int{game.Independent: 1, game.Hinted: 1, game.Editorial: 1},
		Topics: map[string]store.TopicStats{
			"simulation":  {Attempts: 12, Clears: 12},
			"prefix-sums": {Attempts: 4, Clears: 1},
		},
		Uptime: uptime,
	}
	same(t, "stats", got, want)

	empty, err := open(t, tempPath(t)).Stats()
	if err != nil {
		t.Fatal(err)
	}
	same(t, "empty stats", empty, store.Stats{
		Crashes: map[game.Outcome]int{},
		Solves:  map[game.SolveKind]int{},
		Topics:  map[string]store.TopicStats{},
	})
}
