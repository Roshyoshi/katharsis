package story

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Every style the story screens use is here, so the move to the shared
// internal/ui tokens (docs/design/ui-system.md §2.2, §2.6) touches one
// file: narration is the flavor voice (italic), titles are Strong (bold),
// asides and the footer are Muted (faint), and the ACCEPTED frame is OK.

// SPONGE: switch to ui.Strong, ui.Muted, ui.OK and ui.Footer now that
//         internal/ui is on main (S9 Part 2).

var (
	voice  = lipgloss.NewStyle().Italic(true)
	strong = lipgloss.NewStyle().Bold(true)
	muted  = lipgloss.NewStyle().Faint(true)
	plain  = lipgloss.NewStyle()
	ok     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	key    = lipgloss.NewStyle().Bold(true)
)

// footer is the key hints on the last row: key caps strong, labels muted,
// pairs apart by a muted " · ".
func footer(pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			b.WriteString(muted.Render(" · "))
		}
		b.WriteString(key.Render(pairs[i]) + " " + muted.Render(pairs[i+1]))
	}
	return b.String()
}

// acBanner paints the ACCEPTED banner: the frame and its "exit 0" in the
// OK hue, the lettering strong. A hue never paints bold (ui-system R3),
// and the weak hues paint frames, not words (R2).
func acBanner(art []string) []string {
	out := make([]string, len(art))
	for i, l := range art {
		var b, run strings.Builder
		frame := false
		flush := func() {
			if run.Len() == 0 {
				return
			}
			if frame {
				b.WriteString(ok.Render(run.String()))
			} else {
				b.WriteString(strong.Render(run.String()))
			}
			run.Reset()
		}
		for _, r := range l {
			// The light box runes and the frame's label belong to the
			// frame; the heavy lettering does not.
			f := strings.ContainsRune("┌┐└┘─│", r) || i == len(art)-1
			if f != frame {
				flush()
				frame = f
			}
			run.WriteRune(r)
		}
		flush()
		out[i] = b.String()
	}
	return out
}
