package game

import (
	"math/rand/v2"
	"slices"
	"time"

	"katharsis/internal/rules"
)

// The pick pools past the first seven daemons, four upgrades and four items
// (rules.PerkRules, docs/research/perks.md): ten a pool from the first
// boot, drawn by rarity, under a boot whose rules have Perk.Pool 1. A boot
// saved before them (Pool 0) keeps the first pools and their uniform draw.

// The daemons past the first seven. kdump and tuned are apt's.
const (
	Ccache   Daemon = "ccache"   // a clean AC after Userland owes one more pick
	Ksmd     Daemon = "ksmd"     // each Boss I or II clear frees cycles
	Burst    Daemon = "burst"    // a clear carries some of its unused clock onward
	Watchdog Daemon = "watchdog" // once an act, a timeout becomes overtime, for cycles
	Quiet    Daemon = "quiet"    // no ducks; the clock for the ducks given up
	Cpupower Daemon = "cpupower" // every clock longer; cycles capped, so bosses are one shot
	Selinux  Daemon = "selinux"  // cycles full; every topic door cursed, its clock longer
	Kdump    Daemon = "kdump"    // the first crash but the final's reboots at cycles
	Tuned    Daemon = "tuned"    // -O3 under a longer share of the clock
)

// The upgrades past the first four. fsanitize is apt's; Skip is the pick
// taken as cycles, offered at every pick and never held.
const (
	Heatsink     Upgrade = "heatsink"  // cycles now, up to the start
	Snapshot     Upgrade = "snapshot"  // the next failed submit is free
	PrefetchPick Upgrade = "prefetch"  // clock on the next door
	Turbo        Upgrade = "turbo"     // clock on the next boss door
	Smt          Upgrade = "smt"       // the next room has one more door
	Microcode    Upgrade = "microcode" // later picks offer one more
	Ofast        Upgrade = "ofast"     // the next door is cursed; its clear owes picks
	Fsanitize    Upgrade = "fsanitize" // the next boss door's samples keep their sanitizers
	Skip         Upgrade = "skip"      // no upgrade: a few cycles instead
)

// The items past the first four.
const (
	Nohup  Item = "nohup"  // the next timeout or :q! costs no cycles
	Popd   Item = "popd"   // the next retreat is free
	Diff   Item = "diff"   // the next failed submit shows its test, as strace does
	Whatis Item = "whatis" // the next room's doors show what they are
	Sudo   Item = "sudo"   // a cursed door's fail costs cycles, not the boot
	Make   Item = "make"   // one upgrade pick now
	Mesg   Item = "mesg"   // each duck held becomes clock on the next door
)

// The new pools, in the order cards list them after the first ones.
var (
	PerkDaemons  = []Daemon{Ccache, Ksmd, Burst, Watchdog, Quiet, Cpupower, Selinux, Kdump, Tuned}
	PerkUpgrades = []Upgrade{Heatsink, Snapshot, PrefetchPick, Turbo, Smt, Microcode, Ofast, Fsanitize}
	PerkItems    = []Item{Nohup, Popd, Diff, Whatis, Sudo, Make, Mesg}
)

// Rarity is a perk's odds in a draw (rules.PerkRules.Weights), and the tag
// its card shows.
type Rarity int

const (
	Common Rarity = iota
	Uncommon
	Rare
)

func (r Rarity) String() string { return [...]string{"common", "uncommon", "rare"}[r] }

// rarities are the perks that are not common: the build-definers rare, the
// strong nets and engines uncommon.
var rarities = map[string]Rarity{
	string(Strace): Uncommon, string(Chrt): Uncommon, string(Ccache): Uncommon, string(Watchdog): Uncommon,
	string(Quiet): Uncommon, string(Cpupower): Rare, string(Selinux): Rare, string(Kdump): Rare,
	string(Overclock): Uncommon, string(ECC): Uncommon, string(Turbo): Uncommon, string(Microcode): Uncommon,
	string(Ofast): Rare, string(Fsanitize): Rare,
}

// RarityOf is a daemon's or an upgrade's rarity.
func RarityOf[T ~string](x T) Rarity { return rarities[string(x)] }

// perks reports whether the boot draws from the ten-a-pool roster.
func (s *State) perks() bool { return s.Rules.Perk.Pool >= 1 }

// DaemonPool, UpgradePool and ItemPool are every perk the boot's rules
// know, locked ones included: the first ones, then the roster's.
func (s *State) DaemonPool() []Daemon {
	if !s.perks() {
		return AllDaemons
	}
	return slices.Concat(AllDaemons, PerkDaemons)
}

func (s *State) UpgradePool() []Upgrade {
	if !s.perks() {
		return AllUpgrades
	}
	return slices.Concat(AllUpgrades, PerkUpgrades)
}

func (s *State) ItemPool() []Item {
	if !s.perks() {
		return AllItems
	}
	return slices.Concat(AllItems, PerkItems)
}

// draw takes n of xs: uniformly under the first pools, else by rarity
// weight without repeats.
func draw[T ~string](s *State, rng *rand.Rand, xs []T, n int) []T {
	if !s.perks() {
		return sample(rng, xs, n)
	}
	w := s.Rules.Perk.Weights
	pool := slices.Clone(xs)
	var out []T
	for len(out) < n && len(pool) > 0 {
		total := 0
		for _, x := range pool {
			total += w[RarityOf(x)]
		}
		if total <= 0 {
			break
		}
		k := rng.IntN(total)
		for i, x := range pool {
			if k -= w[RarityOf(x)]; k < 0 {
				out = append(out, x)
				pool = slices.Delete(pool, i, i+1)
				break
			}
		}
	}
	return out
}

// has reports whether daemon d runs this boot.
func (s *State) has(d Daemon) bool { return slices.Contains(s.Daemons, d) }

// daemonClock is what the running daemons multiply a door's clock by, the
// gains capped together (rules.PerkRules.DaemonTimerMax) and chrt's cut
// after: nice, quiet's ducks given up, cpupower, and selinux on a cursed
// topic door. Under the first pools it is nice's and chrt's alone.
func (s *State) daemonClock(boss bool) (gain, cut float64) {
	r := &s.Rules
	gain, cut = 1, 1
	if s.has(Nice) {
		gain *= 1 + r.NiceTimerFrac
	}
	if s.has(Chrt) {
		cut = 1 - r.ChrtTimerFrac
	}
	if !s.perks() {
		return gain, cut
	}
	p := &r.Perk
	if s.has(Quiet) {
		gain *= 1 + s.QuietClock
	}
	if s.has(Cpupower) {
		gain *= p.CpupowerTimer
	}
	if s.has(Selinux) && !boss {
		gain *= p.SelinuxTimer
	}
	return min(gain, max(p.DaemonTimerMax, 1)), cut
}

// quietClock is what quiet adds to the clock when picked: QuietPerDuck for
// each duck a window would refill, QuietMax at most; 0, and quiet not
// offered, when the refill is none already (P2's ducks).
func (e *engine) quietClock() float64 {
	p := &e.s.Rules.Perk
	return min(p.QuietPerDuck*float64(e.duckRefill()), p.QuietMax)
}

// refillDucks is how many ducks a merge window tops the boot up to: its
// base and one per Duck upgrade, none under quiet.
func (e *engine) refillDucks() int {
	if e.s.has(Quiet) {
		return 0
	}
	return e.duckRefill()
}

// duckRefill is the refill quiet aside: the base and the Duck upgrades.
func (e *engine) duckRefill() int {
	n := e.baseDucks()
	for _, u := range e.s.Upgrades {
		if u == Duck {
			n++
		}
	}
	return n
}

// startCap is where the heals that stop at the start stop (heatsink, a
// skip): the starting cycles, or the boot's lower cap (cpupower).
func (s *State) startCap() int { return min(s.StartCycles(), s.MaxCycles) }

// heal adds n cycles up to limit, never taking any away.
func (s *State) heal(n, limit int) {
	if s.Cycles < limit {
		s.Cycles = min(s.Cycles+n, limit)
	}
}

// actOf is the act the boot stands in, for once-an-act perks (watchdog).
func (s *State) actOf() int { return s.ActOf(s.Biome) }

// charged counts a's failed submits that were charged (not a sample's,
// not a free verdict): what ccache calls unclean.
func (s *State) charged(a Attempt) int {
	n := 0
	for _, ev := range a.Events {
		if ev.Kind == "submit" && ev.Verdict != Accepted && !ev.SampleFailed &&
			!slices.Contains(s.Rules.FreeVerdicts, string(ev.Verdict)) {
			n++
		}
	}
	return n
}

// stock draws a merge window's shelf (rules.PerkRules.Stock): fsck when
// it is for sale, then the rest drawn from what is unlocked.
func (e *engine) stock() {
	s := e.s
	if !s.perks() || s.Trial != nil {
		s.Stock = nil
		return
	}
	var rest []Item
	s.Stock = nil
	for _, it := range s.ItemPool() {
		switch {
		case s.ItemLocked(it) || s.basePrice(it) <= 0:
		case it == Fsck:
			s.Stock = append(s.Stock, it)
		default:
			rest = append(rest, it)
		}
	}
	s.Stock = append(s.Stock, sample(e.rng, rest, s.Rules.Perk.Stock-len(s.Stock))...)
}

// Stocked reports whether a merge window has it on its shelf: always under
// the first pools, which sold everything unlocked.
func (s *State) Stocked(it Item) bool { return !s.perks() || slices.Contains(s.Stock, it) }

// burstCarry is what burst carries from attempt a, cleared at now: its
// share of the clock a left unused, the time bought for a (Attempt.At,
// carried time among it) excluded, so carried time never carries again.
func (s *State) burstCarry(a *Attempt, now time.Time) time.Duration {
	p := &s.Rules.Perk
	if !s.perks() || !s.has(Burst) {
		return 0
	}
	unused := a.Deadline.Sub(now) - a.At
	if unused <= 0 {
		return 0
	}
	return min(scale(unused, p.BurstFrac), p.BurstMax.Duration)
}

// courtGate is the cycles the Court's full-pool gate asks for: the rules'
// cap, not the boot's, which cpupower lowers and selinux fills.
func (s *State) courtGate() int { return s.Rules.MaxCycles }

// everyBossOneShot reports whether every boss of the boot is one shot
// already (State.OneShot's count: P8's), where cpupower's catch is none.
func (s *State) everyBossOneShot() bool {
	n := 0
	for _, b := range s.Biomes {
		if b.Kind == rules.Boss {
			n++
		}
	}
	return max(s.Rules.OneShot, s.Rules.Ascend(s.Prestige).OneShot) >= n
}

// offerDaemons draws a merge window's daemons: with a seat free, the
// unlocked ones not running and not idle, by rarity, the window after Boss
// I holding a rare among them (Slay the Spire's boss relic); every seat
// full, the roster's windows offer them as swaps (PickDaemon.Replace, for
// SwapPrice), so the later windows have a daemon decision too. A trial
// offers none: fork and kexec need rooms that re-roll, which a trial's do
// not, and the rest are boot perks.
func (e *engine) offerDaemons() {
	s, r := e.s, &e.s.Rules
	full := len(s.Daemons) >= r.MaxDaemons
	if s.Trial != nil || full && !s.perks() {
		return
	}
	// D62: the roster is set once the first DaemonActs acts are done, the
	// window after their last boss the last to offer one.
	if r.DaemonActs > 0 && s.ActOf(s.Biome) >= r.DaemonActs {
		s.DaemonOffers = nil
		return
	}
	var pool, rares []Daemon
	for _, d := range s.DaemonPool() {
		if s.has(d) || s.DaemonLocked(d) || s.idle(d) || d == Quiet && e.duckRefill() == 0 {
			continue
		}
		pool = append(pool, d)
		if RarityOf(d) == Rare {
			rares = append(rares, d)
		}
	}
	n := r.DaemonOffers + r.Grants(s.Packages).DaemonOffers
	b := s.Biome
	if s.perks() && !full && len(rares) > 0 && b < len(s.Biomes) && s.Biomes[b].Kind == rules.Boss && s.ActOf(b) == 0 {
		relic := sample(e.rng, rares, 1)
		pool = slices.DeleteFunc(pool, func(d Daemon) bool { return d == relic[0] })
		s.DaemonOffers = append(relic, draw(s, e.rng, pool, n-1)...)
		return
	}
	s.DaemonOffers = draw(s, e.rng, pool, n)
}

// swapOut frees replace's seat for a daemon taken over it, for SwapPrice
// of the uncommitted bytes.
func (e *engine) swapOut(replace Daemon) error {
	s, p := e.s, &e.s.Rules.Perk
	switch {
	case !s.perks():
		return invalid("every daemon seat is taken")
	case !s.has(replace):
		return invalid("swap needs a running daemon to replace, not %q", replace)
	case s.Uncommitted < p.SwapPrice:
		return invalid("a swap costs %d B, %d B uncommitted", p.SwapPrice, s.Uncommitted)
	}
	s.Uncommitted -= p.SwapPrice
	s.Spent += p.SwapPrice
	s.Daemons = slices.DeleteFunc(s.Daemons, func(d Daemon) bool { return d == replace })
	switch replace {
	case Cpupower:
		s.MaxCycles = max(s.Rules.MaxCycles, s.MaxCycles)
	case Quiet:
		s.QuietClock = 0
	}
	e.logf("daemon: %s stopped for a swap, -%d B", replace, p.SwapPrice)
	return nil
}

// startDaemon carries out what a daemon does as it starts: quiet takes the
// ducks for its clock, cpupower caps the cycles, selinux fills them.
func (e *engine) startDaemon(d Daemon) {
	s, p := e.s, &e.s.Rules.Perk
	switch d {
	case Quiet:
		s.QuietClock = e.quietClock()
		s.Ducks = 0
	case Cpupower:
		s.MaxCycles = min(s.MaxCycles, p.CpupowerCycles)
		s.Cycles = min(s.Cycles, s.MaxCycles)
	case Selinux:
		s.Cycles = s.MaxCycles
	}
}

// o3Frac is the share of a door's clock -O3 asks a clear to beat: tuned's
// when it runs, else the rules'.
func (s *State) o3Frac() float64 {
	if s.perks() && s.has(Tuned) {
		return max(s.Rules.O3Frac, s.Rules.Perk.TunedO3Frac)
	}
	return s.Rules.O3Frac
}

// rosterClear carries out what the roster does at a clear the boot goes on
// from, and returns the picks it owes: ccache's for a clean door past
// Userland, dev-random's for a mystery door, -Ofast's for the door it
// cursed; ksmd frees cycles at a Boss I or II clear, cron at a regression's.
func (e *engine) rosterClear(a Attempt) int {
	s, p := e.s, &e.s.Rules.Perk
	if !s.perks() {
		return 0
	}
	d, picks := a.Door, 0
	if s.has(Ccache) && a.Biome > 0 && s.charged(a) == 0 {
		picks += p.CcachePicks
		e.logf("ccache: a clean AC, %d more pick", p.CcachePicks)
	}
	if s.has(DevRandom) && d.Mystery {
		picks += p.DevRandomPicks
		e.logf("/dev/random: a mystery cleared, %d more pick", p.DevRandomPicks)
	}
	if a.Ofast {
		picks += p.OfastPicks
		e.logf("-Ofast: the cursed door cleared, %d more picks", p.OfastPicks)
	}
	if s.has(Ksmd) && d.Boss != "" {
		s.heal(p.KsmdCycles, s.MaxCycles)
		e.logf("ksmd: +%d cycles merged back, %d", p.KsmdCycles, s.Cycles)
	}
	if s.has(Cron) && d.Regression {
		s.heal(p.CronCycles, s.MaxCycles)
		e.logf("cron: the regression held, +%d cycles, %d", p.CronCycles, s.Cycles)
	}
	return picks
}

// watchdog turns the open door's timeout into overtime, once an act: its
// deadline moves WatchdogOvertime on for WatchdogCycles, and a clear in it
// pays and rates as overtime does. Never on a one-shot door (the final,
// the Court), only while the cycles cover it, and still a hit (vault.go).
func (e *engine) watchdog() bool {
	s, p := e.s, &e.s.Rules.Perk
	a := s.Current
	act := s.actOf() + 1
	if a == nil || !s.perks() || !s.has(Watchdog) || slices.Contains(s.WatchdogActs, act) ||
		s.OneShot() || s.Cycles <= p.WatchdogCycles {
		return false
	}
	dl := a.Deadline.Add(p.WatchdogOvertime.Duration)
	if !e.now.Before(dl) {
		return false // the game was closed past the overtime too
	}
	a.Deadline, a.Overtime = dl, true
	s.Cycles -= p.WatchdogCycles
	s.WatchdogActs = append(s.WatchdogActs, act)
	e.hit()
	e.event(AttemptEvent{Kind: "overtime"})
	e.logf("%s: watchdog: overtime to %s, -%d cycles, %d left", e.door(), dl.Format(time.TimeOnly), p.WatchdogCycles, s.Cycles)
	return true
}

// sudo survives a failed submit on a cursed door for SudoCycles instead of
// the boot, while one is held and the cycles cover it; never the Court,
// whose curse is its rule.
func (e *engine) sudo() bool {
	s, p := e.s, &e.s.Rules.Perk
	if !s.perks() || s.Sudo <= 0 || s.Current.Door.Boss == TrialBoss || s.Cycles <= p.SudoCycles {
		return false
	}
	s.Sudo--
	s.Cycles -= p.SudoCycles
	e.logf("%s: sudo: the curse held off, -%d cycles, %d left", e.door(), p.SudoCycles, s.Cycles)
	return true
}

// rosterOfferable is whether the roster's upgrade u may be offered, with
// roster false for one it does not rule on: each repeatable upgrade has a
// hold cap (Risk of Rain 2's lesson: percentages stack away), smt waits
// out a boss's room, and quiet takes Duck off the pool.
func (s *State) rosterOfferable(u Upgrade) (ok, roster bool) {
	if !s.perks() {
		return false, false
	}
	p := &s.Rules.Perk
	switch u {
	case Heatsink:
		return s.Cycles < s.startCap(), true
	case Snapshot:
		return s.Snapshots < p.SnapshotHold, true
	case PrefetchPick:
		return s.Prefetches < p.PrefetchHold, true
	case Turbo:
		return s.Turbo == 0, true
	case Smt:
		return s.Smt == 0 && !s.nextRoomBoss(), true
	case Microcode:
		return s.count(Microcode) == 0, true
	case Ofast:
		return !s.Ofast, true
	case Fsanitize:
		return s.Fsanitizes == 0, true
	case Duck:
		return !s.has(Quiet), true
	}
	return false, false
}

// nextRoomBoss reports whether the room smt would deal is a boss's, whose
// one door it cannot add to: the room on screen at its doors, the next
// biome's first from a merge window.
func (s *State) nextRoomBoss() bool {
	b := s.Biome
	if b < len(s.Biomes) && s.Room >= s.Biomes[b].Rooms {
		b++
	}
	return b < len(s.Biomes) && s.Biomes[b].Kind == rules.Boss
}

// rosterPick carries out the roster's upgrade u as it is taken.
func (e *engine) rosterPick(u Upgrade) {
	s, p := e.s, &e.s.Rules.Perk
	switch u {
	case Skip:
		s.heal(p.SkipCycles, s.startCap())
	case Heatsink:
		s.heal(p.HeatsinkCycles, s.startCap())
	case Snapshot:
		s.Snapshots++
	case PrefetchPick:
		s.AtTime += p.PrefetchTime.Duration
		s.Prefetches++
	case Turbo:
		s.Turbo = 1
	case Smt:
		// Picked at a room's doors (after a clear), it deals them again
		// with more; picked in a merge window, it waits for the next room.
		s.Smt += p.SmtDoors
		if s.Phase == PhaseDoors && len(s.Doors) > 0 {
			e.rollRoom(false, "smt")
		}
	case Ofast:
		s.Ofast = true
	case Fsanitize:
		s.Fsanitizes++
	}
}

// kdump reboots the boot into its capture kernel at its first crash but
// the final's or the Court's (they end the boot by rule), never a trial's:
// the panic is written as a crash writes it (the uncommitted bytes become
// the core dump, the door a regression), and the boot goes on at
// KdumpCycles with the room lost. Abandoning a boot is no crash it takes.
func (e *engine) kdump(cause Outcome) bool {
	s, m, p := e.s, e.meta, &e.s.Rules.Perk
	if !s.perks() || !s.has(Kdump) || s.Kdumped || s.Trial != nil || s.Current == nil {
		return false
	}
	d := s.Current.Door
	if d.Boss == FinalBoss || d.Boss == TrialBoss || s.Biome >= len(s.Biomes)-1 || s.Rules.Biomes[s.Biome].Ending != "" {
		return false
	}
	b := s.Biomes[s.Biome]
	name := e.door()
	a := e.close(cause)
	if m.CoreDump != nil {
		e.logf("coredump: %d B on %s overwritten", m.CoreDump.Bytes, m.CoreDump.Problem)
		m.CoreDump = nil
	}
	if s.Uncommitted > 0 {
		m.CoreDump = &CoreDump{Problem: d.Problem, Bytes: s.Uncommitted, Boot: s.Boot, At: e.now, Mult: s.Rules.DumpIndependentMult}
		e.logf("core dumped: %d B on %s", s.Uncommitted, d.Problem)
		s.KdumpBytes += s.Uncommitted
		s.Uncommitted = 0
	}
	e.regress(d.Problem)
	s.Kdumped = true
	s.Cycles = min(p.KdumpCycles, s.MaxCycles)
	e.hit()
	e.logf("%s: kdump: the panic captured, rebooted at %d cycles", name, s.Cycles)
	e.emit(CloseProblem{Door: a.Door}, Sound{Name: "crash"})
	switch {
	case b.Kind == rules.Boss:
		s.Phase = PhaseDoors
		e.rollRoom(false, "retry")
	case s.Room+1 < b.Rooms:
		s.Room++
		s.Phase = PhaseDoors
		e.rollRoom(true, "fresh")
	default:
		s.Room++
		e.enterMerge()
	}
	return true
}
