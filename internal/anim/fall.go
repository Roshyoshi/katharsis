package anim

import (
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Fall is the rabbit-hole transition: you drop out of one screen, down a
// shaft lined with cupboards and shelves, and land on the next. The From
// screen slides up and away, the shaft goes by, and To rises into place,
// slowly at first and last, the way Alice fell: "either the well was very
// deep, or she fell very slowly".
//
// From and To are rendered screens, H rows of W cells (shorter ones are
// padded with blank rows). Label, if set, is painted on the shaft wall
// halfway down: where you are falling to.
type Fall struct {
	From, To []string
	W, H     int
	D        time.Duration
	Label    string
	Salt     uint64 // which furniture lines the shaft
}

// span is how many rows the window scrolls: the strip less one screen.
func (f Fall) span() int {
	h := max(f.H, 1)
	return 2*h + h/2
}

// rows is the strip the window scrolls over: From, the shaft, To.
func (f Fall) rows() []string {
	h := max(f.H, 1)
	out := make([]string, 0, 3*h)
	out = append(out, pad(f.From, f.W, h)...)
	out = append(out, pad(Shaft(f.W, h+h/2, f.Label, f.Salt), f.W, h+h/2)...)
	return append(out, pad(f.To, f.W, h)...)
}

// pad is ls cut or padded to n rows, each cut or padded to w cells.
func pad(ls []string, w, n int) []string {
	out := make([]string, n)
	for i := range out {
		if i < len(ls) {
			out[i] = ansi.Truncate(ls[i], w, "")
		}
		out[i] += strings.Repeat(" ", max(w-ansi.StringWidth(out[i]), 0))
	}
	return out
}

// offset is how far the window has scrolled at t, in rows: an
// ease-in-out over the strip.
func (f Fall) offset(t time.Duration, total int) int {
	if t >= f.D || f.D <= 0 {
		return total
	}
	if t <= 0 {
		return 0
	}
	x := float64(t) / float64(f.D)
	e := x * x * (3 - 2*x) // smoothstep
	return min(int(e*float64(total)), total)
}

func (f Fall) Len() time.Duration { return f.D }

func (f Fall) Frame(t time.Duration) []string {
	rows := f.rows()
	h := max(f.H, 1)
	o := f.offset(t, f.span())
	return rows[o : o+h]
}

func (f Fall) Next(t time.Duration) time.Duration {
	if t >= f.D {
		return Never
	}
	total := f.span()
	cur := f.offset(t, total)
	// The offset only grows, so the first change is a binary search away.
	lo, hi := t, f.D
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if f.offset(mid, total) > cur {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// furniture is what Alice passed on the way down (Alice, ch. I): plain
// glyphs, nothing wider than a cell.
var furniture = []string{
	"|=|=|=|=|  book-shelves",
	"[ cupboard ]",
	"( ORANGE MARMALADE )",
	"~ a map, on a peg ~",
	"[ a picture, on a peg ]",
	"|=|=|=|",
}

// Shaft is n rows of the rabbit-hole, w cells wide: two walls and, every
// few rows, something on a shelf. label, if set, goes on the middle row.
func Shaft(w, n int, label string, salt uint64) []string {
	inner := max(min(w-2, 44), 4)
	left := max((w-inner-2)/2, 0)
	out := make([]string, n)
	for i := range out {
		var mid string
		switch {
		case label != "" && i == n/2:
			mid = "· " + label + " ·"
		case i%4 == 1:
			mid = furniture[mix(salt+uint64(i))%uint64(len(furniture))]
		}
		mid = ansi.Truncate(mid, inner, "")
		gap := inner - ansi.StringWidth(mid)
		// Furniture sits against a wall, alternating sides; the label is
		// centered.
		lpad := gap / 2
		if mid != "" && i != n/2 {
			lpad = 1
			if mix(salt^uint64(i)<<7)%2 == 0 {
				lpad = max(gap-1, 0)
			}
		}
		row := strings.Repeat(" ", left) + wall.Render("|") +
			strings.Repeat(" ", lpad) + mid + strings.Repeat(" ", gap-lpad) + wall.Render("|")
		out[i] = row
	}
	return out
}
