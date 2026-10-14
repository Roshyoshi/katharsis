// Package meta holds the screens outside a boot: the GRUB menu (start
// screen), git log (boot history), top (stats) and the upsolve picker.
//
// The screens take view models defined here (BootRow, AttemptRow, Stats,
// UpsolveItem) rather than store records, so they can be built and tested
// before the store settles; the app maps its records onto these types.
// Display strings that need the catalog (titles) or flavor (biome names)
// are resolved by the app when it builds the rows.
package meta

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
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

// UpsolveItem is a problem waiting to be upsolved: the core dump, or a
// regression whose due date has passed.
type UpsolveItem struct {
	Problem  catalog.ID
	Title    string
	CoreDump bool  // false: a regression
	Bytes    int64 // core dump bytes; 0 for regressions
	Boot     int   // the boot it crashed in
	Due      time.Time
	Stage    int // regression stage, index into rules.RegressionDays
}

// DueUpsolves lists what can be upsolved now: the core dump first, then
// regressions due at or before now, oldest due first. title resolves
// display titles; nil shows the catalog ID.
func DueUpsolves(m game.Meta, title func(catalog.ID) string, now time.Time) []UpsolveItem {
	name := func(id catalog.ID) string {
		if title != nil {
			if t := title(id); t != "" {
				return t
			}
		}
		return string(id)
	}
	var out []UpsolveItem
	if d := m.CoreDump; d != nil {
		out = append(out, UpsolveItem{
			Problem: d.Problem, Title: name(d.Problem), CoreDump: true,
			Bytes: d.Bytes, Boot: d.Boot,
		})
	}
	var regs []UpsolveItem
	for _, r := range m.Regressions {
		if r.Due.After(now) {
			continue
		}
		regs = append(regs, UpsolveItem{
			Problem: r.Problem, Title: name(r.Problem), Due: r.Due, Stage: r.Stage,
		})
	}
	slices.SortStableFunc(regs, func(a, b UpsolveItem) int { return a.Due.Compare(b.Due) })
	return append(out, regs...)
}

// ---- formatting ---------------------------------------------------------------

// Bytes formats a byte count the way the game shows currency: B, KiB, MiB.
func Bytes(n int64) string {
	neg := ""
	if n < 0 {
		neg, n = "-", -n
	}
	if n < 1024 {
		return fmt.Sprintf("%s%d B", neg, n)
	}
	v := float64(n)
	for _, unit := range []string{"KiB", "MiB", "GiB"} {
		v /= 1024
		if v < 1024 || unit == "GiB" {
			if v < 10 {
				return fmt.Sprintf("%s%.1f %s", neg, v, unit)
			}
			return fmt.Sprintf("%s%.0f %s", neg, v, unit)
		}
	}
	panic("unreachable")
}

// clock formats a duration as h:mm:ss, or m:ss under an hour.
func clock(d time.Duration) string {
	d = d.Round(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

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
	case d < 0:
		return "in " + coarse(-d)
	case d < time.Minute:
		return "just now"
	}
	return coarse(d) + " ago"
}

func coarse(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// causeText names a crash cause the way the crash screen does.
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
	case game.Cleared:
		return "cleared"
	}
	return string(c)
}

// ---- styles and layout ---------------------------------------------------------

var (
	bold    = lipgloss.NewStyle().Bold(true)
	faint   = lipgloss.NewStyle().Faint(true)
	reverse = lipgloss.NewStyle().Reverse(true)
	red     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	green   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	blue    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	magenta = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	cyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	gray    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// frame makes exactly h lines, each cut to w cells, so a screen never
// scrolls the terminal or wraps.
func frame(lines []string, w, h int) string {
	out := make([]string, h)
	for i := range out {
		if i < len(lines) {
			out[i] = ansi.Truncate(lines[i], w, "…")
		}
	}
	return strings.Join(out, "\n")
}

// pad right-pads s with spaces to w cells, cutting it when longer.
func pad(s string, w int) string {
	n := ansi.StringWidth(s)
	if n > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-n)
}

// padLeft right-aligns s in w cells.
func padLeft(s string, w int) string {
	n := ansi.StringWidth(s)
	if n >= w {
		return s
	}
	return strings.Repeat(" ", w-n) + s
}

// center centers s in w cells.
func center(s string, w int) string {
	n := ansi.StringWidth(s)
	if n >= w {
		return s
	}
	return strings.Repeat(" ", (w-n)/2) + s
}

// spread puts left and right on one line of w cells, cutting left first.
func spread(left, right string, w int) string {
	rw := ansi.StringWidth(right)
	if rw >= w {
		return ansi.Truncate(right, w, "…")
	}
	left = ansi.Truncate(left, max(w-rw-1, 0), "…")
	gap := w - ansi.StringWidth(left) - rw
	return left + strings.Repeat(" ", max(gap, 1)) + right
}

// ratingStyle colours a rating by Codeforces rank, the scale every rating
// in the game is on: gray newbie through red grandmaster.
func ratingStyle(r float64) lipgloss.Style {
	switch {
	case r < 1200:
		return gray
	case r < 1400:
		return green
	case r < 1600:
		return cyan
	case r < 1900:
		return blue
	case r < 2100:
		return magenta
	case r < 2400:
		return yellow
	}
	return red
}

// keyBar renders key help as htop's function-key bar: the key plain, the
// label on a cyan field.
func keyBar(pairs ...string) string {
	label := lipgloss.NewStyle().Background(lipgloss.Color("6")).Foreground(lipgloss.Color("0"))
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		b.WriteString(pairs[i])
		b.WriteString(label.Render(pairs[i+1]))
	}
	return b.String()
}
