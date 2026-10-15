package boot

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/units"
)

// The palette is the terminal's own 16 ANSI colors, so every screen follows
// the user's theme (light or dark) instead of imposing one.
var (
	red     = lipgloss.Color("1")
	green   = lipgloss.Color("2")
	yellow  = lipgloss.Color("3")
	magenta = lipgloss.Color("5")
	cyan    = lipgloss.Color("6")
)

var (
	sBold    = lipgloss.NewStyle().Bold(true)
	sFaint   = lipgloss.NewStyle().Faint(true)
	sItalic  = lipgloss.NewStyle().Italic(true)
	sRed     = lipgloss.NewStyle().Foreground(red)
	sGreen   = lipgloss.NewStyle().Foreground(green)
	sYellow  = lipgloss.NewStyle().Foreground(yellow)
	sMagenta = lipgloss.NewStyle().Foreground(magenta)
	sCyan    = lipgloss.NewStyle().Foreground(cyan)
)

// frame is a box: its glyphs and how its edges are painted. Selection
// changes the glyphs (thin to thick), not just the color, so it still reads
// on a monochrome terminal.
type frame struct {
	border lipgloss.Border
	paint  lipgloss.Style
}

var (
	frameIdle      = frame{lipgloss.RoundedBorder(), sFaint}
	frameSelected  = frame{lipgloss.ThickBorder(), sCyan.Bold(true)}
	frameCursed    = frame{lipgloss.RoundedBorder(), sRed}
	frameCursedSel = frame{lipgloss.ThickBorder(), sRed.Bold(true)}
	frameBoss      = frame{lipgloss.DoubleBorder(), sMagenta}
	frameBossSel   = frame{lipgloss.DoubleBorder(), sMagenta.Bold(true)}
	frameDone      = frame{lipgloss.RoundedBorder(), sGreen}
	frameHere      = frame{lipgloss.RoundedBorder(), sYellow.Bold(true)}
)

// box draws body inside f, exactly w cells wide. title and tag sit in the
// top edge, left and right, the way TUI dialogs label themselves. The tag
// names a danger, so when the edge is short the title is clipped first;
// the tag goes only when the title would shrink to a stub.
func box(f frame, w int, title, tag string, body []string) []string {
	b, p := f.border, f.paint
	inner := max(w-2, 2)
	edge := inner - 2 // one rule glyph kept at each end of the top edge
	label := func(s string, n int) string {
		if s == "" || n < 3 {
			return ""
		}
		return " " + clip(s, n-2) + " "
	}
	right := label(tag, edge)
	if widthOf(title)+2+widthOf(right) > edge && edge-widthOf(right) < 10 {
		right = ""
	}
	left := label(title, edge-widthOf(right))
	fill := edge - widthOf(left) - widthOf(right)
	out := make([]string, 0, len(body)+2)
	out = append(out, p.Render(b.TopLeft+b.Top)+left+
		p.Render(strings.Repeat(b.Top, fill))+right+p.Render(b.Top+b.TopRight))
	for _, l := range body {
		out = append(out, p.Render(b.Left)+" "+fit(l, inner-2)+" "+p.Render(b.Right))
	}
	out = append(out, p.Render(b.BottomLeft+strings.Repeat(b.Bottom, inner)+b.BottomRight))
	return out
}

// fit clips s to w cells and pads it to exactly w.
func fit(s string, w int) string {
	s = clip(s, w)
	return s + strings.Repeat(" ", max(w-widthOf(s), 0))
}

// clip shortens s to at most w cells, marking the cut with an ellipsis.
func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if widthOf(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}

// spread puts left and right at the two ends of a w-cell line. When both
// don't fit, right goes; it always carries the less important half.
func spread(left, right string, w int) string {
	lw, rw := widthOf(left), widthOf(right)
	if right == "" || lw+2+rw > w {
		return clip(left, w)
	}
	return left + strings.Repeat(" ", w-lw-rw) + right
}

// center pads s on the left so it sits in the middle of w cells.
func center(s string, w int) string {
	return strings.Repeat(" ", max((w-widthOf(s))/2, 0)) + s
}

// indent prefixes every non-empty line with n spaces.
func indent(n int, ls []string) []string {
	pad := strings.Repeat(" ", max(n, 0))
	out := make([]string, len(ls))
	for i, l := range ls {
		if l != "" {
			out[i] = pad + l
		}
	}
	return out
}

// wrapN word-wraps s to w cells and keeps at most n lines, ending the last
// kept line with an ellipsis when text was dropped. n <= 0 keeps all.
func wrapN(s string, w, n int) []string {
	if s == "" {
		return nil
	}
	ls := strings.Split(ansi.Wrap(s, max(w, 1), ""), "\n")
	if n > 0 && len(ls) > n {
		ls = ls[:n]
		ls[n-1] = clip(ls[n-1]+" …", w)
	}
	return ls
}

// joinH lays equal-height blocks of equal-width lines side by side.
func joinH(gap int, blocks ...[]string) []string {
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
				sb.WriteString(strings.Repeat(" ", widthOf(b[0])))
			}
		}
		out[i] = sb.String()
	}
	return out
}

// padLines grows ls with blank lines to n.
func padLines(ls []string, n int) []string {
	for len(ls) < n {
		ls = append(ls, "")
	}
	return ls
}

// hint renders key help: pairs of key and what it does.
func hint(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, sBold.Render(pairs[i])+" "+sFaint.Render(pairs[i+1]))
	}
	return strings.Join(parts, sFaint.Render(" · "))
}

// layout is the last step of every View: each line clipped to width, the
// footer pinned to the bottom row, and the result exactly height lines, so
// a screen never wraps or scrolls the terminal.
func layout(width, height int, body []string, footer string) string {
	var out []string
	for _, l := range body {
		for _, s := range strings.Split(l, "\n") {
			out = append(out, clip(s, width))
		}
	}
	if height > 0 {
		room := height
		if footer != "" {
			room--
		}
		if len(out) > room {
			out = out[:max(room, 0)]
		}
		out = padLines(out, room)
	}
	if footer != "" {
		out = append(out, clip(footer, width))
	}
	return strings.Join(out, "\n")
}

// hearts colors units.Hearts: the lives left red, the lost ones faint.
func hearts(lives, maxLives int) string {
	h := units.Hearts(lives, maxLives)
	left := strings.TrimRight(h, "♡")
	return sRed.Render(left) + sFaint.Render(h[len(left):])
}

// fmtMult renders a byte multiplier: ×2, ×1.5.
func fmtMult(m float64) string {
	return "×" + strconv.FormatFloat(m, 'f', -1, 64)
}

func widthOf(s string) int { return ansi.StringWidth(s) }

// centerBlock indents lines as one block centered in width.
func centerBlock(ls []string, width int) []string {
	return indent(max((width-maxWidth(ls))/2, 0), ls)
}

func maxWidth(ls []string) int {
	w := 0
	for _, l := range ls {
		w = max(w, widthOf(l))
	}
	return w
}

// centered stacks head, then block centered vertically in the rows left
// above a one-line footer, keeping at least one blank line under head.
func centered(head, block []string, height int) []string {
	pad := 0
	if free := height - len(head) - 1 - len(block); free > 0 {
		pad = max(free/2, 1)
	}
	return append(append(append([]string(nil), head...), make([]string, pad)...), block...)
}

// concat joins slices into a new one.
func concat[T any](parts ...[]T) []T {
	var out []T
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
