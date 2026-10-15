package game_test

import (
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// A clock that stepped back between opening a door and clearing it gave
// the solve a negative time and the -O3 free upgrade (qa/chaos r14b).
func TestClockBehindTheDoor(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, plain(), nil, ps, 0, 1)
	h.clearDoor(0, -24*time.Hour)
	a := h.s.History[0]
	if a.O3 || h.s.UpgradePicks != 1 {
		t.Errorf("O3 %v picks %d, want no -O3 and one pick", a.O3, h.s.UpgradePicks)
	}
	if got := h.m.Solves[0].Elapsed; got != 0 {
		t.Errorf("solve elapsed %v, want 0", got)
	}
}

// An upsolve timed across a clock step back records 0, not a negative
// solve time.
func TestUpsolveClockBehind(t *testing.T) {
	t.Parallel()
	var m game.Meta
	game.Upsolve(&m, "usaco:1", game.Independent, -time.Hour, t0)
	if got := m.Solves[0].Elapsed; got != 0 {
		t.Errorf("solve elapsed %v, want 0", got)
	}
}
