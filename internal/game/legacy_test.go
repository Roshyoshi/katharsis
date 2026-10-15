package game_test

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/game/sim"
)

// legacyFixture is an in-flight boot saved by the engine before the cycles
// meta (D47, map.md §(h)), as the store keeps it: the State with its rules
// snapshot and the Meta, both as JSON. Each continuation is the frozen
// fuzzer (legacyFuzz) played from it on that engine to the boot's end,
// with what each step did and where it ended. The files were written once
// and are never regenerated: they are the old engine's word.
type legacyFixture struct {
	Name  string
	Note  string
	Pool  [2]uint64 // sim.SyntheticPool(n, seed)
	Now   time.Time
	State json.RawMessage
	Meta  json.RawMessage
	Conts []legacyCont
}

type legacyCont struct {
	Seed     [2]uint64 // legacyFuzz's PCG seeds
	Accepted string    // per step: + accepted, - refused
	EndNow   time.Time
	LogLen   int
	EndState json.RawMessage // Log left out: its wording moved to cycles
	EndMeta  json.RawMessage
}

// TestLegacySaves loads every in-flight boot the old engine saved and
// plays it on: the same steps must be accepted and refused, and the boot
// must end where it did, field for field (the log only by its length,
// since its words now say cycles). A snapshot without the cycles fields
// plays as it did (map.md §(h)).
func TestLegacySaves(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join("testdata", "legacy", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fx legacyFixture
		if err := json.Unmarshal(b, &fx); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		ps := sim.SyntheticPool(int(fx.Pool[0]), fx.Pool[1])
		for ci, c := range fx.Conts {
			var s game.State
			var m game.Meta
			if err := json.Unmarshal(fx.State, &s); err != nil {
				t.Fatalf("%s: state: %v", fx.Name, err)
			}
			if err := json.Unmarshal(fx.Meta, &m); err != nil {
				t.Fatalf("%s: meta: %v", fx.Name, err)
			}
			g := &legacyFuzz{rng: rand.New(rand.NewPCG(c.Seed[0], c.Seed[1])), now: fx.Now}
			var acc strings.Builder
			for n := 0; n < 400 && s.Phase != game.PhaseCrashed && s.Phase != game.PhaseCleared; n++ {
				g.tick()
				if _, err := game.Step(&s, &m, ps, g.action(&s), g.now); err != nil {
					acc.WriteByte('-')
				} else {
					acc.WriteByte('+')
				}
			}
			if acc.String() != c.Accepted || !g.now.Equal(c.EndNow) || len(s.Log) != c.LogLen {
				t.Errorf("%s cont %d: steps %s, want %s; log %d lines, want %d", fx.Name, ci, acc.String(), c.Accepted, len(s.Log), c.LogLen)
				continue
			}
			s.Log = nil
			var want game.State
			var wantM game.Meta
			if err := json.Unmarshal(c.EndState, &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(c.EndMeta, &wantM); err != nil {
				t.Fatal(err)
			}
			if got, w := mustJSON(t, &s), mustJSON(t, &want); got != w {
				t.Errorf("%s cont %d: the boot ends\n%s\nwant\n%s", fx.Name, ci, got, w)
			}
			if got, w := mustJSON(t, &m), mustJSON(t, &wantM); got != w {
				t.Errorf("%s cont %d: meta ends\n%s\nwant\n%s", fx.Name, ci, got, w)
			}
		}
	}
}
