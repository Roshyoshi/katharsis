package store

import (
	"database/sql"
	"fmt"
)

// migrations[i] takes the schema from user_version i to i+1. They are
// forward-only and append-only: never edit a shipped step, add a new one.
//
// Timestamps are unix nanoseconds (0 for the zero time). Each table that
// mirrors a game struct keeps the whole struct as JSON; the other columns
// are copies for WHERE, ORDER BY and GROUP BY.
var migrations = []string{
	// 1: initial schema.
	`
	CREATE TABLE meta (
		id    INTEGER PRIMARY KEY CHECK (id = 1),
		json  TEXT    NOT NULL, -- game.Meta
		saved INTEGER NOT NULL
	);

	CREATE TABLE boots (
		id        INTEGER PRIMARY KEY, -- State.Boot
		seed      INTEGER NOT NULL,    -- State.Seed, uint64 bits as int64
		prestige  INTEGER NOT NULL,
		rules     TEXT    NOT NULL,    -- preset name
		started   INTEGER NOT NULL,
		ended     INTEGER,             -- NULL while running
		phase     TEXT    NOT NULL,
		outcome   TEXT    NOT NULL
			CHECK (outcome IN ('', 'crashed', 'cleared', 'abandoned')),
		cause     TEXT    NOT NULL,    -- crash cause (game.Outcome)
		uptime    INTEGER NOT NULL,    -- ns, as of the last checkpoint
		committed INTEGER NOT NULL,    -- bytes banked this boot
		last_door TEXT    NOT NULL,    -- catalog.ID
		state     TEXT    NOT NULL,    -- game.State snapshot
		saved     INTEGER NOT NULL
	);

	CREATE TABLE attempts (
		boot       INTEGER NOT NULL REFERENCES boots (id),
		idx        INTEGER NOT NULL, -- index into State.History
		problem    TEXT    NOT NULL,
		topic      TEXT    NOT NULL,
		rating     INTEGER NOT NULL,
		blind      INTEGER NOT NULL,
		mystery    INTEGER NOT NULL,
		cursed     INTEGER NOT NULL,
		regression INTEGER NOT NULL,
		boss       TEXT    NOT NULL,
		opened     INTEGER NOT NULL,
		closed     INTEGER NOT NULL,
		outcome    TEXT    NOT NULL,
		submits    INTEGER NOT NULL,
		failed     INTEGER NOT NULL,
		quacks     INTEGER NOT NULL,
		best_pass  INTEGER NOT NULL,
		total      INTEGER NOT NULL,
		json       TEXT    NOT NULL, -- game.Attempt
		PRIMARY KEY (boot, idx)
	);
	CREATE INDEX attempts_problem ON attempts (problem, closed);
	CREATE INDEX attempts_closed ON attempts (closed);

	CREATE TABLE rating_history (
		id     INTEGER PRIMARY KEY,
		at     INTEGER NOT NULL,
		rating REAL    NOT NULL
	);
	CREATE INDEX rating_history_at ON rating_history (at);
	`,
}

// migrate applies the missing steps, one transaction each, so a failed step
// leaves the file at the last good version rather than half-migrated.
func migrate(db *sql.DB, steps []string) error {
	for {
		done, err := migrateOne(db, steps)
		if err != nil || done {
			return err
		}
	}
}

// migrateOne reads user_version inside the write transaction: two processes
// opening a fresh file at once then apply each step exactly once, the
// second seeing the first's commit.
func migrateOne(db *sql.DB, steps []string) (done bool, err error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil || done {
			tx.Rollback()
		}
	}()
	var v int
	if err := tx.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return false, err
	}
	switch {
	case v > len(steps):
		return true, fmt.Errorf("%w: schema %d, this build knows %d", ErrNewerSchema, v, len(steps))
	case v == len(steps):
		return true, nil
	}
	if _, err := tx.Exec(steps[v]); err != nil {
		return false, fmt.Errorf("migration %d: %w", v+1, err)
	}
	// PRAGMA takes no bound parameters; v is an int we control.
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v+1)); err != nil {
		return false, fmt.Errorf("migration %d: %w", v+1, err)
	}
	return false, tx.Commit()
}
