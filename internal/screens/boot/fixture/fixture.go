// Package fixture is hand-built boot state for the boot screens' golden
// tests and cmd/demo-boot. It does not run the engine: each State is what
// the engine plausibly holds at that point of one boot (Boot #42, Silver,
// four biomes), so the screens can be checked while the engine is written.
// Every function returns fresh values; callers may mutate them.
package fixture

import (
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// Now is the wall clock every fixture is built around.
var Now = time.Date(2026, 10, 14, 21, 30, 0, 0, time.UTC)

const (
	cowTipping   catalog.ID = "usaco:689"
	haybales     catalog.ID = "usaco:104"
	milkMeasure  catalog.ID = "usaco:761"
	moocast      catalog.ID = "usaco:668"
	bovine       catalog.ID = "usaco:764"
	closingFarm  catalog.ID = "usaco:644"
	wormholeSort catalog.ID = "usaco:992"
	cereal       catalog.ID = "usaco:1039"
	snowBoots    catalog.ID = "usaco:811"
	moobuzz      catalog.ID = "usaco:966"
	cowPoetry    catalog.ID = "usaco:897"
	pathCounting catalog.ID = "cses:1136"
)

var problems = map[catalog.ID]catalog.Problem{}

func init() {
	add := func(id catalog.ID, title, contest, div string, pos, rating int, topic string, tags ...string) {
		problems[id] = catalog.Problem{
			ID: id, Source: id.Source(), Title: title, Division: div, Contest: contest, Position: pos,
			Tags: tags, Topic: topic, Rating: rating, RatingSource: "division+position",
			TimeLimit: 2 * time.Second, MemLimit: 256 << 20, Judgeable: true, HasEditorial: true,
		}
	}
	add(cowTipping, "Cow Tipping", "2017 January, Bronze P3", "bronze", 3, 1000, "mixed", "Greedy", "Simulation")
	add(haybales, "Haybale Stacking", "2012 January, Bronze P1", "bronze", 1, 1050, "prefix-sums", "Prefix Sums", "Difference Arrays")
	add(milkMeasure, "Milk Measurement", "2017 December, Bronze P3", "bronze", 3, 1100, "simulation", "Simulation", "Sorting")
	add(moocast, "Moocast", "2016 December, Silver P3", "silver", 3, 1250, "graphs", "Graphs", "DFS")
	add(bovine, "The Bovine Shuffle", "2017 December, Silver P1", "silver", 1, 1300, "graphs", "Functional Graphs", "Cycle Detection")
	add(closingFarm, "Closing the Farm", "2016 US Open, Silver P3", "silver", 3, 1350, "dsu", "DSU", "Offline Queries", "Connected Components")
	add(wormholeSort, "Wormhole Sort", "2020 January, Silver P3", "silver", 3, 1450, "binary-search", "Binary Search on Answer", "DSU")
	add(cereal, "Cereal", "2020 US Open, Silver P2", "silver", 2, 1400, "data-structures", "Simulation", "Amortized Analysis")
	add(snowBoots, "Snow Boots", "2018 February, Silver P1", "silver", 1, 1350, "graphs", "Graphs", "BFS", "Sorting")
	add(moobuzz, "MooBuzz", "2019 December, Silver P1", "silver", 1, 1200, "math", "Math", "Periodicity")
	add(cowPoetry, "Cow Poetry", "2019 January, Gold P2", "gold", 2, 1700, "dp", "Knapsack DP", "Modular Exponentiation")
	add(pathCounting, "Path Queries", "Tree Algorithms", "", 0, 1600, "trees", "Euler Tour", "Fenwick Tree")
}

// Lookup resolves fixture problems; it has the signature of
// catalog.Catalog.Get.
func Lookup(id catalog.ID) (catalog.Problem, bool) {
	p, ok := problems[id]
	return p, ok
}

// Rules is the standard preset with a second choice biome, so the map has
// four biomes to lay out.
func Rules() rules.Rules {
	r := rules.Standard()
	r.Biomes = []rules.Biome{
		{Kind: rules.Fixed, Name: "Userland", Rooms: 2, TargetClear: 0.9, Timer: rules.Duration{Duration: 15 * time.Minute}},
		{Kind: rules.Choice, Rooms: 2, Choices: 2, TargetClear: 0.7, Timer: rules.Duration{Duration: 30 * time.Minute}},
		{Kind: rules.Choice, Rooms: 2, Choices: 3, TargetClear: 0.6, Timer: rules.Duration{Duration: 35 * time.Minute}},
		{Kind: rules.Boss, Rooms: 1, TargetClear: 0.5, Timer: rules.Duration{Duration: 50 * time.Minute}},
	}
	return r
}

// Meta is the player: rated about 1180, with enough doors behind dp, data
// structures and graphs that those aim by their own ratings, Silver
// unlocked, a regression due.
func Meta() game.Meta {
	return game.Meta{
		Rating:        1180,
		TopicRating:   map[string]float64{"dp": 1120, "data-structures": 1240, "graphs": 1210},
		TopicAttempts: map[string]int{"dp": 6, "data-structures": 9, "graphs": 7},
		Prestige:      1,
		Bank:          2150,
		Regressions:   []game.Regression{{Problem: cereal, Due: Now.Add(-2 * time.Hour), Stage: 0}},
		Boots:         42,
	}
}

func door(id catalog.ID, timer time.Duration, bytes int64, p float64) game.Door {
	pr := problems[id]
	return game.Door{Problem: id, Topic: pr.Topic, Rating: pr.Rating, Timer: timer, Bytes: bytes, SolveP: p}
}

// at offsets into the boot: minutes and seconds since it started.
func at(m, s int) time.Duration { return time.Duration(m)*time.Minute + time.Duration(s)*time.Second }

func attempt(start time.Time, d game.Door, open, closed time.Duration, o game.Outcome) game.Attempt {
	a := game.Attempt{Door: d, Opened: start.Add(open), Deadline: start.Add(open + d.Timer), Outcome: o}
	if o != "" {
		a.Closed = start.Add(closed)
	}
	return a
}

// The doors of the second biome's second room: a plain door, a blind and
// cursed one, and a mystery.
func roomDoors() []game.Door {
	plain := door(bovine, 30*time.Minute, 80, 0.74)
	cursed := door(closingFarm, 30*time.Minute, 240, 0.66)
	cursed.Blind, cursed.Cursed = true, true
	mystery := door(wormholeSort, 30*time.Minute, 160, 0.58)
	mystery.Blind, mystery.Mystery = true, true
	return []game.Door{plain, cursed, mystery}
}

// base is the boot 47 minutes in: Userland cleared (with one retreat), the
// first merge window committed 112 B, and one room of the data-structures
// biome cleared. It stands at the doors of the next room.
func base() *game.State {
	start := Now.Add(-at(47, 12))
	st := &game.State{
		Boot: 42, Seed: 0x8f3a, Rules: Rules(), Prestige: 1, Started: start,
		Phase: game.PhaseDoors,
		Biomes: []game.BiomeState{
			{Name: "Userland", Topic: "mixed", Kind: rules.Fixed, Rooms: 2},
			{Topic: "data-structures", Kind: rules.Choice, Rooms: 2, Offers: []string{"dp", "data-structures"}},
			{Kind: rules.Choice, Rooms: 2},
			// The engine gives a boss biome the mixed topic and names its
			// boss at boot.
			{Topic: "mixed", Kind: rules.Boss, Rooms: 1, Boss: "heisenbug"},
		},
		Biome: 1, Room: 1,
		Doors: roomDoors(),
		History: []game.Attempt{
			attempt(start, door(cowTipping, 15*time.Minute, 48, 0.91), at(0, 30), at(9, 50), game.Cleared),
			attempt(start, door(haybales, 15*time.Minute, 48, 0.88), at(10, 40), at(14, 50), game.Retreated),
			attempt(start, door(milkMeasure, 15*time.Minute, 64, 0.86), at(15, 0), at(27, 31), game.Cleared),
			attempt(start, door(moocast, 30*time.Minute, 96, 0.71), at(30, 0), at(44, 2), game.Cleared),
		},
		Lives: 2, MaxLives: 3, Ducks: 1,
		Uncommitted: 96, Committed: 112,
		Upgrades: []game.Upgrade{game.Overclock},
		Daemons:  []game.Daemon{game.Strace},
		Log: []game.LogLine{
			{At: start, Text: "boot: prestige 1, preset standard, seed 8f3a"},
			{At: start.Add(at(9, 50)), Text: `door_1: AC "Cow Tipping" +48 B`},
			{At: start.Add(at(14, 50)), Text: `door_2: retreat from "Haybale Stacking", -1 life`},
			{At: start.Add(at(27, 31)), Text: `door_3: AC "Milk Measurement" +64 B`},
			{At: start.Add(at(29, 10)), Text: "merge: committed 112 B, loaded strace"},
			{At: start.Add(at(44, 2)), Text: `door_4: AC "Moocast" +96 B`},
		},
	}
	st.History[2].Quacks = 1
	return st
}

// Doors is the centerpiece: three doors, one plain, one blind and cursed,
// one mystery.
func Doors() *game.State { return base() }

// DoorsFork is the third biome's first room with fork loaded: four doors,
// including a regression and a cursed mystery.
func DoorsFork() *game.State {
	st := cleared5()
	st.Biome, st.Room = 2, 0
	st.Biomes[2].Topic = "graphs"
	st.Biomes[2].Offers = []string{"graphs", "dp", "math"}
	st.Daemons = append(st.Daemons, game.Fork)
	reg := door(cereal, 35*time.Minute, 128, 0.62)
	reg.Regression = true
	blind := door(snowBoots, 35*time.Minute, 168, 0.64)
	blind.Blind = true
	cm := door(pathCounting, 35*time.Minute, 512, 0.41)
	cm.Blind, cm.Mystery, cm.Cursed = true, true, true
	st.Doors = []game.Door{reg, door(moobuzz, 35*time.Minute, 96, 0.69), blind, cm}
	return st
}

// cleared5 is base after the fifth door (The Bovine Shuffle) was cleared,
// finishing the data-structures biome.
func cleared5() *game.State {
	st := base()
	start := st.Started
	st.History = append(st.History,
		attempt(start, st.Doors[0], at(47, 30), at(71, 4), game.Cleared))
	st.Room = 2
	st.Doors = nil
	st.Uncommitted = 176
	st.Log = append(st.Log, game.LogLine{At: start.Add(at(71, 4)), Text: `door_5: AC "The Bovine Shuffle" +80 B`})
	rebase(st, at(72, 30))
	return st
}

// Merge is the merge window after the data-structures biome: bytes to
// commit, an upgrade pending, three daemons on offer.
func Merge() *game.State {
	st := cleared5()
	st.Phase = game.PhaseMerge
	st.PendingUpgrade, st.UpgradePicks = true, 1
	st.UpgradeOffers = []game.Upgrade{game.Overclock, game.ECC, game.Duck}
	st.DaemonOffers = []game.Daemon{game.Nice, game.Fork, game.DevRandom}
	return st
}

// Upgrade is the pick right after a clear: Moocast was just cleared in
// under half its timer, an -O3 clear, so two picks are owed.
func Upgrade() *game.State {
	st := base()
	st.History[len(st.History)-1].O3 = true
	st.PendingUpgrade, st.UpgradePicks = true, 2
	st.UpgradeOffers = []game.Upgrade{game.Overclock, game.ECC, game.Duck}
	return st
}

// Route is the fork into the second biome, before a topic is chosen.
func Route() *game.State {
	st := base()
	st.Phase = game.PhaseRoute
	st.Biome, st.Room = 1, 0
	st.Biomes[1].Topic = ""
	st.History = st.History[:3]
	st.Doors = nil
	st.Uncommitted = 0
	st.Log = st.Log[:5]
	return st
}

// Route3 is the fork into the third biome: three topics on offer.
func Route3() *game.State {
	st := cleared5()
	st.Phase = game.PhaseRoute
	st.Biome, st.Room = 2, 0
	st.Biomes[2].Offers = []string{"graphs", "dp", "math"}
	return st
}

// Boss is the boss room: one door, the Heisenbug.
func Boss() *game.State {
	st := cleared5()
	start := st.Started
	st.History = append(st.History,
		attempt(start, door(snowBoots, 35*time.Minute, 168, 0.64), at(74, 0), at(98, 41), game.Cleared),
		attempt(start, door(moobuzz, 35*time.Minute, 96, 0.69), at(99, 0), at(111, 12), game.Cleared),
	)
	st.Biomes[2].Topic = "graphs"
	st.Biome, st.Room = 3, 0
	boss := door(cowPoetry, 50*time.Minute, 576, 0.47)
	boss.Boss = "heisenbug"
	st.Doors = []game.Door{boss}
	st.Uncommitted, st.Committed = 264, 288
	rebase(st, at(113, 5))
	return st
}

// rebase moves the whole boot in time so that Now falls up into it: the
// fixtures lay events out as offsets from the start, and each wants the
// clock to read just after its last event.
func rebase(st *game.State, up time.Duration) {
	d := Now.Add(-up).Sub(st.Started)
	st.Started = st.Started.Add(d)
	mv := func(t time.Time) time.Time {
		if t.IsZero() {
			return t
		}
		return t.Add(d)
	}
	for i := range st.History {
		a := &st.History[i]
		a.Opened, a.Deadline, a.Closed = mv(a.Opened), mv(a.Deadline), mv(a.Closed)
	}
	if a := st.Current; a != nil {
		a.Opened, a.Deadline, a.Closed = mv(a.Opened), mv(a.Deadline), mv(a.Closed)
	}
	for i := range st.Log {
		st.Log[i].At = mv(st.Log[i].At)
	}
	if c := st.Crash; c != nil {
		c.At = mv(c.At)
	}
}

// Problem is the HUD's view: The Bovine Shuffle open with 23:41 left,
// two submits in, the best passing 37 of 52 tests.
func Problem() *game.State {
	st := base()
	st.Phase = game.PhaseProblem
	a := attempt(st.Started, st.Doors[0], at(47, 30), 0, "")
	a.Submits, a.Failed, a.BestPass, a.Total, a.LastVerdict = 2, 1, 37, 52, "WA"
	st.Current = &a
	st.Doors = nil
	st.Lives = 1
	rebase(st, at(53, 49))
	return st
}

// Crash is the boot crashed in the second biome's second room, by cause:
// the timer ran out on the blind, cursed door; lives ran out on the plain
// one; or the player quit the mystery with :q!.
func Crash(cause game.Outcome) *game.State {
	st := base()
	ds := roomDoors()
	start := st.Started
	var a game.Attempt
	switch cause {
	case game.OutOfLives:
		a = attempt(start, ds[0], at(47, 30), at(66, 8), cause)
		a.Submits, a.Failed, a.BestPass, a.Total, a.LastVerdict = 2, 2, 37, 52, "WA"
		st.Lives = 0
	case game.GaveUp:
		a = attempt(start, ds[2], at(47, 30), at(58, 44), cause)
		a.Submits, a.Failed, a.BestPass, a.Total, a.LastVerdict = 1, 1, 12, 20, "TLE"
	default:
		cause = game.TimedOut
		a = attempt(start, ds[1], at(47, 30)+337*time.Millisecond, 0, cause)
		a.Closed = a.Deadline
	}
	st.Phase = game.PhaseCrashed
	st.Doors = nil
	crashAt := a.Closed
	st.Crash = &game.Crash{
		Cause: cause, Door: a.Door, Verdict: a.LastVerdict, Lost: st.Uncommitted,
		At: crashAt, Uptime: crashAt.Sub(start),
	}
	// One shape keeps the crashing attempt open in Current, the other has
	// already filed it in History; the screens must handle both.
	if cause == game.OutOfLives {
		st.Current = &a
	} else {
		st.History = append(st.History, a)
	}
	st.Uncommitted = 0
	rebase(st, crashAt.Sub(start))
	return st
}

// CrashMeta is Meta right after Crash(cause): the crash moved the
// uncommitted bytes into a core dump on the crashing door.
func CrashMeta(cause game.Outcome) game.Meta {
	cr := Crash(cause).Crash
	m := Meta()
	m.CoreDump = &game.CoreDump{Problem: cr.Door.Problem, Bytes: cr.Lost, Boot: 42, At: cr.At}
	return m
}

// Cleared is the boot after the Heisenbug fell: all eight rooms cleared,
// Gold unlocked.
func Cleared() *game.State {
	st := Boss()
	boss := st.Doors[0]
	a := attempt(st.Started, boss, at(113, 30), at(149, 52), game.Cleared)
	a.Quacks = 1
	st.History = append(st.History, a)
	st.Phase = game.PhaseCleared
	st.Biome, st.Room = 3, 1
	st.Doors = nil
	st.Committed, st.Uncommitted = 1240, 0
	rebase(st, at(150, 30))
	return st
}

// ClearedMeta is Meta after the clear: Gold unlocked and the bank grown.
func ClearedMeta() game.Meta {
	m := Meta()
	m.Prestige = 2
	m.Bank = 3390
	return m
}
