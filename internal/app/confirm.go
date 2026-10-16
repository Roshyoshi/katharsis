package app

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/problem"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/units"
)

// The game asks before a misinput (UX J3, J4): a full submit while the
// samples already fail (free if a sample fails again, sampleFailed), and
// a retreat, which burns cycles. The editor's :Submit refuses and names
// :Submit!; the pane's s and R ask on the keys' row and go ahead on a
// second press. Every price is the engine's own (State.RetreatCost),
// said before it is paid.

// samplesFail is the question for a full submit of the open boot door
// while its samples fail, again (how to go ahead) before the note, since
// vim's message line cuts a reply's tail; ok is false
// when there is nothing to ask: no sample has failed yet (none has run, or
// all that ran passed), the samples did not compile (a CE costs nothing),
// or the door is an upsolve, where a failure costs nothing either. On
// Heisenbug's doors (Door.QuietSamples) it says only that they fail. A
// submit that fails a sample again is free (sampleFailed), so it names no
// price: the question guards against a misinput, not a cost.
func (m *Model) samplesFail(again string) (string, bool) {
	h := m.hud()
	// A submit still judging answers any other as busy first.
	if h == nil || !m.inBootDoor() || m.state.Current == nil || m.judging {
		return "", false
	}
	s := h.Samples()
	// One sample that failed is enough, the pass still running or not: a
	// :Submit sent on the heels of a :w would otherwise beat the guard.
	if !s.Compiled || s.Failed == 0 {
		return "", false
	}
	samples := fmt.Sprintf("samples %d/%d pass", s.Passed, s.Total)
	if m.state.Current.Door.QuietSamples {
		samples = "samples fail"
	}
	return samples + " · " + again + " · free if a sample fails again", true
}

// retreatAsk is the question before the pane's R swaps the door: what it
// burns, or that kexec makes it free, and when the door's lock shuts it.
// ok is false when the retreat is refused anyway (past the lock, or the
// last cycles), so the refusal's reason shows at once.
func (m *Model) retreatAsk() (string, bool) {
	st := m.state
	if m.attempt() == nil {
		return "", false
	}
	if _, locked := bootui.RetreatLocked(st, m.d.Now()); locked {
		return "", false
	}
	lock := ""
	if at := st.RetreatLockAt(); !at.IsZero() {
		lock = " · shuts in " + units.Countdown(at.Sub(m.d.Now()))
	}
	c, free := st.RetreatCost()
	switch {
	case free:
		return "kexec: a free swap, once per " + kexecSpan(st) + lock + " · R again", true
	case c >= st.Cycles:
		return "", false
	}
	return fmt.Sprintf("retreat: burns %s, %d left%s · new doors · R again", units.Plural(c, "cycle"), st.Cycles-c, lock), true
}

// kexecSpan is how long kexec's free swap lasts: an act under
// rules.KexecPerAct, else a biome.
func kexecSpan(st *game.State) string {
	if st.Rules.KexecPerAct {
		return "act"
	}
	return "biome"
}

// doorCost is the open boot door before a verdict is stepped, what charged
// measures the verdict's cost against; ok is false outside a boot's door
// (an upsolve costs nothing).
type doorCost struct {
	cycles   int
	deadline time.Time
	ok       bool
}

// doorLives is the open boot door as charged counts from.
func (m *Model) doorLives() doorCost {
	c := m.attempt()
	if !m.inBootDoor() || c == nil {
		return doorCost{}
	}
	return doorCost{cycles: m.state.Cycles, deadline: c.Deadline, ok: true}
}

// charged is what the verdict just stepped cost, for vim's :Submit reply:
// the cycles it burned, "burned 2 cycles, 22 left"; under ECC the clock it
// took, "ECC: -05:00 on the clock, 12:34 left" (no time left on a hidden
// clock), or why it burned cycles instead, "burned 2 cycles, 22 left (ECC:
// clock too short)"; "kernel panic" after either when it ended the boot;
// "" for nothing (an AC, a CE, an upsolve).
func (m *Model) charged(before doorCost) string {
	s := m.state
	if !before.ok || s == nil {
		return ""
	}
	lost := ""
	if n := before.cycles - s.Cycles; n > 0 {
		lost = bootui.Burned(n, s.Cycles)
		// The submit's own burn, the door open still or the boot out of
		// cycles: a verdict past the deadline is a timeout's instead.
		own := s.Phase == game.PhaseProblem || s.Crash != nil && s.Crash.Cause == game.OutOfLives
		if own && s.ECCConverts() {
			lost += " (" + bootui.ECCShort + ")"
		}
	} else if c := s.Current; c != nil && s.Phase == game.PhaseProblem {
		if took := before.deadline.Sub(c.Deadline); took > 0 {
			lost = "ECC: -" + units.Clock(took) + " on the clock"
			if !bootui.HiddenClock(s) {
				lost += ", " + units.Countdown(c.Deadline.Sub(m.d.Now())) + " left"
			}
		}
	}
	switch {
	case s.Phase == game.PhaseCrashed && lost != "":
		return lost + " · kernel panic"
	case s.Phase == game.PhaseCrashed:
		return "kernel panic"
	}
	return lost
}

// sampleFree is vim's :Submit reply after a failure on a sample.
const sampleFree = "a sample failed: no charge"

// sampleFailed reports whether one of failed, the full tests a submit of
// the open boot door failed, is one of the door's samples. Such a submit
// is free (meta.md §(b)): :w had already shown the sample failing. A full
// test is a sample by its input's tokens, as a solution reads it: a
// statement's sample can drop the blank lines between cases that the
// official test keeps (Codeforces 1692C). A full test much larger than
// every sample is not read.
func (m *Model) sampleFailed(failed []problem.Test) bool {
	if !m.inBootDoor() || len(failed) == 0 {
		return false
	}
	var samples []string
	var most int64
	for _, s := range m.open.Assets.Samples {
		if b, err := os.ReadFile(s.In); err == nil {
			samples, most = append(samples, testText(b)), max(most, int64(len(b)))
		}
	}
	for _, t := range failed {
		if fi, err := os.Stat(t.In); err != nil || fi.Size() > 2*most+1024 {
			continue
		}
		if b, err := os.ReadFile(t.In); err == nil && slices.Contains(samples, testText(b)) {
			return true
		}
	}
	return false
}

// testText is a test's input for comparing: its tokens, one space apart.
func testText(b []byte) string { return strings.Join(strings.Fields(string(b)), " ") }
