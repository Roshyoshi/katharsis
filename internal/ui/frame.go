package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// FrameKind is what a frame says about its contents (ui-system.md §2.5).
type FrameKind uint8

const (
	FrameIdle           FrameKind = iota // thin, Muted: a choosable card, a region
	FrameSelected                        // thick, Focus: the card under the cursor
	FrameCursed                          // thin, Danger
	FrameCursedSelected                  // thick, Danger: cursed keeps its color when selected
	FrameBoss                            // double, Special: the climax
	FrameBossSelected                    // double, Special and bold
	FrameDone                            // thin, OK: a region already cleared
	FrameHere                            // thin, Focus: the region you are in
)

// border is the glyphs and paint of a kind. Selection thickens the frame
// and emboldens its paint; a bold edge is an edge, not words, so it does
// not compete with the text inside.
func (k FrameKind) border() (lipgloss.Border, lipgloss.Style) {
	switch k {
	case FrameSelected:
		return G.Thick, Focus.Style().Bold(true)
	case FrameCursed:
		return G.Thin, Danger.Style()
	case FrameCursedSelected:
		return G.Thick, Danger.Style().Bold(true)
	case FrameBoss:
		return G.Double, Special.Style()
	case FrameBossSelected:
		return G.Double, Special.Style().Bold(true)
	case FrameDone:
		return G.Thin, OK.Style()
	case FrameHere:
		return G.Thin, Focus.Style()
	}
	return G.Thin, Muted.Style()
}

// Render paints s the way k paints its edges, for a label that must match
// them: the selected card's "▸ 1" in its top edge.
func (k FrameKind) Render(s string) string {
	if s == "" {
		return ""
	}
	_, p := k.border()
	return p.Render(s)
}

// Frame draws body inside a frame of kind k, exactly w cells wide, with one
// cell of padding each side. title and tag sit in the top edge, left and
// right, the way TUI dialogs label themselves. The tag names a danger, so
// when the edge is short the title is clipped first; the tag goes only
// when the title would shrink to a stub.
func Frame(k FrameKind, w int, title, tag string, body []string) []string {
	b, p := k.border()
	inner := max(w-2, 2)
	edge := inner - 2 // one rule glyph kept at each end of the top edge
	label := func(s string, n int) string {
		if s == "" || n < 3 {
			return ""
		}
		return " " + Clip(s, n-2) + " "
	}
	right := label(tag, edge)
	if ansi.StringWidth(title)+2+ansi.StringWidth(right) > edge && edge-ansi.StringWidth(right) < 10 {
		right = ""
	}
	left := label(title, edge-ansi.StringWidth(right))
	fill := edge - ansi.StringWidth(left) - ansi.StringWidth(right)
	top := p.Render(b.TopLeft+b.Top) + left
	if fill > 0 {
		top += p.Render(strings.Repeat(b.Top, fill))
	}
	out := make([]string, 0, len(body)+2)
	out = append(out, top+right+p.Render(b.Top+b.TopRight))
	for _, l := range body {
		out = append(out, p.Render(b.Left)+" "+Fit(l, inner-2)+" "+p.Render(b.Right))
	}
	out = append(out, p.Render(b.BottomLeft+strings.Repeat(b.Bottom, inner)+b.BottomRight))
	return out
}
