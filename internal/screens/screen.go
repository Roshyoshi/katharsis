// Package screens holds what every full-screen view shares. Screens are
// read-only views over game state: they never mutate State or Meta. A
// player choice leaves a screen as a tea.Msg, either a game.Action (the app
// passes it to the engine) or one of the intents below (the app routes it).
package screens

import (
	tea "charm.land/bubbletea/v2"
)

// Screen is one full-terminal view. The app owns the tea.Program and calls
// View with the current terminal size.
type Screen interface {
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View(width, height int) string
}

// Emit wraps a message (a game.Action or an intent) as a command.
func Emit(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

// Intents: navigation the app handles, not the engine.
type (
	Quit      struct{}
	Back      struct{} // leave this screen for the previous one
	StartBoot struct {
		Prestige int
		Preset   string // rules preset name, "standard" by default
		Seed     uint64 // 0 means random
	}
	OpenLearn     struct{}
	OpenLog       struct{}                 // git log
	OpenTop       struct{}                 // stats
	Upsolve       struct{ Problem string } // catalog.ID of a core dump or regression
	OpenStatement struct{ Problem string }
	OpenEditorial struct{ Problem string }
)
