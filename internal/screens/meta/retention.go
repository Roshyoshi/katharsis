package meta

import (
	"fmt"
	"strings"
	"time"

	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Retention is top's retention panel (docs/research/retention.md RT13):
// how play fits the player's days, and GR11's alarms. It watches and
// never steers. The app maps the store's figures onto it.
type Retention struct {
	Now  time.Time // local; the strip ends on its day
	Load [3]float64
	// Days is the last 91 days, Now's day last: practice day or not.
	Days  []bool
	Weeks []RetentionWeek // oldest first

	// The last 30 days.
	MedianSession, MedianReturn time.Duration
	Replay, Upsolve, Late       Ratio

	Worth Worth // the last 14 days

	// What the alarms quote.
	Budget                   time.Duration
	LateWeek, OverBudgetWeek int
	Fortnights               [2]Fortnight // the 14 days before the last 14, then the last 14

	// Alarms are the alarms to show this time (store.Alarm* names): each
	// shows once a week while it holds.
	Alarms []string
}

type RetentionWeek struct {
	Start          time.Time
	Days, Sessions int
	Play           time.Duration
}

// Ratio is N of Of.
type Ratio struct{ N, Of int }

// Worth counts worth-it answers.
type Worth struct{ Yes, No, Skip int }

// Fortnight is 14 days of play and how it felt.
type Fortnight struct {
	Play  time.Duration
	Worth Worth
}

// Alarm names, as the store has them.
const (
	alarmWorthIt      = "worth_it"
	alarmLateNights   = "late_nights"
	alarmOverBudget   = "over_budget"
	alarmHoursVsWorth = "hours_vs_worth"
)

// LoadText is the load average as `uptime` prints it: "load average: 0.71,
// 0.20, 0.08".
func LoadText(l [3]float64) string {
	return fmt.Sprintf("load average: %.2f, %.2f, %.2f", l[0], l[1], l[2])
}

// alarmText says what an alarm saw, in plain numbers.
func (r *Retention) alarmText(name string) string {
	switch name {
	case alarmWorthIt:
		return fmt.Sprintf("%d of %d worth-it answers in 14 days were no", r.Worth.No, r.Worth.Yes+r.Worth.No)
	case alarmLateNights:
		return units.Plural(r.LateWeek, "boot") + " ended between 00:00 and 05:00 this week"
	case alarmOverBudget:
		return fmt.Sprintf("%s ran past %s this week", units.Plural(r.OverBudgetWeek, "session"), span(r.Budget))
	case alarmHoursVsWorth:
		b, a := r.Fortnights[0], r.Fortnights[1]
		return fmt.Sprintf("play went %s → %s a fortnight while worth-it went %s → %s",
			units.HM(b.Play), units.HM(a.Play), share(b.Worth), share(a.Worth))
	}
	return name
}

func share(w Worth) string {
	return fmt.Sprintf("%.0f%%", 100*float64(w.Yes)/float64(max(w.Yes+w.No, 1)))
}

// span is a duration said briefly: "57m", "40h40m", "19d16h".
func span(d time.Duration) string {
	d = d.Round(time.Minute)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		if m := int(d.Minutes()) % 60; m != 0 {
			return fmt.Sprintf("%dh%dm", int(d.Hours()), m)
		}
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
}

func ratio(r Ratio) string { return fmt.Sprintf("%d/%d", r.N, r.Of) }

// alarmLine is the line between top's meters and its table: the alarms, if
// any, with a pointer to the panel.
func (t *Top) alarmLine(w int) string {
	r := t.st.Retention
	if r == nil || len(r.Alarms) == 0 || t.tab == tabRetention {
		return ""
	}
	s := "! " + r.alarmText(r.Alarms[0])
	if n := len(r.Alarms) - 1; n > 0 {
		s += fmt.Sprintf(" (+%d more)", n)
	}
	return ui.Clip(ui.Warn.Render(s)+ui.Muted.Render(" · r retention"), w)
}

// keys is top's function-key bar.
func (t *Top) keys() string {
	pairs := []string{"j/k", "Move  ", "s", "Sort " + topSorts[t.sortBy].name + " "}
	if t.st.Retention != nil {
		name := "Retention "
		if t.tab == tabRetention {
			name = "Topics "
		}
		pairs = append(pairs, "r", name)
	}
	return keyBar(append(pairs, "q", "Back  ")...)
}

// retentionHead is the panel's title bar, in the topic table header's colors.
const retentionHead = ui.FieldHeader

// Top's tabs under the meters.
const (
	tabTopics = iota
	tabRetention
)

// toggleTab switches between the topics table and the retention panel,
// when there is one.
func (t *Top) toggleTab() {
	if t.st.Retention != nil {
		t.tab ^= tabRetention
	}
}

// retention is the panel in place of the topics table: h lines at most.
func (t *Top) retention(w, h int) []string {
	r := t.st.Retention
	title := " RETENTION  " + LoadText(r.Load)
	out := []string{retentionHead.Render(ui.Fit(title, w))}
	for _, a := range r.Alarms {
		out = append(out, ui.Warn.Render("! "+r.alarmText(a)))
	}
	label := func(s string) string { return ui.Focus.Render(s + ": ") }
	out = append(out,
		label("Sessions")+fmt.Sprintf("median %s, %s apart, %s ended 00:00–05:00", span(r.MedianSession),
			span(r.MedianReturn), ratio(r.Late))+ui.Muted.Render(" · 30 days"),
		label("Crashes")+fmt.Sprintf("%s replayed in 10m, %s upsolved in 48h", ratio(r.Replay), ratio(r.Upsolve))+
			ui.Muted.Render(" · 30 days"),
		label("Worth it")+fmt.Sprintf("%d yes, %d no, %d skipped", r.Worth.Yes, r.Worth.No, r.Worth.Skip)+
			ui.Muted.Render(" · 14 days"),
	)
	strip, weeks := t.strip(), t.weeks()
	for i := range max(len(strip), len(weeks)) {
		l, rt := "", ""
		if i < len(strip) {
			l = strip[i]
		}
		if i < len(weeks) {
			rt = weeks[i]
		}
		out = append(out, ui.Fit(l, 32)+rt)
	}
	if len(out) > h {
		out = out[:max(h, 0)]
	}
	return out
}

// strip is the last 13 weeks, a column per week and a row per weekday, a
// filled cell per practice day: when play happens, with nothing to keep
// unbroken.
func (t *Top) strip() []string {
	r := t.st.Retention
	if len(r.Days) == 0 {
		return nil
	}
	var grid [7][13]string
	for i := range grid {
		for j := range grid[i] {
			grid[i][j] = " "
		}
	}
	// idx counts days from this week's Monday (0) back into the past; the
	// last column is this week.
	wd := (int(r.Now.Weekday()) + 6) % 7
	for k, practiced := range r.Days {
		idx := wd - (len(r.Days) - 1 - k)
		week := idx / 7
		if idx%7 < 0 {
			week--
		}
		col, row := 12+week, idx-7*week
		if col < 0 {
			continue
		}
		grid[row][col] = ui.Muted.Render(ui.G.Empty)
		if practiced {
			grid[row][col] = ui.OK.Render(ui.G.Fill)
		}
	}
	out := []string{ui.Muted.Render("    13 weeks, today last")}
	for i, name := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
		out = append(out, ui.Muted.Render(name)+" "+strings.Join(grid[i][:], " "))
	}
	return out
}

// weeks is the last 8 weeks as a small table, this week first, so a short
// terminal cuts the oldest.
func (t *Top) weeks() []string {
	r := t.st.Retention
	if len(r.Weeks) == 0 {
		return nil
	}
	out := []string{ui.Muted.Render("week of  days  sessions  play")}
	for i := len(r.Weeks) - 1; i >= 0; i-- {
		wk := r.Weeks[i]
		out = append(out, fmt.Sprintf("%s  %4d  %8d  %4s", wk.Start.Format("01-02"), wk.Days, wk.Sessions, units.HM(wk.Play)))
	}
	return out
}
