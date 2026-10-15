package anim

import (
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/ui"
)

// Para is a paragraph of plain text and the style it is drawn in. A "\n"
// in Text starts a new line, for verse.
type Para struct {
	Text  string
	Style lipgloss.Style
}

// Card is the boss card reveal. A double-bordered card W cells wide opens
// from its middle row outward, its name is stamped in a letter at a time,
// and its paragraphs fade in one after another. The name is letter-spaced
// and upper-cased when that fits, as on the boss door card. The border is
// ui.G.Double, the boss frame, read as each frame is drawn, so the ASCII
// glyph set draws it in ASCII.
type Card struct {
	Label     string // in the top edge, e.g. "⚔ BOSS"
	Name      string
	NameStyle lipgloss.Style
	Paras     []Para
	W         int
	D         time.Duration
}

// The card's phases, as fractions of D: open, then stamp the name, then
// fade the paragraphs in.
const (
	cardOpen  = 0.25
	cardStamp = 0.55
)

// inner is the text width inside the border and its one-cell padding.
func (c Card) inner() int { return max(c.W-4, 8) }

// name is the stamped name: spaced when it fits.
func (c Card) name() []rune {
	up := strings.ToUpper(c.Name)
	if sp := strings.Join(strings.Split(up, ""), " "); ansi.StringWidth(sp) <= c.inner() {
		return []rune(sp)
	}
	return []rune(ansi.Truncate(up, c.inner(), ui.G.Ellipsis))
}

// cardRow is one row inside the border: a paragraph's line, the name or a
// blank.
type cardRow struct {
	para int // the paragraph's index, -1 for the name row, -2 for a blank
	text string
}

// body is the card's rows inside the border: a blank, the name, then each
// paragraph after a blank, then a blank.
func (c Card) body() []cardRow {
	rows := []cardRow{{-2, ""}, {-1, ""}}
	for i, p := range c.Paras {
		rows = append(rows, cardRow{-2, ""})
		for _, line := range strings.Split(p.Text, "\n") { // a poem keeps its lines
			for _, l := range strings.Split(ansi.Wrap(line, c.inner(), ""), "\n") {
				rows = append(rows, cardRow{i, l})
			}
		}
	}
	return append(rows, cardRow{-2, ""})
}

// height is the card's rows, border included.
func (c Card) height() int { return len(c.body()) + 2 }

func (c Card) at(f float64) time.Duration { return time.Duration(f * float64(c.D)) }

// band is how many rows either side of the middle are open at t.
func (c Card) band(t time.Duration) int {
	half := c.height() / 2
	open := c.at(cardOpen)
	if t >= open || open <= 0 {
		return half
	}
	return min(int(int64(max(t, 0))*int64(half+1)/int64(open)), half)
}

// stamped is how many runes of the name are in at t.
func (c Card) stamped(t time.Duration) int {
	n := len(c.name())
	from, to := c.at(cardOpen), c.at(cardStamp)
	switch {
	case t >= to:
		return n
	case t < from:
		return 0
	}
	return min(int(int64(t-from)*int64(n+1)/int64(to-from)), n)
}

// fade is a paragraph's fade: the last phase, split evenly between them.
func (c Card) fade() Fade {
	n := max(len(c.Paras), 1)
	return Fade{From: Gone, To: Normal, D: (c.D - c.at(cardStamp)) / time.Duration(n)}
}

// paraStart is when paragraph i starts to fade in.
func (c Card) paraStart(i int) time.Duration {
	return c.at(cardStamp) + c.fade().D*time.Duration(i)
}

func (c Card) paraLevel(i int, t time.Duration) Level {
	return c.fade().LevelAt(t - c.paraStart(i))
}

func (c Card) Len() time.Duration { return c.D }

func (c Card) Frame(t time.Duration) []string {
	body := c.body()
	h := len(body) + 2
	mid, k := (h-1)/2, c.band(t)
	top, bot := max(mid-k, 0), min(mid+k+(h-1)%2, h-1)
	if t >= c.at(cardOpen) {
		top, bot = 0, h-1
	}
	blank := strings.Repeat(" ", c.W)
	b := ui.G.Double
	w := c.inner() + 2 // between the side walls
	out := make([]string, h)
	for r := range out {
		switch {
		case r < top || r > bot:
			out[r] = blank
		case r == top:
			out[r] = c.edge(b.TopLeft, b.Top, b.TopRight, top == 0)
		case r == bot:
			out[r] = c.edge(b.BottomLeft, b.Bottom, b.BottomRight, false)
		case t < c.at(cardOpen):
			out[r] = cardEdge.Render(b.Left) + strings.Repeat(" ", w) + cardEdge.Render(b.Right)
		default:
			out[r] = cardEdge.Render(b.Left) + " " + c.row(body[r-1], t) + " " + cardEdge.Render(b.Right)
		}
	}
	return out
}

// edge is a top or bottom border row: corners l and r around runs of h.
// The top one carries the label.
func (c Card) edge(l, h, r string, label bool) string {
	run := c.W - 2
	if !label || c.Label == "" {
		return cardEdge.Render(l + strings.Repeat(h, run) + r)
	}
	lab := " " + ansi.Truncate(c.Label, max(run-4, 0), ui.G.Ellipsis) + " "
	rest := max(run-2-ansi.StringWidth(lab), 0)
	return cardEdge.Render(l + strings.Repeat(h, 2) + lab + strings.Repeat(h, rest) + r)
}

// row is one body row at t, exactly inner() wide.
func (c Card) row(r cardRow, t time.Duration) string {
	iw := c.inner()
	switch r.para {
	case -2:
		return strings.Repeat(" ", iw)
	case -1:
		name := c.name()
		k := c.stamped(t)
		lead := (iw - len(name)) / 2
		shown := paint(string(name), func(i int) Level {
			if i < k {
				return Normal
			}
			return Gone
		}, c.NameStyle, 0)
		return strings.Repeat(" ", lead) + shown + strings.Repeat(" ", iw-lead-len(name))
	}
	p := c.Paras[r.para]
	s := Shade(r.text, c.paraLevel(r.para, t), p.Style, uint64(r.para)*977+uint64(len(r.text)))
	return s + strings.Repeat(" ", max(iw-ansi.StringWidth(r.text), 0))
}

// marks is every time the frame changes.
func (c Card) marks() []time.Duration {
	var ms []time.Duration
	half := c.height() / 2
	open := c.at(cardOpen)
	for k := 1; k <= half; k++ {
		ms = append(ms, time.Duration((int64(open)*int64(k)+int64(half))/int64(half+1)))
	}
	ms = append(ms, open)
	n := len(c.name())
	from, to := open, c.at(cardStamp)
	for k := 1; k <= n; k++ {
		ms = append(ms, from+time.Duration((int64(to-from)*int64(k)+int64(n))/int64(n+1)))
	}
	f := c.fade()
	for i := range c.Paras {
		for t := time.Duration(0); t < f.D; t = f.Next(t) {
			ms = append(ms, c.paraStart(i)+f.Next(t))
		}
	}
	slices.Sort(ms)
	return ms
}

func (c Card) Next(t time.Duration) time.Duration {
	for _, m := range c.marks() {
		if m > t {
			return m
		}
	}
	return Never
}
