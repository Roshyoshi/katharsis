package meta

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Calendar is git log's header: TIME+ per day over the last year, a
// column per week from Monday and a row per weekday, each day shaded by
// its time inside doors, the way GitHub draws contributions. It shows when
// play happened and counts no days in a row (GR1): a day without play is
// an empty cell, and nothing marks today (GR2).
type Calendar struct {
	Today time.Time   // local; the last column is its week
	Days  []Day       // any order; a day not listed had no play
	Load  *[3]float64 // the load average (RT9); nil leaves it out
}

// Day is one local day's play, as the store counts it: door time cut at
// midnight, each boot, clear, crash and upsolve on the day it happened.
type Day struct {
	Date                             time.Time     // local midnight
	Door                             time.Duration // TIME+ spent this day
	Boots, Clears, Crashes, Upsolves int
}

// calendarWeeks is a year of weeks: 52 whole ones and Today's.
const calendarWeeks = 53

// CalendarStart is the first day a calendar ending on today draws: the
// Monday 52 weeks before today's week's. The app reads days from there.
func CalendarStart(today time.Time) time.Time {
	return weekStart(today).AddDate(0, 0, -7*(calendarWeeks-1))
}

// weekStart is midnight on the Monday of t's week.
func weekStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d-(int(t.Weekday())+6)%7, 0, 0, 0, 0, t.Location())
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func dateKey(t time.Time) string { return t.Format(time.DateOnly) }

// calendar is a Calendar as git log draws it, with its cursor.
type calendar struct {
	Calendar
	byDate map[string]Day  // by dateKey
	door   []time.Duration // the year's door times above 0, ascending
	active int             // days with door time in the last 365
	focus  bool            // keys move the cursor, not the list
	at     time.Time       // the cursor's day
	first  time.Time       // the first day drawn, as of the last View
}

func newCalendar(c Calendar) *calendar {
	cal := &calendar{Calendar: c, byDate: map[string]Day{}, first: CalendarStart(c.Today), at: midnight(c.Today)}
	for _, d := range c.Days {
		cal.byDate[dateKey(d.Date)] = d
	}
	for day := cal.first; !day.After(c.Today); day = day.AddDate(0, 0, 1) {
		if d := cal.byDate[dateKey(day)].Door; d > 0 {
			cal.door = append(cal.door, d)
		}
	}
	slices.Sort(cal.door)
	for i := range 365 {
		if cal.byDate[dateKey(c.Today.AddDate(0, 0, -i))].Door > 0 {
			cal.active++
		}
	}
	return cal
}

// heat is a day's shade: 0 without door time, else 1 to 4 by the quarter
// its time ranks in among the year's days with any. The shades follow the
// player's own days, not a fixed target.
func (c *calendar) heat(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	atMost, _ := slices.BinarySearch(c.door, d+1)
	n := len(c.door)
	return max((4*atMost+n-1)/n, 1)
}

// ---- keys --------------------------------------------------------------------

// enter puts the cursor on at's day, or on today when at is zero or older
// than the calendar.
func (c *calendar) enter(at time.Time) {
	c.focus, c.at = true, midnight(c.Today)
	if at.IsZero() {
		return
	}
	if d := midnight(at.In(c.Today.Location())); !d.Before(c.first) && !d.After(c.Today) {
		c.at = d
	}
}

// key acts on a key while the calendar has focus: hjkl and the arrows
// move a week across or a day down, g/G go to the ends, c and esc give the
// keys back to the list.
func (c *calendar) key(k string) {
	switch k {
	case "left", "h":
		c.move(-7)
	case "right", "l":
		c.move(7)
	case "up", "k":
		c.move(-1)
	case "down", "j":
		c.move(1)
	case "home", "g":
		c.at = c.first
	case "end", "G":
		c.at = midnight(c.Today)
	case "esc", "c":
		c.focus = false
	}
}

// move steps the cursor n days, staying on the drawn days.
func (c *calendar) move(n int) {
	if at := c.at.AddDate(0, 0, n); !at.Before(c.first) && !at.After(c.Today) {
		c.at = at
	}
}

// ---- drawing -----------------------------------------------------------------

const (
	dayLabelW = len("Mon ")
	legendW   = len("less ") + 4 + len(" more")
	// monthsOver is how far a month's label may run past the last column,
	// so the newest month keeps its label.
	monthsOver = len("Jan") - 1
	// minLogRows is the boots git log keeps under its calendar; with less
	// room the calendar steps aside until c asks for it.
	minLogRows = 5
)

var weekdays = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// gridWidth is the width of weeks columns of cell cells (2: a glyph and a
// gap, 1: the glyph alone) after the weekday labels.
func gridWidth(weeks, cell int) int { return dayLabelW + weeks*cell - (cell - 1) }

// lines is the calendar for w columns, ending in a blank row: the year two
// cells a day where it fits with its legend; with less width, the last
// weeks that fit beside the summary, or, under focus, the year one cell a
// day. Out of focus it steps aside when the list would keep fewer than
// minLogRows of room rows.
func (c *calendar) lines(w, room int) []string {
	var ls []string
	switch {
	case gridWidth(calendarWeeks, 2)+2+legendW <= w:
		ls = c.year(w, calendarWeeks, 2)
	case c.focus:
		ls = c.year(w, min(calendarWeeks, max(w-dayLabelW-monthsOver, 1)), 1)
	default:
		ls = c.compact(w)
	}
	if !c.focus && room-len(ls) < minLogRows {
		return nil
	}
	return ls
}

// year is the summary over the grid, the legend beside its last row when
// there is room, and under focus the cursor's day below.
func (c *calendar) year(w, weeks, cell int) []string {
	g := c.grid(weeks, cell)
	if gridWidth(weeks, cell)+2+legendW <= w {
		g[len(g)-1] += "  " + heatLegend()
	}
	out := append([]string{c.summary()}, g...)
	if c.focus {
		out = append(out, c.detail())
	}
	return append(out, "")
}

// compact is the last weeks that fit two cells a day beside the summary's
// parts, the legend level with Sunday; nil when fewer than 4 weeks fit.
func (c *calendar) compact(w int) []string {
	side := make([]string, 1+len(weekdays))
	side[0] = ui.Strong.Render("TIME+ per day")
	side[1] = ui.Muted.Render(c.activeText())
	if c.Load != nil {
		side[2] = ui.Muted.Render(LoadText(*c.Load))
	}
	side[len(side)-1] = heatLegend()
	sideW := 0
	for _, s := range side {
		sideW = max(sideW, ansi.StringWidth(s))
	}
	weeks := min(calendarWeeks, (w-gridWidth(0, 2)-monthsOver-1-sideW)/2)
	if weeks < 4 {
		return nil
	}
	g := c.grid(weeks, 2)
	for i := range g {
		g[i] = ui.Fit(g[i], gridWidth(weeks, 2)+monthsOver) + " " + side[i]
	}
	return append(g, "")
}

func (c *calendar) activeText() string {
	return units.Plural(c.active, "active day") + " (last 365)"
}

// summary is the calendar's title and counts on one line.
func (c *calendar) summary() string {
	s := ui.G.Sep + c.activeText()
	if c.Load != nil {
		s += ui.G.Sep + LoadText(*c.Load)
	}
	return ui.Strong.Render("TIME+ per day") + ui.Muted.Render(s)
}

// grid is the months row, then a row per weekday, for the weeks columns
// ending on Today's week. A day after today is blank; the cursor's day is
// reversed under focus.
func (c *calendar) grid(weeks, cell int) []string {
	c.first = weekStart(c.Today).AddDate(0, 0, -7*(weeks-1))
	if c.at.Before(c.first) { // a narrower View cut its column
		c.at = c.first
	}
	out := []string{strings.Repeat(" ", dayLabelW) + ui.Muted.Render(months(c.first, weeks, cell))}
	for r, name := range weekdays {
		var b strings.Builder
		b.WriteString(ui.Muted.Render(name) + " ")
		for col := range weeks {
			if col > 0 && cell > 1 {
				b.WriteString(strings.Repeat(" ", cell-1))
			}
			day := c.first.AddDate(0, 0, 7*col+r)
			lv := c.heat(c.byDate[dateKey(day)].Door)
			switch {
			case day.After(c.Today):
				b.WriteByte(' ')
			case c.focus && day.Equal(c.at):
				b.WriteString(ui.Select.Render(ui.G.Heat[lv]))
			default:
				b.WriteString(ui.Heat(lv))
			}
		}
		out = append(out, b.String())
	}
	return out
}

// months labels each column whose Monday starts a new month, as GitHub
// does, over the columns and monthsOver past them. A label with no room
// before the next is left out, so the first column's goes first.
func months(first time.Time, weeks, cell int) string {
	row := []byte(strings.Repeat(" ", gridWidth(weeks, cell)-dayLabelW+monthsOver))
	next := len(row) + 1 // where the label to the right starts
	for col := weeks - 1; col >= 0; col-- {
		mon := first.AddDate(0, 0, 7*col)
		if col > 0 && mon.Month() == mon.AddDate(0, 0, -7).Month() {
			continue
		}
		if at := col * cell; at+len("Jan") < next {
			copy(row[at:], mon.Format("Jan"))
			next = at
		}
	}
	return string(row)
}

// heatLegend is the shades, least to most: "less ░▒▓█ more".
func heatLegend() string {
	var b strings.Builder
	for lv := 1; lv < len(ui.G.Heat); lv++ {
		b.WriteString(ui.Heat(lv))
	}
	return ui.Muted.Render("less ") + b.String() + ui.Muted.Render(" more")
}

// detail is the cursor's day: its date, then what it held, if anything.
func (c *calendar) detail() string {
	d := c.byDate[dateKey(c.at)]
	parts := []string{ui.Strong.Render(c.at.Format("Mon 2006-01-02"))}
	if d.Door > 0 {
		parts = append(parts, units.Clock(d.Door)+" in doors")
	}
	if d.Boots > 0 {
		parts = append(parts, units.Plural(d.Boots, "boot"))
	}
	if d.Clears > 0 {
		parts = append(parts, fmt.Sprintf("%d cleared", d.Clears))
	}
	if d.Crashes > 0 {
		parts = append(parts, fmt.Sprintf("%d crashed", d.Crashes))
	}
	if d.Upsolves > 0 {
		parts = append(parts, units.Plural(d.Upsolves, "upsolve"))
	}
	return strings.Join(parts, ui.Muted.Render(ui.G.Sep))
}
