package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// BootSummary is one `git log` line.
type BootSummary struct {
	Boot     int
	Seed     uint64
	Prestige int
	Rules    string // preset name
	Started  time.Time
	Outcome  BootOutcome
	Cause    game.Outcome  // crash cause when Outcome is BootCrashed
	Uptime   time.Duration // as of the last checkpoint while running
	// Committed is bytes banked during the boot.
	Committed int64
	// Dumped is the uncommitted bytes the crash left in the core dump
	// (Crash.Lost); 0 when the boot did not crash or was a trial, which
	// loses its bytes without a dump.
	Dumped   int64
	LastDoor catalog.ID
	// Ending is how a cleared boot ended short of its last boss
	// (game.State.Ending): "wake" is the Trial, git log's "awake".
	Ending string
}

// dumpedSQL reads Dumped out of the stored State. It has no column of its
// own: the snapshot already holds it, so boots saved before the field
// existed show it too.
const dumpedSQL = `CASE WHEN json_extract(state, '$.Trial') IS NULL
	THEN COALESCE(json_extract(state, '$.Crash.Lost'), 0) ELSE 0 END`

// Boots lists boots newest first; limit <= 0 lists all.
func (s *Store) Boots(limit int) ([]BootSummary, error) {
	var out []BootSummary
	err := eachRow(s.db, `
		SELECT id, seed, prestige, rules, started, outcome, cause, uptime,
			committed, `+dumpedSQL+`, last_door, ending
		FROM boots ORDER BY id DESC LIMIT ?`, []any{sqlLimit(limit)},
		func(rows *sql.Rows) error {
			var (
				b                     BootSummary
				seed, started, uptime int64
			)
			if err := rows.Scan(&b.Boot, &seed, &b.Prestige, &b.Rules, &started,
				&b.Outcome, &b.Cause, &uptime, &b.Committed, &b.Dumped, &b.LastDoor, &b.Ending); err != nil {
				return err
			}
			b.Seed = uint64(seed)
			b.Started = fromUnixNano(started)
			b.Uptime = time.Duration(uptime)
			out = append(out, b)
			return nil
		})
	return out, err
}

// AttemptFilter narrows Attempts; zero fields match everything.
type AttemptFilter struct {
	Boot    int        // > 0: this boot only
	Problem catalog.ID // this problem only
	Limit   int        // <= 0: no limit
}

// AttemptRecord is a closed attempt as the engine left it in State.History.
type AttemptRecord struct {
	Boot  int
	Index int // position in the boot's History
	game.Attempt
}

// Attempts lists closed attempts newest first.
func (s *Store) Attempts(f AttemptFilter) ([]AttemptRecord, error) {
	var (
		where []string
		args  []any
	)
	if f.Boot > 0 {
		where, args = append(where, "boot = ?"), append(args, f.Boot)
	}
	if f.Problem != "" {
		where, args = append(where, "problem = ?"), append(args, f.Problem)
	}
	q := "SELECT boot, idx, json FROM attempts"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY closed DESC, boot DESC, idx DESC LIMIT ?"
	args = append(args, sqlLimit(f.Limit))

	var out []AttemptRecord
	err := eachRow(s.db, q, args, func(rows *sql.Rows) error {
		var (
			r   AttemptRecord
			raw []byte
		)
		if err := rows.Scan(&r.Boot, &r.Index, &raw); err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &r.Attempt); err != nil {
			return fmt.Errorf("store: boot %d attempt %d: %w", r.Boot, r.Index, err)
		}
		out = append(out, r)
		return nil
	})
	return out, err
}

type RatingPoint struct {
	At     time.Time
	Rating float64
}

// RatingHistory lists rating changes at or after since, oldest first (the
// sparkline's order). The zero time lists all of them.
func (s *Store) RatingHistory(since time.Time) ([]RatingPoint, error) {
	var out []RatingPoint
	err := eachRow(s.db, `
		SELECT at, rating FROM rating_history WHERE at >= ? ORDER BY at, id`,
		[]any{unixNano(since)},
		func(rows *sql.Rows) error {
			var (
				p  RatingPoint
				at int64
			)
			if err := rows.Scan(&at, &p.Rating); err != nil {
				return err
			}
			p.At = fromUnixNano(at)
			out = append(out, p)
			return nil
		})
	return out, err
}

// TopicStats counts doors of one catalog topic across all boots.
type TopicStats struct {
	Attempts int
	Clears   int
	Time     time.Duration // TIME+ on the topic (topicTimes)
}

// Stats is the totals behind `top`.
type Stats struct {
	Boots int
	// FullClears counts the boots cleared to their last boss: a wake (the
	// Trial's ending, BootSummary.Ending) is cleared but no full clear.
	FullClears int
	Crashes    map[game.Outcome]int   // by cause
	Solves     map[game.SolveKind]int // from Meta.Solves, upsolves included
	Topics     map[string]TopicStats
	TimePlus   time.Duration // lifetime time inside doors (timeplus.go)
}

// Stats reads every total from one snapshot, so the numbers agree with each
// other even while a boot is checkpointing.
func (s *Store) Stats() (Stats, error) {
	st := Stats{
		Crashes: map[game.Outcome]int{},
		Solves:  map[game.SolveKind]int{},
		Topics:  map[string]TopicStats{},
	}
	err := s.read(func(tx *sql.Tx) error {
		if err := tx.QueryRow(`
			SELECT COUNT(*), COALESCE(SUM(outcome = 'cleared' AND ending = ''), 0)
			FROM boots`).Scan(&st.Boots, &st.FullClears); err != nil {
			return err
		}

		if err := eachRow(tx, `
			SELECT cause, COUNT(*) FROM boots
			WHERE outcome = 'crashed' GROUP BY cause`, nil,
			func(rows *sql.Rows) error {
				var (
					cause game.Outcome
					n     int
				)
				err := rows.Scan(&cause, &n)
				st.Crashes[cause] = n
				return err
			}); err != nil {
			return err
		}

		if err := eachRow(tx, `
			SELECT topic, COUNT(*), SUM(outcome = 'cleared')
			FROM attempts GROUP BY topic`, nil,
			func(rows *sql.Rows) error {
				var (
					topic string
					t     TopicStats
				)
				err := rows.Scan(&topic, &t.Attempts, &t.Clears)
				st.Topics[topic] = t
				return err
			}); err != nil {
			return err
		}

		m, _, err := readMeta(tx)
		if err != nil {
			return err
		}
		for _, sv := range m.Solves {
			st.Solves[sv.Kind]++
		}

		ups, err := upsolveTimes(tx, m.Solves)
		if err != nil {
			return err
		}
		if st.TimePlus, err = timePlus(tx, 0, ups); err != nil {
			return err
		}
		times, err := topicTimes(tx, ups)
		for topic, d := range times {
			t := st.Topics[topic]
			t.Time = d
			st.Topics[topic] = t
		}
		return err
	})
	return st, err
}

// querier is a *sql.DB or a *sql.Tx.
type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// eachRow runs q and calls scan on each row, stopping at its first error.
func eachRow(db querier, q string, args []any, scan func(*sql.Rows) error) error {
	rows, err := db.Query(q, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		if err := scan(rows); err != nil {
			return errors.Join(err, rows.Close())
		}
	}
	return errors.Join(rows.Err(), rows.Close())
}

// sqlLimit maps the API's "<= 0 is no limit" onto SQLite's LIMIT -1.
func sqlLimit(n int) int {
	if n <= 0 {
		return -1
	}
	return n
}
