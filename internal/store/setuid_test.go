package store_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/store"
)

// TestMigrateKeepsV3Rows: migration 4 only adds boots.ending. Every table
// and column of a schema-3 file reads back, row for row and value for
// value, and the rows from before have no ending.
func TestMigrateKeepsV3Rows(t *testing.T) {
	t.Parallel()
	path := tempPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", store.DSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer mustClose(t, db)
	if err := store.Migrate(db, store.Migrations[:3]); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	st := clearBoot(midBoot(3, t0))
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(sampleMeta())
	if err != nil {
		t.Fatal(err)
	}
	saved := t0.Add(time.Hour).UnixNano()
	exec(`INSERT INTO meta (id, json, saved) VALUES (1, ?, ?)`, meta, saved)
	exec(`INSERT INTO rating_history (at, rating) VALUES (?, ?)`, saved, 1040.5)
	exec(`INSERT INTO boots (id, seed, prestige, rules, started, ended, phase, outcome, cause, uptime,
		committed, last_door, state, saved, rules_hash, model)
		VALUES (3, ?, 1, 'standard', ?, ?, 'cleared', 'cleared', '', ?, 312, 'usaco:1060', ?, ?, 'f00dfeedf00dfeed', 'elo')`,
		int64(st.Seed), t0.UnixNano(), saved, int64(55*time.Minute), raw, saved)
	for i, a := range st.History {
		ar, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO attempts (boot, idx, problem, topic, rating, blind, mystery, cursed, regression,
			boss, opened, closed, outcome, submits, failed, quacks, best_pass, total, json,
			biome, p, p_base, slot, propensity, offer)
			VALUES (3, ?, ?, ?, ?, 0, 0, 0, 0, '', ?, ?, ?, 1, 0, 0, 10, 10, ?, 1, 0.9, 0.85, 'w100', 0.5, ?)`,
			i, a.Door.Problem, a.Door.Topic, a.Door.Rating, a.Opened.UnixNano(), a.Closed.UnixNano(), a.Outcome, ar, i)
	}
	exec(`INSERT INTO events (at, session, boot, attempt, problem, kind, json)
		VALUES (?, 1, 3, 0, 'usaco:987', 'samples_judged', '{"passed":2}')`, t0.UnixNano())
	exec(`INSERT INTO offers (boot, seq, kind, biome, room, at, policy, reason, set_prop, chosen, chosen_at, json)
		VALUES (3, 0, 'doors', 0, 0, ?, 'elo-window/1', 'fresh', 0.25, 1, ?, '{}')`, t0.UnixNano(), t0.UnixNano())

	tables := []string{"meta", "boots", "attempts", "rating_history", "offers", "events", "skill_history", "model_versions"}
	v3 := map[string][]string{}
	before := map[string][][]any{}
	for _, table := range tables {
		v3[table] = columns(t, db, table)
		before[table] = dumpRows(t, db, table, v3[table])
	}

	mustClose(t, open(t, path))
	if v := userVersion(t, db); v != len(store.Migrations) || v != 4 {
		t.Fatalf("user_version %d, want 4", v)
	}
	for _, table := range tables {
		same(t, table+" after migration 4", dumpRows(t, db, table, v3[table]), before[table])
	}
	var ending string
	if err := db.QueryRow(`SELECT ending FROM boots WHERE id = 3`).Scan(&ending); err != nil || ending != "" {
		t.Errorf("ending %q, %v; a boot from before has none", ending, err)
	}
}

// TestEnding: a boot's ending (game.State.Ending) is stored and listed; a
// wake is cleared, but no full clear and no best clear.
func TestEnding(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0)
	m := sampleMeta()
	full := clearBoot(midBoot(1, t0))
	wake := clearBoot(midBoot(2, t0.Add(2*time.Hour)))
	wake.Ending = game.EndingWake
	last := &wake.History[len(wake.History)-1]
	last.Closed = last.Opened.Add(5 * time.Minute) // a faster boot than the clear
	checkpoint(t, s, m, full)
	checkpoint(t, s, m, wake)
	boots, err := s.Boots(0)
	if err != nil || len(boots) != 2 {
		t.Fatalf("boots %+v, %v", boots, err)
	}
	if boots[0].Boot != 2 || boots[0].Ending != game.EndingWake || boots[0].Outcome != store.BootCleared || boots[1].Ending != "" {
		t.Errorf("summaries %+v", boots)
	}
	stats, err := s.Stats()
	if err != nil || stats.Boots != 2 || stats.FullClears != 1 {
		t.Errorf("stats %+v, %v: the wake is no full clear", stats, err)
	}
	// The wake is the faster boot; the best clear is still the clear's.
	best, ok, err := s.BestClear("standard", 1, 3)
	if err != nil || !ok || best != full.BootTime(t0) {
		t.Errorf("best clear %v, %v, %v; want the full clear's %v", best, ok, err, full.BootTime(t0))
	}
	run, ok, err := s.SeedBest(wake.Seed)
	if err != nil || !ok || run.Cleared {
		t.Errorf("the wake's seed %+v, %v, %v: a wake clears no seed", run, ok, err)
	}
}
