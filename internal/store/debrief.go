package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Read-only queries for the end of a boot (retention RT1, RT3, RT7, RT12),
// the nightly (RT15) and `katharsis motd` (RT14). Every figure is the
// player's own, from their boots (GR3): an ETA is a median of what they
// did, never a guess.

// ErrNoStore means the file OpenReadOnly was given does not exist: nobody
// has played yet.
var ErrNoStore = errors.New("store: no database yet")

// OpenReadOnly opens path for reading only, as a reader beside the game
// does (katharsis motd): no migration, no write probe, and no file or
// directory created. It never writes the file. A file from a newer
// katharsis is refused as Open refuses it; one not yet migrated (schema
// 0) reads as ErrNoStore.
func OpenReadOnly(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoStore
	} else if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeout.Milliseconds()))
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: abs, RawQuery: q.Encode()}).String())
	if err != nil {
		return nil, err
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return nil, errors.Join(fmt.Errorf("%s: %w", abs, err), db.Close())
	}
	switch {
	case v > len(migrations):
		return nil, errors.Join(fmt.Errorf("%w: schema %d, this build knows %d", ErrNewerSchema, v, len(migrations)), db.Close())
	case v == 0:
		return nil, errors.Join(ErrNoStore, db.Close())
	}
	return &Store{db: db, now: time.Now}, nil
}

// ended is a boot that ran to its end, crashed or cleared; trials and
// abandoned boots are not boots the player finished.
const ended = `outcome IN ('crashed', 'cleared') AND json_extract(state, '$.Trial') IS NULL`

// MedianCommitted is the median bytes the last n finished boots committed
// (apt's goal bar paces by it, RT3), 0 when there are none.
func (s *Store) MedianCommitted(n int) (int64, error) {
	var xs []int64
	err := eachRow(s.db, `SELECT committed FROM boots WHERE `+ended+` ORDER BY id DESC LIMIT ?`,
		[]any{sqlLimit(n)}, func(rows *sql.Rows) error {
			var c int64
			err := rows.Scan(&c)
			xs = append(xs, c)
			return err
		})
	if err != nil || len(xs) == 0 {
		return 0, err
	}
	slices.Sort(xs)
	if k := len(xs); k%2 == 0 {
		return (xs[k/2-1] + xs[k/2]) / 2, nil
	}
	return xs[len(xs)/2], nil
}

// MedianUptime is the median uptime of the last n finished boots on
// preset, the ETA "one more" on it offers (RT7); 0 when there are none.
func (s *Store) MedianUptime(preset string, n int) (time.Duration, error) {
	var ds []time.Duration
	err := eachRow(s.db, `SELECT uptime FROM boots WHERE `+ended+` AND rules = ? ORDER BY id DESC LIMIT ?`,
		[]any{preset, sqlLimit(n)}, func(rows *sql.Rows) error {
			var u int64
			err := rows.Scan(&u)
			ds = append(ds, time.Duration(u))
			return err
		})
	return median(ds), err
}

// PlayToday is how long the player has played on now's calendar day, in
// now's location: boot uptime and upsolving, as the load average counts
// it (RT9). The session budget's nudge reads it (RT12).
func (s *Store) PlayToday(now time.Time) (time.Duration, error) {
	d, err := s.retentionData(now)
	if err != nil {
		return 0, err
	}
	return d.play(now.Location())[dayKey(now, now.Location())], nil
}

// BossClearedBefore reports whether a boot before boot cleared the boss
// key, for the clear's "first kill" line (RT1).
func (s *Store) BossClearedBefore(boss string, boot int) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM attempts WHERE boss = ? AND outcome = 'cleared' AND boot < ? LIMIT 1`,
		boss, boot).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// BestClear is the fastest clear before boot on preset at prestige, ok
// false when there was none: the clear's personal-best line (RT1).
func (s *Store) BestClear(preset string, prestige, boot int) (best time.Duration, ok bool, err error) {
	var u sql.NullInt64
	err = s.db.QueryRow(`SELECT MIN(uptime) FROM boots WHERE outcome = 'cleared' AND rules = ? AND prestige = ? AND id < ?
		AND json_extract(state, '$.Trial') IS NULL`, preset, prestige, boot).Scan(&u)
	if err != nil || !u.Valid {
		return 0, false, err
	}
	return time.Duration(u.Int64), true, nil
}

// SeedRun is the best a seed has been played (RT15's personal best on a
// nightly): a clear, the fastest; else the boot that got furthest.
type SeedRun struct {
	Boots   int // boots played on the seed, abandoned ones included
	Cleared bool
	Uptime  time.Duration // the fastest clear, else the longest crash
	Doors   int           // doors cleared on that boot
}

// SeedBest is how the seed has been played, ok false when it never has.
func (s *Store) SeedBest(seed uint64) (r SeedRun, ok bool, err error) {
	err = eachRow(s.db, `
		SELECT b.outcome, b.uptime,
			(SELECT COUNT(*) FROM attempts a WHERE a.boot = b.id AND a.outcome = 'cleared')
		FROM boots b WHERE b.seed = ? AND json_extract(b.state, '$.Trial') IS NULL`,
		[]any{int64(seed)}, func(rows *sql.Rows) error {
			var (
				outcome BootOutcome
				uptime  int64
				doors   int
			)
			if err := rows.Scan(&outcome, &uptime, &doors); err != nil {
				return err
			}
			r.Boots++
			up := time.Duration(uptime)
			switch cleared := outcome == BootCleared; {
			case cleared && (!r.Cleared || up < r.Uptime):
				r.Cleared, r.Uptime, r.Doors = true, up, doors
			case !cleared && !r.Cleared && (doors > r.Doors || doors == r.Doors && up > r.Uptime):
				r.Uptime, r.Doors = up, doors
			}
			return nil
		})
	return r, r.Boots > 0, err
}

// Played reports whether any boot was played on seed, for the motd's "a
// nightly is new" (RT14).
func (s *Store) Played(seed uint64) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM boots WHERE seed = ? LIMIT 1`, int64(seed)).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
