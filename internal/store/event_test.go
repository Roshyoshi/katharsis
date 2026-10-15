package store_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/store"
)

func TestEvents(t *testing.T) {
	t.Parallel()
	s := open(t, tempPath(t))
	idx := 2
	start, err := s.Event(store.Event{At: t0, Kind: store.EventAppStart, Data: map[string]any{"tmux": true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Event(store.Event{At: t0.Add(time.Minute), Session: start, Boot: 3, Attempt: &idx,
		Problem: "usaco:987", Kind: store.EventSamplesJudged,
		Data: map[string]any{"compiled": true, "passed": 2, "total": 3}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Event(store.Event{At: t0.Add(2 * time.Minute), Session: start, Kind: store.EventAppQuit}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Event(store.Event{At: t0}); err == nil {
		t.Error("an event without a kind was stored")
	}

	evs, err := s.Events(store.EventFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 3 {
		t.Fatalf("%d events, want 3: %+v", len(evs), evs)
	}
	// app_start is its own session; the rest name it.
	for _, e := range evs {
		if e.Session != start {
			t.Errorf("%s: session %d, want %d", e.Kind, e.Session, start)
		}
	}
	got := evs[1]
	if got.Boot != 3 || got.Attempt == nil || *got.Attempt != 2 || got.Problem != "usaco:987" ||
		!got.At.Equal(t0.Add(time.Minute)) {
		t.Errorf("samples_judged read back as %+v", got)
	}
	var data map[string]any
	if err := json.Unmarshal(got.Data.(json.RawMessage), &data); err != nil || data["passed"] != 2.0 {
		t.Errorf("payload %s, %v", got.Data, err)
	}
	if e := evs[2]; e.Boot != 0 || e.Attempt != nil || string(e.Data.(json.RawMessage)) != "{}" {
		t.Errorf("app_quit read back as %+v (%s)", e, e.Data)
	}

	quits, err := s.Events(store.EventFilter{Kinds: []string{store.EventAppQuit, store.EventAppStart},
		Since: t0.Add(time.Second)})
	if err != nil || len(quits) != 1 || quits[0].Kind != store.EventAppQuit {
		t.Errorf("filtered events %+v, %v", quits, err)
	}
}

// TestMigrateFromV1 opens a file a v1 build wrote, rows and all: migration
// 2 keeps every row, backfills the ML columns it can, and the store goes
// on writing to it.
func TestMigrateFromV1(t *testing.T) {
	t.Parallel()
	path := tempPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	meta, st := sampleMeta(), midBoot(3, t0)
	st.History[1].Biome = 1
	writeV1(t, path, meta, st)

	s := openClocked(t, path, t0.Add(50*time.Minute))
	if v := userVersion(t, s.DB()); v != len(store.Migrations) {
		t.Fatalf("user_version = %d, want %d", v, len(store.Migrations))
	}
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
	same(t, "attempts", recs, []store.AttemptRecord{
		{Boot: 3, Index: 1, Attempt: st.History[1]},
		{Boot: 3, Index: 0, Attempt: st.History[0]},
	})
	hist, err := s.RatingHistory(time.Time{})
	if err != nil || len(hist) != 1 || hist[0].Rating != meta.Rating {
		t.Errorf("rating history %+v, %v", hist, err)
	}

	// The backfill: p from SolveP, slot legacy, biome where the JSON has
	// one. v1's idx 0 lost its Biome key, as attempts saved before
	// Attempt.Biome did.
	cols := attemptCols(t, s.DB(), 3)
	want := []attemptML{
		{biome: sql.NullInt64{}, p: sql.NullFloat64{Float64: 0.9, Valid: true}, slot: "legacy"},
		{biome: sql.NullInt64{Int64: 1, Valid: true}, p: sql.NullFloat64{Float64: 0.9, Valid: true}, slot: "legacy"},
	}
	same(t, "ML columns", cols, want)
	for _, name := range []string{"offers", "events", "skill_history", "model_versions",
		"events_kind_at", "events_boot", "skill_history_topic_at"} {
		if !hasTable(t, s.DB(), name) {
			t.Errorf("migrated schema lacks %s", name)
		}
	}

	// And it goes on: the next checkpoint adds the open door's attempt
	// with the new columns filled, and an event lands.
	crash(st, game.OutOfLives)
	st.History[2].Biome = 1
	checkpoint(t, s, meta, st)
	cols = attemptCols(t, s.DB(), 3)
	if len(cols) != 3 || cols[2] != (attemptML{biome: sql.NullInt64{Int64: 1, Valid: true},
		p: sql.NullFloat64{Float64: 0.7, Valid: true}, slot: "legacy"}) {
		t.Errorf("attempt after migration: %+v", cols)
	}
	var hash string
	if err := s.DB().QueryRow(`SELECT rules_hash FROM boots WHERE id = 3`).Scan(&hash); err != nil || len(hash) != 16 {
		t.Errorf("rules_hash %q, %v", hash, err)
	}
	if _, err := s.Event(store.Event{At: t0, Kind: store.EventAppStart}); err != nil {
		t.Errorf("event on a migrated file: %v", err)
	}
}

// TestMigrateKeepsV1Rows: migration 2 only adds. Every column a v1 table
// had reads back, row for row and value for value, from the migrated
// file, so the first launch of a v2 build loses nothing a v1 build saved.
func TestMigrateKeepsV1Rows(t *testing.T) {
	t.Parallel()
	path := tempPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	st := midBoot(3, t0)
	st.History[1].Biome = 1
	writeV1(t, path, sampleMeta(), st)

	db, err := sql.Open("sqlite", store.DSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer mustClose(t, db)
	v1 := map[string][]string{} // table -> its v1 columns
	for _, table := range []string{"meta", "boots", "attempts", "rating_history"} {
		v1[table] = columns(t, db, table)
	}
	before := map[string][][]any{}
	for table, cols := range v1 {
		before[table] = dumpRows(t, db, table, cols)
		if len(before[table]) == 0 {
			t.Fatalf("v1 fixture wrote no %s rows", table)
		}
	}

	mustClose(t, open(t, path))
	for table, cols := range v1 {
		same(t, table+" after migration", dumpRows(t, db, table, cols), before[table])
	}
}

func columns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// dumpRows reads cols of every row of table, in rowid order, as the
// driver returns them.
func dumpRows(t *testing.T, db *sql.DB, table string, cols []string) [][]any {
	t.Helper()
	rows, err := db.Query(`SELECT ` + strings.Join(cols, ", ") + ` FROM ` + table + ` ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		row := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

type attemptML struct {
	biome sql.NullInt64
	p     sql.NullFloat64
	slot  string
}

func attemptCols(t *testing.T, db *sql.DB, boot int) []attemptML {
	t.Helper()
	rows, err := db.Query(`SELECT biome, p, slot FROM attempts WHERE boot = ? ORDER BY idx`, boot)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []attemptML
	for rows.Next() {
		var a attemptML
		if err := rows.Scan(&a.biome, &a.p, &a.slot); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// writeV1 writes meta and st the way a v1 build's Checkpoint did, onto a
// file migrated to v1 only. History[0] is saved without its Biome key, as
// attempts were before Attempt.Biome.
func writeV1(t *testing.T, path string, meta game.Meta, st *game.State) {
	t.Helper()
	db, err := sql.Open("sqlite", store.DSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer mustClose(t, db)
	if err := store.Migrate(db, store.Migrations[:1]); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustJSON := func(v any) []byte {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	saved := t0.Add(45 * time.Minute).UnixNano()
	exec(`INSERT INTO meta (id, json, saved) VALUES (1, ?, ?)`, mustJSON(meta), saved)
	exec(`INSERT INTO rating_history (at, rating) VALUES (?, ?)`, saved, meta.Rating)
	exec(`INSERT INTO boots (id, seed, prestige, rules, started, ended, phase, outcome,
		cause, uptime, committed, last_door, state, saved)
		VALUES (?, ?, ?, ?, ?, NULL, ?, '', '', ?, ?, ?, ?, ?)`,
		st.Boot, int64(st.Seed), st.Prestige, st.Rules.Name, st.Started.UnixNano(),
		st.Phase, int64(45*time.Minute), st.Committed, st.Current.Door.Problem, mustJSON(st), saved)
	for i, a := range st.History {
		raw := mustJSON(a)
		if i == 0 {
			var v map[string]any
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatal(err)
			}
			delete(v, "Biome")
			raw = mustJSON(v)
		}
		d := a.Door
		exec(`INSERT INTO attempts (boot, idx, problem, topic, rating, blind, mystery,
			cursed, regression, boss, opened, closed, outcome, submits, failed, quacks,
			best_pass, total, json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			st.Boot, i, d.Problem, d.Topic, d.Rating, d.Blind, d.Mystery, d.Cursed,
			d.Regression, d.Boss, a.Opened.UnixNano(), a.Closed.UnixNano(), a.Outcome,
			a.Submits, a.Failed, a.Quacks, a.BestPass, a.Total, raw)
	}
}
