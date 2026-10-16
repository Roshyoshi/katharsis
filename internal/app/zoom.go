package app

import (
	"os"

	"katharsis/internal/screens"
	"katharsis/internal/session"
)

// zoomMin is the narrowest pane a statement or editorial is read in as
// is. Below it the game's pane is zoomed over the editor while the reader
// is open (ui-system.md §2.6, E14): the HUD pane is 36 to 48 columns, and
// a statement wrapped there broke mid-phrase.
const zoomMin = 60

// zoomer is a Runner that can zoom the game's pane (the tmux runner).
type zoomer interface {
	Zoom(on bool) error
}

// Zoom zooms the game's own pane over its window, or unzooms it.
func (r *TmuxRunner) Zoom(on bool) error { return session.Zoom(os.Getenv("TMUX_PANE"), on) }

// zoomFor zooms the game's pane for s when the pane is narrower than
// below, the width s is read in (zoomMin for a reader, samples.Wide for
// the samples side by side), and remembers s, so that the pane unzooms
// once s is gone.
func (m *Model) zoomFor(s screens.Screen, below int) {
	z, ok := m.d.Runner.(zoomer)
	if !ok || m.zoomed != nil || m.width <= 0 || m.width >= below {
		return
	}
	if z.Zoom(true) == nil {
		m.zoomed = s
	}
}

// syncZoom unzooms the pane once the reader it was zoomed for has left
// the stack, by q or because the door closed under it. It runs after
// every Update.
func (m *Model) syncZoom() {
	if m.zoomed == nil {
		return
	}
	for _, s := range m.stack {
		if s == m.zoomed {
			return
		}
	}
	m.zoomed = nil
	if z, ok := m.d.Runner.(zoomer); ok {
		_ = z.Zoom(false) // best effort: a zoomed pane is only wider
	}
}
