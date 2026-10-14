package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// BootOutcome is how a boot ended, "" while it can still be resumed.
type BootOutcome string

const (
	BootRunning   BootOutcome = ""
	BootCrashed   BootOutcome = "crashed"
	BootCleared   BootOutcome = "cleared"
	BootAbandoned BootOutcome = "abandoned" // left without crashing; see AbandonBoot
)

// Checkpoint saves meta and the boot snapshot in one transaction and records
// the attempts in st.History not recorded yet, keyed by (boot, index), so
// calling it after every engine Step records each attempt once. Either all
// of it lands or none does: a crash mid-way leaves the previous checkpoint.
//
// Boots are numbered from 1 (Boot #1). Checkpointing a new boot abandons
// any older boot still running, since there is one Meta and only the newest
// boot can resume against it.
func (s *Store) Checkpoint(meta game.Meta, st *game.State) error {
	if st == nil {
		return fmt.Errorf("%w: nil state; use SaveMeta outside a boot", ErrInvalidBoot)
	}
	if st.Boot < 1 {
		return fmt.Errorf("%w: boot %d; boots are numbered from 1", ErrInvalidBoot, st.Boot)
	}
	// Encode before taking the write lock, so no other writer waits on it.
	metaRaw, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("store: meta: %w", err)
	}
	stateRaw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("store: boot %d: %w", st.Boot, err)
	}
	now := s.now()
	return s.write(func(tx *sql.Tx) error {
		if err := s.putMeta(tx, metaRaw, now); err != nil {
			return err
		}
		if err := s.putRating(tx, meta.Rating, now); err != nil {
			return err
		}
		if err := s.putBoot(tx, st, stateRaw, now); err != nil {
			return err
		}
		return s.putAttempts(tx, st)
	})
}

// bootRow is what a State says about its boots row.
type bootRow struct {
	outcome  BootOutcome
	cause    game.Outcome
	ended    sql.NullInt64
	uptime   time.Duration
	lastDoor catalog.ID
}

// summarize derives the boots columns. A running boot's uptime is wall time
// since it started, as of this checkpoint; a finished one's is fixed by how
// it ended.
func summarize(st *game.State, now time.Time) bootRow {
	var r bootRow
	end := now
	switch st.Phase {
	case game.PhaseCrashed:
		r.outcome = BootCrashed
		if c := st.Crash; c != nil {
			r.cause = c.Cause
			if !c.At.IsZero() {
				end = c.At
			}
		}
		r.ended = sql.NullInt64{Int64: unixNano(end), Valid: true}
	case game.PhaseCleared:
		r.outcome = BootCleared
		if n := len(st.History); n > 0 && !st.History[n-1].Closed.IsZero() {
			end = st.History[n-1].Closed
		}
		r.ended = sql.NullInt64{Int64: unixNano(end), Valid: true}
	}
	if !st.Started.IsZero() {
		r.uptime = max(end.Sub(st.Started), 0)
	}
	if c := st.Crash; c != nil && c.Uptime > 0 {
		r.uptime = c.Uptime // the engine's number is what the panic screen showed
	}

	switch {
	case st.Crash != nil:
		r.lastDoor = st.Crash.Door.Problem
	case st.Current != nil:
		r.lastDoor = st.Current.Door.Problem
	case len(st.History) > 0:
		r.lastDoor = st.History[len(st.History)-1].Door.Problem
	}
	return r
}

func (s *Store) putBoot(tx *sql.Tx, st *game.State, raw []byte, now time.Time) error {
	r := summarize(st, now)
	var (
		seed, started int64
		outcome       BootOutcome
	)
	err := tx.QueryRow(`SELECT seed, started, outcome FROM boots WHERE id = ?`, st.Boot).
		Scan(&seed, &started, &outcome)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.Exec(`
			UPDATE boots SET outcome = 'abandoned', ended = ?
			WHERE outcome = '' AND id < ?`, unixNano(now), st.Boot); err != nil {
			return err
		}
	case err != nil:
		return err
	case uint64(seed) != st.Seed || started != unixNano(st.Started):
		return fmt.Errorf("%w: boot %d", ErrBootReused, st.Boot)
	case outcome != BootRunning && outcome != r.outcome:
		return fmt.Errorf("%w: boot %d is %s", ErrBootEnded, st.Boot, outcome)
	}

	// A finished boot keeps its first ended/uptime: later checkpoints of the
	// same crash (say, a log line) must not stretch it.
	_, err = tx.Exec(`
		INSERT INTO boots (id, seed, prestige, rules, started, ended, phase,
			outcome, cause, uptime, committed, last_door, state, saved)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			prestige  = excluded.prestige,
			rules     = excluded.rules,
			ended     = COALESCE(boots.ended, excluded.ended),
			phase     = excluded.phase,
			outcome   = excluded.outcome,
			cause     = excluded.cause,
			uptime    = CASE WHEN boots.outcome = '' THEN excluded.uptime ELSE boots.uptime END,
			committed = excluded.committed,
			last_door = excluded.last_door,
			state     = excluded.state,
			saved     = excluded.saved`,
		st.Boot, int64(st.Seed), st.Prestige, st.Rules.Name, unixNano(st.Started),
		r.ended, st.Phase, r.outcome, r.cause, int64(r.uptime), st.Committed,
		r.lastDoor, raw, unixNano(now))
	if err != nil {
		return err
	}
	return s.step("boot")
}

// putAttempts inserts History[next:], where next is one past the highest
// index already stored. History is append-only, so the earlier entries are
// already there and need not be re-encoded on every Tick.
func (s *Store) putAttempts(tx *sql.Tx, st *game.State) error {
	var next int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(idx) + 1, 0) FROM attempts WHERE boot = ?`,
		st.Boot).Scan(&next); err != nil {
		return err
	}
	for i := next; i < len(st.History); i++ {
		a := &st.History[i]
		raw, err := json.Marshal(a)
		if err != nil {
			return fmt.Errorf("store: boot %d attempt %d: %w", st.Boot, i, err)
		}
		d := &a.Door
		if _, err := tx.Exec(`
			INSERT INTO attempts (boot, idx, problem, topic, rating, blind,
				mystery, cursed, regression, boss, opened, closed, outcome,
				submits, failed, quacks, best_pass, total, json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (boot, idx) DO NOTHING`,
			st.Boot, i, d.Problem, d.Topic, d.Rating, d.Blind, d.Mystery,
			d.Cursed, d.Regression, d.Boss, unixNano(a.Opened),
			unixNano(a.Closed), a.Outcome, a.Submits, a.Failed, a.Quacks,
			a.BestPass, a.Total, raw); err != nil {
			return err
		}
	}
	return s.step("attempts")
}

// ActiveBoot returns the boot to resume, or nil when the newest boot has
// ended (crashed, cleared or abandoned).
func (s *Store) ActiveBoot() (*game.State, error) {
	var raw []byte
	err := s.db.QueryRow(`SELECT state FROM boots WHERE outcome = '' ORDER BY id DESC LIMIT 1`).
		Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st := new(game.State)
	if err := json.Unmarshal(raw, st); err != nil {
		return nil, fmt.Errorf("store: boot snapshot: %w", err)
	}
	return st, nil
}

// AbandonBoot marks a running boot as never to be resumed. It only records
// that: crash penalties (lost bytes, core dump) are the engine's to apply
// through Meta. Abandoning a boot that already ended is a no-op.
func (s *Store) AbandonBoot(boot int) error {
	now := s.now()
	return s.write(func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			UPDATE boots SET outcome = 'abandoned', ended = ?, saved = ?
			WHERE id = ? AND outcome = ''`, unixNano(now), unixNano(now), boot)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n > 0 {
			return err
		}
		var one int
		err = tx.QueryRow(`SELECT 1 FROM boots WHERE id = ?`, boot).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %d", ErrNoBoot, boot)
		}
		return err
	})
}
