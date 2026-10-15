package story

import (
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/flavor"
)

// The title screen's timings. The fire climbs to its height in a second
// at 80x24 and in under three at 200x60, burns on to titleBurn, then dies
// down for titleOut, and the title leaves titleTail after: a few seconds
// of fire after it settles. Any key leaves at once.
const (
	titleBurn = 5 * time.Second
	titleOut  = time.Second
	titleTail = 100 * time.Millisecond
	tagAt     = 700 * time.Millisecond  // the tagline shows
	keyAt     = 1400 * time.Millisecond // "press any key" shows
	titleSeed = 1993                    // the fire burns the same way every launch
)

// NewTitle is the title screen at launch: KATHARSIS in heavy bevelled
// block letters over a fire that burns up from the bottom, behind and
// under them, then the tagline (title.tagline) and "press any key". It
// leaves on its own once the fire dies down, or at any key. Under reduced
// motion it is one still frame of the fire at its height, shown as long
// as the fire would burn.
//
// It is drawn over the whole screen, with no footer, and fits any size:
// the logo is drawn at the largest size that fits, or as the plain banner
// in a narrow pane.
func NewTitle() *Scene {
	out, dwell := titleOut, titleTail
	if anim.CurrentMotion().Reduced {
		out, dwell = 0, titleBurn
	}
	s := newScene("", dwell, func(w, h int) anim.Clip { return titleFire(w, h, titleBurn, out) })
	s.bare = true
	return s
}

// font is the logo's letters, hand-drawn on an 8-pixel grid (# is a
// pixel; a pixel is half a cell tall, so about square). The strokes are
// two pixels, the counters two: heavy and square, to be bevelled.
var font = map[rune][]string{
	'K': {
		"##..##",
		"##.##.",
		"####..",
		"###...",
		"####..",
		"##.##.",
		"##..##",
		"##..##",
	},
	'A': {
		".####.",
		"######",
		"##..##",
		"##..##",
		"######",
		"######",
		"##..##",
		"##..##",
	},
	'T': {
		"######",
		"######",
		"..##..",
		"..##..",
		"..##..",
		"..##..",
		"..##..",
		"..##..",
	},
	'H': {
		"##..##",
		"##..##",
		"##..##",
		"######",
		"######",
		"##..##",
		"##..##",
		"##..##",
	},
	'R': {
		"#####.",
		"######",
		"##..##",
		"##..##",
		"#####.",
		"####..",
		"##.##.",
		"##..##",
	},
	'S': {
		".#####",
		"######",
		"##....",
		"#####.",
		".#####",
		"....##",
		"######",
		"#####.",
	},
	'I': {
		"##",
		"##",
		"##",
		"##",
		"##",
		"##",
		"##",
		"##",
	},
}

// logoSize is a size the logo is drawn at: each font pixel is sx pixels
// across and sy down, the letters gap pixels apart; the top row leans out
// by up to flare pixels each side, and the face stands depth pixels deep.
type logoSize struct{ sx, sy, gap, flare, depth int }

// logoSizes are the sizes, largest first, for half blocks (two pixels a
// cell row): 126 cells across and 11 rows, or 64 and 6.
var logoSizes = []logoSize{{2, 2, 2, 4, 3}, {1, 1, 1, 2, 2}}

// asciiSizes are the sizes for the ASCII glyph set, one pixel a cell row:
// each font row takes a whole cell row, or the letters lose their
// counters. 126 across and 10 rows, or 64 and 10.
var asciiSizes = []logoSize{{2, 2, 2, 4, 2}, {1, 2, 1, 2, 2}}

// logoWord is what the logo spells.
const logoWord = "KATHARSIS"

// logo is word in the font at size z, as inks: the faces lit from above
// (a white top edge, bright yellow to bright red down the face), the
// depth in red under them, and a background outline all round, so the
// letters stand clear of the flames. The letters lean out from the
// middle toward the top, like lettering that looms over you.
func logo(word string, z logoSize) [][]anim.Ink {
	var widths []int
	base := 0
	for i, r := range word {
		if i > 0 {
			base += z.gap
		}
		w := len(font[r][0]) * z.sx
		widths = append(widths, w)
		base += w
	}
	fh := 8 * z.sy
	pw, ph := base+2*z.flare+2, fh+z.depth+2
	pix := make([][]anim.Ink, ph)
	for y := range pix {
		pix[y] = make([]anim.Ink, pw)
	}
	isFace := make([][]bool, ph)
	for y := range isFace {
		isFace[y] = make([]bool, pw)
	}
	x0 := 0
	for i, r := range word {
		glyph, w := font[r], widths[i]
		// How far this letter leans: -1 at the left end, 1 at the right.
		rel := float64(2*x0+w-base) / float64(base)
		// The lean steps once a cell row (the face starts on one), so the
		// two pixels of a cell are always one font row's.
		cells := max(fh/2-1, 1)
		for y := range fh {
			dx := int(rel*float64(z.flare*(cells-y/2))/float64(cells) + 0.5*sign(rel))
			row := glyph[y/z.sy]
			for x := range w {
				if row[x/z.sx] == '#' {
					isFace[1+y][1+z.flare+x0+x+dx] = true
				}
			}
		}
		x0 += w + z.gap
	}
	for y := range ph {
		for x := range pw {
			if !isFace[y][x] {
				continue
			}
			f := y - 1 // the row down the face
			switch {
			case !isFace[y-1][x]:
				pix[y][x] = anim.Glow
			case f < fh*3/8:
				pix[y][x] = anim.Blaze
			case f < fh*6/8:
				pix[y][x] = anim.Gold
			default:
				pix[y][x] = anim.Flame
			}
			for k := 1; k <= z.depth && y+k < ph; k++ {
				if !isFace[y+k][x] && pix[y+k][x] == anim.Clear {
					pix[y+k][x] = anim.Ember
				}
			}
		}
	}
	solid := func(y, x int) bool {
		return y >= 0 && y < ph && x >= 0 && x < pw && pix[y][x] != anim.Clear && pix[y][x] != anim.Void
	}
	for y := range ph {
		for x := range pw {
			if pix[y][x] != anim.Clear {
				continue
			}
		near:
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if solid(y+dy, x+dx) {
						pix[y][x] = anim.Void
						break near
					}
				}
			}
		}
	}
	return pix
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

// titleFire is the title at w×h: the logo (or the banner, where it does
// not fit) and two lines of text over a fire whose flames reach the logo.
func titleFire(w, h int, burn, out time.Duration) anim.Clip {
	tags := tagRows(w)
	letters, banner := titleLogo(w, h), plainBanner()
	artRows := len(banner)
	if letters != nil {
		artRows = len(letters) / 2
	}
	top := max((h-artRows-textRows(w))*2/5, 0)
	f := anim.Fire{W: w, H: h, Seed: titleSeed, Burn: burn, Out: out}
	if letters != nil {
		// The art's first pixel row is its outline, in the cell row above
		// it, so its face starts at the top of a cell.
		f.Over = anim.Sprite{X: (w - len(letters[0])) / 2, Y: 2*top - 1, Pix: letters}
	} else {
		bw := widest(banner)
		for i, l := range banner {
			// Each row as wide as the widest, so the lettering keeps its shape.
			l += strings.Repeat(" ", bw-ansi.StringWidth(l))
			f.Spans = append(f.Spans, centered(l, plain().Render(l), w, top+i, 0))
		}
	}
	y := top + artRows + 1
	for _, l := range tags {
		f.Spans = append(f.Spans, centered(l, voice().Render(l), w, y, tagAt))
		y++
	}
	key := "press any key"
	f.Spans = append(f.Spans, centered(key, muted().Render(key), w, y+1, keyAt))
	// The flames reach, on average, the middle of the art: they lick up
	// behind it, and their tips rise past it now and then.
	f.Reach = max(2*h-(2*top+artRows), 4)
	return f.Clip()
}

// tagRows is the tagline, wrapped where w is too narrow for it.
func tagRows(w int) []string { return wrapTo([]string{flavor.Story("title.tagline", 0)}, w-4) }

// textRows is how many rows the text under the art takes at width w: a
// gap, the tagline, a gap, the key line.
func textRows(w int) int { return 1 + len(tagRows(w)) + 2 }

// titleLogo is the logo at the largest size that fits w×h with the text
// and titleRoom rows of fire under it, nil where none does.
func titleLogo(w, h int) [][]anim.Ink {
	sizes := logoSizes
	if !anim.HalfBlocks() {
		sizes = asciiSizes
	}
	for _, z := range sizes {
		l := logo(logoWord, z)
		if len(l[0])+2 <= w && len(l)/2+textRows(w)+titleRoom <= h {
			return l
		}
	}
	return nil
}

// titleRoom is the fewest rows of fire the logo leaves under the text.
const titleRoom = 4

// wrapTo is ls word-wrapped to w cells, never narrower than 10.
func wrapTo(ls []string, w int) []string {
	w = max(w, 10)
	var out []string
	for _, l := range ls {
		if ansi.StringWidth(l) <= w {
			out = append(out, l)
			continue
		}
		out = append(out, strings.Split(ansi.Wrap(l, w, ""), "\n")...)
	}
	return out
}

// centered is a Span of the plain line l, drawn as styled, centered on
// row y with two blank cells each side to hold the flames off, cut to w.
func centered(l, styled string, w, y int, at time.Duration) anim.Span {
	lw := ansi.StringWidth(l)
	sw := min(lw+4, w)
	pad := (sw - lw) / 2
	s := strings.Repeat(" ", max(pad, 0)) + styled + strings.Repeat(" ", max(sw-lw-pad, 0))
	if lw > sw {
		s = ansi.Truncate(styled, sw, "")
	}
	return anim.Span{X: (w - sw) / 2, Y: y, W: sw, S: s, At: at}
}

// plainBanner is the logo where the art does not fit: the banner's
// lettering, or the word spaced out in the ASCII glyph set, which has no
// box drawing.
func plainBanner() []string {
	if !anim.HalfBlocks() {
		return []string{strings.Join(strings.Split(logoWord, ""), " ")}
	}
	return art("grub")[:3]
}
