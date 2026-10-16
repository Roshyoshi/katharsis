package anim

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/ui"
)

// packScreen is a boss pane as a screen draws it: a frame, a bar, a wide
// glyph and text.
var packScreen = []string{
	"Boot #57 · act II · the Court",
	"♥ 50/50 cycles  🦆0",
	"╔════════════════════════════════════╗",
	"║  T H E   C O U R T                 ║",
	"╚════════════════════════════════════╝",
	"HP ▓▓▓▓▓▓▓▓▓▓░░░░░░░░ 10/20",
	"──────────────────────────────────────",
	"judge: AC",
}

// The pack starts as the screen it lifts off, minus nothing, and ends as
// nothing: every card has left and the text gone dark. Between, some
// card has left its cell; a flight is the same every time for one seed.
func TestPack(t *testing.T) {
	p := Pack{From: packScreen, W: 40, H: 8, D: 900 * time.Millisecond, Seed: 7}
	c := p.Clip()
	first := plain(c.Frame(0))
	for i, l := range packScreen {
		if strings.TrimRight(first[i], " ") != l {
			t.Errorf("row %d at 0: %q, want %q", i, first[i], l)
		}
	}
	for i, l := range c.Frame(c.Len()) {
		if strings.TrimSpace(ansi.Strip(l)) != "" || ansi.StringWidth(l) != 40 {
			t.Errorf("row %d at the end: %q", i, l)
		}
	}
	mid := plain(c.Frame(c.Len() / 2))
	if slices.Equal(mid, first) || strings.Contains(strings.Join(mid, ""), "╔══") {
		t.Errorf("half way the frame is still whole:\n%s", strings.Join(mid, "\n"))
	}
	if !slices.Equal(c.Frame(c.Len()/2), p.Clip().Frame(c.Len()/2)) {
		t.Error("one seed flew two ways")
	}
	if other := (Pack{From: packScreen, W: 40, H: 8, D: p.D, Seed: 8}).Clip(); slices.Equal(other.Frame(c.Len()/2), c.Frame(c.Len()/2)) {
		t.Error("two seeds flew one way")
	}
}

// Under the ASCII glyphs the cards are the frames' + | = # and tumble in
// ASCII; no block glyph is drawn.
func TestPackASCII(t *testing.T) {
	ui.SetGlyphs(ui.ASCII)
	defer ui.SetGlyphs(ui.Unicode)
	screen := []string{"+==========+", "|  door 1  |", "+==========+", "text stays a while"}
	c := Pack{From: screen, W: 20, H: 4, D: 600 * time.Millisecond, Seed: 3}.Clip()
	for _, at := range samples(c) {
		for _, l := range plain(c.Frame(at)) {
			if strings.ContainsAny(l, "▀▄▌▐█") {
				t.Errorf("at %v a block glyph: %q", at, l)
			}
		}
	}
	if mid := strings.Join(plain(c.Frame(150*time.Millisecond)), "\n"); !strings.Contains(mid, "text stays") {
		t.Errorf("the text left too soon:\n%s", mid)
	}
}
