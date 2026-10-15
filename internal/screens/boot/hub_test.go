package boot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot/fixture"
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
}

// The hub draws the path taken, where the boot stands and the bosses
// ahead, and its keys: enter resumes, n asks before y abandons, u
// upsolves the core dump, t opens RTFM, q quits.
func TestHubKeys(t *testing.T) {
	h := NewHub(hubEnv(fixture.MapRoute()), true)
	v := ansi.Strip(h.View(80, 24))
	for _, want := range []string{"MAP", "suspended", "Userland", "The Scheduler", "Off-by-One", "act II", "The Halting Problem", "312 B"} {
		if !strings.Contains(v, want) {
			t.Errorf("hub lacks %q:\n%s", want, v)
		}
	}
	key := func(k string) tea.Msg { _, cmd := h.Update(press(k)); return emitted(cmd) }
	for k, want := range map[string]tea.Msg{
		"enter": Resume{Launch: true}, "u": screens.Upsolve{Problem: "usaco:764"},
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
	// A dump on offer as a door is not upsolved from the hub.
	st := fixture.MapDoors()
	e := hubEnv(st)
	e.Meta.CoreDump.Problem = st.Doors[0].Problem
	e.Meta.Regressions = nil
	if id, _ := upsolveTarget(e); id != "" {
		t.Errorf("upsolves %s, a door on offer", id)
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
