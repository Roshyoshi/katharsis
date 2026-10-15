package game_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// A merge window sells items for uncommitted bytes: each costs its price,
// does what it says, and is refused, changing nothing, when the bytes do
// not cover it, when it would do nothing, or outside a merge window.
func TestSpend(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	r := plain()
	h := newHarness(t, r, nil, ps, 0, 1)
	if err := h.try(game.Spend{Item: game.Dup}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("spent at the doors: %v", err)
	}
	h.toMerge()
	h.pick()
	s := h.s
	refuse := func(it game.Item, why string) {
		t.Helper()
		before := mustJSON(t, s)
		if err := h.try(game.Spend{Item: it}); !errors.Is(err, game.ErrInvalid) || mustJSON(t, s) != before {
			t.Errorf("%s (%s): err %v, state changed %v", it, why, err, mustJSON(t, s) != before)
		}
	}
	refuse("bogus", "not sold")
	s.Cycles = s.MaxCycles
	refuse(game.Fsck, "full lives")
	s.Cycles = 2
	s.Uncommitted = r.FsckPrice + r.DupPrice + r.ShufPrice - 1
	earned, ducks := s.Uncommitted, s.Ducks

	h.do(game.Spend{Item: game.Fsck})
	h.do(game.Spend{Item: game.Dup})
	if s.Cycles != 3 || s.Ducks != ducks+1 || s.Spent != r.FsckPrice+r.DupPrice || s.Uncommitted != earned-s.Spent {
		t.Errorf("after fsck and dup: lives %d ducks %d spent %d uncommitted %d", s.Cycles, s.Ducks, s.Spent, s.Uncommitted)
	}
	refuse(game.Shuf, "a byte short")
	s.Uncommitted++
	h.do(game.Spend{Item: game.Shuf})
	if s.Rerolls != 1 || s.Uncommitted != 0 {
		t.Errorf("after shuf: %d shufs, %d uncommitted", s.Rerolls, s.Uncommitted)
	}

	off := plain()
	off.DupPrice = 0
	h = newHarness(t, off, nil, ps, 0, 1)
	h.toMerge()
	h.pick()
	h.s.Uncommitted = 1000
	if err := h.try(game.Spend{Item: game.Dup}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("bought dup at price 0: %v", err)
	}
}

// A shuf re-rolls the room on offer once, keeping its regression door; it
// cannot touch a boss room, or a room with an upgrade still to pick.
func TestReroll(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	target := catalog.ID("t:dp:1200")
	m := &game.Meta{Regressions: []game.Regression{{Problem: target, Due: t0.Add(-time.Hour), Days: []int{3, 17}}}}
	h := newHarness(t, plain(), m, ps, 0, 1)
	s := h.s
	if err := h.try(game.Reroll{}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("re-rolled with no shuf: %v", err)
	}
	h.toMerge()
	h.pick()
	s.Uncommitted = 2 * h.s.Rules.ShufPrice
	h.do(game.Spend{Item: game.Shuf})
	h.do(game.Spend{Item: game.Shuf})
	h.do(game.LeaveMerge{})
	h.do(game.ChooseRoute{Index: 0})
	before := slices.Clone(s.Doors)
	reg := slices.IndexFunc(before, func(d game.Door) bool { return d.Regression })
	if reg < 0 {
		t.Fatalf("no regression door in %+v", before)
	}
	fx := h.do(game.Reroll{})
	if s.Rerolls != 1 || len(s.Doors) != len(before) {
		t.Fatalf("after a shuf: %d left, %d doors", s.Rerolls, len(s.Doors))
	}
	if p, ok := has[game.Prefetch](fx); !ok || len(p.Problems) != len(s.Doors) {
		t.Errorf("shuf effects %v", fx)
	}
	kept := 0
	for _, d := range s.Doors {
		if d.Regression && d.Problem == target {
			kept++
		}
		if !d.Regression && slices.ContainsFunc(before, func(b game.Door) bool { return b.Problem == d.Problem }) {
			t.Errorf("door %s survived the shuf", d.Problem)
		}
	}
	if kept != 1 {
		t.Errorf("regression door kept %d times in %+v", kept, s.Doors)
	}

	// No shuf on a boss room.
	for s.Phase != game.PhaseMerge {
		h.clearDoor(0, 10*time.Minute)
	}
	h.pick()
	h.do(game.LeaveMerge{})
	if s.Biomes[s.Biome].Kind != "boss" || s.Phase != game.PhaseDoors {
		t.Fatalf("biome %d phase %s, want the boss doors", s.Biome, s.Phase)
	}
	if err := h.try(game.Reroll{}); !errors.Is(err, game.ErrInvalid) || s.Rerolls != 1 {
		t.Errorf("shuffled the boss: %v, %d left", err, s.Rerolls)
	}
}

// Spending takes bytes out of the crash's reach: what a crash loses is
// what was still uncommitted.
func TestSpendThenCrash(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	h := newHarness(t, plain(), nil, ps, 0, 1)
	h.toMerge()
	h.pick()
	h.s.Cycles = 1
	earned := h.s.Uncommitted
	h.do(game.Spend{Item: game.Fsck})
	h.do(game.LeaveMerge{})
	h.do(game.ChooseRoute{Index: 0})
	h.open(0)
	h.do(game.GiveUp{})
	c := h.s.Crash
	if c.Lost != earned-h.s.Rules.FsckPrice || h.m.CoreDump == nil || h.m.CoreDump.Bytes != c.Lost {
		t.Errorf("earned %d, spent %d: crash lost %d, dump %+v", earned, h.s.Spent, c.Lost, h.m.CoreDump)
	}
}
