package setuid

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/units"
)

// Table is every bit, in the order Check sets them when several hold at
// once (achievements.md §(d)): progression, skill, secret, completion. A
// number its text states is read off the rules it names. B below is the
// boot checked (holds.go).
var Table = table()

// Verdicts are the full-test verdicts a boot's doors can hear, as
// .consider-your-verdict lists them.
var Verdicts = []game.Verdict{"AC", "WA", "TLE", "MLE", "RE", "CE", "OLE"}

func table() []Def {
	std, b := rules.Standard(), rules.Setuid()
	top := game.MaxPrestige
	daemons := len(game.AllDaemons)
	caps := slices.Sorted(maps.Values(game.TopicCapabilities))
	verdicts := make([]string, len(Verdicts))
	for i, v := range Verdicts {
		verdicts[i] = string(v)
	}
	hours := func(d time.Duration) string { return fmt.Sprintf("%.0f h", d.Hours()) }

	// beat is "defeat boss k", on any built-in boot (a linear one too).
	beat := func(k string) func(*view) bool { return func(v *view) bool { return v.beat(k) } }
	// boss is "defeat boss k, its door as ok says".
	boss := func(k string, ok func(v *view, i int, a game.Attempt) bool) func(*view) bool {
		return func(v *view) bool {
			a, i, found := v.door(k)
			return found && ok(v, i, a)
		}
	}
	// twist is boss for a door that bore the boss's twist: the skill bits of
	// S8-S13 are each the twist mastered, so a boot saved before twists
	// sets none.
	twist := func(k string, ok func(v *view, i int, a game.Attempt) bool) func(*view) bool {
		return boss(k, func(v *view, i int, a game.Attempt) bool { return a.Door.Twist == k && ok(v, i, a) })
	}
	// unlocked is "a clear of B unlocked prestige level"; its counter is
	// the prestige gate's rated doors while level is next.
	unlocked := func(level int) func(*view) bool {
		return func(v *view) bool { return v.s != nil && v.s.Unlocked == level }
	}
	ring := func(level int) func(m *game.Meta) (int, int) {
		return func(m *game.Meta) (int, int) {
			if m.Prestige+1 != level {
				return 0, 0
			}
			g := game.NextPrestige(std, m)
			return g.Rated, g.Need
		}
	}
	// full is "clear a three-act boot", and ok of it.
	full := func(ok func(v *view) bool) func(*view) bool {
		return func(v *view) bool { return v.full() && ok(v) }
	}
	pct := func(at int) func(*view) bool {
		return func(v *view) bool { got, _ := Completion(v.m); return got >= at }
	}

	return []Def{
		// ---- progression · /bin ---------------------------------------------
		{ID: "hello-world", Kind: KindProgression, Name: "hello, world", Text: "Clear a door.",
			holds: func(v *view) bool {
				return v.s != nil && slices.ContainsFunc(v.s.History, func(a game.Attempt) bool { return a.Outcome == game.Cleared })
			}},
		{ID: "exit-0", Kind: KindProgression, Name: "exit 0", Text: "Clear a boot.",
			holds: (*view).clear},
		{ID: "two-twins", Kind: KindProgression, Name: "Two Twins, One Gap", Text: "Defeat Off-by-One.",
			holds: beat("off-by-one")},
		{ID: "there-goes-bill", Kind: KindProgression, Name: "There Goes Bill", Text: "Defeat the OOM Killer.",
			holds: beat("oom-killer")},
		{ID: "shelf-stays-full", Kind: KindProgression, Name: "The Shelf Stays Full", Text: "Defeat Heisenbug.",
			holds: beat("heisenbug")},
		{ID: "caucus-race", Kind: KindProgression, Name: "The Caucus-Race", Text: "Defeat Race Condition.",
			holds: beat("race-condition")},
		{ID: "handsome-pig", Kind: KindProgression, Name: "A Handsome Pig", Text: "Defeat Undefined Behavior. It opens from prestige 1.",
			holds: beat("undefined-behavior")},
		{ID: "snicker-snack", Kind: KindProgression, Name: "Snicker-Snack", Text: "Defeat The Halting Problem: clear a three-act boot.",
			holds: (*view).full},
		{ID: "through-the-glass", Kind: KindProgression, Name: "Through the Glass",
			Text:  fmt.Sprintf("Unlock Silver: clear at Bronze once %d doors are rated.", std.CalibAfter),
			holds: unlocked(1), progress: ring(1)},
		{ID: "nameless-wood", Kind: KindProgression, Name: "The Nameless Wood", Text: "Unlock Gold.",
			holds: unlocked(2), progress: ring(2)},
		{ID: "eighth-square", Kind: KindProgression, Name: "Eighth Square", Text: "Unlock Platinum: ring 0.",
			holds: unlocked(3), progress: ring(3)},
		{ID: "red-kings-dream", Kind: KindProgression, Name: "The Red King's Dream", Text: "Unlock Advanced: ring -1.",
			holds: unlocked(4), progress: ring(4)},
		{ID: "sunny-at-midnight", Kind: KindProgression, Name: "Sunny at Midnight", Text: "Unlock prestige 5: ring -2.",
			holds: unlocked(5), progress: ring(5)},
		{ID: "where-the-fall-began", Kind: KindProgression, Name: "Where the Fall Began", Text: "Unlock prestige 7: ring -3.",
			holds: unlocked(7), progress: ring(7)},
		{ID: "you-halted", Kind: KindProgression, Name: "You Halted", Text: fmt.Sprintf("Clear a three-act boot at prestige %d.", top),
			holds: full(func(v *view) bool { return v.s.Prestige == top })},
		{ID: "stamped", Kind: KindProgression, Name: "Stamped by the Gryphon", Text: "Pass an RTFM trial that grants a capability.",
			current: true, holds: func(v *view) bool { return len(v.m.Capabilities) > 0 }},
		{ID: "lobster-quadrille", Kind: KindProgression, Name: "The Lobster Quadrille",
			Text:    "Pass a division's conclusion trial in RTFM.",
			current: true, rtfm: true, holds: func(v *view) bool { return slices.ContainsFunc(v.m.TrialsPassed, curriculum.Conclusion) }},
		{ID: "best-of-educations", Kind: KindProgression, Name: "The Best of Educations",
			Text:    "Pass every trial of one RTFM division.",
			current: true, rtfm: true, holds: (*view).schooled},
		{ID: "another-road", Kind: KindProgression, Name: "Another Road Opens",
			Text: "Clear a room behind a locked exit (a capability, or `apt git`).",
			holds: func(v *view) bool {
				return v.mapped() && slices.ContainsFunc(v.s.History, func(a game.Attempt) bool {
					n, ok := v.node(a)
					return a.Outcome == game.Cleared && ok && game.CapabilityFor(n.Topic) != ""
				})
			}},
		{ID: "grin-returns", Kind: KindProgression, Name: "The Grin Comes Back", Text: "Recover a core dump by upsolving its problem.",
			current: true, holds: func(v *view) bool { return v.m.Record.Recovered > 0 }},
		{ID: "struck-off", Kind: KindProgression, Name: "Struck Off in Ink", Text: "Retire a regression: re-solve it at every stage.",
			current: true, holds: func(v *view) bool { return v.m.Retired > 0 }},
		{ID: "humpty", Kind: KindProgression, Name: "Humpty's First Word", Text: "Save a lib (a `// lib:` block in a cleared door).",
			current: true, holds: func(v *view) bool { return len(v.m.Libs) > 0 }},
		{ID: "one-topic", Kind: KindProgression, Name: "One Topic Knows You",
			Text:    fmt.Sprintf("Master a topic: %d rated doors on it.", std.MasteryAttempts),
			current: true, holds: func(v *view) bool { return mastered(v.m) > 0 }},
		{ID: "by-railway", Kind: KindProgression, Name: "By Railway", Text: "Clear a boot that took act III's skip.",
			holds: full(func(v *view) bool { return v.took(func(n rules.Biome) bool { return n.Skip }) })},
		{ID: "un-birthday", Kind: KindProgression, Name: "An Un-Birthday Present", Text: "Clear a nightly build, any date.",
			holds: full(func(v *view) bool { return v.s.Nightly() })},

		// ---- skill · /sbin ---------------------------------------------------
		{ID: "speedrun-1", Kind: KindSkill, Name: "Late, but Inside the Limit",
			Text:  fmt.Sprintf("Clear a three-act boot in under %s of door time.", units.Clock(b.Speedrun1)),
			holds: full(func(v *view) bool { return v.s.BootTime(v.now) < b.Speedrun1 })},
		{ID: "speedrun-2", Kind: KindSkill, Name: "Faster Than the Rabbit",
			Text:  fmt.Sprintf("Clear a three-act boot in under %s of door time.", units.Clock(b.Speedrun2)),
			holds: full(func(v *view) bool { return v.s.BootTime(v.now) < b.Speedrun2 })},
		{ID: "all-o3", Kind: KindSkill, Name: "Twice as Fast as That!",
			Text: "Clear a three-act boot with every door -O3. Retreats are allowed; a lost room is not.",
			holds: full(func(v *view) bool {
				return v.every(func(a game.Attempt) bool { return a.Outcome == game.Cleared && a.O3 || a.Outcome == game.Retreated })
			})},
		{ID: "finds-none", Kind: KindSkill, Name: "Finds None. Sulks.", Text: "Clear a three-act boot with no failed hidden-test submit.",
			holds: full((*view).clean)},
		{ID: "catless", Kind: KindSkill, Name: "Not Even the Grin", Text: "Clear a three-act boot without a duck.",
			holds: full(func(v *view) bool { return v.ducks() == 0 })},
		{ID: "essence", Kind: KindSkill, Name: "Time Is of the Essence",
			Text: "Clear a three-act boot without buying time: no Overclock, ECC, nice, at or retreat (kexec swaps count as retreats).",
			holds: full(func(v *view) bool {
				s := v.s
				return !slices.Contains(s.Upgrades, game.Overclock) && !slices.Contains(s.Upgrades, game.ECC) &&
					!slices.Contains(s.Daemons, game.Nice) &&
					v.every(func(a game.Attempt) bool { return a.At == 0 && a.Outcome != game.Retreated })
			})},
		{ID: "doesnt-blink", Kind: KindSkill, Name: "She Doesn't Blink",
			Text:  fmt.Sprintf("Clear at prestige %d with no failed hidden-test submit and no duck.", top),
			holds: full(func(v *view) bool { return v.s.Prestige == top && v.clean() && v.ducks() == 0 })},
		{ID: "ditto-ditto", Kind: KindSkill, Name: "Ditto, Ditto", Text: "Defeat Off-by-One with no failed hidden-test submit on its door.",
			holds: twist("off-by-one", func(v *view, _ int, a game.Attempt) bool { return burns(&v.s.Rules, a) == 0 })},
		{ID: "rss-modest", Kind: KindSkill, Name: "RSS: Modest", Text: "Defeat the OOM Killer on your first :Submit.",
			holds: twist("oom-killer", func(_ *view, _ int, a game.Attempt) bool { return a.Submits == 1 })},
		{ID: "stopped-looking", Kind: KindSkill, Name: "You Stopped Looking Hard",
			Text: "Defeat Heisenbug without running the samples (`:w`) on its door.",
			holds: twist("heisenbug", func(v *view, i int, a game.Attempt) bool {
				n, ok := v.samples(i, a)
				return ok && n == 0
			})},
		{ID: "every-thread", Kind: KindSkill, Name: "Every Thread Joined", Text: "Defeat Race Condition with -O3, under half its fast clock.",
			holds: twist("race-condition", func(_ *view, _ int, a game.Attempt) bool { return a.O3 })},
		{ID: "bounds-checked", Kind: KindSkill, Name: "Bounds-Checked the Soup", Text: "Defeat Undefined Behavior with no failed hidden-test submit.",
			holds: twist("undefined-behavior", func(v *view, _ int, a game.Attempt) bool { return burns(&v.s.Rules, a) == 0 })},
		{ID: "decided-this-one", Kind: KindSkill, Name: "You Decided This One",
			Text:  "Defeat The Halting Problem with -O3, under half its hidden clock.",
			holds: twist(game.FinalBoss, func(_ *view, _ int, a game.Attempt) bool { return a.O3 })},
		{ID: "independent", Kind: KindSkill, Name: "The Cat Kept a Copy",
			Text:    "Recover a core dump with an upsolve made without the editorial or a duck.",
			current: true, holds: func(v *view) bool { return v.m.Record.RecoveredFree > 0 }},
		{ID: "speed-completion", Kind: KindSkill, Name: "Speed Completion",
			Text:    fmt.Sprintf("Reach %d%% with under %s of boot time (TIME+ less upsolves and trials).", Full, hours(b.SpeedCompletion)),
			current: true, holds: func(v *view) bool { return v.speed("hundred", Full, b.SpeedCompletion) }},
		{ID: "speed-completion-112", Kind: KindSkill, Name: "Pure Speed",
			Text:    fmt.Sprintf("Reach %d%% with under %s of boot time.", Pure, hours(b.SpeedCompletion112)),
			current: true, holds: func(v *view) bool { return v.speed("hundred-twelve", Pure, b.SpeedCompletion112) }},

		// ---- secret · /home/you ----------------------------------------------
		{ID: ".dull-reality", Kind: KindSecret, Name: "Dull Reality", Hint: "Who Stole the Tarts?", Text: "Win the Court and wake on the bank.",
			holds: func(v *view) bool {
				return v.mapped() && v.s.Phase == game.PhaseCleared && v.s.Ending == game.EndingWake
			}},
		{ID: ".two-miles-high", Kind: KindSecret, Name: "Nearly Two Miles High", Hint: "EAT ME",
			Text:  fmt.Sprintf("Reach the cycle cap (%s) in a boot.", units.Hearts(rules.Map().MaxCycles, rules.Map().MaxCycles)),
			holds: (*view).capped},
		{ID: ".stuff-and-nonsense", Kind: KindSecret, Name: "Stuff and Nonsense!", Hint: "The court sits in the garden.", Text: "Find the Court.",
			holds: func(v *view) bool {
				return v.mapped() && slices.ContainsFunc(v.s.Biomes, func(b game.BiomeState) bool { return b.Node == rules.TrialNode })
			}},
		{ID: ".then-it-ends", Kind: KindSecret, Name: "Then It Ends", Hint: "The Rabbit cries twice.",
			Text:  "Defeat The Halting Problem in its last minute.",
			holds: boss(game.FinalBoss, func(_ *view, _ int, a game.Attempt) bool { return lastMinute(a, b.LastMinute) })},
		{ID: ".galumphing-back", Kind: KindSecret, Name: "Galumphing Back", Hint: "The Rose knew a shortcut.",
			Text: "Walk away from the Jabberwock, then slay it.",
			holds: boss(game.FinalBoss, func(v *view, i int, _ game.Attempt) bool {
				return v.mapped() && slices.ContainsFunc(v.s.History[:i], func(a game.Attempt) bool {
					return a.Door.Boss == game.FinalBoss && a.Outcome == game.Retreated
				})
			})},
		{ID: ".consider-your-verdict", Kind: KindSecret, Name: "Consider Your Verdict", Hint: "The jury runs out of slate.",
			Text:  "Hear every verdict on boot doors: " + strings.Join(verdicts, ", ") + ".",
			holds: func(v *view) bool { return have(v.m.Record.Verdicts, Verdicts) == len(Verdicts) }},
		{ID: ".every-seat", Kind: KindSecret, Name: "Every Seat Taken", Hint: "Three seats taken.", Text: "Run every daemon seat a boot has.",
			holds: func(v *view) bool {
				return v.s != nil && v.s.Rules.MaxDaemons > 0 && len(v.s.Daemons) >= v.s.Rules.MaxDaemons
			}},
		{ID: ".counted-again", Kind: KindSecret, Name: "Counted Again", Hint: "Count them. Count again.",
			Text: "Defeat Off-by-One after its double burn.",
			holds: boss("off-by-one", func(_ *view, _ int, a game.Attempt) bool {
				return a.Door.Twist == "off-by-one" && slices.ContainsFunc(a.Events, func(e game.AttemptEvent) bool {
					return e.Kind == "submit" && e.Verdict == "WA" && !e.SampleFailed && e.Total > 0 && e.Passed == e.Total-1
				})
			})},
		{ID: ".size-of-a-door", Kind: KindSecret, Name: "Exactly the Size of a Door", Hint: "Under the doormat in the hall.",
			Text:    "Read the note to its last fold.",
			current: true, holds: func(v *view) bool { return game.HasSeen(v.m, "milestone.doors.500") }},
		{ID: ".which-dreamed-it", Kind: KindSecret, Name: "Which Do You Think It Was?", Hint: "Two endings. One dreamer.",
			Text:    "See both endings.",
			current: true, holds: func(v *view) bool { return IsSet(v.m, "snicker-snack") && IsSet(v.m, ".dull-reality") }},

		// ---- completion · /usr/bin -------------------------------------------
		{ID: "every-bug", Kind: KindCompletion, Name: "Every Bug in the Pack", Text: "Defeat all six bosses.",
			holds:    func(v *view) bool { return have(v.m.Record.Bosses, Bosses) == len(Bosses) },
			progress: func(m *game.Meta) (int, int) { return have(m.Record.Bosses, Bosses), len(Bosses) }},
		{ID: "cartographer", Kind: KindCompletion, Name: "Where You Want to Get To",
			Text:     fmt.Sprintf("Clear a room in every map node: %d.", len(Nodes)),
			holds:    func(v *view) bool { return have(v.m.Record.Nodes, Nodes) == len(Nodes) },
			progress: func(m *game.Meta) (int, int) { return have(m.Record.Nodes, Nodes), len(Nodes) }},
		{ID: "regular-course", Kind: KindCompletion, Name: "The Regular Course",
			Text: fmt.Sprintf("Hold all %d capabilities.", len(caps)), current: true,
			holds:    func(v *view) bool { return have(v.m.Capabilities, caps) == len(caps) },
			progress: func(m *game.Meta) (int, int) { return have(m.Capabilities, caps), len(caps) }},
		{ID: "apt-full", Kind: KindCompletion, Name: "apt list --installed",
			Text: fmt.Sprintf("Install every package: %d.", len(std.Packages)), current: true,
			holds:    func(v *view) bool { return have(v.m.Installed, packages(std)) == len(std.Packages) },
			progress: func(m *game.Meta) (int, int) { return have(m.Installed, packages(std)), len(std.Packages) }},
		{ID: "own-invention", Kind: KindCompletion, Name: "It's My Own Invention",
			Text:     fmt.Sprintf("Clear a boot with each daemon running: %d.", daemons),
			holds:    func(v *view) bool { return have(v.m.Record.Daemons, game.AllDaemons) == daemons },
			progress: func(m *game.Meta) (int, int) { return have(m.Record.Daemons, game.AllDaemons), daemons }},
		{ID: "her-list", Kind: KindCompletion, Name: "Her List, in Ink", Text: fmt.Sprintf("Retire %d regressions.", b.Retired), current: true,
			holds:    func(v *view) bool { return v.m.Retired >= b.Retired },
			progress: func(m *game.Meta) (int, int) { return m.Retired, b.Retired }},
		{ID: "ledger", Kind: KindCompletion, Name: "The Cat's Ledger Balances", Text: fmt.Sprintf("Recover %d core dumps.", b.Recovered), current: true,
			holds:    func(v *view) bool { return v.m.Record.Recovered >= b.Recovered },
			progress: func(m *game.Meta) (int, int) { return m.Record.Recovered, b.Recovered }},
		{ID: "not-in-general", Kind: KindCompletion, Name: "Not in General", Text: fmt.Sprintf("Rate %d doors.", b.Rated), current: true,
			holds:    func(v *view) bool { return game.RatedDoors(v.m) >= b.Rated },
			progress: func(m *game.Meta) (int, int) { return game.RatedDoors(m), b.Rated }},
		{ID: "dictionary", Kind: KindCompletion, Name: "Humpty's Dictionary", Text: fmt.Sprintf("Save %d libs.", b.Libs), current: true,
			holds:    func(v *view) bool { return len(v.m.Libs) >= b.Libs },
			progress: func(m *game.Meta) (int, int) { return len(m.Libs), b.Libs }},
		{ID: "every-topic", Kind: KindCompletion, Name: "Every Topic Knows You",
			Text: fmt.Sprintf("Master every topic but mixed: %d.", len(Topics)), current: true,
			holds:    func(v *view) bool { return mastered(v.m) == len(Topics) },
			progress: func(m *game.Meta) (int, int) { return mastered(m), len(Topics) }},
		{ID: "hundred", Kind: KindCompletion, Name: "100%", Text: fmt.Sprintf("Reach %d%%.", Full), current: true,
			holds:    pct(Full),
			progress: func(m *game.Meta) (int, int) { got, _ := Completion(m); return got, Full }},
		{ID: "hundred-twelve", Kind: KindCompletion, Name: "112%", Text: fmt.Sprintf("Reach %d%%.", Pure), current: true,
			holds:    pct(Pure),
			progress: func(m *game.Meta) (int, int) { got, _ := Completion(m); return got, Pure }},
	}
}

// have counts the items of of that are in got.
func have[T comparable](got, of []T) int {
	n := 0
	for _, x := range of {
		if slices.Contains(got, x) {
			n++
		}
	}
	return n
}

// packages are r's package names.
func packages(r rules.Rules) []string {
	names := make([]string, len(r.Packages))
	for i, p := range r.Packages {
		names[i] = p.Name
	}
	return names
}
