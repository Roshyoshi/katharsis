package anim

import (
	"charm.land/lipgloss/v2"

	"katharsis/internal/ui"
)

// Every token the clips paint with is here, from the shared internal/ui
// set (docs/design/ui-system.md §2.2): Muted is faint, Special the boss
// hue. Each is read when a frame is drawn, so the palette the app picked
// (Mono under NO_COLOR) applies. Text clips take their base style from the
// caller and only add a level's attribute to it.

const (
	// wall paints the rabbit-hole's walls.
	wall = ui.Muted
	// cardEdge paints a boss card's double border and the label in its
	// top edge: the special hue, as the boss door card does. A hue is
	// never bold (ui-system.md R3).
	cardEdge = ui.Special
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
