package game_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// mapPlain is the map without easing, door modifiers or boot #1's rules.
func mapPlain() rules.Rules {
	r := calm(rules.Map())
	r.BlindChance, r.MysteryChance, r.CursedChance = 0, 0, 0
	return r
}

// exitIndex is the index of node id among the route's exits.
func exitIndex(t *testing.T, s *game.State, id string) int {
	t.Helper()
	i := slices.IndexFunc(s.Biomes[s.Biome].Exits, func(x game.Exit) bool { return x.Node == id })
	if i < 0 {
		t.Fatalf("no exit %s in %+v", id, s.Biomes[s.Biome].Exits)
	}
	return i
}

// nextTier clears the current biome and leaves its merge window.
func nextTier(h *harness) {
	h.t.Helper()
	toMergeFast(h)
	h.pick()
	h.do(game.LeaveMerge{})
}

func TestMapLayout(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, mapPlain(), &game.Meta{}, ps, 0, 1)
	s := h.s
	if len(s.Biomes) != 7 || s.Biomes[0].Node != "userland" || s.Biomes[0].Rooms != 2 || s.Phase != game.PhaseDoors {
		t.Fatalf("layout %+v phase %s", s.Biomes, s.Phase)
	}
	for i := 1; i < 7; i++ {
		if s.Biomes[i].Node != "" || s.Biomes[i].Kind != s.Rules.Biomes[i].Kind {
			t.Errorf("tier %d laid out %+v before its exit", i, s.Biomes[i])
		}
	}
	nextTier(h)
	b := s.Biomes[1]
	if s.Phase != game.PhaseRoute || len(b.Exits) != 4 || len(b.Offers) != 4 {
		t.Fatalf("act I: phase %s exits %+v", s.Phase, b.Exits)
	}
	want := map[string]string{"bunker": "", "emulator": "", "scheduler": "", "accumulator": "CAP_PREFIX"}
	for i, x := range b.Exits {
		if lock, ok := want[x.Node]; !ok || x.Locked != lock || b.Offers[i] != x.Topic {
			t.Errorf("exit %+v (offer %q)", x, b.Offers[i])
		}
	}
	if x := b.Exits[exitIndex(t, s, "bunker")]; !slices.Equal(x.Bosses, []string{"off-by-one", "oom-killer"}) || x.Rooms != 1 || x.Picks != 0 {
		t.Errorf("bunker %+v", x)
	}
	if x := b.Exits[exitIndex(t, s, "accumulator")]; x.Gate.Capability != "CAP_PREFIX" || x.Picks != 1 {
		t.Errorf("accumulator %+v", x)
	}
	o := s.Offers[len(s.Offers)-1]
	if o.Kind != "route" || o.Chosen != -1 || len(o.Items) != 4 || o.Items[3].Slot != "locked" || o.Items[0].Node != "bunker" {
		t.Errorf("route offer %+v", o)
	}
	for _, it := range o.Items {
		if it.Propensity != 1 {
			t.Errorf("exit %s logged with propensity %v", it.Node, it.Propensity)
		}
	}
	if err := h.try(game.ChooseRoute{Index: exitIndex(t, s, "accumulator")}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("a locked exit taken: %v", err)
	}
	// The Emulator leads only to the OOM Killer: its exit is taken at once.
	h.do(game.ChooseRoute{Index: exitIndex(t, s, "emulator")})
	if s.Biomes[1].Node != "emulator" || s.Biomes[1].Topic != "simulation" || s.Phase != game.PhaseDoors || s.Rules.Biomes[1].ID != "emulator" {
		t.Fatalf("took emulator: %+v phase %s", s.Biomes[1], s.Phase)
	}
	nextTier(h)
	if s.Biome != 2 || s.Biomes[2].Node != "oom-killer" || s.Biomes[2].Boss != "oom-killer" || len(s.Doors) != 1 ||
		s.Doors[0].Boss != "oom-killer" || s.Doors[0].Twist != "oom-killer" {
		t.Fatalf("boss I: %+v doors %+v", s.Biomes[2], s.Doors)
	}
	if lo := s.Offers[len(s.Offers)-2]; lo.Kind != "route" || lo.Chosen != 0 || len(lo.Items) != 1 {
		t.Errorf("the one exit's offer %+v", lo)
	}
}

// TestMapGates: a capability opens its biome, as does git for Bisect
// Bridge; Undefined Behavior waits for P1 and The Framebuffer for P2; and
// the exits read the capabilities the boot began with.
func TestMapGates(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	act2 := func(m *game.Meta, prestige int) *harness {
		h := newHarness(t, mapPlain(), m, ps, prestige, 2)
		nextTier(h)
		h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")}) // to Off-by-One
		nextTier(h)
		nextTier(h) // Off-by-One cleared
		return h
	}
	h := act2(&game.Meta{Capabilities: []string{"CAP_TWOPTR"}}, 0)
	if h.s.Phase != game.PhaseRoute {
		t.Fatalf("act II: phase %s", h.s.Phase)
	}
	locks := map[string]string{}
	for _, x := range h.s.Biomes[3].Exits {
		locks[x.Node] = x.Locked
	}
	if locks["bisect"] != "CAP_BSEARCH or apt git" || locks["window"] != "" || locks["alu"] != "" {
		t.Errorf("act II locks %v", locks)
	}
	// A capability granted mid-boot opens nothing in it.
	game.GrantCapability(h.m, "CAP_BSEARCH")
	if _, err := game.Step(h.s, h.m, h.ps, game.ChooseRoute{Index: exitIndex(t, h.s, "bisect")}, h.now); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("a trial passed mid-boot opened Bisect Bridge: %v", err)
	}

	h = act2(&game.Meta{Installed: []string{"git"}}, 0)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "bisect")})
	if h.s.Biomes[3].Node != "bisect" {
		t.Fatalf("git did not open Bisect Bridge: %+v", h.s.Biomes[3])
	}
	// Clearing a gated biome pays its scroll at the exit: two picks (past
	// half the timer, so no -O3 pick).
	h.clearDoor(0, 13*time.Minute)
	if h.s.UpgradePicks != 2 || h.s.Phase != game.PhaseMerge {
		t.Errorf("Bisect Bridge cleared: %d picks owed, phase %s", h.s.UpgradePicks, h.s.Phase)
	}

	all := &game.Meta{Prestige: 1}
	for _, c := range game.TopicCapabilities {
		game.GrantCapability(all, c)
	}
	for _, p := range []int{0, 1} {
		h = newHarness(t, mapPlain(), all, ps, p, 3)
		nextTier(h)
		h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "emulator")})
		nextTier(h)
		nextTier(h) // the OOM Killer
		h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "heap")})
		nextTier(h)
		ub := h.s.Biomes[4].Exits[exitIndex(t, h.s, "undefined-behavior")]
		if want := map[int]string{0: "prestige 1", 1: ""}[p]; ub.Locked != want {
			t.Errorf("P%d: Undefined Behavior locked %q, want %q", p, ub.Locked, want)
		}
	}
}

// TestMapSkip: every Boss II exits to a skip past act III: no room, two
// upgrade picks, then The Halting Problem.
func TestMapSkip(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, mapPlain(), &game.Meta{}, ps, 0, 4)
	nextTier(h)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")})
	nextTier(h)
	nextTier(h) // Off-by-One; The ALU is its one open exit, taken at once
	if h.s.Biomes[3].Node != "alu" {
		t.Fatalf("act II took %q", h.s.Biomes[3].Node)
	}
	nextTier(h)
	if h.s.Phase != game.PhaseRoute || h.s.Biome != 4 {
		t.Fatalf("after The ALU: phase %s biome %d", h.s.Phase, h.s.Biome)
	}
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "heisenbug")})
	nextTier(h)
	x := h.s.Biomes[5].Exits[exitIndex(t, h.s, "skip")]
	if !x.Skip || x.Picks != 2 || x.Rooms != 0 || x.Topic != "" || x.Locked != "" || !slices.Equal(x.Bosses, []string{game.FinalBoss}) {
		t.Errorf("skip exit %+v", x)
	}
	picks := h.s.UpgradePicks
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "skip")})
	if h.s.Biome != 6 || h.s.Biomes[5].Rooms != 0 || h.s.Biomes[6].Boss != game.FinalBoss || h.s.UpgradePicks != picks+2 ||
		h.s.Phase != game.PhaseDoors || !h.s.OneShot() {
		t.Fatalf("skipped: biome %d tiers %+v picks %d phase %s", h.s.Biome, h.s.Biomes[5:], h.s.UpgradePicks, h.s.Phase)
	}
	h.pick()
	h.clearDoor(0, time.Minute)
	if h.s.Phase != game.PhaseCleared {
		t.Errorf("the final cleared: phase %s", h.s.Phase)
	}
}

// TestMapFirstBoot: boot #1 plays the short shape off the map (RT11); the
// map starts at boot 2.
func TestMapFirstBoot(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	m := &game.Meta{}
	s, _, err := game.NewBoot(rules.Map(), m, ps, 0, 1, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Rules.Nodes) != 0 || len(s.Biomes) != 3 || s.Biomes[0].Node != "" || s.Rules.Biomes[0].FailCycles != 10 {
		t.Errorf("boot #1: nodes %d, biomes %+v", len(s.Rules.Nodes), s.Biomes)
	}
	s2, _, err := game.NewBoot(rules.Map(), m, ps, 0, 2, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Biomes) != 7 || s2.Biomes[0].Node != "userland" {
		t.Errorf("boot #2: biomes %+v", s2.Biomes)
	}
}

// TestMapUIContract pins what the map hub's screens read (coordinator,
// 10-15): every tier laid out at boot start with tier 0 on its node, a map
// told by len(Rules.Nodes) > 0 && len(Biomes) == len(Rules.Biomes) (so
// boot #1's linear snapshot carries no nodes); during a route, Biome is the
// tier being entered with its Exits written, kept after the choice; taking
// a node names the tier; Gate is set on open exits too, Locked only when
// the exit cannot be taken; Bosses on topic nodes and the skip; FailCost
// and OneShot read the routed tier; every attempt knows its biome.
func TestMapUIContract(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	isMap := func(s *game.State) bool { return len(s.Rules.Nodes) > 0 && len(s.Biomes) == len(s.Rules.Biomes) }
	m := &game.Meta{Installed: []string{"git"}}
	first, _, err := game.NewBoot(rules.Map(), m, ps, 0, 9, t0)
	if err != nil || isMap(first) || len(first.Rules.Nodes) != 0 {
		t.Fatalf("boot #1 reads as a map: %v nodes %d", err, len(first.Rules.Nodes))
	}
	h := newHarness(t, mapPlain(), m, ps, 0, 9)
	s := h.s
	if !isMap(s) || s.Biomes[0].Node != "userland" || len(s.Biomes) != 7 {
		t.Fatalf("map boot: %+v", s.Biomes)
	}
	routes := 0
	for range 200 {
		if s.Phase == game.PhaseCrashed || s.Phase == game.PhaseCleared {
			break
		}
		switch {
		case s.PendingUpgrade:
			h.pick()
		case s.Phase == game.PhaseRoute:
			routes++
			b := s.Biomes[s.Biome]
			if b.Node != "" || len(b.Exits) == 0 || len(b.Offers) != len(b.Exits) {
				t.Fatalf("route into tier %d: %+v", s.Biome, b)
			}
			fc, ok := s.FailCost(game.TimedOut)
			if want := s.Rules.Biomes[s.Biome].FailCycles; s.OneShot() != (s.Biome == len(s.Biomes)-1) ||
				!s.OneShot() && (fc != want || ok != (s.Cycles > fc)) {
				t.Errorf("route into tier %d: FailCost %d %v (want %d), one shot %v", s.Biome, fc, ok, want, s.OneShot())
			}
			open := -1
			for j, x := range b.Exits {
				if n, _ := s.Rules.Node(x.Node); game.CapabilityFor(n.Topic) != "" && x.Gate.Capability == "" {
					t.Errorf("exit %s: no gate", x.Node)
				}
				if x.Boss == "" && len(x.Bosses) == 0 {
					t.Errorf("exit %s: no boss behind it", x.Node)
				}
				if x.Locked != "" {
					if _, err := game.Step(s, h.m, h.ps, game.ChooseRoute{Index: j}, h.now); err == nil {
						t.Fatalf("locked exit %s taken", x.Node)
					}
				} else if open < 0 {
					open = j
				}
			}
			want := b.Exits
			h.do(game.ChooseRoute{Index: open})
			nb := s.Biomes[s.Biome-boolInt(want[open].Skip)]
			n, _ := s.Rules.Node(want[open].Node)
			if nb.Node != n.ID || nb.Name != n.Name || nb.Boss != n.Boss || len(nb.Exits) != len(want) {
				t.Errorf("took %s: tier %+v", n.ID, nb)
			}
		case s.Phase == game.PhaseDoors:
			h.clearDoor(0, time.Minute)
		case s.Phase == game.PhaseMerge:
			h.do(game.LeaveMerge{})
		}
	}
	for _, a := range s.History {
		if a.Biome < 0 || a.Biome >= len(s.Biomes) || s.Biomes[a.Biome].Node == "" {
			t.Errorf("attempt %+v in biome %d", a.Door, a.Biome)
		}
	}
	if s.Phase != game.PhaseCleared || routes == 0 {
		t.Errorf("phase %s after %d routes", s.Phase, routes)
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
