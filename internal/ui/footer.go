package ui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Footer is the key help on a screen's last row: pairs of a key and what
// it does, "←/→ select · 1-3 jump · q back", keys Strong and the rest
// Muted. List the keys as navigate, act, secondary, then back or quit.
// When w is short, pairs drop from the end, whole, but the last one, the
// way out, stays as long as any pair does.
func Footer(w int, pairs ...string) string {
	pick := fitPairs(pairs, w)
	if pick == nil && len(pairs) >= 2 {
		pick = pairs[len(pairs)/2*2-2 : len(pairs)/2*2] // the way out, clipped below
	}
	parts := make([]string, 0, len(pick)/2)
	for i := 0; i+1 < len(pick); i += 2 {
		parts = append(parts, Strong.Render(pick[i])+" "+Muted.Render(pick[i+1]))
	}
	return Clip(strings.Join(parts, Muted.Render(G.Sep)), w)
}

// StatusBar is a pager's prompt line (man, less), the bottom row of the
// RTFM screens and the reader: reverse, full width, where you are on the
// left and the keys on the right. Both are plain text, since the bar is
// styled as a whole. Keys give way as in Footer, then left is clipped.
func StatusBar(w int, left string, pairs ...string) string {
	if w <= 0 {
		return ""
	}
	left = " " + left
	line := Fit(left, w)
	if pick := fitPairs(pairs, w-ansi.StringWidth(left)-3); pick != nil {
		line = Spread(left, keyText(pick)+" ", w)
	}
	return Select.Render(Fit(line, w))
}

// StatusBarFits reports whether a StatusBar w cells wide shows left and
// every key pair whole, for a caller with a shorter left to fall back on.
func StatusBarFits(w int, left string, pairs ...string) bool {
	return 1+ansi.StringWidth(left)+2+ansi.StringWidth(keyText(pairs))+1 <= w
}

// fitPairs picks the key pairs that fit in room cells: all of them, or as
// many from the front as fit plus the last; nil when not even the last
// fits.
func fitPairs(pairs []string, room int) []string {
	n := len(pairs) / 2
	for k := n; k > 0; k-- {
		pick := pairs[:2*k]
		if k < n {
			pick = append(slices.Clip(pairs[:2*(k-1)]), pairs[2*(n-1):2*n]...)
		}
		if ansi.StringWidth(keyText(pick)) <= room {
			return pick
		}
	}
	return nil
}

// keyText is key pairs as plain text, for a bar that is styled as a whole.
func keyText(pairs []string) string {
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, pairs[i]+" "+pairs[i+1])
	}
	return strings.Join(parts, G.Sep)
}

// PagerLine is where a pager is, as less says it: "line 12/240", with
// "(END)" once the last line is on screen. top is the first line shown
// (0-based), rows how many show, total how many there are.
func PagerLine(top, rows, total int) string {
	total = max(total, 1)
	s := "line " + strconv.Itoa(min(top+1, total)) + "/" + strconv.Itoa(total)
	if top+rows >= total {
		s += " (END)"
	}
	return s
}
