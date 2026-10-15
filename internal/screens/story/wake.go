package story

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/flavor"
	"katharsis/internal/ui"
)

// The Trial's ending, Dull Reality (docs/design/achievements.md (f)):
// Wonderland's, where the final clear is Looking-Glass's. Six beats, one
// page each, from ending.wake.1 to .6:
//
//	1 the court     the boss pane as it was, the pool at its cap
//	2 the verdict   the judge's AC line; your AC speaks for you
//	3 the cards     every box on screen lifts off and falls (anim.Pack)
//	4 the bank      Mono: the terminal's own colors, a riverbank line
//	5 the man page  the prologue's, with EXAMPLES now: this boot's doors
//	6 the loop      the sister dreams on; GRUB's colors come back
//
// The first wake plays them all and cannot be skipped; any later one
// plays 4 to 6 and esc leaves it. Reduced motion shows each page's last
// frame: page 3's is a cut to black.

// Timings of the wake.
const (
	packTime  = 1600 * time.Millisecond // the cards up and down
	grubTime  = 900 * time.Millisecond  // GRUB's colors seeping back
	wakeFadeT = 400 * time.Millisecond
)

// Wake is what the ending shows of the boot that took it.
type Wake struct {
	// From is the screen the Trial's door was won on, as drawn: the court
	// on page 1 and 2, the boxes that lift off on page 3.
	From []string
	// AC is the judge's line for the door ("AC · 10/10 tests · 31:00 /
	// 36:00"): real values, as everywhere (GR3).
	AC string
	// Examples are the titles of the boot's cleared doors, in order, for
	// the man page's EXAMPLES.
	Examples []string
	// First is the first wake: every beat, recorded as ending.wake.
	First bool
	// Seed picks the cards' flight.
	Seed uint64
}

// riverbank is the bank in one line of plain text: grass, then the river.
const riverbank = `  ,.;,.,,;.,,.,.;.,,  ~ ~~ ~~~ ~~ ~ ~~~ ~~ ~ ~~ ~~~ ~ ~~ ~  `

// NewWake is the Trial's ending scene.
func NewWake(w Wake) *Scene {
	beat := func(n int) []string {
		key := "ending.wake." + strconv.Itoa(n)
		if b := flavor.StoryBlock(key); b != nil {
			return b
		}
		return lines(flavor.Story(key, 0))
	}
	// The court as it was: the screen's first rows, its header and the
	// pool at its cap, as many as the page leaves over its words.
	court := func(width, h, below int, from []string) []string {
		n := min(len(from), max(h-below, 1))
		out := make([]string, n)
		for i := range out {
			out[i] = ansi.Truncate(from[i], width, "")
		}
		return out
	}
	pages := []page{
		func(width, h int) anim.Clip {
			said := typed(beat(1), width)
			return anim.Then(anim.Hold{Lines: court(width, h, len(said.Lines)+1, w.From)}, blank, said)
		},
		func(width, h int) anim.Clip {
			said := typed(beat(2), width)
			ac := fadeIn(fit([]string{w.AC}, width), ui.OK.Style())
			return anim.Then(anim.Hold{Lines: court(width, h, len(said.Lines)+len(ac.Lines)+2, w.From)}, ac, blank, said)
		},
		func(width, h int) anim.Clip {
			said := typed(beat(3), width)
			rows := max(h-len(said.Lines)-1, 1)
			pack := anim.Pack{From: court(width, h, len(said.Lines)+1, w.From), W: width, H: rows, D: packTime, Seed: w.Seed}
			return anim.Then(pack.Clip(), blank, said)
		},
		// Dull reality: Mono from here, no theme, no italics.
		func(width, _ int) anim.Clip {
			bank := []string{ansi.Truncate(riverbank, max(width-2, 1), "")}
			said := anim.Typewriter{Lines: fit(beat(4), width), Style: lipgloss.NewStyle()}
			return anim.Then(fadeIn(bank, lipgloss.NewStyle()), blank, said)
		},
		func(width, h int) anim.Clip {
			said := anim.Typewriter{Lines: fit(beat(5), width), Style: lipgloss.NewStyle()}
			man := manPage(w.Examples, width, h-len(said.Lines)-1)
			return anim.Then(anim.Hold{Lines: man}, blank, said)
		},
		func(width, _ int) anim.Clip {
			said := typed(beat(6), width)
			grub := art("grub")
			fade := anim.Fade{Lines: grub, From: anim.Gone, To: anim.Normal, D: grubTime, Style: ui.Focus.Style()}
			return anim.Then(said, blank, over(fade, widest(said.Lines)))
		},
	}
	id := ""
	if w.First {
		id = "ending.wake"
	} else {
		pages = pages[3:]
	}
	s := newScene(id, 0, pages...)
	s.noSkip = w.First
	return s
}

// manPage is the prologue's man page at last with an EXAMPLES section:
// examples, as many as rows leave room for, the rest counted. It is
// plain: Mono, as man draws it.
func manPage(examples []string, w, rows int) []string {
	mw := min(max(w-2, 20), 78)
	head := "YOU(1)"
	title := "General Commands Manual"
	gap := max(mw-2*len(head)-len(title), 2)
	top := head + strings.Repeat(" ", gap/2) + title + strings.Repeat(" ", gap-gap/2) + head
	const in = "       "
	out := []string{ansi.Truncate(top, mw, ""), "", "NAME", in + "you - PID 1", "", "EXAMPLES"}
	tail := []string{"", "SEE ALSO", in + "rabbit(6)"}
	room := max(rows-len(out)-len(tail), 1)
	shown := examples
	if len(examples) > room {
		shown = append(examples[:room-1:room-1], ui.G.Ellipsis+" and "+strconv.Itoa(len(examples)-room+1)+" more")
	}
	for _, e := range shown {
		out = append(out, ansi.Truncate(in+e, mw, ui.G.Ellipsis))
	}
	return append(out, tail...)
}
