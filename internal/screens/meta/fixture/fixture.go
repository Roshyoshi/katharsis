// Package fixture is hand-built data for the out-of-boot screens: the meta
// screens' golden tests and cmd/demo-meta render the same player, so what a
// human eyeballs in the demo is what the goldens pin.
//
// Everything is fixed: times are absolute (UTC) and derived values come from
// arithmetic, not randomness, so renders are stable byte for byte.
package fixture

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens/meta"
	"katharsis/internal/setuid"
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
// Past the divisions it falls back as flavor does, to "Prestige N".
func PrestigeName(p int) string {
	names := []string{"Bronze", "Silver", "Gold", "Platinum", "Advanced"}
	if p < 0 || p >= len(names) {
		return fmt.Sprintf("Prestige %d", p)
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
		// The story's words stay out too: stubs of realistic length.
		Story:    func(hook string, _ uint64) string { return "stub " + hook + ", a line in the story's voice" },
		Gate:     game.PrestigeGate{Next: 3, Rated: 34, Need: 60},
		Packages: Packages(),
		// The same TIME+ as top's Stats.
		TimePlus: Stats().TimePlus,
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

// Upsolves are the two upsolves the calendar counts: boot #40's :q! door
// solved after its editorial, recovering part of that core dump, and boot
// #42's timed-out door left unsolved, its dump still waiting.
func Upsolves() []meta.UpsolveRow {
	return []meta.UpsolveRow{
		{
			Problem: "usaco:1014", Title: "Bovine Shuffle", Topic: "graphs", Rating: 1720,
			Opened: at(14, 19, 10), Elapsed: mins(22, 5),
			Verdicts: []meta.VerdictRow{
				{Verdict: "TLE", Passed: 7, Total: 10, At: mins(9, 30)},
				{Verdict: "WA", Passed: 8, Total: 10, At: mins(18, 2)},
			},
		},
		{
			Problem: "usaco:740", Title: "Haybale Stacking", Topic: "prefix-sums", Rating: 1480,
			Opened: at(12, 22, 20), Elapsed: mins(14, 30),
			Solved: game.Editorial, Editorial: true, Recovered: 70,
			Verdicts: []meta.VerdictRow{
				{Verdict: "WA", Passed: 6, Total: 10, At: mins(4, 10)},
				{Verdict: "AC", Passed: 10, Total: 10, At: mins(12, 55), Editorial: true},
			},
		},
	}
}

// Calendar is git log's calendar for the same player: the boots, and two
// upsolves of what boots #40 and #42 crashed on (Upsolves).
func Calendar() meta.Calendar {
	return CalendarOf(Boots(),
		meta.Day{Date: day(at(12, 0, 0)), Door: mins(14, 30), Upsolves: 1},
		meta.Day{Date: day(at(14, 0, 0)), Door: mins(22, 5), Upsolves: 1})
}

// CalendarOf is the calendar a store would give for boots and the upsolve
// days ups: each boot's uptime, start and end on the day it started, and a
// load average of the days with 15 minutes or more (RT9).
func CalendarOf(boots []meta.BootRow, ups ...meta.Day) meta.Calendar {
	byDay := map[time.Time]*meta.Day{}
	add := func(d meta.Day) {
		p := byDay[d.Date]
		if p == nil {
			p = &meta.Day{Date: d.Date}
			byDay[d.Date] = p
		}
		p.Door += d.Door
		p.Boots += d.Boots
		p.Clears += d.Clears
		p.Crashes += d.Crashes
		p.Upsolves += d.Upsolves
	}
	for _, b := range boots {
		d := meta.Day{Date: day(b.Started), Door: b.Uptime, Boots: 1}
		switch {
		case b.Cleared:
			d.Clears = 1
		case b.Cause != "":
			d.Crashes = 1
		}
		add(d)
	}
	for _, u := range ups {
		add(u)
	}
	c := meta.Calendar{Today: Now}
	for _, d := range byDay {
		c.Days = append(c.Days, *d)
	}
	slices.SortFunc(c.Days, func(a, b meta.Day) int { return a.Date.Compare(b.Date) })

	// GRUB's rule: the windows end today once today counts, yesterday
	// until then.
	practiced := func(t time.Time) bool { p := byDay[t]; return p != nil && p.Door >= 15*time.Minute }
	end := day(Now)
	if !practiced(end) {
		end = end.AddDate(0, 0, -1)
	}
	var load [3]float64
	for i, n := range []int{7, 30, 90} {
		for k := range n {
			if practiced(end.AddDate(0, 0, -k)) {
				load[i] += 1 / float64(n)
			}
		}
	}
	if load[2] > 0 {
		c.Load = &load
	}
	return c
}

func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// SparseBoots is a year with little play: eight boots months apart, the
// last two this month.
func SparseBoots() []meta.BootRow {
	var boots []meta.BootRow
	for i, b := range []struct {
		y       int
		m       time.Month
		d, mins int
	}{
		{2025, time.November, 2, 41}, {2026, time.January, 17, 112}, {2026, time.March, 8, 26},
		{2026, time.March, 9, 67}, {2026, time.June, 21, 190}, {2026, time.September, 27, 12},
		{2026, time.October, 3, 58}, {2026, time.October, 11, 83},
	} {
		row := meta.BootRow{
			Boot: i + 1, Seed: uint64(i+1) * 0x2545f4914f6cdd1d, Preset: "standard",
			Started: time.Date(b.y, b.m, b.d, 19, 30, 0, 0, time.UTC), Uptime: time.Duration(b.mins) * time.Minute,
			Cause: game.TimedOut, Biome: "Userland", Problem: "Teleportation",
		}
		if i%3 == 1 {
			row.Cleared, row.Cause, row.Biome, row.Problem, row.Banked = true, "", "", "", 640
		}
		boots = append(boots, row)
	}
	return boots
}

// DenseBoots is a year of steady play ending at Now: a boot or two on most
// days, none over a winter break, and few in July.
func DenseBoots() []meta.BootRow {
	var boots []meta.BootRow
	causes := []game.Outcome{"", game.TimedOut, game.OutOfLives, game.TimedOut, game.GaveUp}
	breakFrom, breakTo := time.Date(2025, 12, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	start := meta.CalendarStart(Now)
	for i := 0; ; i++ {
		d := start.AddDate(0, 0, i)
		if d.After(Now) {
			break
		}
		// A fixed hash of the day (splitmix64's mixer): the same year on
		// every run, with no pattern to the eye.
		z := uint64(i+1) * 0x9e3779b97f4a7c15
		z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
		h := uint32((z ^ z>>27) >> 32)
		if h%100 >= 75 || d.Month() == time.July && h%100 >= 25 || d.After(breakFrom) && d.Before(breakTo) {
			continue
		}
		for k := range 1 + int(h>>9)%2 {
			b := meta.BootRow{
				Seed: uint64(h)<<4 | uint64(k), Preset: "standard", Prestige: min(i/120, 2),
				Started: d.Add(time.Duration(17+2*k)*time.Hour + time.Duration((h>>20)%60)*time.Minute),
				Uptime:  mins(12+int(h>>(11+3*k))%150, int(h>>3)%60),
				Cause:   causes[(i+k)%len(causes)], Biome: "The Heap", Problem: "Angry Cows",
			}
			if b.Cause == "" {
				b.Cleared, b.Biome, b.Problem, b.Banked = true, "", "", 700+int64(h%300)
			}
			boots = append(boots, b)
		}
	}
	for i := range boots {
		boots[i].Boot = i + 1
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
			{Topic: "dp", Attempts: 22, Clears: 14, Rating: 1612, Time: 9*time.Hour + 41*time.Minute + 20*time.Second},
			{Topic: "graphs", Attempts: 19, Clears: 8, Rating: 1430, Time: 7*time.Hour + 58*time.Minute + 5*time.Second},
			{Topic: "sorting-greedy", Attempts: 31, Clears: 24, Rating: 1515, Time: 8*time.Hour + 12*time.Minute + 44*time.Second},
			{Topic: "prefix-sums", Attempts: 14, Clears: 12, Rating: 1550, Time: 3*time.Hour + 31*time.Minute + 10*time.Second},
			{Topic: "binary-search", Attempts: 12, Clears: 7, Rating: 1390, Time: 4*time.Hour + 5*time.Minute + 33*time.Second},
			{Topic: "simulation", Attempts: 26, Clears: 23, Rating: 1350, Time: 5*time.Hour + 52*time.Minute + 18*time.Second},
			{Topic: "complete-search", Attempts: 18, Clears: 15, Rating: 1320, Time: 4*time.Hour + 40*time.Minute + 2*time.Second},
			{Topic: "two-pointers", Attempts: 7, Clears: 5, Rating: 1405, Time: time.Hour + 48*time.Minute + 30*time.Second},
			{Topic: "trees", Attempts: 9, Clears: 3, Rating: 1280, Time: 3*time.Hour + 20*time.Minute + 47*time.Second},
			{Topic: "shortest-paths", Attempts: 6, Clears: 2, Rating: 1240, Time: 2*time.Hour + 31*time.Minute + 9*time.Second},
			{Topic: "dsu", Attempts: 4, Clears: 3, Rating: 1460, Time: time.Hour + 2*time.Minute + 15*time.Second},
			{Topic: "math", Attempts: 11, Clears: 6, Rating: 1300, Time: 3*time.Hour + 9*time.Minute + 51*time.Second},
			{Topic: "strings", Attempts: 3, Clears: 1, Rating: 1150, Time: 47*time.Minute + 12*time.Second},
			{Topic: "data-structures", Attempts: 5, Clears: 2, Rating: 1260, Time: time.Hour + 55*time.Minute + 40*time.Second},
			{Topic: "geometry", Rating: 1000},
			{Topic: "mixed", Attempts: 8, Clears: 5, Rating: 1380, Time: 2*time.Hour + 44*time.Minute + 8*time.Second},
		},
		Solves: map[game.SolveKind]int{
			game.Independent: 88, game.Hinted: 30, game.Editorial: 12,
		},
		Boots:      42,
		FullClears: 9,
		Crashes: map[game.Outcome]int{
			game.TimedOut: 18, game.OutOfLives: 10, game.GaveUp: 4,
		},
		// TIME+ is the topics' times added up: where the hours went.
		TimePlus:      61*time.Hour + 20*time.Minute + 44*time.Second,
		LifetimeBytes: 12698,
		Bank:          1331,
		// The last 20 boots: the engine forecast 18% early crashes, 3 did.
		EarlyCrash: meta.EarlyCrash{Boots: 20, Crashed: 3, Forecast: 0.18},
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

// Packages is apt's shelf for Meta, cheapest first: two themes installed
// (cowsay in use), the next one 1331 B covers, one short of the bank, one
// waiting on a requirement, and a capstone.
func Packages() []meta.Package {
	return []meta.Package{
		{Name: "cowsay", Price: 64, Grants: "GRUB banner theme cowsay", Theme: "cowsay", Installed: true},
		{Name: "tux", Price: 128, Grants: "GRUB banner theme tux", Theme: "tux", Installed: true},
		{Name: "rng-tools", Price: 256, Grants: "daemon /dev/random joins the merge windows: Mystery doors pay 2x bytes."},
		{Name: "strace", Price: 400, Grants: "daemon strace joins the merge windows: A failed submit shows the failing test's input."},
		{Name: "procps", Price: 400, Grants: "+1 daemon on offer per merge window", Requires: []string{"rng-tools", "strace"}},
		{Name: "binutils", Price: 4096, Grants: "+1 lib link slot"},
	}
}

// ShopConfig is the shop for Meta.
func ShopConfig() meta.ShopConfig {
	m := Meta()
	return meta.ShopConfig{Packages: Packages(), Bank: m.Bank, Theme: "cowsay"}
}

// SetuidConfig is the setuid bits' screen for Meta, as the achievements
// doc sketches it (docs/design/achievements.md (g)): bits set over boots
// #38 to #41, the first by the backfill, two bosses down, nine map nodes,
// four capabilities, and the Trial's trail begun (.two-miles-high). No
// three-act boot is cleared yet, so the percentage stays hidden; ShowsPercent
// adds one.
func SetuidConfig() meta.SetuidConfig {
	m := Meta()
	m.Setuid = []game.SetuidBit{
		{ID: "hello-world", At: at(2, 21, 7), Backfill: true},
		{ID: "exit-0", At: at(13, 23, 40), Boot: 38},
		{ID: "two-twins", At: at(14, 1, 12), Boot: 39},
		{ID: "there-goes-bill", At: at(14, 1, 55), Boot: 39},
		{ID: "humpty", At: at(14, 2, 2)},
		{ID: "finds-none", At: at(14, 2, 30), Boot: 40},
		{ID: ".two-miles-high", At: at(14, 2, 31), Boot: 41},
		{ID: "ditto-ditto", At: at(14, 3, 2), Boot: 41},
	}
	caps := slices.Sorted(maps.Values(game.TopicCapabilities))
	m.Capabilities = caps[:4]
	m.Libs = []game.Lib{{Name: "dsu"}, {Name: "fenwick"}}
	m.Record = game.Record{
		Folded:  41,
		Bosses:  []string{"off-by-one", "oom-killer"},
		Nodes:   slices.Clone(setuid.Nodes[:9]),
		Daemons: []game.Daemon{"strace", "nice"},
	}
	return meta.SetuidConfig{Meta: m, Now: Now, Notices: true}
}

// ShowsPercent is c past its first three-act clear (Record.Cleared), when
// the screen shows the completion percentage and its breakdown.
func ShowsPercent(c meta.SetuidConfig) meta.SetuidConfig {
	c.Meta.Record.Cleared = []int{1}
	c.Meta.Record.Unlocked = []int{1, 2}
	c.Meta.Setuid = append(slices.Clone(c.Meta.Setuid),
		game.SetuidBit{ID: "snicker-snack", At: at(14, 4, 10), Boot: 41})
	return c
}
