// Package fixture is hand-built data for the out-of-boot screens: the meta
// screens' golden tests and cmd/demo-meta render the same player, so what a
// human eyeballs in the demo is what the goldens pin.
//
// Everything is fixed: times are absolute (UTC) and derived values come from
// arithmetic, not randomness, so renders are stable byte for byte.
package fixture

import (
	"fmt"
	"math"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens/meta"
)

// Now is the moment every fixture is rendered at.
var Now = time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)

// presets are custom rules presets beside "standard".
var presets = []string{"hardcore", "speedrun", "blind"}

var titles = map[catalog.ID]string{
	"usaco:1014": "Bovine Shuffle",
	"usaco:891":  "Shell Game",
	"usaco:1205": "Milk Exchange",
	"usaco:740":  "Haybale Stacking",
	"usaco:855":  "Mixing Milk",
	"usaco:856":  "The Bucket List",
	"usaco:615":  "Milk Pails",
	"usaco:1060": "Cowntact Tracing",
	"usaco:944":  "Wormhole Sort",
	"usaco:858":  "Convention",
	"usaco:1132": "Closest Cow Wins",
	"usaco:920":  "Social Distancing",
}

// lookup resolves the fixture's titles, as the app's catalog would.
func lookup(id catalog.ID) (catalog.Problem, bool) {
	t, ok := titles[id]
	return catalog.Problem{ID: id, Title: t}, ok
}

// PrestigeName pins prestige labels so goldens do not follow flavor edits.
func PrestigeName(p int) string {
	names := []string{"Bronze", "Silver", "Gold", "Platinum", "Advanced"}
	if p < 0 || p >= len(names) {
		return ""
	}
	return names[p]
}

// Meta is a player at prestige 2 with a core dump from boot #42 and two of
// three regressions due.
func Meta() game.Meta {
	return game.Meta{
		Rating:        1432,
		Prestige:      2,
		Bank:          1331,
		LifetimeBytes: 12698,
		CoreDump: &game.CoreDump{
			Problem: "usaco:1014", Bytes: 312, Boot: 42,
			At: Now.Add(-time.Hour),
		},
		Regressions: []game.Regression{
			{Problem: "usaco:1205", Due: Now.Add(-5 * time.Hour), Stage: 1},
			{Problem: "usaco:891", Due: Now.Add(-50 * time.Hour), Stage: 0},
			{Problem: "usaco:740", Due: Now.Add(72 * time.Hour), Stage: 0},
		},
		Boots: 42,
	}
}

// GrubConfig is the start screen for Meta.
func GrubConfig() meta.GrubConfig {
	return meta.GrubConfig{
		Meta:         Meta(),
		Presets:      presets,
		Lookup:       lookup,
		Now:          Now,
		PrestigeName: PrestigeName,
		Banner:       func(string) string { return "" }, // flavor's art stays out of goldens
		Gate:         game.PrestigeGate{Next: 3, Rated: 34, Need: 60},
	}
}

func at(day, h, m int) time.Time { return time.Date(2026, 10, day, h, m, 0, 0, time.UTC) }

func mins(m, s int) time.Duration { return time.Duration(m)*time.Minute + time.Duration(s)*time.Second }

// Boots is the boot history: six recent boots told in detail, then 36 older
// ones in a fixed rotation of outcomes.
func Boots() []meta.BootRow {
	boots := []meta.BootRow{
		{
			Boot: 42, Seed: 0x8f3a2c1d9e04b7a1, Preset: "standard", Prestige: 2,
			Started: at(14, 17, 58), Uptime: mins(64, 12),
			Cause: game.TimedOut, Biome: "The Heap", Problem: "Bovine Shuffle",
			Banked: 96, Dumped: 312,
			Attempts: []meta.AttemptRow{
				{Problem: "usaco:855", Title: "Mixing Milk", Topic: "simulation", Rating: 1150,
					Outcome: game.Cleared, Elapsed: mins(8, 41), Timer: mins(15, 0),
					Submits: 1, Kind: game.Independent},
				{Problem: "usaco:856", Title: "The Bucket List", Topic: "sorting-greedy", Rating: 1280,
					Outcome: game.Cleared, Elapsed: mins(13, 2), Timer: mins(15, 0),
					Submits: 3, Failed: 2, Quacks: 1, Kind: game.Hinted, Blind: true},
				{Problem: "usaco:944", Title: "Wormhole Sort", Topic: "dsu", Rating: 1610,
					Outcome: game.Cleared, Elapsed: mins(21, 37), Timer: mins(30, 0),
					Submits: 2, Failed: 1, Kind: game.Independent},
				{Problem: "usaco:1014", Title: "Bovine Shuffle", Topic: "graphs", Rating: 1720,
					Outcome: game.TimedOut, Elapsed: mins(30, 0), Timer: mins(30, 0),
					Submits: 4, Failed: 4, Quacks: 1, Cursed: true},
			},
		},
		{
			Boot: 41, Seed: 0x51c0ffee00d1ce5, Preset: "standard", Prestige: 2,
			Started: at(13, 20, 11), Uptime: mins(138, 40), Cleared: true, Banked: 1184,
			Attempts: []meta.AttemptRow{
				{Title: "Milk Pails", Topic: "complete-search", Rating: 1200, Outcome: game.Cleared,
					Elapsed: mins(6, 15), Timer: mins(15, 0), Submits: 1, Kind: game.Independent},
				{Title: "Social Distancing", Topic: "binary-search", Rating: 1390, Outcome: game.Cleared,
					Elapsed: mins(14, 50), Timer: mins(15, 0), Submits: 2, Failed: 1, Kind: game.Independent},
				{Title: "Convention", Topic: "binary-search", Rating: 1560, Outcome: game.Cleared,
					Elapsed: mins(24, 3), Timer: mins(30, 0), Submits: 1, Kind: game.Independent, Mystery: true, Blind: true},
				{Title: "Closest Cow Wins", Topic: "sorting-greedy", Rating: 1650, Outcome: game.Cleared,
					Elapsed: mins(28, 44), Timer: mins(30, 0), Submits: 3, Failed: 2, Quacks: 2, Kind: game.Hinted},
				{Title: "Cowntact Tracing", Topic: "simulation", Rating: 1750, Outcome: game.Cleared,
					Elapsed: mins(47, 10), Timer: mins(75, 0), Submits: 2, Failed: 1, Kind: game.Independent,
					Boss: "Heisenbug"},
			},
		},
		{
			Boot: 40, Seed: 0x2b7e151628aed2a6, Preset: "hardcore", Prestige: 2,
			Started: at(12, 21, 30), Uptime: mins(41, 5),
			Cause: game.GaveUp, Biome: "Page Tables", Problem: "Haybale Stacking", Banked: 0, Dumped: 140,
			Attempts: []meta.AttemptRow{
				{Title: "Shell Game", Topic: "simulation", Rating: 1100, Outcome: game.Cleared,
					Elapsed: mins(5, 2), Timer: mins(12, 0), Submits: 1, Kind: game.Independent},
				{Title: "Haybale Stacking", Topic: "prefix-sums", Rating: 1480, Outcome: game.GaveUp,
					Elapsed: mins(36, 3), Timer: mins(24, 0), Submits: 2, Failed: 2, Regress: true},
			},
		},
		{
			Boot: 39, Seed: 0x9e3779b97f4a7c15, Preset: "standard", Prestige: 1,
			Started: at(11, 19, 4), Uptime: mins(77, 30),
			Cause: game.OutOfLives, Biome: "The Scheduler", Problem: "Milk Exchange", Banked: 410, Dumped: 220,
			Attempts: []meta.AttemptRow{
				{Title: "Mixing Milk", Topic: "simulation", Rating: 1150, Outcome: game.Retreated,
					Elapsed: mins(3, 20), Timer: mins(15, 0), Submits: 0},
				{Title: "Milk Exchange", Topic: "two-pointers", Rating: 1500, Outcome: game.OutOfLives,
					Elapsed: mins(19, 47), Timer: mins(30, 0), Submits: 3, Failed: 3, Quacks: 1},
			},
		},
		{
			Boot: 38, Seed: 0x6a09e667f3bcc908, Preset: "speedrun", Prestige: 1,
			Started: at(10, 22, 15), Uptime: mins(101, 2), Cleared: true, Banked: 902,
		},
		{
			Boot: 37, Seed: 0xbb67ae8584caa73b, Preset: "standard", Prestige: 1,
			Started: at(9, 23, 50), Uptime: mins(12, 9), Banked: 0,
		},
	}
	biomes := []string{"Userland", "The Stack", "The Heap", "The Cache", "Page Tables", "Shortest Paths"}
	for i := range 36 {
		n := 36 - i
		b := meta.BootRow{
			Boot: n, Seed: uint64(n) * 0x2545f4914f6cdd1d, Preset: "standard",
			Prestige: min(n/14, 2),
			Started:  Now.Add(-time.Duration(6+i) * 24 * time.Hour).Add(-time.Duration(i%5) * time.Hour),
			Uptime:   mins(25+(i*37)%110, (i*13)%60),
			Banked:   int64((i * 53) % 400),
		}
		switch i % 5 {
		case 0:
			b.Cleared, b.Banked = true, 700+int64(i*11)
		case 1, 3:
			b.Cause, b.Biome, b.Problem = game.TimedOut, biomes[i%len(biomes)], "Teleportation"
		case 2:
			b.Cause, b.Biome, b.Problem = game.OutOfLives, biomes[(i+2)%len(biomes)], "Moocast"
		case 4:
			b.Cause, b.Biome, b.Problem = game.GaveUp, biomes[(i+4)%len(biomes)], "Angry Cows"
		}
		boots = append(boots, b)
	}
	return boots
}

// Stats is top for the same player.
func Stats() meta.Stats {
	var hist []meta.RatingPoint
	start := Now.Add(-40 * 24 * time.Hour)
	for i := range 90 {
		r := 1000 + 4.6*float64(i) + 38*math.Sin(float64(i)/4.5)
		if i%7 == 3 {
			r -= 30
		}
		hist = append(hist, meta.RatingPoint{At: start.Add(time.Duration(i) * 10 * time.Hour), Rating: math.Round(r)})
	}
	// End on a dip from a fresh peak, so peak and current differ.
	n := len(hist)
	hist[n-3].Rating, hist[n-2].Rating, hist[n-1].Rating = 1471, 1450, 1432
	return meta.Stats{
		Rating:        1432,
		RatingHistory: hist,
		Topics: []meta.TopicStat{
			{Topic: "dp", Attempts: 22, Clears: 14, Rating: 1612},
			{Topic: "graphs", Attempts: 19, Clears: 8, Rating: 1430},
			{Topic: "sorting-greedy", Attempts: 31, Clears: 24, Rating: 1515},
			{Topic: "prefix-sums", Attempts: 14, Clears: 12, Rating: 1550},
			{Topic: "binary-search", Attempts: 12, Clears: 7, Rating: 1390},
			{Topic: "simulation", Attempts: 26, Clears: 23, Rating: 1350},
			{Topic: "complete-search", Attempts: 18, Clears: 15, Rating: 1320},
			{Topic: "two-pointers", Attempts: 7, Clears: 5, Rating: 1405},
			{Topic: "trees", Attempts: 9, Clears: 3, Rating: 1280},
			{Topic: "shortest-paths", Attempts: 6, Clears: 2, Rating: 1240},
			{Topic: "dsu", Attempts: 4, Clears: 3, Rating: 1460},
			{Topic: "math", Attempts: 11, Clears: 6, Rating: 1300},
			{Topic: "strings", Attempts: 3, Clears: 1, Rating: 1150},
			{Topic: "data-structures", Attempts: 5, Clears: 2, Rating: 1260},
			{Topic: "geometry", Rating: 1000},
			{Topic: "mixed", Attempts: 8, Clears: 5, Rating: 1380},
		},
		Solves: map[game.SolveKind]int{
			game.Independent: 88, game.Hinted: 30, game.Editorial: 12,
		},
		Boots:      42,
		FullClears: 9,
		Crashes: map[game.Outcome]int{
			game.TimedOut: 18, game.OutOfLives: 10, game.GaveUp: 4,
		},
		Uptime:        51*time.Hour + 12*time.Minute + 9*time.Second,
		LifetimeBytes: 12698,
		Bank:          1331,
	}
}

// Doc is a reader page as plain strings, so this package need not import
// the reader.
type Doc struct{ Title, Source, URL, Markdown string }

// Statement is a USACO-style statement: TeX math in prose, sample I/O in
// bare fences, the way the content pipeline converts usaco.org pages.
var Statement = Doc{
	Title:  "Bovine Shuffle",
	Source: "USACO 2024 December, Silver P2",
	URL:    "https://usaco.org/index.php?page=viewproblem2&cpid=1014",
	Markdown: `**Note: The time limit for this problem is 4s, twice the default.**

Farmer John has $N$ cows ($1 \le N \le 2 \cdot 10^5$) standing in a line,
the $i$-th of which has height $h_i$ ($1 \le h_i \le 10^9$). Every
morning he performs the same shuffle: for each $i$, the cow in position
$i$ moves to position $a_i$, where $a_1, a_2, \ldots, a_N$ is given.

A position is *always occupied* if some cow stands there after every
shuffle, no matter how many shuffles have happened. Count the always
occupied positions, and output $\lfloor S / 2 \rfloor \bmod (10^9+7)$
where $S$ is the sum of their heights.

### INPUT FORMAT (input arrives from the terminal / stdin):

The first line contains $N$. The second line contains $a_1 \ldots a_N$
($1 \le a_i \le N$). The third line contains $h_1 \ldots h_N$.

### OUTPUT FORMAT (print output to the terminal / stdout):

Two integers: the count, and $\lfloor S/2 \rfloor$ modulo $10^9+7$.

### SAMPLE INPUT:

` + "```" + `
4
3 2 1 3
5 1 7 2
` + "```" + `

### SAMPLE OUTPUT:

` + "```" + `
3 6
` + "```" + `

Positions 1, 2 and 3 stay occupied; $S = 5 + 1 + 7 = 13$ and
$\lfloor 13/2 \rfloor = 6$.

**SCORING:**

- Inputs 2-5: $N \le 100$
- Inputs 6-10: $N \le 10^3$
- Inputs 11-14: no additional constraints.

Problem credits: Nick Wu
`,
}

// Learn is a learn-mode page in the style of a USACO Guide module.
var Learn = Doc{
	Title:  "Prefix Sums",
	Source: "USACO Guide · Silver",
	URL:    "https://usaco.guide/silver/prefix-sums",
	Markdown: `## Introduction

Given an array $a_1, a_2, \ldots, a_N$, a **prefix sum** array stores
$p_k = \sum_{i=1}^{k} a_i$ for every $k$, with $p_0 = 0$. Any range sum
is then a difference of two prefix sums:

$$
a_l + a_{l+1} + \cdots + a_r = p_r - p_{l-1}
$$

Building $p$ takes $O(N)$ and each query $O(1)$, so $Q$ queries cost
$O(N + Q)$ instead of $O(NQ)$.

## Implementation

` + "```cpp" + `
#include <bits/stdc++.h>
using namespace std;

int main() {
	int n, q;
	cin >> n >> q;
	vector<long long> p(n + 1);
	for (int i = 1; i <= n; i++) {
		long long a;
		cin >> a;
		p[i] = p[i - 1] + a;
	}
	while (q--) {
		int l, r;
		cin >> l >> r;
		cout << p[r] - p[l - 1] << "\n";  // sum of a[l..r]
	}
}
` + "```" + `

## Pitfalls

- Sums overflow ` + "`int`" + ` quickly: $N = 2 \times 10^5$ values up to $10^9$
  reach $2 \times 10^{14}$. Use ` + "`long long`" + `.
- Off by one: with 1-indexed $p$, the range $[l, r]$ is $p_r - p_{l-1}$.

| Problem | Source | Difficulty |
| --- | --- | --- |
| Static Range Sum Queries | CSES | Easy |
| Breed Counting | USACO Silver | Easy |
| Subsequences Summing to Sevens | USACO Silver | Normal |
`,
}

// Editorial is a short official-analysis page.
var Editorial = Doc{
	Title:  "Bovine Shuffle — analysis",
	Source: "USACO official analysis",
	URL:    "https://usaco.org/current/data/sol_shuffle_silver_dec17.html",
	Markdown: fmt.Sprintf(`(Analysis by Nick Wu)

Treat the shuffle as a functional graph: position $i$ has one edge to
$a_i$. A position stays occupied forever exactly when it lies on a
cycle, since every other position eventually empties. Repeatedly delete
positions with in-degree $0$ (a queue, as in topological sort); what
remains are the cycles. This is $O(N)$.

%s
for (int i = 1; i <= n; i++) if (indeg[i] == 0) q.push(i);
while (!q.empty()) {
	int v = q.front(); q.pop(); alive[v] = false;
	if (--indeg[a[v]] == 0) q.push(a[v]);
}
%s
`, "```cpp", "```"),
}
