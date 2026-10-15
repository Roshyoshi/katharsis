package setuid

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/units"
)

// Table is every bit, in the order Check sets them when several hold at
// once (achievements.md §(d)): progression, skill, secret, completion. A
// number its text states is read off the rules it names.
var Table = table()

// Verdicts are the full-test verdicts a boot's doors can hear, as
// .consider-your-verdict lists them.
var Verdicts = []game.Verdict{"AC", "WA", "TLE", "MLE", "RE", "CE", "OLE"}

func table() []Def {
	std, b := rules.Standard(), rules.Setuid()
	daemons := len(game.AllDaemons)
	caps := len(game.TopicCapabilities)
	verdicts := make([]string, len(Verdicts))
	for i, v := range Verdicts {
		verdicts[i] = string(v)
	}
	hours := func(d time.Duration) string { return fmt.Sprintf("%.0f h", d.Hours()) }
	ring := func(level int) func(m *game.Meta) (int, int) {
		return func(m *game.Meta) (int, int) {
			if m.Prestige+1 != level {
				return 0, 0
			}
			g := game.NextPrestige(std, m)
			return g.Rated, g.Need
		}
	}
	return []Def{
		// ---- progression · /bin ---------------------------------------------
		{ID: "hello-world", Kind: KindProgression, Name: "hello, world", Text: "Clear a door."},
		{ID: "exit-0", Kind: KindProgression, Name: "exit 0", Text: "Clear a boot."},
		{ID: "two-twins", Kind: KindProgression, Name: "Two Twins, One Gap", Text: "Defeat Off-by-One."},
		{ID: "there-goes-bill", Kind: KindProgression, Name: "There Goes Bill", Text: "Defeat the OOM Killer."},
		{ID: "shelf-stays-full", Kind: KindProgression, Name: "The Shelf Stays Full", Text: "Defeat Heisenbug."},
		{ID: "caucus-race", Kind: KindProgression, Name: "The Caucus-Race", Text: "Defeat Race Condition."},
		{ID: "handsome-pig", Kind: KindProgression, Name: "A Handsome Pig", Text: "Defeat Undefined Behavior. It opens from prestige 1."},
		{ID: "snicker-snack", Kind: KindProgression, Name: "Snicker-Snack", Text: "Defeat The Halting Problem: clear a three-act boot."},
		{ID: "through-the-glass", Kind: KindProgression, Name: "Through the Glass",
			Text: fmt.Sprintf("Unlock Silver: clear at Bronze once %d doors are rated.", std.CalibAfter), progress: ring(1)},
		{ID: "nameless-wood", Kind: KindProgression, Name: "The Nameless Wood", Text: "Unlock Gold.", progress: ring(2)},
		{ID: "eighth-square", Kind: KindProgression, Name: "Eighth Square", Text: "Unlock Platinum: ring 0.", progress: ring(3)},
		{ID: "red-kings-dream", Kind: KindProgression, Name: "The Red King's Dream", Text: "Unlock Advanced: ring -1.", progress: ring(4)},
		{ID: "sunny-at-midnight", Kind: KindProgression, Name: "Sunny at Midnight", Text: "Unlock prestige 5: ring -2.", progress: ring(5)},
		{ID: "where-the-fall-began", Kind: KindProgression, Name: "Where the Fall Began", Text: "Unlock prestige 7: ring -3.", progress: ring(7)},
		{ID: "you-halted", Kind: KindProgression, Name: "You Halted", Text: fmt.Sprintf("Clear a three-act boot at prestige %d.", game.MaxPrestige)},
		{ID: "stamped", Kind: KindProgression, Name: "Stamped by the Gryphon", Text: "Pass an RTFM trial that grants a capability.", current: true},
		{ID: "another-road", Kind: KindProgression, Name: "Another Road Opens", Text: "Clear a room behind a locked exit (a capability, or `apt git`)."},
		{ID: "grin-returns", Kind: KindProgression, Name: "The Grin Comes Back", Text: "Recover a core dump by upsolving its problem.", current: true},
		{ID: "struck-off", Kind: KindProgression, Name: "Struck Off in Ink", Text: "Retire a regression: re-solve it at every stage.", current: true},
		{ID: "humpty", Kind: KindProgression, Name: "Humpty's First Word", Text: "Save a lib (a `// lib:` block in a cleared door).", current: true},
		{ID: "one-topic", Kind: KindProgression, Name: "One Topic Knows You",
			Text: fmt.Sprintf("Master a topic: %d rated doors on it.", std.MasteryAttempts), current: true},
		{ID: "by-railway", Kind: KindProgression, Name: "By Railway", Text: "Clear a boot that took act III's skip."},
		{ID: "un-birthday", Kind: KindProgression, Name: "An Un-Birthday Present", Text: "Clear a nightly build, any date."},

		// ---- skill · /sbin ---------------------------------------------------
		{ID: "speedrun-1", Kind: KindSkill, Name: "Late, but Inside the Limit",
			Text: fmt.Sprintf("Clear a three-act boot in under %s of door time.", units.Clock(b.Speedrun1))},
		{ID: "speedrun-2", Kind: KindSkill, Name: "Faster Than the Rabbit",
			Text: fmt.Sprintf("Clear a three-act boot in under %s of door time.", units.Clock(b.Speedrun2))},
		{ID: "all-o3", Kind: KindSkill, Name: "Twice as Fast as That!",
			Text: "Clear a three-act boot with every door -O3. Retreats are allowed; a lost room is not."},
		{ID: "finds-none", Kind: KindSkill, Name: "Finds None. Sulks.", Text: "Clear a three-act boot with no failed hidden-test submit."},
		{ID: "catless", Kind: KindSkill, Name: "Not Even the Grin", Text: "Clear a three-act boot without a duck."},
		{ID: "essence", Kind: KindSkill, Name: "Time Is of the Essence",
			Text: "Clear a three-act boot without buying time: no Overclock, ECC, nice, at or retreat (kexec swaps count as retreats)."},
		{ID: "doesnt-blink", Kind: KindSkill, Name: "She Doesn't Blink",
			Text: fmt.Sprintf("Clear at prestige %d with no failed hidden-test submit and no duck.", game.MaxPrestige)},
		{ID: "ditto-ditto", Kind: KindSkill, Name: "Ditto, Ditto", Text: "Defeat Off-by-One with no failed hidden-test submit on its door."},
		{ID: "rss-modest", Kind: KindSkill, Name: "RSS: Modest", Text: "Defeat the OOM Killer on your first :Submit."},
		{ID: "stopped-looking", Kind: KindSkill, Name: "You Stopped Looking Hard", Text: "Defeat Heisenbug without running the samples (`:w`) on its door."},
		{ID: "every-thread", Kind: KindSkill, Name: "Every Thread Joined", Text: "Defeat Race Condition with -O3, under half its fast clock."},
		{ID: "bounds-checked", Kind: KindSkill, Name: "Bounds-Checked the Soup", Text: "Defeat Undefined Behavior with no failed hidden-test submit."},
		{ID: "decided-this-one", Kind: KindSkill, Name: "You Decided This One", Text: "Defeat The Halting Problem with -O3, under half its hidden clock."},
		{ID: "independent", Kind: KindSkill, Name: "The Cat Kept a Copy",
			Text: "Recover a core dump with an upsolve made without the editorial or a duck.", current: true},
		{ID: "speed-completion", Kind: KindSkill, Name: "Speed Completion",
			Text:    fmt.Sprintf("Reach %d%% with under %s of boot time (TIME+ less upsolves and trials).", Full, hours(b.SpeedCompletion)),
			current: true},
		{ID: "speed-completion-112", Kind: KindSkill, Name: "Pure Speed",
			Text: fmt.Sprintf("Reach %d%% with under %s of boot time.", Pure, hours(b.SpeedCompletion112)), current: true},

		// ---- secret · /home/you ----------------------------------------------
		{ID: ".dull-reality", Kind: KindSecret, Name: "Dull Reality", Hint: "Who Stole the Tarts?", Text: "Win the Trial and wake on the bank."},
		{ID: ".two-miles-high", Kind: KindSecret, Name: "Nearly Two Miles High", Hint: "EAT ME",
			Text: fmt.Sprintf("Reach the cycle cap (%s) in a boot.", units.Hearts(rules.Map().MaxCycles, rules.Map().MaxCycles))},
		{ID: ".stuff-and-nonsense", Kind: KindSecret, Name: "Stuff and Nonsense!", Hint: "The court sits in the garden.", Text: "Find the Trial."},
		{ID: ".then-it-ends", Kind: KindSecret, Name: "Then It Ends", Hint: "The Rabbit cries twice.", Text: "Defeat The Halting Problem in its last minute."},
		{ID: ".galumphing-back", Kind: KindSecret, Name: "Galumphing Back", Hint: "The Rose knew a shortcut.", Text: "Walk away from the Jabberwock, then slay it."},
		{ID: ".consider-your-verdict", Kind: KindSecret, Name: "Consider Your Verdict", Hint: "The jury runs out of slate.",
			Text: "Hear every verdict on boot doors: " + strings.Join(verdicts, ", ") + "."},
		{ID: ".every-seat", Kind: KindSecret, Name: "Every Seat Taken", Hint: "Three seats taken.", Text: "Run every daemon seat a boot has."},
		{ID: ".counted-again", Kind: KindSecret, Name: "Counted Again", Hint: "Count them. Count again.", Text: "Defeat Off-by-One after its double burn."},
		{ID: ".size-of-a-door", Kind: KindSecret, Name: "Exactly the Size of a Door", Hint: "Under the doormat in the hall.",
			Text: "Read the note to its last fold.", current: true},
		{ID: ".which-dreamed-it", Kind: KindSecret, Name: "Which Do You Think It Was?", Hint: "Two endings. One dreamer.",
			Text: "See both endings.", current: true},

		// ---- completion · /usr/bin -------------------------------------------
		{ID: "every-bug", Kind: KindCompletion, Name: "Every Bug in the Pack", Text: "Defeat all six bosses.",
			progress: func(m *game.Meta) (int, int) { return have(m.Record.Bosses, Bosses), len(Bosses) }},
		{ID: "cartographer", Kind: KindCompletion, Name: "Where You Want to Get To",
			Text:     fmt.Sprintf("Clear a room in every map node: %d.", len(Nodes)),
			progress: func(m *game.Meta) (int, int) { return have(m.Record.Nodes, Nodes), len(Nodes) }},
		{ID: "regular-course", Kind: KindCompletion, Name: "The Regular Course",
			Text: fmt.Sprintf("Hold all %d capabilities.", caps), current: true,
			progress: func(m *game.Meta) (int, int) {
				return have(m.Capabilities, slices.Collect(maps.Values(game.TopicCapabilities))), caps
			}},
		{ID: "apt-full", Kind: KindCompletion, Name: "apt list --installed",
			Text: fmt.Sprintf("Install every package: %d.", len(std.Packages)), current: true,
			progress: func(m *game.Meta) (int, int) { return have(m.Installed, packages(std)), len(std.Packages) }},
		{ID: "own-invention", Kind: KindCompletion, Name: "It's My Own Invention",
			Text: fmt.Sprintf("Clear a boot with each daemon running: %d.", daemons),
			progress: func(m *game.Meta) (int, int) {
				return have(m.Record.Daemons, game.AllDaemons), daemons
			}},
		{ID: "her-list", Kind: KindCompletion, Name: "Her List, in Ink", Text: fmt.Sprintf("Retire %d regressions.", b.Retired), current: true,
			progress: func(m *game.Meta) (int, int) { return m.Retired, b.Retired }},
		{ID: "ledger", Kind: KindCompletion, Name: "The Cat's Ledger Balances", Text: fmt.Sprintf("Recover %d core dumps.", b.Recovered), current: true,
			progress: func(m *game.Meta) (int, int) { return m.Record.Recovered, b.Recovered }},
		{ID: "not-in-general", Kind: KindCompletion, Name: "Not in General", Text: fmt.Sprintf("Rate %d doors.", b.Rated), current: true,
			progress: func(m *game.Meta) (int, int) { return game.RatedDoors(m), b.Rated }},
		{ID: "dictionary", Kind: KindCompletion, Name: "Humpty's Dictionary", Text: fmt.Sprintf("Save %d libs.", b.Libs), current: true,
			progress: func(m *game.Meta) (int, int) { return len(m.Libs), b.Libs }},
		{ID: "every-topic", Kind: KindCompletion, Name: "Every Topic Knows You",
			Text: fmt.Sprintf("Master every topic but mixed: %d.", len(Topics)), current: true,
			progress: func(m *game.Meta) (int, int) { return mastered(m), len(Topics) }},
		{ID: "hundred", Kind: KindCompletion, Name: "100%", Text: fmt.Sprintf("Reach %d%%.", Full), current: true,
			progress: func(m *game.Meta) (int, int) { pct, _ := Completion(m); return pct, Full }},
		{ID: "hundred-twelve", Kind: KindCompletion, Name: "112%", Text: fmt.Sprintf("Reach %d%%.", Pure), current: true,
			progress: func(m *game.Meta) (int, int) { pct, _ := Completion(m); return pct, Pure }},
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
