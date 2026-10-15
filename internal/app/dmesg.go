package app

import (
	"katharsis/internal/game"
	bootui "katharsis/internal/screens/boot"
)

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
	now := m.d.Now()
	evs := m.dmesg.Observe(m.state, &m.meta, now)
	// The bits a step set follow its own lines, in their moment, the newest
	// speaking (setuid.go).
	if m.state != nil {
		for _, ev := range m.setuid.strip {
			ev.At = now
			m.dmesg.Add(ev)
			evs = append(evs, ev)
		}
	}
	m.setuid.strip = nil
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
	// A one-time scene a screen drew (the merge window's daemon.intro) is
	// seen once drawn; the screen keeps it up while it stays.
	for _, s := range m.stack {
		if sh, ok := s.(interface{ Shown() []string }); ok {
			for _, id := range sh.Shown() {
				m.markSeen(id) // a story scene: saved, and noted (RT6)
			}
		}
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
