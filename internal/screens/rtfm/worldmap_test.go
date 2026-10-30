package rtfm_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/rtfm"
	"katharsis/internal/screens/rtfm/fixture"
	"katharsis/internal/ui"
)

// TestAtlasFixture keeps fixture/atlas.json the atlas of the guide
// snapshot at KATHARSIS_GUIDE; -update writes it.
func TestAtlasFixture(t *testing.T) {
	dir := os.Getenv("KATHARSIS_GUIDE")
	if dir == "" {
		t.Skip("set KATHARSIS_GUIDE to a usaco-guide snapshot to run")
	}
	c, err := curriculum.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	judgeable := func(id catalog.ID) bool { return id.Source() == catalog.USACO || id.Source() == catalog.CSES }
	got, err := json.MarshalIndent(c.Atlas(judgeable), "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("fixture", "atlas.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if want, err := os.ReadFile(path); err != nil || string(want) != string(got) {
		t.Errorf("%s is not the snapshot's atlas (go test -update with KATHARSIS_GUIDE set)", path)
	}
}

// world is the index of the world with division id in a.
func world(t testing.TB, a curriculum.Atlas, id string) int {
	t.Helper()
	i := slices.IndexFunc(a.Worlds, func(w curriculum.World) bool { return w.ID == id })
	if i < 0 {
		t.Fatalf("atlas has no world %q", id)
	}
	return i
}

// midSilver is a player part way into Silver: six of General's pages
// opened, nine of Bronze's trials passed and three of Silver's, each with
// its capability and its first playable problem solved.
func midSilver(t testing.TB, a curriculum.Atlas) game.Meta {
	var m game.Meta
	for _, l := range a.Worlds[world(t, a, "general")].Levels()[:6] {
		m.Opened = append(m.Opened, l.Module)
	}
	pass := func(id string, n int) {
		for _, l := range a.Worlds[world(t, a, id)].Levels() {
			if l.Kind == curriculum.PageLevel || n == 0 {
				continue
			}
			n--
			m.TrialsPassed = append(m.TrialsPassed, l.Module)
			if l.Capability != "" && !slices.Contains(m.Capabilities, l.Capability) {
				m.Capabilities = append(m.Capabilities, l.Capability)
			}
			if len(l.Playable) > 0 {
				m.Solves = append(m.Solves, game.Solve{Problem: l.Playable[0]})
			}
		}
	}
	pass("bronze", 9)
	pass("silver", 3)
	return m
}

// mapAt is the full guide's map for the mid-Silver player, in world id at
// the level of module at ("" leaves the map's own choice).
func mapAt(t testing.TB, id, at string) *rtfm.Map {
	t.Helper()
	a := fixture.Atlas()
	m := rtfm.NewMapOf(a, midSilver(t, a))
	wi := world(t, a, id)
	for range a.Worlds {
		m.Update(key("up"))
	}
	for range wi {
		m.Update(key("down"))
	}
	m.Update(key("enter"))
	if at != "" {
		i := slices.IndexFunc(a.Worlds[wi].Levels(), func(l curriculum.Level) bool { return l.Module == at })
		if i < 0 {
			t.Fatalf("%s has no level %q", id, at)
		}
		m.Update(key("home"))
		for range i {
			m.Update(key("right"))
		}
	}
	return m
}

// The world map's goldens, from the full guide's atlas: the overworld, and
// Bronze, Silver (at a level with warps) and Advanced (the largest
// world), in both skins.
func TestMapGoldens(t *testing.T) {
	if len(fixture.Atlas().Worlds) == 0 {
		t.Fatal("fixture/atlas.json is empty (go test -update with KATHARSIS_GUIDE set)")
	}
	views := []struct {
		name string
		m    func() *rtfm.Map
	}{
		{"overworld", func() *rtfm.Map {
			a := fixture.Atlas()
			return rtfm.NewMapOf(a, midSilver(t, a))
		}},
		{"bronze", func() *rtfm.Map { return mapAt(t, "bronze", "") }},
		{"silver", func() *rtfm.Map { return mapAt(t, "silver", "graph-traversal") }},
		{"adv", func() *rtfm.Map { return mapAt(t, "adv", "eulerian-tours") }},
	}
	for _, skin := range []struct {
		name string
		g    ui.Glyphs
	}{{"", ui.Unicode}, {"_ascii", ui.ASCII}} {
		ui.SetGlyphs(skin.g)
		for _, v := range views {
			for _, sz := range sizes {
				golden(t, "map_"+v.name+skin.name+"_"+dims(sz), v.m().View(sz[0], sz[1]))
			}
		}
	}
	ui.SetGlyphs(ui.Unicode)
}

func dims(sz [2]int) string { return fmt.Sprintf("%dx%d", sz[0], sz[1]) }

// Every world of the full guide fits its screen, with each of its levels
// selected in turn: no line wider than the screen, the screen's height
// exactly, the cursor in view; at 80x24 and up the whole world shows
// without scrolling, so its first region is always named.
func TestMapFits(t *testing.T) {
	a := fixture.Atlas()
	for _, sz := range [][2]int{{80, 24}, {140, 40}, {100, 30}, {60, 20}, {40, 12}} {
		w, h := sz[0], sz[1]
		for _, wd := range a.Worlds {
			m := mapAt(t, wd.ID, "")
			m.Update(key("home"))
			for i := range wd.Levels() {
				if i > 0 {
					m.Update(key("right"))
				}
				view := m.View(w, h)
				lines := strings.Split(view, "\n")
				if len(lines) != h {
					t.Fatalf("%s at %dx%d, level %d: %d lines", wd.ID, w, h, i, len(lines))
				}
				for n, l := range lines {
					if lw := ansi.StringWidth(l); lw > w {
						t.Fatalf("%s at %dx%d, level %d: line %d is %d wide:\n%s", wd.ID, w, h, i, n, lw, ansi.Strip(l))
					}
				}
				if !strings.Contains(ansi.Strip(view), ui.G.Cursor) {
					t.Fatalf("%s at %dx%d, level %d: the cursor is off screen:\n%s", wd.ID, w, h, i, ansi.Strip(view))
				}
				if w >= 80 && h >= 24 && !strings.Contains(ansi.Strip(view), wd.Regions[0].Title[:min(4, len(wd.Regions[0].Title))]) {
					t.Fatalf("%s at %dx%d, level %d: scrolled:\n%s", wd.ID, w, h, i, ansi.Strip(view))
				}
			}
		}
	}
}

// The map's keys: → and ← walk the path in guide order, ↓ goes to the row
// below, w warps to a later world and back, tab changes world, q climbs
// to the overworld and from there leaves, and r, t and i are intents.
func TestMapKeys(t *testing.T) {
	a := fixture.Atlas()
	si := world(t, a, "silver")
	silver := a.Worlds[si].Levels()
	m := mapAt(t, "silver", silver[0].Module)
	sel := func() string {
		t.Helper()
		v := ansi.Strip(m.View(80, 24))
		for _, w := range a.Worlds {
			for _, l := range w.Levels() {
				if strings.Contains(v, "╭─ "+l.Title+" ") {
					return l.Module
				}
			}
		}
		t.Fatalf("no card title in:\n%s", v)
		return ""
	}
	if got := sel(); got != silver[0].Module {
		t.Fatalf("at %s, want %s", got, silver[0].Module)
	}
	press(t, m, "right", "right")
	if got := sel(); got != silver[2].Module {
		t.Errorf("→→ at %s, want %s", got, silver[2].Module)
	}
	press(t, m, "left")
	if got := sel(); got != silver[1].Module {
		t.Errorf("← at %s, want %s", got, silver[1].Module)
	}
	// Silver's 14 levels take a row of 11 at 80 columns and come back on
	// a row of 3 at its right end. ↓ goes to the nearest node on the row
	// below, the castle (first on screen, last on the path), and ↑ from
	// there to the node over it.
	press(t, m, "down")
	if got, want := sel(), silver[len(silver)-1].Module; got != want {
		t.Errorf("↓ went to %s, want %s", got, want)
	}
	press(t, m, "up")
	if got, want := sel(), silver[8].Module; got != want {
		t.Errorf("↑ went to %s, want %s", got, want)
	}
	press(t, m, "up")
	if got, want := sel(), silver[8].Module; got != want {
		t.Errorf("↑ off the top went to %s, want %s", got, want)
	}

	m = mapAt(t, "silver", "graph-traversal")
	press(t, m, "w")
	if got := sel(); got != "eulerian-tours" {
		t.Errorf("w went to %s, want eulerian-tours", got)
	}
	press(t, m, "w")
	if got := sel(); got != "graph-traversal" {
		t.Errorf("w back went to %s, want graph-traversal", got)
	}
	press(t, m, "w")
	if got := sel(); got != "string-search" {
		t.Errorf("w again went to %s, want string-search", got)
	}

	// A warp only looks ahead: String Searching is far past where the path
	// has reached, so nothing enters it, and the bar offers none of it.
	for _, k := range []string{"enter", "p", "r", "t"} {
		wantMsg(t, press(t, m, k), nil)
	}
	if v := ansi.Strip(m.View(80, 24)); !strings.Contains(v, "╭─ String Searching") ||
		!strings.Contains(v, ui.G.CapLocked+" locked: pass ") || strings.Contains(v, "enter level") {
		t.Errorf("enter on a locked level shows:\n%s", v)
	}
	wantMsg(t, press(t, m, "i"), screens.OpenIndex{Module: "string-search"})

	// The first trial not passed is open: the path is at it.
	passed := midSilver(t, a).TrialsPassed
	at := silver[slices.IndexFunc(silver, func(l curriculum.Level) bool {
		return l.Kind.HasTrial() && !l.Optional && !slices.Contains(passed, l.Module)
	})].Module
	m = mapAt(t, "silver", at)
	wantMsg(t, press(t, m, "r"), screens.OpenModule{Module: at})
	wantMsg(t, press(t, m, "t"), screens.StartTrial{Module: at})

	m = mapAt(t, "general", "")
	wantMsg(t, press(t, m, "t"), nil) // a page has no trial
	press(t, m, "tab")
	if v := ansi.Strip(m.View(80, 24)); !strings.HasPrefix(v, "RTFM › Bronze") {
		t.Errorf("tab from General shows:\n%s", v)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if v := ansi.Strip(m.View(80, 24)); !strings.HasPrefix(v, "RTFM › General") {
		t.Errorf("shift+tab back shows:\n%s", v)
	}
	wantMsg(t, press(t, m, "q"), nil)
	if v := ansi.Strip(m.View(80, 24)); !strings.HasPrefix(v, "RTFM(7)") {
		t.Errorf("q from a world shows:\n%s", v)
	}
	wantMsg(t, press(t, m, "q"), screens.Back{})
}

// A map opened at a module for another screen (Open) is in its world at
// its level, and q goes straight back.
func TestMapOpen(t *testing.T) {
	a := fixture.Atlas()
	m := rtfm.NewMapOf(a, game.Meta{})
	if m.Open("no-such-module") {
		t.Error("Open found a module the guide lacks")
	}
	if !m.Open("intro-tree") {
		t.Fatal("Open missed intro-tree")
	}
	if v := ansi.Strip(m.View(80, 24)); !strings.Contains(v, "╭─ Introduction to Tree Algorithms") || !strings.HasPrefix(v, "RTFM › Silver") {
		t.Errorf("Open shows:\n%s", v)
	}
	wantMsg(t, press(t, m, "q"), screens.Back{})
}

// The switch palace: after Flip, the world's marks for the new capability
// show □ until the clip reaches them; a key skips to the end without
// acting, and under reduced motion the map opens at the end.
func TestMapFlip(t *testing.T) {
	a := fixture.Atlas()
	si := world(t, a, "silver")
	l, _, _ := a.Level("graph-traversal")
	meta := midSilver(t, a)
	if !slices.Contains(meta.Capabilities, l.Capability) {
		meta.Capabilities = append(meta.Capabilities, l.Capability)
	}
	n := 0
	for _, x := range a.Worlds[si].Levels() {
		if x.Capability == l.Capability {
			n++
		}
	}
	if n == 0 {
		t.Fatalf("Silver has no level teaching %s", l.Capability)
	}
	held := func(m *rtfm.Map) int { return strings.Count(ansi.Strip(m.View(80, 24)), ui.G.Cap) }
	open := func() *rtfm.Map {
		m := rtfm.NewMapOf(a, meta)
		m.Open("graph-traversal")
		m.Flip([]string{l.Capability})
		m.Init()
		return m
	}

	m := open()
	before := held(m)
	wantMsg(t, press(t, m, "i"), nil) // skips; does not open the index
	if got := held(m) - before; got != n {
		t.Errorf("the switch palace filled in %d marks, want %d", got, n)
	}
	wantMsg(t, press(t, m, "i"), screens.OpenIndex{Module: "graph-traversal"})

	anim.SetMotion(anim.Motion{Reduced: true})
	defer anim.SetMotion(anim.Motion{})
	if got := held(open()); got != before+n {
		t.Errorf("reduced motion shows %d marks held, want %d", got, before+n)
	}
}

// A player holding nothing sees □ beside the counts, not ■: the glyph says
// held, so it waits for the first.
func TestMapNothingHeld(t *testing.T) {
	m := rtfm.NewMapOf(fixture.Atlas(), game.Meta{})
	if v := ansi.Strip(m.View(80, 24)); !strings.Contains(v, ui.G.CapLocked+" 0/11 capabilities held") {
		t.Errorf("the overworld of a new player:\n%s", v)
	}
	m.Update(key("enter"))
	if v := ansi.Strip(m.View(80, 24)); !strings.Contains(v, ui.G.CapLocked+" 0/3") || strings.Contains(v, ui.G.Cap+" 0/") {
		t.Errorf("Bronze for a new player:\n%s", v)
	}
}

// Back from a practice AC (Meta refreshed under the open level), the cursor
// stays on the problem, now ticked, and its card says so. Prefix Sums is
// passed, so the path has reached Two Pointers.
func TestMapRefreshKeepsTheLevel(t *testing.T) {
	c := loadCurriculum(t)
	meta := fixture.Meta()
	meta.TrialsPassed = []string{"prefix-sums"}
	m := rtfm.NewMap(c, meta, fixture.Judgeable, fixture.Lookup)
	m.Open("two-pointers")
	press(t, m, "enter", "p")
	lines := func() []string { return strings.Split(ansi.Strip(m.View(80, 24)), "\n") }
	cursor := func(ls []string) string {
		i := slices.IndexFunc(ls, func(l string) bool { return strings.Contains(l, ui.G.Cursor) })
		return ls[max(i, 0)]
	}
	if l := cursor(lines()); !strings.Contains(l, ui.G.Cursor+ui.G.Here+" Paired Up") {
		t.Fatalf("p put the cursor on %q, want Paired Up (usaco:738), open", l)
	}
	meta.Solves = []game.Solve{{Problem: "usaco:738", Upsolve: true}}
	m.Refresh(meta)
	ls := lines()
	if l := cursor(ls); !strings.Contains(l, ui.G.Cursor+ui.G.Pass+" Paired Up") {
		t.Errorf("after the solve the cursor is on %q, want Paired Up ticked", l)
	}
	if v := strings.Join(ls, "\n"); !strings.Contains(v, ui.G.Pass+" solved") {
		t.Errorf("the solve does not show:\n%s", v)
	}
}

// n goes to the next trial: the first in guide order not passed, past
// General and past pages, from anywhere on the map, and the bar names it.
func TestMapNext(t *testing.T) {
	a := fixture.Atlas()
	m := rtfm.NewMapOf(a, game.Meta{})
	if v := ansi.Strip(m.View(80, 24)); !strings.Contains(v, "next "+ui.G.Cursor+" Bronze"+ui.G.Crumb+"Simulation") {
		t.Errorf("a new player's overworld bar:\n%s", v)
	}
	press(t, m, "n")
	if got := m.Selected(); got != "simulation" {
		t.Errorf("n on a new player's map goes to %q, want simulation (Bronze's first trial)", got)
	}

	meta := midSilver(t, a)
	m = rtfm.NewMapOf(a, meta)
	press(t, m, "n")
	got := m.Selected()
	var order []curriculum.Level
	for _, w := range a.Worlds[1:] { // past General
		order = append(order, w.Levels()...)
	}
	i := slices.IndexFunc(order, func(l curriculum.Level) bool { return l.Module == got })
	if i < 0 || !order[i].Kind.HasTrial() || slices.Contains(meta.TrialsPassed, got) {
		t.Fatalf("n goes to %q, want a trial not passed", got)
	}
	for _, l := range order[:i] {
		if l.Kind.HasTrial() && !slices.Contains(meta.TrialsPassed, l.Module) {
			t.Errorf("n skips %s, a trial before %s not passed", l.Module, got)
		}
	}
}
