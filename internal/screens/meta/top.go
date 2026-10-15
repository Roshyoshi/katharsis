package meta

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
)

// Top is the stats screen, laid out like htop: meters on top (a braille
// rating chart on the left, totals on the right), then a per-topic table
// with a selection bar, then the function-key bar.
type Top struct {
	st      Stats
	ratings []float64 // st.RatingHistory's ratings, oldest first
	sortBy  int       // index into topSorts
	rows    []TopicStat
	sel     int
	top     int
	h       int // table rows at the last View, for paging
}

type topSort struct {
	name string
	cmp  func(a, b TopicStat) int
}

var topSorts = []topSort{
	{"RATING", func(a, b TopicStat) int { return cmp.Compare(b.Rating, a.Rating) }},
	{"ATT", func(a, b TopicStat) int { return cmp.Compare(b.Attempts, a.Attempts) }},
	{"CLR%", func(a, b TopicStat) int { return cmp.Compare(clearRate(b), clearRate(a)) }},
	{"TOPIC", func(a, b TopicStat) int { return cmp.Compare(a.Topic, b.Topic) }},
}

func clearRate(t TopicStat) float64 {
	if t.Attempts == 0 {
		return -1 // untried sorts below 0%
	}
	return float64(t.Clears) / float64(t.Attempts)
}

func NewTop(st Stats) *Top {
	t := &Top{st: st}
	for _, p := range st.RatingHistory {
		t.ratings = append(t.ratings, p.Rating)
	}
	t.sort()
	return t
}

func (t *Top) sort() {
	t.rows = slices.Clone(t.st.Topics)
	by := topSorts[t.sortBy].cmp
	slices.SortStableFunc(t.rows, func(a, b TopicStat) int {
		if c := by(a, b); c != 0 {
			return c
		}
		return cmp.Compare(a.Topic, b.Topic)
	})
}

func (t *Top) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return t, nil
	}
	last := max(len(t.rows)-1, 0)
	switch k.String() {
	case "q", "esc", "f10":
		return t, screens.Emit(screens.Back{})
	case "up", "k":
		t.sel = max(t.sel-1, 0)
	case "down", "j":
		t.sel = min(t.sel+1, last)
	case "pgup", "b":
		t.sel = max(t.sel-max(t.h, 1), 0)
	case "pgdown", "space":
		t.sel = min(t.sel+max(t.h, 1), last)
	case "home", "g":
		t.sel = 0
	case "end", "G":
		t.sel = last
	case "s", "f6", ">":
		t.sortBy = (t.sortBy + 1) % len(topSorts)
		t.sort()
	}
	return t, nil
}

func (t *Top) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	// Meters take what the table does not need, between the 8 rows the
	// totals column needs and half the screen.
	meterH := min(max(h-len(t.rows)-4, 8), 16, max(h/2, 8))
	if h < 12 {
		meterH = max(h-4, 1)
	}
	// The chart gets a little under half: the totals beside it are text
	// that truncates badly, the chart only gets coarser.
	lw := w * 9 / 20
	rw := w - lw - 2
	left := t.chart(lw, meterH)
	right := t.meters(rw)

	var out []string
	for i := range meterH {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		out = append(out, pad(l, lw)+"  "+r)
	}
	out = append(out, "")

	tableH := max(h-len(out)-2, 0)
	t.h = tableH
	out = append(out, t.tableHeader(w))
	out = append(out, t.table(w, tableH)...)
	for len(out) < h-1 {
		out = append(out, "")
	}
	out = append(out[:h-1], keyBar("j/k", "Move  ", "s", "Sort "+topSorts[t.sortBy].name+" ", "q", "Back  "))
	return frame(out, w, h)
}

func (t *Top) chart(w, h int) []string {
	vals := t.ratings
	title := cyan.Render("Rating ") + ratingStyle(t.st.Rating).Bold(true).Render(fmt.Sprintf("%.0f", t.st.Rating))
	if len(vals) > 0 {
		peak := slices.Max(vals)
		title += faint.Render(fmt.Sprintf("  peak %.0f · %d rated", peak, len(vals)))
	}
	if len(vals) < 2 {
		return []string{title, "", faint.Render("  not enough rated attempts for a trend")}
	}
	return append([]string{title}, axisChart(vals, w, h-1, ratingStyle)...)
}

// meters are htop's right-hand column: "Label: value" lines and one
// multi-colour bar for solves by kind.
func (t *Top) meters(w int) []string {
	st := t.st
	label := func(s string) string { return cyan.Render(s + ": ") }
	val := func(s string) string { return bold.Render(s) }

	crashes := 0
	for _, n := range st.Crashes {
		crashes += n
	}
	var causes []string
	for _, c := range []game.Outcome{game.TimedOut, game.OutOfLives, game.GaveUp} {
		causes = append(causes, val(fmt.Sprint(st.Crashes[c]))+" "+causeText(c))
	}

	ind, hint, ed := st.Solves[game.Independent], st.Solves[game.Hinted], st.Solves[game.Editorial]
	solveBar := meterBar("Solves", w, []barPart{
		{ind, green}, {hint, yellow}, {ed, red},
	}, fmt.Sprint(ind+hint+ed))

	delta := ""
	if n := len(t.ratings); n > 1 {
		d := st.Rating - t.ratings[max(n-11, 0)]
		switch {
		case d > 0:
			delta = green.Render(fmt.Sprintf(" ▲%.0f", d))
		case d < 0:
			delta = red.Render(fmt.Sprintf(" ▼%.0f", -d))
		}
	}

	return []string{
		label("Boots") + val(fmt.Sprint(st.Boots)) + ", " + val(fmt.Sprint(st.FullClears)) +
			" full clears, " + val(fmt.Sprint(crashes)) + " crashes",
		label("Crashes") + strings.Join(causes, ", "),
		solveBar,
		legend(w, ind, hint, ed),
		label("Uptime") + val(uptime(st.Uptime)),
		label("Bytes") + val(Bytes(st.LifetimeBytes)) + " lifetime, " + val(Bytes(st.Bank)) + " bank",
		label("Trend") + cyan.Render(sparkline(t.ratings, max(min(w-20, 30), 0))) + delta + faint.Render(" last 10"),
	}
}

// legend explains the solves bar's colours, indented under the bar when
// there is room.
func legend(w, ind, hint, ed int) string {
	s := green.Render(fmt.Sprint(ind)) + " independent · " +
		yellow.Render(fmt.Sprint(hint)) + " hinted · " + red.Render(fmt.Sprint(ed)) + " editorial"
	if indent := len("Solves["); ansi.StringWidth(s)+indent <= w {
		return strings.Repeat(" ", indent) + s
	}
	return s
}

type barPart struct {
	n     int
	style lipgloss.Style
}

// meterBar is an htop meter: "Name[|||||||||      text]", each part's
// share of the bar in its own colour, the text right-aligned inside.
func meterBar(name string, w int, parts []barPart, text string) string {
	inner := w - len(name) - 2
	if inner < len(text)+1 {
		return cyan.Render(name) + " " + text
	}
	total := 0
	for _, p := range parts {
		total += p.n
	}
	var b strings.Builder
	sum, used := 0, 0
	for _, p := range parts {
		n := 0
		if total > 0 {
			// Round the running sum so the parts add up to the filled width.
			sum += p.n
			n = (sum*(inner-len(text)-1)+total/2)/total - used
		}
		b.WriteString(p.style.Render(strings.Repeat("|", n)))
		used += n
	}
	fill := inner - used - len(text)
	return cyan.Render(name) + bold.Render("[") + b.String() + strings.Repeat(" ", max(fill, 0)) +
		faint.Render(text) + bold.Render("]")
}

const (
	colTopic  = 17
	colNum    = 6
	colRating = 8
)

func (t *Top) tableHeader(w int) string {
	head := lipgloss.NewStyle().Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0"))
	cells := []struct {
		text, key string
	}{
		{pad("  TOPIC", colTopic), "TOPIC"},
		{padLeft("ATT", colNum), "ATT"},
		{padLeft("CLR", colNum), ""},
		{padLeft("CLR%", colNum), "CLR%"},
		{padLeft("RATING", colRating), "RATING"},
		{"  CLEARS / ATTEMPTS", ""},
	}
	var b strings.Builder
	for _, c := range cells {
		st := head
		if c.key != "" && c.key == topSorts[t.sortBy].name {
			st = field
		}
		b.WriteString(st.Render(c.text))
	}
	return b.String() + head.Render(strings.Repeat(" ", max(w-ansi.StringWidth(b.String()), 0)))
}

func (t *Top) table(w, h int) []string {
	if len(t.rows) == 0 {
		return []string{faint.Render("  no attempts yet")}
	}
	if t.sel < t.top {
		t.top = t.sel
	}
	if t.sel >= t.top+h {
		t.top = t.sel - h + 1
	}
	t.top = max(min(t.top, len(t.rows)-h), 0)

	maxAtt := 1
	for _, r := range t.rows {
		maxAtt = max(maxAtt, r.Attempts)
	}
	barW := w - colTopic - 3*colNum - colRating - 4

	var out []string
	for i := t.top; i < len(t.rows) && i < t.top+h; i++ {
		r := t.rows[i]
		rate := faint.Render(padLeft("-", colNum))
		if r.Attempts > 0 {
			rate = padLeft(fmt.Sprintf("%.0f%%", 100*clearRate(r)), colNum)
		}
		line := pad("  "+r.Topic, colTopic) +
			padLeft(fmt.Sprint(r.Attempts), colNum) +
			padLeft(fmt.Sprint(r.Clears), colNum) + rate +
			ratingStyle(r.Rating).Render(padLeft(fmt.Sprintf("%.0f", r.Rating), colRating)) +
			"  " + topicBar(r, maxAtt, barW)
		if i == t.sel {
			line = field.Render(pad(ansi.Strip(line), w))
		}
		out = append(out, line)
	}
	return out
}

// topicBar draws attempts to scale with the busiest topic: clears in green,
// the rest in red, so both volume and success read at a glance.
func topicBar(r TopicStat, maxAtt, w int) string {
	if w < 4 {
		return ""
	}
	inner := w - 2
	att := (r.Attempts*inner + maxAtt/2) / maxAtt
	clr := 0
	if r.Attempts > 0 {
		clr = (r.Clears*att + r.Attempts/2) / r.Attempts
	}
	return bold.Render("[") + green.Render(strings.Repeat("|", clr)) +
		red.Render(strings.Repeat("|", att-clr)) + strings.Repeat(" ", inner-att) + bold.Render("]")
}
