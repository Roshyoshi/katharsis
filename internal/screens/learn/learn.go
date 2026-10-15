// Package learn holds the RTFM (learn mode) screens: the curriculum browser,
// laid out like a man page index over the USACO Guide's divisions, chapters
// and modules, and the module page, which shows one module's facts and the
// trial that passing it takes.
//
// Both screens take plain data (a loaded curriculum, the player's Meta and
// two catalog functions) so the app can wire them without adapters. They
// never read files or the catalog themselves beyond those functions, and
// they leave every decision to the app as an intent: screens.OpenModule
// (read the module), screens.StartTrial, reader.OpenURL and screens.Back.
//
// The guide is CC BY-NC-SA 4.0. The browser names it in its header and
// prints the collection notice (learn.Attribution) at the end of the index;
// the module page prints the module's own attribution line.
package learn

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	guide "katharsis/internal/learn"
)

// Judgeable reports whether the catalog can serve and judge a problem; it
// decides which of a module's problems make its trial unit. nil accepts
// every USACO and CSES reference, as learn.TrialFor does.
type Judgeable = func(catalog.ID) bool

// Lookup resolves a catalog problem for display (title, rating, contest).
// nil, or a miss, falls back to the guide's own name for the problem.
type Lookup = func(catalog.ID) (catalog.Problem, bool)

// SPONGE: read this from the trial rules once the engine has them, so the
//         plan cannot drift from what a trial boot plays.

// integrationDoors is how many blind doors follow a trial's unit (DESIGN.md
// "Learn mode": 2 blind doors at the same difficulty). The engine owns the
// real number; this is only what the plan says.
const integrationDoors = 2

// entry is one module as both screens show it. The trial is built once, up
// front: its unit size and capability appear on every browser row.
type entry struct {
	mod   guide.Module
	trial guide.Trial
	biome string // flavor name of the biome the trial's doors come from
}

func newEntry(m guide.Module, judgeable Judgeable) entry {
	t := guide.TrialFor(m, judgeable)
	// The integration topic, not the module's own: General modules have
	// no topic, and their doors are drawn from Userland ("mixed").
	return entry{mod: m, trial: t, biome: flavor.BiomeName(t.IntegTopic)}
}

// short is how many unit doors the engine tops up from the catalog, by the
// integration topic and band.
func (e entry) short() int { return max(guide.DefaultUnitSize-len(e.trial.Unit), 0) }

// capSet is the capabilities the player holds.
func capSet(m game.Meta) map[string]bool {
	s := make(map[string]bool, len(m.Capabilities))
	for _, c := range m.Capabilities {
		s[c] = true
	}
	return s
}

// ---- formatting -------------------------------------------------------------

const (
	heldMark   = "◆"
	lockedMark = "◇"
)

// capCell renders a capability with its status mark: green ◆ held, faint ◇
// locked. Modules that teach no gated topic have none and get "".
func capCell(c string, held bool) string {
	switch {
	case c == "":
		return ""
	case held:
		return green.Render(heldMark + " " + c)
	}
	return faint.Render(lockedMark + " " + c)
}

// unitLabel says how big a trial's unit is: "4 doors", or "2+2 topped up"
// when the module has too few judgeable problems and the catalog fills the
// rest; the topped-up part is faint, since it is the catalog's and not the
// module's. compact drops the words for narrow terminals: "4", "2+2".
func unitLabel(have, short int, compact bool) string {
	switch {
	case short == 0 && compact:
		return fmt.Sprint(have)
	case short == 0:
		return fmt.Sprintf("%d doors", have)
	case compact:
		return fmt.Sprint(have) + faint.Render(fmt.Sprintf("+%d", short))
	}
	return fmt.Sprint(have) + faint.Render(fmt.Sprintf("+%d topped up", short))
}

// stars draws the guide's 0-4 "how often in contests" as four stars; an
// unset frequency (-1) draws nothing.
func stars(freq int) string {
	if freq < 0 {
		return ""
	}
	f := min(freq, 4)
	return yellow.Render(strings.Repeat("★", f)) + faint.Render(strings.Repeat("☆", 4-f))
}

// frequencyText is the guide's label for each frequency level.
func frequencyText(freq int) string {
	labels := []string{"has not appeared", "rare", "not frequent", "somewhat frequent", "very frequent"}
	if freq < 0 || freq >= len(labels) {
		return ""
	}
	return labels[freq]
}

// band formats a rating band in the colour of its middle.
func band(b [2]int) string {
	return ratingStyle(float64(b[0]+b[1]) / 2).Render(fmt.Sprintf("%d–%d", b[0], b[1]))
}

// highlight reverses each case-insensitive occurrence of q in s, a plain
// string. Search hits must stand out in a list that is otherwise mostly
// unstyled.
func highlight(s, q string) string {
	if q == "" {
		return s
	}
	low, lq := strings.ToLower(s), strings.ToLower(q)
	if len(low) != len(s) {
		return s // lower-casing moved bytes; positions would not line up
	}
	var b strings.Builder
	for {
		i := strings.Index(low, lq)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(reverse.Render(s[i : i+len(lq)]))
		s, low = s[i+len(lq):], low[i+len(lq):]
	}
}

// ---- styles and layout (as in screens/meta) ---------------------------------

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

// manHeader is a man page's first line: the page name at both ends and the
// manual's name centred. The centre gives way first on narrow terminals.
func manHeader(side, centre string, w int) string {
	sw, cw := ansi.StringWidth(side), ansi.StringWidth(centre)
	if 2*sw+cw+4 > w {
		return spread(side, side, w)
	}
	left := (w - cw) / 2
	return side + strings.Repeat(" ", left-sw) + centre +
		strings.Repeat(" ", w-left-cw-sw) + side
}

// wrap word-wraps s to w cells and indents every line by indent spaces.
func wrap(s string, indent, w int) []string {
	var out []string
	for _, l := range strings.Split(ansi.Wordwrap(s, max(w-indent, 10), ""), "\n") {
		out = append(out, strings.Repeat(" ", indent)+l)
	}
	return out
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

// distinctCaps counts the distinct capabilities es teach and how many of
// them the player holds; several modules can teach one capability.
func distinctCaps(es []entry, caps map[string]bool) (held, total int) {
	var seen []string
	for _, e := range es {
		c := e.trial.Capability
		if c == "" || slices.Contains(seen, c) {
			continue
		}
		seen = append(seen, c)
		if caps[c] {
			held++
		}
	}
	return held, len(seen)
}

// plural is "1 module", "2 modules"; "match" takes "es".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	if strings.HasSuffix(noun, "ch") {
		return fmt.Sprintf("%d %ses", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
