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

// spent is time inside one door outside a boot: an upsolve of problem.
type spent struct {
	problem catalog.ID
	d       time.Duration
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
func upsolveTimes(q querier, solves []game.Solve) ([]spent, error) {
	var out []spent
	if err := eachRow(q, `
		SELECT problem, COALESCE(json_extract(json, '$.elapsed_ms'), 0)
		FROM events WHERE kind = ?`, []any{EventUpsolveClose},
		func(rows *sql.Rows) error {
			var (
				u  spent
				ms int64
			)
			err := rows.Scan(&u.problem, &ms)
			u.d = time.Duration(ms) * time.Millisecond
			out = append(out, u)
			return err
		}); err != nil {
		return nil, err
	}
	var first sql.NullInt64
	if err := q.QueryRow(`SELECT MIN(at) FROM events WHERE kind = ?`, EventAppStart).
		Scan(&first); err != nil {
		return nil, err
	}
	for _, sv := range solves {
		if sv.Upsolve && (!first.Valid || sv.At.Before(fromUnixNano(first.Int64))) {
			out = append(out, spent{sv.Problem, sv.Elapsed})
		}
	}
	return out, nil
}

// topicTimes is TIME+ by topic: each closed door's clock, as State.Uptime
// runs it (to its close, a timed-out door to its deadline), and each
// upsolve on the topic its problem was last played as. A running boot's
// open door is in TIME+ but in no topic until it closes; an upsolve of a
// problem never met in a boot is in no topic at all.
func topicTimes(q querier, ups []spent) (map[string]time.Duration, error) {
	doors := map[string][]game.Attempt{}
	topicOf := map[catalog.ID]string{}
	var last time.Time
	// The columns and the deadline are what State.Uptime reads of an
	// attempt; decoding each attempt's JSON whole would cost far more.
	if err := eachRow(q, `
		SELECT problem, topic, opened, closed, outcome, json_extract(json, '$.Deadline')
		FROM attempts ORDER BY closed, boot, idx`, nil,
		func(rows *sql.Rows) error {
			var (
				problem        catalog.ID
				topic          string
				opened, closed int64
				a              game.Attempt
				deadline       sql.NullString
			)
			if err := rows.Scan(&problem, &topic, &opened, &closed, &a.Outcome, &deadline); err != nil {
				return err
			}
			a.Opened, a.Closed = fromUnixNano(opened), fromUnixNano(closed)
			if deadline.Valid {
				t, err := time.Parse(time.RFC3339Nano, deadline.String)
				if err != nil {
					return fmt.Errorf("store: %s deadline: %w", problem, err)
				}
				a.Deadline = t
			}
			doors[topic] = append(doors[topic], a)
			topicOf[problem] = topic
			last = a.Closed // rows come by close
			return nil
		}); err != nil {
		return nil, err
	}
	out := map[string]time.Duration{}
	for topic, as := range doors {
		// The engine's own rule for a boot's doors, over a topic's: as of
		// the last close, each of them runs to its own close or deadline.
		out[topic] = (&game.State{History: as}).Uptime(last)
	}
	for _, u := range ups {
		if topic, ok := topicOf[u.problem]; ok {
			out[topic] += u.d
		}
	}
	return out, nil
}
