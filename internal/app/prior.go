package app

import (
	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens/meta"
)

// askPrior puts the first-launch rating question (D37) over GRUB: never
// answered, and no door rated yet.
func (m *Model) askPrior() {
	if m.d.Prompts && m.state == nil && game.NeedsPrior(&m.meta) {
		m.stack = append(m.stack, meta.NewPrior())
	}
}

// answerPrior records the answer, seeds the rating with it, and saves.
// GRUB is rebuilt under it, with the new rating on its status line.
func (m *Model) answerPrior(a meta.PriorAnswer) tea.Cmd {
	game.SeedPrior(&m.meta, a.CF, m.d.Now())
	m.reset()
	if err := m.d.Store.SaveMeta(m.meta); err != nil {
		return fail("save: %v", err)
	}
	if a.CF > 0 {
		return note("rating %.0f: your first doors start near it", m.meta.Rating)
	}
	return nil
}
