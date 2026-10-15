package game

import (
	"maps"
	"time"

	"katharsis/internal/catalog"
)

// Brief is what the end of a boot says (retention RT1; UX J1, J6, J7):
// what the boot kept, the one lever behind how it ended, and one next
// step. The crash and clear screens render it; every number in it is the
// engine's own, so a screen never shows a figure the game did not use
// (retention GR3).
type Brief struct {
	Cleared bool // the boot beat its last boss; otherwise it crashed
	Trial   bool // a learn-mode trial: no prestige, no core dump

	// ---- kept ----

	// Rating is the overall rating as the boot began and now. Topic is the
	// crashed door's topic ("" for a clear, or a mixed door), and
	// TopicRating that topic's rating.
	Rating      Change
	Topic       string
	TopicRating Change
	// Bank is Meta.Bank now; Committed what this boot banked.
	Bank, Committed int64
	Dump            Dump
	// Regression is the crashed door's return, as the crash scheduled it;
	// nil for a clear or a trial.
	Regression *Regression
	// Gate is what the next prestige still needs (D31), Unlocked the
	// prestige this clear opened (0 for none), and Top is set for a clear
	// at the highest prestige, which has nothing left to open.
	Gate     PrestigeGate
	Unlocked int
	Top      bool

	// Lever is the one fact about the task that decided the boot.
	Lever Lever

	// Next is the one step the screen highlights.
	Next Next
}

// Change is a number before and after the boot. Known is false when the
// boot does not record the before (saved before State.AtStart existed).
type Change struct {
	Before, After float64
	Known         bool
}

// Delta is After - Before.
func (c Change) Delta() float64 { return c.After - c.Before }

// DumpKind is what became of the core dump at the end of a boot.
type DumpKind string

const (
	DumpNone      DumpKind = ""          // nothing was uncommitted
	DumpLive      DumpKind = "live"      // the crash left bytes, still to upsolve
	DumpRecovered DumpKind = "recovered" // upsolved since the crash and banked
	DumpGone      DumpKind = "gone"      // the crash's dump is no longer in Meta
	DumpTrial     DumpKind = "trial"     // a trial lost its bytes outright
)

// Dump is the core dump's fate.
type Dump struct {
	Kind    DumpKind
	Problem catalog.ID
	// Bytes is what the crash dumped. Independent is what upsolving it
	// with the editorial never opened banks (Bytes times its Mult), and
	// Recovered what the upsolve since banked (DumpRecovered).
	Bytes, Independent, Recovered int64
	// Lost is the earlier boot's dump this crash destroyed, nil for none.
	// Its Bytes are what the crash says it destroyed (Crash.DumpLost).
	Lost *CoreDump
}

// LeverKind is what decided the boot.
type LeverKind string

const (
	LeverTimer   LeverKind = "timer"   // the door's timer ran out
	LeverLives   LeverKind = "lives"   // failed submits spent the last life
	LeverCursed  LeverKind = "cursed"  // the curse: one failed submit
	LeverGaveUp  LeverKind = "gave-up" // :q!
	LeverCleared LeverKind = "cleared" // the last boss fell
)

// Lever is the attempt that decided the boot, as the task saw it: the
// door, its verdict and tests, its time. It never judges the player
// (retention RT1: a fixable cause names a test, a time, a tool or a
// topic).
type Lever struct {
	Kind    LeverKind
	Door    Door
	Verdict Verdict // the last full-test verdict, "" before any submit
	// Passed of Total is the best submit's tests (the HP bar).
	Passed, Total   int
	Submits, Failed int
	Quacks          int
	Elapsed, Timer  time.Duration
	Overtime        bool
	// Predicted is the engine's clear chance for the door when it was
	// offered (Door.SolveP). D36 keeps it off the doors and shows it here,
	// after the door, as calibration feedback; it does not count the curse.
	Predicted float64
}

// NextKind is the step the end screen highlights.
type NextKind string

const (
	NextUpsolve NextKind = "upsolve" // a live core dump: upsolve it
	NextReboot  NextKind = "reboot"  // nothing is owed: another boot
)

// Next is the one highlighted step. Problem and Bytes are the upsolve's
// door and what it banks done without the editorial.
type Next struct {
	Kind    NextKind
	Problem catalog.ID
	Bytes   int64
}

// Debrief reads the boot st has just ended against m as the engine left
// it. A boot still running gives the zero Brief.
func Debrief(st *State, m *Meta) Brief {
	if st == nil || m == nil || st.Phase != PhaseCrashed && st.Phase != PhaseCleared {
		return Brief{}
	}
	d := Brief{
		Cleared:   st.Phase == PhaseCleared,
		Trial:     st.Trial != nil,
		Bank:      m.Bank,
		Committed: st.Committed,
		Rating:    Change{After: m.Rating},
		Next:      Next{Kind: NextReboot},
	}
	if a := st.AtStart; a != nil {
		d.Rating.Before, d.Rating.Known = a.Rating, true
	}
	if !d.Trial {
		d.Gate, d.Unlocked = NextPrestige(st.Rules, m), st.Unlocked
		d.Top = d.Cleared && st.Prestige >= MaxPrestige
	}
	if d.Cleared {
		if a, ok := lastClear(st); ok {
			d.Lever = leverOf(LeverCleared, a)
		}
		return d
	}
	c := st.Crash
	if c == nil {
		return d
	}
	a := crashAttempt(st)
	kind := map[Outcome]LeverKind{TimedOut: LeverTimer, OutOfLives: LeverLives, GaveUp: LeverGaveUp}[c.Cause]
	if c.Cursed {
		kind = LeverCursed
	}
	d.Lever = leverOf(kind, a)
	if d.Lever.Verdict == "" {
		d.Lever.Verdict = c.Verdict
	}
	if t := c.Door.Topic; t != "" && t != Mixed {
		d.Topic = t
		d.TopicRating = Change{After: m.TopicRating[t]}
		if s := st.AtStart; s != nil {
			before, ok := s.TopicRating[t]
			if !ok {
				before = s.Rating // a topic's first door starts it at the overall (rate)
			}
			d.TopicRating.Before, d.TopicRating.Known = before, true
		}
		if _, ok := m.TopicRating[t]; !ok {
			d.Topic, d.TopicRating = "", Change{} // nothing rated it
		}
	}
	d.Dump = dumpOf(st, m, c)
	if d.Dump.Kind == DumpLive {
		d.Next = Next{Kind: NextUpsolve, Problem: d.Dump.Problem, Bytes: d.Dump.Independent}
	}
	if !d.Trial {
		for _, g := range m.Regressions {
			if g.Problem == c.Door.Problem {
				d.Regression = &g
			}
		}
	}
	return d
}

// dumpOf is what became of the crash's core dump, and of the one it
// destroyed.
func dumpOf(st *State, m *Meta, c *Crash) Dump {
	out := Dump{Problem: c.Door.Problem, Bytes: c.Lost}
	if c.DumpLost > 0 {
		lost := CoreDump{Bytes: c.DumpLost}
		if s := st.AtStart; s != nil && s.Dump != nil {
			lost = *s.Dump
			lost.Bytes = c.DumpLost
		}
		out.Lost = &lost
	}
	switch cur := m.CoreDump; {
	case c.Lost <= 0:
		out.Kind = DumpNone
	case st.Trial != nil:
		out.Kind = DumpTrial
	case cur != nil && cur.Problem == c.Door.Problem:
		out.Kind, out.Bytes, out.Independent = DumpLive, cur.Bytes, cur.Recovers(Independent)
	default:
		out.Kind = DumpGone
		if kind, ok := upsolvedSince(m, c); ok {
			// The dump this crash wrote, as Upsolve read it.
			dump := CoreDump{Bytes: c.Lost, Mult: st.Rules.DumpIndependentMult}
			out.Kind, out.Recovered = DumpRecovered, dump.Recovers(kind)
		}
	}
	return out
}

// upsolvedSince is the first upsolve of the crashed door after the crash,
// which is what banked its dump (Upsolve): its kind, ok false for none.
func upsolvedSince(m *Meta, c *Crash) (SolveKind, bool) {
	for _, s := range m.Solves {
		if s.Upsolve && s.Problem == c.Door.Problem && !s.At.Before(c.At) {
			return s.Kind, true
		}
	}
	return "", false
}

// crashAttempt is the attempt that crashed the boot: still open in
// Current, or already filed in History; failing both, one on the crash's
// door with nothing else known.
func crashAttempt(st *State) Attempt {
	p := st.Crash.Door.Problem
	if c := st.Current; c != nil && c.Door.Problem == p {
		return *c
	}
	for i := len(st.History) - 1; i >= 0; i-- {
		if st.History[i].Door.Problem == p {
			return st.History[i]
		}
	}
	return Attempt{Door: st.Crash.Door}
}

// lastClear is the attempt that cleared the boot: its last clear.
func lastClear(st *State) (Attempt, bool) {
	for i := len(st.History) - 1; i >= 0; i-- {
		if st.History[i].Outcome == Cleared {
			return st.History[i], true
		}
	}
	return Attempt{}, false
}

func leverOf(k LeverKind, a Attempt) Lever {
	l := Lever{
		Kind: k, Door: a.Door, Verdict: a.LastVerdict,
		Passed: a.BestPass, Total: a.Total, Submits: a.Submits, Failed: a.Failed,
		Quacks: a.Quacks, Timer: a.Door.Timer, Overtime: a.Overtime, Predicted: a.Door.SolveP,
	}
	if !a.Closed.IsZero() && !a.Opened.IsZero() {
		l.Elapsed = max(a.Closed.Sub(a.Opened), 0)
	}
	return l
}

// startOf is State.AtStart for the boot s that m is starting: the player's
// ratings and bank, and the core dump the boot puts at risk.
func startOf(s *State, m *Meta) *Start {
	st := &Start{Rating: m.Rating, Bank: m.Bank}
	if len(m.TopicRating) > 0 {
		st.TopicRating = maps.Clone(m.TopicRating)
	}
	if d := m.CoreDump; d != nil && s.Trial == nil {
		dump := *d
		st.Dump = &dump
	}
	return st
}
