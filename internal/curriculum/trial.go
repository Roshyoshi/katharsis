package curriculum

import (
	"slices"
	"sort"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// DefaultUnitSize is the number of unit doors in a module trial: the rooms
// of rules.Trial's unit biome, its first (DESIGN.md "Learn mode": 4 doors
// from the module list, Easy -> Hard). A shorter unit is the engine's to
// top up, so this is also the size the RTFM screens promise.
var DefaultUnitSize = rules.Trial().Biomes[0].Rooms

// Trial is the learn-mode challenge for one module: a unit of problems from
// the module's own lists, then two blind integration doors the engine draws
// from the catalog by topic and rating band.
//
// Every level of the atlas but a page has one (Kind.HasTrial). Most grant
// the capability of the module's topic. One whose topic is ungated (Bronze
// material, "mixed") grants nothing: passing it records the module in
// Meta.TrialsPassed, the level's ✓, and no more. A castle, a division's
// conclusion, is its gauntlet and grants nothing either; see TrialForN.
type Trial struct {
	Module Module
	// Unit is playable problems from the module, sorted Very Easy ->
	// Insane. It can be shorter than requested (or empty) when the module
	// lists few judgeable problems.
	Unit       []catalog.ID
	IntegTopic string // catalog topic for the blind integration doors
	IntegBand  [2]int // CF-scale rating band, from the module's division
	// Capability is what passing grants: CapabilityFor(Module.Topic), ""
	// when the module teaches no gated topic.
	Capability string
	// Kind is the module's level on the atlas: PageLevel when it has no
	// trial to start (General, or no capability and no problem the game
	// can judge).
	Kind LevelKind
}

// Spec is the trial as game.NewTrial plays it. A unit shorter than
// DefaultUnitSize is fine: the engine tops it up from the catalog by
// IntegTopic and IntegBand (rules.Trial sets the size).
func (t Trial) Spec() game.TrialSpec {
	return game.TrialSpec{
		Module:     t.Module.ID,
		Title:      t.Module.Title,
		Unit:       slices.Clone(t.Unit),
		IntegTopic: t.IntegTopic,
		IntegBand:  t.IntegBand,
		Capability: t.Capability,
	}
}

// TrialFor builds the trial for m with DefaultUnitSize unit doors.
// judgeable reports whether the catalog can serve and judge an ID; nil
// accepts every reference CatalogID maps.
func TrialFor(m Module, judgeable func(catalog.ID) bool) Trial {
	return TrialForN(m, judgeable, DefaultUnitSize)
}

type candidate struct {
	ref   ProblemRef
	order int // position in the module's lists
}

// rankWithin orders problems sharing a difficulty label: starred first,
// then problems whose solution is not already walked through in the module
// text (a focus problem is solved in the reading, so it tests little), then
// guide order.
func rankWithin(a, b candidate) bool {
	if a.ref.Starred != b.ref.Starred {
		return a.ref.Starred
	}
	ai, bi := a.ref.SolutionKind == "in-module", b.ref.SolutionKind == "in-module"
	if ai != bi {
		return !ai
	}
	return a.order < b.order
}

// TrialForN picks up to n unit doors. Picking the n easiest would often
// give four "Easy" problems; the unit should ramp like the module does, so
// selection goes round-robin over difficulty labels from easiest up,
// taking the best-ranked remaining problem of each label per round, and the
// result is sorted by label.
//
// A castle's trial is built the same way, and that is already a division
// gauntlet: a conclusion has no topic (TopicFor), so its unit is the
// conclusion's own problems, the guide's uncategorised practice for the
// division, ramped as above; the engine tops a short unit up and draws the
// integration doors from every topic (Mixed) at the division's band; and
// no capability is granted. Nothing new is asked of the engine, so a castle
// trial plays and replays as any other.
func TrialForN(m Module, judgeable func(catalog.ID) bool, n int) Trial {
	t := Trial{
		Module:     m,
		IntegTopic: m.Topic,
		IntegBand:  DivisionBand(m.Division),
		Capability: CapabilityFor(m.Topic),
	}
	if t.IntegTopic == "" {
		t.IntegTopic = catalog.Mixed
	}

	seen := map[catalog.ID]int{}
	var cands []candidate
	for _, l := range m.Problems {
		for _, p := range l.Problems {
			if p.ID == "" || judgeable != nil && !judgeable(p.ID) {
				continue
			}
			if i, dup := seen[p.ID]; dup {
				// Listed twice (focus list and main list): starred
				// anywhere counts.
				cands[i].ref.Starred = cands[i].ref.Starred || p.Starred
				continue
			}
			seen[p.ID] = len(cands)
			cands = append(cands, candidate{ref: p, order: len(cands)})
		}
	}

	byLabel := map[int][]candidate{}
	var labels []int
	for _, c := range cands {
		r := DifficultyRank(c.ref.Difficulty)
		if _, ok := byLabel[r]; !ok {
			labels = append(labels, r)
		}
		byLabel[r] = append(byLabel[r], c)
	}
	t.Kind = levelKind(m.ID, m.Division, len(cands) > 0)
	sort.Ints(labels)
	for _, r := range labels {
		g := byLabel[r]
		sort.SliceStable(g, func(i, j int) bool { return rankWithin(g[i], g[j]) })
	}

	// The unit ramps Easy → Hard (DESIGN.md "Learn mode"). The guide's Very
	// Hard and Insane problems are its stretch: they fill a unit only when
	// the module has too few others, as the module's own problems still
	// beat the catalog's filler.
	var ramp, stretch []int
	for _, r := range labels {
		if r == difficultyRank["Very Hard"] || r == difficultyRank["Insane"] {
			stretch = append(stretch, r)
		} else {
			ramp = append(ramp, r)
		}
	}
	var picked []candidate
	for _, pass := range [][]int{ramp, stretch} {
		for len(picked) < n {
			progress := false
			for _, r := range pass {
				if len(picked) == n {
					break
				}
				if g := byLabel[r]; len(g) > 0 {
					picked = append(picked, g[0])
					byLabel[r] = g[1:]
					progress = true
				}
			}
			if !progress {
				break
			}
		}
	}
	sort.SliceStable(picked, func(i, j int) bool {
		ri, rj := DifficultyRank(picked[i].ref.Difficulty), DifficultyRank(picked[j].ref.Difficulty)
		if ri != rj {
			return ri < rj
		}
		return rankWithin(picked[i], picked[j])
	})
	for _, c := range picked {
		t.Unit = append(t.Unit, c.ref.ID)
	}
	return t
}
