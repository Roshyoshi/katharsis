package game_test

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current engine")

// TestDoorsGolden replays fuzzed boots and lists every set of doors put on
// offer and how each door ended, against testdata/doors.golden. Logging
// the draw (State.Offers, ml-pipeline slice ML1) must leave the draw
// alone: the same seeds and actions offer the same doors as when the file
// was written. A change meant to move the doors rewrites it with -update.
func TestDoorsGolden(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 8)
	m := &game.Meta{Prestige: game.MaxPrestige}
	f := &fuzzer{rng: rand.New(rand.NewPCG(31, 32)), now: t0}
	var w strings.Builder
	for b := range 60 {
		s := startBoot(t, rules.Standard(), m, ps, f)
		last := ""
		note := func(step int) {
			if s.Phase != game.PhaseDoors {
				return
			}
			var ds []string
			for _, d := range s.Doors {
				ds = append(ds, fmt.Sprintf("%s/%d/%s/%dB/%.4f/%s", d.Problem, d.Rating, d.Timer, d.Bytes, d.SolveP,
					strings.ReplaceAll(doorFlags(d), " ", "+")))
			}
			if line := strings.Join(ds, " "); line != last {
				fmt.Fprintf(&w, "boot %d step %d room %d.%d: %s\n", b, step, s.Biome, s.Room, line)
				last = line
			}
		}
		note(0)
		for n := 1; n <= 400 && s.Phase != game.PhaseCrashed && s.Phase != game.PhaseCleared; n++ {
			f.tick()
			a := f.action(s)
			if _, err := game.Step(s, m, ps, a, f.now); err == nil {
				note(n)
			}
		}
		for _, h := range s.History {
			fmt.Fprintf(&w, "boot %d: %s %s\n", b, h.Door.Problem, h.Outcome)
		}
		fmt.Fprintf(&w, "boot %d ends %s, rating %.4f\n", b, s.Phase, m.Rating)
	}
	path := filepath.Join("testdata", "doors.golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(w.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	if got := w.String(); got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range min(len(gl), len(wl)) {
			if gl[i] != wl[i] {
				t.Fatalf("line %d differs:\n got %s\nwant %s", i+1, gl[i], wl[i])
			}
		}
		t.Fatalf("%d lines, want %d", len(gl), len(wl))
	}
}

// doorFlags names a door's modifiers and easing, for the golden file.
func doorFlags(d game.Door) string {
	var fs []string
	for _, f := range []struct {
		on   bool
		name string
	}{
		{d.Blind, "blind"}, {d.Mystery, "mystery"}, {d.Cursed, "cursed"}, {d.Regression, "regression"},
		{d.Boss != "", "boss-" + d.Boss}, {d.Mastered, "mastered"}, {d.Floor, "floor"},
		{d.TimerBuy > 0, fmt.Sprintf("x%g", d.TimerBuy)}, {d.Tier != "", "tier-" + d.Tier},
	} {
		if f.on {
			fs = append(fs, f.name)
		}
	}
	if len(fs) == 0 {
		return "-"
	}
	return strings.Join(fs, " ")
}
