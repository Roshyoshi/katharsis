package boot

import (
	"katharsis/internal/game"
	"katharsis/internal/ui"
)

// The engine's notices, in the door's own terms: D44's timeout rule and
// RT8's easing. Each is placed on the screens already (the door cards'
// modifier lines, the HUD bar, the lines above the cards, the strip), so
// these functions only say what the engine decided.

// timeoutNote is the D44 line for the doors of a biome whose timeout costs
// a life rather than the boot (State.TimeoutCostsLife): never "easy mode"
// (GR9). ok is false everywhere else, and once the life a timeout would
// take is the last one, since then it ends the boot.
func timeoutNote(st *game.State) (text string, ok bool) {
	if !st.TimeoutCostsLife() {
		return "", false
	}
	return "a timeout costs " + ui.G.Life + ", not the boot", true
}

// easing is the boot's announced easing (RT8): what the engine eased and
// why, one line each, in task terms that name the boots ("Userland aim
// retuned · +1 ♥ (the last 2 boots ended there)"; GR9: never "easy",
// "assist" or "mercy"), and the action x sends to decline it, nil once it
// can no longer be declined. Announced only when it happened (GR3).
func easing(_ *game.State) (lines []string, decline game.Action) {
	// SPONGE: RT8 waits on the engine lane's State.Ease reasons and its
	//         decline action (x at the first room); until they land a
	//         boot has no easing, so nothing shows and x does nothing.
	return nil, nil
}
