package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
)

// A door chosen before its problem is on disk says so on the doors
// screen's last row, with its timer not running and the ready doors named;
// esc gives the choice back, and a late download opens nothing. Choosing
// a ready door meanwhile opens it at once (J9).
func TestDoorWaitCancelsAndSwitches(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.toDoors()
	if v := view(h); !strings.Contains(v, "ready") {
		t.Fatalf("prefetched doors do not say they are ready:\n%s", v)
	}
	slow, quick := h.m.state.Doors[0].Problem, h.m.state.Doors[1].Problem
	c := newGateCatalog(h.m.d.Catalog.(*fakeCatalog))
	c.gates[slow] = make(chan struct{})
	t.Cleanup(func() { close(c.gates[slow]) })
	c.gates[quick] = make(chan struct{})
	close(c.gates[quick])
	h.m.d.Catalog = c

	h.send(game.ChooseDoor{Index: 0})
	if last := lastLine(h); !strings.Contains(last, "fetching door 1") || !strings.Contains(last, "timer not running") ||
		!strings.Contains(last, "2 is ready") || !strings.Contains(last, "esc cancel") {
		t.Fatalf("waiting row %q", last)
	}
	h.clock.t = h.clock.t.Add(21 * time.Second)
	h.send(tickMsg(h.clock.t))
	if last := lastLine(h); !strings.Contains(last, "00:21") {
		t.Errorf("the wait does not count up: %q", last)
	}
	h.send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if h.m.fetch.wait != nil || h.m.choosing != "" || strings.Contains(lastLine(h), "fetching door") {
		t.Fatalf("esc kept the wait: %+v, choosing %q, row %q", h.m.fetch.wait, h.m.choosing, lastLine(h))
	}
	h.send(doorReadyMsg{index: 0, id: slow})
	if h.m.state.Phase != game.PhaseDoors {
		t.Fatalf("a download let go of opened its door: phase %q", h.m.state.Phase)
	}

	h.send(game.ChooseDoor{Index: 0})
	h.send(game.ChooseDoor{Index: 1})
	if cur := h.m.state.Current; h.m.state.Phase != game.PhaseProblem || cur == nil || cur.Door.Problem != quick {
		t.Fatalf("choosing the ready door while another downloads: phase %q current %+v", h.m.state.Phase, cur)
	}
	if h.m.fetch.wait != nil {
		t.Error("the wait outlived the door that opened")
	}
}

// The cards show a download's bytes as they arrive, and a failed one.
func TestDoorCardsShowTheirFetch(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.toDoors()
	ds := h.m.state.Doors
	b := &h.m.fetch.board
	b.set(ds[0].Problem, bootui.FetchGoing)
	b.adder(ds[0].Problem)(3 << 20)
	b.set(ds[1].Problem, bootui.FetchFailed)
	h.send(tickMsg(h.clock.t))
	if v := view(h); !strings.Contains(v, "fetching 3M") || !strings.Contains(v, "fetch failed") {
		t.Fatalf("cards without their downloads:\n%s", v)
	}
}
