package anim

import (
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Para is a paragraph of plain text and the style it is drawn in.
type Para struct {
	Text  string
	Style lipgloss.Style
}

// Card is the boss card reveal. A double-bordered card W cells wide opens
// from its middle row outward, its name is stamped in a letter at a time,
// and its paragraphs fade in one after another. The name is letter-spaced
// and upper-cased when that fits, as on the boss door card.
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
	return []rune(ansi.Truncate(up, c.inner(), "…"))
}

// body is the card's rows inside the border: a blank, the name, then each
// paragraph after a blank, then a blank. Name rows are -1; paragraph rows
// carry their paragraph index.
type cardRow struct {
	para int // -1 for the name row, -2 for a blank
	text string
}

func (c Card) body() []cardRow {
	rows := []cardRow{{-2, ""}, {-1, ""}}
	for i, p := range c.Paras {
		rows = append(rows, cardRow{-2, ""})
		for _, l := range strings.Split(ansi.Wrap(p.Text, c.inner(), ""), "\n") {
			rows = append(rows, cardRow{i, l})
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

// fade is paragraph i's fade, over its share of the last phase.
func (c Card) fade(i int) Fade {
	from, to := c.at(cardStamp), c.D
	n := max(len(c.Paras), 1)
	slot := (to - from) / time.Duration(n)
	return Fade{From: Gone, To: Normal, D: slot, Salt: uint64(i) * 977}
}

func (c Card) paraLevel(i int, t time.Duration) Level {
	from := c.at(cardStamp) + c.fade(0).D*time.Duration(i)
	return c.fade(i).LevelAt(t - from)
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
	w := c.inner() + 2 // between the side walls
	out := make([]string, h)
	for r := range out {
		switch {
		case r < top || r > bot:
			out[r] = blank
		case r == top:
			out[r] = c.edge("╔", "╗", top == 0)
		case r == bot:
			out[r] = c.edge("╚", "╝", false)
		case t < c.at(cardOpen):
			out[r] = cardEdge.Render("║") + strings.Repeat(" ", w) + cardEdge.Render("║")
		default:
			out[r] = cardEdge.Render("║") + " " + c.row(body[r-1], t) + " " + cardEdge.Render("║")
		}
	}
	return out
}

// edge is a top or bottom border row; the top one carries the label.
func (c Card) edge(l, r string, label bool) string {
	run := c.W - 2
	if !label || c.Label == "" {
		return cardEdge.Render(l + strings.Repeat("═", run) + r)
	}
	lab := " " + ansi.Truncate(c.Label, max(run-4, 0), "…") + " "
	rest := max(run-2-ansi.StringWidth(lab), 0)
	return cardEdge.Render(l+"══") + cardLabel.Render(lab) + cardEdge.Render(strings.Repeat("═", rest)+r)
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
	for i := range c.Paras {
		start := c.at(cardStamp) + c.fade(0).D*time.Duration(i)
		f := c.fade(i)
		for t := time.Duration(0); t < f.D; t = f.Next(t) {
			ms = append(ms, start+f.Next(t))
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
