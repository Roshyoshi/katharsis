package boot

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"deadcases/internal/catalog"
	"deadcases/internal/game"
)

// tone is how urgent the countdown looks.
type tone int

const (
	toneCalm tone = iota // more than half the timer left
	toneWarn             // more than a fifth left
	toneLow              // under a fifth
	toneCrit             // under two minutes: blinks
)

var toneStyle = map[tone]lipgloss.Style{
	toneCalm: sGreen,
	toneWarn: sYellow,
	toneLow:  sRed,
	toneCrit: sRed.Bold(true).Blink(true),
}

// timerTone picks the countdown color from the fraction of the door's
// timer left; the last two minutes blink whatever the fraction.
func timerTone(remaining, total time.Duration) tone {
	switch {
	case remaining < 2*time.Minute:
		return toneCrit
	case total <= 0:
		return toneCalm
	}
	switch f := float64(remaining) / float64(total); {
	case f > 0.5:
		return toneCalm
	case f > 0.2:
		return toneWarn
	}
	return toneLow
}

// HUDBar is the top of the in-problem HUD pane (35 to 60 columns): the
// countdown, lives, ducks and bytes; the door's modifiers; the tags as far
// as the ducks have revealed them; and the HP bar, the best full-test
// submit so far. Every line fits width.
func HUDBar(st *game.State, prob catalog.Problem, now time.Time, width int) string {
	if width <= 0 {
		width = 40
	}
	if st == nil {
		return sFaint.Render("no boot")
	}
	var att game.Attempt
	open := st.Current != nil
	if open {
		att = *st.Current
	}
	door := att.Door
	var out []string

	// Countdown, lives, ducks, bytes; the pending reward goes first when
	// the pane is too narrow.
	timer := sFaint.Render("⏱ --:--")
	if open {
		left := att.Deadline.Sub(now)
		timer = toneStyle[timerTone(left, door.Timer)].Render("⏱ " + fmtCountdown(left))
	}
	ducks := fmt.Sprintf("🦆%d", st.Ducks)
	if door.Cursed {
		ducks = sFaint.Render("🦆✗")
	}
	status := timer + "  " + hearts(st.Lives, st.MaxLives) + "  " + ducks + "  " +
		sGreen.Render(FormatBytes(st.Uncommitted))
	if reward := sFaint.Render(" +" + FormatBytes(door.Bytes)); door.Bytes > 0 && widthOf(status+reward) <= width {
		status += reward
	}
	out = append(out, status)

	// Modifiers, packed greedily into as few lines as fit.
	var mods []string
	if door.Boss != "" {
		mods = append(mods, sMagenta.Bold(true).Render("⚔ "+bossLabel(door.Boss)))
	}
	if door.Cursed {
		mods = append(mods, sRed.Bold(true).Render("☠ cursed: 1 WA = crash"))
	}
	if door.Mystery {
		mods = append(mods, sMagenta.Render("? mystery"))
	} else if door.Blind {
		mods = append(mods, sCyan.Render("◌ blind"))
	}
	if door.Regression {
		mods = append(mods, sYellow.Render("↺ regression"))
	}
	out = append(out, pack(mods, sFaint.Render(" · "), width)...)

	// Tags: hidden on blind doors until the first quack; the second quack
	// adds the technique.
	hidden := door.Blind || door.Mystery || st.Rules.BlindAll
	switch {
	case hidden && att.Quacks < 1 && door.Cursed:
		out = append(out, sFaint.Render("tags hidden · cursed: no ducks"))
	case hidden && att.Quacks < 1:
		out = append(out, sFaint.Render("tags hidden · 🦆 quack to reveal"))
	case len(prob.Tags) == 0:
		out = append(out, sFaint.Render("no tags"))
	default:
		for i, l := range wrapN(strings.Join(prob.Tags, " · "), width-5, 2) {
			pre := "     "
			if i == 0 {
				pre = sFaint.Render("tags ")
			}
			out = append(out, pre+sCyan.Render(l))
		}
	}
	if att.Quacks >= 2 && prob.Topic != "" {
		out = append(out, sFaint.Render("technique ")+sBold.Render(prob.Topic))
	}

	out = append(out, hpBar(att, width))
	for i, l := range out {
		out[i] = clip(l, width)
	}
	return strings.Join(out, "\n")
}

// hpBar is the problem's health from the player's side: tests passed by
// the best submit, out of the full set.
func hpBar(att game.Attempt, width int) string {
	count := "not submitted"
	if att.Total > 0 {
		count = fmt.Sprintf("%d/%d", att.BestPass, att.Total)
	}
	bw := min(max(width-widthOf(count)-1, 6), 20)
	filled := 0
	if att.Total > 0 {
		filled = min(max(att.BestPass, 0)*bw/att.Total, bw)
	}
	bar := sGreen.Render(strings.Repeat("▓", filled)) + sFaint.Render(strings.Repeat("░", bw-filled))
	s := bar + " " + count
	if att.Total == 0 {
		s = bar + " " + sFaint.Render(count)
	}
	if att.Submits > 0 {
		more := fmt.Sprintf("  %d submit", att.Submits)
		if att.Submits != 1 {
			more += "s"
		}
		if att.LastVerdict != "" {
			more += " · " + att.LastVerdict
		}
		if widthOf(s)+widthOf(more) <= width {
			s += sFaint.Render(more)
		}
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
