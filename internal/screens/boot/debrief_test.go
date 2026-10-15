package boot

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot/fixture"
	"katharsis/internal/screens/story"
)

// stubTale is story text of realistic length, so the end screens' goldens
// pin layout, not the story's words. The beat chosen is the real
// flavor.NextBeat: which beat waits is logic, not text.
var stubTale = taleSource{
	Story: func(hook string, _ uint64) string {
		return "Stub " + hook + ", a line in the story's voice."
	},
	Block: func(hook string) []string {
		return []string{"Stub " + hook + " verse, the first line,", "the second, {bytes} of it gone."}
	},
	NextBeat:  flavor.NextBeat,
	BeatTitle: func(id string) string { return "stub title of " + id },
	BeatLines: func(id string) []string { return []string{"Stub first line of " + id + ", a question left open."} },
}

func init() { tale = stubTale }

// end is the app's figures as a player twenty boots in would have them:
// apt's next package, ETAs from their own boots, short and patch offered.
func end() End {
	return End{
		Goal:    &Goal{Name: "strace", Price: 2500, Per: 140},
		ETA:     101 * time.Minute,
		Reboots: []Reboot{{Key: "s", Preset: "short", ETA: 62 * time.Minute}, {Key: "p", Preset: "patch"}},
	}
}

func clearedEnv() Env {
	e := env(fixture.Cleared())
	e.Meta = fixture.ClearedMeta()
	return e
}

func endCases() []screenCase {
	return []screenCase{
		{"crash_end", func() screens.Screen {
			e := crashEnv(game.OutOfLives)
			drySpell(&e.Meta)
			e.End = end()
			return NewCrash(e)
		}},
		// A crash over a live dump: the loss is the headline (RT4).
		{"crash_lost", func() screens.Screen {
			e := crashEnv(game.TimedOut)
			e.State.Crash.DumpLost = 183
			e.State.AtStart.Dump = &game.CoreDump{Problem: "usaco:897", Bytes: 183, Boot: 41}
			e.End = end()
			return NewCrash(e)
		}},
		{"crash_armed", func() screens.Screen {
			e := crashEnv(game.GaveUp)
			e.End = end()
			c := NewCrash(e)
			c.Update(press("r"))
			return c
		}},
		{"crash_recovered", func() screens.Screen {
			e := crashEnv(game.TimedOut)
			game.Upsolve(&e.Meta, e.State.Crash.Door.Problem, game.Independent, time.Minute, e.State.Crash.At.Add(time.Hour))
			e.End = end()
			e.End.Today, e.End.Budget = 3*time.Hour+12*time.Minute, 3*time.Hour
			return NewCrash(e)
		}},
		{"cleared_end", func() screens.Screen {
			e := clearedEnv()
			e.End = end()
			e.End.Firsts = []string{"first Heisenbug kill", "fastest clear: 2:29:52"}
			return NewCleared(e)
		}},
		// The easing the boot ran with, in kept with its numbers (RT8), and the
		// engine's forecast on the lever line (D36).
		{"crash_eased", func() screens.Screen {
			e := crashEnv(game.OutOfLives)
			e.State.Ease = fixture.Userland().Ease
			e.State.Forecast = fixture.Userland().Forecast
			e.End = end()
			return NewCrash(e)
		}},
		{"cleared_declined", func() screens.Screen {
			e := clearedEnv()
			e.State.Ease, e.State.Declined = fixture.Userland().Ease, true
			e.End = end()
			return NewCleared(e)
		}},
		{"merge_goal", func() screens.Screen {
			e := env(fixture.Merge())
			e.End = End{Goal: &Goal{Name: "strace", Price: 2300, Per: 140}}
			return NewMerge(e)
		}},
	}
}

func TestEndGoldens(t *testing.T) {
	for _, c := range endCases() {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				golden(t, name, ansi.Strip(c.make().View(sz[0], sz[1]))+"\n")
			})
		}
	}
}

// The end screens fit every size with the real story and flavor too.
func TestEndScreensFit(t *testing.T) {
	defer func() { tale, fl = stubTale, stubFlavor }()
	for _, src := range []struct {
		name string
		tale taleSource
		fl   flavorSource
	}{{"stub", stubTale, stubFlavor}, {"real", realTale, realFlavor}} {
		tale, fl = src.tale, src.fl
		for _, c := range append(endCases(), cases()...) {
			if !strings.HasPrefix(c.name, "crash") && !strings.HasPrefix(c.name, "cleared") && !strings.HasPrefix(c.name, "merge") {
				continue
			}
			for _, sz := range fitSizes {
				checkFits(t, src.name+"/"+c.name, c.make().View(sz[0], sz[1]), sz[0], sz[1])
			}
		}
	}
}

// Required lines survive at 80x24: the dump, kept, lever, next and the
// keys; the panic line too.
func TestCrashEndAt80x24(t *testing.T) {
	for _, cause := range []game.Outcome{game.TimedOut, game.OutOfLives, game.GaveUp} {
		e := crashEnv(cause)
		e.End = end()
		v := ansi.Strip(NewCrash(e).View(80, 24))
		for _, want := range []string{"Kernel panic - not syncing", "core dumped: 96 B", "  kept ", "  lever ", "  next ▸ u upsolve", "[g] GRUB"} {
			if !strings.Contains(v, want) {
				t.Errorf("%s: missing %q:\n%s", cause, want, v)
			}
		}
	}
}

// With a live core dump every reboot key asks twice (RT4); without one,
// once. Another key disarms.
func TestCrashRebootGuard(t *testing.T) {
	e := crashEnv(game.TimedOut)
	e.End = end()
	c := NewCrash(e)
	if _, cmd := c.Update(press("r")); cmd != nil {
		t.Fatalf("first r with a live dump emitted %#v", emitted(cmd))
	}
	if v := ansi.Strip(c.View(120, 40)); !strings.Contains(v, "reboot leaves 96 B at risk · r again · u upsolve first") {
		t.Errorf("armed screen does not warn:\n%s", v)
	}
	if _, cmd := c.Update(press("s")); cmd != nil {
		t.Fatalf("s after r emitted %#v; a different key arms itself", emitted(cmd))
	}
	_, cmd := c.Update(press("s"))
	if got := emitted(cmd); got != (screens.StartBoot{Prestige: 1, Preset: "short"}) {
		t.Errorf("s s: got %#v", got)
	}
	c.Update(press("r"))
	c.Update(press("x")) // disarms
	if _, cmd := c.Update(press("r")); cmd != nil {
		t.Errorf("r after a disarming key emitted %#v", emitted(cmd))
	}

	// No dump: one key reboots.
	e = crashEnv(game.TimedOut)
	e.Meta.CoreDump = nil
	e.State.Crash.Lost = 0
	_, cmd = NewCrash(e).Update(press("r"))
	if got := emitted(cmd); got != (screens.StartBoot{Prestige: 1, Preset: "standard"}) {
		t.Errorf("r without a dump: got %#v", got)
	}
}

func TestEndKeys(t *testing.T) {
	ce := crashEnv(game.TimedOut)
	drySpell(&ce.Meta)
	ce.End = end()
	cl := clearedEnv()
	cl.End = end()
	for _, tc := range []struct {
		name string
		s    screens.Screen
		key  string
		want any
	}{
		{"crash b", NewCrash(ce), "b", story.Read{ID: "milestone.doors.10"}},
		{"crash g", NewCrash(ce), "g", screens.OpenGrub{}},
		{"cleared b", NewCleared(cl), "b", story.Read{ID: "reveal.prestige.1"}},
		{"cleared r", NewCleared(cl), "r", screens.StartBoot{Prestige: 2, Preset: "standard"}},
		{"cleared s", NewCleared(cl), "s", screens.StartBoot{Prestige: 2, Preset: "short"}},
		{"cleared p", NewCleared(cl), "p", screens.StartBoot{Prestige: 2, Preset: "patch"}},
		{"cleared g", NewCleared(cl), "g", screens.OpenGrub{}},
		{"cleared q", NewCleared(cl), "q", screens.OpenGrub{}},
		{"cleared x", NewCleared(cl), "x", nil},
	} {
		_, cmd := tc.s.Update(press(tc.key))
		if got := emitted(cmd); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %#v, want %#v", tc.name, got, tc.want)
		}
	}
	// The Cat plays by itself (app/story.go): a crash that owes it teases
	// nothing, and neither does one whose beat was read.
	read := crashEnv(game.TimedOut)
	c := NewCrash(read)
	if _, cmd := c.Update(press("b")); cmd != nil {
		t.Errorf("b with no beat waiting emitted %#v", emitted(cmd))
	}
	if v := ansi.Strip(c.View(120, 40)); strings.Contains(v, "[b] story") || strings.Contains(v, "read on") {
		t.Errorf("a read beat still teased:\n%s", v)
	}
}

// Every lever names a test, a time, a tool or a topic (RT1): never the
// player.
func TestLeverNamesTheTask(t *testing.T) {
	task := regexp.MustCompile(`\d+/\d+ tests|\d\d:\d\d|:Submit|:q!|upsolve|complexity|memory|compiled|cursed`)
	player := regexp.MustCompile(`(?i)\byou\b|\byour\b|careless|lazy|bad|failure`)
	e := crashEnv(game.TimedOut)
	door := e.State.Crash.Door
	for _, l := range []game.Lever{
		{Kind: game.LeverTimer, Door: door, Timer: 30 * time.Minute},
		{Kind: game.LeverTimer, Door: door, Timer: 30 * time.Minute, Submits: 2, Verdict: "TLE", Passed: 51, Total: 52},
		{Kind: game.LeverLives, Door: door, Failed: 3, Verdict: "WA", Passed: 37, Total: 52},
		{Kind: game.LeverLives, Door: door, Failed: 3, Verdict: "RE", Passed: 2, Total: 52},
		{Kind: game.LeverLives, Door: door, Failed: 3, Verdict: "MLE", Passed: 9, Total: 52},
		{Kind: game.LeverLives, Door: door, Failed: 1, Verdict: "CE"},
		{Kind: game.LeverCursed, Door: door, Failed: 1, Verdict: "WA", Passed: 3, Total: 10},
		{Kind: game.LeverGaveUp, Door: door, Elapsed: 11 * time.Minute, Timer: 30 * time.Minute},
		{Kind: game.LeverGaveUp, Door: door, Elapsed: 11 * time.Minute, Timer: 30 * time.Minute, Submits: 1, Verdict: "WA", Passed: 1, Total: 9},
		{Kind: game.LeverCleared, Door: door, Elapsed: 36 * time.Minute, Timer: 50 * time.Minute, Quacks: 1},
	} {
		s := ansi.Strip(leverText(e, l))
		if !task.MatchString(s) || player.MatchString(s) {
			t.Errorf("%s %s: %q does not name the task", l.Kind, l.Verdict, s)
		}
	}
}

// A crash paints no gain (GR3): the rating's rise on a crash is plain,
// on a clear it is OK.
func TestRatingItemNoWinCueOnCrash(t *testing.T) {
	up := game.Change{Before: 1100, After: 1150, Known: true}
	if a, b := ratingItem(up, "rating", true), ratingItem(up, "rating", false); a == b || !strings.Contains(ansi.Strip(a), "1100 → 1150 ▲50") {
		t.Errorf("crash %q, clear %q", a, b)
	}
}

func TestGoalItem(t *testing.T) {
	for _, tc := range []struct {
		g    Goal
		bank int64
		want string
	}{
		{Goal{Name: "strace", Price: 512, Per: 133}, 312, "apt strace ▓▓▓▓░░░░ 312 B/512 B · ≈2 boots"},
		{Goal{Name: "strace", Price: 512}, 312, "apt strace ▓▓▓▓░░░░ 312 B/512 B"},
		{Goal{Name: "strace", Price: 512, Per: 133}, 600, "apt strace affordable at GRUB"},
		{Goal{Name: "tux", Price: 128, Per: 400}, 0, "apt tux ░░░░░░░░ 0 B/128 B · ≈1 boot"},
	} {
		if got := ansi.Strip(goalItem(&tc.g, tc.bank)); got != tc.want {
			t.Errorf("goal %+v bank %d: %q, want %q", tc.g, tc.bank, got, tc.want)
		}
	}
}

// The status line in a door carries the dump the boot puts at risk.
func TestHUDBarDumpAtRisk(t *testing.T) {
	st := fixture.Problem()
	p, _ := fixture.Lookup(st.Current.Door.Problem)
	if v := ansi.Strip(HUDBar(st, p, fixture.Now, 44)); strings.Contains(v, "core dump") {
		t.Errorf("no dump at risk, but:\n%s", v)
	}
	st.AtStart = &game.Start{Dump: &game.CoreDump{Problem: "usaco:897", Bytes: 183}}
	if v := ansi.Strip(HUDBar(st, p, fixture.Now, 44)); !strings.Contains(v, "☠ 183 B core dump at risk") {
		t.Errorf("dump at risk missing:\n%s", v)
	}
	st.Trial = &game.TrialState{}
	if v := ansi.Strip(HUDBar(st, p, fixture.Now, 44)); strings.Contains(v, "core dump") {
		t.Errorf("a trial risks no dump, but:\n%s", v)
	}
}

// Past the budget one line says so, at the end of a boot only (RT12,
// GR6); under it, or with no budget, nothing.
func TestBudgetLine(t *testing.T) {
	for _, tc := range []struct {
		today, budget time.Duration
		want          bool
	}{{4 * time.Hour, 3 * time.Hour, true}, {time.Hour, 3 * time.Hour, false}, {9 * time.Hour, 0, false}} {
		got := budgetLine(End{Today: tc.today, Budget: tc.budget}, 80)
		if (len(got) == 1) != tc.want || len(got) > 1 {
			t.Errorf("today %v budget %v: %q", tc.today, tc.budget, got)
		}
		if tc.want && strings.Contains(strings.ToLower(ansi.Strip(got[0])), "streak") {
			t.Errorf("budget line %q", got[0])
		}
	}
}

// drySpell makes the end of the fixture's boot 42 owe a milestone: the
// Cat seen at boot 30, eleven boots without a beat since (RT6's gap).
func drySpell(m *game.Meta) {
	m.Seen = append(m.Seen, game.SeenScene{ID: "story.first-crash", Boot: 30})
}

// A beat read, seen in the boot it ends, leaves no key behind.
func TestReadBeatLeaves(t *testing.T) {
	e := crashEnv(game.TimedOut)
	drySpell(&e.Meta)
	c := NewCrash(e)
	if v := ansi.Strip(c.View(120, 40)); !strings.Contains(v, "read on") {
		t.Fatalf("no teaser before the read:\n%s", v)
	}
	e.Meta.Seen = append(e.Meta.Seen, game.SeenScene{ID: "milestone.doors.10", Boot: 42})
	c.Refresh(e)
	if v := ansi.Strip(c.View(120, 40)); strings.Contains(v, "read on") || strings.Contains(v, "[b] story") {
		t.Errorf("the read beat is still teased:\n%s", v)
	}
}

// A crash reboots at the boot's own prestige, as it always has; a clear
// at the highest unlocked.
func TestRebootPrestige(t *testing.T) {
	e := crashEnv(game.TimedOut)
	e.Meta.CoreDump, e.State.Crash.Lost = nil, 0
	e.State.Prestige, e.Meta.Prestige = 0, 2
	if got := emitted(func() tea.Cmd { _, c := NewCrash(e).Update(press("r")); return c }()); got != (screens.StartBoot{Prestige: 0, Preset: "standard"}) {
		t.Errorf("crash r: %#v", got)
	}
}
