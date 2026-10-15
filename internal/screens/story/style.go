package story

import (
	"strings"

	"charm.land/lipgloss/v2"

	"katharsis/internal/ui"
)

// Every style the story screens use is here, on the shared internal/ui
// tokens (docs/design/ui-system.md §2.2, §2.6): narration is the flavor
// Voice (italic), titles are Strong (bold), asides are Muted (faint), and
// the ACCEPTED frame is OK. Each is read as a scene is built or drawn,
// not at init, so the palette the app picked (Mono under NO_COLOR)
// applies.

func voice() lipgloss.Style  { return ui.Voice.Style() }
func strong() lipgloss.Style { return ui.Strong.Style() }
func muted() lipgloss.Style  { return ui.Muted.Style() }
func plain() lipgloss.Style  { return ui.FG.Style() }
func danger() lipgloss.Style { return ui.Danger.Style() }

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
				b.WriteString(ui.OK.Render(run.String()))
			} else {
				b.WriteString(ui.Strong.Render(run.String()))
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
