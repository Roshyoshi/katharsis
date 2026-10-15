package anim

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim/animtest"
	"katharsis/internal/ui"
)

// withUI runs f under glyph set g and palette p, then restores the
// defaults.
func withUI(g ui.Glyphs, p ui.Palette, f func()) {
	ui.SetGlyphs(g)
	ui.SetPalette(p)
	defer func() {
		ui.SetGlyphs(ui.Unicode)
		ui.SetPalette(ui.ANSI)
	}()
	f()
}

// testFire is a small fire with a bevelled block of art and a line of
// text over it, which shows from 500 ms.
func testFire(w, h int) Fire {
	return Fire{
		W: w, H: h, Seed: 7, Reach: 3 * h / 2, Burn: 2 * time.Second, Out: 500 * time.Millisecond,
		Over: Sprite{X: 2, Y: 3, Pix: [][]Ink{
			{Void, Void, Void, Void, Void},
			{Void, Glow, Glow, Glow, Void},
			{Void, Gold, Flame, Gold, Void},
			{Void, Ember, Ember, Ember, Void},
			{Void, Void, Void, Void, Void},
		}},
		Spans: []Span{{X: w/2 - 3, Y: 1, W: 6, S: " text ", At: 500 * time.Millisecond}},
	}
}

// fireFrames is the frames of c at times ts, each under a header.
func fireFrames(c Clip, ts ...time.Duration) string {
	var b strings.Builder
	for _, at := range ts {
		fmt.Fprintf(&b, "=== %v of %v\n%s", at, c.Len(), animtest.Grids(c.Frame(at)))
	}
	return b.String()
}

// TestFireGoldens holds the fire's frames at a time, at two sizes, in
// both glyph sets and under Mono: lit, climbing, at its height, and out.
// The fire is seeded, so a frame is a function of time.
func TestFireGoldens(t *testing.T) {
	ts := []time.Duration{0, 600 * time.Millisecond, 2 * time.Second, 2500 * time.Millisecond}
	for _, size := range [][2]int{{32, 10}, {60, 16}} {
		name := fmt.Sprintf("fire_%dx%d", size[0], size[1])
		animtest.Golden(t, name, fireFrames(testFire(size[0], size[1]).Clip(), ts...))
		withUI(ui.ASCII, ui.ANSI, func() {
			animtest.Golden(t, name+"_ascii", fireFrames(testFire(size[0], size[1]).Clip(), ts...))
		})
	}
	withUI(ui.Unicode, ui.Mono, func() {
		animtest.Golden(t, "fire_32x10_mono", fireFrames(testFire(32, 10).Clip(), ts...))
	})
}

// TestFireIsAFunctionOfTime: the same fire draws the same frame at the
// same time, in any order the frames are asked for; another seed burns
// another way.
func TestFireIsAFunctionOfTime(t *testing.T) {
	f := testFire(40, 12)
	a, b := f.Clip(), f.Clip()
	at := []time.Duration{time.Second, 300 * time.Millisecond, 0, 2 * time.Second, time.Second}
	for _, t1 := range at {
		b.Frame(2 * time.Second) // b is always further on, so it starts over
		if !slices.Equal(a.Frame(t1), b.Frame(t1)) {
			t.Errorf("two clips of one fire differ at %v", t1)
		}
	}
	g := f
	g.Seed++
	if slices.Equal(f.Clip().Frame(time.Second), g.Clip().Frame(time.Second)) {
		t.Error("another seed burns the same")
	}
	// Between two steps the frame holds, and Next is the next step.
	c := f.Clip()
	if !slices.Equal(c.Frame(FireStep), c.Frame(2*FireStep-1)) || c.Next(FireStep+1) != 2*FireStep {
		t.Error("the fire moves between steps")
	}
	if c.Next(c.Len()) != Never {
		t.Error("the fire ticks past its end")
	}
}

// fireCells is how many cells of a frame are flames: painted, or drawn
// in a shade, outside the art and the text.
func fireCells(f []string, art func(x, y int) bool) int {
	n := 0
	for y, l := range strings.Split(strings.SplitN(animtest.Grids(f), "\n-- fg --\n", 2)[0], "\n") {
		for x, r := range []rune(l) {
			if r != ' ' && !art(x, y) {
				n++
			}
		}
	}
	return n
}

// TestFireBurnsAndGoesOut: the flames climb from the bottom row, reach
// about as high as Reach says, and are out at the end.
func TestFireBurnsAndGoesOut(t *testing.T) {
	f := testFire(60, 16)
	f.Over, f.Spans, f.Reach = Sprite{}, nil, 8
	c := f.Clip()
	none := func(int, int) bool { return false }
	if n := fireCells(c.Frame(0), none); n != 60 {
		t.Errorf("at the start %d cells burn, want the bottom row's 60", n)
	}
	lit := c.Frame(2 * time.Second)
	// Reach is 8 pixel rows, 4 cell rows; the tongues of the columns that
	// cool slowest climb about twice as high, and none to the top half.
	if n := fireCells(lit[:8], none); n > 0 {
		t.Errorf("%d cells burn in the top half", n)
	}
	if n := fireCells(lit[12:], none); n < 60*3 {
		t.Errorf("only %d cells burn in the bottom four rows", n)
	}
	if n := fireCells(c.Frame(c.Len()), none); n != 0 {
		t.Errorf("%d cells still burn at the end", n)
	}
}

// TestFireArtAndText: the art covers the flames where it has ink, and
// the text shows from its time on, where the flames were.
func TestFireArtAndText(t *testing.T) {
	c := testFire(32, 10).Clip()
	f := strings.Split(animtest.Grids(c.Frame(2*time.Second)), "\n")
	// Pixel rows 3-7 are the art: cell row 1 holds its Void top (pixel
	// 3), row 2 its Glow over Gold and Flame.
	if got := string([]rune(f[2])[2:7]); got != " ▀▀▀ " {
		t.Errorf("art row 2 is %q", got)
	}
	fg := f[11:21]
	bg := f[22:32]
	if fg[2][3:6] != "WWW" || bg[2][3:6] != "yRy" {
		t.Errorf("art row 2 is fg %q over bg %q, want WWW over yRy", fg[2][3:6], bg[2][3:6])
	}
	// The text shows from the first step at or after its time.
	if strings.Contains(c.Frame(499 * time.Millisecond)[1], "text") {
		t.Error("the text shows before its time")
	}
	if !strings.Contains(c.Frame(500*time.Millisecond + FireStep)[1], " text ") {
		t.Error("the text does not show after its time")
	}
}

// TestFireHues: Mono paints no hue, and ASCII draws no rune outside
// ASCII, at any frame.
func TestFireHues(t *testing.T) {
	check := func(name string, bad func(string) bool) {
		c := testFire(40, 12).Clip()
		for at := time.Duration(0); at <= c.Len(); at += 100 * time.Millisecond {
			for _, l := range c.Frame(at) {
				if bad(l) {
					t.Errorf("%s at %v: %q", name, at, l)
					return
				}
			}
		}
	}
	withUI(ui.Unicode, ui.Mono, func() {
		check("mono", func(l string) bool { return strings.Contains(l, "\x1b[") })
	})
	withUI(ui.ASCII, ui.ANSI, func() {
		check("ascii", func(l string) bool {
			for _, r := range ansi.Strip(l) {
				if r > 0x7e {
					return true
				}
			}
			return false
		})
	})
	withUI(ui.ASCII, ui.Mono, func() {
		check("ascii mono", func(l string) bool { return strings.Contains(l, "\x1b[") })
	})
}
