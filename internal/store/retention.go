package store

import (
	"database/sql"
	"encoding/json"
	"slices"
	"time"

	"katharsis/internal/catalog"
)

// Retention queries (docs/research/retention.md RT9, RT13, GR11): how play
// fits the player's days, for GRUB's load average and top's retention
// panel. They watch; nothing here steers a door (GR7). Days are calendar
// days in now's location: hand them local time.

// RetentionOptions are the thresholds the queries take; each is a setting
// in the retention doc.
type RetentionOptions struct {
	// PracticeMin is how much play (boots in wall time, plus upsolving)
	// makes a day a practice day.
	PracticeMin time.Duration
	// SessionGap: a boot starting longer than this after the last one
	// ended starts a new session.
	SessionGap time.Duration
	// SessionBudget: a session longer than this is over budget (RT12).
	SessionBudget time.Duration
}

// DefaultRetention is the retention doc's defaults: 15 min, 30 min, 3 h.
func DefaultRetention() RetentionOptions {
	return RetentionOptions{PracticeMin: 15 * time.Minute, SessionGap: 30 * time.Minute, SessionBudget: 3 * time.Hour}
}

const (
	replayWithin  = 10 * time.Minute // a boot this soon after a crash is a replay
	upsolveWithin = 48 * time.Hour   // an upsolve this soon after a crash counts
	lateBefore    = 5                // a boot ending before 05:00 ended late
	retentionDays = 120              // how far back the queries read
	alarmQuiet    = 7 * 24 * time.Hour
)

// Alarm names (GR11). An alarm is shown once a week at most while it holds:
// the app records each showing as an alarm_shown event.
const (
	AlarmWorthIt      = "worth_it"       // "no" is a third or more of 3+ answers in 14 days
	AlarmLateNights   = "late_nights"    // 2+ boots ended 00:00-05:00 in 7 days
	AlarmOverBudget   = "over_budget"    // 3+ sessions over budget in 7 days
	AlarmHoursVsWorth = "hours_vs_worth" // more play than the fortnight before, a lower yes share
)

// Ratio is N of Of.
type Ratio struct{ N, Of int }

// WorthIt counts the worth-it answers in a window.
type WorthIt struct{ Yes, No, Skip int }

// Week is one Monday-start week of play.
type Week struct {
	Start    time.Time // Monday 00:00, local
	Days     int       // practice days
	Sessions int       // sessions that started in it
	Play     time.Duration
}

// Fortnight is 14 days of play and how it felt.
type Fortnight struct {
	Play  time.Duration
	Worth WorthIt
}

type Alarm struct {
	Name  string
	Fresh bool // not shown in the last 7 days
}

// Retention is RT13's panel as of Now.
type Retention struct {
	Now time.Time
	// Load is the share of practice days over the last 7, 30 and 90 days
	// (RT9), ending today once today is a practice day and yesterday until
	// then: a day not played yet is not a day missed.
	Load [3]float64
	// Days is the last 91 days, today last: practice day or not.
	Days []bool
	// Weeks is the last 8 weeks, this one last.
	Weeks []Week

	// Over the last 30 days: the median session, the median time between
	// sessions, crashes replayed within 10 minutes, crashes upsolved within
	// 48 hours (of the crashes at least 48 hours old), and boots that
	// ended 00:00-05:00.
	MedianSession time.Duration
	MedianReturn  time.Duration
	Replay        Ratio
	Upsolve       Ratio
	Late          Ratio

	// Over the last 7 days, for the alarms.
	LateWeek       int // boots that ended 00:00-05:00
	OverBudgetWeek int // sessions longer than the budget

	Worth      WorthIt      // the last 14 days
	Fortnights [2]Fortnight // the 14 days before the last 14, then the last 14
	Alarms     []Alarm
}

// span is a boot as the retention queries see it. Its play is wall time,
// from the start to the end or the last checkpoint, not the boot's uptime
// (time inside doors; PlayToday says why).
type span struct {
	started, ended time.Time // ended zero while running
	play           time.Duration
	crashed, trial bool
	lastDoor       catalog.ID
}

// timed is an event the queries read: an upsolve's time, a worth-it
// answer, an alarm shown.
type timed struct {
	at   time.Time
	kind string
	data struct {
		ElapsedMS int64  `json:"elapsed_ms"`
		Answer    string `json:"answer"`
		Alarm     string `json:"alarm"`
	}
}

type session struct{ start, end time.Time }

// retentionData is everything the queries read, from one snapshot.
type retentionData struct {
	boots   []span // by start
	events  []timed
	upsolve []solveAt // Meta.Solves made as upsolves
}

type solveAt struct {
	problem catalog.ID
	at      time.Time
}

func (s *Store) retentionData(now time.Time) (retentionData, error) {
	var d retentionData
	since := unixNano(now.AddDate(0, 0, -retentionDays))
	err := s.read(func(tx *sql.Tx) error {
		if err := eachRow(tx, `
			SELECT started, COALESCE(ended, 0), saved, outcome,
				json_extract(state, '$.Trial') IS NOT NULL, last_door
			FROM boots WHERE started >= ? ORDER BY started, id`, []any{since},
			func(rows *sql.Rows) error {
				var (
					b                     span
					started, ended, saved int64
					outcome               BootOutcome
					isTrial               bool
				)
				if err := rows.Scan(&started, &ended, &saved, &outcome, &isTrial, &b.lastDoor); err != nil {
					return err
				}
				b.started, b.ended = fromUnixNano(started), fromUnixNano(ended)
				b.crashed, b.trial = outcome == BootCrashed, isTrial
				// A running boot has played to its last checkpoint. So has an
				// abandoned one, which ended when the app marked it, maybe a
				// day later.
				last := b.ended
				if outcome == BootRunning || outcome == BootAbandoned {
					last = fromUnixNano(saved)
				}
				if outcome == BootAbandoned {
					b.ended = last
				}
				b.play = max(last.Sub(b.started), 0)
				d.boots = append(d.boots, b)
				return nil
			}); err != nil {
			return err
		}
		if err := eachRow(tx, `
			SELECT at, kind, json FROM events
			WHERE kind IN (?, ?, ?) AND at >= ? ORDER BY at, id`,
			[]any{EventUpsolveClose, EventWorthIt, EventAlarmShown, since},
			func(rows *sql.Rows) error {
				var (
					e   timed
					at  int64
					raw []byte
				)
				if err := rows.Scan(&at, &e.kind, &raw); err != nil {
					return err
				}
				e.at = fromUnixNano(at)
				_ = json.Unmarshal(raw, &e.data) // a payload it cannot read counts as empty
				d.events = append(d.events, e)
				return nil
			}); err != nil {
			return err
		}
		m, _, err := readMeta(tx)
		if err != nil {
			return err
		}
		for _, sv := range m.Solves {
			if sv.Upsolve {
				d.upsolve = append(d.upsolve, solveAt{sv.Problem, sv.At})
			}
		}
		return nil
	})
	return d, err
}

// dayKey names t's calendar day in loc.
func dayKey(t time.Time, loc *time.Location) string { return t.In(loc).Format(time.DateOnly) }

// midnight is the start of t's day in its own location.
func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// play sums play per local day: each boot's play (wall time, span) on the
// day it started, each upsolve's time on the day it closed.
func (d *retentionData) play(loc *time.Location) map[string]time.Duration {
	out := map[string]time.Duration{}
	for _, b := range d.boots {
		out[dayKey(b.started, loc)] += b.play
	}
	for _, e := range d.events {
		if e.kind == EventUpsolveClose {
			out[dayKey(e.at, loc)] += time.Duration(e.data.ElapsedMS) * time.Millisecond
		}
	}
	return out
}

// load is RT9's three figures; ok is false when no day in 90 was a
// practice day, which GRUB shows as nothing rather than as zeros.
func load(play map[string]time.Duration, now time.Time, least time.Duration) (avg [3]float64, ok bool) {
	loc := now.Location()
	end := midnight(now)
	if play[dayKey(end, loc)] < least {
		end = end.AddDate(0, 0, -1)
	}
	for i, n := range []int{7, 30, 90} {
		days := 0
		for k := range n {
			if play[dayKey(end.AddDate(0, 0, -k), loc)] >= least {
				days++
			}
		}
		avg[i] = float64(days) / float64(n)
		ok = ok || days > 0
	}
	return avg, ok
}

// LoadAverage is RT9's GRUB line: the share of practice days over the last
// 7, 30 and 90 days. ok is false when there was none in 90 days.
func (s *Store) LoadAverage(now time.Time, o RetentionOptions) (avg [3]float64, ok bool, err error) {
	d, err := s.retentionData(now)
	if err != nil {
		return avg, false, err
	}
	avg, ok = load(d.play(now.Location()), now, o.PracticeMin)
	return avg, ok, nil
}

// sessions groups the finished boots: a gap longer than gap starts one.
func (d *retentionData) sessions(gap time.Duration) []session {
	var out []session
	for _, b := range d.boots {
		if b.ended.IsZero() {
			continue
		}
		if n := len(out); n > 0 && b.started.Sub(out[n-1].end) <= gap {
			out[n-1].end = later(out[n-1].end, b.ended)
			continue
		}
		out = append(out, session{b.started, b.ended})
	}
	return out
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// median sorts xs and returns its middle, 0 for none.
func median[T ~int64](xs []T) T {
	if len(xs) == 0 {
		return 0
	}
	slices.Sort(xs)
	n := len(xs)
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

// Retention is RT13's panel and GR11's alarms as of now.
func (s *Store) Retention(now time.Time, o RetentionOptions) (Retention, error) {
	d, err := s.retentionData(now)
	if err != nil {
		return Retention{}, err
	}
	loc := now.Location()
	r := Retention{Now: now}
	play := d.play(loc)
	r.Load, _ = load(play, now, o.PracticeMin)

	today := midnight(now)
	for k := 90; k >= 0; k-- {
		r.Days = append(r.Days, play[dayKey(today.AddDate(0, 0, -k), loc)] >= o.PracticeMin)
	}

	sess := d.sessions(o.SessionGap)
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	for w := 7; w >= 0; w-- {
		wk := Week{Start: monday.AddDate(0, 0, -7*w)}
		next := wk.Start.AddDate(0, 0, 7)
		for day := wk.Start; day.Before(next); day = day.AddDate(0, 0, 1) {
			p := play[dayKey(day, loc)]
			wk.Play += p
			if p >= o.PracticeMin {
				wk.Days++
			}
		}
		for _, se := range sess {
			if t := se.start.In(loc); !t.Before(wk.Start) && t.Before(next) {
				wk.Sessions++
			}
		}
		r.Weeks = append(r.Weeks, wk)
	}

	month, week := now.AddDate(0, 0, -30), now.AddDate(0, 0, -7)
	var lengths, gaps []time.Duration
	for i, se := range sess {
		if se.start.Before(month) {
			continue
		}
		lengths = append(lengths, se.end.Sub(se.start))
		if i > 0 {
			gaps = append(gaps, se.start.Sub(sess[i-1].end))
		}
		if !se.start.Before(week) && se.end.Sub(se.start) > o.SessionBudget {
			r.OverBudgetWeek++
		}
	}
	r.MedianSession, r.MedianReturn = median(lengths), median(gaps)

	for i, b := range d.boots {
		if b.ended.IsZero() || b.ended.Before(month) {
			continue
		}
		late := b.ended.In(loc).Hour() < lateBefore
		r.Late.Of++
		if late {
			r.Late.N++
			if !b.ended.Before(week) {
				r.LateWeek++
			}
		}
		if !b.crashed || b.trial {
			continue
		}
		r.Replay.Of++
		if i+1 < len(d.boots) && d.boots[i+1].started.Sub(b.ended) <= replayWithin {
			r.Replay.N++
		}
		if now.Sub(b.ended) >= upsolveWithin {
			r.Upsolve.Of++
			if slices.ContainsFunc(d.upsolve, func(u solveAt) bool {
				return u.problem == b.lastDoor && !u.at.Before(b.ended) && u.at.Sub(b.ended) <= upsolveWithin
			}) {
				r.Upsolve.N++
			}
		}
	}

	fortnight := now.AddDate(0, 0, -14)
	window := func(t time.Time) int { // 1 the last 14 days, 0 the 14 before, -1 neither
		switch {
		case t.After(now) || t.Before(now.AddDate(0, 0, -28)):
			return -1
		case t.Before(fortnight):
			return 0
		}
		return 1
	}
	for _, b := range d.boots {
		if w := window(b.started); w >= 0 {
			r.Fortnights[w].Play += b.play
		}
	}
	shown := map[string]bool{}
	for _, e := range d.events {
		w := window(e.at)
		switch e.kind {
		case EventUpsolveClose:
			if w >= 0 {
				r.Fortnights[w].Play += time.Duration(e.data.ElapsedMS) * time.Millisecond
			}
		case EventWorthIt:
			if w >= 0 {
				count(&r.Fortnights[w].Worth, e.data.Answer)
			}
		case EventAlarmShown:
			if now.Sub(e.at) < alarmQuiet {
				shown[e.data.Alarm] = true
			}
		}
	}
	r.Worth = r.Fortnights[1].Worth
	r.Alarms = alarms(&r, shown)
	return r, nil
}

func count(w *WorthIt, answer string) {
	switch answer {
	case "yes":
		w.Yes++
	case "no":
		w.No++
	default:
		w.Skip++
	}
}

// yesShare is the share of yes among the answers; ok is false under 2.
func (w WorthIt) yesShare() (float64, bool) {
	n := w.Yes + w.No
	return float64(w.Yes) / float64(max(n, 1)), n >= 2
}

// alarms are GR11's, in a fixed order. The worth-it alarm needs 3 answers,
// so one "no" does not trip it; the trend alarm needs 2 per fortnight.
func alarms(r *Retention, shown map[string]bool) []Alarm {
	var out []Alarm
	add := func(name string, on bool) {
		if on {
			out = append(out, Alarm{Name: name, Fresh: !shown[name]})
		}
	}
	n := r.Worth.Yes + r.Worth.No
	add(AlarmWorthIt, n >= 3 && 3*r.Worth.No >= n)
	add(AlarmLateNights, r.LateWeek >= 2)
	add(AlarmOverBudget, r.OverBudgetWeek >= 3)
	before, okB := r.Fortnights[0].Worth.yesShare()
	after, okA := r.Fortnights[1].Worth.yesShare()
	add(AlarmHoursVsWorth, okB && okA && after < before && r.Fortnights[1].Play > r.Fortnights[0].Play)
	return out
}
