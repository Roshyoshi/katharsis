package app

import (
	"cmp"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/setuid"
	"katharsis/internal/store"
)

// The setuid bits, app side (docs/design/achievements.md (g)-(i)): the
// find / -perm -4000 screen off GRUB and what it records.

// setuidState is the session's side of the bits.
type setuidState struct {
	// notices is setuid_notices for this session: the find screen's n
	// turns it off or on until the game quits.
	notices bool
}

// setuidIntent handles the find screen's intents: opening it, a hint
// shown, the notices turned off or on.
func (m *Model) setuidIntent(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case screens.OpenSetuid:
		return m.openSetuid()
	case meta.SetuidHint:
		m.event(store.EventSetuidHint, "", store.SetuidHint{ID: msg.ID})
	case meta.SetuidNotices:
		m.setuid.notices = msg.On
	}
	return nil
}

// openSetuid shows find / -perm -4000 and records that it opened, with
// the spoilers it opened under and how many bits are set.
func (m *Model) openSetuid() tea.Cmd {
	spoilers := cmp.Or(m.d.Spoilers, meta.SpoilersNone)
	set := 0
	for _, d := range setuid.Table {
		if setuid.IsSet(&m.meta, d.ID) {
			set++
		}
	}
	m.event(store.EventSetuidOpen, "", store.SetuidOpen{Spoilers: spoilers, Set: set, Of: len(setuid.Table)})
	return m.push(meta.NewSetuid(meta.SetuidConfig{
		Meta: m.meta, Now: m.d.Now(), Spoilers: spoilers, Notices: m.setuid.notices,
	}))
}
