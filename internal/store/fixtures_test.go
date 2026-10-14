package store_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/store"
)

// Fixtures are built by hand rather than by the engine, so the store's
// tests pin the storage contract and not engine behavior.

var t0 = time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)

// clock ticks one second per reading, so every write gets a distinct,
// predictable timestamp.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(time.Second)
	return c.t
}

func tempPath(t *testing.T) string {
	// A missing parent directory checks that Open creates it.
	return filepath.Join(t.TempDir(), "data", "katharsis", "katharsis.db")
}

func open(t *testing.T, path string) *store.Store {
	t.Helper()
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// openClocked opens with a fake clock starting at start.
func openClocked(t *testing.T, path string, start time.Time) *store.Store {
	s := open(t, path)
	s.SetClock((&clock{t: start}).Now)
	return s
}

func sampleMeta() game.Meta {
	return game.Meta{
		Rating:        1040.5,
		TopicRating:   map[string]float64{"simulation": 1100, "prefix-sums": 980.25},
		Prestige:      1,
		Bank:          312,
		LifetimeBytes: 4096,
		Capabilities:  []string{"CAP_PREFIX"},
		Libs:          []game.Lib{{Name: "dsu", Source: "struct DSU {\n\tvector<int> p;\n};\n"}},
		CoreDump:      &game.CoreDump{Problem: "usaco:1012", Bytes: 96, Boot: 2, At: t0.Add(-48 * time.Hour)},
		Regressions:   []game.Regression{{Problem: "usaco:1012", Due: t0.Add(24 * time.Hour)}},
		Solves: []game.Solve{
			{Problem: "usaco:987", Kind: game.Independent, At: t0.Add(10 * time.Minute), Elapsed: 9 * time.Minute, Boot: 3},
			{Problem: "usaco:1011", Kind: game.Hinted, At: t0.Add(35 * time.Minute), Elapsed: 24 * time.Minute, Boot: 3},
			{Problem: "usaco:1012", Kind: game.Editorial, At: t0.Add(-47 * time.Hour), Elapsed: time.Hour, Boot: 0},
		},
		Boots:         3,
		TopicAttempts: map[string]int{"simulation": 6, "prefix-sums": 2},
	}
}

func door(id catalog.ID, topic string, rating int, timer time.Duration) game.Door {
	return game.Door{Problem: id, Topic: topic, Rating: rating, Timer: timer, Bytes: 48, SolveP: 0.9}
}

// midBoot is boot n, started at start, with two cleared doors in Userland
// and a third open in a choice biome: the state an interrupted boot leaves.
func midBoot(n int, start time.Time) *game.State {
	d1 := door("usaco:987", "simulation", 900, 15*time.Minute)
	d2 := door("usaco:1011", "simulation", 1000, 15*time.Minute)
	d2.Blind, d2.Bytes = true, 72
	d3 := door("usaco:1060", "prefix-sums", 1250, 30*time.Minute)
	d3.Blind, d3.Mystery, d3.Bytes, d3.SolveP = true, true, 192, 0.7
	at := func(m int) time.Time { return start.Add(time.Duration(m) * time.Minute) }
	return &game.State{
		Boot: n,
		// High bits set: Seed must survive the signed INTEGER column, and
		// both must survive JSON exactly (the PRNG resumes from RNG).
		Seed:     0x8f3a_0000_dead_beef + uint64(n),
		RNG:      0xfeed_face_cafe_beef,
		Rules:    rules.Standard(),
		Prestige: 1,
		Started:  start,
		Phase:    game.PhaseProblem,
		Biomes: []game.BiomeState{
			{Name: "Userland", Topic: "simulation", Kind: rules.Fixed, Rooms: 2},
			{Name: "Prefix Sums", Topic: "prefix-sums", Kind: rules.Choice, Rooms: 2, Offers: []string{"prefix-sums", "graphs"}},
			{Name: "Off-by-One", Topic: "mixed", Kind: rules.Boss, Rooms: 1},
		},
		Biome: 1,
		Current: &game.Attempt{
			Door: d3, Opened: at(40), Deadline: at(70),
			Submits: 1, Failed: 1, Quacks: 1, BestPass: 37, Total: 52, LastVerdict: "WA",
		},
		History: []game.Attempt{
			{Door: d1, Opened: at(1), Deadline: at(16), Closed: at(10), Outcome: game.Cleared,
				Submits: 1, BestPass: 10, Total: 10, LastVerdict: "AC"},
			{Door: d2, Opened: at(11), Deadline: at(26), Closed: at(25), Outcome: game.Cleared,
				Submits: 2, Failed: 1, BestPass: 15, Total: 15, LastVerdict: "AC"},
		},
		Lives: 2, MaxLives: 5, Ducks: 0,
		Uncommitted: 120,
		Upgrades:    []game.Upgrade{game.Overclock, game.ECC},
		Daemons:     []game.Daemon{game.Strace},
		Linked:      []string{"dsu"},
		Log: []game.LogLine{
			{At: start, Text: "boot: prestige 1, standard"},
			{At: at(10), Text: "door_1: AC usaco:987"},
			{At: at(25), Text: "door_2: AC usaco:1011"},
		},
	}
}

// crash closes the open door with cause, as the engine would.
func crash(st *game.State, cause game.Outcome) *game.State {
	a := *st.Current
	a.Closed, a.Outcome = a.Opened.Add(20*time.Minute), cause
	st.History = append(st.History, a)
	st.Current = nil
	st.Phase = game.PhaseCrashed
	st.Crash = &game.Crash{
		Cause: cause, Door: a.Door, Verdict: a.LastVerdict, Lost: st.Uncommitted,
		At: a.Closed, Uptime: a.Closed.Sub(st.Started),
	}
	st.Uncommitted = 0
	return st
}

// clearBoot closes the open door as cleared and ends the boot.
func clearBoot(st *game.State) *game.State {
	a := *st.Current
	a.Closed, a.Outcome, a.LastVerdict = a.Opened.Add(25*time.Minute), game.Cleared, "AC"
	st.History = append(st.History, a)
	st.Current = nil
	st.Phase = game.PhaseCleared
	st.Committed += st.Uncommitted
	st.Uncommitted = 0
	return st
}

// same compares with DeepEqual and prints both sides as JSON on mismatch,
// which reads better than %+v for nested game structs.
func same(t *testing.T, what string, got, want any) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		return
	}
	g, _ := json.MarshalIndent(got, "", "  ")
	w, _ := json.MarshalIndent(want, "", "  ")
	t.Errorf("%s differs\ngot:  %s\nwant: %s", what, g, w)
}

func checkpoint(t *testing.T, s *store.Store, m game.Meta, st *game.State) {
	t.Helper()
	if err := s.Checkpoint(m, st); err != nil {
		t.Fatalf("Checkpoint boot %d: %v", st.Boot, err)
	}
}
