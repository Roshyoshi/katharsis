package flavor

import (
	"math"
	"strconv"

	"katharsis/internal/game"
	"katharsis/internal/rules"
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

// bosses covers game.BossKeys and game.FinalBoss.
var bosses = map[string]named{
	"off-by-one":         {"Off-by-One", "It waits at index n. Your loop stops at n-1. Or n+1. It knows which."},
	"heisenbug":          {"Heisenbug", "Fails on the judge; passes the moment you add a printf."},
	"oom-killer":         {"OOM Killer", "Scores every process by RSS and SIGKILLs the biggest. Guess who."},
	"race-condition":     {"Race Condition", "Two events, no ordering, and the answer goes to whoever got there first."},
	"undefined-behavior": {"Undefined Behavior", "Signed overflow, a read past the end, and demons out of your nose."},
	game.FinalBoss:       {"The Halting Problem", "You cannot know whether this ends. Turing proved it; you get to live it."},
}

// prestiges are the USACO divisions, mapped onto x86 privilege rings: each
// level sits one ring closer to the metal, and Advanced runs below ring 0
// like a hypervisor.
var prestiges = []struct {
	name string
	ring int
}{
	{"Bronze", 3},
	{"Silver", 2},
	{"Gold", 1},
	{"Platinum", 0},
	{"Advanced", -1},
}

// PrestigeRing is the ring flavor for a prestige level, e.g. "ring 3"; ""
// past the table.
func PrestigeRing(p int) string {
	if p >= 0 && p < len(prestiges) {
		return "ring " + strconv.Itoa(prestiges[p].ring)
	}
	return ""
}

// PrestigeLabel is PrestigeName with its ring, e.g. "Bronze · ring 3", for
// screens with room for both.
func PrestigeLabel(p int) string {
	if r := PrestigeRing(p); r != "" {
		return PrestigeName(p) + " · " + r
	}
	return PrestigeName(p)
}

// UpgradeTextFor is UpgradeText with the numbers of r, so a preset that
// changes OverclockFrac shows what it actually grants. The effects follow
// the comments on the game.Upgrade constants.
func UpgradeTextFor(u game.Upgrade, r rules.Rules) (name, desc string) {
	switch u {
	case game.Overclock:
		return "Overclock", "+" + pct(r.OverclockFrac) + " timer on later doors."
	case game.ECC:
		return "ECC", "+1 life."
	case game.Duck:
		return "Rubber Duck", "+1 duck (one hint charge)."
	}
	return string(u), ""
}

// DaemonTextFor is DaemonText with the numbers of r. Names are the Unix
// tools they parody; the effects follow the comments on the game.Daemon
// constants.
func DaemonTextFor(d game.Daemon, r rules.Rules) (name, desc string) {
	switch d {
	case game.Strace:
		return "strace", "A failed submit shows the failing test's input."
	case game.Nice:
		return "nice -20", "+" + pct(r.NiceTimerFrac) + " timer; a failed submit costs " +
			plural(r.NiceLifeCost, "life", "lives") + "."
	case game.Fork:
		return "fork", strconv.Itoa(r.ForkDoors) + " doors per room after an AC (normally " +
			strconv.Itoa(r.DoorsPerRoom) + ")."
	case game.DevRandom:
		return "/dev/random", "Mystery doors pay " + mult(r.DevRandomMult) + " bytes."
	case game.Kexec:
		return "kexec", "One free door swap per biome; a swap normally costs a life."
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
