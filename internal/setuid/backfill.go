package setuid

import (
	"time"

	"katharsis/internal/game"
)

// The backfill (achievements.md §(i)): a save from before the bits sets,
// once, the bits its stored history proves, each marked Backfill ("from
// git log"), and none it cannot (GR3: never fabricate).
//
//	NewBackfill(&m, samples, now)
//	├─ Boot(B) for each finished boot, oldest first   Fold, then the bits of
//	│                                                 B and the fold, at B's end
//	└─ Done()                                         the bits only Meta
//	                                                  proves, at now; Backfilled

// Door names a boot's door: its boot and its index in the boot's History.
type Door struct{ Boot, Index int }

// Samples is what the events table says of sample runs, for boots saved
// before Attempt.Samples counted them (State.CountsSamples false): Runs
// counts the samples_judged events by the door they were made on, and
// Since is when the events began (the first app_start). A door opened
// before Since has no record, so a bit that needs it stays unset.
type Samples struct {
	Since time.Time
	Runs  map[Door]int
}

// Backfill is one backfill in progress.
type Backfill struct {
	m       *game.Meta
	samples Samples
	now     time.Time
	set     []game.SetuidBit
}

// NewBackfill starts a backfill of m at now.
func NewBackfill(m *game.Meta, samples Samples, now time.Time) *Backfill {
	return &Backfill{m: m, samples: samples, now: now}
}

// Boot folds the finished boot s and sets the bits it and the fold so far
// prove, stamped with its end. Boots must come oldest first; one still
// running, or already folded, changes nothing.
func (b *Backfill) Boot(s *game.State) {
	if !ended(s) || s.Boot <= b.m.Record.Folded {
		return
	}
	Fold(b.m, s)
	b.set = append(b.set, check(b.m, s, s.EndTime(b.now), true, func(d Def) bool { return !d.current })...)
}

// Done sets the bits Meta proves as it stands, stamped now, marks m
// backfilled and returns every bit the backfill set, in the order set.
func (b *Backfill) Done() []game.SetuidBit {
	b.set = append(b.set, check(b.m, nil, b.now, true, func(Def) bool { return true })...)
	b.m.Record.Backfilled = true
	return b.set
}
