package boot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot/fixture"
	"katharsis/internal/units"
)

// hubEnv is env over a map boot with a core dump from boot #41 in Meta.
func hubEnv(st *game.State) Env {
	e := env(st)
	e.Meta.CoreDump = &game.CoreDump{Problem: "usaco:764", Bytes: 312, Boot: 41, At: fixture.Now.Add(-24 * time.Hour)}
	return e
}

// mapCases are the map hub and the route's exit cards on the map.
func mapCases() []screenCase {
	return []screenCase{
		{"hub", func() screens.Screen { return NewHub(hubEnv(fixture.MapRoute()), true) }},
		{"hub_doors", func() screens.Screen { return NewHub(hubEnv(fixture.MapDoors()), false) }},
		{"hub_merge", func() screens.Screen { return NewHub(hubEnv(fixture.MapMerge()), false) }},
		{"hub_final", func() screens.Screen { return NewHub(hubEnv(fixture.MapRouteFinal()), true) }},
		{"hub_abandon", func() screens.Screen {
			h := NewHub(hubEnv(fixture.MapRoute()), true)
			h.Update(press("n"))
			return h
		}},
		// Gated exits: The ALU open, two behind a capability, each +1.
		{"exits_locked", func() screens.Screen { return NewRoute(env(fixture.MapRoute())) }},
		{"exits_locked_sel", func() screens.Screen {
			r := NewRoute(env(fixture.MapRoute()))
			r.Update(press("2"))
			return r
		}},
		// The exits into Boss I are bosses.
		{"exits_boss", func() screens.Screen { return NewRoute(env(fixture.MapRouteBoss())) }},
		// Into act III: a lock, a pick, Kernel Space and the skip.
		{"exits_skip", func() screens.Screen { return NewRoute(env(fixture.MapRouteFinal())) }},
		// A Bronze boot: Undefined Behavior opens from prestige 1.
		{"exits_prestige", func() screens.Screen { return NewRoute(env(fixture.MapRoutePrestige())) }},
	}
}

func TestMapGoldens(t *testing.T) {
	for _, c := range mapCases() {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				golden(t, name, ansi.Strip(c.make().View(sz[0], sz[1]))+"\n")
			})
		}
	}
}

// Every map screen fills its terminal, no line wider than it, with the
// stub text and the real.
func TestMapScreensFit(t *testing.T) {
	defer func() { fl = stubFlavor }()
	for _, src := range []flavorSource{stubFlavor, realFlavor} {
		fl = src
		for _, c := range mapCases() {
			for _, sz := range fitSizes {
				s := c.make()
				for range 5 {
					checkFits(t, c.name, s.View(sz[0], sz[1]), sz[0], sz[1])
					s, _ = s.Update(press("right"))
				}
			}
		}
	}
}

// A locked exit cannot be taken: enter does nothing on it, and t opens
// RTFM at its topic when a capability locks it, not a prestige.
func TestExitKeys(t *testing.T) {
	r := NewRoute(env(fixture.MapRoute()))
	key := func(k string) tea.Msg { _, cmd := r.Update(press(k)); return emitted(cmd) }
	if got := key("enter"); got != nil {
		t.Errorf("enter on Bisect Bridge (locked): %#v", got)
	}
	if got := key("t"); got != (screens.OpenLearn{Topic: "binary-search"}) {
		t.Errorf("t on Bisect Bridge: %#v", got)
	}
	key("3")
	if got := key("enter"); got != (game.ChooseRoute{Index: 2}) {
		t.Errorf("enter on The ALU: %#v", got)
	}
	if got := key("t"); got != nil {
		t.Errorf("t on an open exit: %#v", got)
	}

	r = NewRoute(env(fixture.MapRoutePrestige()))
	key("2")
	if got := key("t"); got != nil {
		t.Errorf("t on a prestige lock: %#v", got)
	}
	if got := key("enter"); got != nil {
		t.Errorf("enter on Undefined Behavior at Bronze: %#v", got)
	}
}

// An exit card names its biome, its band, its boss, its pick, and for a
// lock what opens it: the module when RTFM knows it.
func TestExitCards(t *testing.T) {
	e := env(fixture.MapRoute())
	e.Module = func(topic string) string { return map[string]string{"two-pointers": "Two Pointers"}[topic] }
	v := ansi.Strip(NewRoute(e).View(140, 40))
	for _, want := range []string{
		"Pick an exit.", "Bisect Bridge", "□ locked: CAP_BSEARCH or apt git", "Stub line for route.locked.",
		"The Sliding Window", "□ locked: CAP_TWOPTR", "+1 upgrade pick", "→ ⚔ Heisenbug", "The ALU",
		"a fail −12 ♥, the room lost", "◆",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("exit cards lack %q:\n%s", want, v)
		}
	}
	// The real text names the module from RTFM, else the capability, and a
	// prestige lock its ring.
	fl = realFlavor
	defer func() { fl = stubFlavor }()
	lock := func(r *Route, i int) string {
		return ansi.Strip(strings.Join(r.exitCard(r.exits()[i]).locked, " "))
	}
	r := NewRoute(e)
	for i, want := range map[int]string{0: "CAP_BSEARCH opens it", 1: "RTFM › Two Pointers opens it"} {
		if got := lock(r, i); !strings.Contains(got, want) {
			t.Errorf("exit %d: %q, want %q", i, got, want)
		}
	}
	if got := lock(NewRoute(env(fixture.MapRoutePrestige())), 1); !strings.Contains(got, "opens from ring 2 down") {
		t.Errorf("the prestige lock: %q", got)
	}
	v = ansi.Strip(NewRoute(env(fixture.MapRouteFinal())).View(140, 40))
	for _, want := range []string{"skip", "+2 upgrade picks", "→ ⚔ The Halting Problem", "Kernel Space", "The Cache"} {
		if !strings.Contains(v, want) {
			t.Errorf("act III exits lack %q:\n%s", want, v)
		}
	}
	// A node's own line (Kernel Space's), else its topic's.
	r = NewRoute(env(fixture.MapRouteFinal()))
	for i, want := range map[int]string{1: flavor.BiomeBlurb("dp"), 2: flavor.NodeBlurb("kernel", "")} {
		if got := r.exitCard(r.exits()[i]).blurb; got != want || got == "" {
			t.Errorf("exit %d blurb %q, want %q", i, got, want)
		}
	}
}

// An exit card's clock is the engine's for that exit (game.Exit.Timer),
// which carries its boss's twist (Race Condition's fast clock); an exit
// saved without one (an older save) falls back to the tier's.
func TestExitCardTimers(t *testing.T) {
	st := fixture.MapRouteBoss()
	ex := st.Biomes[st.Biome].Exits
	ex[0].Timer = 25*time.Minute + 36*time.Second
	e := env(st)
	tier := "~" + units.Clock(st.RouteTimer(&e.Meta, ex[1].Topic))
	for _, sz := range [][2]int{{140, 40}, {80, 24}} {
		v := ansi.Strip(NewRoute(e).View(sz[0], sz[1]))
		if !strings.Contains(v, "~25:36") || !strings.Contains(v, tier) {
			t.Errorf("%dx%d: want the exit's own ~25:36 and the fallback %s:\n%s", sz[0], sz[1], tier, v)
		}
	}
	// The Halting Problem hides its clock (D54), on its exit's card too.
	ex[0].Boss = game.FinalBoss
	for _, sz := range [][2]int{{140, 40}, {80, 24}} {
		v := ansi.Strip(NewRoute(e).View(sz[0], sz[1]))
		if strings.Contains(v, "25:36") || !strings.Contains(v, "⏱ ??:??") {
			t.Errorf("%dx%d: the final's exit shows its clock:\n%s", sz[0], sz[1], v)
		}
	}
}

// At 80x24 the exits are stacked cards or one row each, short of room for
// the wide cards, and still say what the choice decides: the tier's price
// of a fail (the same behind every exit, so said once), and in rows the
// selected exit's clock, the bosses it leads to, and what opens its lock.
func TestNarrowExits(t *testing.T) {
	// Act II, three exits: stacked cards.
	st := fixture.MapRoute()
	v := ansi.Strip(NewRoute(env(st)).View(80, 24))
	if want := ansi.Strip(failText(st)); !strings.Contains(v, want) {
		t.Errorf("stacked exits lack %q:\n%s", want, v)
	}
	// Act III, four exits: rows.
	st = fixture.MapRouteFinal()
	r := NewRoute(env(st))
	v = ansi.Strip(r.View(80, 24))
	for _, want := range []string{ansi.Strip(failText(st)), "⏱ ~", "→ ⚔ The Halting Problem", "Stub line for route.locked."} {
		if !strings.Contains(v, want) {
			t.Errorf("exit rows, Routing Table selected, lack %q:\n%s", want, v)
		}
	}
	r.Update(press("2"))
	v = ansi.Strip(r.View(80, 24))
	if !strings.Contains(v, "→ ⚔ The Halting Problem") || strings.Contains(v, "Stub line for route.locked.") {
		t.Errorf("exit rows, The Cache selected: want its boss and no lock line:\n%s", v)
	}
	// The real lock line names the module whole, wrapped rather than cut.
	fl = realFlavor
	defer func() { fl = stubFlavor }()
	e := env(fixture.MapRouteFinal())
	e.Module = func(string) string { return "Introduction to Graphs and Their Traversals" }
	v = flat(ansi.Strip(NewRoute(e).View(80, 24)))
	if !strings.Contains(v, "RTFM › Introduction to Graphs and Their Traversals opens it.") {
		t.Errorf("exit rows: the lock's module line is cut:\n%s", v)
	}
}

// The hub draws the path taken, where the boot stands and the bosses
// ahead, and its keys: enter resumes, n asks before y abandons, t opens
// RTFM, q quits. Upsolves are GRUB's, not the map's.
func TestHubKeys(t *testing.T) {
	h := NewHub(hubEnv(fixture.MapRoute()), true)
	v := ansi.Strip(h.View(80, 24))
	for _, want := range []string{"MAP", "suspended", "Userland", "The Scheduler", "Off-by-One", "act II", "The Halting Problem"} {
		if !strings.Contains(v, want) {
			t.Errorf("hub lacks %q:\n%s", want, v)
		}
	}
	key := func(k string) tea.Msg { _, cmd := h.Update(press(k)); return emitted(cmd) }
	for k, want := range map[string]tea.Msg{
		"enter": Resume{Launch: true}, "u": nil,
		"t": screens.OpenLearn{}, "q": screens.Quit{},
	} {
		if got := key(k); got != want {
			t.Errorf("%s: %#v, want %#v", k, got, want)
		}
	}
	if got := key("n"); got != nil {
		t.Errorf("n abandoned without asking: %#v", got)
	}
	if v := ansi.Strip(h.View(80, 24)); !strings.Contains(v, "Stub line for map.abandon.") || !strings.Contains(v, "ends boot #42 as a crash") {
		t.Errorf("n asks nothing:\n%s", v)
	}
	if got := key("x"); got != nil {
		t.Errorf("x answered the question: %#v", got)
	}
	key("n")
	if got := key("y"); got != (Abandon{}) {
		t.Errorf("n, y: %#v", got)
	}
	// Asked on another's behalf, y goes on to it.
	h.Ask(screens.StartTrial{Module: "two-pointers"})
	if got := key("y"); got != (Abandon{Then: screens.StartTrial{Module: "two-pointers"}}) {
		t.Errorf("y after Ask: %#v", got)
	}
	// Upsolves are GRUB's: u does nothing on the map, a dump and a due
	// regression at hand.
	st := fixture.MapDoors()
	e := hubEnv(st)
	e.Meta.Regressions = []game.Regression{{Problem: "usaco:1205", Due: e.Now.Add(-time.Hour)}}
	u := NewHub(e, true)
	if _, cmd := u.Update(tea.KeyPressMsg{Code: 'u', Text: "u"}); cmd != nil {
		t.Errorf("u on the map: %#v", cmd())
	}
	if v := ansi.Strip(u.View(140, 40)); strings.Contains(v, "upsolve") || strings.Contains(v, "regression due") {
		t.Errorf("the map offers an upsolve:\n%s", v)
	}
}

// The skip's two picks are the skip's: the pick screen at the final's
// doors does not show the last clear (Boss II's, its bytes long banked)
// as if just made, nor credit the picks to its -O3.
func TestSkipPicks(t *testing.T) {
	st := fixture.MapRouteFinal()
	st.History[len(st.History)-1].O3 = true
	st.Biomes[5] = game.BiomeState{Name: "skip", Kind: rules.Choice, Node: "skip", Exits: st.Biomes[5].Exits, Offers: st.Biomes[5].Offers}
	st.Biomes[6] = game.BiomeState{Name: "The Halting Problem", Topic: game.Mixed, Kind: rules.Boss, Rooms: 1, Boss: game.FinalBoss, Node: game.FinalBoss}
	st.Biome, st.Phase = 6, game.PhaseDoors
	st.PendingUpgrade, st.UpgradePicks = true, 2
	st.UpgradeOffers = []game.Upgrade{game.Overclock, game.ECC, game.Duck}
	v := ansi.Strip(NewUpgradeScreen(env(st)).View(140, 40))
	for _, bad := range []string{"✓ AC", "Snow Boots", "-O3"} {
		if strings.Contains(v, bad) {
			t.Errorf("after the skip the pick screen says %q:\n%s", bad, v)
		}
	}
	if !strings.Contains(v, "skip") || !strings.Contains(v, "+2 upgrade picks") {
		t.Errorf("after the skip the pick screen does not name it:\n%s", v)
	}
}

// The graph marks the exits on offer, the locks met ahead, the rooms lost
// and the one-shot final.
func TestGraph(t *testing.T) {
	g := newGraph(fixture.MapRoute())
	if g.offeredAt != 3 {
		t.Fatalf("exits offered at tier %d, want 3", g.offeredAt)
	}
	var offered []string
	for _, s := range g.tiers[3] {
		if s.offered {
			offered = append(offered, s.id)
		}
	}
	if strings.Join(offered, " ") != "bisect window alu" {
		t.Errorf("offered %v", offered)
	}
	for _, s := range g.tiers[6] {
		if !s.oneShot || !s.reach {
			t.Errorf("the final %+v", s)
		}
	}
	for _, s := range g.tiers[4] {
		if s.id == "undefined-behavior" && s.reach {
			t.Errorf("Undefined Behavior reachable from Off-by-One: %+v", s)
		}
	}
	if rooms := g.tiers[0][0].rooms; len(rooms) != 2 || rooms[0] != nodeDone || rooms[1] != nodeLost {
		t.Errorf("Userland's rooms %v, want cleared then lost", rooms)
	}
	// In the merge window the next tier's exits are on offer too.
	if g := newGraph(fixture.MapMerge()); g.offeredAt != 3 || g.merge != 2 {
		t.Errorf("merge window: offered at %d, after tier %d", g.offeredAt, g.merge)
	}
}
