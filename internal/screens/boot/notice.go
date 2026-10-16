package boot

import (
	"fmt"
	"math"
	"strings"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// The engine's notices, in the door's own terms: the prices of cycles
// (D47), D44's timeout rule and RT8's easing. Each is placed on the
// screens already (the door cards' modifier lines, the HUD bar, the lines
// above the cards, the strip, the end screens' kept block), so these
// functions only say what the engine decided. Every price is read from the
// engine's queries (game/cycles.go), so a screen never quotes one the
// engine does not charge, and each is said before it is paid.

// Cycles is the pool on a status line, "♥ 24/30 cycles" (D48: lives
// renamed cycles, still a heart): one heart and a count out of the pool
// the boot was dealt (State.StartCycles), the word Muted as the other
// labels are.
func Cycles(st *game.State) string {
	return cyclesShort(st) + ui.Muted.Render(" cycles")
}

// cyclesShort is Cycles without its word, for a line short of room.
func cyclesShort(st *game.State) string { return ui.Hearts(st.Cycles, st.MaxCycles) }

// Burned is a burn as it happened, for the strip and vim's :Submit reply:
// "burned 2 cycles, 22 left".
func Burned(n, left int) string {
	return "burned " + units.Plural(n, "cycle") + fmt.Sprintf(", %d left", max(left, 0))
}

// SubmitPrice is what a failed full submit of the open door will cost at
// now, said before it is made (the door's tip; the HUD bar once ECC cannot
// pay): "a fail burns 2 cycles"; under ECC's conversion "a fail costs
// 05:00 of the clock (ECC)"; on Off-by-One's door the near miss too, "a
// fail burns 2 cycles, 4 one test short"; where it ends the boot, why:
// "(cursed)", "(2 cycles left)"; and when ECC is held but the clock is too
// short to pay in (game.State.ECCShort), that this is why it burns cycles.
func SubmitPrice(st *game.State, now time.Time) string {
	cyc, clock := st.SubmitCost(0, now)
	near, nearClock := st.SubmitCost(1, now)
	switch {
	case st.Current != nil && st.Current.Door.Cursed:
		return "a fail ends the boot (cursed)"
	case clock > 0:
		s := "a fail costs " + units.Clock(clock) + " of the clock (ECC)"
		switch {
		case nearClock > clock:
			s += ", " + units.Clock(nearClock) + " one test short"
		case near > 0: // the near miss's double does not fit the clock
			s += ", " + units.Plural(near, "cycle") + " one test short"
		}
		return s
	}
	why := ""
	if st.ECCShort(0, now) {
		why = ECCShort
	}
	if cyc >= st.Cycles {
		left := units.Plural(st.Cycles, "cycle") + " left"
		if why != "" {
			left += "; " + why
		}
		return "a fail ends the boot (" + left + ")"
	}
	s := "a fail burns " + units.Plural(cyc, "cycle")
	if near > cyc {
		s += fmt.Sprintf(", %d one test short", near)
	}
	if why != "" {
		s += " (" + why + ")"
	}
	return s
}

// ECCShort says why a failed submit burns cycles though ECC is held: the
// door's clock is too short to pay in (game.State.ECCShort).
const ECCShort = "ECC: clock too short"

// inBoss reports whether the boot stands in a boss biome.
func inBoss(st *game.State) bool {
	return st.Biome >= 0 && st.Biome < len(st.Biomes) && st.Biomes[st.Biome].Kind == rules.Boss
}

// failNote is what running out the open door's clock, or :q!, will cost,
// for the door cards and the HUD bar: under cycles the room is lost and
// the boot goes on (D47), "a timeout or :q! burns 12 cycles", the act's
// price; before cycles D44's Userland rule, "a timeout burns 1 cycle, not
// the boot"; and when the burn would take the last cycles, that it ends
// the boot. A boss says BossPrice. ok is false where both end the boot
// whatever is left, which goes without saying: a boot saved before D44.
func failNote(st *game.State) (text string, ok bool) {
	if inBoss(st) {
		return BossPrice(st), true
	}
	tc, tok := st.FailCost(game.TimedOut)
	gc, gok := st.FailCost(game.GaveUp)
	switch {
	case tok && gok && tc == gc:
		return "a timeout or :q! burns " + units.Plural(tc, "cycle"), true
	case tok:
		return "a timeout burns " + units.Plural(tc, "cycle") + ", not the boot", true
	case tc > 0 && tc == gc:
		return "last cycles: a timeout or :q! ends the boot", true
	}
	return "", false
}

// BossPrice is a boss's rule under cycles, on its door card, its scene
// card and in the HUD bar (map.md §(f)): "a fail or :q! burns 20 cycles
// and deals a new boss door (22 left)"; a one-shot boss's, the final's and
// every boss's before cycles, "one shot: a fail or :q! ends the boot".
func BossPrice(st *game.State) string {
	c, ok := st.FailCost(game.TimedOut)
	switch {
	case st.OneShot() || c == 0:
		return "one shot: a fail or :q! ends the boot"
	case !ok:
		return fmt.Sprintf("a fail or :q! burns %d cycles, the last: it ends the boot", c)
	}
	return fmt.Sprintf("a fail or :q! burns %d cycles and deals a new boss door (%d left)", c, st.Cycles-c)
}

// bossPriceShort is BossPrice for the HUD bar's 36 columns: "fail or :q!
// -20, new door, 4 left"; "one shot: a fail or :q! ends it".
func bossPriceShort(st *game.State) string {
	c, ok := st.FailCost(game.TimedOut)
	switch {
	case st.OneShot() || c == 0:
		return "one shot: a fail or :q! ends it"
	case !ok:
		return fmt.Sprintf("fail or :q! -%d: the last, it ends", c)
	}
	return fmt.Sprintf("fail or :q! -%d, new door, %d left", c, st.Cycles-c)
}

// BossTwist reports whether a door of the boss biome bends its boss's rule
// (game.Door.Twist), so its card names the rule (boss.rule.<key>). The
// engine sets the twist on the doors it carries it out on; a boot without
// twists names none.
func BossTwist(st *game.State) (key string, ok bool) {
	doors := st.Doors
	if c := st.Current; c != nil {
		doors = append([]game.Door{c.Door}, doors...)
	}
	for _, d := range doors {
		if d.Twist != "" {
			return d.Twist, true
		}
	}
	return "", false
}

// twistNote is the rule a boss door bends (D54), in the boot's numbers,
// for the HUD bar and under the boss's own line on its card (the story's
// boss.rule.<key> says it in the boss's voice; this is the mechanic):
// "memory ×0.5, samples without ASan" (they are held to it, and
// ASan's redzones would read as memory: judge.Toolchain.Hardened). "" for
// a door that bends none.
func twistNote(st *game.State, d game.Door) string {
	t := st.Rules.Twists
	switch d.Twist {
	case "off-by-one":
		return "one test short burns " + fmtMult(float64(max(t.OffByOne, 1)))
	case "oom-killer":
		if d.MemMult > 0 {
			return "memory " + fmtMult(d.MemMult) + ", samples without ASan"
		}
	case "heisenbug":
		return "samples say only pass or fail"
	case "race-condition":
		if t.RaceClock > 0 {
			s := "clock " + fmtMult(math.Round(100/t.RaceClock)/100) + " fast"
			if t.RacePay > 0 {
				s += ", pays " + fmtMult(t.RacePay)
			}
			return s
		}
	case "undefined-behavior":
		return "samples run without sanitizers"
	case "halting-problem":
		return "clock hidden, warned at 05:00 and 01:00"
	}
	return ""
}

// HiddenClock reports whether the open door hides its countdown (The
// Halting Problem's twist): the HUD bar, the editor's border and its
// message line show ClockHidden instead. The strip's 5:00 and 1:00
// warnings still fire.
func HiddenClock(st *game.State) bool {
	return st.Current != nil && st.Current.Door.HiddenClock && st.Phase == game.PhaseProblem
}

// bossHidesClock reports whether boss's doors will hide their clock, for an
// exit's card before the engine has dealt a door (whose own flag is
// Door.HiddenClock): The Halting Problem, where the rules bend it so
// (rules.Twists.HaltingClock, as the engine's twist reads them).
func bossHidesClock(st *game.State, boss string) bool {
	return boss == game.FinalBoss && st.Rules.Twists.HaltingClock
}

// ClockHidden stands in for a hidden countdown: "??:??".
const ClockHidden = "??:??"

// RetreatLocked reports whether the open door's retreat has shut
// (rules.RetreatLock: kexec's free swap too), and how far into the door
// it shuts, for the keys row and the strip's door.retreat.locked.
func RetreatLocked(st *game.State, now time.Time) (after time.Duration, locked bool) {
	at := st.RetreatLockAt()
	if at.IsZero() || st.Current == nil || st.Phase != game.PhaseProblem {
		return 0, false
	}
	return at.Sub(st.Current.Opened), !now.Before(at)
}

// RetreatRefused is the strip's line for an R or a :Retreat past the
// open door's lock (door.retreat.locked, {time} the lock): no swap is
// left, kexec's included. ok is false while a retreat stands.
func RetreatRefused(st *game.State, now time.Time) (ev Event, ok bool) {
	after, locked := RetreatLocked(st, now)
	if !locked {
		return Event{}, false
	}
	t := units.Clock(after)
	return Event{
		At: now, Kind: EvRetreatLocked, Seed: st.Seed ^ uint64(now.Unix()),
		Text: ui.Warn.Render(ui.G.Retreat) + " retreat: locked " + t + " in" + ui.Muted.Render(ui.G.Sep+"solve it, or :q!"),
		Hook: "door.retreat.locked", Fill: []string{"time", t},
	}, true
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
	if es.Cycles > 0 {
		out = append(out, "+"+units.Plural(es.Cycles, "cycle"))
	}
	if es.Ducks > 0 {
		out = append(out, "+"+ui.G.Duck+fmt.Sprint(es.Ducks))
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
