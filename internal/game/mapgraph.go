package game

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"katharsis/internal/rules"
	"katharsis/internal/units"
)

// The run map (docs/design/map.md §(c), §(h)): a boot laid out on a graph
// of biomes in tiers (rules.Rules.Nodes). State.Biomes keeps one entry per
// tier, so every index the engine, the screens, the store and the debrief
// use stays valid; a tier's node is written into the boot's rules snapshot
// when its exit is taken, as a trial writes its sizes (trial.go).
//
//	Userland ─ 1 of 4 ─ ⚔ I ── 1 of 3 ─ ⚔ II ── 1 of 3 or skip ─ ⚔ final
//	tier 0     1        2      3        4       5                6
//
// Exits are read off the rules snapshot, State.Caps, State.Packages, the
// prestige and the pool: they draw no random number, so a boot's RNG
// stream is as it would be without them, and the route offers log them
// with propensity 1, a fixed policy.

// mapped reports whether the boot plays a map.
func (s *State) mapped() bool { return len(s.Rules.Nodes) > 0 }

// layoutMap lays out a map boot: a BiomeState per tier of the skeleton,
// and tier 0 playing the start node at once.
func (e *engine) layoutMap() {
	s, r := e.s, &e.s.Rules
	for _, rb := range r.Biomes {
		s.Biomes = append(s.Biomes, BiomeState{Name: rb.Name, Kind: rb.Kind, Rooms: rb.Rooms})
	}
	i := slices.IndexFunc(r.Nodes, func(n rules.Biome) bool { return n.Tier == 0 })
	start := r.Nodes[i]
	e.applyNode(0, start.ID)
	x := e.exitOf(start, 0)
	s.Biomes[0].Exits = []Exit{x}
	s.Biomes[0].Offers = []string{s.Biomes[0].Topic}
	e.logExits(0, s.Biomes[0].Exits, 0)
}

// applyNode makes tier i play node id: the node goes into the rules
// snapshot (its tier's kind kept) and names the tier.
func (e *engine) applyNode(i int, id string) {
	s := e.s
	n, _ := s.Rules.Node(id)
	n.Kind = s.Rules.Biomes[i].Kind
	s.Rules.Biomes[i] = n
	b := &s.Biomes[i]
	b.Node, b.Name, b.Rooms, b.Boss = id, n.Name, n.Rooms, n.Boss
	b.Topic = n.Topic
	if b.Topic == "" {
		b.Topic = Mixed
	}
	if n.Skip {
		b.Rooms, b.Topic = 0, ""
	}
}

// enterMapTier offers tier i's exits, or takes its only exit, and reports
// whether it did either: false when tier i already has its node or the
// boot has no map. One open exit among locked ones is still a route: a
// locked exit stays on screen with the RTFM module that opens it (map.md
// §(c)).
func (e *engine) enterMapTier(i int) bool {
	s := e.s
	b := &s.Biomes[i]
	if !s.mapped() || b.Node != "" || i == 0 {
		return false
	}
	ex := e.exits(i)
	b.Exits = ex
	b.Offers = make([]string, len(ex))
	for j, x := range ex {
		b.Offers[j] = x.Topic
	}
	if len(ex) == 1 { // open: exits opens a plain exit when none is
		e.logExits(i, ex, 0)
		e.takeNode(i, ex[0].Node)
		return true
	}
	e.logExits(i, ex, -1)
	s.Phase = PhaseRoute
	names := make([]string, len(ex))
	for j, x := range ex {
		names[j] = x.Node
		if x.Locked != "" {
			names[j] += " [" + x.Locked + "]"
		}
	}
	e.logf("route: biome_%d exits %s", i, strings.Join(names, ", "))
	return true
}

// takeNode plays node id at tier i: a skip owes its picks and enters the
// next tier at once; any other node is announced and its first room
// drawn.
func (e *engine) takeNode(i int, id string) {
	s := e.s
	e.applyNode(i, id)
	if n := s.Rules.Biomes[i]; n.Skip {
		e.logf("route: biome_%d skipped by %s, %s", i, id, units.Plural(n.Picks, "upgrade pick"))
		e.owe(n.Picks)
		e.enterBiome(i + 1)
		return
	}
	e.logf("route: biome_%d takes %s (%s, %s)", i, id, s.Biomes[i].Topic, units.Plural(s.Biomes[i].Rooms, "room"))
	e.openBiome(i)
}

// exits are the exits into tier i from the node tier i-1 took, in the
// graph's order, locked ones included. A hidden node's exit (the Trial)
// is left out while locked, until the player has met its gate's hint
// (Meta.Capped): before that the route never names it. When no exit but a
// hidden one is open, the first other is opened: a boot never stalls, and
// an open Trial never stands alone, so it is never the one exit taken at
// once.
func (e *engine) exits(i int) []Exit {
	s := e.s
	from, _ := s.Rules.Node(s.Biomes[i-1].Node)
	var out []Exit
	for _, id := range from.Next {
		n, _ := s.Rules.Node(id)
		x := e.exitOf(n, i)
		if x.Hidden && x.Locked != "" && e.meta.Capped == 0 {
			continue
		}
		out = append(out, x)
	}
	plain := func(x Exit) bool { return !x.Hidden }
	if j := slices.IndexFunc(out, plain); j >= 0 && !slices.ContainsFunc(out, func(x Exit) bool { return plain(x) && x.Locked == "" }) {
		out[j].Locked = ""
	}
	return out
}

// exitOf is node n's exit card as tier i's.
func (e *engine) exitOf(n rules.Biome, i int) Exit {
	s := e.s
	x := Exit{Node: n.ID, Name: n.Name, Topic: n.Topic, Rooms: n.Rooms, Boss: n.Boss, Picks: n.Picks, Skip: n.Skip,
		Hidden: n.Hidden, Ending: n.Ending}
	if x.Topic == "" {
		x.Topic = Mixed
	}
	if n.Skip {
		x.Topic, x.Rooms = "", 0
	}
	if n.Boss == "" {
		for _, id := range n.Next {
			// A hidden boss is no boss the card names ahead.
			if m, ok := s.Rules.Node(id); ok && m.Boss != "" && !m.Hidden && !slices.Contains(x.Bosses, m.Boss) {
				x.Bosses = append(x.Bosses, m.Boss)
			}
		}
	}
	x.Gate = Gate{Capability: CapabilityFor(n.Topic), Package: n.Package, Prestige: n.MinPrestige, Full: n.NeedFull}
	if x.Gate.Capability == "" {
		x.Gate.Package = "" // a package opens only what a capability closes
	}
	x.Locked = e.locked(n, x.Gate, i)
	if !n.Skip {
		topic := x.Topic
		x.Timer = e.doorTimer(n, n.Boss != "", e.target(topic, i)-PlayerRating(&s.Rules, e.meta, topic))
		if tw := s.Rules.Twists; n.Boss == "race-condition" && tw.RaceClock > 0 {
			x.Timer = roundTimer(scale(x.Timer, tw.RaceClock))
		}
	}
	return x
}

// locked is why node n cannot be taken as tier i, "" when it can: a
// prestige short of its gate, its capability neither earned at boot start
// (State.Caps) nor opened by an installed package, a cycle pool short of
// its cap where the gate wants it full (LockedFull), or too few problems
// for its rooms near the player's aim. The pool is read as the tier is
// entered and draws no random number.
func (e *engine) locked(n rules.Biome, g Gate, i int) string {
	s := e.s
	if s.Prestige < g.Prestige {
		return "prestige " + strconv.Itoa(g.Prestige)
	}
	if full := s.MaxCycles; g.Full {
		if s.perks() {
			// The rules' cap: cpupower lowers the boot's, selinux fills it,
			// and either would open the Court for nothing.
			full = s.courtGate()
		}
		if s.Cycles < full {
			return LockedFull
		}
	}
	if c := g.Capability; c != "" && !slices.Contains(s.Caps, c) && (g.Package == "" || !slices.Contains(s.Packages, g.Package)) {
		if g.Package != "" {
			return c + " or apt " + g.Package
		}
		return c
	}
	if !e.stocked(n, i) {
		return "pool"
	}
	return ""
}

// stocked reports whether the pool holds rooms·RouteMinPool problems of
// n's topic within the widest rating window of tier i's aim, as
// routeOffers asks of a topic. A mixed node, a boss and a skip always are.
func (e *engine) stocked(n rules.Biome, i int) bool {
	if n.Topic == "" || n.Topic == Mixed || n.Boss != "" || n.Skip {
		return true
	}
	r := &e.s.Rules
	widest := float64(slices.Max(r.RatingWindows))
	tg := e.target(n.Topic, i)
	ex := e.excluded()
	c := 0
	for j := range e.pool {
		p := &e.pool[j]
		if p.Judgeable && !ex[p.ID] && topicOf(p) == n.Topic && math.Abs(difficulty(r, p)-tg) <= widest {
			if c++; c >= n.Rooms*r.RouteMinPool {
				return true
			}
		}
	}
	return false
}

// logExits logs tier i's exits as a route offer, each with propensity 1
// (the graph is fixed, not drawn); a locked exit is logged in slot
// "locked". chosen is the exit taken at once, -1 when the player chooses.
func (e *engine) logExits(i int, ex []Exit, chosen int) {
	s := e.s
	topics := make([]string, len(ex))
	props := make([]float64, len(ex))
	for j, x := range ex {
		topics[j], props[j] = x.Topic, 1
	}
	e.logRoute(i, topics, props, 1, s.RNG, chosen)
	o := &s.Offers[len(s.Offers)-1]
	for j, x := range ex {
		o.Items[j].Node = x.Node
		if x.Locked != "" {
			o.Items[j].Slot = "locked"
		}
	}
}

// chooseExit takes exit i of the route on offer in a map boot.
func (e *engine) chooseExit(i int) error {
	s := e.s
	b := &s.Biomes[s.Biome]
	if i < 0 || i >= len(b.Exits) {
		return invalid("exit %d of %d", i, len(b.Exits))
	}
	if x := b.Exits[i]; x.Locked != "" {
		return invalid("exit %s is locked: %s", x.Node, x.Locked)
	}
	e.choose("route", i)
	e.takeNode(s.Biome, b.Exits[i].Node)
	return nil
}
