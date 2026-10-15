package fixture

import (
	"slices"
	"strconv"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// The map boot (docs/design/map.md): boot #42 again, on rules.Map's three
// acts. The engine's exits are written here the way the contract has them
// (game.Exit): read off the graph, the boot's capability and package
// snapshots and its prestige, locked ones included.

// Exits are the exits of map node from in r's graph, for a boot holding
// caps and pkgs at prestige p, in the graph's order.
func Exits(r rules.Rules, from string, caps, pkgs []string, p int) []game.Exit {
	byID := map[string]rules.Biome{}
	for _, n := range r.Nodes {
		byID[n.ID] = n
	}
	var out []game.Exit
	for _, id := range byID[from].Next {
		n := byID[id]
		if n.Hidden { // the Trial: off the route until the engine shows it (TrialExit)
			continue
		}
		e := game.Exit{Node: n.ID, Name: n.Name, Topic: n.Topic, Rooms: n.Rooms, Boss: n.Boss, Picks: n.Picks, Skip: n.Skip}
		switch {
		case n.Skip:
			e.Topic, e.Rooms = "", 0
		case n.Topic == "":
			e.Topic = game.Mixed
		}
		if n.Boss == "" {
			for _, next := range n.Next {
				if b := byID[next]; b.Boss != "" && !b.Hidden { // no card names a secret boss ahead
					e.Bosses = append(e.Bosses, b.Boss)
				}
			}
		}
		e.Gate = game.Gate{Capability: game.CapabilityFor(n.Topic), Package: n.Package, Prestige: n.MinPrestige}
		capOK := e.Gate.Capability == "" || slices.Contains(caps, e.Gate.Capability) ||
			e.Gate.Package != "" && slices.Contains(pkgs, e.Gate.Package)
		switch {
		case n.MinPrestige > p:
			e.Locked = "prestige " + strconv.Itoa(n.MinPrestige)
		case !capOK && e.Gate.Package != "":
			e.Locked = e.Gate.Capability + " or apt " + e.Gate.Package
		case !capOK:
			e.Locked = e.Gate.Capability
		}
		out = append(out, e)
	}
	return out
}

// mapBoot is boot #42 on the map, 50 minutes in: Userland cleared but for
// a room lost to the timer, The Scheduler cleared, and Off-by-One beaten;
// the merge window after it filled the fountain. It stands at the route
// into act II. caps are the capabilities the boot began with.
func mapBoot(caps ...string) *game.State {
	r := rules.Map()
	start := Now.Add(-at(50, 0))
	userland := Exits(r, "userland", caps, nil, 1)
	st := &game.State{
		Boot: 42, Seed: 0x8f3a, Rules: r, Prestige: 1, Started: start,
		Phase: game.PhaseRoute,
		Biomes: []game.BiomeState{
			{Name: "Userland", Topic: game.Mixed, Kind: rules.Fixed, Rooms: 2, Node: "userland"},
			{Name: "The Scheduler", Topic: "sorting-greedy", Kind: rules.Choice, Rooms: 1, Node: "scheduler",
				Offers: topics(userland), Exits: userland},
			{Name: "Off-by-One", Topic: game.Mixed, Kind: rules.Boss, Rooms: 1, Boss: "off-by-one", Node: "off-by-one",
				Offers: []string{game.Mixed}, Exits: Exits(r, "scheduler", caps, nil, 1)},
			{Kind: rules.Choice, Rooms: 1, Exits: Exits(r, "off-by-one", caps, nil, 1)},
			{Kind: rules.Boss, Rooms: 1},
			{Kind: rules.Choice, Rooms: 1},
			{Kind: rules.Boss, Rooms: 1},
		},
		Biome: 3, Room: 0,
		History: []game.Attempt{
			attempt(start, door(cowTipping, 10*time.Minute, 48, 0.92), at(0, 30), at(7, 50), game.Cleared),
			attempt(start, door(haybales, 10*time.Minute, 48, 0.9), at(8, 10), at(18, 10), game.TimedOut),
			attempt(start, door(milkMeasure, 21*time.Minute, 96, 0.75), at(19, 0), at(31, 31), game.Cleared),
			attempt(start, boss(moocast, 32*time.Minute, 320, 0.55, "off-by-one"), at(33, 0), at(48, 2), game.Cleared),
		},
		Cycles: 23, MaxCycles: 50, Ducks: 1,
		Committed:  512,
		Upgrades:   []game.Upgrade{game.Overclock},
		Daemons:    []game.Daemon{game.Strace},
		Caps:       caps,
		Fountained: 5,
		Log: []game.LogLine{
			{At: start, Text: "boot: prestige 1, preset standard, seed 8f3a"},
			{At: start.Add(at(7, 50)), Text: `door_1: AC "Cow Tipping" +48 B`},
			{At: start.Add(at(18, 10)), Text: `door_2: timer expired, burned 10 cycles, 20 left · room lost`},
			{At: start.Add(at(31, 31)), Text: `door_3: AC "Milk Measurement" +96 B`},
			{At: start.Add(at(48, 2)), Text: `door_4: AC "Moocast" +320 B`},
			{At: start.Add(at(49, 0)), Text: "merge: committed 512 B, fountain +5 cycles"},
		},
	}
	st.Biomes[3].Offers = topics(st.Biomes[3].Exits)
	return st
}

func boss(id catalog.ID, timer time.Duration, bytes int64, p float64, key string) game.Door {
	d := door(id, timer, bytes, p)
	d.Boss = key
	return d
}

// topics are the exits' topics, as BiomeState.Offers keeps them.
func topics(es []game.Exit) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Topic
	}
	return out
}

// MapRoute is the route into act II after Off-by-One: The ALU open, Bisect
// Bridge locked behind CAP_BSEARCH or apt git and The Sliding Window
// behind CAP_TWOPTR, each with a +1 pick.
func MapRoute() *game.State { return mapBoot() }

// MapRouteBoss is the route into Boss I from Brute Force Bunker: its two
// exits are bosses, Off-by-One and the OOM Killer.
func MapRouteBoss() *game.State {
	st := mapBoot()
	st.Biomes[1].Name, st.Biomes[1].Topic, st.Biomes[1].Node = "Brute Force Bunker", "complete-search", "bunker"
	st.Biomes[2] = game.BiomeState{Kind: rules.Boss, Rooms: 1, Exits: Exits(st.Rules, "bunker", nil, nil, 1)}
	st.Biomes[2].Offers = topics(st.Biomes[2].Exits)
	st.Biomes[3] = game.BiomeState{Kind: rules.Choice, Rooms: 1}
	st.Biome = 2
	st.History = st.History[:3]
	st.Committed, st.Uncommitted, st.Cycles, st.Fountained = 0, 192, 18, 0
	st.Log = st.Log[:4]
	rebase(st, at(32, 10))
	return st
}

// MapRouteFinal is the route into act III after Heisenbug, the boot holding
// CAP_DP: Routing Table locked, The Cache open with its pick, Kernel Space,
// and the skip for two picks.
func MapRouteFinal() *game.State {
	st := mapBoot("CAP_DP")
	start := st.Started
	st.Biomes[3] = game.BiomeState{Name: "The ALU", Topic: "math", Kind: rules.Choice, Rooms: 1, Node: "alu",
		Exits: st.Biomes[3].Exits, Offers: st.Biomes[3].Offers}
	st.Biomes[4] = game.BiomeState{Name: "Heisenbug", Topic: game.Mixed, Kind: rules.Boss, Rooms: 1, Boss: "heisenbug", Node: "heisenbug",
		Exits: Exits(st.Rules, "alu", st.Caps, nil, 1)}
	st.Biomes[4].Offers = topics(st.Biomes[4].Exits)
	st.Biomes[5].Exits = Exits(st.Rules, "heisenbug", st.Caps, nil, 1)
	st.Biomes[5].Offers = topics(st.Biomes[5].Exits)
	st.History = append(st.History,
		attempt(start, door(moobuzz, 24*time.Minute, 128, 0.7), at(52, 0), at(70, 12), game.Cleared),
		attempt(start, boss(cowPoetry, 36*time.Minute, 576, 0.45, "heisenbug"), at(72, 0), at(108, 0), game.TimedOut),
		attempt(start, boss(snowBoots, 36*time.Minute, 576, 0.45, "heisenbug"), at(109, 0), at(131, 40), game.Cleared),
	)
	st.Biome = 5
	st.Cycles, st.Committed, st.Fountained = 8, 1216, 10
	st.Log = append(st.Log,
		game.LogLine{At: start.Add(at(70, 12)), Text: `door_5: AC "MooBuzz" +128 B`},
		game.LogLine{At: start.Add(at(108, 0)), Text: `door_6: timer expired, burned 20 cycles, 3 left · a new boss door`},
		game.LogLine{At: start.Add(at(131, 40)), Text: `door_7: AC "Snow Boots" +576 B`},
	)
	rebase(st, at(133, 0))
	return st
}

// MapRoutePrestige is a Bronze boot at the route into Boss II from The
// Heap: Heisenbug open, Undefined Behavior behind prestige 1.
func MapRoutePrestige() *game.State {
	st := mapBoot("CAP_DS")
	st.Prestige = 0
	st.Biomes[3] = game.BiomeState{Name: "The Heap", Topic: "data-structures", Kind: rules.Choice, Rooms: 1, Node: "heap",
		Exits: Exits(st.Rules, "oom-killer", st.Caps, nil, 0)}
	st.Biomes[3].Offers = topics(st.Biomes[3].Exits)
	st.Biomes[2].Name, st.Biomes[2].Boss, st.Biomes[2].Node = "OOM Killer", "oom-killer", "oom-killer"
	st.Biomes[2].Exits = Exits(st.Rules, "emulator", st.Caps, nil, 0)
	st.Biomes[1].Name, st.Biomes[1].Topic, st.Biomes[1].Node = "The Emulator", "simulation", "emulator"
	st.Biomes[1].Exits = Exits(st.Rules, "userland", st.Caps, nil, 0)
	st.Biomes[4].Exits = Exits(st.Rules, "heap", st.Caps, nil, 0)
	st.Biomes[4].Offers = topics(st.Biomes[4].Exits)
	start := st.Started
	st.History = append(st.History,
		attempt(start, door(cereal, 24*time.Minute, 128, 0.7), at(52, 0), at(66, 30), game.Cleared))
	st.History[3].Door.Boss = "oom-killer"
	st.Biome = 4
	rebase(st, at(67, 0))
	return st
}

// MapDoors is the boot at the doors of The ALU, act II's one room.
func MapDoors() *game.State {
	st := mapBoot()
	st.Biomes[3].Name, st.Biomes[3].Topic, st.Biomes[3].Node = "The ALU", "math", "alu"
	st.Biomes[4].Exits = Exits(st.Rules, "alu", nil, nil, 1)
	st.Biomes[4].Offers = topics(st.Biomes[4].Exits)
	st.Phase = game.PhaseDoors
	st.Doors = []game.Door{door(moobuzz, 24*time.Minute, 128, 0.7), door(watermelon, 24*time.Minute, 96, 0.74)}
	return st
}

// MapMerge is the merge window after Off-by-One, its fountain just drunk.
func MapMerge() *game.State {
	st := mapBoot()
	st.Phase = game.PhaseMerge
	st.Biome, st.Room = 2, 1
	st.Uncommitted, st.Committed = 464, 48
	st.Fountain = 5
	return st
}

// MapRouteTrial is the route into Boss II from The ALU, act II's room
// cleared, with the Trial's exit among Heisenbug's and Race Condition's:
// locked behind its ??? while the pool is short of its cap (34/50) and
// once its hint was met (Meta.Capped); open at 50/50.
func MapRouteTrial(open bool) *game.State {
	st := MapRouteFinal()
	exits := append(Exits(st.Rules, "alu", st.Caps, nil, 1), TrialExit(st.Rules, open))
	st.Biomes[4] = game.BiomeState{Kind: rules.Boss, Rooms: 1, Exits: exits, Offers: topics(exits)}
	st.Biomes[5] = game.BiomeState{Kind: rules.Choice, Rooms: 1}
	st.History = st.History[:5]
	st.Biome = 4
	st.Phase = game.PhaseRoute
	st.Cycles, st.Committed, st.Fountained = 34, 1216, 10
	if open {
		st.Cycles, st.Capped = 50, true
	}
	st.Log = st.Log[:7]
	rebase(st, at(71, 0))
	return st
}

// TrialExit is the Trial's exit as the engine writes it (game.Exit): hidden,
// the boot's end, gated on a full pool, and locked behind its ??? unless
// open.
func TrialExit(r rules.Rules, open bool) game.Exit {
	n, _ := r.Node(rules.TrialNode)
	e := game.Exit{Node: n.ID, Name: n.Name, Topic: game.Mixed, Rooms: n.Rooms, Boss: n.Boss,
		Hidden: n.Hidden, Ending: n.Ending, Gate: game.Gate{Full: n.NeedFull}}
	if !open {
		e.Locked = game.LockedFull
	}
	return e
}

// TrialDoor is the Trial's one door, cursed and one shot, at its room's
// doors (game.TrialBoss).
func TrialDoor() *game.State {
	st := MapRouteTrial(true)
	st.Phase = game.PhaseDoors
	st.Biomes[4] = game.BiomeState{Name: "The Trial", Topic: game.Mixed, Kind: rules.Boss, Rooms: 1, Boss: game.TrialBoss,
		Node: rules.TrialNode, Exits: st.Biomes[4].Exits, Offers: st.Biomes[4].Offers}
	st.Rules.Biomes = slices.Clone(st.Rules.Biomes)
	st.Rules.Biomes[4].FailCycles = 0 // the node's, as the engine applies it: one shot
	d := boss(cowPoetry, 36*time.Minute, 576, 0.55, game.TrialBoss)
	d.Cursed = true
	st.Doors = []game.Door{d}
	return st
}

// TrialAwake is the boot the Trial ended, awake (game.EndingWake): no act
// III, no final, no prestige unlocked.
func TrialAwake() *game.State {
	st := TrialDoor()
	start := st.Started
	d := st.Doors[0]
	a := attempt(start, d, at(72, 0), at(99, 48), game.Cleared)
	a.Biome = 4
	st.History = append(st.History, a)
	st.Doors = nil
	st.Phase, st.Ending, st.Room = game.PhaseCleared, game.EndingWake, 1
	st.Committed += st.Uncommitted + d.Bytes
	st.Uncommitted = 0
	rebase(st, at(100, 0))
	return st
}
