package rtfm

import (
	"katharsis/internal/game"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// ClockFace draws an RTFM practice problem's clock (game.PracticeClock):
// "⏱ 12:34" in the body colour while it runs, green once solved without
// the editorial; red at the mark the editorial stopped it, then the time
// from there on, to the AC once solved, in yellow: "⏱ 22:10 +09:12".
// paint is a token's Render, or its Tmux for the editor's border. A
// problem solved before its clock ran has nothing to show: "".
func ClockFace(c game.PracticeClock, solved bool, paint func(ui.Token, string) string) string {
	switch {
	case c.Editorial > 0:
		return paint(ui.Danger, ui.G.Stopwatch+units.Clock(c.Editorial)) + " " +
			paint(ui.Warn, "+"+units.Clock(c.Spent-c.Editorial))
	case solved && c.Spent == 0:
		return ""
	case solved:
		return paint(ui.OK, ui.G.Stopwatch+units.Clock(c.Spent))
	}
	return paint(ui.FG, ui.G.Stopwatch+units.Clock(c.Spent))
}
