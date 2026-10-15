package boot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot/fixture"
)

// The setuid bits on the boot screens (docs/design/achievements.md (h)):
// the strip's chmod line at a safe point, the cap's first line, the
// cleared screen's kept line, and nothing on a crash.

// seenTips is the fixture's Meta with every tip met, as a player this
// deep has.
func seenTips() game.Meta {
	m := fixture.Meta()
	for _, id := range []string{"tip.doors", "tip.door", "tip.upgrade", "tip.bytes", "tip.fail", "tip.duck", "tip.regression"} {
		game.MarkSeen(&m, id)
	}
	return m
}

// bossFell is the merge window after Off-by-One fell, its AC on the strip
// and then the bits the step set, as the app adds them after the look.
func bossFell(bits ...game.SetuidBit) Env {
	m := seenTips()
	p := fixture.CyclesProblem(fixture.CyclesBoss("off-by-one"))
	n := fixture.CyclesMerge("full")
	p.Cycles = n.Cycles // no cycles line: the AC and the bits
	var tr Tracker
	tr.Lookup = fixture.Lookup
	tr.Observe(p, &m, fixture.Now)
	tr.Observe(n, &m, fixture.Now)
	if len(bits) > 0 {
		tr.Add(SetuidEvent(bits, n.Seed, fixture.Now))
	}
	e := env(n)
	e.Meta, e.Events = m, tr.Events()
	return e
}

func bits(ids ...string) []game.SetuidBit {
	var out []game.SetuidBit
	for _, id := range ids {
		out = append(out, game.SetuidBit{ID: id, At: fixture.Now, Boot: 57})
	}
	return out
}

// capped is the merge window after fsck filled the pool to its cap, the
// first boot ever to (Meta.Capped).
func capped() Env {
	m := seenTips()
	p := fixture.CyclesMerge("full")
	n := fixture.CyclesMerge("full")
	n.Spent, n.Uncommitted = 64, n.Uncommitted-64
	n.Cycles, n.MaxCycles, n.Capped = 50, 50, true
	p.MaxCycles = 50
	m.Capped = n.Boot
	var tr Tracker
	tr.Lookup = fixture.Lookup
	tr.Observe(p, &m, fixture.Now)
	tr.Observe(n, &m, fixture.Now.Add(20*time.Second))
	e := env(n)
	e.Meta, e.Events = m, tr.Events()
	return e
}

// clearedWith is the map boot's clear with the bits it set, the notices on
// or off.
func clearedWith(on bool, ids ...string) Env {
	e := env(fixture.CyclesCleared())
	e.Meta = fixture.ClearedMeta()
	for _, b := range bits(ids...) {
		b.Boot = e.State.Boot
		e.Meta.Setuid = append(e.Meta.Setuid, b)
	}
	e.Meta.Setuid = append(e.Meta.Setuid, game.SetuidBit{ID: "hello-world", Backfill: true}) // git log's
	e.SetuidNotices = on
	return e
}

func TestSetuidGoldens(t *testing.T) {
	for _, c := range []struct {
		name string
		make func() screens.Screen
	}{
		{"setuid_strip", func() screens.Screen { return NewMerge(bossFell(bits("two-twins", "ditto-ditto")...)) }},
		{"setuid_strip_more", func() screens.Screen {
			return NewMerge(bossFell(bits("two-twins", "ditto-ditto", ".counted-again", "rss-modest")...))
		}},
		{"setuid_cap", func() screens.Screen { return NewMerge(capped()) }},
		{"setuid_cleared", func() screens.Screen { return NewCleared(clearedWith(true, "snicker-snack", "finds-none")) }},
	} {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			v := c.make().View(sz[0], sz[1])
			checkFits(t, name, v, sz[0], sz[1])
			golden(t, name, ansi.Strip(v)+"\n")
		}
	}
}

// The chmod line names two bits and counts the rest; its flavor line is
// the first bit's that has one.
func TestChmod(t *testing.T) {
	for _, c := range []struct {
		ids  []string
		want string
	}{
		{[]string{"hello-world"}, "chmod u+s /bin/hello-world"},
		{[]string{"two-twins", "ditto-ditto"}, "chmod u+s /bin/two-twins /sbin/ditto-ditto"},
		{[]string{"two-twins", "ditto-ditto", "every-bug"}, "chmod u+s /bin/two-twins /sbin/ditto-ditto and 1 more · find / -perm -4000"},
		{[]string{".dull-reality"}, "chmod u+s /home/you/.dull-reality"},
	} {
		if got := Chmod(bits(c.ids...)); got != c.want {
			t.Errorf("Chmod(%v) = %q, want %q", c.ids, got, c.want)
		}
	}
	if ev := SetuidEvent(bits(".dull-reality"), 0, fixture.Now); ev.Hook != "setuid.dull-reality" || ev.Kind != EvSetuid {
		t.Errorf("event %+v", ev)
	}
}

// The cap's line comes once, the first time any boot fills its pool
// (Meta.Capped is that boot), not in a later boot that fills it again.
func TestCapLine(t *testing.T) {
	e := capped()
	if n := countKind(e.Events, EvCap); n != 1 {
		t.Fatalf("%d cap lines, want 1: %+v", n, e.Events)
	}
	m := seenTips()
	m.Capped = 3 // an earlier boot was the first
	p, n := fixture.CyclesMerge("full"), fixture.CyclesMerge("full")
	n.Cycles, n.MaxCycles, n.Capped = 50, 50, true
	var tr Tracker
	tr.Observe(p, &m, fixture.Now)
	tr.Observe(n, &m, fixture.Now)
	if c := countKind(tr.Events(), EvCap); c != 0 {
		t.Errorf("a later cap: %d lines", c)
	}
}

func countKind(evs []Event, kind string) int {
	n := 0
	for _, e := range evs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// The kept line counts the boot's own bits, not the backfill's; with the
// notices off it is gone. A crash screen never says a bit (GR3), however
// many the boot set.
func TestSetuidKeptAndCrash(t *testing.T) {
	for _, sz := range goldenSizes {
		on := ansi.Strip(NewCleared(clearedWith(true, "snicker-snack")).View(sz[0], sz[1]))
		if !strings.Contains(on, "setuid +1 · snicker-snack") {
			t.Errorf("at %v the kept line is missing:\n%s", sz, on)
		}
		if off := ansi.Strip(NewCleared(clearedWith(false, "snicker-snack")).View(sz[0], sz[1])); strings.Contains(off, "setuid") {
			t.Errorf("at %v with the notices off:\n%s", sz, off)
		}
		for _, how := range []string{"timeout", "submits", "final"} {
			e := env(fixture.CyclesCrash(how))
			e.SetuidNotices = true
			for _, b := range bits("exit-0", "two-twins") {
				b.Boot = e.State.Boot
				e.Meta.Setuid = append(e.Meta.Setuid, b)
			}
			if v := ansi.Strip(NewCrash(e).View(sz[0], sz[1])); strings.Contains(v, "setuid") || strings.Contains(v, "chmod") {
				t.Errorf("the %s crash says a bit at %v:\n%s", how, sz, v)
			}
		}
	}
}
