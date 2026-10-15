package app

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/screens"
	"katharsis/internal/session"
	"katharsis/internal/ui"
)

// samplesFailed is a boot door whose samples were judged and fail: the
// HUD judges waSrc against the one sample.
func samplesFailed(t *testing.T) *harness {
	t.Helper()
	if _, err := exec.LookPath(judge.Compiler); err != nil {
		t.Skipf("%s not on PATH", judge.Compiler)
	}
	h := hudHarness(t, submitDir(t, waSrc), judge.Profile{})
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	_, cmd := h.m.Update(hud.SavedMsg{}) // a save: the samples judge again
	h.pump(cmd, func() bool { return h.m.hud().Samples().Done })
	if s := h.m.hud().Samples(); !s.Compiled || s.Passed != 0 || s.Total != 1 {
		t.Fatalf("samples %+v, want 0/1 judged", s)
	}
	return h
}

// :Submit while the samples fail asks for :Submit! and spends nothing;
// :Submit! is judged and charged (J3), and its reply in vim names the
// cost.
func TestSubmitBangOverFailingSamples(t *testing.T) {
	h := samplesFailed(t)
	lives := h.m.state.Cycles
	req, reply := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	h.send(req)
	rep, ok := answer(reply)
	if !ok || rep.OK || !strings.Contains(rep.Text, "samples 0/1 pass") || !strings.Contains(rep.Text, ":Submit!") {
		t.Fatalf(":Submit over failing samples answered %v %+v, want the :Submit! question", ok, rep)
	}
	if h.m.judging || h.m.state.Cycles != lives || h.m.state.Current.Submits != 0 {
		t.Fatalf("a refused :Submit judged or charged: judging %v lives %d submits %d",
			h.m.judging, h.m.state.Cycles, h.m.state.Current.Submits)
	}
	req, reply = session.NewRequest(session.Command{Cmd: session.CmdSubmit, Args: []string{session.ArgForce}})
	_, cmd := h.m.Update(req)
	h.pump(cmd, func() bool { return !h.m.judging })
	cost := fmt.Sprintf(" · -1 %s, %d left", ui.G.Life, lives-1)
	if rep, _ := answer(reply); !strings.HasPrefix(rep.Text, "WA 0/1") || !strings.HasSuffix(rep.Text, cost) {
		t.Errorf(":Submit! answered %+v, want its verdict and %q", rep, cost)
	}
	if h.m.state.Cycles != lives-1 {
		t.Errorf("lives %d after :Submit!'s WA, want %d", h.m.state.Cycles, lives-1)
	}
}

// The pane's s asks on the keys' row while the samples fail; a second s
// submits, any other key lets it go.
func TestPaneSubmitAsks(t *testing.T) {
	h := samplesFailed(t)
	h.send(key("s"))
	if last := h.lastLine(); !strings.Contains(last, "samples 0/1 pass") || !strings.Contains(last, "s again submits") {
		t.Fatalf("keys row %q after s over failing samples, want the question", last)
	}
	if h.m.judging {
		t.Fatal("the first s submitted")
	}
	h.send(key("x"))
	if last := h.lastLine(); strings.Contains(last, "again") {
		t.Fatalf("keys row %q after another key, want the keys back", last)
	}
	h.send(key("s"))
	_, cmd := h.m.Update(key("s")) // emits the submit
	_, cmd = h.m.Update(cmd())
	if !h.m.judging {
		t.Fatal("s s did not submit")
	}
	h.pump(cmd, func() bool { return !h.m.judging })
}

// R asks first and says what the swap costs; R R retreats (J4).
func TestRetreatAsks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarnessWith(t, func(f *fakeRunner) Runner {
			return &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: t.TempDir(), Embedded: true}}
		})
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(tea.WindowSizeMsg{Width: 60, Height: 30})
		lives, door := h.m.state.Cycles, h.m.state.Current.Door.Problem
		h.send(key("R"))
		if last := h.lastLine(); !strings.Contains(last, "retreat: -1 ♥") || !strings.Contains(last, "R again") {
			t.Fatalf("keys row %q after R, want what the retreat costs", last)
		}
		if h.m.state.Current == nil || h.m.state.Current.Door.Problem != door {
			t.Fatal("one R retreated")
		}
		h.send(key("R"))
		if h.m.state.Cycles != lives-1 || h.m.state.Current != nil && h.m.state.Current.Door.Problem == door {
			t.Fatalf("R R: lives %d (want %d), current %+v", h.m.state.Cycles, lives-1, h.m.state.Current)
		}
	})
}

// A failed submit that ends the boot says so in vim's reply rather than
// counting lives left; an upsolve's costs nothing.
func TestChargedNamesTheCrash(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.m.state.Cycles = 1
		lives := h.m.doorLives()
		h.verdict = "WA"
		h.send(submitMsg{})
		if got, want := h.m.charged(lives), "-1 "+ui.G.Life+", kernel panic"; h.m.state.Phase != game.PhaseCrashed || got != want {
			t.Fatalf("phase %s, charged %q; want a crash and %q", h.m.state.Phase, got, want)
		}
		h.send(screens.Upsolve{Problem: h.m.state.Crash.Door.Problem})
		if lives := h.m.doorLives(); lives != -1 || h.m.charged(lives) != "" {
			t.Errorf("an upsolve: doorLives %d, charged %q; want -1 and nothing", lives, h.m.charged(lives))
		}
	})
}
