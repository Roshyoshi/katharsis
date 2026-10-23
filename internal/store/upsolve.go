package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// glance is how long an upsolve can stay open with nothing judged and still
// be a look rather than an attempt: git log leaves it out and the calendar
// does not count it, though its time is in TIME+ all the same.
const glance = time.Minute

// UpsolveRecord is one upsolve in git log: a door opened outside a boot,
// from its upsolve_open event to its upsolve_close.
type UpsolveRecord struct {
	Problem catalog.ID
	Opened  time.Time
	// Elapsed is the time it was open: to its close, or to its last
	// verdict when no close came (the app was killed).
	Elapsed time.Duration
	// Kind is how it was solved, "" when it was left unsolved: Editorial
	// when the editorial was read in the game before the first AC.
	Kind      game.SolveKind
	Editorial bool             // the editorial was read in the game
	Recovered int64            // core dump bytes the solve banked
	Verdicts  []UpsolveVerdict // its full-test verdicts, in order
}

// UpsolveVerdict is one full-test verdict in an upsolve.
type UpsolveVerdict struct {
	Verdict       string
	Passed, Total int
	At            time.Duration // since the upsolve opened
	Editorial     bool          // the editorial was read before it
}

func (u UpsolveRecord) glance() bool { return len(u.Verdicts) == 0 && u.Elapsed < glance }

// upsolveEvent is the payload of the upsolve_* events.
type upsolveEvent struct {
	Verdict   string `json:"verdict"`
	Passed    int    `json:"passed"`
	Total     int    `json:"total"`
	Editorial bool   `json:"editorial"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Recovered int64  `json:"recovered"`
}

// Upsolves lists the upsolves newest first, glances left out: each one
// its upsolve_* events in one session, then the upsolves among solves
// (Meta.Solves) from before the events began, which have no verdicts.
// Events whose upsolve_open was lost are dropped.
func (s *Store) Upsolves() ([]UpsolveRecord, error) {
	var out []UpsolveRecord
	err := s.read(func(tx *sql.Tx) error {
		var (
			cur     *UpsolveRecord // the upsolve open as of the event read
			session int64
		)
		done := func() {
			if cur != nil && !cur.glance() {
				out = append(out, *cur)
			}
			cur = nil
		}
		if err := eachRow(tx, `
			SELECT at, COALESCE(session, 0), problem, kind, json FROM events
			WHERE kind IN (?, ?, ?) ORDER BY id`,
			[]any{EventUpsolveOpen, EventUpsolveJudged, EventUpsolveClose},
			func(rows *sql.Rows) error {
				var (
					at, sess int64
					problem  catalog.ID
					kind     string
					raw      []byte
					p        upsolveEvent
				)
				if err := rows.Scan(&at, &sess, &problem, &kind, &raw); err != nil {
					return err
				}
				if err := json.Unmarshal(raw, &p); err != nil {
					return fmt.Errorf("store: %s event: %w", kind, err)
				}
				if kind == EventUpsolveOpen {
					done()
					cur, session = &UpsolveRecord{Problem: problem, Opened: fromUnixNano(at), Editorial: p.Editorial}, sess
					return nil
				}
				if cur == nil || sess != session || problem != cur.Problem {
					return nil
				}
				d := time.Duration(p.ElapsedMS) * time.Millisecond
				cur.Elapsed, cur.Editorial = d, cur.Editorial || p.Editorial
				if kind == EventUpsolveClose {
					cur.Recovered = p.Recovered
					done()
					return nil
				}
				cur.Verdicts = append(cur.Verdicts, UpsolveVerdict{p.Verdict, p.Passed, p.Total, d, p.Editorial})
				if p.Verdict == "AC" && cur.Kind == "" {
					cur.Kind = game.Independent
					if p.Editorial {
						cur.Kind = game.Editorial
					}
				}
				return nil
			}); err != nil {
			return err
		}
		done()

		m, _, err := readMeta(tx)
		if err != nil {
			return err
		}
		began, ok, err := eventsBegan(tx)
		if err != nil {
			return err
		}
		for _, sv := range m.Solves {
			if sv.Upsolve && (!ok || sv.At.Before(began)) {
				out = append(out, UpsolveRecord{Problem: sv.Problem, Opened: sv.At.Add(-sv.Elapsed),
					Elapsed: sv.Elapsed, Kind: sv.Kind, Editorial: sv.Kind == game.Editorial})
			}
		}
		return nil
	})
	slices.SortStableFunc(out, func(a, b UpsolveRecord) int { return b.Opened.Compare(a.Opened) })
	return out, err
}

// eventsBegan is when the first session's app_start landed; ok is false
// before any did.
func eventsBegan(q querier) (at time.Time, ok bool, err error) {
	var first sql.NullInt64
	if err := q.QueryRow(`SELECT MIN(at) FROM events WHERE kind = ?`, EventAppStart).
		Scan(&first); err != nil {
		return at, false, err
	}
	if !first.Valid {
		return at, false, nil
	}
	return fromUnixNano(first.Int64), true, nil
}
