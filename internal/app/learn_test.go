package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/learn"
	"katharsis/internal/screens"
)

// srcRunner writes each door's source the way the tmux runner does (libs
// pasted into a fresh template) and reports its path, so lib harvesting and
// linking run through the real app paths.
type srcRunner struct {
	*fakeRunner
	dir   string
	src   string
	libs  [][]game.Lib // Door.Libs per Open
	write string       // what the "player" saves into each new source
}

func (r *srcRunner) Open(ctx context.Context, d Door) error {
	if err := r.fakeRunner.Open(ctx, d); err != nil {
		return err
	}
	r.libs = append(r.libs, d.Libs)
	r.src = filepath.Join(r.dir, strings.ReplaceAll(string(d.Door.Problem), ":", "")+".cpp")
	return os.WriteFile(r.src, []byte(r.write), 0o644)
}

func (r *srcRunner) Source() string { return r.src }

func TestLibsHarvestedOnACAndLinkedNextBoot(t *testing.T) {
	var r *srcRunner
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		r = &srcRunner{fakeRunner: f, dir: t.TempDir(),
			write: "// lib: dsu\nstruct DSU {};\n// end lib\nint main() {}\n"}
		return r
	})
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	h.send(submitMsg{}) // the fake judge accepts
	if !slices.ContainsFunc(h.m.meta.Libs, func(l game.Lib) bool { return l.Name == "dsu" && l.Source == "struct DSU {};\n" }) {
		t.Fatalf("libs after an AC = %+v, want dsu", h.m.meta.Libs)
	}

	// A new boot offers the link screen; link dsu and open a door.
	h.send(screens.StartBoot{Preset: "standard"})
	if _, ok := h.m.top().(*linkScreen); !ok {
		t.Fatalf("top screen %T, want the link screen", h.m.top())
	}
	h.send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	h.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !slices.Equal(h.m.state.Linked, []string{"dsu"}) {
		t.Fatalf("linked %v, want [dsu]", h.m.state.Linked)
	}
	h.enterDoor()
	got := r.libs[len(r.libs)-1]
	if len(got) != 1 || got[0].Name != "dsu" {
		t.Fatalf("door opened with libs %+v, want dsu", got)
	}
}

func TestNoLinkScreenWithoutLibs(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	if _, ok := h.m.top().(*linkScreen); ok {
		t.Fatal("link screen shown with an empty lib bank")
	}
}

// loadGuide gives the app the learn package's fixture guide as its RTFM
// curriculum.
func loadGuide(h *harness) *learn.Curriculum {
	h.t.Helper()
	cur, err := learn.Load(filepath.Join("..", "learn", "testdata", "guide"))
	if err != nil {
		h.t.Fatal(err)
	}
	h.m.cur = cur
	return cur
}

func TestTrialGrantsItsCapability(t *testing.T) {
	h := newHarness(t)
	cur := loadGuide(h)
	mod, ok := cur.Module("prefix-sums")
	if !ok {
		t.Fatal("fixture guide has no prefix-sums module")
	}
	want := learn.TrialFor(mod, h.m.judgeable).Capability
	if want == "" {
		t.Fatal("prefix-sums grants no capability")
	}

	h.send(screens.StartTrial{Module: "prefix-sums"})
	if h.m.state == nil || h.m.state.Trial == nil {
		t.Fatalf("no trial running: %+v", h.m.state)
	}
	if got := h.play(); got != game.PhaseCleared {
		t.Fatalf("trial ended %q, want cleared", got)
	}
	if !slices.Contains(h.m.meta.Capabilities, want) {
		t.Fatalf("capabilities %v after passing, want %s", h.m.meta.Capabilities, want)
	}
}

func TestModuleWithoutTopicHasNoTrial(t *testing.T) {
	h := newHarness(t)
	loadGuide(h)
	h.send(screens.StartTrial{Module: "fast-io"})
	if h.m.state != nil && h.m.state.Trial != nil {
		t.Fatal("a topic-less module started a trial")
	}
}

// A trial's timer is soft: its door resumed past the deadline goes on in
// overtime, so it reopens; it used to wait, editorless, for an e.
func TestResumedTrialInOvertimeReopens(t *testing.T) {
	h := newHarness(t)
	loadGuide(h)
	h.send(screens.StartTrial{Module: "prefix-sums"})
	h.clock.t = h.enterDoor().Deadline.Add(time.Minute)
	h.restart()
	if h.m.state.Phase != game.PhaseProblem || h.m.open == nil {
		t.Fatalf("phase %q open %v after resuming a trial door in overtime, want it open", h.m.state.Phase, h.m.open != nil)
	}
}
