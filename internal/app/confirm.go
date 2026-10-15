package app

import (
	"fmt"
	"slices"
	"strings"

	"katharsis/internal/game"
	"katharsis/internal/ui"
)

// The game asks before it spends a life on a misinput (UX J3, J4): a full
// submit while the samples already fail, and a retreat. The editor's
// :Submit refuses and names :Submit!; the pane's s and R ask on the keys'
// row and go ahead on a second press.

// samplesFail is the question for a full submit of the open boot door
// while its samples fail, ending in again (how to go ahead); ok is false
// when there is nothing to ask: no sample has failed yet (none has run, or
// all that ran passed), the samples did not compile (a CE costs nothing),
// or the door is an upsolve, where a failure costs nothing either.
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
	return fmt.Sprintf("samples %d/%d pass · %s · %s", s.Passed, s.Total, m.failCost(), again), true
}

// failCost is what a failed full submit of the open door costs, in the
// boot's terms.
func (m *Model) failCost() string {
	st := m.state
	cost := 1
	if slices.Contains(st.Daemons, game.Nice) {
		cost = st.Rules.NiceLifeCost
	}
	switch {
	case st.Current.Door.Cursed:
		return "a fail ends the boot (cursed)"
	case cost >= st.Lives:
		return "a fail ends the boot (last " + ui.G.Life + ")"
	}
	return "a fail costs " + strings.Repeat(ui.G.Life, cost)
}

// retreatAsk is the question before the pane's R swaps the door: what it
// costs, or that kexec makes it free. ok is false when the engine refuses
// the retreat anyway (the last life), so its reason shows at once.
func (m *Model) retreatAsk() (string, bool) {
	st := m.state
	if st == nil || st.Phase != game.PhaseProblem || st.Current == nil {
		return "", false
	}
	if slices.Contains(st.Daemons, game.Kexec) && !st.KexecUsed {
		return "kexec: a free swap, once per biome · R again", true
	}
	if st.Lives <= 1 {
		return "", false
	}
	return fmt.Sprintf("retreat: -1 %s, %d left · new doors · R again", ui.G.Life, st.Lives-1), true
}
