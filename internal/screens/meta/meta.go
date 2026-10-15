// Package meta holds the screens outside a boot: the GRUB menu (start
// screen), git log (boot history) and top (stats).
//
// The screens take view models defined here (BootRow, AttemptRow, Stats)
// rather than store records, so they can be built and tested before the
// store settles; the app maps its records onto these types. Display
// strings that need the catalog (titles) or flavor (biome names) are
// resolved by the app when it builds the rows.
package meta

import (
	"fmt"
	"strings"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/ui"
)

// BootRow is one boot in git log.
type BootRow struct {
	Boot     int
	Seed     uint64
	Preset   string // rules preset name
	Prestige int
	Started  time.Time // shown in its own location; hand over local times
	Uptime   time.Duration

	// Cleared is true when the boot beat its last boss. Otherwise Cause is
	// how it crashed (game.TimedOut, game.OutOfLives, game.GaveUp), or ""
	// when the boot ended without a crash (the app quit mid-boot).
	Cleared bool
	Cause   game.Outcome
	Biome   string // display name of the biome it crashed in
	Problem string // title of the problem it crashed on

	Banked int64 // bytes committed during the boot
	Dumped int64 // uncommitted bytes left in the core dump by the crash

	// Nightly marks a boot on its day's nightly seed, First the first boot
	// played on that seed (retention RT15); NewLog sets both.
	Nightly, First bool

	Attempts []AttemptRow // in the order they were opened
}

// AttemptRow is one door opened during a boot.
type AttemptRow struct {
	Problem catalog.ID
	Title   string
	Topic   string
	Rating  int

	Outcome game.Outcome
	Elapsed time.Duration // from opening the door to closing it
	Timer   time.Duration
	Submits int
	Failed  int
	Quacks  int
	Kind    game.SolveKind // how it was solved; "" unless Outcome is Cleared

	Boss    string // display name of the named bug, "" for normal doors
	Blind   bool
	Mystery bool
	Cursed  bool
	Regress bool // a regression door
}

// Stats is what top shows.
type Stats struct {
	Rating        float64
	RatingHistory []RatingPoint // oldest first
	Topics        []TopicStat

	Solves map[game.SolveKind]int

	Boots      int
	FullClears int
	Crashes    map[game.Outcome]int // by cause: TimedOut, OutOfLives, GaveUp

	Uptime        time.Duration // summed over all boots
	LifetimeBytes int64
	Bank          int64

	// EarlyCrash is the engine's forecast of a crash in the first biome
	// against what the last boots with one did (D36: the predicted chance,
	// shown after the doors); zero Boots hides it.
	EarlyCrash EarlyCrash

	// Retention is the retention panel (retention.go); nil hides it.
	Retention *Retention
}

// EarlyCrash is the engine's mean forecast of a crash in the first biome
// (guardrail G1) over Boots boots, Crashed of which did.
type EarlyCrash struct {
	Boots, Crashed int
	Forecast       float64
}

// RatingPoint is the player's rating after one rated attempt.
type RatingPoint struct {
	At     time.Time
	Rating float64
}

// TopicStat is one catalog topic's line in top.
type TopicStat struct {
	Topic    string
	Attempts int
	Clears   int
	Rating   float64
}

// ---- formatting ---------------------------------------------------------------

// uptime formats a long duration as htop does: "3 days, 04:12:33".
func uptime(d time.Duration) string {
	d = d.Round(time.Second)
	days := int(d.Hours()) / 24
	hms := fmt.Sprintf("%02d:%02d:%02d", int(d.Hours())%24, int(d.Minutes())%60, int(d.Seconds())%60)
	switch days {
	case 0:
		return hms
	case 1:
		return "1 day, " + hms
	}
	return fmt.Sprintf("%d days, %s", days, hms)
}

// ago formats how long before now t was, coarsely: "5h ago", "2d ago".
func ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// causeText names how a boot crashed, or how an attempt ended without a
// clear, the way the crash screen does.
func causeText(c game.Outcome) string {
	switch c {
	case game.TimedOut:
		return "timer"
	case game.OutOfLives:
		return "out of lives"
	case game.GaveUp:
		return ":q!"
	case game.Retreated:
		return "retreated"
	}
	return string(c)
}

// prestigeText is "prestige 2 (Gold)", or "prestige 6" when name has none
// or only says the number again (flavor's "Prestige 6" past the divisions).
func prestigeText(p int, name func(int) string) string {
	s := fmt.Sprintf("prestige %d", p)
	if n := name(p); n != "" && !strings.EqualFold(n, s) {
		s += " (" + n + ")"
	}
	return s
}

// ---- styles and layout ---------------------------------------------------------

// The screens paint with internal/ui: its tokens (a parody keeps its tool's
// look through them, R8), ui.Layout to fill the screen, ui.Rank for
// ratings.

// keyBar renders key help as htop's function-key bar: the key plain, the
// label on a cyan field.
func keyBar(pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		b.WriteString(pairs[i])
		b.WriteString(ui.Field.Render(pairs[i+1]))
	}
	return b.String()
}
