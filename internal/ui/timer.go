package ui

import (
	"time"

	"katharsis/internal/units"
)

// Timer is a door's countdown, "⏱ 23:41", toned by how much of total is
// left:
//
//	calm  more than half    OK
//	warn  more than a fifth Warn
//	low   the last fifth    Danger, bold
//	crit  under 2:00        Danger, reverse
//
// Low and crit add a shape to the color, so urgency reads without it.
func Timer(left, total time.Duration) string {
	return timerTone(left, total).Render(G.Timer + units.Countdown(left))
}

// BorderTimer is Timer for a tmux format, such as the editor pane's
// border: the same text and tone as tmux style directives.
func BorderTimer(left, total time.Duration) string {
	return timerTone(left, total).Tmux(G.Timer + units.Countdown(left))
}

func timerTone(left, total time.Duration) Token {
	switch {
	case left < 2*time.Minute:
		return timerCrit
	case total <= 0:
		return OK
	}
	switch f := float64(left) / float64(total); {
	case f > 0.5:
		return OK
	case f > 0.2:
		return Warn
	}
	return timerLow
}
