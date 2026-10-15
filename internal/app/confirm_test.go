package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/problem"
	"katharsis/internal/screens"
	"katharsis/internal/session"
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
	// A submit that fails a sample again is free: the question must not
	// quote a hidden test's price for it.
	if !strings.Contains(rep.Text, "free if a sample fails again") || strings.Contains(rep.Text, "burns") {
		t.Errorf(":Submit question %q, want the free sample fail and no price", rep.Text)
	}
	if h.m.judging || h.m.state.Cycles != lives || h.m.state.Current.Submits != 0 {
		t.Fatalf("a refused :Submit judged or charged: judging %v lives %d submits %d",
			h.m.judging, h.m.state.Cycles, h.m.state.Current.Submits)
	}
	req, reply = session.NewRequest(session.Command{Cmd: session.CmdSubmit, Args: []string{session.ArgForce}})
	_, cmd := h.m.Update(req)
	h.pump(cmd, func() bool { return !h.m.judging })
	cost := fmt.Sprintf(" · burned 1 cycle, %d left", lives-1)
	if rep, _ := answer(reply); !strings.HasPrefix(rep.Text, "WA 0/1") || !strings.HasSuffix(rep.Text, cost) {
		t.Errorf(":Submit! answered %+v, want its verdict and %q", rep, cost)
	}
	if h.m.state.Cycles != lives-1 {
		t.Errorf("lives %d after :Submit!'s WA, want %d", h.m.state.Cycles, lives-1)
	}
}

// A submit that fails on one of the door's samples is free (meta.md §(b)):
// the samples had said so. The full test that failed is the sample by its
// input's tokens: a statement's sample can differ from the official test
// in blank lines and line ends. vim's reply says why nothing burned.
func TestSampleFailIsFree(t *testing.T) {
	h := samplesFailed(t)
	sample := filepath.Join(t.TempDir(), "1.in")
	if err := os.WriteFile(sample, []byte("\n2 \r\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.m.open.Assets.Samples = []problem.Test{{Name: "1", In: sample}}
	lives := h.m.state.Cycles
	req, reply := session.NewRequest(session.Command{Cmd: session.CmdSubmit, Args: []string{session.ArgForce}})
	_, cmd := h.m.Update(req)
	h.pump(cmd, func() bool { return !h.m.judging })
	if rep, _ := answer(reply); !strings.HasPrefix(rep.Text, "WA 0/1") || !strings.HasSuffix(rep.Text, " · "+sampleFree) {
		t.Errorf(":Submit! answered %+v, want its verdict and %q", rep, sampleFree)
	}
	if c := h.m.state.Current; h.m.state.Cycles != lives || c == nil || c.Submits != 1 {
		t.Errorf("cycles %d (want %d), attempt %+v: a sample's fail must be judged and free", h.m.state.Cycles, lives, c)
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
		if last := h.lastLine(); !strings.Contains(last, "retreat: burns 1 cycle") || !strings.Contains(last, "R again") {
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
		if got, want := h.m.charged(lives), "burned 1 cycle, 0 left · kernel panic"; h.m.state.Phase != game.PhaseCrashed || got != want {
			t.Fatalf("phase %s, charged %q; want a crash and %q", h.m.state.Phase, got, want)
		}
		h.send(screens.Upsolve{Problem: h.m.state.Crash.Door.Problem})
		if lives := h.m.doorLives(); lives.ok || h.m.charged(lives) != "" {
			t.Errorf("an upsolve: doorLives %+v, charged %q; want none and nothing", lives, h.m.charged(lives))
		}
	})
}
