package boot

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/ui"
)

// The boot screens' own layout policy: indents, centering, joins. The
// visual system itself (tokens, glyphs, frames, text helpers) is
// internal/ui.

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

// squeeze clips s to the cells w leaves beside the rest of its row, the
// two-cell gap included, down to a stub: a one-line row cuts its title,
// never its numbers (§2.6: the text column is cut, numerals never are).
func squeeze(s string, w int, rest ...string) string {
	room := w - 2
	for _, r := range rest {
		room -= widthOf(r)
	}
	if room < 4 {
		return s
	}
	return ui.Clip(s, room)
}

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
	return slices.Concat(head, make([]string, pad), block)
}
