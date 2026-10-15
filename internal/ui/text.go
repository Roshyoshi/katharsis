package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Text helpers. Every width here is in terminal cells (ansi.StringWidth),
// ANSI styling passes through, and a cut is marked with G.Ellipsis.

// Clip shortens s to at most w cells.
func Clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, G.Ellipsis)
}

// Fit clips s to w cells and pads it on the right to exactly w.
func Fit(s string, w int) string {
	s = Clip(s, w)
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}

// AlignRight clips s to w cells and pads it on the left to exactly w, for
// numerals in a table column.
func AlignRight(s string, w int) string {
	s = Clip(s, w)
	return strings.Repeat(" ", max(w-ansi.StringWidth(s), 0)) + s
}

// Spread puts left and right at the two ends of a w-cell line, at least
// two cells apart. When both do not fit, right goes: it always carries the
// less important half.
func Spread(left, right string, w int) string {
	lw, rw := ansi.StringWidth(left), ansi.StringWidth(right)
	if right == "" || lw+2+rw > w {
		return Clip(left, w)
	}
	return left + strings.Repeat(" ", w-lw-rw) + right
}

// Center pads s on the left so it sits in the middle of w cells, clipping
// it when it is wider.
func Center(s string, w int) string {
	return strings.Repeat(" ", max((w-ansi.StringWidth(s))/2, 0)) + Clip(s, w)
}

// WrapN word-wraps s to w cells and keeps at most n lines, ending the last
// kept line with an ellipsis when text was dropped; n <= 0 keeps every
// line. A word longer than w is broken.
func WrapN(s string, w, n int) []string {
	if s == "" {
		return nil
	}
	ls := strings.Split(ansi.Wrap(s, max(w, 1), ""), "\n")
	if n > 0 && len(ls) > n {
		ls = ls[:n]
		ls[n-1] = Clip(ls[n-1]+" "+G.Ellipsis, w)
	}
	return ls
}

// JoinH lays blocks side by side, gap cells apart. Each block's lines are
// one width; a shorter block is padded below with blank lines of its
// width.
func JoinH(gap int, blocks ...[]string) []string {
	h := 0
	for _, b := range blocks {
		h = max(h, len(b))
	}
	sp := strings.Repeat(" ", gap)
	out := make([]string, h)
	for i := range h {
		var sb strings.Builder
		for j, b := range blocks {
			if j > 0 {
				sb.WriteString(sp)
			}
			if i < len(b) {
				sb.WriteString(b[i])
			} else if len(b) > 0 {
				sb.WriteString(strings.Repeat(" ", ansi.StringWidth(b[0])))
			}
		}
		out[i] = sb.String()
	}
	return out
}

// PadLines grows ls with blank lines to n.
func PadLines(ls []string, n int) []string {
	for len(ls) < n {
		ls = append(ls, "")
	}
	return ls
}

// Rule is a horizontal rule w cells wide, Muted: the line between a
// header and a body, or between sections.
func Rule(w int) string { return Muted.Render(strings.Repeat(G.Rule, max(w, 0))) }
