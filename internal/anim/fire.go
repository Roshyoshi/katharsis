package anim

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"katharsis/internal/ui"
)

// FireStep is how often a Fire moves: about 30 steps a second, as the
// PlayStation original ran its fire, whatever the frame-rate cap. A lower
// cap draws every other step; a higher one never draws more.
const FireStep = time.Second / DefaultFPS

// Fire is the title screen's fire, the spread-and-decay fire of the
// PlayStation port of a 1993 shooter (Fabien Sanglard's write-up, "How
// DOOM fire was done", describes it; this is written from the algorithm).
//
// It is a grid of heat, two pixels to a cell row, whose bottom row burns
// at full heat. Each step, every pixel's heat moves up one row, lands a
// pixel to either side or straight above at random (left more often, so
// the flames lean), and cools by a random amount, so flames rise and burn
// out as they climb. Put the bottom row out and the fire dies the same
// way. One thing is added to the original: the cooling sways across the
// fire in slow waves (sway), so the flames rise in tongues where even
// cooling, at a terminal's few pixels, leaves their top flat. The
// randomness is seeded, so a frame is a function of time.
//
// The heat ramp is the terminal's own 16 colors: black (the background)
// through red, bright red, yellow and bright yellow to white, 21 steps,
// each color and three shades (░▒▓) between each two, on a curve that
// gives the reds most of a flame and white only its root. A cell draws its
// two pixels as one shade where they are a step apart or less, and as a
// half block (▀, the top pixel's color over the bottom's) where they
// differ more. The ASCII glyph set draws each pixel pair as one cell in
// .:*#, and Mono (no hue) draws the heat as shades alone.
type Fire struct {
	W, H int // cells
	Seed uint64
	// Reach is how high the flames climb, in pixel rows (two to a cell
	// row): a pixel loses full heat over Reach rows, on average. The
	// flames sway, and the columns that cool slowest climb about twice as
	// high.
	Reach int
	// Burn is how long the bottom row burns; for Out after it, the fire
	// dies down. Len is Burn+Out, and the last frame is the fire then.
	Burn, Out time.Duration
	// Over is pixel art drawn over the flames, Spans text over them.
	Over  Sprite
	Spans []Span
}

// Sprite is pixel art over a Fire: Pix[y][x] is drawn at pixel (X+x,
// Y+y), two pixel rows to a cell row. Pixels off the fire are dropped.
type Sprite struct {
	X, Y int
	Pix  [][]Ink
}

// Ink is a Sprite pixel: a color of the heat ramp, drawn solid over the
// flames. ASCII and Mono, which cannot paint it, draw what it is instead:
// the face (Flame to Glow), the depth (Ember), or nothing (Void).
type Ink uint8

const (
	Clear Ink = iota // the flames show through
	Void             // the background over the flames: an outline
	Ember            // red: the depth under a bevelled face
	Flame            // bright red
	Gold             // yellow
	Blaze            // bright yellow
	Glow             // white: a highlight
	numInks
)

// Span is a line of text over a Fire, from At on: S, already styled and
// ending with its styles reset, W cells wide, at cell (X, Y). Its cells
// hide the flames, so a Span padded with spaces reads over any heat. A
// span that overlaps one to its left on its row is dropped.
type Span struct {
	X, Y, W int
	S       string
	At      time.Duration
}

// heat is fixed point: a ramp level is heat>>8.
const (
	levels  = 21 // black, then 4 steps to each of 5 colors
	hotHeat = levels<<8 - 1
)

// noColor is no color: the terminal's own foreground or background.
// The rest are ANSI palette indices.
const noColor int8 = -1

// ramp is the heat ramp's colors, coolest first: level 4k is ramp[k] solid.
var ramp = [6]int8{noColor, 1, 9, 3, 11, 15}

// inkColor is each Ink's color.
var inkColor = [numInks]int8{Clear: noColor, Void: noColor, Ember: 1, Flame: 9, Gold: 3, Blaze: 11, Glow: 15}

// Clip is the fire as a clip. It reads the palette and the glyph set in
// use now (ui.Hued, ui.G), as the rest of anim reads them when a frame is
// drawn: a screen builds its clip at View time.
func (f Fire) Clip() Clip {
	c := &fireClip{f: f, w: max(f.W, 1), h: max(f.H, 1)}
	c.px = 2 * c.h
	c.half, c.hued = HalfBlocks(), ui.Hued()
	c.looks = lookTable(c.half, c.hued)
	c.over = make([]uint8, c.w*c.px)
	for y, row := range f.Over.Pix {
		for x, ink := range row {
			px, py := f.Over.X+x, f.Over.Y+y
			if ink != Clear && ink < numInks && px >= 0 && px < c.w && py >= 0 && py < c.px {
				c.over[py*c.w+px] = uint8(ink)
			}
		}
	}
	c.rows = make([][]Span, c.h)
	for _, s := range f.Spans {
		if s.Y >= 0 && s.Y < c.h && s.W > 0 && s.X >= 0 && s.X+s.W <= c.w {
			c.rows[s.Y] = append(c.rows[s.Y], s)
		}
	}
	for _, r := range c.rows {
		slices.SortStableFunc(r, func(a, b Span) int { return a.X - b.X })
	}
	// Burning, a pixel cools by full heat over Reach rows; once out, at
	// least twice as fast, and fast enough that the heat at the bottom is
	// gone within Out, with a margin for the pixels that cool slowest.
	mean := hotHeat / max(f.Reach, 1)
	outSteps := max(int(f.Out/FireStep), 1)
	c.cool, c.coolOut = uint64(2*mean+1), uint64(2*max(2*mean, 5*hotHeat/(2*outSteps))+1)
	c.coolX = make([]uint64, c.w)
	c.phase = [2]float64{float64(f.Seed%628) / 100, float64(f.Seed/628%628) / 100}
	c.reset()
	return c
}

// HalfBlocks reports whether a Fire draws two pixels to a cell: whether
// the glyph set in use has block elements (its bar fill is one). The
// ASCII set has none, so a cell is one pixel, shaded in .:*#.
func HalfBlocks() bool { return ui.G.Fill == ui.Unicode.Fill }

type fireClip struct {
	f          Fire
	w, h, px   int // cells across, cell rows, pixel rows
	half, hued bool
	looks      [][]look
	over       []uint8  // the Sprite's ink at each pixel, Clear for none
	rows       [][]Span // each cell row's spans, left to right
	cool       uint64   // a step cools a pixel by up to cool-1, while it burns
	coolOut    uint64   // ... and once the fire is out
	coolX      []uint64 // this step's cool at each column (sway)
	phase      [2]float64

	mu   sync.Mutex
	step int     // the step heat is at
	heat []int32 // px rows of w pixels
	rng  uint64
}

func (c *fireClip) Len() time.Duration { return max(c.f.Burn, 0) + max(c.f.Out, 0) }

// steps is the step shown at t.
func (c *fireClip) steps(t time.Duration) int {
	return int(min(max(t, 0), c.Len()) / FireStep)
}

func (c *fireClip) Next(t time.Duration) time.Duration {
	if t >= c.Len() {
		return Never
	}
	return min(time.Duration(c.steps(t)+1)*FireStep, c.Len())
}

func (c *fireClip) Frame(t time.Duration) []string {
	n := c.steps(t)
	c.mu.Lock()
	defer c.mu.Unlock()
	if n < c.step {
		c.reset()
	}
	for c.step < n {
		c.spread()
	}
	return c.draw(time.Duration(n) * FireStep)
}

// reset lights the fire: cold everywhere but the bottom row.
func (c *fireClip) reset() {
	c.heat = make([]int32, c.w*c.px)
	bottom := c.heat[(c.px-1)*c.w:]
	for i := range bottom {
		bottom[i] = hotHeat
	}
	c.step, c.rng = 0, c.f.Seed
}

// rand is splitmix64.
func (c *fireClip) rand() uint64 {
	c.rng += 0x9e3779b97f4a7c15
	return mix(c.rng)
}

// spread is one step: every pixel's heat moves up a row, drifts and cools.
// The bottom row is the source; once Burn is over it goes out, and the
// rest cools faster, so the fire dies within Out.
func (c *fireClip) spread() {
	w, h := c.w, c.heat
	if time.Duration(c.step)*FireStep >= max(c.f.Burn, 0) {
		// Out: the bottom row goes dark and every column cools alike.
		clear(h[(c.px-1)*w:])
		for x := range c.coolX {
			c.coolX[x] = c.coolOut
		}
	} else {
		c.sway()
	}
	for y := 1; y < c.px; y++ {
		row, up := h[y*w:(y+1)*w], h[(y-1)*w:y*w]
		for x, v := range row {
			if v == 0 {
				up[x] = 0
				continue
			}
			r := c.rand()
			// Drift: one right, straight up, one left or two left, so
			// the flames lean left by half a pixel a row; the fire wraps
			// round the sides.
			dx := (x + 1 - int(r&3) + w) % w
			up[dx] = max(v-int32((r>>8)%c.coolX[x]), 0)
		}
	}
	c.step++
}

// pixel is the value at pixel (x, y) the look table is indexed by: a heat
// level, or levels+ink for a Sprite pixel.
func (c *fireClip) pixel(x, y int) uint8 {
	i := y*c.w + x
	if ink := c.over[i]; ink != 0 {
		return levels - 1 + ink
	}
	return heatLevel[c.heat[i]]
}

// draw renders the heat as it is, with the spans shown at t.
func (c *fireClip) draw(t time.Duration) []string {
	out := make([]string, c.h)
	var b strings.Builder
	b.Grow(c.w * 4)
	for y := range c.h {
		b.Reset()
		var cur look // the SGR state written so far: no colors
		cur.fg, cur.bg = noColor, noColor
		spans := c.rows[y]
		for x := 0; x < c.w; {
			for len(spans) > 0 && spans[0].X < x {
				spans = spans[1:] // overlaps the span before it
			}
			if len(spans) > 0 && spans[0].X == x && spans[0].At <= t {
				if cur.fg != noColor || cur.bg != noColor {
					b.WriteString("\x1b[m")
					cur.fg, cur.bg = noColor, noColor
				}
				b.WriteString(spans[0].S)
				x += spans[0].W
				spans = spans[1:]
				continue
			}
			if len(spans) > 0 && spans[0].X == x {
				spans = spans[1:] // not shown yet: the fire draws there
			}
			l := &c.looks[c.pixel(x, 2*y)][c.pixel(x, 2*y+1)]
			l.paint(&b, &cur)
			x++
		}
		if cur.fg != noColor || cur.bg != noColor {
			b.WriteString("\x1b[m")
		}
		out[y] = b.String()
	}
	return out
}

// look is how a cell is drawn: its glyph over its colors.
type look struct {
	fg, bg int8
	glyph  string
}

var fgCode, bgCode = sgrCodes(30, 90, 39), sgrCodes(40, 100, 49)

// sgrCodes are the SGR parameters for each color, none first: base+n for
// the first eight, bright+n-8 for the rest.
func sgrCodes(base, bright, reset int) [17]string {
	var out [17]string
	out[0] = strconv.Itoa(reset)
	for n := range 16 {
		if n < 8 {
			out[n+1] = strconv.Itoa(base + n)
		} else {
			out[n+1] = strconv.Itoa(bright + n - 8)
		}
	}
	return out
}

// paint writes l, changing only the colors that differ from cur. A blank
// cell needs no foreground.
func (l *look) paint(b *strings.Builder, cur *look) {
	fg := l.fg != cur.fg && l.glyph != " "
	bg := l.bg != cur.bg
	if fg || bg {
		b.WriteString("\x1b[")
		if fg {
			b.WriteString(fgCode[l.fg+1])
			cur.fg = l.fg
		}
		if bg {
			if fg {
				b.WriteByte(';')
			}
			b.WriteString(bgCode[l.bg+1])
			cur.bg = l.bg
		}
		b.WriteByte('m')
	}
	b.WriteString(l.glyph)
}

// lookTable is the look of every pair of pixels, top then bottom, for a
// glyph set with block elements or without and a palette with hues or
// without.
func lookTable(half, hued bool) [][]look {
	shades := []string{"░", "▒", "▓", "█"}
	if !half {
		shades = []string{".", ":", "*", "#"}
	}
	n := levels - 1 + int(numInks)
	t := make([][]look, n)
	for a := range t {
		t[a] = make([]look, n)
		for b := range t[a] {
			t[a][b] = lookOf(uint8(a), uint8(b), half, hued, shades)
		}
	}
	return t
}

// isInk reports whether pixel value v is a Sprite's, and which ink.
func isInk(v uint8) (Ink, bool) {
	if v >= levels {
		return Ink(v - levels + 1), true
	}
	return Clear, false
}

func face(v uint8) bool {
	ink, ok := isInk(v)
	return ok && ink >= Flame
}

func depth(v uint8) bool {
	ink, ok := isInk(v)
	return ok && ink == Ember
}

// colorOf is the color pixel value v is drawn in as half a cell: an ink's
// own, or the ramp color nearest a heat level.
func colorOf(v uint8) int8 {
	if ink, ok := isInk(v); ok {
		return inkColor[ink]
	}
	return ramp[(v+2)/4]
}

func lookOf(a, b uint8, half, hued bool, shades []string) look {
	_, inkA := isInk(a)
	_, inkB := isInk(b)
	art := inkA || inkB
	plain := func(g string) look { return look{fg: noColor, bg: noColor, glyph: g} }
	// A shade for one level: a heat ramp cell, or its glyph alone.
	level := func(lv int) look {
		if lv == 0 {
			return plain(" ")
		}
		g := shades[min((lv-1)/5, 3)]
		if !hued {
			return plain(g)
		}
		if !half {
			return look{fg: ramp[(lv+3)/4], bg: noColor, glyph: g}
		}
		band, sub := lv/4, lv%4
		if sub == 0 {
			return look{fg: noColor, bg: ramp[band], glyph: " "}
		}
		return look{fg: ramp[band+1], bg: ramp[band], glyph: shades[sub-1]}
	}
	switch {
	case !art:
		lv, d := (int(a)+int(b)+1)/2, int(a)-int(b)
		if !half || !hued || d >= -1 && d <= 1 {
			return level(lv)
		}
		return halves(colorOf(a), colorOf(b))
	case half && hued:
		return halves(colorOf(a), colorOf(b))
	case half: // Mono: the face in half blocks, its depth a shade
		switch {
		case face(a) && face(b):
			return plain(shades[3])
		case face(a):
			return plain("▀")
		case face(b):
			return plain("▄")
		case depth(a) || depth(b):
			return plain(shades[1])
		}
		return plain(" ")
	}
	// ASCII: one pixel per cell, the face over the depth.
	for _, v := range []uint8{a, b} {
		if face(v) {
			if hued {
				return look{fg: colorOf(v), bg: noColor, glyph: shades[3]}
			}
			return plain(shades[3])
		}
	}
	if depth(a) || depth(b) {
		if hued {
			return look{fg: inkColor[Ember], bg: noColor, glyph: shades[1]}
		}
		return plain(shades[1])
	}
	return plain(" ")
}

// halves draws a cell as two colors, top over bottom.
func halves(top, bottom int8) look {
	switch top {
	case bottom:
		return look{fg: noColor, bg: top, glyph: " "}
	case noColor:
		return look{fg: bottom, bg: noColor, glyph: "▄"}
	}
	return look{fg: top, bg: bottom, glyph: "▀"}
}

// heatLevel is the ramp level of each heat: a curve, not a line, so the
// reds take most of a flame's height and white only its root, as in the
// original's palette.
var heatLevel = func() []uint8 {
	out := make([]uint8, hotHeat+1)
	for h := range out {
		f := float64(h) / hotHeat
		out[h] = uint8(math.Round(float64(levels-1) * math.Pow(f, fireGamma)))
	}
	return out
}()

const fireGamma = 2.0

// sway sets each column's cooling for this step while the fire burns:
// the even cooling, scaled up and down across the fire by two slow waves
// that drift apart over time. The columns that cool slower burn taller,
// so the top of the fire rises and falls in tongues and sways, where even
// cooling leaves it flat.
func (c *fireClip) sway() {
	s := float64(c.step)
	for x := range c.coolX {
		fx := float64(x)
		m := 1 + 0.35*math.Sin(fx*2*math.Pi/23+c.phase[0]+s*0.07) +
			0.2*math.Sin(fx*2*math.Pi/9+c.phase[1]-s*0.11)
		c.coolX[x] = uint64(float64(c.cool)*m) + 1
	}
}
