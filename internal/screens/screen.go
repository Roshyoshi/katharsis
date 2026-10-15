// Package screens holds what every full-screen view shares. Screens are
// read-only views over game state: they never mutate State or Meta. A
// player choice leaves a screen as a tea.Msg, either a game.Action (the app
// passes it to the engine) or one of the intents below (the app routes it).
package screens

import (
	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
)

// Screen is one full-terminal view. The app owns the tea.Program and calls
// View with the current terminal size.
type Screen interface {
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View(width, height int) string
}

// Lookup resolves a catalog problem for display: its title, rating,
// contest. catalog.Catalog.Get satisfies it. A miss, or a nil Lookup,
// leaves the screen to show what it has instead, such as the catalog ID.
type Lookup func(catalog.ID) (catalog.Problem, bool)

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
	OpenLog       struct{}                     // git log
	OpenTop       struct{}                     // stats
	OpenCredits   struct{}                     // music, licenses, sources
	Upsolve       struct{ Problem catalog.ID } // a core dump or regression
	OpenStatement struct{ Problem catalog.ID }
	OpenEditorial struct{ Problem catalog.ID }
	// StartTrial begins a learn-mode trial for a USACO Guide module id.
	StartTrial struct{ Module string }
	// OpenModule shows a learn-mode module page in the reader.
	OpenModule struct{ Module string }
	// OpenShop shows apt, the packages the bank buys (D25); Install buys
	// one by name, and UseTheme puts an installed banner theme in use, ""
	// for the default.
	OpenShop struct{}
	Install  struct{ Package string }
	UseTheme struct{ Theme string }
	// OpenGrub leaves a boot that has ended for the GRUB menu (UX J12).
	OpenGrub struct{}
)
