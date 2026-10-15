package app

import (
	"strings"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/audio"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/story"
)

// titleRestart starts the game again on the same store as cmd/katharsis
// runs it: the story and the title on, prompts as given.
func (h *harness) titleRestart(prompts bool) {
	h.t.Helper()
	d := h.m.d
	d.Story, d.Title, d.Prompts = true, true, prompts
	h.start(d)
}

// title is the title screen on top, nil when it is not up.
func (h *harness) title() *story.Scene {
	s := h.scene()
	if s == nil || s.ID() != "" {
		return nil
	}
	if !strings.Contains(ansi.Strip(h.m.View().Content), "press any key") {
		s.Seek(0, s.PageLen(0))
	}
	if !strings.Contains(ansi.Strip(h.m.View().Content), "press any key") {
		return nil
	}
	return s
}

// TestTitleAtLaunch: a launch that lands on GRUB opens on the title, over
// GRUB, with GRUB's music; a key goes on to GRUB, and nothing of the
// title is left on the stack or in the story queue.
func TestTitleAtLaunch(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			anim.SetMotion(anim.Motion{Reduced: reduced})
			defer anim.SetMotion(anim.Motion{})
			h := newHarness(t)
			if h.scene() != nil {
				t.Fatal("a scene without Deps.Title")
			}
			h.titleRestart(false)
			if h.title() == nil {
				t.Fatalf("reduced %v: the launch opens on %T, want the title", reduced, h.m.top())
			}
			if _, ok := h.m.stack[0].(*meta.Menu); !ok || len(h.m.stack) != 2 {
				t.Fatalf("under the title: %T, stack of %d", h.m.stack[0], len(h.m.stack))
			}
			if cue := h.m.audioCue(); cue.Pool != audio.PoolGrub {
				t.Errorf("the title's music is %v, want GRUB's", cue)
			}
			// It fits the window at the sizes the game promises (send
			// checks every line's width).
			for _, size := range [][2]int{{80, 24}, {200, 60}, {100, 30}} {
				h.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				if rows := strings.Count(h.m.View().Content, "\n") + 1; rows != size[1] {
					t.Errorf("at %v the title is %d rows", size, rows)
				}
			}
			h.send(key("x"))
			if _, ok := h.m.top().(*meta.Menu); !ok || len(h.m.stack) != 1 {
				t.Fatalf("after a key: %T on top, stack of %d; want GRUB alone", h.m.top(), len(h.m.stack))
			}
			if h.m.story.on != nil || len(h.m.story.queue) != 0 {
				t.Error("the title left itself in the story state")
			}
			// A boot from GRUB plays its own scenes, never the title.
			h.send(screens.StartBoot{Preset: "standard"})
			if h.title() != nil {
				t.Error("the title shows again at a boot start")
			}
		})
	}
}

// TestTitleOverThePrior: on the first launch the title comes first, then
// the rating question it covered.
func TestTitleOverThePrior(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.titleRestart(true)
		if h.title() == nil {
			t.Fatalf("the first launch opens on %T, want the title", h.m.top())
		}
		h.send(tea.KeyPressMsg{Code: tea.KeyEnter})
		if _, ok := h.m.top().(*meta.Prior); !ok {
			t.Fatalf("after the title: %T, want the rating question", h.m.top())
		}
	})
}

// TestNoTitleInABoot: a launch that resumes a boot shows no title,
// between doors or with one open.
func TestNoTitleInABoot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		for h.m.state.Phase != game.PhaseDoors {
			h.send(game.ChooseRoute{Index: 0})
		}
		h.titleRestart(false)
		for h.scene() != nil {
			if h.title() != nil {
				t.Fatal("the title shows over a resumed boot")
			}
			h.send(key("x"))
		}
		h.send(game.ChooseDoor{Index: 0})
		if h.m.open == nil {
			t.Fatal("no door open")
		}
		h.titleRestart(false)
		if h.m.open == nil || h.scene() != nil {
			t.Fatalf("resumed into the door: open %v, %T on top", h.m.open != nil, h.m.top())
		}
	})
}
