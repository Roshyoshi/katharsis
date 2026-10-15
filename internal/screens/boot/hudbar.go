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

// HUDBar is the top of the in-problem HUD pane (35 to 60 columns): the
// countdown, lives, ducks and bytes; the door's modifiers; the tags as far
// as the ducks have revealed them; and the HP bar, the best full-test
// submit so far. Every line fits width.
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

	// Countdown, lives, ducks, bytes; the pending reward goes first when
	// the pane is too narrow. The bytes at stake are plain: they are lost
	// on a crash, so they must not read as a gain; the reward is one.
	timer := ui.Muted.Render(ui.G.Timer + "--:--")
	if open {
		timer = ui.Timer(att.Deadline.Sub(now), door.Timer)
	}
	ducks := fmt.Sprintf("%s%d", ui.G.Duck, st.Ducks)
	if door.Cursed {
		ducks = ui.Muted.Render(ui.G.Duck + ui.G.Fail)
	}
	status := timer + "  " + ui.Hearts(st.Lives, st.MaxLives) + "  " + ducks + "  " + units.Bytes(st.Uncommitted)
	if reward := " " + ui.OK.Render("+"+units.Bytes(door.Bytes)); door.Bytes > 0 && widthOf(status+reward) <= width {
		status += reward
	}
	out = append(out, status)

	// Modifiers, packed greedily into as few lines as fit.
	var mods []string
	if door.Boss != "" {
		mods = append(mods, ui.Modifier(ui.ModBoss, bossLabel(door.Boss)))
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
	out = append(out, pack(mods, ui.Muted.Render(ui.G.Sep), width)...)

	// Tags: hidden on blind doors and mastered topics until the duck reveals
	// them (level 1); level 2 adds the technique. This keys on Hint, the
	// level shown, not on Quacks: a door whose tags are already visible
	// spends its first quack on the technique, so one quack can mean level 2.
	hidden := door.Blind || door.Mystery || door.Mastered || st.Rules.BlindAll
	switch {
	case hidden && att.Hint < 1 && door.Cursed:
		out = append(out, ui.Muted.Render("tags hidden · cursed: no ducks"))
	case hidden && att.Hint < 1:
		out = append(out, ui.Muted.Render("tags hidden · "+strings.TrimSpace(ui.G.Duck)+" quack to reveal"))
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
	if att.Hint >= 2 && prob.Topic != "" {
		out = append(out, ui.Muted.Render("technique ")+ui.Strong.Render(prob.Topic))
	}

	out = append(out, hpBar(att, width))
	for i, l := range out {
		out[i] = ui.Clip(l, width)
	}
	return strings.Join(out, "\n")
}

// hpBar is the problem's health from the player's side: tests passed by
// the best submit, out of the full set.
func hpBar(att game.Attempt, width int) string {
	// A submit being judged (Attempt.SubmitAt) shows here too: the HUD
	// below read "judging 5/10" while this bar said "not submitted".
	judging := !att.SubmitAt.IsZero()
	count := "not submitted"
	switch {
	case att.Total > 0:
		count = fmt.Sprintf("%d/%d", att.BestPass, att.Total)
	case judging:
		count = "judging…"
	}
	// The bar gives way to the word that a submit is in flight.
	room := width - widthOf(count) - 1
	if judging && att.Total > 0 {
		room -= widthOf("  judging…")
	}
	bw := min(max(room, 6), 20)
	filled := 0
	if att.Total > 0 {
		filled = min(max(att.BestPass, 0)*bw/att.Total, bw)
	}
	bar := ui.OK.Render(strings.Repeat(ui.G.Fill, filled)) + ui.Muted.Render(strings.Repeat(ui.G.Empty, bw-filled))
	s := bar + " " + count
	if att.Total == 0 {
		s = bar + " " + ui.Muted.Render(count)
	}
	more := ""
	if att.Submits > 0 {
		more = "  " + units.Plural(att.Submits, "submit")
		if att.LastVerdict != "" {
			more += ui.G.Sep + string(att.LastVerdict)
		}
	}
	if judging && att.Total > 0 {
		// The submit in flight comes first when both do not fit.
		if more += ui.G.Sep + "judging…"; widthOf(s)+widthOf(more) > width {
			more = "  judging…"
		}
	}
	if more != "" && widthOf(s)+widthOf(more) <= width {
		s += ui.Muted.Render(more)
	}
	return s
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
