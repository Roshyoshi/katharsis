package boot

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/ui"
)

// The s* styles and the helpers below forward to internal/ui while the
// screens migrate onto it (phase C part 2); they go once nothing calls
// them.
var (
	sBold   = ui.Strong.Style()
	sFaint  = ui.Muted.Style()
	sItalic = ui.Voice.Style()
	sRed    = ui.Danger.Style()
	sGreen  = ui.OK.Style()
)

func spread(left, right string, w int) string { return ui.Spread(left, right, w) }
func wrapN(s string, w, n int) []string       { return ui.WrapN(s, w, n) }
func layout(width, height int, body []string, footer string) string {
	return ui.Layout(width, height, body, footer)
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

// fmtMult renders a byte multiplier: ×2, ×1.5.
func fmtMult(m float64) string {
	return ui.G.Times + strconv.FormatFloat(m, 'f', -1, 64)
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
