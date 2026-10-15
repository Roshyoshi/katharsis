package meta_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/screens/meta"
)

func TestPriorKeys(t *testing.T) {
	for _, tc := range []struct {
		keys []string
		want meta.PriorAnswer
	}{
		{[]string{"1", "4", "5", "0", "enter"}, meta.PriorAnswer{CF: 1450}},
		{[]string{"1", "9", "x", "0", "0", "7", "enter"}, meta.PriorAnswer{CF: 1900}}, // 4 digits at most
		{[]string{"2", "1", "backspace", "6", "0", "0", "enter"}, meta.PriorAnswer{CF: 2600}},
		{[]string{"0", "0", "8", "enter"}, meta.PriorAnswer{CF: 8}}, // no leading zeros
		{[]string{"enter"}, meta.PriorAnswer{}},
		{[]string{"1", "2", "esc"}, meta.PriorAnswer{}},
	} {
		wantMsg(t, press(t, meta.NewPrior(), tc.keys...), tc.want)
	}
}

func TestPriorView(t *testing.T) {
	for _, sz := range sizes {
		p := meta.NewPrior()
		press(t, p, "1", "4", "5", "0")
		golden(t, fmt.Sprintf("prior_%dx%d", sz[0], sz[1]), p.View(sz[0], sz[1]))
	}
	for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {80, 24}, {220, 60}} {
		lines := strings.Split(meta.NewPrior().View(sz[0], sz[1]), "\n")
		if len(lines) != sz[1] {
			t.Errorf("%dx%d: %d lines", sz[0], sz[1], len(lines))
		}
		for i, l := range lines {
			if n := ansi.StringWidth(l); n > sz[0] {
				t.Errorf("%dx%d: line %d is %d wide", sz[0], sz[1], i, n)
			}
		}
	}
}
