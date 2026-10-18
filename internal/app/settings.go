package app

import (
	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/screens/meta"
	"katharsis/internal/store"
)

// Account is a content source's login, as the settings screen manages it:
//
//	Name() string                                       // "CSES"
//	Account() (nick string, ok bool)                    // the account in use
//	Login(ctx context.Context, nick, pass string) error // checks it with the site, then stores it
//	Logout() error
type Account = meta.Account

// Setting is one effective setting passwd lists, read-only: its key, its
// value and what set it, e.g. {"session_budget", "3h", "settings.toml"}.
type Setting = meta.Setting

// openSettings shows passwd over GRUB: the settings in effect and the
// accounts. A login runs under the app's lifetime (background), so quit
// waits for it.
func (m *Model) openSettings() tea.Cmd {
	c := meta.SettingsConfig{Accounts: m.d.Accounts, Run: m.background}
	if m.d.Settings != nil {
		c.Settings = m.d.Settings()
	}
	return m.push(meta.NewSettings(c))
}

// accountDone takes a login's or logout's result before passwd does. A
// source logged in or out judges other problems now: the catalog says so
// itself, but the pool is a copy. RTFM's map and its trials ask the
// catalog each time they open (judgeable), so the pool is the one copy.
func (m *Model) accountDone(d meta.AccountDone) tea.Cmd {
	if d.Err == nil {
		action := "login"
		if d.Logout {
			action = "logout"
		}
		m.event(store.EventAccount, "", kv{"source": d.Name, "action": action})
		m.repool()
	}
	var cmds []tea.Cmd
	for i, s := range m.stack {
		if p, ok := s.(*meta.Settings); ok {
			var cmd tea.Cmd
			m.stack[i], cmd = p.Update(d)
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// repool rereads the pool after a source logged in or out. Between boots
// it is the catalog's again. With a boot under way, as recheck does, it
// only stops offering what can no longer be judged: what a login made
// judgeable waits for the next boot, so the boot keeps the doors its seed
// made.
func (m *Model) repool() {
	if m.state == nil {
		m.pool = drawPool(m.d.Catalog)
		return
	}
	pool := make([]catalog.Problem, len(m.pool))
	for i, p := range m.pool {
		if q, ok := m.d.Catalog.Get(p.ID); p.Judgeable && (!ok || !q.Judgeable) {
			p.Judgeable = false
		}
		pool[i] = p
	}
	m.pool = pool
}
