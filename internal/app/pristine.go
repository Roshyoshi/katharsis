package app

import (
	"os"

	"katharsis/internal/libs"
)

// pristine reports whether src is still the template the door started
// from, its linked libs pasted in: a source nobody has edited, whose
// samples could only fail. The HUD then waits for the first save before
// judging them (hud.Options.Unjudged, UX J13). A source it cannot read is
// judged as before.
func (r *TmuxRunner) pristine(d Door, src string) bool {
	b, err := os.ReadFile(src)
	return err == nil && string(b) == libs.Inject(r.template(d.Problem), d.Libs)
}
