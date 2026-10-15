package story

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/ui"
)

type built struct {
	name string
	id   string // the seen id it records
	text []string
	make func() *Scene
}

// all is every scene, with its seen id and the lines it must show.
func all() []built {
	out := []built{
		{"prologue", "story.prologue", flavor.StoryBlock("story.prologue"), NewPrologue},
		{"first door", "door.first.inscription", []string{flavor.Story("door.first.inscription", 0), "hello, world"}, NewFirstDoor},
		{"first merge", "story.first-merge", flavor.StoryBlock("story.first-merge"), NewFirstMerge},
		{"first crash", "story.first-crash",
			append(flavor.StoryBlock("story.first-crash"), "a grin without a cat · 15.2 KiB still on Bovine Shuffle"),
			func() *Scene { return NewFirstCrash("15.2 KiB", "Bovine Shuffle") }},
		{"accepted", "", []string{"exit 0", "AC · 3/3"}, func() *Scene { return NewAccepted("AC · 3/3") }},
		{"first boss", "story.first-boss", flavor.StoryBlock("story.first-boss"),
			func() *Scene { return NewBossIntro("off-by-one", 0, true) }},
		{"first final boss", "reveal.final-boss", flavor.StoryBlock("reveal.final-boss"),
			func() *Scene { return NewBossIntro(game.FinalBoss, 0, true) }},
		{"boot splash", "", []string{"katharsis boot #42 (k&r@bell-labs)", "cpu: entering ring 1: the nameless wood"},
			func() *Scene { return NewBootSplash(42, 2) }},
		{"beat", "reveal.prestige.3", flavor.StoryBlock("reveal.prestige.3"), func() *Scene { return NewBeat("reveal.prestige.3") }},
	}
	for _, k := range append(append([]string{}, game.BossKeys...), game.FinalBoss) {
		enter := flavor.Story("boss.enter."+k, 5)
		out = append(out, built{"boss " + k, "", append([]string{flavor.BossBlurb(k)[:20]}, lines(enter)...),
			func() *Scene { return NewBossIntro(k, 5, false) }})
	}
	for _, t := range catalog.Topics {
		out = append(out, built{"biome " + t, "", []string{flavor.Story("biome.intro."+t, 0)},
			func() *Scene { return NewBiome(t, []string{"the room you leave"}) }})
	}
	return out
}

// frames is every page of s at size w×h, at its start, along it, and at
// its end.
func frames(s *Scene, w, h int) []string {
	s.View(w, h)
	var out []string
	for i := range s.Pages() {
		n := s.PageLen(i)
		for _, t := range []time.Duration{0, n / 4, n / 2, 3 * n / 4, n} {
			s.Seek(i, t)
			out = append(out, s.View(w, h))
		}
	}
	return out
}

// TestScenesFit: every frame of every scene is exactly the terminal's
// height and never wider, at 80x24 and in a small pane.
func TestScenesFit(t *testing.T) {
	for _, b := range all() {
		for _, size := range [][2]int{{80, 24}, {60, 18}, {100, 40}} {
			for _, f := range frames(b.make(), size[0], size[1]) {
				rows := strings.Split(f, "\n")
				if len(rows) != size[1] {
					t.Errorf("%s at %v: %d rows", b.name, size, len(rows))
				}
				for i, r := range rows {
					if w := ansi.StringWidth(r); w > size[0] {
						t.Errorf("%s at %v, row %d: %d cells: %q", b.name, size, i, w, ansi.Strip(r))
					}
				}
			}
		}
	}
}

// TestScenesSayTheirText: by the end of its last page, a scene has shown
// every line it owns, at 80 columns and rewrapped in a narrow pane (never
// cut); and it records the right seen id.
func TestScenesSayTheirText(t *testing.T) {
	// A card or a narrow pane wraps text, so compare words.
	words := func(s string) string { return strings.Join(strings.Fields(strings.ReplaceAll(s, "║", " ")), " ") }
	for _, b := range all() {
		if s := b.make(); s.ID() != b.id {
			t.Errorf("%s records %q, want %q", b.name, s.ID(), b.id)
		}
		for _, size := range [][2]int{{80, 24}, {50, 40}} {
			s := b.make()
			var shown strings.Builder
			s.View(size[0], size[1])
			for i := range s.Pages() {
				s.Seek(i, s.PageLen(i))
				shown.WriteString(ansi.Strip(s.View(size[0], size[1])) + "\n")
			}
			for _, l := range b.text {
				if l != "" && !strings.Contains(words(shown.String()), words(l)) {
					t.Errorf("%s at %v never shows %q", b.name, size, l)
				}
			}
		}
	}
}

func TestPrologueFallsAtDownDownDown(t *testing.T) {
	s := NewPrologue()
	if s.Pages() != 2 {
		t.Fatalf("%d pages, want the typing, then the fall", s.Pages())
	}
	s.View(80, 24)
	s.Seek(0, s.PageLen(0))
	if p := ansi.Strip(s.View(80, 24)); strings.Contains(p, "Down, down, down") || !strings.Contains(p, "Nobody checks") {
		t.Errorf("page 1 ends wrong:\n%s", p)
	}
	// Part way down the second page you are in the shaft.
	s.Seek(1, fallTime/2)
	if p := ansi.Strip(s.View(80, 24)); !strings.Contains(p, "|") || strings.Contains(p, "Pick a door") {
		t.Errorf("mid-fall:\n%s", p)
	}
}

// run executes cmd and returns the message it sends, skipping ticks: a
// story screen's Done is an immediate command.
func emitted(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func press(s *Scene, k string) tea.Cmd {
	msg := tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
	if k == "esc" {
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	_, cmd := s.Update(msg)
	return cmd
}

func TestKeys(t *testing.T) {
	s := NewPrologue()
	if s.Init() == nil {
		t.Fatal("the prologue does not play")
	}
	// A key while typing shows the page; the next goes on; the last leaves.
	if cmd := press(s, "x"); s.i != 0 || !s.p.Done() || cmd != nil {
		t.Fatalf("a key while typing: page %d, done %v", s.i, s.p.Done())
	}
	press(s, "x")
	if s.i != 1 || s.p.Done() {
		t.Fatalf("a key on a shown page: page %d, done %v", s.i, s.p.Done())
	}
	press(s, " ")
	if msg := emitted(press(s, "x")); msg != (Done{Scene: "story.prologue"}) {
		t.Errorf("the last key emits %#v", msg)
	}
	// esc leaves at once, mid-page.
	s = NewFirstMerge()
	s.Init()
	if msg := emitted(press(s, "esc")); msg != (Done{Scene: "story.first-merge"}) {
		t.Errorf("esc emits %#v", msg)
	}
}

func TestAutoLeave(t *testing.T) {
	s := NewAccepted("AC")
	s.Init()
	press(s, "x") // skip to the end: the dwell starts
	cur := leaveMsg{s, s.gen}
	if _, cmd := s.Update(leaveMsg{s, s.gen - 1}); cmd != nil {
		t.Error("a stale timer left the scene")
	}
	if _, cmd := s.Update(cur); emitted(cmd) != (Done{}) {
		t.Error("ACCEPTED does not leave on its own")
	}
	// A scene without a dwell waits for a key.
	m := NewFirstMerge()
	m.Init()
	press(m, "x")
	if m.afterward() != nil {
		t.Error("the merge scene leaves on its own")
	}
}

func TestReducedMotion(t *testing.T) {
	anim.SetMotion(anim.Motion{Reduced: true})
	defer anim.SetMotion(anim.Motion{})
	s := NewBossIntro("heisenbug", 0, false)
	if cmd := s.Init(); cmd != nil {
		t.Error("reduced motion still ticks")
	}
	if !s.p.Done() || !strings.Contains(ansi.Strip(s.View(80, 24)), "H E I S E N B U G") {
		t.Error("reduced motion does not show the card at once")
	}
	// The biome card still dwells: that is time to read, not motion.
	if NewBiome("dp", nil).Init() == nil {
		t.Error("reduced motion dropped the biome card's dwell")
	}
}

// TestCrashHasNoWinCue is retention GR3 on the Cheshire beat: a crash
// shows no win cue, at any frame.
func TestCrashHasNoWinCue(t *testing.T) {
	for _, f := range frames(NewFirstCrash("1 KiB", "x"), 80, 24) {
		p := ansi.Strip(f)
		for _, cue := range []string{"exit 0", "★", "ACCEPTED", "cleared", "+1"} {
			if strings.Contains(p, cue) {
				t.Errorf("the crash beat shows %q", cue)
			}
		}
	}
}

// hued reports whether s paints with a color: an SGR foreground or
// background parameter (30-49, 90-107).
func hued(s string) bool {
	for _, m := range sgr.FindAllStringSubmatch(s, -1) {
		for _, p := range strings.Split(m[1], ";") {
			if n, err := strconv.Atoi(p); err == nil && (n >= 30 && n <= 49 || n >= 90 && n <= 107) {
				return true
			}
		}
	}
	return false
}

var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// TestPaletteAndGlyphs: the scenes paint with the internal/ui tokens and
// glyphs the app picked. Under Mono (NO_COLOR) no frame carries a hue,
// and under the ASCII glyph set the boss card is drawn in ASCII. The
// default palette does paint the boss card and the ACCEPTED frame.
func TestPaletteAndGlyphs(t *testing.T) {
	if !hued(strings.Join(frames(NewBossIntro("heisenbug", 0, false), 80, 24), "\n")) ||
		!hued(strings.Join(frames(NewAccepted("AC"), 80, 24), "\n")) {
		t.Error("the default palette paints no hue on the boss card or ACCEPTED")
	}
	ui.SetPalette(ui.Mono)
	ui.SetGlyphs(ui.ASCII)
	defer func() { ui.SetPalette(ui.ANSI); ui.SetGlyphs(ui.Unicode) }()
	for _, b := range all() {
		for _, f := range frames(b.make(), 80, 24) {
			if hued(f) {
				t.Errorf("%s paints a hue under Mono: %q", b.name, f)
				break
			}
		}
	}
	card := ansi.Strip(strings.Join(frames(NewBossIntro("oom-killer", 0, false), 80, 24), "\n"))
	for _, g := range []string{"║", "═", "╔", "⚔", "·"} {
		if strings.Contains(card, g) {
			t.Errorf("the ASCII boss card draws %q", g)
		}
	}
	if !strings.Contains(card, "+== X BOSS ") {
		t.Errorf("the ASCII boss card's top edge:\n%s", card)
	}
}
