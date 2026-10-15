package anim

import (
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/ui"
)

// Pack is the Trial's ending, the pack of cards: "At this the whole pack
// rose up into the air, and came flying down" (Alice, ch. XII). Every box
// on a screen, the box-drawing and block cells of its frames, rules and
// bars, lifts off as a card. Each rises at its own moment and speed,
// tumbling, and falls past the bottom edge, as the title's fire climbs and
// dies; the text the boxes held goes dark behind them. The last frame is
// the screen gone, blank, so reduced motion is one cut to black.
//
// From is a rendered screen, its rows cut or padded to H rows of W cells;
// its styles are dropped. A frame is a function of t: Seed picks every
// card's flight.
type Pack struct {
	From []string
	W, H int
	D    time.Duration
	Seed uint64
}

// Clip is the pack as a clip. It reads the glyph set in use now, as the
// fire does: a screen builds its clip at View time.
func (p Pack) Clip() Clip {
	c := &packClip{p: p, w: max(p.W, 1), h: max(p.H, 1), half: HalfBlocks()}
	c.grid = make([][]packCell, c.h)
	for y := range c.grid {
		c.grid[y] = make([]packCell, c.w)
		line := ""
		if y < len(p.From) {
			line = ansi.Strip(p.From[y])
		}
		x := 0
		for _, r := range line {
			n := ansi.StringWidth(string(r))
			if n == 0 || x+n > c.w {
				continue
			}
			c.grid[y][x] = packCell{r: r, w: n}
			for k := 1; k < n; k++ {
				c.grid[y][x+k] = packCell{w: -1} // the rest of a wide rune
			}
			x += n
		}
	}
	d := p.D.Seconds()
	for y, row := range c.grid {
		for x, cell := range row {
			if !c.isCard(cell.r) {
				continue
			}
			k := mix(p.Seed ^ uint64(y*c.w+x+1)*0x9e3779b97f4a7c15)
			r1, r2, r3 := unit(k), unit(mix(k+1)), unit(mix(k+2))
			// Launched over the first third, each card rises up to its
			// height above it, then falls past the bottom by the end.
			t0 := d * 0.35 * r1
			c.cards = append(c.cards, flight(x, y, t0, d-t0, float64(y+1)*(0.4+0.6*r2), (r3-0.5)*0.5*float64(c.w), c.h))
			c.grid[y][x].card = true
		}
	}
	return c
}

// unit maps a hash to [0, 1).
func unit(k uint64) float64 { return float64(k>>11) / (1 << 53) }

// card is one card's flight: from cell (x, y) at t0, rising at v rows a
// second under gravity g, drifting dx columns over its flight.
type card struct {
	x, y         int
	t0, t, v, g  float64
	dx           float64
	tumbleOffset int
}

// flight solves a card's v and g: up rows above its cell at the top of
// its arc, two rows past the bottom (h) t seconds after launch.
func flight(x, y int, t0, t, up, dx float64, h int) card {
	t = math.Max(t, 0.05)
	// y(t) = y - v·t + g·t²/2 = h + 2 with v = sqrt(2·g·up): a quadratic
	// in sqrt(g).
	s := (math.Sqrt(2*up) + math.Sqrt(2*up+2*float64(h+2-y))) / t
	g := s * s
	return card{x: x, y: y, t0: t0, t: t, v: math.Sqrt(2 * g * up), g: g, dx: dx, tumbleOffset: x + y}
}

type packCell struct {
	r    rune
	w    int // cells the rune takes; -1 for the rest of a wide one, 0 empty
	card bool
}

type packClip struct {
	p     Pack
	w, h  int
	half  bool
	grid  [][]packCell
	cards []card
}

// isCard reports whether r is a box's: box drawing or a block element, or
// under the ASCII glyphs a frame's + | = #.
func (c *packClip) isCard(r rune) bool {
	switch {
	case r >= 0x2500 && r <= 0x259f:
		return true
	case !c.half:
		return strings.ContainsRune("+|=#", r)
	}
	return false
}

func (c *packClip) Len() time.Duration { return c.p.D }

// step is t on the pack's clock, which moves as the fire's does, FireStep at
// a time, so a frame holds still between two Nexts.
func (c *packClip) step(t time.Duration) time.Duration { return t.Truncate(FireStep) }

func (c *packClip) Next(t time.Duration) time.Duration {
	if t >= c.p.D {
		return Never
	}
	return min(c.step(t)+FireStep, c.p.D)
}

// textLevel is the level the text left behind is drawn at, by how far
// into the clip t is: there while the cards lift, then faint, then dust,
// then gone.
func (c *packClip) textLevel(t time.Duration) Level {
	switch f := float64(t) / float64(max(c.p.D, 1)); {
	case f < 0.3:
		return Normal
	case f < 0.5:
		return Faint
	case f < 0.7:
		return Dust
	}
	return Gone
}

// tumble is a card's face as it turns, a step every tenth of a second.
var (
	tumbleHalf  = []string{"▀", "▐", "▄", "▌"}
	tumbleASCII = []string{"-", "/", "|", `\`}
)

func (c *packClip) Frame(t time.Duration) []string {
	if t < c.p.D {
		t = c.step(max(t, 0))
	}
	type cell struct {
		s     string
		style int // 0 blank, 1 text, 2 a card
	}
	rows := make([][]cell, c.h)
	lvl := c.textLevel(t)
	for y, row := range c.grid {
		rows[y] = make([]cell, c.w)
		for x, g := range row {
			switch {
			case g.w < 0:
				rows[y][x] = cell{} // drawn by its rune
			case g.w == 0 || g.card || lvl == Gone || lvl == Dust && !dusty(c.p.Seed+uint64(y), x):
				rows[y][x] = cell{s: strings.Repeat(" ", max(g.w, 1))}
			default:
				rows[y][x] = cell{s: string(g.r), style: 1}
			}
		}
	}
	tumble := tumbleHalf
	if !c.half {
		tumble = tumbleASCII
	}
	sec := t.Seconds()
	for _, k := range c.cards {
		face := string(c.grid[k.y][k.x].r)
		x, y := k.x, k.y
		if tau := sec - k.t0; tau > 0 {
			fy := float64(k.y) - k.v*tau + k.g*tau*tau/2
			fx := float64(k.x) + k.dx*math.Min(tau/k.t, 1)
			x, y = int(math.Round(fx)), int(math.Floor(fy))
			face = tumble[(int(tau*10)+k.tumbleOffset)%len(tumble)]
		}
		if t >= c.p.D || y < 0 || y >= c.h || x < 0 || x >= c.w || rows[y][x].s == "" {
			continue
		}
		if c.grid[y][x].w > 1 {
			rows[y][x+1] = cell{s: " "} // the wide rune it covers keeps its width
		}
		rows[y][x] = cell{s: face, style: 2}
	}
	text, cardStyle := shade(lipgloss.NewStyle(), lvl), ui.FG.Style()
	out := make([]string, c.h)
	for y, row := range rows {
		var b, run strings.Builder
		cur := -1
		flush := func() {
			switch {
			case run.Len() == 0:
			case cur == 1:
				b.WriteString(text.Render(run.String()))
			case cur == 2:
				b.WriteString(cardStyle.Render(run.String()))
			default:
				b.WriteString(run.String())
			}
			run.Reset()
		}
		for _, cl := range row {
			if cl.s == "" {
				continue
			}
			if cl.style != cur {
				flush()
				cur = cl.style
			}
			run.WriteString(cl.s)
		}
		flush()
		out[y] = b.String()
	}
	return out
}
