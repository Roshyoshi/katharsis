package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// HP is the problem's health bar, the one drawn per pane: tests passed out
// of the full set, then a note the caller builds (the verdict, the judging
// count, the best submit).
//
//	HP ░░░░░░░░░░░░░░░░░░░░ not submitted
//	HP ▓▓▓▓▓▓▓▓▓░░░░░░░░░░░ 14/30  judging 22/30…
//	HP ▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░ 16/30  WA #17 · best 20/30
//
// total 0 means nothing was submitted. The fill is OK, since passed tests
// are progress; failure is the note's to show. The bar takes up to 20
// cells and gives way to the note down to 6; past that the note is cut.
func HP(w, passed, total int, note string) string {
	count := Muted.Render("not submitted")
	cw := len("not submitted")
	if total > 0 {
		c := strconv.Itoa(passed) + "/" + strconv.Itoa(total)
		count, cw = c, len(c)
	}
	if note != "" {
		note = "  " + note
	}
	label := Muted.Render("HP") + " "
	bw := min(max(w-3-cw-1-ansi.StringWidth(note), 6), 20)
	filled := 0
	if total > 0 {
		filled = min(max(passed, 0)*bw/total, bw)
	}
	bar := OK.Render(strings.Repeat(G.Fill, filled)) + Muted.Render(strings.Repeat(G.Empty, bw-filled))
	return Clip(label+bar+" "+count+note, w)
}
