package story

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/anim/animtest"
	"katharsis/internal/flavor"
	"katharsis/internal/ui"
)

// titleAt is the title at w×h, t into its page, as drawn.
func titleAt(w, h int, t time.Duration) string {
	s := NewTitle()
	s.View(w, h)
	s.Seek(0, t)
	return s.View(w, h)
}

// TestTitleGoldens holds the title's frames at a time: the logo at both
// its sizes over the fire as it climbs and at its height, in both glyph
// sets.
func TestTitleGoldens(t *testing.T) {
	for _, set := range []struct {
		name string
		g    ui.Glyphs
	}{{"", ui.Unicode}, {"_ascii", ui.ASCII}} {
		ui.SetGlyphs(set.g)
		for _, size := range [][2]int{{80, 24}, {140, 40}} {
			var b strings.Builder
			for _, at := range []time.Duration{time.Second, titleBurn} {
				fmt.Fprintf(&b, "=== %v\n%s", at, animtest.Grids(strings.Split(titleAt(size[0], size[1], at), "\n")))
			}
			animtest.Golden(t, fmt.Sprintf("title_%dx%d%s", size[0], size[1], set.name), b.String())
		}
	}
	ui.SetGlyphs(ui.Unicode)
}

// TestTitleFits: every frame is the whole screen, no wider, from a small
// pane to 200x60, in both glyph sets. The logo is drawn large where it
// fits, small at 80x24, and as the plain banner in a narrow pane.
func TestTitleFits(t *testing.T) {
	defer ui.SetGlyphs(ui.Unicode)
	for _, g := range []ui.Glyphs{ui.Unicode, ui.ASCII} {
		ui.SetGlyphs(g)
		for _, size := range [][2]int{{200, 60}, {140, 40}, {130, 30}, {100, 30}, {80, 24}, {66, 16}, {60, 20}, {40, 12}, {20, 6}, {1, 1}} {
			for _, at := range []time.Duration{0, time.Second, titleBurn, titleBurn + titleOut} {
				v := titleAt(size[0], size[1], at)
				rows := strings.Split(v, "\n")
				if len(rows) != size[1] {
					t.Errorf("%v at %v: %d rows", size, at, len(rows))
				}
				for i, r := range rows {
					if w := ansi.StringWidth(r); w != size[0] {
						t.Errorf("%v at %v, row %d: %d cells: %q", size, at, i, w, ansi.Strip(r))
						break
					}
				}
			}
		}
	}
	ui.SetGlyphs(ui.Unicode)
	bannerRow := art("grub")[0]
	for _, c := range []struct {
		w, h  int
		art   int // the widest run of logo glyphs: 0 for the banner
		label string
	}{{200, 60, 126, "large"}, {140, 40, 126, "large"}, {80, 24, 64, "small"}, {66, 16, 64, "small"}, {60, 20, 0, "banner"}, {130, 17, 64, "small"}} {
		v := ansi.Strip(titleAt(c.w, c.h, titleBurn))
		if banner := strings.Contains(v, bannerRow); banner != (c.art == 0) {
			t.Errorf("%dx%d: banner %v, want the %s logo", c.w, c.h, banner, c.label)
		}
		if c.art > 0 {
			if got := len(titleLogo(c.w, c.h)[0]); got != c.art {
				t.Errorf("%dx%d: the logo is %d pixels across, want %d (%s)", c.w, c.h, got, c.art, c.label)
			}
		}
	}
}

// TestTitleSaysItsLines: at its height the title shows the tagline and
// the key line, and the logo spells the word: each letter of the font is
// in the large ASCII logo, one font row a cell row.
func TestTitleSaysItsLines(t *testing.T) {
	v := ansi.Strip(titleAt(80, 24, titleBurn))
	for _, l := range []string{flavor.Story("title.tagline", 0), "press any key"} {
		if !strings.Contains(v, l) {
			t.Errorf("the title never says %q:\n%s", l, v)
		}
	}
	if strings.Contains(ansi.Strip(titleAt(80, 24, 0)), "press any key") {
		t.Error("press any key shows before the fire is lit")
	}
	if len(flavor.Story("title.tagline", 0)) == 0 {
		t.Error("no tagline in the story table")
	}
	for _, r := range logoWord {
		if len(font[r]) != 8 {
			t.Errorf("the font's %c is %d rows, want 8", r, len(font[r]))
		}
	}
}

// TestTitleReducedMotion: one still frame of the fire at its height,
// with every line shown, held as long as the fire would burn; no frame
// ticks.
func TestTitleReducedMotion(t *testing.T) {
	anim.SetMotion(anim.Motion{Reduced: true})
	defer anim.SetMotion(anim.Motion{})
	synctest.Test(t, func(t *testing.T) {
		s := NewTitle()
		s.View(80, 24)
		start := time.Now()
		ticks, done := pump(t, s, s.Init(), nil)
		if ticks != 0 {
			t.Errorf("reduced motion ticked %d times", ticks)
		}
		if got := time.Since(start); got != titleBurn {
			t.Errorf("reduced motion left after %v, want %v", got, titleBurn)
		}
		if done != (Done{}) {
			t.Errorf("reduced motion left with %#v", done)
		}
	})
	s := NewTitle()
	s.View(80, 24)
	s.Init()
	anim.SetMotion(anim.Motion{})
	still := strings.Split(s.View(80, 24), "\n")
	if want := strings.Split(titleAt(80, 24, titleBurn), "\n"); !slices.Equal(still, want) {
		t.Errorf("the still frame is not the fire at its height:\n%s", ansi.Strip(strings.Join(still, "\n")))
	}
}

// TestTitlePlaysAndLeaves: under full motion the title ticks at the
// frame-rate cap while the fire burns and dies, leaves on its own after
// it, and ticks no more once it has left, even for the tick in flight.
func TestTitlePlaysAndLeaves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := NewTitle()
		s.View(80, 24)
		start := time.Now()
		var gaps []time.Duration
		last := start
		ticks, done := pump(t, s, s.Init(), func() {
			gaps = append(gaps, time.Since(last))
			last = time.Now()
		})
		took := time.Since(start)
		if want := titleBurn + titleOut + titleTail; took < want || took > want+anim.FireStep {
			t.Errorf("the title left after %v, want %v", took, want)
		}
		if want := int((titleBurn + titleOut) / anim.FireStep); ticks < want-1 || ticks > want+1 {
			t.Errorf("%d frame ticks over %v, want about %d", ticks, titleBurn+titleOut, want)
		}
		for i, g := range gaps {
			if g < anim.FireStep-time.Millisecond {
				t.Errorf("tick %d came %v after the last, faster than the cap", i, g)
				break
			}
		}
		if done != (Done{}) {
			t.Errorf("the title left with %#v", done)
		}
	})
}

// TestTitleKeys: any key leaves at once, mid-fire; esc leaves skipped. No
// tick lands on the title once it has left.
func TestTitleKeys(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for _, c := range []struct {
			key  tea.KeyPressMsg
			want Done
		}{{tea.KeyPressMsg{Code: 'x', Text: "x"}, Done{}}, {tea.KeyPressMsg{Code: tea.KeyEnter}, Done{}},
			{tea.KeyPressMsg{Code: tea.KeyEscape}, Done{Skipped: true}}} {
			s := NewTitle()
			s.View(80, 24)
			// The first frame tick, in flight when the key comes.
			tick := msgsOf(s.Init())
			if len(tick) != 1 {
				t.Fatalf("the title's first command sent %d messages, want its frame tick", len(tick))
			}
			if _, cmd := s.Update(c.key); emitted(cmd) != c.want {
				t.Errorf("%s: emitted %#v, want %#v", c.key, emitted(cmd), c.want)
			}
			if _, cmd := s.Update(tick[0]); cmd != nil {
				t.Errorf("%s: the tick in flight ticks again after the title left", c.key)
			}
			if _, cmd := s.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd != nil {
				t.Errorf("%s: a key after leaving does something", c.key)
			}
		}
	})
}

// msgsOf runs cmd, in a bubble, and every command a batch holds, and is
// the messages they send.
func msgsOf(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch m := cmd().(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range m {
			out = append(out, msgsOf(c)...)
		}
		return out
	default:
		return []tea.Msg{m}
	}
}

// pump plays s, in a bubble, as a tea.Program would: each command runs
// on its own and its message goes to s, until s emits Done. It counts the
// frame ticks (onTick runs at each) and returns the Done. After the Done,
// whatever is still in flight is delivered too, and must send nothing
// more.
func pump(t *testing.T, s *Scene, cmd tea.Cmd, onTick func()) (ticks int, done Done) {
	t.Helper()
	msgs := make(chan tea.Msg, 64)
	inFlight := 0
	launch := func(c tea.Cmd) {
		if c != nil {
			inFlight++
			go func() { msgs <- c() }()
		}
	}
	launch(cmd)
	left := false
	for inFlight > 0 {
		msg := <-msgs
		inFlight--
		switch m := msg.(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range m {
				launch(c)
			}
		case Done:
			done, left = m, true
		default:
			if _, ok := m.(leaveMsg); !ok && !left {
				ticks++
				if onTick != nil {
					onTick()
				}
			}
			_, c := s.Update(m)
			if left && c != nil {
				t.Errorf("after the title left, %T sent a command", m)
			}
			launch(c)
		}
		if inFlight > 10 {
			t.Fatalf("%d commands in flight", inFlight)
		}
	}
	if !left {
		t.Fatal("the title never left")
	}
	return ticks, done
}
