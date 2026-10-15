package setuid_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
	"katharsis/internal/setuid"
)

// live plays a model through boots with the bits checked at every safe
// point, as the app checks them, and returns the bits, the finished boots
// (snapshots, oldest first) and the Meta it ended with. It checks on the
// way that Check twice is once and that the set never shrinks.
func live(t *testing.T, p sim.Player, boots int, seed uint64) ([]game.SetuidBit, []*game.State, *game.Meta) {
	t.Helper()
	var ended []*game.State
	var prev []game.SetuidBit
	at := func(s *game.State) time.Time { return s.EndTime(s.Started.Add(time.Hour)) }
	rep := sim.Run(sim.Config{
		Rules: rules.Standard(), Pool: sim.SyntheticPool(6000, 1), Player: p, Boots: boots, Seed: seed, Prestige: -1,
		Check: func(s *game.State, m *game.Meta, _ game.Action, _ []game.Effect, err error) {
			if err != nil || s == nil {
				return
			}
			setuid.Check(m, s, at(s))
			if again := setuid.Check(m, s, at(s)); len(again) != 0 {
				t.Fatalf("boot %d: a second check set %v", s.Boot, again)
			}
			if len(m.Setuid) < len(prev) || !slices.Equal(m.Setuid[:len(prev)], prev) {
				t.Fatalf("boot %d: the bits shrank or changed", s.Boot)
			}
			prev = slices.Clone(m.Setuid)
			if (s.Phase == game.PhaseCrashed || s.Phase == game.PhaseCleared) && (len(ended) == 0 || ended[len(ended)-1].Boot != s.Boot) {
				ended = append(ended, snapshot(t, s))
			}
		},
	})
	if rep.Errors != 0 {
		t.Fatalf("%d model errors", rep.Errors)
	}
	m := rep.Meta
	setuid.Check(m, nil, at(ended[len(ended)-1]).Add(time.Hour)) // after the last upsolve
	return m.Setuid, ended, m
}

func snapshot(t *testing.T, s *game.State) *game.State {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var out game.State
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

func ids(bits []game.SetuidBit) []string {
	var out []string
	for _, b := range bits {
		out = append(out, b.ID)
	}
	slices.Sort(out)
	return out
}

// TestBackfillMatchesLive: backfilling a save from before the bits (its
// Meta without bits or record) over the boots it played sets what live
// play set over the same boots, stamped and marked as backfilled. The
// speed completions are the exception the backfill allows itself: it sets
// one only if it holds now (none does here).
func TestBackfillMatchesLive(t *testing.T) {
	t.Parallel()
	seeker := sim.DefaultPlayer(1300)
	seeker.Trial = true
	for name, run := range map[string]struct {
		p     sim.Player
		boots int
	}{
		"climber": {sim.DefaultPlayer(1700), 220},
		"seeker":  {seeker, 120},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bits, boots, m := live(t, run.p, run.boots, 3)
			pre := clone(m)
			pre.Setuid, pre.Record = nil, game.Record{}
			now := boots[len(boots)-1].EndTime(t0).Add(48 * time.Hour)
			b := setuid.NewBackfill(pre, setuid.Samples{}, now)
			for _, s := range boots {
				b.Boot(s)
			}
			back := b.Done()
			if got, want := ids(back), ids(bits); !slices.Equal(got, want) {
				t.Errorf("backfill set %v\nlive set     %v", got, want)
			}
			if !pre.Record.Backfilled || !slices.Equal(ids(pre.Setuid), ids(back)) {
				t.Error("the backfill did not mark the save or keep its bits")
			}
			for _, x := range back {
				if !x.Backfill || x.At.After(now) {
					t.Errorf("bit %+v", x)
				}
			}
			r1, r2 := m.Record, pre.Record
			r1.Backfilled, r2.Backfilled = false, false
			if mustJSON(t, r1) != mustJSON(t, r2) {
				t.Errorf("record\nlive     %s\nbackfill %s", mustJSON(t, r1), mustJSON(t, r2))
			}
			t.Logf("%s: %d boots, %d bits: %v", name, len(boots), len(bits), ids(bits))
			if name == "seeker" && !slices.Contains(ids(bits), ".dull-reality") {
				t.Error("a model set out for the Trial never woke")
			}
		})
	}
}

// TestDeterminism: the same boots set the same bits at the same times.
func TestDeterminism(t *testing.T) {
	t.Parallel()
	a, _, _ := live(t, sim.DefaultPlayer(1500), 80, 9)
	b, _, _ := live(t, sim.DefaultPlayer(1500), 80, 9)
	if mustJSON(t, a) != mustJSON(t, b) || len(a) == 0 {
		t.Errorf("two runs set\n%s\n%s", mustJSON(t, a), mustJSON(t, b))
	}
}

// legacySaves are the finished boots and Metas of the engine's frozen
// pre-cycles saves (internal/game/testdata/legacy): each continuation's
// end, as a store from before the bits holds it.
func legacySaves(t *testing.T) (boots []*game.State, metas []*game.Meta) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "game", "testdata", "legacy", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no legacy fixtures: %v", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fx struct {
			Conts []struct{ EndState, EndMeta json.RawMessage }
		}
		if err := json.Unmarshal(raw, &fx); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, c := range fx.Conts {
			var s game.State
			var m game.Meta
			if err := json.Unmarshal(c.EndState, &s); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(c.EndMeta, &m); err != nil {
				t.Fatal(err)
			}
			boots, metas = append(boots, &s), append(metas, &m)
		}
	}
	return boots, metas
}

// TestBackfillLegacy: the pre-cycles saves, linear boots of the old
// standard rules, backfill only what a linear boot can prove: a door, a
// clear, a boss, a ring, the record's sets. No three-act bit, no twist's
// bit, no Heisenbug without its sample runs.
func TestBackfillLegacy(t *testing.T) {
	t.Parallel()
	boots, metas := legacySaves(t)
	mapOnly := []string{"snicker-snack", "you-halted", "another-road", "by-railway", "un-birthday", "speedrun-1", "speedrun-2",
		"all-o3", "finds-none", "catless", "essence", "doesnt-blink", "ditto-ditto", "rss-modest", "stopped-looking",
		"every-thread", "bounds-checked", "decided-this-one", ".dull-reality", ".two-miles-high", ".stuff-and-nonsense",
		".galumphing-back", ".then-it-ends", ".counted-again", "cartographer"}
	seen := map[string]int{}
	for i, s := range boots {
		if s.Phase != game.PhaseCrashed && s.Phase != game.PhaseCleared {
			t.Fatalf("legacy save %d did not end: %s", i, s.Phase)
		}
		m := metas[i]
		b := setuid.NewBackfill(m, setuid.Samples{}, t0.AddDate(1, 0, 0))
		b.Boot(s)
		for _, x := range b.Done() {
			seen[x.ID]++
			if slices.Contains(mapOnly, x.ID) {
				t.Errorf("legacy boot %d set %s", s.Boot, x.ID)
			}
			if !x.Backfill {
				t.Errorf("%s not marked from git log", x.ID)
			}
		}
		if m.Record.Folded != s.Boot || m.Record.BootTime != s.BootTime(t0) {
			t.Errorf("legacy boot %d folded %+v", s.Boot, m.Record)
		}
	}
	t.Logf("%d legacy boots set %v", len(boots), seen)
	if seen["hello-world"] == 0 {
		t.Error("no legacy boot cleared a door?")
	}
	if len(slices.Collect(maps.Keys(seen))) == 0 {
		t.Error("the legacy saves set nothing")
	}
}

// TestBackfillHistory: the rules a backfill keeps to on history it cannot
// date or that lacks the evidence (achievements.md §(i)).
func TestBackfillHistory(t *testing.T) {
	t.Parallel()
	now := t0.Add(30 * 24 * time.Hour)

	// Stamps: a boot's bits at its end; Meta's at the backfill's time.
	m := &game.Meta{Capabilities: []string{"CAP_DP"}}
	s := mapClear()
	b := setuid.NewBackfill(m, setuid.Samples{}, now)
	b.Boot(s)
	b.Boot(s) // folded: nothing twice
	got := b.Done()
	stamp := map[string]time.Time{}
	for _, x := range got {
		stamp[x.ID] = x.At
		if x.Boot == 0 && x.ID != "stamped" {
			t.Errorf("%s set outside a boot", x.ID)
		}
	}
	if !stamp["snicker-snack"].Equal(s.EndTime(now)) || !stamp["stamped"].Equal(now) {
		t.Errorf("stamps %v", stamp)
	}
	if u := slices.Compact(ids(got)); len(u) != len(got) {
		t.Errorf("bits set twice: %v", ids(got))
	}

	// Heisenbug without :w: a boot saved before Attempt.Samples needs the
	// events table, from its start on.
	old := mapClear()
	old.CountsSamples = false
	hb := slices.IndexFunc(old.History, func(a game.Attempt) bool { return a.Door.Boss == "heisenbug" })
	door := setuid.Door{Boot: old.Boot, Index: hb}
	for name, c := range map[string]struct {
		samples setuid.Samples
		want    bool
	}{
		"no events":         {setuid.Samples{}, false},
		"events after":      {setuid.Samples{Since: old.History[hb].Opened.Add(time.Second)}, false},
		"events, no :w":     {setuid.Samples{Since: t0.Add(-time.Hour)}, true},
		"events, a :w":      {setuid.Samples{Since: t0.Add(-time.Hour), Runs: map[setuid.Door]int{door: 1}}, false},
		"another door's :w": {setuid.Samples{Since: t0.Add(-time.Hour), Runs: map[setuid.Door]int{{old.Boot, hb - 1}: 2}}, true},
	} {
		m := &game.Meta{}
		b := setuid.NewBackfill(m, c.samples, now)
		b.Boot(old)
		b.Done()
		if setuid.IsSet(m, "stopped-looking") != c.want {
			t.Errorf("%s: stopped-looking %v, want %v", name, !c.want, c.want)
		}
	}

	// A core dump counts as recovered by an upsolve after its crash and
	// before the next crash, which destroys it; independent if the upsolve
	// was.
	dumped := func(boot int, at time.Time, lost int64) *game.State {
		c := crash(mapClear())
		c.Boot = boot
		c.Crash.At, c.Crash.Lost = at, lost
		return c
	}
	// Every dump is on t:7, the crashed final door.
	up := func(at time.Time, k game.SolveKind) game.Solve {
		return game.Solve{Problem: "t:7", Kind: k, At: at, Upsolve: true}
	}
	day := func(n int) time.Time { return t0.AddDate(0, 0, n) }
	m = &game.Meta{Solves: []game.Solve{
		up(day(1).Add(time.Hour), game.Independent), // boot 1's dump, before boot 2: recovered, free
		up(day(2).Add(-time.Hour), game.Editorial),  // boot 1's again: nothing left
		up(day(4).Add(time.Hour), game.Editorial),   // boot 3's dump went with boot 4's crash
		up(day(5).Add(time.Hour), game.Hinted),      // boot 5's, the last: recovered
	}}
	b = setuid.NewBackfill(m, setuid.Samples{}, now)
	for i, lost := range []int64{96, 0, 128, 0, 32} { // boots 1..5 at days 1..5
		b.Boot(dumped(i+1, day(i+1), lost))
	}
	b.Done()
	if m.Record.Recovered != 2 || m.Record.RecoveredFree != 1 {
		t.Errorf("recovered %d (%d free), want 2 (1)", m.Record.Recovered, m.Record.RecoveredFree)
	}

	// The speed completions: only if they hold now.
	for _, c := range []struct {
		boot time.Duration
		want bool
	}{{60*time.Hour - time.Second, true}, {60 * time.Hour, false}} {
		m := full100()
		m.Record.BootTime = c.boot
		b := setuid.NewBackfill(m, setuid.Samples{}, now)
		b.Done()
		if setuid.IsSet(m, "speed-completion") != c.want || !setuid.IsSet(m, "hundred") {
			t.Errorf("boot time %v: speed completion %v, want %v", c.boot, !c.want, c.want)
		}
	}
}
