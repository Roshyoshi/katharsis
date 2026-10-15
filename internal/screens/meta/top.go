package meta

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
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
	tab     int // tabTopics or tabRetention (retention.go)
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
	{"TIME+", func(a, b TopicStat) int { return cmp.Compare(b.Time, a.Time) }},
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
	case "r":
		t.toggleTab()
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
		out = append(out, ui.Fit(l, lw)+"  "+r)
	}
	out = append(out, t.alarmLine(w))

	tableH := max(h-len(out)-2, 0)
	t.h = tableH
	if t.tab == tabRetention {
		out = append(out, t.retention(w, tableH+1)...)
	} else {
		out = append(out, t.tableHeader(w))
		out = append(out, t.table(w, tableH)...)
	}
	return ui.Layout(w, h, out, t.keys())
}

func (t *Top) chart(w, h int) []string {
	vals := t.ratings
	title := ui.Focus.Render("Rating ") + ui.Rank(t.st.Rating).Render(fmt.Sprintf("%.0f", t.st.Rating))
	if len(vals) > 0 {
		peak := slices.Max(vals)
		title += ui.Muted.Render(fmt.Sprintf("  peak %.0f"+ui.G.Sep+"%d rated", peak, len(vals)))
	}
	if len(vals) < 2 {
		return []string{title, "", ui.Muted.Render("  not enough rated attempts for a trend")}
	}
	return append([]string{title}, axisChart(vals, w, h-1, ui.Rank)...)
}

// meters are htop's right-hand column: "Label: value" lines and one
// multi-colour bar for solves by kind.
func (t *Top) meters(w int) []string {
	st := t.st
	label := func(s string) string { return ui.Focus.Render(s + ": ") }
	val := func(s string) string { return ui.Strong.Render(s) }
	// counted is n of noun, the number a value: "1 crash", "3 crashes".
	counted := func(n int, noun string) string {
		s := units.Plural(n, noun)
		i := strings.IndexByte(s, ' ')
		return val(s[:i]) + s[i:]
	}

	crashes := 0
	for _, n := range st.Crashes {
		crashes += n
	}
	var causes []string
	for _, c := range []game.Outcome{game.TimedOut, game.OutOfLives, game.GaveUp} {
		causes = append(causes, val(fmt.Sprint(st.Crashes[c]))+" "+causeText(c))
	}

	ind, hint, ed := st.Solves[game.Independent], st.Solves[game.Hinted], st.Solves[game.Editorial]
	// Each kind has a glyph of its own, htop's monochrome bar characters in
	// order, so the parts read apart without color (R1).
	solves := []barPart{
		{ind, ui.OK, "|", "independent"}, {hint, ui.Warn, "#", "hinted"}, {ed, ui.Danger, "*", "editorial"},
	}

	delta := ""
	if n := len(t.ratings); n > 1 {
		d := st.Rating - t.ratings[max(n-11, 0)]
		switch {
		case d > 0:
			delta = ui.OK.Render(fmt.Sprintf(" ▲%.0f", d))
		case d < 0:
			delta = ui.Danger.Render(fmt.Sprintf(" ▼%.0f", -d))
		}
	}

	out := []string{
		label("Boots") + val(fmt.Sprint(st.Boots)) + ", " + counted(st.FullClears, "full clear") + ", " +
			counted(crashes, "crash"),
		label("Crashes") + strings.Join(causes, ", "),
		meterBar("Solves", w, solves, fmt.Sprint(ind+hint+ed)),
		legend(w, solves),
		label("TIME+") + val(units.Clock(st.TimePlus)),
		label("Bytes") + val(units.Bytes(st.LifetimeBytes)) + " lifetime, " + val(units.Bytes(st.Bank)) + " bank",
		label("Trend") + ui.Focus.Render(sparkline(t.ratings, max(min(w-20, 30), 0))) + delta + ui.Muted.Render(" last 10"),
	}
	// The engine's forecast against the boots, as calibration (D36): it is
	// never shown before a door.
	if e := st.EarlyCrash; e.Boots > 0 {
		out = append(out, label("Userland crash")+ui.G.Approx+val(fmt.Sprintf("%.0f%%", 100*e.Forecast))+" forecast, "+
			val(fmt.Sprint(e.Crashed))+fmt.Sprintf("/%d boots", e.Boots))
	}
	return out
}

// legend names a meter's parts, each by its glyph and count ("|88
// independent, #30 hinted"), indented under the bar when there is room.
func legend(w int, parts []barPart) string {
	items := make([]string, len(parts))
	for i, p := range parts {
		items[i] = p.style.Render(p.glyph+fmt.Sprint(p.n)) + " " + p.name
	}
	s := strings.Join(items, ", ")
	if indent := len("Solves["); ansi.StringWidth(s)+indent <= w {
		return strings.Repeat(" ", indent) + s
	}
	return s
}

// barPart is one part of a meter: its count, and the color, glyph and
// name it is drawn and named with.
type barPart struct {
	n           int
	style       ui.Token
	glyph, name string
}

// meterBar is an htop meter: "Name[|||||##**      text]", each part's
// share of the bar in its own color and glyph, the text right-aligned
// inside.
func meterBar(name string, w int, parts []barPart, text string) string {
	inner := w - len(name) - 2
	if inner < len(text)+1 {
		return ui.Focus.Render(name) + " " + text
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
		b.WriteString(p.style.Render(strings.Repeat(p.glyph, n)))
		used += n
	}
	fill := inner - used - len(text)
	return ui.Focus.Render(name) + ui.Strong.Render("[") + b.String() + strings.Repeat(" ", max(fill, 0)) +
		ui.Muted.Render(text) + ui.Strong.Render("]")
}

const (
	colTopic  = 17
	colNum    = 6
	colRating = 8
	colTime   = 10 // "  41:12:08": units.Clock, as every duration column
)

func (t *Top) tableHeader(w int) string {
	head := ui.FieldHeader // htop's green column header
	cells := []struct {
		text, key string
	}{
		{ui.Fit("  TOPIC", colTopic), "TOPIC"},
		{ui.AlignRight("ATT", colNum), "ATT"},
		{ui.AlignRight("CLR", colNum), ""},
		{ui.AlignRight("CLR%", colNum), "CLR%"},
		{ui.AlignRight("RATING", colRating), "RATING"},
		{ui.AlignRight("TIME+", colTime), "TIME+"},
		{"  CLEARS / ATTEMPTS", ""},
	}
	var b strings.Builder
	for _, c := range cells {
		st := head
		if c.key != "" && c.key == topSorts[t.sortBy].name {
			st = ui.Field
		}
		b.WriteString(st.Render(c.text))
	}
	return b.String() + head.Render(strings.Repeat(" ", max(w-ansi.StringWidth(b.String()), 0)))
}

func (t *Top) table(w, h int) []string {
	if len(t.rows) == 0 {
		return []string{ui.Muted.Render("  no attempts yet")}
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
	barW := w - colTopic - 3*colNum - colRating - colTime - 4

	var out []string
	for i := t.top; i < len(t.rows) && i < t.top+h; i++ {
		r := t.rows[i]
		rate := ui.Muted.Render(ui.AlignRight("-", colNum))
		if r.Attempts > 0 {
			rate = ui.AlignRight(fmt.Sprintf("%.0f%%", 100*clearRate(r)), colNum)
		}
		// 0 is a topic the engine keeps no rating for (mixed).
		rating := ui.Muted.Render(ui.AlignRight("-", colRating))
		if r.Rating > 0 {
			rating = ui.Rank(r.Rating).Render(ui.AlignRight(fmt.Sprintf("%.0f", r.Rating), colRating))
		}
		spent := ui.Muted.Render(ui.AlignRight("-", colTime))
		if r.Time > 0 {
			spent = ui.AlignRight(units.Clock(r.Time), colTime)
		}
		line := ui.Fit("  "+r.Topic, colTopic) +
			ui.AlignRight(fmt.Sprint(r.Attempts), colNum) +
			ui.AlignRight(fmt.Sprint(r.Clears), colNum) + rate + rating + spent +
			"  " + topicBar(r, maxAtt, barW)
		if i == t.sel {
			line = ui.Field.Render(ui.Fit(ansi.Strip(line), w))
		}
		out = append(out, line)
	}
	return out
}

// topicBar draws attempts to scale with the busiest topic: clears as | in
// OK, the rest as # in Danger, so both volume and success read at a
// glance, and the split survives without color and on the selected row
// (R1; htop's monochrome meters mark a second segment with # too).
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
	return ui.Strong.Render("[") + ui.OK.Render(strings.Repeat("|", clr)) +
		ui.Danger.Render(strings.Repeat("#", att-clr)) + strings.Repeat(" ", inner-att) + ui.Strong.Render("]")
}
