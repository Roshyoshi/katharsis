package fixture

import (
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// The map boot (D52): boot #57 at Silver on the three-act map, priced in
// cycles (rules.Map), as the engine plausibly holds it at each of the
// in-door screens the cycles goldens pin. Like the rest of the package it
// runs no engine; the twists' flags sit on the boss doors as the engine
// sets them (game.Door.Twist).

// cyclesTimer is tier t's base clock (rules.MapBiomes).
func cyclesTimer(t int) time.Duration { return rules.MapBiomes()[t].Timer.Duration }

// cyclesRooms are the problems the map boot's doors play, in order.
var cyclesRooms = []catalog.ID{cowTipping, haybales, milkMeasure, cowPoetry, moobuzz, closingFarm, bovine, snowBoots}

// cyclesBoot is the map boot at the doors of tier's first room, every room
// before it cleared, 12 minutes a door: 24 of its 30 cycles left (two
// failed submits and a retreat on the way), one Overclock picked.
func cyclesBoot(tier int) *game.State {
	start := Now.Add(-time.Hour)
	st := &game.State{
		Boot: 57, Seed: 0x51c3, Rules: rules.Map(), Prestige: 1, Started: start,
		Phase: game.PhaseDoors,
		Biomes: []game.BiomeState{
			{Name: "Userland", Topic: game.Mixed, Kind: rules.Fixed, Rooms: 2, Node: "userland"},
			{Name: "The Scheduler", Topic: "sorting-greedy", Kind: rules.Choice, Rooms: 1, Node: "scheduler"},
			{Topic: game.Mixed, Kind: rules.Boss, Rooms: 1, Boss: "off-by-one", Node: "off-by-one"},
			{Name: "The ALU", Topic: "math", Kind: rules.Choice, Rooms: 1, Node: "alu"},
			{Topic: game.Mixed, Kind: rules.Boss, Rooms: 1, Boss: "heisenbug", Node: "heisenbug"},
			{Name: "Kernel Space", Topic: game.Mixed, Kind: rules.Choice, Rooms: 1, Node: "kernel"},
			{Topic: game.Mixed, Kind: rules.Boss, Rooms: 1, Boss: game.FinalBoss, Node: "halting-problem"},
		},
		Cycles: 24, MaxCycles: 50, Ducks: 1,
		Upgrades: []game.Upgrade{game.Overclock},
		Daemons:  []game.Daemon{game.Strace},
	}
	t, i := 0, 0
	for b := range tier {
		for range st.Biomes[b].Rooms {
			d := door(cyclesRooms[i%len(cyclesRooms)], cyclesTimer(b), 64, 0.7)
			d.Boss = st.Biomes[b].Boss
			a := attempt(start, d, at(t, 0), at(t+11, 0), game.Cleared)
			a.Biome = b
			st.History = append(st.History, a)
			t, i = t+12, i+1
		}
	}
	st.Biome = tier
	st.Uncommitted, st.Committed = 128, 256
	rebase(st, at(t, 30))
	return st
}

// CyclesRoom is the doors of act II's room (The ALU): what a timeout or :q!
// burns there, 12 cycles, the act's price, on every card.
func CyclesRoom() *game.State {
	st := cyclesBoot(rules.TierActII)
	st.Doors = []game.Door{
		door(moobuzz, cyclesTimer(rules.TierActII), 96, 0.7),
		door(watermelon, cyclesTimer(rules.TierActII), 64, 0.8),
	}
	return st
}

// CyclesBoss is the doors of key's boss room, its door bending the boss's
// rule as the engine sets it (D54): Off-by-One and the OOM Killer close
// act I, Heisenbug, Race Condition and Undefined Behavior act II, The
// Halting Problem the boot.
func CyclesBoss(key string) *game.State {
	tier := rules.TierBossI
	switch key {
	case "heisenbug", "race-condition", "undefined-behavior":
		tier = rules.TierBossII
	case game.FinalBoss:
		tier = rules.TierFinal
	}
	st := cyclesBoot(tier)
	st.Biomes[tier].Boss = key
	d := door(cowPoetry, cyclesTimer(tier), 576, 0.5)
	d.Boss, d.Twist = key, key
	switch key {
	case "oom-killer":
		d.MemMult = 0.5
	case "heisenbug":
		d.QuietSamples = true
	case "race-condition":
		d.Timer = time.Duration(float64(d.Timer) * 0.8)
		d.Bytes = 768
	case "undefined-behavior":
		d.NoSanitizers = true
	case game.FinalBoss:
		d.HiddenClock = true
	}
	st.Doors = []game.Door{d}
	return st
}

// CyclesBossRetry is Off-by-One's room after its first door timed out: 20
// cycles burned and a new boss door dealt (map.md §(f)).
func CyclesBossRetry() *game.State {
	st := CyclesBoss("off-by-one")
	start := st.Started
	failed := door(bovine, cyclesTimer(rules.TierBossI), 576, 0.5)
	failed.Boss, failed.Twist = "off-by-one", "off-by-one"
	a := attempt(start, failed, at(36, 0), at(68, 0), game.TimedOut)
	a.Biome = rules.TierBossI
	st.History = append(st.History, a)
	st.Cycles = 24 - 20 + 18 // 18 healed at the fountain and by fsck before it
	rebase(st, at(68, 30))
	return st
}

// CyclesProblem opens st's first door, 4:10 in.
func CyclesProblem(st *game.State) *game.State {
	d := st.Doors[0]
	opened := Now.Add(-4*time.Minute - 10*time.Second)
	st.Current = &game.Attempt{Door: d, Opened: opened, Deadline: opened.Add(d.Timer), Biome: st.Biome, CyclesAtOpen: st.Cycles}
	st.Phase, st.Doors = game.PhaseProblem, nil
	return st
}

// CyclesMerge is the merge window after Off-by-One fell, between acts I and
// II, by its fountain (map.md §(e)): "drink" poured 5 cycles (17 → 22);
// "full" found them at 32, over the line, and poured none; "dry" is the
// same window at P5, which takes the fountain away. fsck is on sale.
func CyclesMerge(fountain string) *game.State {
	st := cyclesBoot(rules.TierActII)
	st.Phase = game.PhaseMerge
	st.Biome, st.Room = rules.TierBossI, 1
	st.Uncommitted = 640
	st.PendingUpgrade, st.UpgradePicks = true, 1
	st.UpgradeOffers = []game.Upgrade{game.Overclock, game.ECC, game.Duck}
	st.DaemonOffers = []game.Daemon{game.Kexec, game.Nice}
	switch fountain {
	case "drink":
		st.Cycles, st.Fountain, st.Fountained = 22, 5, 5
	case "full":
		st.Cycles = 32
	case "dry":
		st.Prestige = 5
		st.Cycles = 17
	}
	return st
}

// CyclesUpgrade is the pick after act II's room, one Overclock of two taken,
// with ECC as the clock conversion on offer.
func CyclesUpgrade() *game.State {
	st := cyclesBoot(rules.TierBossII)
	st.Biome, st.Room = rules.TierActII, 1
	st.PendingUpgrade, st.UpgradePicks = true, 1
	st.UpgradeOffers = []game.Upgrade{game.Overclock, game.ECC, game.Duck}
	return st
}

// CyclesCrash is the map boot crashed, by how (D47): "timeout", act II's
// door timed out with 8 cycles left against its 12; "submits", failed
// submits burned the 6 cycles act II's door opened with; "final", The
// Halting Problem's one-shot door timed out with 14 cycles to spare.
func CyclesCrash(how string) *game.State {
	tier := rules.TierActII
	if how == "final" {
		tier = rules.TierFinal
	}
	st := cyclesBoot(tier)
	start := st.Started
	t := len(st.History) * 12
	d := door(moobuzz, cyclesTimer(tier), 96, 0.7)
	cause := game.TimedOut
	switch how {
	case "final":
		d = door(cowPoetry, cyclesTimer(tier), 576, 0.5)
		d.Boss, d.Twist, d.HiddenClock = game.FinalBoss, game.FinalBoss, true
		st.Cycles = 14
	case "submits":
		cause = game.OutOfLives
		st.Cycles = 0
	default:
		st.Cycles = 8
	}
	a := attempt(start, d, at(t, 0), 0, cause)
	a.Biome, a.CyclesAtOpen = tier, max(st.Cycles, 8)
	a.Submits, a.Failed, a.BestPass, a.Total, a.LastVerdict = 3, 3, 9, 14, "WA"
	if how == "submits" {
		a.CyclesAtOpen = 6
		a.Closed = start.Add(at(t, 0) + 9*time.Minute)
	} else {
		a.Closed = a.Deadline
	}
	st.History = append(st.History, a)
	st.Phase = game.PhaseCrashed
	st.Crash = &game.Crash{Cause: cause, Door: d, Verdict: a.LastVerdict, Lost: st.Uncommitted, At: a.Closed}
	st.Crash.Uptime = st.Uptime(a.Closed)
	st.Uncommitted = 0
	st.AtStart = atStart()
	rebase(st, a.Closed.Sub(start))
	return st
}

// CyclesCleared is the map boot after The Halting Problem fell: 14 cycles
// left, two fountains drunk, a room lost to a timeout and Off-by-One
// beaten on its second door.
func CyclesCleared() *game.State {
	st := cyclesBoot(rules.TierFinal)
	start := st.Started
	t := len(st.History) * 12
	// Kernel Space's one room was lost to its timer, not cleared.
	k := &st.History[len(st.History)-1]
	k.Outcome, k.Closed = game.TimedOut, k.Deadline
	// Off-by-One's first door timed out before its second fell.
	boss := door(bovine, cyclesTimer(rules.TierBossI), 576, 0.5)
	boss.Boss = "off-by-one"
	ba := attempt(start, boss, at(36, 0), at(36+32, 0), game.TimedOut)
	ba.Biome = rules.TierBossI
	st.History = append(st.History[:3], append([]game.Attempt{ba}, st.History[3:]...)...)
	final := door(cowPoetry, cyclesTimer(rules.TierFinal), 576, 0.5)
	final.Boss, final.HiddenClock = game.FinalBoss, true
	fa := attempt(start, final, at(t+30, 0), at(t+61, 0), game.Cleared)
	fa.Biome = rules.TierFinal
	st.History = append(st.History, fa)
	st.Phase = game.PhaseCleared
	st.Biome, st.Room = rules.TierFinal, 1
	st.Cycles, st.Fountained = 14, 10
	st.Committed, st.Uncommitted = 2048, 0
	st.AtStart = atStart()
	rebase(st, at(t+62, 0))
	return st
}
