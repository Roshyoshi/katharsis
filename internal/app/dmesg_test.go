package app

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
)

// The strip in the running game: a failed submit's cost shows in the
// problem pane, the clear on the screen after it, and the onboarding lines
// are marked seen and saved, so a restart does not repeat them.
func TestDmesgStrip(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.toDoors()
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "doors: aimed at your rating") {
			t.Fatalf("first doors screen without the doors line:\n%s", v)
		}
		if !game.HasSeen(&h.m.meta, "tip.doors") {
			t.Fatal("the doors line was not marked seen")
		}
		h.enterDoor()
		lives := h.m.state.Lives
		h.verdict = "WA"
		h.send(submitMsg{})
		if h.m.state.Lives != lives-1 {
			t.Fatalf("lives %d after a WA, want %d", h.m.state.Lives, lives-1)
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "judge: WA 3/10") || !strings.Contains(v, "-1 ♥") {
			t.Fatalf("the pane does not say what the WA cost:\n%s", v)
		}
		h.verdict = "AC"
		h.clock.t = h.clock.t.Add(time.Minute)
		h.send(submitMsg{})
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "judge: ✓ AC") {
			t.Fatalf("the screen after a clear does not show it:\n%s", v)
		}

		h.restart()
		for _, id := range []string{"tip.doors", "tip.door"} {
			if !game.HasSeen(&h.m.meta, id) {
				t.Errorf("%s not saved", id)
			}
		}
		if evs := h.m.dmesg.Events(); len(evs) != 0 {
			t.Errorf("a restart replays the strip: %+v", evs)
		}
	})
}

// A quack refused for want of a duck gets the Cat's line on the strip, from
// the pane's key and from the editor alike.
func TestDmesgDuckRefused(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.m.state.Ducks = 0
		h.send(key("?"))
		evs := h.m.dmesg.Events()
		if len(evs) == 0 || evs[len(evs)-1].Hook != "duck.empty" {
			t.Fatalf("no duck line after a refused quack: %+v", evs)
		}
	})
}

// The merge window's daemon.intro is marked seen once it is drawn, and a
// second window does not draw it.
func TestDaemonIntroSeen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(tea.WindowSizeMsg{Width: 140, Height: 40})
		h.send(screens.StartBoot{Preset: "standard"})
		for i := 0; h.m.state.Phase != game.PhaseMerge; i++ {
			if i > 40 {
				t.Fatalf("no merge window, phase %q", h.m.state.Phase)
			}
			h.enterDoor()
			h.clock.t = h.clock.t.Add(5 * time.Minute)
			h.send(submitMsg{})
		}
		if len(h.m.state.DaemonOffers) == 0 {
			t.Skip("this seed offers no daemons")
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "A knight falls off his horse") {
			t.Fatalf("no intro over the first daemon offers:\n%s", v)
		}
		h.send(tickMsg(h.clock.t))
		if !game.HasSeen(&h.m.meta, "daemon.intro") {
			t.Fatal("daemon.intro drawn but not marked seen")
		}
	})
}
