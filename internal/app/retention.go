package app

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens/meta"
	"katharsis/internal/store"
	"katharsis/internal/ui"
)

// Retention (docs/research/retention.md RT9, RT13): the load average on
// GRUB, the calendar on git log, the panel in top, and the line under a
// boot's end screen. They watch how play fits the player's days and never
// steer a door (GR7).

// loadAverage is GRUB's RT9 figure, nil when there is none to show. A
// query that fails only hides it.
func (m *Model) loadAverage() *[3]float64 {
	l, ok, err := m.d.Store.LoadAverage(m.d.Now(), store.DefaultRetention())
	if err != nil {
		m.logf("load average: %v", err)
	}
	if !ok {
		return nil
	}
	return &l
}

// calendar is git log's header: TIME+ per day over the last year, with
// the load average. ok is false when the days cannot be read, which leaves
// git log without it.
func (m *Model) calendar() (c meta.Calendar, ok bool) {
	now := m.d.Now()
	days, err := m.d.Store.Days(meta.CalendarStart(now), now)
	if err != nil {
		m.logf("calendar: %v", err)
		return c, false
	}
	c = meta.Calendar{Today: now, Load: m.loadAverage()}
	for _, d := range days {
		c.Days = append(c.Days, meta.Day(d))
	}
	return c, true
}

// withRetention adds the retention panel to top's stats, and notes the
// alarms it shows, so each shows once a week while it holds (GR11).
func (m *Model) withRetention(s meta.Stats) meta.Stats {
	o := store.DefaultRetention()
	r, err := m.d.Store.Retention(m.d.Now(), o)
	if err != nil {
		m.logf("retention: %v", err)
		return s
	}
	p := &meta.Retention{
		Now: r.Now, Load: r.Load, Days: r.Days,
		MedianSession: r.MedianSession, MedianReturn: r.MedianReturn,
		Replay: meta.Ratio(r.Replay), Upsolve: meta.Ratio(r.Upsolve), Late: meta.Ratio(r.Late),
		Worth:  meta.Worth(r.Worth),
		Budget: o.SessionBudget, LateWeek: r.LateWeek, OverBudgetWeek: r.OverBudgetWeek,
	}
	for _, w := range r.Weeks {
		p.Weeks = append(p.Weeks, meta.RetentionWeek(w))
	}
	for i, f := range r.Fortnights {
		p.Fortnights[i] = meta.Fortnight{Play: f.Play, Worth: meta.Worth(f.Worth)}
	}
	for _, a := range r.Alarms {
		if a.Fresh {
			p.Alarms = append(p.Alarms, a.Name)
			m.event(store.EventAlarmShown, "", kv{"alarm": a.Name})
		}
	}
	s.Retention = p
	return s
}

func (m *Model) logf(format string, args ...any) {
	if m.tele.log != nil {
		m.tele.log.Printf(format, args...)
	}
}

// endLine is the one line under a boot's end screen (GR6: a safe point,
// one line, never blocking): "worth it today?" at most once a calendar day
// (RT13), and how the boot felt at most once a boot (ml-pipeline §4.3's
// self_report). Its own keys answer it; every other key goes to the screen
// as before. Leaving the screen unanswered records a skip, so it is not
// asked again.
type endLine struct {
	boot        int
	worth, felt bool // still asking
}

// armEndLine asks at the end of a boot, when the base screen became the
// crash or clear screen.
func (m *Model) armEndLine(screen string) {
	if m.tele.end.worth || m.tele.end.felt {
		m.endLineDone()
	}
	if !m.d.Prompts || m.state == nil || (screen != string(game.PhaseCrashed) && screen != string(game.PhaseCleared)) {
		return
	}
	now := m.d.Now()
	today := now.Format(time.DateOnly)
	e := endLine{boot: m.state.Boot, worth: m.tele.worthDay != today && !m.asked(store.EventWorthIt, midnightOf(now), 0)}
	e.felt = m.tele.feltBoot != e.boot && !m.asked(store.EventSelfReport, time.Time{}, e.boot)
	if e.worth {
		m.tele.worthDay = today
	}
	if e.felt {
		m.tele.feltBoot = e.boot
	}
	m.tele.end = e
}

// asked reports whether an event of kind is stored since since, in boot
// when boot > 0. A store it cannot read counts as asked: no question twice.
func (m *Model) asked(kind string, since time.Time, boot int) bool {
	evs, err := m.d.Store.Events(store.EventFilter{Kinds: []string{kind}, Since: since, Boot: boot, Limit: 1})
	if err != nil {
		m.logf("events: %v", err)
		return true
	}
	return len(evs) > 0
}

func midnightOf(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, t.Location())
}

// endLineShown reports whether the line is up: asking, on the end screen
// itself, not under a screen pushed over it.
func (m *Model) endLineShown() bool {
	e := m.tele.end
	return (e.worth || e.felt) && len(m.stack) == 1 && m.state != nil && m.state.Boot == e.boot
}

// endKey answers the line with one of its keys; ok is false for any other
// key, which the screen gets.
func (m *Model) endKey(k tea.KeyPressMsg) bool {
	if !m.endLineShown() {
		return false
	}
	e := &m.tele.end
	switch s := k.String(); {
	case e.worth && (s == "y" || s == "n"):
		m.event(store.EventWorthIt, "", kv{"answer": map[string]string{"y": "yes", "n": "no"}[s]})
		e.worth = false
	case e.felt && (s == "1" || s == "2" || s == "3"):
		m.event(store.EventSelfReport, "", kv{"felt": map[string]string{"1": "easy", "2": "right", "3": "hard"}[s]})
		e.felt = false
	default:
		return false
	}
	return true
}

// endLineDone records a skip for what the line still asked: the player
// left it unanswered.
func (m *Model) endLineDone() {
	e := &m.tele.end
	if e.worth {
		m.event(store.EventWorthIt, "", kv{"answer": "skip"})
	}
	if e.felt {
		m.event(store.EventSelfReport, "", kv{"felt": "skip"})
	}
	*e = endLine{}
}

// body is the top screen's view, one row shorter with the end line under
// it when that is up.
func (m *Model) body(w, h int) string {
	if !m.endLineShown() || h < 2 {
		return m.top().View(w, h)
	}
	e := m.tele.end
	var parts []string
	if e.worth {
		parts = append(parts, "worth it today? y / n")
	}
	if e.felt {
		parts = append(parts, "this boot felt: 1 easy · 2 about right · 3 hard")
	}
	line := ui.Clip(ui.Muted.Render(strings.Join(parts, "  ·  ")), w)
	return m.top().View(w, h-1) + "\n" + line
}
