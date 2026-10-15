package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
)

// slotLegacy marks an attempt whose door was offered before the door
// policy logged slots (ml-pipeline.md §4.1): its p is Door.SolveP, the Elo
// chance. The engine's ML1 types replace it with the door's own slot.
const slotLegacy = "legacy"

// hashRules names a boot's rules for grouping boots played under the same
// ones (boots.rules_hash): the first 16 hex digits of the SHA-256 of their
// JSON.
func hashRules(r rules.Rules) (string, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8]), nil
}

// Event kinds. The app emits them (ml-pipeline.md §4.3, retention.md RT13);
// the retention queries read some of them back.
const (
	EventAppStart       = "app_start" // a session starts; its id is the session
	EventAppQuit        = "app_quit"  // a clean exit
	EventSamplesJudged  = "samples_judged"
	EventEditorialOpen  = "editorial_open"
	EventEditorialClose = "editorial_close"
	EventUpsolveOpen    = "upsolve_open"
	EventUpsolveJudged  = "upsolve_judged"
	EventUpsolveClose   = "upsolve_close" // json elapsed_ms: time in the upsolve
	EventScreen         = "screen"        // a base screen shown; dwell runs to the next event
	EventSelfReport     = "self_report"   // json felt: easy | right | hard
	EventAptInstall     = "apt_install"
	EventWorthIt        = "worth_it"    // json answer: yes | no | skip
	EventAlarmShown     = "alarm_shown" // json alarm: a retention alarm's name
)

// Event is one fact only the app sees: no game struct records it. Data is
// its payload, stored as JSON.
type Event struct {
	ID int64 // set by the store
	At time.Time
	// Session is the id of the session's app_start event. Event sets it
	// on an app_start, from the id the row gets.
	Session int64
	Boot    int  // 0 outside a boot
	Attempt *int // State.History index of the door, nil for none
	Problem catalog.ID
	Kind    string
	// Data is marshalled to JSON on write (nil stores {}); reads return
	// it as a json.RawMessage.
	Data any
}

// Event appends e and returns its id. It is one small transaction of its
// own, apart from the checkpoints: a failed event write never touches a
// save.
func (s *Store) Event(e Event) (int64, error) {
	if e.Kind == "" {
		return 0, fmt.Errorf("store: event without a kind")
	}
	raw := []byte("{}")
	if e.Data != nil {
		var err error
		if raw, err = json.Marshal(e.Data); err != nil {
			return 0, fmt.Errorf("store: event %s: %w", e.Kind, err)
		}
	}
	var id int64
	err := s.write(func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			INSERT INTO events (at, session, boot, attempt, problem, kind, json)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			unixNano(e.At), nullInt(e.Session), nullInt(int64(e.Boot)),
			e.Attempt, e.Problem, e.Kind, raw)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		if e.Kind == EventAppStart {
			_, err = tx.Exec(`UPDATE events SET session = id WHERE id = ?`, id)
		}
		return err
	})
	return id, err
}

// EventFilter narrows Events; zero fields match everything.
type EventFilter struct {
	Kinds   []string
	Session int64
	Since   time.Time // at or after
	Limit   int       // <= 0: no limit
}

// Events lists events oldest first.
func (s *Store) Events(f EventFilter) ([]Event, error) {
	var (
		where []string
		args  []any
	)
	if len(f.Kinds) > 0 {
		where = append(where, "kind IN (?"+strings.Repeat(", ?", len(f.Kinds)-1)+")")
		for _, k := range f.Kinds {
			args = append(args, k)
		}
	}
	if f.Session != 0 {
		where, args = append(where, "session = ?"), append(args, f.Session)
	}
	if !f.Since.IsZero() {
		where, args = append(where, "at >= ?"), append(args, unixNano(f.Since))
	}
	q := "SELECT id, at, session, boot, attempt, problem, kind, json FROM events"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY at, id LIMIT ?"
	args = append(args, sqlLimit(f.Limit))

	var out []Event
	err := eachRow(s.db, q, args, func(rows *sql.Rows) error {
		var (
			e                      Event
			at                     int64
			session, boot, attempt sql.NullInt64
			raw                    []byte
		)
		if err := rows.Scan(&e.ID, &at, &session, &boot, &attempt, &e.Problem, &e.Kind, &raw); err != nil {
			return err
		}
		e.At = fromUnixNano(at)
		e.Session, e.Boot = session.Int64, int(boot.Int64)
		if attempt.Valid {
			a := int(attempt.Int64)
			e.Attempt = &a
		}
		e.Data = json.RawMessage(raw)
		out = append(out, e)
		return nil
	})
	return out, err
}

// nullInt stores 0 as NULL: no session, no boot.
func nullInt(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: n != 0} }
