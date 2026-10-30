// Package rtfm holds the RTFM (learn mode) screens: the curriculum browser,
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
// prints the collection notice (curriculum.Attribution) at the end of the
// index; the module page prints the module's own attribution line.
package rtfm

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
)

// Judgeable reports whether the catalog can serve and judge a problem; it
// decides which of a module's problems its map plays. nil accepts every
// problem with a catalog ID, as curriculum.TrialFor does.
type Judgeable = func(catalog.ID) bool

// trialPass is how many of a trial's doors passing takes: the rest may be
// lost to :q! (rules.Trial's TrialLostRooms).
var trialPass = curriculum.Doors - rules.Trial().TrialLostRooms

// entry is one module as both screens show it. The trial is built once, up
// front: its map's size and its capability appear on every browser row.
type entry struct {
	mod   curriculum.Module
	trial curriculum.Trial
	biome string // flavor name of the biome the trial's doors come from
}

func newEntry(m curriculum.Module, judgeable Judgeable, lookup screens.Lookup) entry {
	t := curriculum.TrialFor(m, judgeable, lookup)
	// The trial's topic, not the module's own: General modules have no
	// topic, and their doors are drawn from Userland ("mixed").
	return entry{mod: m, trial: t, biome: flavor.BiomeName(t.Topic)}
}

// capSet is the capabilities the player holds.
func capSet(m game.Meta) map[string]bool {
	s := make(map[string]bool, len(m.Capabilities))
	for _, c := range m.Capabilities {
		s[c] = true
	}
	return s
}

// passedSet is the modules whose trial the player passed.
func passedSet(m game.Meta) map[string]bool {
	s := make(map[string]bool, len(m.TrialsPassed))
	for _, id := range m.TrialsPassed {
		s[id] = true
	}
	return s
}

// ---- formatting -------------------------------------------------------------

// capCell renders a capability with its status mark: ■ held (OK), □ not
// yet (Muted). Modules that teach no gated topic have none and get "".
func capCell(c string, held bool) string {
	switch {
	case c == "":
		return ""
	case held:
		return ui.OK.Render(ui.G.Cap + " " + c)
	}
	return ui.Muted.Render(ui.G.CapLocked + " " + c)
}

// mapLabel says how big a module's map is: "4 core +2 stretch", the
// stretch muted since nothing waits on it. compact drops the words for
// narrow terminals: "4+2".
func mapLabel(m curriculum.Map, compact bool) string {
	core, side := m.Core(), len(m.Nodes)-m.Core()
	switch {
	case side == 0 && compact:
		return fmt.Sprint(core)
	case side == 0:
		return fmt.Sprintf("%d core", core)
	case compact:
		return fmt.Sprint(core) + ui.Muted.Render(fmt.Sprintf("+%d", side))
	}
	return fmt.Sprintf("%d core", core) + ui.Muted.Render(fmt.Sprintf(" +%d stretch", side))
}

// aims is a trial's door ratings: "◆1400 → ◆1600 → ◆1700"; "" when the
// map aims none and the doors spread over the band.
func aims(t curriculum.Trial) string {
	var out []string
	for _, r := range t.Targets {
		out = append(out, ui.Rating(r))
	}
	return strings.Join(out, " "+ui.G.Arrow+" ")
}

// stars draws the guide's 0-4 "how often in contests" as four stars; an
// unset frequency (-1) draws nothing.
func stars(freq int) string {
	if freq < 0 {
		return ""
	}
	f := min(freq, 4)
	return ui.Warn.Render(strings.Repeat(ui.G.Star, f)) + ui.Muted.Render(strings.Repeat(ui.G.StarOff, 4-f))
}

// frequencyText is the guide's label for each frequency level.
func frequencyText(freq int) string {
	labels := []string{"has not appeared", "rare", "not frequent", "somewhat frequent", "very frequent"}
	if freq < 0 || freq >= len(labels) {
		return ""
	}
	return labels[freq]
}

// highlight underlines each case-insensitive occurrence of q in s, a plain
// string, as the reader marks search hits.
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
		b.WriteString(ui.Hit.Render(s[i : i+len(lq)]))
		s, low = s[i+len(lq):], low[i+len(lq):]
	}
}

// ---- man page layout --------------------------------------------------------

// manHeader is a man page's first line: the page name at both ends and the
// manual's name centred. The centre gives way first on narrow terminals,
// then the right-hand name.
func manHeader(side, centre string, w int) string {
	sw, cw := ansi.StringWidth(side), ansi.StringWidth(centre)
	if 2*sw+cw+4 > w {
		return ui.Spread(side, side, w)
	}
	left := (w - cw) / 2
	return side + strings.Repeat(" ", left-sw) + centre +
		strings.Repeat(" ", w-left-cw-sw) + side
}

// wrap word-wraps s to w cells and indents every line by indent spaces.
func wrap(s string, indent, w int) []string {
	var out []string
	for _, l := range ui.WrapN(s, max(w-indent, 10), 0) {
		out = append(out, strings.Repeat(" ", indent)+l)
	}
	return out
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
