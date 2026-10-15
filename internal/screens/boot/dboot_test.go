package boot

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens/boot/fixture"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// D36: a door card never shows the engine's clear chance, at any width;
// it is shown after the door.
func TestDoorsHideClearChance(t *testing.T) {
	for _, st := range []func() *Doors{
		func() *Doors { return NewDoors(env(fixture.Doors())) },
		func() *Doors { return NewDoors(env(fixture.DoorsFork())) },
		func() *Doors { return NewDoors(env(fixture.Boss())) },
		func() *Doors { return NewDoors(env(fixture.Userland())) }, // a forecast and an easing
	} {
		for _, sz := range fitSizes {
			v := ansi.Strip(st().View(sz[0], sz[1]))
			if strings.Contains(v, "% clear") || strings.Contains(v, "≈") {
				t.Fatalf("door cards at %dx%d show a clear chance:\n%s", sz[0], sz[1], v)
			}
		}
	}
}

// The White Knight's block shows over the first daemon offers, is reported
// once drawn so the app marks it seen, waits when the window is too short
// for it, and stays away once seen.
func TestMergeDaemonIntro(t *testing.T) {
	m := NewMerge(env(fixture.Merge()))
	if v := ansi.Strip(m.View(80, 24)); strings.Contains(v, "daemon.intro") || m.Shown() != nil {
		t.Errorf("the intro squeezed into a full 80x24 window (shown %v):\n%s", m.Shown(), v)
	}
	if v := ansi.Strip(m.View(140, 40)); !strings.Contains(v, "Stub block for daemon.intro") {
		t.Errorf("no intro over the first daemon offers:\n%s", v)
	}
	if got := m.Shown(); len(got) != 1 || got[0] != "daemon.intro" {
		t.Errorf("Shown = %v after drawing the intro", got)
	}
	e := env(fixture.Merge())
	game.MarkSeen(&e.Meta, "daemon.intro")
	if v := ansi.Strip(NewMerge(e).View(140, 40)); strings.Contains(v, "daemon.intro") {
		t.Errorf("the intro came back once seen:\n%s", v)
	}
}

// The boss card carries the boss's entrance line under its name.
func TestBossCardEntrance(t *testing.T) {
	v := ansi.Strip(NewDoors(env(fixture.Boss())).View(80, 24))
	name := strings.Index(v, "H E I S E N B U G")
	enter := strings.Index(v, "Stub line for boss.enter.heisenbug.")
	if name < 0 || enter < name {
		t.Errorf("no entrance line under the boss's name:\n%s", v)
	}
}

// fetchEnv is the doors fixture mid-download: door 1 on disk, door 2
// chosen and downloading, door 3 waiting its turn.
func fetchEnv() Env {
	e := env(fixture.Doors())
	ds := e.State.Doors
	e.Fetch = map[catalog.ID]Fetch{
		ds[0].Problem: {State: FetchReady},
		ds[1].Problem: {State: FetchGoing, Got: 38 << 20},
		ds[2].Problem: {State: FetchQueued},
	}
	e.Wait = &Wait{Index: 1, Since: e.Now.Add(-21 * time.Second)}
	return e
}

// Door readiness on the cards and the wait on the last row (J9), at both
// golden sizes; esc lets the wait go.
func TestGoldenDoorsFetch(t *testing.T) {
	for _, sz := range goldenSizes {
		name := fmt.Sprintf("doors_fetch_%dx%d", sz[0], sz[1])
		t.Run(name, func(t *testing.T) {
			golden(t, name, ansi.Strip(NewDoors(fetchEnv()).View(sz[0], sz[1]))+"\n")
		})
	}
	for _, sz := range fitSizes {
		checkFits(t, "doors_fetch", NewDoors(fetchEnv()).View(sz[0], sz[1]), sz[0], sz[1])
	}
	d := NewDoors(fetchEnv())
	if _, cmd := d.Update(press("esc")); emitted(cmd) != (CancelWait{}) {
		t.Errorf("esc while waiting emitted %#v, want CancelWait", emitted(cmd))
	}
	if _, cmd := NewDoors(env(fixture.Doors())).Update(press("esc")); cmd != nil {
		t.Errorf("esc with no wait emitted %#v", emitted(cmd))
	}
}

// The copy guardrails (docs/research/retention.md §(d)) over every string
// the boot screens write: no streaks (GR1), and easing is never called
// easy, assist or mercy (GR9).
func TestCopyGuardrails(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	banned := regexp.MustCompile(`(?i)\b(streaks?|easy|assist|mercy)\b`)
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && banned.MatchString(lit.Value) {
				t.Errorf("%s: %s", fset.Position(lit.Pos()), lit.Value)
			}
			return true
		})
	}
}

// D44: while a spare life is left, every door of a biome whose timeout
// costs a life says so, on its card and in the HUD; on the last life, in a
// biome without the rule, and in a trial, none does.
func TestTimeoutNote(t *testing.T) {
	st := fixture.Userland()
	const note = "⏱ a timeout costs ♥, not the boot"
	for _, sz := range goldenSizes {
		if v := ansi.Strip(NewDoors(env(st)).View(sz[0], sz[1])); strings.Count(v, note) != len(st.Doors) {
			t.Errorf("%dx%d: want the D44 line on each of %d cards:\n%s", sz[0], sz[1], len(st.Doors), v)
		}
	}
	open := fixture.Userland()
	a := game.Attempt{Door: open.Doors[0], Opened: fixture.Now, Deadline: fixture.Now.Add(open.Doors[0].Timer)}
	open.Phase, open.Current, open.Doors = game.PhaseProblem, &a, nil
	prob, _ := fixture.Lookup(a.Door.Problem)
	if hud := ansi.Strip(HUDBar(open, prob, fixture.Now, 48)); !strings.Contains(hud, note) {
		t.Errorf("no D44 line in the HUD:\n%s", hud)
	}
	last := fixture.Userland()
	last.Lives = 1
	trial := fixture.Userland()
	trial.Trial = &game.TrialState{}
	for name, st := range map[string]*game.State{"last life": last, "the Heap": fixture.Doors(), "trial": trial} {
		if txt, ok := timeoutNote(st); ok {
			t.Errorf("%s: %q", name, txt)
		}
	}
}

// RT8: before the first door, each easing is a line above the cards in
// task terms with its reason, and x declines them all; once declined each
// line says so and x does nothing. Past the first door, and in a boot
// with no easing, nothing shows.
func TestEasing(t *testing.T) {
	st := fixture.Userland()
	lines, decline := easing(st)
	want := []string{
		"Userland aim retuned (first boot in 5 days)",
		"+1 ♥ (Userland's easiest doors in the pool are rated 800)",
	}
	if got := stripAll(lines); !slices.Equal(got, want) || decline != (game.DeclineEase{}) {
		t.Fatalf("easing = %q, %#v; want %q and DeclineEase", got, decline, want)
	}
	d := NewDoors(env(st))
	v := ansi.Strip(d.View(80, 24))
	for _, w := range append(want, "x decline the easing") {
		if !strings.Contains(v, w) {
			t.Errorf("doors screen lacks %q:\n%s", w, v)
		}
	}
	if _, cmd := d.Update(press("x")); emitted(cmd) != (game.DeclineEase{}) {
		t.Errorf("x emitted %#v, want DeclineEase", emitted(cmd))
	}

	st.Declined = true
	lines, decline = easing(st)
	if decline != nil || len(lines) != 2 || !strings.HasSuffix(ansi.Strip(lines[0]), " · declined") {
		t.Errorf("declined: %q, %#v", stripAll(lines), decline)
	}
	if _, cmd := NewDoors(env(st)).Update(press("x")); cmd != nil {
		t.Errorf("x after the decline emitted %#v", emitted(cmd))
	}
	for name, st := range map[string]*game.State{"past the first door": fixture.UserlandTimeout(), "no easing": fixture.Doors()} {
		if lines, decline := easing(st); lines != nil || decline != nil {
			t.Errorf("%s: %q, %#v", name, stripAll(lines), decline)
		}
	}
}

// GR9 and D36 over every easing the engine can make: the lines name what
// changed, never the player and never "easy", "assist" or "mercy", and
// carry no clear chance before a door; the kept block carries the aims'
// numbers.
func TestEasingCopy(t *testing.T) {
	banned := regexp.MustCompile(`(?i)\b(easy|assist|mercy|you)\b`)
	st := fixture.Userland()
	st.Ease = []game.Ease{
		{Kind: game.EaseFirst, Why: "boot #1", FirstDoor: true, Editorial: true},
		{Kind: game.EaseComeback, Why: "first boot in 5 days", Clear: []float64{0.95, 0, 0, 0}},
		{Kind: game.EaseGovernor, Why: "the last 2 boots ended in Userland", Clear: []float64{0.95, 0.78, 0.65, 0.55}, Lives: 1},
		{Kind: game.EaseFloor, Why: "Userland's easiest doors in the pool are rated 800", Lives: 2, Ducks: 1, Buyback: true},
	}
	lines, _ := easing(st)
	items := easeItems(st)
	if len(lines) != len(st.Ease) || len(items) != len(st.Ease) {
		t.Fatalf("%d lines, %d kept items for %d easings", len(lines), len(items), len(st.Ease))
	}
	for i, l := range stripAll(lines) {
		if banned.MatchString(l) || strings.Contains(l, "%") {
			t.Errorf("line %d: %q", i, l)
		}
	}
	for _, l := range stripAll(items) {
		if banned.MatchString(l) || !strings.HasPrefix(l, "eased: ") {
			t.Errorf("kept item %q", l)
		}
	}
	if got, want := ansi.Strip(items[2]), "eased: aims 90%/70%/60%/50% → 95%/78%/65%/55%, +1 ♥ (the last 2 boots ended in Userland)"; got != want {
		t.Errorf("governor kept item\n got %q\nwant %q", got, want)
	}
	if got, want := ansi.Strip(items[3]), "eased: +2 ♥, +🦆1, longer timers on Userland's floor doors (Userland's easiest doors in the pool are rated 800)"; got != want {
		t.Errorf("floor kept item\n got %q\nwant %q", got, want)
	}
}

func stripAll(ls []string) []string {
	var out []string
	for _, l := range ls {
		out = append(out, ansi.Strip(l))
	}
	return out
}

// The White Queen's block shows over the first room that offers a
// regression door, once drawn is reported, and stays away once seen.
func TestDoorsRegressionIntro(t *testing.T) {
	d := NewDoors(env(fixture.DoorsFork()))
	if v := ansi.Strip(d.View(140, 40)); !strings.Contains(v, "Stub block for story.first-regression") {
		t.Fatalf("no White Queen over the first regression room:\n%s", v)
	}
	if got := d.Shown(); len(got) != 1 || got[0] != "story.first-regression" {
		t.Errorf("Shown = %v", got)
	}
	e := env(fixture.DoorsFork())
	game.MarkSeen(&e.Meta, "story.first-regression")
	if v := ansi.Strip(NewDoors(e).View(140, 40)); strings.Contains(v, "first-regression") {
		t.Errorf("the White Queen came back once seen:\n%s", v)
	}
	if NewDoors(env(fixture.Doors())).intro {
		t.Error("a room with no regression door wants the White Queen")
	}
}

// A route card's aim is the engine's (State.AimClear): an easing's while
// it runs, the biome's own once declined.
func TestRouteAim(t *testing.T) {
	st := fixture.Route()
	st.Ease = []game.Ease{{Kind: game.EaseGovernor, Why: "the last 2 boots ended in Userland", Clear: []float64{0.95, 0.78, 0.65, 0.55}}}
	if v := ansi.Strip(NewRoute(env(st)).View(140, 40)); !strings.Contains(v, "aim 78% clear") {
		t.Errorf("eased route:\n%s", v)
	}
	st.Declined = true
	if v := ansi.Strip(NewRoute(env(st)).View(140, 40)); !strings.Contains(v, "aim 70% clear") || strings.Contains(v, "78%") {
		t.Errorf("declined route:\n%s", v)
	}
}

// A door on its source's floor shows its rating as a ceiling, "◆≤800", in
// the Rating token's rank color, wherever its stats row goes; others keep
// "◆1000".
func TestFloorRating(t *testing.T) {
	st := fixture.Userland()
	if !st.Doors[0].Floor || st.Doors[1].Floor {
		t.Fatal("the fixture's first door is not the floor's")
	}
	for _, sz := range [][2]int{{140, 40}, {80, 24}, {80, 12}} {
		v := NewDoors(env(st)).View(sz[0], sz[1])
		if !strings.Contains(v, ui.RatingAtMost(800)) || !strings.Contains(ansi.Strip(v), "◆1000") {
			t.Errorf("%dx%d:\n%s", sz[0], sz[1], ansi.Strip(v))
		}
	}
	if s := doorStats(st.Doors[1]); strings.Contains(ansi.Strip(s), "≤") {
		t.Errorf("a door above the floor: %q", ansi.Strip(s))
	}
}

// The engine's forecast of a crash in the first biome shows after the
// doors (D36), on the crash's lever line, never on a door card; a trial
// and a boot without one show none.
func TestForecastAfterDoors(t *testing.T) {
	const line = "≈26% forecast to crash in Userland"
	e := crashEnv(game.TimedOut)
	e.State.Forecast = 0.26
	if v := ansi.Strip(NewCrash(e).View(140, 40)); !strings.Contains(v, line) {
		t.Errorf("no forecast on the lever line:\n%s", v)
	}
	if v := ansi.Strip(NewDoors(env(fixture.Userland())).View(140, 40)); strings.Contains(v, "forecast") {
		t.Errorf("a forecast before the doors:\n%s", v)
	}
	e.State.Trial = &game.TrialState{}
	if s := forecastItem(e.State); s != "" {
		t.Errorf("trial: %q", ansi.Strip(s))
	}
	if s := forecastItem(fixture.Crash(game.TimedOut)); s != "" {
		t.Errorf("no forecast: %q", ansi.Strip(s))
	}
}

// A route card's timer is the one its doors will get (State.RouteTimer),
// Overclock included, not the biome's bare default (UX J24), on every
// card layout.
func TestRouteTimerOverclocked(t *testing.T) {
	st := fixture.Route()
	st.Upgrades = []game.Upgrade{game.Overclock, game.Overclock}
	e := env(st)
	bare := "~" + units.Clock(st.Rules.Biomes[st.Biome].Timer.Duration)
	for _, topic := range st.Biomes[st.Biome].Offers {
		want := "~" + units.Clock(st.RouteTimer(&e.Meta, topic))
		if want == bare {
			t.Fatalf("%s: route timer %s is the bare default; the test needs one that moved", topic, want)
		}
		for _, sz := range [][2]int{{140, 40}, {80, 24}} {
			v := ansi.Strip(NewRoute(e).View(sz[0], sz[1]))
			if !strings.Contains(v, want) || strings.Contains(v, bare) {
				t.Errorf("%dx%d: want %s, not %s:\n%s", sz[0], sz[1], want, bare, v)
			}
		}
	}
}
