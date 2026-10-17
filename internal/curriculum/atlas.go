package curriculum

import (
	"slices"
	"strings"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// The atlas is RTFM's world map (.agents/work/rtfm-map-plan.md): the
// guide's divisions are worlds, its chapters regions, its modules levels,
// all in guide order, which is a valid order to read them in (no module's
// prerequisite comes later). Nothing in it is invented: a level's kind
// follows from its module, and a warp is a real prerequisite edge.

// LevelKind is what a level holds.
type LevelKind int

const (
	// TrialLevel has a trial: a capability's, or for a module that grants
	// none, one that clears the level only.
	TrialLevel LevelKind = iota
	// PageLevel is reading only: General's pages, and a module that
	// grants no capability and lists no problem the game can judge. A
	// module that grants one keeps its trial with nothing judgeable: the
	// engine tops the whole unit up.
	PageLevel
	// CastleLevel is a division's conclusion: a mixed trial at the
	// division's band, the world's gauntlet.
	CastleLevel
)

// HasTrial reports whether a level of kind k has a trial to start.
func (k LevelKind) HasTrial() bool { return k != PageLevel }

// Level is one module on the map.
type Level struct {
	Module, Title string
	Kind          LevelKind
	// Optional marks a module the guide calls optional or says has not
	// appeared in contests (Frequency 0): its path is drawn muted.
	Optional bool
	// Capability is what the module's trial grants, "" for none.
	Capability string
	// Prereqs are the module's prerequisites that are modules of the
	// guide; Warps the later-world modules this one opens a shortcut to:
	// those that skip a whole world and whose prerequisites all sit in
	// this one's world or before it (Super Mario World's Star Road).
	Prereqs, Warps []string
	// Playable are the distinct problems of the module the game can judge,
	// for practice's k/n.
	Playable []catalog.ID
}

// Region is a chapter.
type Region struct {
	Title  string
	Levels []Level
}

// World is a division.
type World struct {
	ID, Title string
	Regions   []Region
}

// Levels is every level of w in order.
func (w World) Levels() []Level {
	var out []Level
	for _, r := range w.Regions {
		out = append(out, r.Levels...)
	}
	return out
}

// Atlas is the map, a world per division.
type Atlas struct {
	Worlds []World
}

// Level finds a module's level and its world's index.
func (a Atlas) Level(id string) (Level, int, bool) {
	for i, w := range a.Worlds {
		for _, l := range w.Levels() {
			if l.Module == id {
				return l, i, true
			}
		}
	}
	return Level{}, -1, false
}

// Conclusion reports whether module id is a division's conclusion, a
// CastleLevel.
func Conclusion(id string) bool { return strings.HasSuffix(id, "-conclusion") }

// generalID is the division of pages about contests in general: a library
// island, reading only.
const generalID = "general"

// Atlas lays the curriculum out as the map. judgeable reports whether the
// catalog can serve and judge an ID; nil accepts every USACO/CSES
// reference. The guide's own pages (Module.Meta) are left out, as the
// index leaves them.
func (c *Curriculum) Atlas(judgeable func(catalog.ID) bool) Atlas {
	if judgeable == nil {
		judgeable = func(catalog.ID) bool { return true }
	}
	div := map[string]int{} // module -> world index
	var a Atlas
	for _, d := range c.Divisions {
		w := World{ID: d.ID, Title: d.Title}
		for _, ch := range d.Chapters {
			r := Region{Title: ch.Title}
			for _, ref := range ch.Modules {
				m, ok := c.modules[ref.ID]
				if !ok || m.Meta {
					continue
				}
				div[m.ID] = len(a.Worlds)
				r.Levels = append(r.Levels, c.level(m, d.ID, judgeable))
			}
			if len(r.Levels) > 0 {
				w.Regions = append(w.Regions, r)
			}
		}
		if len(w.Regions) > 0 {
			a.Worlds = append(a.Worlds, w)
		}
	}
	a.warp(div)
	return a
}

func (c *Curriculum) level(m Module, division string, judgeable func(catalog.ID) bool) Level {
	l := Level{
		Module: m.ID, Title: m.Title, Capability: CapabilityFor(m.Topic),
		Optional: strings.Contains(m.Title, "(Optional)") || m.Frequency == 0,
	}
	for _, p := range m.Prerequisites {
		if q, ok := c.modules[p]; ok && !q.Meta {
			l.Prereqs = append(l.Prereqs, p)
		}
	}
	for _, list := range m.Problems {
		for _, p := range list.Problems {
			if p.ID != "" && judgeable(p.ID) && !slices.Contains(l.Playable, p.ID) {
				l.Playable = append(l.Playable, p.ID)
			}
		}
	}
	l.Kind = levelKind(m.ID, division, len(l.Playable) > 0 || l.Capability != "")
	return l
}

// levelKind is the kind of module id's level in division, given whether
// it can hold a trial outside General: it lists a problem the game can
// judge, or passing grants a capability. A conclusion is a castle even
// with neither: its trial's unit is then all topped up.
func levelKind(id, division string, trial bool) LevelKind {
	switch {
	case Conclusion(id):
		return CastleLevel
	case division == generalID || !trial:
		return PageLevel
	}
	return TrialLevel
}

// warp marks the shortcuts, Super Mario World's Star Road: a module of the
// late worlds (Platinum on) whose prerequisites all sit in the early ones
// (Silver or before) is warped to from each prerequisite in the latest of
// them. Any two-world gap would make fourteen; a secret that common is
// no secret.
func (a *Atlas) warp(div map[string]int) {
	early, late := a.world("silver"), a.world("plat")
	if early < 0 || late < 0 {
		return
	}
	at := map[string]*Level{}
	for i := range a.Worlds {
		for j := range a.Worlds[i].Regions {
			for k := range a.Worlds[i].Regions[j].Levels {
				l := &a.Worlds[i].Regions[j].Levels[k]
				at[l.Module] = l
			}
		}
	}
	for i := range a.Worlds {
		for _, l := range a.Worlds[i].Levels() {
			last := -1
			for _, p := range l.Prereqs {
				last = max(last, div[p])
			}
			if last < 0 || i < late || last > early {
				continue
			}
			for _, p := range l.Prereqs {
				if div[p] == last {
					at[p].Warps = append(at[p].Warps, l.Module)
				}
			}
		}
	}
}

// LevelProgress is a level as the player stands on it.
type LevelProgress struct {
	Passed  bool // its trial passed (Meta.TrialsPassed)
	CapHeld bool // its capability held, from any module's trial
	Opened  bool // its page opened in the reader (Meta.Opened)
	Solved  int  // of its Playable, solved anywhere: a boot, an upsolve, a trial
}

// Progress is the atlas against what the player has done; it is derived,
// never stored.
type Progress map[string]LevelProgress

// Progress reads m.
func (a Atlas) Progress(m *game.Meta) Progress {
	solved := map[catalog.ID]bool{}
	for _, s := range m.Solves {
		solved[s.Problem] = true
	}
	p := Progress{}
	for _, w := range a.Worlds {
		for _, l := range w.Levels() {
			lp := LevelProgress{
				Passed:  slices.Contains(m.TrialsPassed, l.Module),
				CapHeld: l.Capability != "" && slices.Contains(m.Capabilities, l.Capability),
				Opened:  slices.Contains(m.Opened, l.Module),
			}
			for _, id := range l.Playable {
				if solved[id] {
					lp.Solved++
				}
			}
			p[l.Module] = lp
		}
	}
	return p
}

// WorldSum is a world's counts, each with its real denominator: trials
// passed of the levels that have one, capabilities held of those it
// teaches, problems solved of those it lists playable.
type WorldSum struct {
	Passed, Trials   int
	CapsHeld, Caps   int
	Solved, Playable int
	Levels           int
}

// Sum counts w under p.
func (p Progress) Sum(w World) WorldSum {
	var s WorldSum
	caps := map[string]bool{}
	for _, l := range w.Levels() {
		s.Levels++
		lp := p[l.Module]
		if l.Kind.HasTrial() {
			s.Trials++
			if lp.Passed {
				s.Passed++
			}
		}
		if l.Capability != "" && !caps[l.Capability] {
			caps[l.Capability] = true
			s.Caps++
			if lp.CapHeld {
				s.CapsHeld++
			}
		}
		s.Solved += lp.Solved
		s.Playable += len(l.Playable)
	}
	return s
}

// world is the index of the world with division id, -1 for none.
func (a *Atlas) world(id string) int {
	return slices.IndexFunc(a.Worlds, func(w World) bool { return w.ID == id })
}
