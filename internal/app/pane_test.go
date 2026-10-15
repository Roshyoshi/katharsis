package app

import (
	"os"
	"strings"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/screens"
)

func paneHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: t.TempDir(), Embedded: true}}
	})
	h.send(screens.StartBoot{Preset: "standard"})
	h.toDoors()
	return h
}

func view(h *harness) string { return ansi.Strip(h.m.View().Content) }

// The pane opens on the statement under the pinned judgement; enter swaps
// it for the tests and back, and h lists every key until the next one
// (J8, J17).
func TestPaneStatementFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := paneHarness(t)
		h.send(game.ChooseDoor{})
		h.send(tea.WindowSizeMsg{Width: 44, Height: 32})
		v := view(h)
		hp, body := strings.Index(v, "HP "), strings.Index(v, "Bessie names this problem")
		if hp < 0 || body < hp {
			t.Fatalf("no statement under the pinned HP bar:\n%s", v)
		}
		if lines := strings.Split(v, "\n"); len(lines) != 32 || !strings.Contains(lines[31], "enter tests") ||
			!strings.Contains(lines[31], "h help") {
			t.Fatalf("pane is %d lines, keys row %q", len(lines), lines[len(lines)-1])
		}
		h.send(tea.KeyPressMsg{Code: tea.KeyEnter})
		if v := view(h); strings.Contains(v, "Bessie names") || !strings.Contains(v, "enter statement") {
			t.Fatalf("enter did not swap to the tests:\n%s", v)
		}
		h.send(tea.KeyPressMsg{Code: tea.KeyEnter})
		h.send(key("h"))
		if v := view(h); !strings.Contains(v, "any key closes this") || !strings.Contains(v, "retreat for a life") {
			t.Fatalf("h shows no keys:\n%s", v)
		}
		h.send(key("s")) // closes the help, and submits nothing
		if v := view(h); strings.Contains(v, "any key closes") || h.m.judging {
			t.Fatalf("the key after h did more than close it (judging %v):\n%s", h.m.judging, v)
		}
	})
}

// A mystery door's statement in the pane is redacted, as in the reader.
func TestPaneMysteryStatement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := paneHarness(t)
		h.m.state.Doors[0].Mystery = true
		p, _ := h.m.d.Catalog.Get(h.m.state.Doors[0].Problem)
		h.send(game.ChooseDoor{})
		h.send(tea.WindowSizeMsg{Width: 60, Height: 32})
		v := view(h)
		if !strings.Contains(v, "Bessie names this problem ???") || strings.Contains(v, p.Title) {
			t.Fatalf("mystery pane gives away %q:\n%s", p.Title, v)
		}
	})
}

// A source still the template is pristine, so its samples wait for a save
// (J13); one edit, or a file that is gone, is judged as before.
func TestPristineTemplate(t *testing.T) {
	r := &TmuxRunner{}
	d := Door{Door: game.Door{Problem: "usaco:1"}}
	src := t.TempDir() + "/sol.cpp"
	if r.pristine(d, src) {
		t.Error("a missing source counted as the template")
	}
	if err := os.WriteFile(src, []byte(stdioTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	if !r.pristine(d, src) {
		t.Error("the template is not pristine")
	}
	if err := os.WriteFile(src, []byte(stdioTemplate+"// x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r.pristine(d, src) {
		t.Error("an edited source counted as the template")
	}
}
