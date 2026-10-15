package anim

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Level is a step of a fade. A terminal has no alpha, only the steps it
// can draw: nothing, a scatter of faint cells, faint, normal and bold.
type Level uint8

const (
	Gone   Level = iota // blank cells
	Dust                // about a third of the cells, faint
	Faint               // faint
	Normal              // the base style
	Bold                // the base style, bold
)

// Shade draws the plain line s at level l over base, as wide as s: a
// hidden cell is a space. salt picks which cells Dust keeps, so one line
// dissolves the same way every time.
func Shade(s string, l Level, base lipgloss.Style, salt uint64) string {
	return paint(s, func(int) Level { return l }, base, salt)
}

// paint draws the plain line s with rune i at level at(i), keeping each
// cell's width.
func paint(s string, at func(i int) Level, base lipgloss.Style, salt uint64) string {
	var b, run strings.Builder
	cur := Level(255)
	flush := func() {
		if run.Len() > 0 {
			b.WriteString(shade(base, cur).Render(run.String()))
			run.Reset()
		}
	}
	for i, r := range []rune(s) {
		l := at(i)
		if l == Dust && !dusty(salt, i) {
			l = Gone
		}
		if l == Gone {
			flush()
			cur = Gone
			b.WriteString(strings.Repeat(" ", ansi.StringWidth(string(r))))
			continue
		}
		if l != cur {
			flush()
			cur = l
		}
		run.WriteRune(r)
	}
	flush()
	return b.String()
}

// dusty reports whether cell i keeps its rune at Dust: about one in three.
func dusty(salt uint64, i int) bool { return mix(salt^uint64(i)*0x9e3779b97f4a7c15)%3 == 0 }

// mix is splitmix64's finalizer: a cheap, well-spread hash.
func mix(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	return x ^ x>>31
}

// Hold is lines, unchanged, for d: a pause, or text that is just there.
type Hold struct {
	Lines []string
	D     time.Duration
}

func (h Hold) Len() time.Duration               { return h.D }
func (h Hold) Frame(time.Duration) []string     { return h.Lines }
func (h Hold) Next(time.Duration) time.Duration { return Never }

// DefaultCPS is a Typewriter's speed when CPS is 0, in runes per second.
const DefaultCPS = 75

// Typewriter types plain lines out one rune at a time, pausing a beat at
// the end of a sentence, a clause and a line, as someone reading aloud
// would. Untyped cells are blank, so each line keeps its width.
type Typewriter struct {
	Lines []string
	CPS   float64
	Style lipgloss.Style
}

// times is when each rune of each line appears.
func (w Typewriter) times() [][]time.Duration {
	cps := w.CPS
	if cps <= 0 {
		cps = DefaultCPS
	}
	per := time.Duration(float64(time.Second) / cps)
	out := make([][]time.Duration, len(w.Lines))
	at := time.Duration(0)
	for i, l := range w.Lines {
		rs := []rune(l)
		out[i] = make([]time.Duration, len(rs))
		for j, r := range rs {
			at += per
			out[i][j] = at
			if j+1 < len(rs) && rs[j+1] != ' ' {
				continue // a pause needs the word to end
			}
			switch r {
			case '.', '!', '?':
				at += 6 * per
			case ',', ';', ':', '—':
				at += 3 * per
			}
		}
		at += 5 * per // the end of a line
	}
	return out
}

func (w Typewriter) Len() time.Duration {
	ts := w.times()
	for i := len(ts) - 1; i >= 0; i-- {
		if n := len(ts[i]); n > 0 {
			return ts[i][n-1]
		}
	}
	return 0
}

func (w Typewriter) Frame(t time.Duration) []string {
	ts := w.times()
	out := make([]string, len(w.Lines))
	for i, l := range w.Lines {
		out[i] = paint(l, func(j int) Level {
			if ts[i][j] <= t {
				return Normal
			}
			return Gone
		}, w.Style, 0)
	}
	return out
}

func (w Typewriter) Next(t time.Duration) time.Duration {
	for _, line := range w.times() {
		for _, at := range line {
			if at > t {
				return at
			}
		}
	}
	return Never
}

// Reveal prints lines one at a time, top-down, over Total: the crash
// screen's reveal. Line i shows at Total·(i+1)/n.
type Reveal struct {
	Lines []string
	Total time.Duration
	Style lipgloss.Style
}

// Shown is how many lines are out at t.
func (r Reveal) Shown(t time.Duration) int {
	n := len(r.Lines)
	if n == 0 || t >= r.Total {
		return n
	}
	if t <= 0 {
		return 0
	}
	return min(int(int64(t)*int64(n)/int64(r.Total)), n)
}

func (r Reveal) Len() time.Duration { return r.Total }

func (r Reveal) Frame(t time.Duration) []string {
	k := r.Shown(t)
	out := make([]string, len(r.Lines))
	for i, l := range r.Lines {
		lv := Normal
		if i >= k {
			lv = Gone
		}
		out[i] = Shade(l, lv, r.Style, 0)
	}
	return out
}

func (r Reveal) Next(t time.Duration) time.Duration {
	n := len(r.Lines)
	k := r.Shown(t)
	if n == 0 || k >= n {
		return Never
	}
	// The first time Shown reaches k+1: ceil(Total·(k+1)/n).
	return time.Duration((int64(r.Total)*int64(k+1) + int64(n) - 1) / int64(n))
}

// Fade steps plain lines from one Level to another, evenly over D: Gone to
// Normal fades in, Normal to Gone fades out, Normal to Bold flares.
type Fade struct {
	Lines    []string
	From, To Level
	D        time.Duration
	Style    lipgloss.Style
	Salt     uint64 // which cells Dust keeps
}

// steps is the number of level changes.
func (f Fade) steps() int {
	if f.To > f.From {
		return int(f.To - f.From)
	}
	return int(f.From - f.To)
}

// LevelAt is the fade's level at t.
func (f Fade) LevelAt(t time.Duration) Level {
	n := f.steps()
	if t >= f.D || n == 0 {
		return f.To
	}
	if t <= 0 {
		return f.From
	}
	k := int(int64(t) * int64(n) / int64(f.D))
	if f.To > f.From {
		return f.From + Level(k)
	}
	return f.From - Level(k)
}

func (f Fade) Len() time.Duration { return f.D }

func (f Fade) Frame(t time.Duration) []string {
	l := f.LevelAt(t)
	out := make([]string, len(f.Lines))
	for i, s := range f.Lines {
		out[i] = Shade(s, l, f.Style, f.Salt+uint64(i)*0x632be59bd9b4e019)
	}
	return out
}

func (f Fade) Next(t time.Duration) time.Duration {
	n := f.steps()
	if n == 0 || t >= f.D {
		return Never
	}
	k := int64(0)
	if t > 0 {
		k = int64(t) * int64(n) / int64(f.D)
	}
	return time.Duration((int64(f.D)*(k+1) + int64(n) - 1) / int64(n))
}

// Wipe uncovers lines left to right, a column at a time, over D. The lines
// may be styled already: a cut keeps each cell's style.
type Wipe struct {
	Lines []string
	D     time.Duration
}

// width is the widest line, in cells.
func (w Wipe) width() int {
	n := 0
	for _, l := range w.Lines {
		n = max(n, ansi.StringWidth(l))
	}
	return n
}

// cols is how many columns are uncovered at t.
func (w Wipe) cols(t time.Duration) int {
	n := w.width()
	if t >= w.D || w.D <= 0 {
		return n
	}
	return min(int(int64(max(t, 0))*int64(n+1)/int64(w.D)), n)
}

func (w Wipe) Len() time.Duration { return w.D }

func (w Wipe) Frame(t time.Duration) []string {
	k := w.cols(t)
	out := make([]string, len(w.Lines))
	for i, l := range w.Lines {
		lw := ansi.StringWidth(l)
		if k >= lw {
			out[i] = l
			continue
		}
		cut := ansi.Truncate(l, k, "")
		out[i] = cut + strings.Repeat(" ", lw-ansi.StringWidth(cut))
	}
	return out
}

func (w Wipe) Next(t time.Duration) time.Duration {
	n := w.width()
	k := w.cols(t)
	if k >= n {
		return Never
	}
	return time.Duration((int64(w.D)*int64(k+1) + int64(n)) / int64(n+1))
}
