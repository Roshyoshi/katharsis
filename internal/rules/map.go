package rules

import "time"

// The run map (docs/design/map.md, D47-D50): three acts on a Dead
// Cells-style graph, priced in cycles. Every number is a setting (D2); the
// simulator's report on them is map.md §(i) and sim TestMapReport.

func minutes(m int) Duration { return Duration{time.Duration(m) * time.Minute} }

// Map is the three-act boot (map.md §(a)): Userland, one of four topic
// biomes, Boss I, one of three, Boss II, a topic biome or a skip, and The
// Halting Problem. It is the cycles meta (meta.md §(b)) with the user's
// numbers: 30 cycles of 50; a failed submit 2, a timeout or give-up 10
// scaled by act (12, 15) and 20 a boss, the room lost and the boot going
// on; Bosses I and II retry, the final is one shot; retreat 5, locked 2:30
// into a door; no heal on AC, fsck 10 for 64 B, a fountain of 5 between
// acts. Time is bought (D50): a tight base clock, Overclock additive and
// two at most for 5 cycles a pick, ECC trading a failed submit's cycles for
// 5:00 of the door's clock, -O3 on the timer before bought time, kexec
// once an act. Boot #1 plays the short shape (RT11), priced the same.
func Map() Rules {
	r := Legacy()
	r.Name = "standard"
	r.Cycles, r.MaxCycles = 30, 50
	r.LifeCycles = 10 // D34's and the governor's life: +10 cycles each
	r.SubmitCycles, r.RetreatCycles, r.FsckCycles = 2, 5, 10
	r.FailMoveOn = true
	r.OneShot = 1
	r.ECCPenalty = minutes(5)
	r.RetreatLock = Duration{150 * time.Second}
	r.FountainCycles, r.FountainWhere = 5, FountainAct
	r.KexecPerAct = true
	r.TimerMult = 1
	r.OverclockMax, r.OverclockCycles = 2, 5
	r.O3OnBase = true
	r.FirstClockOwn = true
	r.NiceLifeCost = 1 // nice is paid in bytes (D46)
	r.Twists = MapTwists()
	r.Biomes = MapBiomes()
	r.Nodes = MapNodes()
	r.Ascensions = MapAscensions()
	r.FirstBiomes = ShortBiomes()
	return r
}

// MapTwists bends one rule per boss (user, 10-15): Off-by-One burns double
// for a failed submit one test short, the OOM Killer halves the memory
// limit, Heisenbug reports samples only as pass or fail, Race Condition's
// clock runs 1.25 times fast for 4 times the pay instead of 3, Undefined
// Behavior runs the samples without sanitizers, and The Halting Problem
// hides its clock.
func MapTwists() Twists {
	return Twists{OffByOne: 2, OOMMemory: 0.5, Heisenbug: true, RaceClock: 0.8, RacePay: 4, UBNoSanitizers: true, HaltingClock: true}
}

// MapBiomes is the map's tier skeleton (map.md §(d)): each tier's kind, its
// nodes' aim, floor, clock and failure price, and what the map shows of a
// tier before its exit is taken. The clocks are about 0.8 of the model
// player's 0.85 solve-time quantile at each aim: the calibration holds the
// clear rates at the aims whatever the clock, so the clock sets the boot's
// length, about three hours of doors for a full clear (§(i)6). Failures
// cost 10 in act I, 12 in act II and 15 in act III (x1, x1.25, x1.5); a
// boss 20. Boss I holds 0.45 or above, so prestige does not make it the
// wall (§(f)).
func MapBiomes() []Biome {
	return []Biome{
		{Kind: Fixed, Name: "Userland", Rooms: 2, TargetClear: 0.92, Timer: minutes(10), EaseClear: 0.95, Plain: 1, FailCycles: 10},
		{Kind: Choice, Name: "act I", Rooms: 1, Choices: 2, TargetClear: 0.75, MinClear: 0.6, Timer: minutes(21), EaseClear: 0.8, FailCycles: 10},
		{Kind: Boss, Name: "boss I", Rooms: 1, TargetClear: 0.55, MinClear: 0.45, Timer: minutes(32), EaseClear: 0.6, FailCycles: 20},
		{Kind: Choice, Name: "act II", Rooms: 1, Choices: 2, TargetClear: 0.7, MinClear: 0.6, Timer: minutes(24), EaseClear: 0.78, FailCycles: 12},
		{Kind: Boss, Name: "boss II", Rooms: 1, TargetClear: 0.45, Timer: minutes(36), EaseClear: 0.5, FailCycles: 20},
		{Kind: Choice, Name: "act III", Rooms: 1, Choices: 2, TargetClear: 0.65, MinClear: 0.6, Timer: minutes(28), EaseClear: 0.72, FailCycles: 15},
		{Kind: Boss, Name: "final", Rooms: 1, TargetClear: 0.5, Timer: minutes(40), EaseClear: 0.55, FailCycles: 20},
	}
}

// Act tiers: each tier of MapBiomes by its place in the acts.
const (
	TierUserland = iota
	TierActI
	TierBossI
	TierActII
	TierBossII
	TierActIII
	TierFinal
)

// node is a node of tier t on that tier's skeleton defaults.
func node(t int, id, name, topic string, next ...string) Biome {
	b := MapBiomes()[t]
	b.ID, b.Tier, b.Name, b.Topic, b.Next = id, t, name, topic, next
	return b
}

func bossNode(t int, key, name string, next ...string) Biome {
	b := node(t, key, name, "", next...)
	b.Boss = key
	return b
}

// gated is a node behind its topic's capability, with a scroll (one more
// upgrade pick) at its exit, as treasure waits behind Dead Cells' rune
// doors.
func gated(b Biome) Biome {
	b.Picks = 1
	return b
}

// MapNodes is the graph (map.md §(c)). Every tier has an exit a player with
// no capability can take (the Bronze topics, The ALU, Kernel Space), as
// Dead Cells always leaves a rune-less route; the exit taken picks the
// boss. Names and keys are the story bible's but Kernel Space's, which the
// storyteller owns.
func MapNodes() []Biome {
	bisect := gated(node(TierActII, "bisect", "Bisect Bridge", "binary-search", "heisenbug"))
	bisect.Package = "git" // git bisect
	ub := bossNode(TierBossII, "undefined-behavior", "Undefined Behavior", "trees", "geometry", "kernel", "skip")
	ub.MinPrestige = 1 // through the glass
	fb := gated(node(TierActIII, "geometry", "The Framebuffer", "geometry", "halting-problem"))
	fb.MinPrestige = 2
	skip := node(TierActIII, "skip", "skip", "", "halting-problem")
	skip.Skip, skip.Picks = true, 2
	return []Biome{
		node(TierUserland, "userland", "Userland", "", "bunker", "emulator", "scheduler", "accumulator"),

		node(TierActI, "bunker", "Brute Force Bunker", "complete-search", "off-by-one", "oom-killer"),
		node(TierActI, "emulator", "The Emulator", "simulation", "oom-killer"),
		node(TierActI, "scheduler", "The Scheduler", "sorting-greedy", "off-by-one"),
		gated(node(TierActI, "accumulator", "The Accumulator", "prefix-sums", "off-by-one")),

		bossNode(TierBossI, "off-by-one", "Off-by-One", "bisect", "window", "alu"),
		bossNode(TierBossI, "oom-killer", "OOM Killer", "alu", "heap", "lexer"),

		bisect,
		gated(node(TierActII, "window", "The Sliding Window", "two-pointers", "race-condition")),
		node(TierActII, "alu", "The ALU", "math", "heisenbug", "race-condition"),
		gated(node(TierActII, "heap", "The Heap", "data-structures", "heisenbug", "undefined-behavior")),
		gated(node(TierActII, "lexer", "The Lexer", "strings", "race-condition", "undefined-behavior")),

		bossNode(TierBossII, "heisenbug", "Heisenbug", "graphs", "dp", "kernel", "skip"),
		bossNode(TierBossII, "race-condition", "Race Condition", "dsu", "sssp", "kernel", "skip"),
		ub,

		gated(node(TierActIII, "graphs", "Routing Table", "graphs", "halting-problem")),
		gated(node(TierActIII, "dp", "The Cache", "dp", "halting-problem")),
		gated(node(TierActIII, "dsu", "Union Hall", "dsu", "halting-problem")),
		gated(node(TierActIII, "sssp", "Dijkstra's Reach", "shortest-paths", "halting-problem")),
		gated(node(TierActIII, "trees", "The Filesystem", "trees", "halting-problem")),
		fb,
		node(TierActIII, "kernel", "Kernel Space", "", "halting-problem"),
		skip,

		bossNode(TierFinal, "halting-problem", "The Halting Problem"),
	}
}

// MapAscensions is the prestige ladder across the three acts (map.md §(d)):
// today's risk modifiers at P1-P4 with D47's cuts, then cuts that bite late
// and on the pool, not in Userland, whose clock is set per level (10:00,
// 9:00 at P3, 8:00 at P6) outside the timer cuts (Rules.FirstClockOwn).
//
//	P1 blind +15%, timer x0.95, a failed submit 3
//	P2 a duck less, mystery +5%, cursed +4%, fsck heals 5
//	P3 cursed +5%, timer x0.9, Userland 9:00, acts II-III fails +5
//	P4 cursed +4%, timer x0.9, start at 25
//	P5 no fountain
//	P6 Userland 8:00, Boss II one shot
//	P7 a failed submit 4, start at 20
//	P8 every boss one shot
func MapAscensions() []Ascension {
	return []Ascension{
		{Level: 1, BlindChance: 0.15, TimerMult: 0.95, SubmitCycles: 1},
		{Level: 2, Ducks: -1, MysteryChance: 0.05, CursedChance: 0.04, HealCycles: -5},
		{Level: 3, CursedChance: 0.05, TimerMult: 0.9, FirstTimer: minutes(9), LateFailCycles: 5},
		{Level: 4, CursedChance: 0.04, TimerMult: 0.9, Cycles: -5},
		{Level: 5, FountainCycles: -5},
		{Level: 6, FirstTimer: minutes(8), OneShot: 2},
		{Level: 7, SubmitCycles: 1, Cycles: -5},
		{Level: 8, OneShot: 3},
	}
}

// ShortBiomes is the short preset's shape in cycles, and boot #1's (RT11):
// two Userland doors, one mid door and a boss, on short timers.
func ShortBiomes() []Biome {
	return []Biome{
		{Kind: Fixed, Name: "Userland", Rooms: 2, TargetClear: 0.9, Timer: minutes(10), EaseClear: 0.95, Plain: 1, FailCycles: 10},
		{Kind: Choice, Rooms: 1, Choices: 2, TargetClear: 0.7, MinClear: 0.6, Timer: minutes(20), EaseClear: 0.78, FailCycles: 10},
		{Kind: Boss, Rooms: 1, TargetClear: 0.5, Timer: minutes(25), EaseClear: 0.55, FailCycles: 20},
	}
}

// PatchBiomes is the patch preset's shape in cycles: one Userland door, one
// mid door, and a mini-boss aimed at 0.6.
func PatchBiomes() []Biome {
	return []Biome{
		{Kind: Fixed, Name: "Userland", Rooms: 1, TargetClear: 0.9, Timer: minutes(10), EaseClear: 0.95, Plain: 1, FailCycles: 10},
		{Kind: Choice, Rooms: 1, Choices: 2, TargetClear: 0.7, MinClear: 0.6, Timer: minutes(20), EaseClear: 0.78, FailCycles: 10},
		{Kind: Boss, Rooms: 1, TargetClear: 0.6, Timer: minutes(20), EaseClear: 0.65, FailCycles: 20},
	}
}
