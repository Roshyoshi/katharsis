package boot

import (
	"fmt"
	"strings"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// HUDBar is the top of the in-problem HUD pane (35 to 60 columns), its
// vitals: the countdown (hidden on The Halting Problem's doors), cycles,
// ducks and bytes; the door's modifiers, a boss's twist among them; what a
// timeout or :q! burns, and a failed submit once ECC's clock is too short
// to pay it; the tags as far as the ducks have revealed them. Every line
// fits width. The pane's one HP bar is the HUD's, or the problem screen's
// without a HUD (C6).
func HUDBar(st *game.State, prob catalog.Problem, now time.Time, width int) string {
	if width <= 0 {
		width = 40
	}
	var att game.Attempt
	open := st.Current != nil
	if open {
		att = *st.Current
	}
	door := att.Door
	var out []string

	// Countdown, cycles, ducks, bytes; the pending reward goes first when
	// the pane is too narrow, then the word "cycles". The bytes at stake
	// are plain: they are lost on a crash, so they must not read as a
	// gain; the reward is one.
	var timer string
	switch left := att.Deadline.Sub(now); {
	case !open:
		timer = ui.Muted.Render(ui.G.Timer + "--:--")
	case HiddenClock(st):
		// The Halting Problem hides its clock (D54): you cannot tell
		// whether it will finish. The strip still warns at 5:00 and 1:00.
		timer = ui.Muted.Render(ui.G.Timer + ClockHidden)
	case left < 0 && st.SoftTimer():
		// A soft timer (a trial) goes on past its deadline at less pay:
		// how far over, as the editor pane's border says it.
		timer = ui.Danger.Render(ui.G.Timer + "+" + units.Clock(-left) + " over")
	default:
		timer = ui.Timer(left, door.Timer)
	}
	ducks := fmt.Sprintf("%s%d", ui.G.Duck, st.Ducks)
	if door.Cursed {
		ducks = ui.Muted.Render(ui.G.Duck + ui.G.Fail)
	}
	rest := "  " + ducks + "  " + units.Bytes(st.Uncommitted)
	status := timer + "  " + Cycles(st) + rest
	if widthOf(status) > width {
		status = timer + "  " + cyclesShort(st) + rest
	}
	if reward := " " + ui.OK.Render("+"+units.Bytes(door.Bytes)); door.Bytes > 0 && widthOf(status+reward) <= width {
		status += reward
	}
	out = append(out, status)
	out = append(out, dumpAtRisk(st, width)...) // ☠ the core dump a crash destroys (debrief.go, RT4)

	// Modifiers, packed greedily into as few lines as fit.
	var mods []string
	if door.Boss != "" {
		boss := bossLabel(door.Boss)
		if t := twistNote(st, door); t != "" {
			boss += ": " + t
		}
		mods = append(mods, ui.Modifier(ui.ModBoss, boss))
	}
	if door.Cursed {
		mods = append(mods, ui.Modifier(ui.ModCursed, "cursed: 1 WA = crash"))
	}
	if door.Mystery {
		mods = append(mods, ui.Modifier(ui.ModMystery, ""))
	} else if door.Blind {
		mods = append(mods, ui.Modifier(ui.ModBlind, ""))
	}
	if door.Regression {
		mods = append(mods, ui.Modifier(ui.ModRegression, ""))
	}
	// A part too wide to pack (a boss's twist) wraps on lines of its own.
	if len(mods) > 0 && door.Boss != "" && widthOf(mods[0]) > width {
		boss := bossLabel(door.Boss) + ": " + twistNote(st, door)
		out = append(out, ui.ModifierLines(ui.ModBoss, boss, width, 2)...)
		mods = mods[1:]
	}
	out = append(out, pack(mods, ui.Muted.Render(ui.G.Sep), width)...)
	// What a timeout or :q! burns, the act's price (D47), or a boss's rule.
	t, ok := failNote(st)
	if inBoss(st) {
		t = bossPriceShort(st)
	}
	if ok && open {
		for _, l := range ui.WrapN(ui.G.Timer+t, width, 2) {
			out = append(out, ui.Muted.Render(l))
		}
	}
	// Under ECC, a failed submit's price once the clock is too short to pay
	// it in, Off-by-One's near miss included: from then on it burns cycles.
	// On a hidden clock this comes as the strip's 05:00 warning does, at
	// ECC's 5:00.
	if open && !door.Cursed && st.ECCShort(1, now) {
		for _, l := range ui.WrapN(SubmitPrice(st, now), width, 2) {
			out = append(out, ui.Warn.Render(l))
		}
	}

	// Tags: hidden on blind doors and mastered topics until the duck reveals
	// them (level 1); level 2 adds the technique. This keys on Hint, the
	// level shown, not on Quacks: a door whose tags are already visible
	// spends its first quack on the technique, so one quack can mean level 2.
	hidden := door.Blind || door.Mystery || door.Mastered || st.Rules.BlindAll
	switch {
	case hidden && att.Hint < 1 && door.Cursed:
		out = append(out, ui.Muted.Render("tags hidden"+ui.G.Sep+"cursed: no ducks"))
	case hidden && att.Hint < 1:
		out = append(out, ui.Muted.Render("tags hidden"+ui.G.Sep+strings.TrimSpace(ui.G.Duck)+" quack to reveal"))
	case len(prob.Tags) == 0:
		out = append(out, ui.Muted.Render("no tags"))
	default:
		for i, l := range ui.WrapN(strings.Join(prob.Tags, ui.G.Sep), width-5, 2) {
			pre := "     "
			if i == 0 {
				pre = ui.Muted.Render("tags ")
			}
			out = append(out, pre+l)
		}
	}
	switch {
	case att.Hint >= 2 && st.Rules.DuckTest:
		out = append(out, ui.Muted.Render("duck: a failed test is in your samples (w)"))
	case att.Hint >= 2 && prob.Topic != "":
		out = append(out, ui.Muted.Render("technique ")+ui.Strong.Render(prob.Topic))
	}

	for i, l := range out {
		out[i] = ui.Clip(l, width)
	}
	return strings.Join(out, "\n")
}

// pack joins parts with sep into lines no wider than width.
func pack(parts []string, sep string, width int) []string {
	var out []string
	cur := ""
	for _, p := range parts {
		switch {
		case cur == "":
			cur = p
		case widthOf(cur)+widthOf(sep)+widthOf(p) <= width:
			cur += sep + p
		default:
			out = append(out, cur)
			cur = p
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
