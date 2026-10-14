package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"deadcases/internal/catalog"
	"deadcases/internal/game"
)

// BootSummary is one `git log` line.
type BootSummary struct {
	Boot     int
	Seed     uint64
	Prestige int
	Rules    string // preset name
	Started  time.Time
	Ended    time.Time // zero while running
	Phase    game.Phase
	Outcome  BootOutcome
	Cause    game.Outcome  // crash cause when Outcome is BootCrashed
	Uptime   time.Duration // as of the last checkpoint while running
	// Committed is bytes banked during the boot.
	Committed int64
	LastDoor  catalog.ID
	Doors     int // attempts closed
	Clears    int // of those, cleared
}

// Boots lists boots newest first; limit <= 0 lists all.
func (s *Store) Boots(limit int) ([]BootSummary, error) {
	if limit <= 0 {
		limit = -1 // SQLite: no limit
	}
	rows, err := s.db.Query(`
		SELECT b.id, b.seed, b.prestige, b.rules, b.started, b.ended, b.phase,
			b.outcome, b.cause, b.uptime, b.committed, b.last_door,
			(SELECT COUNT(*) FROM attempts a WHERE a.boot = b.id),
			(SELECT COUNT(*) FROM attempts a WHERE a.boot = b.id AND a.outcome = ?)
		FROM boots b ORDER BY b.id DESC LIMIT ?`, game.Cleared, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BootSummary
	for rows.Next() {
		var (
			b                     BootSummary
			seed, started, uptime int64
			ended                 sql.NullInt64
		)
		if err := rows.Scan(&b.Boot, &seed, &b.Prestige, &b.Rules, &started,
			&ended, &b.Phase, &b.Outcome, &b.Cause, &uptime, &b.Committed,
			&b.LastDoor, &b.Doors, &b.Clears); err != nil {
			return nil, err
		}
		b.Seed = uint64(seed)
		b.Started = fromUnixNano(started)
		b.Ended = fromUnixNano(ended.Int64)
		b.Uptime = time.Duration(uptime)
		out = append(out, b)
	}
	return out, rows.Err()
}

// AttemptFilter narrows Attempts; zero fields match everything.
type AttemptFilter struct {
	Boot    int        // > 0: this boot only
	Problem catalog.ID // this problem only
	Since   time.Time  // closed at or after
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
	if !f.Since.IsZero() {
		where, args = append(where, "closed >= ?"), append(args, unixNano(f.Since))
	}
	q := "SELECT boot, idx, json FROM attempts"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	limit := f.Limit
	if limit <= 0 {
		limit = -1
	}
	q += " ORDER BY closed DESC, boot DESC, idx DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttemptRecord
	for rows.Next() {
		var (
			r   AttemptRecord
			raw []byte
		)
		if err := rows.Scan(&r.Boot, &r.Index, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &r.Attempt); err != nil {
			return nil, fmt.Errorf("store: boot %d attempt %d: %w", r.Boot, r.Index, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type RatingPoint struct {
	At     time.Time
	Rating float64
}

// RatingHistory lists rating changes at or after since, oldest first (the
// sparkline's order). The zero time lists all of them.
func (s *Store) RatingHistory(since time.Time) ([]RatingPoint, error) {
	rows, err := s.db.Query(`
		SELECT at, rating FROM rating_history WHERE at >= ? ORDER BY at, id`,
		unixNano(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RatingPoint
	for rows.Next() {
		var (
			p  RatingPoint
			at int64
		)
		if err := rows.Scan(&at, &p.Rating); err != nil {
			return nil, err
		}
		p.At = fromUnixNano(at)
		out = append(out, p)
	}
	return out, rows.Err()
}

// TopicStats counts doors of one catalog topic across all boots.
type TopicStats struct {
	Attempts int
	Clears   int
}

// Stats is the totals behind `top`.
type Stats struct {
	Boots      int
	FullClears int
	Abandoned  int
	Crashes    map[game.Outcome]int   // by cause
	Solves     map[game.SolveKind]int // from Meta.Solves, upsolves included
	Topics     map[string]TopicStats
	Attempts   int
	Clears     int
	Uptime     time.Duration // summed over boots
	// LifetimeBytes is Meta.LifetimeBytes, the engine's running total.
	LifetimeBytes int64
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
		var uptime int64
		if err := tx.QueryRow(`
			SELECT COUNT(*),
				COALESCE(SUM(outcome = 'cleared'), 0),
				COALESCE(SUM(outcome = 'abandoned'), 0),
				COALESCE(SUM(uptime), 0)
			FROM boots`).Scan(&st.Boots, &st.FullClears, &st.Abandoned, &uptime); err != nil {
			return err
		}
		st.Uptime = time.Duration(uptime)

		if err := eachRow(tx, `
			SELECT cause, COUNT(*) FROM boots
			WHERE outcome = 'crashed' GROUP BY cause`,
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
			FROM attempts GROUP BY topic`,
			func(rows *sql.Rows) error {
				var (
					topic string
					t     TopicStats
				)
				err := rows.Scan(&topic, &t.Attempts, &t.Clears)
				st.Topics[topic] = t
				st.Attempts += t.Attempts
				st.Clears += t.Clears
				return err
			}); err != nil {
			return err
		}

		var raw []byte
		err := tx.QueryRow(`SELECT json FROM meta WHERE id = 1`).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		m, err := decodeMeta(raw)
		if err != nil {
			return err
		}
		for _, sv := range m.Solves {
			st.Solves[sv.Kind]++
		}
		st.LifetimeBytes = m.LifetimeBytes
		return nil
	})
	return st, err
}

func eachRow(tx *sql.Tx, q string, scan func(*sql.Rows) error) error {
	rows, err := tx.Query(q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
