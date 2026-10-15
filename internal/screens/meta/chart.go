package meta

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"katharsis/internal/ui"
)

// brailleChart draws vals (oldest first, at least two, within lo < hi) as a
// line in cols×rows cells. Each cell is a 2×4 braille dot matrix, so the
// line has 2·cols points across and 4·rows levels up. Values are resampled
// across the width by linear interpolation, and consecutive columns are
// joined vertically so steep moves stay one connected line. rank colours
// each cell by the value there, so the line changes colour where it
// crosses a rank boundary.
func brailleChart(vals []float64, cols, rows int, lo, hi float64, rank func(float64) ui.Token) []string {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	W, H, n := cols*2, rows*4, len(vals)
	at := make([]float64, W) // value at each dot column
	ys := make([]int, W)     // dot row, 0 at the bottom
	for x := range W {
		t := float64(x) * float64(n-1) / float64(W-1)
		i := min(int(t), n-2)
		f := t - float64(i)
		v := vals[i]*(1-f) + vals[i+1]*f
		at[x] = v
		y := int(math.Round((v - lo) / (hi - lo) * float64(H-1)))
		ys[x] = min(max(y, 0), H-1)
	}

	dots := make([][]bool, H) // [row from top][column]
	for i := range dots {
		dots[i] = make([]bool, W)
	}
	set := func(x, y int) { dots[H-1-y][x] = true }
	for x, y := range ys {
		set(x, y)
		if x == 0 {
			continue
		}
		for yy := min(ys[x-1], y) + 1; yy < max(ys[x-1], y); yy++ {
			set(x, yy)
		}
	}

	// Braille dot bits by (column, row) within a cell.
	bits := [2][4]rune{{0x01, 0x02, 0x04, 0x40}, {0x08, 0x10, 0x20, 0x80}}
	out := make([]string, rows)
	for cy := range rows {
		var b strings.Builder
		for cx := range cols {
			var r rune
			for dx := range 2 {
				for dy := range 4 {
					if dots[cy*4+dy][cx*2+dx] {
						r |= bits[dx][dy]
					}
				}
			}
			if r == 0 {
				b.WriteByte(' ')
				continue
			}
			b.WriteString(rank((at[cx*2] + at[cx*2+1]) / 2).Render(string(0x2800 + r)))
		}
		out[cy] = b.String()
	}
	return out
}

// chartRange rounds the data range out to multiples of 50, at least 100
// apart, so the axis labels are numbers a reader can hold.
func chartRange(vals []float64) (lo, hi float64) {
	lo = math.Floor(slices.Min(vals)/50) * 50
	hi = math.Ceil(slices.Max(vals)/50) * 50
	if hi-lo < 100 {
		mid := (hi + lo) / 2
		lo, hi = math.Floor((mid-50)/50)*50, math.Ceil((mid+50)/50)*50
	}
	return lo, hi
}

// axisChart puts a labelled y axis to the left of a braille chart in a
// total of cols×rows cells.
func axisChart(vals []float64, cols, rows int, rank func(float64) ui.Token) []string {
	lo, hi := chartRange(vals)
	labels := map[int]string{0: fmt.Sprintf("%.0f", hi), rows - 1: fmt.Sprintf("%.0f", lo)}
	if rows >= 5 {
		// A round middle value on the row nearest to it, rather than an
		// exact middle row with a value like 1225.
		mid := math.Round((hi+lo)/2/50) * 50
		row := int(math.Round((hi - mid) / (hi - lo) * float64(rows-1)))
		if row > 0 && row < rows-1 {
			labels[row] = fmt.Sprintf("%.0f", mid)
		}
	}
	lw := 0
	for _, l := range labels {
		lw = max(lw, len(l))
	}
	lines := brailleChart(vals, cols-lw-1, rows, lo, hi, rank)
	for i := range lines {
		axis := ui.Muted.Render("│")
		l, ok := labels[i]
		if ok {
			axis = ui.Muted.Render("┤")
		}
		lines[i] = ui.Muted.Render(ui.AlignRight(l, lw)) + axis + lines[i]
	}
	return lines
}

// sparkline draws the last width values as block heights.
func sparkline(vals []float64, width int) string {
	if width <= 0 || len(vals) == 0 {
		return ""
	}
	if len(vals) > width {
		vals = vals[len(vals)-width:]
	}
	lo, hi := slices.Min(vals), slices.Max(vals)
	blocks := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, v := range vals {
		i := len(blocks) - 1
		if hi > lo {
			i = int((v - lo) / (hi - lo) * float64(len(blocks)-1))
		}
		b.WriteRune(blocks[i])
	}
	return b.String()
}
