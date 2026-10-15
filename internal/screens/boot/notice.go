package boot

import (
	"fmt"
	"strings"

	"katharsis/internal/game"
	"katharsis/internal/ui"
)

// The engine's notices, in the door's own terms: D44's timeout rule and
// RT8's easing. Each is placed on the screens already (the door cards'
// modifier lines, the HUD bar, the lines above the cards, the strip, the
// end screens' kept block), so these functions only say what the engine
// decided.

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

// easing is the boot's announced easing (RT8): one line per State.Ease,
// what it changed in task terms and, in brackets, the boots, doors or
// days behind it ("Userland aim retuned (first boot in 5 days)"), and the
// action x sends to decline it. GR9 rules the words: never "easy",
// "assist" or "mercy", and the reason names no player. D36 keeps any
// clear chance off it: it shows before a door. It speaks only before the
// boot's first door, the one time the easing can be turned down; once
// declined, each line says so and decline is nil. A boot with no easing
// has no lines (GR3: announced only when it happened).
func easing(st *game.State) (lines []string, decline game.Action) {
	if len(st.Ease) == 0 || !st.BeforeFirstDoor() {
		return nil, nil
	}
	for _, es := range st.Ease {
		l := strings.Join(easeTerms(st, es, false), ui.G.Sep) + ui.Muted.Render(" ("+es.Why+")")
		if st.Declined {
			l += ui.Muted.Render(ui.G.Sep + "declined")
		}
		lines = append(lines, l)
	}
	if !st.Declined {
		decline = game.DeclineEase{}
	}
	return lines, decline
}

// easeTerms is what es changed, a part each. after is set at the end of
// the boot, where the aims carry their numbers (D36 shows a clear chance
// only after the doors): "Userland aim 92% → 95%".
func easeTerms(st *game.State, es game.Ease, after bool) []string {
	var out []string
	if es.FirstDoor {
		out = append(out, "easiest doors in the first room, tags shown")
	}
	if es.Editorial {
		out = append(out, "every door has an editorial")
	}
	out = append(out, aimTerms(st, es, after)...)
	if es.Lives > 0 {
		out = append(out, fmt.Sprintf("+%d ", es.Lives)+ui.Life.Render(ui.G.Life))
	}
	if es.Ducks > 0 {
		out = append(out, fmt.Sprintf("+%d ", es.Ducks)+ui.G.Duck)
	}
	if es.Buyback {
		out = append(out, "longer timers on "+biomeLabel(st, 0)+"'s floor doors")
	}
	return out
}

// aimTerms names the biomes es aims anew: each by name, or every biome at
// once when it retunes them all (the governor).
func aimTerms(st *game.State, es game.Ease, after bool) []string {
	var aimed []int
	for i, c := range es.Clear {
		if c > 0 && i < len(st.Rules.Biomes) {
			aimed = append(aimed, i)
		}
	}
	pct := func(p float64) string { return fmt.Sprintf("%.0f%%", 100*p) }
	if len(aimed) > 1 && len(aimed) == len(st.Rules.Biomes) {
		if !after {
			return []string{"every biome's aim retuned"}
		}
		var from, to []string
		for _, i := range aimed {
			from = append(from, pct(st.Rules.Biomes[i].TargetClear))
			to = append(to, pct(es.Clear[i]))
		}
		return []string{"aims " + strings.Join(from, "/") + " " + ui.G.Arrow + " " + strings.Join(to, "/")}
	}
	var out []string
	for _, i := range aimed {
		name := biomeLabel(st, i)
		if name == "?" {
			name = fmt.Sprintf("biome %d", i+1)
		}
		if !after {
			out = append(out, name+" aim retuned")
			continue
		}
		out = append(out, name+" aim "+pct(st.Rules.Biomes[i].TargetClear)+" "+ui.G.Arrow+" "+pct(es.Clear[i]))
	}
	return out
}
