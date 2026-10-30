package curriculum

import (
	"slices"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// Trial is the learn-mode challenge for one module (DESIGN.md "Learn
// mode"): three unseen doors on its topic, drawn by the engine from the
// trials' pool (Split), which no module map shows and no boot draws. The
// module's map aims them: they climb from the middle of its core problems
// to past its hardest, so passing says the player can do on problems they
// have not seen what the map taught.
//
// Every level of the atlas but a page has one (Kind.HasTrial). Most grant
// the capability of the module's topic. One whose topic is ungated (Bronze
// material, "mixed") grants nothing: passing it records the module in
// Meta.TrialsPassed, the level's ✓, and opens the next level. A castle, a
// division's conclusion, draws on every topic at its division's band and
// grants nothing either.
type Trial struct {
	Module Module
	// Map is the module's map as the catalog can play it.
	Map   Map
	Topic string // catalog topic of the doors, catalog.Mixed for any
	Band  [2]int // CF-scale ratings, from the module's division
	// Targets are the doors' ratings, easiest first; none when the map's
	// core holds no rated problem, and the engine spreads the doors over
	// Band.
	Targets []int
	// Capability is what passing grants: CapabilityFor(Module.Topic), ""
	// when the module teaches no gated topic.
	Capability string
	// Kind is the module's level on the atlas: PageLevel when it has no
	// trial to start (General, or no capability and no problem the game
	// can judge).
	Kind LevelKind
}

// Spec is the trial as game.NewTrial plays it.
func (t Trial) Spec() game.TrialSpec {
	return game.TrialSpec{
		Module:     t.Module.ID,
		Title:      t.Module.Title,
		Topic:      t.Topic,
		Band:       t.Band,
		Targets:    slices.Clone(t.Targets),
		Capability: t.Capability,
	}
}

// Doors is how many doors a trial plays: rules.Trial's rooms.
var Doors = func() int {
	n := 0
	for _, b := range rules.Trial().Biomes {
		n += b.Rooms
	}
	return n
}()

// TrialFor builds m's trial. judgeable reports whether the catalog can
// serve and judge an ID, for the map (Map.Playable); nil accepts every
// node with a catalog ID. lookup gives the catalog's problem, for its
// rating; nil knows none.
//
// The doors aim at the median of the core's ratings, its hardest, and
// 100 past that (one of the rules' rating windows): the first is what
// the map's middle asked, the last a step past its capstone. CSES ratings
// come from solver counts (catalog.ShowsRating), too rough to aim by, so
// they count only when nothing else on the core is rated.
func TrialFor(m Module, judgeable func(catalog.ID) bool, lookup func(catalog.ID) (catalog.Problem, bool)) Trial {
	mp, _ := MapFor(m)
	t := Trial{
		Module:     m,
		Map:        mp.Playable(judgeable),
		Topic:      m.Topic,
		Band:       DivisionBand(m.Division),
		Capability: CapabilityFor(m.Topic),
	}
	if t.Topic == "" {
		t.Topic = catalog.Mixed
	}
	t.Kind = levelKind(m.ID, m.Division, len(t.Map.Nodes) > 0)
	t.Targets = aim(t.Map, lookup)
	return t
}

// aim is the trial's door ratings for map mp (see TrialFor), nil for none.
func aim(mp Map, lookup func(catalog.ID) (catalog.Problem, bool)) []int {
	if lookup == nil {
		return nil
	}
	var shown, rough []int
	for _, n := range mp.Nodes {
		if !n.Core {
			continue
		}
		if p, ok := lookup(n.ID); ok && p.Rating > 0 {
			if catalog.ShowsRating(n.ID) {
				shown = append(shown, p.Rating)
			} else {
				rough = append(rough, p.Rating)
			}
		}
	}
	rs := shown
	if len(rs) == 0 {
		rs = rough
	}
	if len(rs) == 0 {
		return nil
	}
	slices.Sort(rs)
	mid, top := rs[(len(rs)-1)/2], rs[len(rs)-1]
	step := rules.Trial().WithDefaults().RatingWindows[0]
	out := []int{mid, top, top + step}
	for len(out) < Doors {
		out = append(out, top+step)
	}
	return out[:Doors]
}
