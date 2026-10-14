package app

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/screens"
	"katharsis/internal/store"
)

// fakeCatalog serves the simulator's synthetic pool; Fetch returns no
// files, which the headless loop never reads.
type fakeCatalog struct {
	all  []catalog.Problem
	byID map[catalog.ID]catalog.Problem
}

func newFakeCatalog() *fakeCatalog {
	c := &fakeCatalog{all: sim.SyntheticPool(600, 7), byID: map[catalog.ID]catalog.Problem{}}
	for _, p := range c.all {
		c.byID[p.ID] = p
	}
	return c
}

func (c *fakeCatalog) All() []catalog.Problem { return c.all }
func (c *fakeCatalog) Get(id catalog.ID) (catalog.Problem, bool) {
	p, ok := c.byID[id]
	return p, ok
}
func (c *fakeCatalog) Fetch(context.Context, catalog.ID) (catalog.Assets, error) {
	return catalog.Assets{}, nil
}

type fakeRunner struct{ opened, closed int }

func (r *fakeRunner) Open(context.Context, Door) error { r.opened++; return nil }
func (r *fakeRunner) Close() error                     { r.closed++; return nil }

// clock advances only when the test says so, so timers never expire by
// accident.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

type harness struct {
	t       *testing.T
	m       *Model
	st      *store.Store
	runner  *fakeRunner
	clock   *clock
	verdict string // what the fake judge returns
	// acFirst makes the next n submits AC whatever verdict says, so a
	// test can earn bytes before it starts failing.
	acFirst int
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	h := &harness{t: t, st: st, runner: &fakeRunner{}, clock: &clock{time.Date(2026, 10, 14, 18, 0, 0, 0, time.UTC)}, verdict: "AC"}
	m, err := New(Deps{
		Store:   st,
		Catalog: newFakeCatalog(),
		Runner:  h.runner,
		Judge: func(context.Context, Door) (string, int, int, error) {
			if h.acFirst > 0 {
				h.acFirst--
				return "AC", 10, 10, nil
			}
			if h.verdict == "AC" {
				return "AC", 10, 10, nil
			}
			return h.verdict, 3, 10, nil
		},
		Now:  h.clock.now,
		Seed: func() uint64 { return 42 },
	})
	if err != nil {
		t.Fatal(err)
	}
	h.m = m
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m.Init())
	return h
}

// run executes a command and feeds its messages back, recursively. Commands
// that block (the 1 s tick, reveal animations) are abandoned after a short
// wait, the way a test clock would skip them.
func (h *harness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(30 * time.Millisecond):
		return
	}
	switch msg := msg.(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			h.run(c)
		}
	default:
		h.send(msg)
	}
}

func (h *harness) send(msg tea.Msg) {
	h.t.Helper()
	_, cmd := h.m.Update(msg)
	h.run(cmd)
	// Every state must render at the current size without breaking width.
	for _, l := range strings.Split(h.m.View().Content, "\n") {
		if w := ansi.StringWidth(l); w > h.m.width {
			h.t.Fatalf("line %d wide at width %d in %T: %q", w, h.m.width, h.m.top(), ansi.Strip(l))
		}
	}
	if h.m.flash != "" {
		h.t.Logf("flash: %s", h.m.flash)
	}
}

// play drives the boot with the simplest choices until it ends, returning
// the final phase.
func (h *harness) play() game.Phase {
	h.t.Helper()
	for i := 0; i < 400; i++ {
		s := h.m.state
		if s == nil {
			h.t.Fatal("no boot")
		}
		switch {
		case s.Phase == game.PhaseCleared || s.Phase == game.PhaseCrashed:
			return s.Phase
		case s.PendingUpgrade:
			h.send(game.PickUpgrade{Upgrade: s.UpgradeOffers[0]})
		case s.Phase == game.PhaseRoute:
			h.send(game.ChooseRoute{Index: 0})
		case s.Phase == game.PhaseDoors:
			h.send(game.ChooseDoor{Index: 0})
		case s.Phase == game.PhaseProblem:
			if h.m.open == nil {
				h.t.Fatalf("door %s open in the engine but not in the app", s.Current.Door.Problem)
			}
			h.clock.t = h.clock.t.Add(5 * time.Minute)
			h.send(submitMsg{})
		case s.Phase == game.PhaseMerge:
			if s.Uncommitted > 0 {
				h.send(game.Commit{})
			} else {
				h.send(game.LeaveMerge{})
			}
		default:
			h.t.Fatalf("unexpected phase %q", s.Phase)
		}
	}
	h.t.Fatal("boot did not end in 400 steps")
	return ""
}

func TestFullClear(t *testing.T) {
	h := newHarness(t)
	if _, ok := h.m.top().(interface{ Editing() bool }); !ok {
		t.Fatalf("start screen is %T, want the GRUB menu", h.m.top())
	}
	h.send(screens.StartBoot{Prestige: 0, Preset: "standard"})
	if got := h.play(); got != game.PhaseCleared {
		t.Fatalf("boot ended %q, want cleared", got)
	}
	if h.m.meta.Prestige != 1 {
		t.Errorf("prestige %d after a prestige-0 clear, want 1", h.m.meta.Prestige)
	}
	if h.runner.opened == 0 || h.runner.opened != h.runner.closed {
		t.Errorf("runner opened %d, closed %d", h.runner.opened, h.runner.closed)
	}
	boots, err := h.st.Boots(10)
	if err != nil || len(boots) != 1 || boots[0].Outcome != store.BootCleared {
		t.Fatalf("stored boots %+v, %v", boots, err)
	}
	// History screens open from the cleared screen's intents.
	h.send(screens.OpenLog{})
	h.send(screens.Back{})
	h.send(screens.OpenTop{})
}

func TestCrashThenUpsolveRecoversCoreDump(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.verdict, h.acFirst = "WA", 1 // earn bytes on door 1, then fail
	if got := h.play(); got != game.PhaseCrashed {
		t.Fatalf("boot ended %q, want crashed", got)
	}
	dump := h.m.meta.CoreDump
	if dump == nil {
		t.Fatal("crash left no core dump")
	}
	bank := h.m.meta.Bank

	h.verdict = "AC"
	h.send(screens.Upsolve{Problem: string(dump.Problem)})
	if h.m.open == nil || !h.m.open.Upsolve {
		t.Fatal("upsolve did not open the problem")
	}
	h.send(submitMsg{})
	if h.m.meta.CoreDump != nil {
		t.Error("core dump still there after an accepted upsolve")
	}
	if h.m.meta.Bank != bank+dump.Bytes {
		t.Errorf("bank %d, want %d + %d recovered", h.m.meta.Bank, bank, dump.Bytes)
	}
	if h.m.open != nil {
		t.Error("upsolve door still open")
	}
}

func TestTimerExpiryCrashes(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	for h.m.state.Phase != game.PhaseProblem {
		switch {
		case h.m.state.PendingUpgrade:
			h.send(game.PickUpgrade{Upgrade: h.m.state.UpgradeOffers[0]})
		case h.m.state.Phase == game.PhaseRoute:
			h.send(game.ChooseRoute{})
		default:
			h.send(game.ChooseDoor{})
		}
	}
	h.clock.t = h.m.state.Current.Deadline.Add(time.Second)
	h.send(tickMsg(h.clock.t))
	if h.m.state.Phase != game.PhaseCrashed || h.m.state.Crash.Cause != game.TimedOut {
		t.Fatalf("phase %q crash %+v, want a timeout crash", h.m.state.Phase, h.m.state.Crash)
	}
}

func TestResumeMidDoor(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	for h.m.state.Phase != game.PhaseProblem {
		switch {
		case h.m.state.Phase == game.PhaseRoute:
			h.send(game.ChooseRoute{})
		default:
			h.send(game.ChooseDoor{})
		}
	}
	want := h.m.state.Current.Door

	// A second app on the same store resumes the boot and reopens the door.
	m2, err := New(Deps{Store: h.st, Catalog: h.m.d.Catalog, Runner: h.runner, Judge: h.m.d.Judge, Now: h.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	h.m = m2
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20})
	h.run(m2.Init())
	if h.m.open == nil || !reflect.DeepEqual(h.m.open.Door, want) {
		t.Fatalf("resumed door %+v, want %+v", h.m.open, want)
	}
}
