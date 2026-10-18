package game_test

import (
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// mapBoot is a map boot on mapPlain's rules (the roster's pools), -O3 off,
// at its first doors.
func mapBoot(t *testing.T, seed uint64) *harness {
	t.Helper()
	r := mapPlain()
	r.O3Frac = 0 // a fast clear's free pick apart
	// A pick a door, as before D62: these test what the perks do once
	// picked, not how many picks a boot gets.
	r.PickPerBiome = false
	return newHarness(t, r, &game.Meta{}, dense(catalog.Topics, 800, 2600, 20), 0, seed)
}

func subAC() game.SubmitResult { return game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10} }
func subWA() game.SubmitResult { return game.SubmitResult{Verdict: "WA", Passed: 3, Total: 10} }

// The roster draws ten a pool; a boot saved before it (Pool 0) keeps the
// first pools, so its offers never name the roster's perks.
func TestRosterPools(t *testing.T) {
	t.Parallel()
	seen := map[game.Upgrade]bool{}
	for seed := range uint64(40) {
		h := mapBoot(t, seed)
		h.clearDoor(0, time.Minute)
		for _, u := range h.s.UpgradeOffers {
			seen[u] = true
		}
		if !slices.Contains(h.s.UpgradeOffers, game.Skip) || len(h.s.UpgradeOffers) != 4 {
			t.Fatalf("offers %v: want 3 and the skip", h.s.UpgradeOffers)
		}
	}
	// heatsink waits for cycles below the start.
	for _, u := range []game.Upgrade{game.Snapshot, game.PrefetchPick, game.Turbo, game.Smt, game.Microcode} {
		if !seen[u] {
			t.Errorf("%s never offered in 40 boots", u)
		}
	}
	if seen[game.Fsanitize] {
		t.Error("-fsanitize offered without clang-tools")
	}

	old := calm(rules.Legacy())
	old.BlindChance, old.MysteryChance, old.CursedChance = 0, 0, 0
	h := newHarness(t, old, &game.Meta{}, dense(catalog.Topics, 800, 2600, 20), 0, 1)
	h.clearDoor(0, time.Minute)
	for _, u := range h.s.UpgradeOffers {
		if !slices.Contains(game.AllUpgrades, u) {
			t.Errorf("a boot before the roster offered %s", u)
		}
	}
}

// A skip is a few cycles, up to the start, and holds nothing.
func TestSkip(t *testing.T) {
	t.Parallel()
	h := mapBoot(t, 1)
	h.clearDoor(0, time.Minute)
	h.s.Cycles = 20
	h.do(game.PickUpgrade{Upgrade: game.Skip})
	if h.s.Cycles != 22 || slices.Contains(h.s.Upgrades, game.Skip) {
		t.Errorf("skip: %d cycles, upgrades %v", h.s.Cycles, h.s.Upgrades)
	}
}

// The upgrades: each does its one thing, and the repeatable ones stop at
// their hold.
func TestRosterUpgrades(t *testing.T) {
	t.Parallel()
	pick := func(h *harness, u game.Upgrade) {
		h.t.Helper()
		h.s.UpgradeOffers = []game.Upgrade{u}
		h.do(game.PickUpgrade{Upgrade: u})
	}
	h := mapBoot(t, 2)
	h.clearDoor(0, time.Minute)
	h.s.Cycles = 27
	pick(h, game.Heatsink)
	if h.s.Cycles != 30 {
		t.Errorf("heatsink: %d cycles, want 30 (the start)", h.s.Cycles)
	}

	h = mapBoot(t, 2)
	h.clearDoor(0, time.Minute)
	pick(h, game.PrefetchPick)
	d := h.s.Doors[0]
	if h.s.OpenTimer(d) != d.Timer+5*time.Minute {
		t.Errorf("prefetch: the card says %s for a %s door", h.s.OpenTimer(d), d.Timer)
	}
	h.do(game.ChooseDoor{Index: 0})
	if a := h.s.Current; a.At != 5*time.Minute || h.s.Prefetches != 0 || h.s.AtTime != 0 {
		t.Errorf("prefetch: bought %s, held %d", a.At, h.s.Prefetches)
	}

	h = mapBoot(t, 2)
	h.clearDoor(0, time.Minute)
	pick(h, game.Snapshot)
	h.open(0)
	cycles := h.s.Cycles
	h.do(subWA())
	if h.s.Cycles != cycles || h.s.Snapshots != 0 {
		t.Errorf("snapshot: %d -> %d cycles, %d held", cycles, h.s.Cycles, h.s.Snapshots)
	}
	h.do(subWA())
	if h.s.Cycles != cycles-2 {
		t.Errorf("after the snapshot: %d cycles, want %d", h.s.Cycles, cycles-2)
	}

	h = mapBoot(t, 2)
	h.clearDoor(0, time.Minute)
	n := len(h.s.Doors)
	pick(h, game.Smt) // at the room's doors: dealt again with one more
	if len(h.s.Doors) != n+1 || h.s.Smt != 0 {
		t.Errorf("smt: %d doors from %d, %d held", len(h.s.Doors), n, h.s.Smt)
	}

	h = mapBoot(t, 2)
	h.clearDoor(0, time.Minute)
	pick(h, game.Ofast)
	h.do(game.ChooseDoor{Index: 0})
	if !h.s.Current.Door.Cursed || !h.s.Current.Ofast {
		t.Fatalf("-Ofast: the door opened %+v", h.s.Current.Door)
	}
	h.now = h.now.Add(time.Minute)
	h.do(subAC())
	if h.s.UpgradePicks < 3 {
		t.Errorf("-Ofast: %d picks owed, want the clear's and 2", h.s.UpgradePicks)
	}

	h = mapBoot(t, 2)
	h.clearDoor(0, time.Minute)
	pick(h, game.Microcode)
	h.clearDoor(0, time.Minute)
	if len(h.s.UpgradeOffers) != 5 {
		t.Errorf("microcode: %d offers, want 4 and the skip", len(h.s.UpgradeOffers))
	}
}

// The daemons as they start and run.
func TestRosterDaemons(t *testing.T) {
	t.Parallel()
	start := func(h *harness, d game.Daemon) {
		h.t.Helper()
		toMergeFast(h)
		h.pick()
		h.s.DaemonOffers = []game.Daemon{d}
		h.do(game.PickDaemon{Daemon: d})
	}
	h := mapBoot(t, 3)
	start(h, game.Cpupower)
	if h.s.MaxCycles != 20 || h.s.Cycles > 20 {
		t.Errorf("cpupower: %d/%d", h.s.Cycles, h.s.MaxCycles)
	}

	h = mapBoot(t, 3)
	h.s.Cycles = 12
	start(h, game.Selinux)
	if h.s.Cycles != h.s.MaxCycles {
		t.Errorf("selinux: %d of %d", h.s.Cycles, h.s.MaxCycles)
	}
	h.do(game.LeaveMerge{})
	if h.s.Phase == game.PhaseRoute {
		h.do(game.ChooseRoute{Index: 0})
	}
	for _, d := range h.s.Doors {
		if d.Boss == "" && !d.Regression && !d.Cursed {
			t.Errorf("selinux: a topic door not cursed: %+v", d)
		}
	}

	h = mapBoot(t, 3)
	start(h, game.Quiet)
	if h.s.Ducks != 0 || h.s.QuietClock <= 0 {
		t.Errorf("quiet: %d ducks, clock +%.2f", h.s.Ducks, h.s.QuietClock)
	}

	// The daemons' clock gains cap together: nice, quiet, cpupower and
	// selinux would make a topic door 3.5x; the cap holds it to 1.75x.
	h = mapBoot(t, 3)
	bare := h.s.RouteTimer(h.m, "dp")
	h.s.Daemons = []game.Daemon{game.Nice, game.Cpupower, game.Selinux, game.Quiet}
	h.s.QuietClock = 0.4
	if got, want := h.s.RouteTimer(h.m, "dp"), time.Duration(float64(bare)*1.75); got < want-time.Minute || got > want+time.Minute {
		t.Errorf("stacked daemons: %s from %s, want about %s", got, bare, want)
	}
}

// ksmd frees cycles at a Boss I clear; burst carries a fast clear's
// unused clock onto the next door, as bought time -O3 takes out.
func TestKsmdAndBurst(t *testing.T) {
	t.Parallel()
	r := mapPlain()
	r.O3Frac = 0
	h := toBossI(t, r, func(int, *harness) {})
	h.s.Daemons = append(h.s.Daemons, game.Ksmd)
	h.s.Cycles = 20
	clearBoss(h, time.Minute)
	if h.s.Cycles != 30 {
		t.Errorf("ksmd: %d cycles, want 30", h.s.Cycles)
	}

	h = mapBoot(t, 4)
	h.s.Daemons = []game.Daemon{game.Burst}
	d := h.open(0)
	h.now = h.now.Add(time.Minute)
	h.do(subAC())
	if h.s.AtTime <= 0 || h.s.AtTime > r.Perk.BurstMax.Duration || h.s.AtTime > d.Timer/4 {
		t.Errorf("burst carried %s from a %s door", h.s.AtTime, d.Timer)
	}
}

// watchdog turns one timeout an act into overtime, for its cycles, and
// never the final's.
func TestWatchdog(t *testing.T) {
	t.Parallel()
	h := mapBoot(t, 5)
	h.s.Daemons = []game.Daemon{game.Watchdog}
	d := h.open(0)
	cycles := h.s.Cycles
	h.now = h.now.Add(d.Timer)
	h.do(game.Tick{})
	if h.s.Phase != game.PhaseProblem || !h.s.Current.Overtime || h.s.Cycles != cycles-5 {
		t.Fatalf("watchdog: phase %s, overtime %v, cycles %d", h.s.Phase, h.s.Current != nil && h.s.Current.Overtime, h.s.Cycles)
	}
	h.now = h.now.Add(4 * time.Minute)
	h.do(game.Tick{})
	if h.s.Current != nil && h.s.Current.Door == d {
		t.Error("watchdog fired twice in an act")
	}
}

// The items: each does its one thing; the shelf holds the window's five,
// fsck among them, and what is not on it is not for sale.
func TestRosterItems(t *testing.T) {
	t.Parallel()
	h := mapBoot(t, 6)
	toMergeFast(h)
	if len(h.s.Stock) != 5 || h.s.Stock[0] != game.Fsck {
		t.Fatalf("shelf %v", h.s.Stock)
	}
	for _, it := range h.s.ItemPool() {
		if !slices.Contains(h.s.Stock, it) && h.s.Price(it) != 0 {
			t.Errorf("%s is for sale off the shelf", it)
		}
	}
	buy := func(h *harness, it game.Item) {
		h.t.Helper()
		h.s.Stock = append(h.s.Stock, it)
		h.s.Uncommitted += h.s.Price(it)
		h.do(game.Spend{Item: it})
	}
	h.pick() // the window's own pick first: make waits for it
	buy(h, game.Make)
	if !h.s.PendingUpgrade {
		t.Error("make owes no pick")
	}
	h.pick()
	h.s.Ducks = 2
	buy(h, game.Mesg)
	if h.s.Ducks != 0 || h.s.AtTime != 8*time.Minute {
		t.Errorf("mesg: %d ducks, %s bought", h.s.Ducks, h.s.AtTime)
	}
	buy(h, game.Nohup)
	buy(h, game.Popd)
	buy(h, game.Diff)
	buy(h, game.Whatis)
	if !h.s.ShowsFailingInput() {
		t.Error("diff shows nothing")
	}
	h.do(game.LeaveMerge{})
	if h.s.Phase == game.PhaseRoute {
		h.do(game.ChooseRoute{Index: 0})
	}
	for _, d := range h.s.Doors {
		if d.Blind || d.Mystery {
			t.Errorf("whatis: %+v", d)
		}
	}
	h.open(0)
	if c, free := h.s.RetreatCost(); c != 0 || !free {
		t.Errorf("popd: retreat %d, free %v", c, free)
	}
	if c, ok := h.s.FailCost(game.TimedOut); c != 0 || !ok {
		t.Errorf("nohup: a timeout costs %d (%v)", c, ok)
	}
}

// sudo survives a cursed door's failed submit, for its cycles.
func TestSudo(t *testing.T) {
	t.Parallel()
	h := mapBoot(t, 7)
	h.s.Sudo = 1
	h.open(0)
	h.s.Current.Door.Cursed = true
	cycles := h.s.Cycles
	h.do(subWA())
	if h.s.Phase != game.PhaseProblem || h.s.Cycles != cycles-10 || h.s.Sudo != 0 {
		t.Errorf("sudo: phase %s, cycles %d -> %d", h.s.Phase, cycles, h.s.Cycles)
	}
	h.do(subWA())
	if h.s.Phase != game.PhaseCrashed {
		t.Errorf("a second cursed fail: phase %s", h.s.Phase)
	}
}

// kdump takes the first crash but the final's, the dump written as a
// crash's, and goes on at its cycles.
func TestKdump(t *testing.T) {
	t.Parallel()
	h := mapBoot(t, 8)
	h.s.Daemons = []game.Daemon{game.Kdump}
	h.open(0)
	h.s.Current.Door.Cursed = true
	h.s.Uncommitted = 100
	h.do(subWA())
	if h.s.Phase == game.PhaseCrashed || h.s.Cycles != 10 || !h.s.Kdumped || h.m.CoreDump == nil || h.m.CoreDump.Bytes != 100 {
		t.Fatalf("kdump: phase %s, cycles %d, dump %+v", h.s.Phase, h.s.Cycles, h.m.CoreDump)
	}
	h.pick()
	h.open(0)
	h.s.Current.Door.Cursed = true
	h.do(subWA())
	if h.s.Phase != game.PhaseCrashed {
		t.Errorf("kdump took a second crash: phase %s", h.s.Phase)
	}
}

// The Court's gate is the rules' full pool: cpupower's lower cap, full,
// does not open it; a pool full to the rules' cap does.
func TestCourtGateRulesCap(t *testing.T) {
	t.Parallel()
	h := toBossII(t, &game.Meta{Capped: 1})
	h.s.Daemons = append(h.s.Daemons, game.Cpupower)
	h.s.MaxCycles, h.s.Cycles = 20, 20
	h.do(game.LeaveMerge{})
	if x, _, ok := trialExit(h.s); !ok || x.Locked != game.LockedFull {
		t.Errorf("cpupower's full 20 opened the Court: %+v (shown %v)", x, ok)
	}
	h = toBossII(t, &game.Meta{Capped: 1})
	h.s.Cycles = h.s.Rules.MaxCycles
	h.do(game.LeaveMerge{})
	if x, _, ok := trialExit(h.s); !ok || x.Locked != "" {
		t.Errorf("a full pool left the Court locked: %+v", x)
	}
}
