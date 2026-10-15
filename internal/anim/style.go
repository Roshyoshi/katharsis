package anim

import "charm.land/lipgloss/v2"

// Every style the clips paint with is here, so they move to the shared
// internal/ui tokens in one place (Strong is bold, Muted is faint, Special
// is magenta: docs/design/ui-system.md §2.2). Text clips take their base
// style from the caller and only add a level's attribute to it.

// SPONGE: switch to ui.Strong, ui.Muted and ui.Special now that internal/ui
//         is on main (S9 Part 2).

var (
	// wall paints the rabbit-hole's walls and shelves.
	wall = lipgloss.NewStyle().Faint(true)
	// cardEdge paints a boss card's double border: the special hue, as the
	// boss door card does.
	cardEdge = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	// cardLabel is the label in a card's top edge. A hue is never bold
	// (ui-system.md R3).
	cardLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
)

// shade adds level l's attribute to base: faint below Normal, bold above.
func shade(base lipgloss.Style, l Level) lipgloss.Style {
	switch {
	case l < Normal:
		return base.Faint(true)
	case l > Normal:
		return base.Bold(true)
	}
	return base
}
