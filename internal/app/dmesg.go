package app

import (
	"katharsis/internal/game"
	bootui "katharsis/internal/screens/boot"
)

// dmesgTracker is the Model's strip state.
type dmesgTracker = bootui.Tracker

// The dmesg strip (screens/boot dmesg.go, UX J5): after every update the
// tracker compares the boot with the last look and keeps the events the
// change makes; boot screens draw them under the header and the problem
// pane under its vitals. The onboarding lines among them are one-time, so
// each is marked seen, and saved, as it is made.
func (m *Model) syncDmesg() {
	if m.dmesg.Lookup == nil {
		m.dmesg.Lookup = m.d.Catalog.Get
	}
	// Onboarding lines wait while anything is pushed over the phase's
	// screen: a story scene, a statement, git log.
	m.dmesg.Hold = m.top() != m.stack[0]
	evs := m.dmesg.Observe(m.state, &m.meta, m.d.Now())
	marked := false
	for _, ev := range evs {
		if ev.Seen != "" && game.MarkSeen(&m.meta, ev.Seen) {
			marked = true
		}
	}
	// Saved at once, as a story scene is: a quit right after must not
	// show the line again.
	if marked && m.state != nil {
		m.saved(m.d.Store.Checkpoint(m.meta, m.state))
	}
	if len(evs) > 0 {
		m.refresh() // the screens hold the Env they were last given
	}
}

// refused puts what the strip has to say about an action the engine
// refused on it: the Cat's line for a quack wanting a duck (a cursed door,
// or none left). Other refusals only show where they were made.
func (m *Model) refused(a game.Action) {
	if _, ok := a.(game.Quack); !ok || m.state == nil {
		return
	}
	if ev, ok := bootui.DuckRefused(m.state, m.d.Now()); ok {
		m.dmesg.Add(ev)
		m.refresh()
	}
}
