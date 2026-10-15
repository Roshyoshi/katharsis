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

	// 2: ML logging (docs/research/ml-pipeline.md §4.2). events holds what
	// only the app sees (§4.3, retention RT13); offers, skill_history and
	// model_versions wait for the engine's ML1 types to fill them.
	//
	// Rows from before this step keep NULL in the new attempts columns but
	// three: biome comes from the attempt's JSON where it has one, p is the
	// door's SolveP (the Elo chance it was offered at), and slot is
	// 'legacy', so calibration queries (G6) read every door.
	`
	CREATE TABLE offers (
		boot      INTEGER NOT NULL REFERENCES boots (id),
		seq       INTEGER NOT NULL,        -- index into State.Offers
		kind      TEXT    NOT NULL CHECK (kind IN ('doors', 'route')),
		biome     INTEGER NOT NULL,
		room      INTEGER NOT NULL,
		at        INTEGER NOT NULL,
		policy    TEXT    NOT NULL,
		reason    TEXT    NOT NULL,
		set_prop  REAL    NOT NULL,
		chosen    INTEGER,                 -- NULL until chosen
		chosen_at INTEGER,
		json      TEXT    NOT NULL,        -- game.Offer
		PRIMARY KEY (boot, seq)
	);

	CREATE TABLE events (
		id      INTEGER PRIMARY KEY,
		at      INTEGER NOT NULL,
		session INTEGER,                   -- id of this run's app_start event
		boot    INTEGER,                   -- NULL outside a boot
		attempt INTEGER,                   -- State.History index, NULL if none
		problem TEXT    NOT NULL DEFAULT '',
		kind    TEXT    NOT NULL,
		json    TEXT    NOT NULL DEFAULT '{}'
	);
	CREATE INDEX events_kind_at ON events (kind, at);
	CREATE INDEX events_boot ON events (boot, attempt);

	CREATE TABLE skill_history (
		id      INTEGER PRIMARY KEY,
		at      INTEGER NOT NULL,
		boot    INTEGER,
		attempt INTEGER,
		model   TEXT    NOT NULL,          -- version; 'shadow:<v>' during ML4
		topic   TEXT    NOT NULL,          -- 'general' or a catalog topic
		mu      REAL    NOT NULL,
		sd      REAL    NOT NULL,
		n       INTEGER NOT NULL           -- rated doors behind it
	);
	CREATE INDEX skill_history_topic_at ON skill_history (topic, at);

	CREATE TABLE model_versions (
		id         TEXT    PRIMARY KEY,    -- hash of the params JSON
		created    INTEGER NOT NULL,
		params     TEXT    NOT NULL,       -- rules.ModelParams
		n_doors    INTEGER NOT NULL,
		heldout_ll REAL,                   -- new params on the last 20%
		current_ll REAL,                   -- current params on the same doors
		accepted   INTEGER NOT NULL
	);

	ALTER TABLE boots    ADD COLUMN rules_hash TEXT NOT NULL DEFAULT '';
	ALTER TABLE boots    ADD COLUMN model      TEXT NOT NULL DEFAULT '';
	ALTER TABLE attempts ADD COLUMN biome      INTEGER;
	ALTER TABLE attempts ADD COLUMN p          REAL;
	ALTER TABLE attempts ADD COLUMN p_base     REAL;
	ALTER TABLE attempts ADD COLUMN slot       TEXT NOT NULL DEFAULT '';
	ALTER TABLE attempts ADD COLUMN propensity REAL;
	ALTER TABLE attempts ADD COLUMN offer      INTEGER;

	UPDATE attempts SET
		biome = json_extract(json, '$.Biome'),
		p     = json_extract(json, '$.Door.SolveP'),
		slot  = 'legacy';
	`,
}

// migrate applies the missing steps in one transaction, so a failed step
// leaves the file as it was rather than half-migrated. It reads
// user_version inside that transaction, which holds the write lock from
// BEGIN: two processes opening a fresh file at once apply each step exactly
// once, the second seeing the first's commit.
//
// A file already migrated gets a write that changes nothing instead. SQLite
// opens a file it cannot write (read-only, on a read-only mount) read-only
// without a word, and even begins an immediate transaction on it; only a
// statement that writes fails. Without the probe the game played a whole
// boot on such a file and saved none of it.
func migrate(db *sql.DB, steps []string) error {
	return inTx(db, nil, func(tx *sql.Tx) error {
		var v int
		if err := tx.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
			return err
		}
		switch {
		case v > len(steps):
			return fmt.Errorf("%w: schema %d, this build knows %d", ErrNewerSchema, v, len(steps))
		case v == len(steps):
			// Step 1 made meta; WHERE 0 matches nothing, but the statement
			// still needs the write lock on the file.
			if _, err := tx.Exec(`UPDATE meta SET id = id WHERE 0`); err != nil {
				return fmt.Errorf("%w: %v", ErrReadOnly, err)
			}
			return nil
		}
		for i := v; i < len(steps); i++ {
			if _, err := tx.Exec(steps[i]); err != nil {
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
		}
		// PRAGMA takes no bound parameters; the count is ours, not input.
		_, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(steps)))
		return err
	})
}
