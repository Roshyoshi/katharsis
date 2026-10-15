package learn

import (
	"slices"
	"sort"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// DefaultUnitSize is the number of unit doors in a module trial (DESIGN.md
// "Learn mode": 4 doors from the module list, Easy -> Hard).
const DefaultUnitSize = 4

// Trial is the learn-mode challenge for one module: a unit of problems from
// the module's own lists, then two blind integration doors the engine draws
// from the catalog by topic and rating band.
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
// accepts every USACO/CSES reference.
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
func TrialForN(m Module, judgeable func(catalog.ID) bool, n int) Trial {
	t := Trial{
		Module:     m,
		IntegTopic: m.Topic,
		IntegBand:  DivisionBand(m.Division),
		Capability: CapabilityFor(m.Topic),
	}
	if t.IntegTopic == "" {
		t.IntegTopic = "mixed"
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
	sort.Ints(labels)
	for _, r := range labels {
		g := byLabel[r]
		sort.SliceStable(g, func(i, j int) bool { return rankWithin(g[i], g[j]) })
	}

	var picked []candidate
	for len(picked) < n {
		progress := false
		for _, r := range labels {
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
