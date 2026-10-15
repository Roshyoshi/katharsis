package boot

import "katharsis/internal/game"

// Two notices wait on the engine lane. Each has its slot here, wired into
// the screens already, so landing the engine half changes one function
// and its test.

// timeoutNote is the D44 line for a door whose timeout costs a life rather
// than the boot, in the door's own terms: "a timeout here costs ♥, not
// the boot", never "easy mode" (GR9). ok is false for every other door.
func timeoutNote(_ *game.State, _ game.Door) (text string, ok bool) {
	// SPONGE: D44 waits on the engine lane: return the line for a door
	//         whose timeout the engine charges as a life (its Userland
	//         rule), read from the field it adds. Until it lands no door
	//         has one.
	return "", false
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
