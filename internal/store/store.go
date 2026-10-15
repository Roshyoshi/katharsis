// Package store persists the player's progress in one SQLite file: the Meta
// that survives boots, a resumable snapshot of each boot, and the history
// (attempts, rating) that `git log` and `top` read.
//
// Game structs are stored whole, as JSON, beside a few columns copied out
// for SQL to filter and aggregate on. The JSON is the source of truth, so a
// field the engine adds to game.State or game.Meta round-trips without a
// schema change; the columns only ever hold what queries need.
//
// A Store is safe for concurrent use, and several processes may open the
// same file (a `top` pane next to a running boot): WAL lets readers proceed
// while a writer commits, and every write is one transaction, so a reader
// sees either all of a Checkpoint or none of it. Only one game plays against
// a file at a time (Acquire): two would each resume the same boot.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers driver "sqlite"

	"katharsis/internal/game"
	"katharsis/internal/rules"
)

var (
	// ErrNewerSchema means the file was migrated by a newer katharsis.
	// Opening it anyway would let this binary drop fields it does not know
	// on the next write.
	ErrNewerSchema = errors.New("store: database is from a newer katharsis")
	// ErrNoBoot is returned for a boot id that was never checkpointed.
	ErrNoBoot = errors.New("store: no such boot")
	// ErrInvalidBoot rejects a nil State or a boot numbered below 1. Boot 0
	// would also be unreachable by AttemptFilter, where 0 means any boot.
	ErrInvalidBoot = errors.New("store: invalid boot")
	// ErrBootEnded refuses a Checkpoint that would change how a finished
	// boot ended (for example, resuming an abandoned one).
	ErrBootEnded = errors.New("store: boot already ended")
	// ErrBootReused refuses a Checkpoint whose boot id belongs to a
	// different boot (another seed or start time), which would splice two
	// boots' attempts together.
	ErrBootReused = errors.New("store: boot id belongs to another boot")
	// ErrStaleBoot refuses a Checkpoint whose snapshot has fewer attempts
	// than the store already holds for its boot: it predates them, and
	// saving it would undo them.
	ErrStaleBoot = errors.New("store: boot snapshot is older than the store")
)

type Store struct {
	db  *sql.DB
	now func() time.Time
	// fault, when set by a test, runs between the writes of a transaction;
	// a non-nil error aborts it there, as a crash would.
	fault func(stage string) error
}

// DefaultPath follows the XDG base directory spec. A relative
// XDG_DATA_HOME is ignored, as the spec requires.
func DefaultPath() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" && filepath.IsAbs(d) {
		return filepath.Join(d, "katharsis", "katharsis.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "katharsis", "katharsis.db"), nil
}

// busyTimeout bounds how long a write waits for another process's write.
// Checkpoints are small, so a wait this long means something is stuck.
const busyTimeout = 5 * time.Second

// Open creates the file and its directory if needed and migrates the schema
// to the current version.
func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// 0700 is what the XDG spec asks of a data directory it has to create.
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn(abs))
	if err != nil {
		return nil, err
	}
	if err := migrate(db, migrations); err != nil {
		return nil, errors.Join(fmt.Errorf("%s: %w", abs, err), db.Close())
	}
	return &Store{db: db, now: time.Now}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// dsn sets the per-connection pragmas; database/sql opens connections
// lazily, so they cannot be set once after Open.
//
// synchronous=NORMAL is the WAL setting that survives a process crash
// intact; a power loss can drop the last few commits but never leaves a
// partial one. _txlock=immediate takes the write lock at BEGIN: a deferred
// transaction that reads first and writes later can fail with SQLITE_BUSY
// without waiting, since waiting could deadlock. Read-only transactions
// still begin deferred (the driver ignores _txlock for them).
func dsn(path string) string {
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeout.Milliseconds()))
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Set("_txlock", "immediate")
	return (&url.URL{Scheme: "file", Path: path, RawQuery: q.Encode()}).String()
}

// write runs fn in one immediate transaction.
func (s *Store) write(fn func(tx *sql.Tx) error) error { return inTx(s.db, nil, fn) }

// read runs fn in one read transaction, so multi-query reads (Stats) see a
// single snapshot even while another process commits.
func (s *Store) read(fn func(tx *sql.Tx) error) error {
	return inTx(s.db, &sql.TxOptions{ReadOnly: true}, fn)
}

func inTx(db *sql.DB, opts *sql.TxOptions, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(context.Background(), opts)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

func (s *Store) step(stage string) error {
	if s.fault == nil {
		return nil
	}
	return s.fault(stage)
}

// LoadMeta returns a fresh player (zero Meta at the cold-start rating) when
// nothing has been saved.
func (s *Store) LoadMeta() (game.Meta, error) {
	var raw []byte
	err := s.db.QueryRow(`SELECT json FROM meta WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return game.Meta{Rating: rules.Standard().ColdStartRating}, nil
	}
	if err != nil {
		return game.Meta{}, err
	}
	return decodeMeta(raw)
}

func decodeMeta(raw []byte) (game.Meta, error) {
	var m game.Meta
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("store: meta: %w", err)
	}
	return m, nil
}

// SaveMeta is for changes outside a boot (upsolving, learn mode); inside
// one, Checkpoint saves Meta together with the boot.
func (s *Store) SaveMeta(meta game.Meta) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("store: meta: %w", err)
	}
	now := s.now()
	return s.write(func(tx *sql.Tx) error {
		if err := s.putMeta(tx, raw, now); err != nil {
			return err
		}
		return s.putRating(tx, meta.Rating, now)
	})
}

func (s *Store) putMeta(tx *sql.Tx, raw []byte, now time.Time) error {
	_, err := tx.Exec(`
		INSERT INTO meta (id, json, saved) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET json = excluded.json, saved = excluded.saved`,
		raw, unixNano(now))
	if err != nil {
		return err
	}
	return s.step("meta")
}

// putRating appends to the sparkline only when the rating moved. It compares
// with the last point rather than the previous Meta, so the history stays
// self-consistent whoever wrote the Meta row.
func (s *Store) putRating(tx *sql.Tx, rating float64, now time.Time) error {
	var last sql.NullFloat64
	err := tx.QueryRow(`SELECT rating FROM rating_history ORDER BY id DESC LIMIT 1`).Scan(&last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if last.Valid && last.Float64 == rating {
		return nil
	}
	if _, err := tx.Exec(`INSERT INTO rating_history (at, rating) VALUES (?, ?)`,
		unixNano(now), rating); err != nil {
		return err
	}
	return s.step("rating")
}

// unixNano stores times as integer nanoseconds so SQL can compare and sort
// them. 0 stands for the zero time, whose UnixNano is undefined.
func unixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

func fromUnixNano(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}
