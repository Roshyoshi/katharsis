package curriculum

import (
	"os"
	"slices"
	"strings"
	"testing"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// The fixture's atlas: a world per division in guide order, General's
// levels pages, a module with judgeable problems a trial level, and
// progress read from Meta, every count with its real denominator.
func TestAtlas(t *testing.T) {
	c, err := Load("testdata/guide")
	if err != nil {
		t.Fatal(err)
	}
	a := c.Atlas(nil)
	var ids []string
	for _, w := range a.Worlds {
		ids = append(ids, w.ID)
	}
	if want := []string{"general", "bronze", "silver", "gold", "plat", "adv"}; !slices.Equal(ids, want) {
		t.Fatalf("worlds %v, want %v", ids, want)
	}
	for _, l := range a.Worlds[0].Levels() {
		if l.Kind != PageLevel {
			t.Errorf("General's %s is %v, want a page", l.Module, l.Kind)
		}
	}
	var trial Level
	for _, w := range a.Worlds[1:] {
		for _, l := range w.Levels() {
			if l.Kind == TrialLevel && len(l.Playable) > 0 {
				trial = l
			}
			if (l.Kind == PageLevel) != (len(l.Playable) == 0 && l.Capability == "") {
				t.Errorf("%s: kind %v with %d playable", l.Module, l.Kind, len(l.Playable))
			}
		}
	}
	if trial.Module == "" {
		t.Fatal("no trial level in the fixture")
	}

	m := &game.Meta{TrialsPassed: []string{trial.Module}, Opened: []string{trial.Module},
		Solves: []game.Solve{{Problem: trial.Playable[0]}}}
	if trial.Capability != "" {
		m.Capabilities = []string{trial.Capability}
	}
	p := a.Progress(m)
	if lp := p[trial.Module]; !lp.Passed || !lp.Opened || lp.Solved != 1 || lp.CapHeld != (trial.Capability != "") {
		t.Errorf("progress %+v", lp)
	}
	_, wi, _ := a.Level(trial.Module)
	if s := p.Sum(a.Worlds[wi]); s.Passed != 1 || s.Solved != 1 || s.Trials < 1 || s.Playable < len(trial.Playable) {
		t.Errorf("world sum %+v", s)
	}
	none := a.Progress(&game.Meta{})
	if lp := none[trial.Module]; lp.Passed || lp.Solved != 0 {
		t.Errorf("a new player's progress %+v", lp)
	}
}

// The real guide's atlas, as the plan counted it: six worlds, 144 levels,
// the four conclusions castles, the warps exactly the guide's three
// world-skipping edges, and no prerequisite later in guide order.
func TestFullAtlas(t *testing.T) {
	dir := os.Getenv("KATHARSIS_GUIDE")
	if dir == "" {
		t.Skip("set KATHARSIS_GUIDE to a usaco-guide checkout to run")
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	judgeable := func(id catalog.ID) bool { return id.Source() == catalog.USACO || id.Source() == catalog.CSES }
	a := c.Atlas(judgeable)
	levels, castles := 0, 0
	seen := map[string]int{}
	var warps []string
	for _, w := range a.Worlds {
		for _, l := range w.Levels() {
			seen[l.Module] = levels
			levels++
			if l.Kind == CastleLevel {
				castles++
			}
			for _, to := range l.Warps {
				warps = append(warps, l.Module+">"+to)
			}
		}
	}
	slices.Sort(warps)
	if len(a.Worlds) != 6 || levels != 144 || castles != 4 {
		t.Errorf("%d worlds, %d levels, %d castles; want 6, 144, 4", len(a.Worlds), levels, castles)
	}
	if want := []string{"graph-traversal>eulerian-tours", "graph-traversal>string-search", "intro-tree>centroid"}; !slices.Equal(warps, want) {
		t.Errorf("warps %s, want %s", strings.Join(warps, " "), strings.Join(want, " "))
	}
	// Bronze's Introduction to Data Structures lists no problem: its
	// capability is earned four levels on, in Introduction to Sets & Maps.
	if l, _, ok := a.EarnedIn("intro-ds"); !ok || l.Module != "intro-sets" {
		t.Errorf("intro-ds's capability is earned in %q (%v), want intro-sets", l.Module, ok)
	}
	for _, w := range a.Worlds {
		for _, l := range w.Levels() {
			for _, p := range l.Prereqs {
				if seen[p] > seen[l.Module] {
					t.Errorf("%s needs %s, later in guide order", l.Module, p)
				}
			}
		}
	}
}

// Every level of the real guide but a page yields a trial the engine can
// plan: the trial's kind is the atlas's, its band is not empty, and only a
// level whose topic is gated grants a capability.
func TestFullAtlasTrials(t *testing.T) {
	dir := os.Getenv("KATHARSIS_GUIDE")
	if dir == "" {
		t.Skip("set KATHARSIS_GUIDE to a usaco-guide checkout to run")
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	judgeable := func(id catalog.ID) bool { return id.Source() == catalog.USACO || id.Source() == catalog.CSES }
	kinds := map[LevelKind]int{}
	free, topped := 0, 0
	for _, w := range c.Atlas(judgeable).Worlds {
		for _, l := range w.Levels() {
			kinds[l.Kind]++
			if l.Kind.HasTrial() && len(l.Playable) == 0 {
				topped++
			}
			m, _ := c.Module(l.Module)
			tr := TrialFor(m, judgeable)
			spec := tr.Spec()
			switch {
			case tr.Kind != l.Kind:
				t.Errorf("%s: trial kind %v, atlas kind %v", l.Module, tr.Kind, l.Kind)
			case !l.Kind.HasTrial():
			case spec.Module != l.Module || spec.IntegBand[0] > spec.IntegBand[1] || spec.IntegTopic == "":
				t.Errorf("%s: spec %+v cannot be planned", l.Module, spec)
			case spec.Capability != l.Capability || l.Kind == CastleLevel && spec.Capability != "":
				t.Errorf("%s (%v): grants %q, want %q", l.Module, l.Kind, spec.Capability, l.Capability)
			case spec.Capability == "":
				free++
			}
		}
	}
	t.Logf("%d trial levels (%d granting nothing, castles included; %d all topped up), %d castles, %d pages",
		kinds[TrialLevel], free, topped, kinds[CastleLevel], kinds[PageLevel])
}

// A gated module with nothing to judge is a page whose capability is earned
// in the next trial on it, else the last before: the fixture's Counting
// Minimums (Advanced, CAP_DS, CSES and others only) points back to Gold's
// Hashmaps. A level with its own trial, or none to earn, points nowhere.
func TestEarnedIn(t *testing.T) {
	c, err := Load("testdata/guide")
	if err != nil {
		t.Fatal(err)
	}
	a := c.Atlas(func(id catalog.ID) bool { return id.Source() == catalog.USACO })
	if l, _, _ := a.Level("count-min"); l.Kind != PageLevel || l.Capability == "" {
		t.Fatalf("count-min is %v granting %q, want a gated page", l.Kind, l.Capability)
	}
	if l, wi, ok := a.EarnedIn("count-min"); !ok || l.Module != "hashmaps" || a.Worlds[wi].ID != "gold" {
		t.Errorf("count-min's capability is earned in %q (world %d, %v), want gold's hashmaps", l.Module, wi, ok)
	}
	for _, id := range []string{"hashmaps", "casework", "fast-io", "no-such-module"} {
		if l, _, ok := a.EarnedIn(id); ok {
			t.Errorf("%s points to %s", id, l.Module)
		}
	}
}

// General's problems are its pages' examples, not practice: a solve of one
// counts in no world's tally. The fixture's Fast I/O has one the USACO
// catalog judges.
func TestLibraryCountsNoPractice(t *testing.T) {
	c, err := Load("testdata/guide")
	if err != nil {
		t.Fatal(err)
	}
	a := c.Atlas(func(id catalog.ID) bool { return id.Source() == catalog.USACO })
	io, wi, ok := a.Level("fast-io")
	if !ok || len(io.Playable) == 0 || io.Kind != PageLevel {
		t.Fatalf("fast-io: %+v; want a page with a judgeable example", io)
	}
	p := a.Progress(&game.Meta{Solves: []game.Solve{{Problem: io.Playable[0]}}})
	if s := p.Sum(a.Worlds[wi]); s.Playable != 0 || s.Solved != 0 {
		t.Errorf("General counts %d/%d practice, want none", s.Solved, s.Playable)
	}
}
