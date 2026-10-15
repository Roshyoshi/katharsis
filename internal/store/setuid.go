package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/setuid"
)

// The setuid bits live on Meta (docs/design/achievements.md §(i)), so they
// land in the Checkpoint of the boot that set them: no table, and no
// window between "boot saved" and "bit saved". The store's part is the
// history a save from before them is backfilled from.

// FinishedBoots calls each with the snapshot of every boot that ran to its
// end, crashed or cleared, oldest first, trials included. A boot the store
// marked abandoned without an end (outcome 'abandoned') never reached one.
// The snapshots come from one read transaction, so each sees one store.
func (s *Store) FinishedBoots(each func(*game.State) error) error {
	return s.read(func(tx *sql.Tx) error {
		return finishedBoots(tx, each)
	})
}

func finishedBoots(q querier, each func(*game.State) error) error {
	return eachRow(q, `SELECT id, state FROM boots WHERE outcome IN ('crashed', 'cleared') ORDER BY id`, nil,
		func(rows *sql.Rows) error {
			var (
				id  int
				raw []byte
			)
			if err := rows.Scan(&id, &raw); err != nil {
				return err
			}
			st := new(game.State)
			if err := json.Unmarshal(raw, st); err != nil {
				return fmt.Errorf("store: boot %d snapshot: %w", id, err)
			}
			return each(st)
		})
}

// SampleRuns is the events table's word on sample runs (setuid.Samples):
// the samples_judged events of each boot door, upsolves aside, and when
// the events began.
func (s *Store) SampleRuns() (setuid.Samples, error) {
	var out setuid.Samples
	err := s.read(func(tx *sql.Tx) error {
		var err error
		out, err = sampleRuns(tx)
		return err
	})
	return out, err
}

func sampleRuns(q querier) (setuid.Samples, error) {
	out := setuid.Samples{Runs: map[setuid.Door]int{}}
	var first sql.NullInt64
	if err := q.QueryRow(`SELECT MIN(at) FROM events WHERE kind = ?`, EventAppStart).Scan(&first); err != nil {
		return out, err
	}
	if first.Valid {
		out.Since = fromUnixNano(first.Int64)
	}
	err := eachRow(q, `
		SELECT boot, attempt, COUNT(*) FROM events
		WHERE kind = ? AND boot IS NOT NULL AND attempt IS NOT NULL
			AND COALESCE(json_extract(json, '$.upsolve'), 0) = 0
		GROUP BY boot, attempt`, []any{EventSamplesJudged},
		func(rows *sql.Rows) error {
			var (
				d setuid.Door
				n int
			)
			err := rows.Scan(&d.Boot, &d.Index, &n)
			out.Runs[d] = n
			return err
		})
	return out, err
}

// BackfillSetuid backfills m's setuid bits from the store's history at now
// (setuid.NewBackfill): every finished boot, oldest first, then Meta. It
// returns the bits set and leaves the save to the caller (SaveMeta). On an
// error m is as it was: the backfill runs on a copy.
func (s *Store) BackfillSetuid(m *game.Meta, now time.Time) ([]game.SetuidBit, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var work game.Meta
	if err := json.Unmarshal(raw, &work); err != nil {
		return nil, err
	}
	var set []game.SetuidBit
	err = s.read(func(tx *sql.Tx) error {
		samples, err := sampleRuns(tx)
		if err != nil {
			return err
		}
		b := setuid.NewBackfill(&work, samples, now)
		if err := finishedBoots(tx, func(st *game.State) error {
			b.Boot(st)
			return nil
		}); err != nil {
			return err
		}
		set = b.Done()
		return nil
	})
	if err != nil {
		return nil, err
	}
	*m = work
	return set, nil
}
