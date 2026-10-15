package boot

import (
	"fmt"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Door readiness (UX J9): how far each door's problem is from being on
// disk, so a fast choice is not met by a minute of dead air. The app keeps
// the downloads and hands their state over in Env.Fetch and Env.Wait.

// FetchState is where a door's download stands.
type FetchState uint8

const (
	FetchNone   FetchState = iota // nothing known: no line on the card
	FetchQueued                   // waiting behind another door of its source
	FetchGoing                    // downloading
	FetchReady                    // on disk: opening it is instant
	FetchFailed                   // the last try failed; choosing it tries again
)

// Fetch is one door's download.
type Fetch struct {
	State FetchState
	Got   int64 // body bytes received so far
}

// Wait is the door the player chose while its problem still downloads: its
// timer has not started, and esc gives the choice back (CancelWait).
type Wait struct {
	Index int
	Since time.Time
}

// CancelWait asks the app to stop waiting on the chosen door's download.
// The download itself goes on, so choosing the door again picks it up.
type CancelWait struct{}

// fetchNote is the card's word on its door's download, "" for none.
func (e Env) fetchNote(d game.Door) string {
	f := e.Fetch[d.Problem]
	switch f.State {
	case FetchQueued:
		return ui.Muted.Render("queued")
	case FetchGoing:
		if f.Got > 0 {
			return ui.Muted.Render("fetching " + units.Mem(f.Got))
		}
		return ui.Muted.Render("fetching")
	case FetchReady:
		return ui.Muted.Render("ready")
	case FetchFailed:
		return ui.Danger.Render("fetch failed")
	}
	return ""
}

// waitLine is the doors screen's last row while a chosen door downloads:
// which one, how far, that its timer is not running, and the ready doors
// a choice could switch to.
func (e Env) waitLine(width int) string {
	w := e.Wait
	st := e.State
	if w == nil || w.Index < 0 || w.Index >= len(st.Doors) {
		return ""
	}
	f := e.Fetch[st.Doors[w.Index].Problem]
	s := fmt.Sprintf("fetching door %d%s", w.Index+1, ui.G.Ellipsis)
	if f.Got > 0 {
		s += " " + units.Mem(f.Got)
	}
	if !w.Since.IsZero() && !e.Now.IsZero() {
		s += ui.G.Sep + units.Clock(e.Now.Sub(w.Since))
	}
	s += ui.G.Sep + "timer not running"
	for i, d := range st.Doors {
		if i != w.Index && e.Fetch[d.Problem].State == FetchReady {
			s += ui.G.Sep + fmt.Sprintf("%d is ready", i+1)
			break
		}
	}
	keys := ui.Footer(width, "esc", "cancel")
	// The way out stays; the status gives way.
	return ui.Spread(ui.Muted.Render(ui.Clip(s, max(width-widthOf(keys)-2, 1))), keys, width)
}
