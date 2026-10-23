package store

import (
	"database/sql"
	"fmt"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// TIME+ is named after top's cumulative-time column: here, the player's
// lifetime time inside doors. It counts what the doors' clocks counted and
// nothing between them (routes, picks, merge windows, story scenes), the
// rule game.State.Uptime keeps for one boot:
//
//   - boots: boots.uptime, the engine's in-door time since migration 3, the
//     number each boot's own screens and git log show. Trials are boots.
//   - upsolves: each one's time open, from its upsolve_close event. An
//     upsolve solved before the first session logged events (app_start)
//     left only its Solve, so it counts the Solve's Elapsed.
//
// It is a count that only grows and never resets: nothing keys a goal or
// a nudge to it (retention GR1, GR11), and the session budget keeps wall
// time (PlayToday).
//
// By topic and by day it is split from the doors themselves: each stored
// attempt's clock (game.Attempt.Ran) and each upsolve's time open. By day
// it also has a boot's open door, as the uptime its closed doors leave.
// Boots from before migration 3 are the one gap: their uptime ran a
// timed-out door to its close, where Ran stops at the deadline.

// TimePlus is the player's TIME+; except > 0 leaves that boot out, which
// the end of a boot shows as before and after.
func (s *Store) TimePlus(except int) (time.Duration, error) {
	var d time.Duration
	err := s.read(func(tx *sql.Tx) error {
		m, _, err := readMeta(tx)
		if err != nil {
			return err
		}
		ups, err := upsolveTimes(tx, m.Solves)
		if err != nil {
			return err
		}
		d, err = timePlus(tx, except, ups)
		return err
	})
	return d, err
}

// spent is time inside one door outside a boot: an upsolve of problem,
// closed at at after d open.
type spent struct {
	problem catalog.ID
	at      time.Time
	d       time.Duration
	glance  bool // an upsolve closed with nothing judged inside glance
}

// timePlus is TIME+ with boot except left out: the boots' uptime and ups.
func timePlus(q querier, except int, ups []spent) (time.Duration, error) {
	var boots int64
	if err := q.QueryRow(`SELECT COALESCE(SUM(uptime), 0) FROM boots WHERE id <> ?`, except).
		Scan(&boots); err != nil {
		return 0, err
	}
	d := time.Duration(boots)
	for _, u := range ups {
		d += u.d
	}
	return d, nil
}

// upsolveTimes lists every upsolve's time: the upsolve_close events, then
// the upsolves among solves (Meta.Solves) from before the events began.
// A close's verdict is the upsolve's last, "" when nothing was judged.
func upsolveTimes(q querier, solves []game.Solve) ([]spent, error) {
	var out []spent
	if err := eachRow(q, `
		SELECT problem, at, COALESCE(json_extract(json, '$.elapsed_ms'), 0),
			COALESCE(json_extract(json, '$.verdict'), '')
		FROM events WHERE kind = ?`, []any{EventUpsolveClose},
		func(rows *sql.Rows) error {
			var (
				u       spent
				at, ms  int64
				verdict string
			)
			err := rows.Scan(&u.problem, &at, &ms, &verdict)
			u.at, u.d = fromUnixNano(at), time.Duration(ms)*time.Millisecond
			u.glance = verdict == "" && u.d < glance
			out = append(out, u)
			return err
		}); err != nil {
		return nil, err
	}
	began, ok, err := eventsBegan(q)
	if err != nil {
		return nil, err
	}
	for _, sv := range solves {
		if sv.Upsolve && (!ok || sv.At.Before(began)) {
			out = append(out, spent{problem: sv.Problem, at: sv.At, d: sv.Elapsed})
		}
	}
	return out, nil
}

// doorClock is what a stored attempt's clock is read from (its Opened,
// Closed, Outcome and Deadline), and where the door was.
type doorClock struct {
	boot    int
	problem catalog.ID
	topic   string
	game.Attempt
}

// doorClocks lists every stored attempt's clock, by close. It reads the
// columns and the deadline only: decoding each attempt's JSON whole would
// cost far more.
func doorClocks(q querier) ([]doorClock, error) {
	var out []doorClock
	err := eachRow(q, `
		SELECT boot, problem, topic, opened, closed, outcome, json_extract(json, '$.Deadline')
		FROM attempts ORDER BY closed, boot, idx`, nil,
		func(rows *sql.Rows) error {
			var (
				c              doorClock
				opened, closed int64
				deadline       sql.NullString
			)
			if err := rows.Scan(&c.boot, &c.problem, &c.topic, &opened, &closed, &c.Outcome, &deadline); err != nil {
				return err
			}
			c.Opened, c.Closed = fromUnixNano(opened), fromUnixNano(closed)
			if deadline.Valid {
				t, err := time.Parse(time.RFC3339Nano, deadline.String)
				if err != nil {
					return fmt.Errorf("store: %s deadline: %w", c.problem, err)
				}
				c.Deadline = t
			}
			out = append(out, c)
			return nil
		})
	return out, err
}

// topicTimes is TIME+ by topic: each closed door's clock, and each upsolve
// on the topic its problem was last played as. A running boot's open door
// is in TIME+ but in no topic until it closes; an upsolve of a problem
// never met in a boot is in no topic at all.
func topicTimes(q querier, ups []spent) (map[string]time.Duration, error) {
	clocks, err := doorClocks(q)
	if err != nil {
		return nil, err
	}
	out := map[string]time.Duration{}
	topicOf := map[catalog.ID]string{}
	for _, c := range clocks {
		out[c.topic] += c.Ran(c.Closed)
		topicOf[c.problem] = c.topic // clocks come by close
	}
	for _, u := range ups {
		if topic, ok := topicOf[u.problem]; ok {
			out[topic] += u.d
		}
	}
	return out, nil
}

// Day is one calendar day of play, for git log's calendar.
type Day struct {
	Date time.Time // midnight, in the location Days was given
	// Door is the TIME+ spent this day. Time is cut at midnight: a door
	// open 23:50 to 00:20 puts 10 minutes on its first day and 20 on the
	// next, so each day holds what the clocks ran that day.
	Door time.Duration
	// What happened this day, each counted on the day it happened: boots
	// started; boots cleared (a wake among them) and crashed, on the day
	// they ended; upsolves, on the day they closed, glances aside (their
	// time is in Door).
	Boots, Clears, Crashes, Upsolves int
}

// Days is play per calendar day, from's day through to's, oldest first,
// one Day for each, as days run in from's location: hand it local time.
// Their door time is TIME+ split by day, so over every day played it adds
// up to TimePlus(0).
func (s *Store) Days(from, to time.Time) ([]Day, error) {
	cal := newCalendar(from, to)
	err := s.read(func(tx *sql.Tx) error {
		m, _, err := readMeta(tx)
		if err != nil {
			return err
		}
		ups, err := upsolveTimes(tx, m.Solves)
		if err != nil {
			return err
		}
		for _, u := range ups {
			cal.spread(u.at.Add(-u.d), u.at)
			if d := cal.day(u.at); d != nil && !u.glance {
				d.Upsolves++ // as git log lists them (Upsolves)
			}
		}

		clocks, err := doorClocks(tx)
		if err != nil {
			return err
		}
		closed := map[int]time.Duration{} // door time by boot
		for _, c := range clocks {
			d := c.Ran(c.Closed)
			closed[c.boot] += d
			cal.spread(c.Opened, c.Opened.Add(d))
		}

		return eachRow(tx, `
			SELECT id, started, COALESCE(ended, 0), outcome, uptime,
				json_extract(state, '$.Current.Opened')
			FROM boots`, nil,
			func(rows *sql.Rows) error {
				var (
					id                     int
					started, ended, uptime int64
					outcome                BootOutcome
					open                   sql.NullString
				)
				if err := rows.Scan(&id, &started, &ended, &outcome, &uptime, &open); err != nil {
					return err
				}
				if d := cal.day(fromUnixNano(started)); d != nil {
					d.Boots++
				}
				if d := cal.day(fromUnixNano(ended)); d != nil {
					switch outcome {
					case BootCleared:
						d.Clears++
					case BootCrashed:
						d.Crashes++
					}
				}
				// The door open at the boot's last checkpoint ran for the
				// rest of its uptime, from its opening: a door has no other
				// record until it closes.
				rest := time.Duration(uptime) - closed[id]
				if !open.Valid || rest <= 0 {
					return nil
				}
				t, err := time.Parse(time.RFC3339Nano, open.String)
				if err != nil {
					return fmt.Errorf("store: boot %d open door: %w", id, err)
				}
				if !t.IsZero() {
					cal.spread(t, t.Add(rest))
				}
				return nil
			})
	})
	return cal.days, err
}

// calendar adds play up by day for Days: its days run from first, a
// midnight, in first's location.
type calendar struct {
	first time.Time
	days  []Day
}

func newCalendar(from, to time.Time) *calendar {
	c := &calendar{first: midnight(from)}
	n := daysBetween(c.first, to.In(from.Location())) + 1
	for i := range max(n, 0) {
		c.days = append(c.days, Day{Date: c.first.AddDate(0, 0, i)})
	}
	return c
}

// daysBetween counts the calendar days from a's date to b's, as their
// dates read: a day across a DST change is 23 or 25 hours long.
func daysBetween(a, b time.Time) int {
	date := func(t time.Time) time.Time {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	return int(date(b).Sub(date(a)) / (24 * time.Hour))
}

// day is the Day t falls on, nil outside the calendar.
func (c *calendar) day(t time.Time) *Day {
	if t.IsZero() {
		return nil
	}
	i := daysBetween(c.first, t.In(c.first.Location()))
	if i < 0 || i >= len(c.days) {
		return nil
	}
	return &c.days[i]
}

// spread adds the time from start to end to the days it ran on, cut at
// each midnight. What falls outside the calendar is left out.
func (c *calendar) spread(start, end time.Time) {
	if len(c.days) == 0 {
		return
	}
	stop := c.days[len(c.days)-1].Date.AddDate(0, 0, 1)
	start, end = later(start, c.first).In(c.first.Location()), earlier(end, stop)
	for start.Before(end) {
		next := midnight(start).AddDate(0, 0, 1)
		c.day(start).Door += earlier(end, next).Sub(start)
		start = next
	}
}

func earlier(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
