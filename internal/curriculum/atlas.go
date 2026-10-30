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
	// PageLevel is reading only: General's pages, and a module whose map
	// holds no problem the game can judge, with nothing to practise
	// before a trial. One that grants a capability has it earned in
	// another module on the topic (Atlas.EarnedIn).
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
	// appeared in contests (Frequency 0): its path is drawn dotted, and
	// the path goes on past it unpassed (Progress).
	Optional bool
	// Capability is what the module's trial grants, "" for none.
	Capability string
	// Topic is the catalog topic the module teaches, "" for none; Frequency
	// the guide's 0-4 "how often in contests", -1 if unset.
	Topic     string
	Frequency int
	// Prereqs are the module's prerequisites that are modules of the
	// guide; Warps the later-world modules this one opens a shortcut to:
	// those that skip a whole world and whose prerequisites all sit in
	// this one's world or before it (Super Mario World's Star Road).
	Prereqs, Warps []string
	// Playable are the distinct problems of the module the game can judge,
	// for General's examples.
	Playable []catalog.ID
	// Map is the module's map as the catalog can play it (MapFor,
	// Map.Playable): what RTFM practises, and what opens the trial.
	Map Map
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

// Before is the trial level whose pass opens module id's (Progress): the
// last one before it in guide order, across worlds, that is not optional;
// ok is false for the path's first and for levels off it (pages,
// General's).
func (a Atlas) Before(id string) (l Level, ok bool) {
	for _, w := range a.Worlds {
		if w.ID == generalID {
			continue
		}
		for _, x := range w.Levels() {
			switch {
			case !x.Kind.HasTrial():
				if x.Module == id {
					return Level{}, false
				}
			case x.Module == id:
				return l, ok
			case !x.Optional:
				l, ok = x, true
			}
		}
	}
	return Level{}, false
}

// Conclusion reports whether module id is a division's conclusion, a
// CastleLevel.
func Conclusion(id string) bool { return strings.HasSuffix(id, "-conclusion") }

// EarnedIn is where the capability of module id's level is earned when
// that level is a page: the next trial level in guide order granting it,
// else the last before. ok is false for a level with a trial of its own,
// none to earn, or no other module granting it.
func (a Atlas) EarnedIn(id string) (l Level, world int, ok bool) {
	at, _, found := a.Level(id)
	if !found || at.Capability == "" || at.Kind.HasTrial() {
		return Level{}, -1, false
	}
	after := false
	for i, w := range a.Worlds {
		for _, x := range w.Levels() {
			switch {
			case x.Module == id:
				after = true
			case x.Capability != at.Capability || !x.Kind.HasTrial():
			case after:
				return x, i, true
			default:
				l, world, ok = x, i, true
			}
		}
	}
	return l, world, ok
}

// generalID is the division of pages about contests in general: a library
// island, reading only.
const generalID = "general"

// Atlas lays the curriculum out as the map. judgeable reports whether the
// catalog can serve and judge an ID; nil accepts every reference CatalogID
// maps (USACO, CSES, Codeforces). The guide's own pages (Module.Meta) are
// left out, as the index leaves them.
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
		Topic: m.Topic, Frequency: m.Frequency,
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
	mp, _ := MapFor(m)
	l.Map = mp.Playable(judgeable)
	l.Kind = levelKind(m.ID, division, len(l.Map.Nodes) > 0)
	return l
}

// levelKind is the kind of module id's level in division, given whether
// its map holds a problem the game can judge: a trial needs one, or there
// would be nothing to practise before it (Bronze's Introduction to Data
// Structures lists none). A gated module without one is a page, its
// capability earned in another module on the topic (Atlas.EarnedIn). A
// conclusion is a castle even with none: the division's gauntlet.
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
	// Core and Side count its map's problems of each kind, CoreSolved and
	// SideSolved those solved anywhere. The trial opens once CoreSolved
	// reaches Core.
	Core, CoreSolved int
	Side, SideSolved int
	// Open is whether the player may play it (Atlas.Progress, the path).
	Open bool
}

// started reports whether the player made progress on the level that a
// locked path must not take away: a solve on its map, or its trial passed.
func (p LevelProgress) started() bool { return p.Passed || p.CoreSolved+p.SideSolved > 0 }

// Progress is the atlas against what the player has done; it is derived,
// never stored.
type Progress map[string]LevelProgress

// Progress reads m.
//
// The path is fixed, as the USACO training pages are (DESIGN.md "Learn
// mode"): a trial level opens once the trial before it in guide order is
// passed, a world's first once the world before it is, through its
// castle. An optional level (Level.Optional) opens in its turn but holds
// nothing up: the guide says it can wait. Pages and General, the library,
// are open and gate nothing: opening a page says nothing of reading it
// (plan Q4), so it is no checkpoint. A level the player already made
// progress on stays open whatever comes before it, so a save from before
// the path was fixed loses nothing.
func (a Atlas) Progress(m *game.Meta) Progress {
	solved := game.Solved(m)
	p := Progress{}
	prev := true // the first level of the path is open
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
			for _, n := range l.Map.Nodes {
				switch {
				case n.Core:
					lp.Core++
					if solved[n.ID] {
						lp.CoreSolved++
					}
				default:
					lp.Side++
					if solved[n.ID] {
						lp.SideSolved++
					}
				}
			}
			switch {
			case w.ID == generalID || !l.Kind.HasTrial():
				lp.Open = true
			case l.Optional:
				lp.Open = prev || lp.started()
			default:
				lp.Open = prev || lp.started()
				prev = lp.Passed
			}
			p[l.Module] = lp
		}
	}
	return p
}

// WorldSum is a world's counts, each with its real denominator: trials
// passed of the levels that have one, capabilities held of those it
// teaches, problems solved of those on its trial levels' maps, pages
// opened of all its levels.
type WorldSum struct {
	Passed, Trials   int
	CapsHeld, Caps   int
	Solved, Playable int
	Opened, Levels   int
}

// Sum counts w under p.
func (p Progress) Sum(w World) WorldSum {
	var s WorldSum
	caps := map[string]bool{}
	for _, l := range w.Levels() {
		s.Levels++
		lp := p[l.Module]
		if lp.Opened {
			s.Opened++
		}
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
		// A page's problems count nowhere: General's are the pages' examples
		// (Fast I/O cites a Platinum problem), not practice, and any other
		// page has none the game judges.
		if l.Kind.HasTrial() {
			s.Solved += lp.CoreSolved + lp.SideSolved
			s.Playable += lp.Core + lp.Side
		}
	}
	return s
}

// world is the index of the world with division id, -1 for none.
func (a *Atlas) world(id string) int {
	return slices.IndexFunc(a.Worlds, func(w World) bool { return w.ID == id })
}
