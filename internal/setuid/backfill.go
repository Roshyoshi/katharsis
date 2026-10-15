package setuid

import (
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// The backfill (achievements.md §(i)): a save from before the bits sets,
// once, the bits its stored history proves, each marked Backfill ("from
// git log"), and none it cannot (GR3: never fabricate).
//
//	NewBackfill(&m, samples, now)
//	├─ Boot(B) for each finished boot, oldest first   Fold, then the bits of
//	│                                                 B and the fold so far,
//	│                                                 stamped B's end
//	└─ Done()                                         the core dumps the
//	                                                  upsolves recovered, then
//	                                                  every bit Meta proves as
//	                                                  it stands, stamped now
//
// What it will not guess:
//   - a bit that reads Meta beyond the fold (capabilities, libs, ratings,
//     completion, another bit) waits for Done: history cannot date it.
//   - the speed completions hold only if completion holds now and the boot
//     time is under the bound now, the one case where the bound was met.
//   - Heisenbug's door needs its sample runs: counted on the attempt
//     (State.CountsSamples), or in the events table from its start on.
//   - a core dump counts as recovered when an upsolve of its problem came
//     after the crash that dumped it and before the next crash, which
//     destroys a dump.

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

	// dumps are the core dumps the boots left, in order, each with the
	// time the next crash destroyed it (zero: none did).
	dumps []dump
}

// dump is a crash's core dump: its problem, when it was dumped and when
// the next crash destroyed it.
type dump struct {
	problem catalog.ID
	at, end time.Time
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
	b.crashed(s)
	Fold(b.m, s)
	v := &view{m: b.m, boot: s, now: s.EndTime(b.now), backfill: &b.samples}
	b.set = append(b.set, check(v, func(d Def) bool { return !d.current })...)
}

// crashed notes the core dump a crash of s destroyed and the one it left:
// a crash destroys the dump before it, and leaves its uncommitted bytes as
// the next (an abandoned boot loses them outright). A trial touches none.
func (b *Backfill) crashed(s *game.State) {
	c := s.Crash
	if s.Phase != game.PhaseCrashed || c == nil || s.Trial != nil {
		return
	}
	if n := len(b.dumps); n > 0 && b.dumps[n-1].end.IsZero() {
		b.dumps[n-1].end = c.At
	}
	if c.Lost > 0 && c.Cause != game.Abandoned && c.Door.Problem != "" {
		b.dumps = append(b.dumps, dump{problem: c.Door.Problem, at: c.At})
	}
}

// Done counts the core dumps the upsolves recovered (Meta.Solves), sets
// the bits Meta proves as it stands, stamped now, marks m backfilled and
// returns every bit the backfill set, in the order set.
func (b *Backfill) Done() []game.SetuidBit {
	r := &b.m.Record
	recovered, free := 0, 0
	for _, d := range b.dumps {
		i := slices.IndexFunc(b.m.Solves, func(sv game.Solve) bool {
			return sv.Upsolve && sv.Problem == d.problem && sv.At.After(d.at) && (d.end.IsZero() || sv.At.Before(d.end))
		})
		if i < 0 {
			continue
		}
		recovered++
		if b.m.Solves[i].Kind == game.Independent {
			free++
		}
	}
	r.Recovered, r.RecoveredFree = max(r.Recovered, recovered), max(r.RecoveredFree, free)
	b.set = append(b.set, check(&view{m: b.m, now: b.now, backfill: &b.samples}, func(Def) bool { return true })...)
	r.Backfilled = true
	return b.set
}
