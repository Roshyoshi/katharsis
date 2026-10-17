package flavor

import (
	"math"
	"strconv"
	"strings"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/units"
)

type named struct{ name, blurb string }

// biomes covers every catalog.Topics entry; the test fails when a topic is
// added without a name here.
var biomes = map[string]named{
	"complete-search": {"Brute Force Bunker", "Try every subset. 2^20 is a million; 2^60 is a career."},
	"simulation":      {"The Emulator", "Do exactly what the statement says, cycle by cycle, bugs and all."},
	"sorting-greedy":  {"The Scheduler", "Run the locally best job and pray the exchange argument holds."},
	"prefix-sums":     {"The Accumulator", "Precompute once, answer in O(1), and mind index 0."},
	"binary-search":   {"Bisect Bridge", "git bisect for answers: lo, hi, and a mid that won't converge."},
	"two-pointers":    {"The Sliding Window", "Two cursors, one array, and neither may step back."},
	"graphs":          {"Routing Table", "Every node knows its neighbours; nobody knows the way."},
	"trees":           {"The Filesystem", "One root, no cycles, n-1 edges, and a DFS deeper than your stack."},
	"shortest-paths":  {"Dijkstra's Reach", "Relax every edge, never yourself. No negative weights past here."},
	"dsu":             {"Union Hall", "find(), unite(), and path compression for the soul."},
	"dp":              {"The Cache", "Those who forget their subproblems are condemned to recompute them."},
	"math":            {"The ALU", "Everything mod 1e9+7, except the overflow you forgot."},
	"strings":         {"The Lexer", "Prefixes, suffixes, hashes, and one character you didn't escape."},
	"data-structures": {"The Heap", "Segment trees, Fenwick trees, and a priority queue of regrets."},
	"geometry":        {"The Framebuffer", "Cross products, collinear points, and an epsilon in every corner."},
	"mixed":           {"Userland", "Ring 3: anything can show up, and nothing has privileges."},
}

// nodes are the map nodes (rules.MapNodes) whose route blurb is not their
// topic's. Kernel Space is act III's mixed biome, Userland's topic one
// ring down, so it has a line of its own.
var nodes = map[string]string{
	"kernel": "Any topic, every privilege, and no MMU to catch you.",
}

// NodeBlurb is a map node's line on its exit card: its own (Kernel Space),
// else its topic's (BiomeBlurb).
func NodeBlurb(id, topic string) string {
	if b, ok := nodes[id]; ok {
		return b
	}
	return BiomeBlurb(topic)
}

// bosses covers game.BossKeys and game.FinalBoss.
var bosses = map[string]named{
	"off-by-one":         {"Off-by-One", "It waits at index n. Your loop stops at n-1. Or n+1. It knows which."},
	"heisenbug":          {"Heisenbug", "Fails on the judge; passes the moment you add a printf."},
	"oom-killer":         {"OOM Killer", "Scores every process by RSS and SIGKILLs the biggest. Guess who."},
	"race-condition":     {"Race Condition", "Two events, no ordering, and the answer goes to whoever got there first."},
	"undefined-behavior": {"Undefined Behavior", "Signed overflow, a read past the end, and demons out of your nose."},
	game.FinalBoss:       {"The Halting Problem", "You cannot know whether this ends. Turing proved it; you get to live it."},
	// The Trial (docs/design/achievements.md (f)) is no boss the bits count
	// among the six: a hidden exit's, at the Court of Hearts.
	game.TrialBoss: {"The Court", "The King judges, the Queen sentences, the jury writes it on slates."},
}

// prestiges are the USACO divisions, one per prestige level.
var prestiges = []string{"Bronze", "Silver", "Gold", "Platinum", "Advanced"}

// UpgradeTextFor is an upgrade's name and effect in r's numbers, so a
// preset that changes OverclockFrac shows what it actually grants. The
// effects follow the comments on the game.Upgrade constants.
func UpgradeTextFor(u game.Upgrade, r rules.Rules) (name, desc string) {
	switch u {
	case game.Overclock:
		desc := "+" + pct(r.OverclockFrac) + " timer on later doors"
		if r.OverclockMax > 0 {
			// Time is bought (D50): additive, capped, and paid in cycles.
			desc += "; "
			if r.OverclockCycles > 0 {
				desc += plural(r.OverclockCycles, "cycle", "cycles") + " a pick, "
			}
			desc += strconv.Itoa(r.OverclockMax) + " at most"
		}
		return "Overclock", desc + "."
	case game.ECC:
		if p := r.ECCPenalty.Duration; p > 0 {
			return "ECC", "A failed submit costs " + units.Clock(p) + " of the door's clock, not cycles, while more is left."
		}
		return "ECC", "+" + plural(r.ECCHeal(), "cycle", "cycles") + "."
	case game.Duck:
		return "Rubber Duck", "+1 duck (one hint charge)."
	case game.ShufPick:
		return "shuf", "+1 shuf: re-roll one room's doors."
	case game.Heatsink:
		return "heatsink", "+" + plural(r.Perk.HeatsinkCycles, "cycle", "cycles") + " now, up to the boot's start."
	case game.Snapshot:
		return "snapshot", "Your next failed submit is free; " + strconv.Itoa(r.Perk.SnapshotHold) + " held at most. Not on a cursed door."
	case game.PrefetchPick:
		return "prefetch", "+" + units.Clock(r.Perk.PrefetchTime.Duration) + " on the next door's clock; " + strconv.Itoa(r.Perk.PrefetchHold) + " held at most."
	case game.Turbo:
		return "turbo", "+" + pct(r.Perk.TurboFrac) + " on the next boss door's clock."
	case game.Smt:
		return "smt", "Deals this room again with " + plural(r.Perk.SmtDoors, "more door", "more doors") + " (from a merge window: the next room)."
	case game.Microcode:
		return "microcode", "Every later pick offers " + strconv.Itoa(r.UpgradeOffers+r.Perk.MicrocodeOffer) + " upgrades."
	case game.Ofast:
		return "-Ofast", "The next door you open is cursed: clear it for " + plural(r.Perk.OfastPicks, "more pick", "more picks") + "."
	case game.Fsanitize:
		return "-fsanitize", "The next boss door's samples keep their sanitizers and full output, whatever its twist."
	case game.Skip:
		return "skip", "No upgrade: +" + plural(r.Perk.SkipCycles, "cycle", "cycles") + ", up to the boot's start."
	}
	return string(u), ""
}

// DaemonTextFor is a daemon's name and effect in r's numbers. Names are the
// Unix tools they parody; the effects follow the comments on the
// game.Daemon constants.
func DaemonTextFor(d game.Daemon, r rules.Rules) (name, desc string) {
	switch d {
	case game.Strace:
		return "strace", "A failed submit shows the failing test's input."
	case game.Nice:
		desc := "+" + pct(r.NiceTimerFrac) + " timer"
		if r.NiceBytesFrac > 0 {
			desc += "; every door pays " + mult(1-r.NiceBytesFrac) + " bytes"
		}
		if r.NiceLifeCost > 1 {
			desc += "; a failed submit burns " + mult(float64(r.NiceLifeCost))
		}
		return "nice -20", desc + "."
	case game.Fork:
		return "fork", strconv.Itoa(r.ForkDoors) + " doors per room after an AC (normally " +
			strconv.Itoa(r.DoorsPerRoom) + ")."
	case game.DevRandom:
		if r.Perk.Pool > 0 {
			return "/dev/random", "A mystery door's clear owes " + plural(r.Perk.DevRandomPicks, "more pick", "more picks") + "."
		}
		return "/dev/random", "Mystery doors pay " + mult(r.DevRandomMult) + " bytes."
	case game.Kexec:
		span := "biome"
		if r.KexecPerAct {
			span = "act"
		}
		return "kexec", "One free door swap per " + span + "; a swap normally burns " + plural(r.RetreatPrice(), "cycle", "cycles") + "."
	case game.Cron:
		if r.Perk.Pool > 0 {
			return "cron", "A regression's clear frees " + plural(r.Perk.CronCycles, "cycle", "cycles") + "."
		}
		return "cron", "A regression door pays " + mult(r.CronMult) + " bytes."
	case game.Chrt:
		return "chrt -f 99", "-" + pct(r.ChrtTimerFrac) + " timer; every door pays " + mult(r.ChrtBytesMult) + " bytes."
	case game.Ccache:
		return "ccache", "A clean AC past Userland (no failed submit charged) owes " + plural(r.Perk.CcachePicks, "more pick", "more picks") + "."
	case game.Ksmd:
		return "ksmd", "Each Boss I or II clear frees " + plural(r.Perk.KsmdCycles, "cycle", "cycles") + "."
	case game.Burst:
		return "burst", "A clear carries " + pct(r.Perk.BurstFrac) + " of its unused clock to the next door, " + units.Clock(r.Perk.BurstMax.Duration) + " at most."
	case game.Watchdog:
		return "watchdog", "Once an act, a timeout becomes " + units.Clock(r.Perk.WatchdogOvertime.Duration) + " of overtime for " + plural(r.Perk.WatchdogCycles, "cycle", "cycles") + ". Never the final."
	case game.Quiet:
		return "quiet", "No ducks this boot; every clock +" + pct(r.Perk.QuietPerDuck) + " for each duck a window would refill, +" + pct(r.Perk.QuietMax) + " at most."
	case game.Cpupower:
		return "cpupower", "Every clock " + mult(r.Perk.CpupowerTimer) + "; cycles cap at " + strconv.Itoa(r.Perk.CpupowerCycles) + ", so every boss is one shot."
	case game.Selinux:
		return "selinux", "Enforcing: cycles fill now; every topic door is cursed for the rest of the boot, its clock " + mult(r.Perk.SelinuxTimer) + "."
	case game.Kdump:
		return "kdump", "Your first crash but the final's reboots you at " + plural(r.Perk.KdumpCycles, "cycle", "cycles") + "; its bytes still go to the core dump."
	case game.Tuned:
		return "tuned", "-O3 pays for a clear under " + pct(r.Perk.TunedO3Frac) + " of the clock, not " + pct(r.O3Frac) + "."
	}
	return string(d), ""
}

// pct renders a fraction as a whole-ish percent: 0.15 is "15%", not the
// "15.000000000000002%" that plain float formatting gives.
func pct(f float64) string {
	return strconv.FormatFloat(math.Round(f*1000)/10, 'f', -1, 64) + "%"
}

func mult(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) + "x" }

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// ItemTextFor is a merge-window item's name and effect in r's numbers.
func ItemTextFor(it game.Item, r rules.Rules) (name, desc string) {
	switch it {
	case game.Fsck:
		return "fsck", "+" + plural(r.FsckHeal(), "cycle", "cycles") + " now."
	case game.Dup:
		return "dup", "+1 rubber duck."
	case game.Shuf:
		return "shuf", "Re-roll one room's doors."
	case game.At:
		return "at", "+" + units.Clock(r.AtTime.Duration) + " on the next door's timer."
	case game.Nohup:
		return "nohup", "The next timeout or :q! costs no cycles; the room is still lost. Not a one-shot door."
	case game.Popd:
		return "popd", "Your next retreat is free, before its lock."
	case game.Diff:
		return "diff", "Your next failed submit shows its failing test, as strace does."
	case game.Whatis:
		return "whatis", "The next room's doors show their titles and tags; blind and mystery doors pay as plain."
	case game.Sudo:
		return "sudo", "A cursed door's failed submit costs " + plural(r.Perk.SudoCycles, "cycle", "cycles") + ", not the boot. Not the Court."
	case game.Make:
		return "make", "One upgrade pick, now."
	case game.Mesg:
		return "mesg n", "Each duck you hold becomes +" + units.Clock(r.Perk.MesgPerDuck.Duration) + " on the next door's clock."
	}
	return string(it), ""
}

// PackageText says in one line what an apt package grants every boot
// after it is installed, in r's numbers: "daemon cron joins the merge
// windows: A regression door pays 2x bytes." Effects a package stacks
// are joined with " · ".
func PackageText(p rules.Package, r rules.Rules) string {
	var parts []string
	if p.Daemon != "" {
		name, desc := DaemonTextFor(game.Daemon(p.Daemon), r)
		parts = append(parts, "daemon "+name+" joins the merge windows: "+desc)
	}
	if p.Item != "" {
		name, desc := ItemTextFor(game.Item(p.Item), r)
		parts = append(parts, "merge windows sell "+name+": "+desc)
	}
	if p.Upgrade != "" {
		name, desc := UpgradeTextFor(game.Upgrade(p.Upgrade), r)
		parts = append(parts, "upgrade "+name+" joins the picks: "+desc)
	}
	for _, c := range []struct {
		n         int
		one, many string
	}{
		{p.LinkSlots, "lib link slot", "lib link slots"},
		{p.Ducks, "rubber duck at boot start", "rubber ducks at boot start"},
		{p.DaemonOffers, "daemon on offer per merge window", "daemons on offer per merge window"},
		{p.UpgradeOffers, "upgrade on offer per pick", "upgrades on offer per pick"},
		{p.RouteOffers, "topic on offer per route", "topics on offer per route"},
		{p.Doors, "door per room (not a boss's)", "doors per room (not a boss's)"},
	} {
		if c.n > 0 {
			parts = append(parts, "+"+plural(c.n, c.one, c.many))
		}
	}
	if p.Theme != "" {
		parts = append(parts, "GRUB banner theme "+p.Theme)
	}
	return strings.Join(parts, " · ")
}
