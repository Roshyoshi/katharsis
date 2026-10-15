package screens

import (
	"fmt"

	"katharsis/internal/game"
)

// GateText says what unlocking the next prestige still needs (D31), top
// being the highest unlocked and name naming a prestige: "Silver unlocks
// after 60 rated doors (34 so far) and a clear at Bronze", or "Silver
// unlocks with a clear at Bronze" once enough doors are rated. It is ""
// when every prestige is unlocked.
func GateText(g game.PrestigeGate, top int, name func(int) string) string {
	switch {
	case g.Next == 0:
		return ""
	case g.Open():
		return fmt.Sprintf("%s unlocks with a clear at %s", name(g.Next), name(top))
	}
	return fmt.Sprintf("%s unlocks after %d rated doors (%d so far) and a clear at %s",
		name(g.Next), g.Need, g.Rated, name(top))
}
