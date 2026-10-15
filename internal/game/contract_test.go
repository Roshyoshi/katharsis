package game_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// TestStateKeysKept: lives were renamed cycles (D48) but their JSON keys
// were not, so the store's saved boots load as before.
func TestStateKeysKept(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 50)
	h := newHarness(t, plain(), nil, ps, 0, 1)
	h.open(0)
	h.s.Ease = []game.Ease{{Kind: game.EaseFloor, Why: "w", Cycles: 10}}
	js := mustJSON(t, h.s)
	for _, k := range []string{`"Lives":3,`, `"MaxLives":5,`, `"LivesAtOpen":3,`, `"Lives":10}`} {
		if !strings.Contains(js, k) {
			t.Errorf("state JSON has no %s", k)
		}
	}
	for _, k := range []string{`"Cycles"`, `"MaxCycles"`, `"CyclesAtOpen"`} {
		if strings.Contains(js, k) {
			t.Errorf("state JSON names %s; saved boots name lives", k)
		}
	}
}

// TestAbandon: a new boot while one is suspended ends the old one as a
// crash (meta.md §(c)): its uncommitted bytes are lost, an earlier boot's
// core dump is destroyed, no door is dumped on or scheduled, and the
// governor does not count it.
func TestAbandon(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 50)
	dump := &game.CoreDump{Problem: "t:dp:1500", Bytes: 77, Boot: 1, At: t0}
	start := func() *harness {
		m := &game.Meta{CoreDump: dump, Governor: game.Governor{EarlyCrashes: 1, FullClears: 2}}
		return newHarness(t, plain(), m, ps, 0, 3)
	}
	for _, tc := range []struct {
		name string
		to   func(h *harness)
	}{
		{"doors", func(h *harness) {}},
		{"an upgrade pick", func(h *harness) { h.clearDoor(0, time.Minute) }},
		{"merge", func(h *harness) { h.toMerge() }},
		{"route", func(h *harness) { h.toMerge(); h.pick(); h.do(game.LeaveMerge{}) }},
	} {
		h := start()
		tc.to(h)
		if !h.s.Suspendable() {
			t.Fatalf("%s: phase %s is not suspendable", tc.name, h.s.Phase)
		}
		up, gov, regs := h.s.Uncommitted, h.m.Governor, len(h.m.Regressions)
		h.do(game.Abandon{})
		c := h.s.Crash
		if h.s.Phase != game.PhaseCrashed || c == nil || c.Cause != game.Abandoned || c.Lost != up || c.DumpLost != 77 ||
			c.Door.Problem != "" || h.m.CoreDump != nil || h.s.Uncommitted != 0 || len(h.s.Doors) != 0 {
			t.Errorf("%s: phase %s crash %+v dump %+v", tc.name, h.s.Phase, c, h.m.CoreDump)
		}
		if h.m.Governor != gov || len(h.m.Regressions) != regs || h.s.Suspendable() {
			t.Errorf("%s: governor %+v (was %+v), %d regressions (was %d)", tc.name, h.m.Governor, gov, len(h.m.Regressions), regs)
		}
		if b := game.Debrief(h.s, h.m); b.Lever.Kind != game.LeverAbandoned {
			t.Errorf("%s: debrief lever %q", tc.name, b.Lever.Kind)
		}
		if err := h.try(game.Abandon{}); !errors.Is(err, game.ErrInvalid) {
			t.Errorf("%s: abandoned twice: %v", tc.name, err)
		}
	}

	// With a door open it is the door's give-up, ending the boot.
	h := start()
	d := h.open(0)
	if h.s.Suspendable() {
		t.Error("a door open is suspendable")
	}
	h.do(game.Abandon{})
	if c := h.s.Crash; c == nil || c.Cause != game.GaveUp || c.Door.Problem != d.Problem {
		t.Errorf("abandoned in a door: %+v", c)
	}
}

// TestCostsBeforeCycles: on a boot saved before cycles every price is one
// life, a timeout ends the boot outside D44's rule and a give-up always
// does, and a boss is one-shot.
func TestCostsBeforeCycles(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 50)
	r := noFirst(rules.Legacy())
	h := newHarness(t, r, nil, ps, 0, 1)
	h.open(0)
	s := h.s
	if c, ok := s.FailCost(game.TimedOut); c != 1 || !ok {
		t.Errorf("Userland timeout costs %d (%v), want a life", c, ok)
	}
	if c, ok := s.FailCost(game.GaveUp); c != 0 || ok {
		t.Errorf("give-up costs %d (%v), want the boot", c, ok)
	}
	if c, k := s.SubmitCost(1); c != 1 || k != 0 {
		t.Errorf("failed submit costs %d and %s", c, k)
	}
	if c, free := s.RetreatCost(); c != 1 || free || !s.RetreatLockAt().IsZero() {
		t.Errorf("retreat costs %d (free %v), locks at %v", c, free, s.RetreatLockAt())
	}
	if s.FsckHeal() != 1 || s.ECCHeal() != 1 || s.StartCycles() != 3 || s.ECCConverts() || s.OneShot() {
		t.Errorf("heals %d %d, start %d, ecc %v, one-shot %v", s.FsckHeal(), s.ECCHeal(), s.StartCycles(), s.ECCConverts(), s.OneShot())
	}
	s.Cycles = 1
	if _, ok := s.FailCost(game.TimedOut); ok || s.TimeoutCostsLife() {
		t.Error("a timeout on the last life spared the boot")
	}
	s.Biome = 2
	if !s.OneShot() {
		t.Error("a boss before cycles is not one-shot")
	}
	if acts := s.Acts(); len(acts) != 1 || acts[0] != (game.Act{First: 0, Last: 2}) || s.ActOf(1) != 0 {
		t.Errorf("acts %+v", acts)
	}
}

// TestMapGraph: the map's graph against the engine's tables. Every boss
// node names a boss the story knows; every node's gate is its topic's
// capability or a package; and a player with no capability, no package and
// no prestige reaches the final through open nodes alone, as Dead Cells
// always leaves a rune-less route. The Trial, hidden behind a full pool,
// is no part of that route: it ends the boot where it stands.
func TestMapGraph(t *testing.T) {
	t.Parallel()
	r := rules.Map()
	nodes := map[string]rules.Biome{}
	for _, n := range r.Nodes {
		nodes[n.ID] = n
		if n.Boss != "" && !slices.Contains(game.BossKeys, n.Boss) && n.Boss != game.FinalBoss && n.Boss != game.TrialBoss {
			t.Errorf("node %s: boss %q unknown", n.ID, n.Boss)
		}
		if n.Topic != "" && !slices.Contains(catalog.Topics, n.Topic) {
			t.Errorf("node %s: topic %q unknown", n.ID, n.Topic)
		}
		if n.Picks > 0 && !n.Skip && game.CapabilityFor(n.Topic) == "" {
			t.Errorf("node %s: a scroll without a gate", n.ID)
		}
		if n.Package != "" {
			if _, ok := r.Package(n.Package); !ok {
				t.Errorf("node %s: package %q unknown", n.ID, n.Package)
			}
		}
	}
	if nodes["halting-problem"].Tier != rules.TierFinal {
		t.Fatal("The Halting Problem is not the final")
	}
	open := func(n rules.Biome) bool {
		return n.MinPrestige == 0 && game.CapabilityFor(n.Topic) == "" && !n.Hidden && !n.NeedFull && n.Ending == ""
	}
	reach := map[string]bool{"userland": true}
	for tier := range len(r.Biomes) - 1 {
		for _, n := range r.Nodes {
			if n.Tier != tier || !reach[n.ID] {
				continue
			}
			for _, id := range n.Next {
				if open(nodes[id]) {
					reach[id] = true
				}
			}
		}
		// Every node of a tier reached can go on through an open exit.
		for _, n := range r.Nodes {
			if n.Tier == tier && reach[n.ID] && !slices.ContainsFunc(n.Next, func(id string) bool { return open(nodes[id]) }) {
				t.Errorf("node %s has no open exit", n.ID)
			}
		}
	}
	if !reach["halting-problem"] {
		t.Error("no open route to the final")
	}
	// Each Boss II exits to act III's mixed biome and to the skip.
	for _, id := range []string{"heisenbug", "race-condition", "undefined-behavior"} {
		if n := nodes[id]; !slices.Contains(n.Next, "kernel") || !slices.Contains(n.Next, "skip") {
			t.Errorf("%s exits %v", id, n.Next)
		}
	}
	if s := nodes["skip"]; !s.Skip || s.Picks != 2 {
		t.Errorf("skip %+v", s)
	}
	// Every act II node leads, hidden, to the Trial at Boss II's tier.
	if tr := nodes[rules.TrialNode]; tr.Tier != rules.TierBossII || tr.Boss != game.TrialBoss || !tr.Hidden || !tr.NeedFull ||
		!tr.Cursed || tr.Ending != game.EndingWake || len(tr.Next) != 0 || tr.FailCycles != 0 {
		t.Errorf("the Trial %+v", tr)
	}
	for _, n := range r.Nodes {
		if n.Tier == rules.TierActII && !slices.Contains(n.Next, rules.TrialNode) {
			t.Errorf("%s does not lead to the Trial: %v", n.ID, n.Next)
		}
	}
}
