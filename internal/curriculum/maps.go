package curriculum

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"katharsis/internal/catalog"
)

// A map is one module's problems as RTFM plays them (DESIGN.md "Learn
// mode"): a small graph, designed per module, in maps/<division>/<id>.map.
// READ opens its roots; a problem opens once every problem leading into it
// is solved; the module's trial opens once every core problem is solved;
// side problems are stretch and block nothing.
//
// A .map file is a line per chain, each with a note on the idea it carries:
//
//	core  usaco-738 > cses-1641 > usaco-643   # both ends, then fix one
//	side  cses-1641 > cf-702C                 # nearest tower, same sweep
//	skip  cf-1520E                            # binary search's, not this
//
// Every node of a core chain is core; the chain starts at a root or at a
// core node another chain placed. A side chain starts at its anchor, a node
// another chain placed, and the rest of it is side. Problems are the
// guide's uniqueIds. skip names a listed problem left off on purpose.
// Lines starting with # are comments.

//go:embed maps
var mapFiles embed.FS

// Map is a module's problem graph.
type Map struct {
	Module string
	// Nodes are parents first: a node's Parents index earlier nodes.
	Nodes []Node
	// Lanes are the core chains' notes, in file order; Node.Lane indexes
	// them.
	Lanes []string
	Skip  []string // guide uniqueIds
}

// Node is one problem of a map.
type Node struct {
	Problem string // guide uniqueId: "usaco-643"
	// ID is the guide entry's catalog ID (ProblemRef.ID), "" for a judge
	// the game lacks. ParseMap, which has no guide, guesses
	// CatalogID(Problem); MapFor takes the module's.
	ID   catalog.ID
	Core bool
	// Parents must all be solved before the node opens: indexes into
	// Map.Nodes. None means READ opens it.
	Parents []int
	// Lane is the core chain that placed the node, or for a side node its
	// anchor's lane; Note is the note of the chain that placed it.
	Lane int
	Note string
}

// Core is how many of m's nodes are core.
func (m Map) Core() int {
	n := 0
	for _, x := range m.Nodes {
		if x.Core {
			n++
		}
	}
	return n
}

// Playable is m without the nodes judgeable rejects (nil accepts every
// node with a catalog ID). A removed node's children inherit its parents,
// so the order of what stays is kept: a removed core node's children
// open when what led into it is solved.
func (m Map) Playable(judgeable func(catalog.ID) bool) Map {
	keep := func(n Node) bool { return n.ID != "" && (judgeable == nil || judgeable(n.ID)) }
	out := Map{Module: m.Module, Lanes: m.Lanes, Skip: m.Skip}
	at := make([]int, len(m.Nodes))    // old index -> new, -1 when removed
	via := make([][]int, len(m.Nodes)) // a removed node's parents, new indexes
	for i, n := range m.Nodes {
		var ps []int
		for _, p := range n.Parents {
			if at[p] >= 0 {
				ps = append(ps, at[p])
			} else {
				ps = append(ps, via[p]...)
			}
		}
		slices.Sort(ps)
		ps = slices.Compact(ps)
		if !keep(n) {
			at[i], via[i] = -1, ps
			continue
		}
		n.Parents = ps
		at[i] = len(out.Nodes)
		out.Nodes = append(out.Nodes, n)
	}
	return out
}

// MapFor is module m's map, ok false for a module with none: one that
// lists no problem the game can judge. Each node's ID is that of m's
// entry for it, so a misnamed uniqueId (refID) plays the problem the
// guide links; a node m does not list gets none.
func MapFor(m Module) (Map, bool) {
	mp, ok := embedded()[m.ID]
	if !ok {
		return Map{}, false
	}
	ids := map[string]catalog.ID{}
	for _, l := range m.Problems {
		for _, p := range l.Problems {
			ids[p.UniqueID] = p.ID
		}
	}
	mp.Nodes = slices.Clone(mp.Nodes)
	for i := range mp.Nodes {
		mp.Nodes[i].ID = ids[mp.Nodes[i].Problem]
	}
	return mp, true
}

var embedded = sync.OnceValue(func() map[string]Map {
	ms, err := ParseMaps(mapFiles)
	if err != nil {
		panic(err) // TestMaps parses every embedded map
	}
	return ms
})

// ParseMaps reads every maps/<division>/<id>.map file of fsys.
func ParseMaps(fsys fs.FS) (map[string]Map, error) {
	ms := map[string]Map{}
	err := fs.WalkDir(fsys, "maps", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".map" {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		id := strings.TrimSuffix(path.Base(p), ".map")
		m, err := ParseMap(id, string(data))
		if err != nil {
			return fmt.Errorf("curriculum: %s: %w", p, err)
		}
		if _, dup := ms[id]; dup {
			return fmt.Errorf("curriculum: %s: module %s has two maps", p, id)
		}
		ms[id] = m
		return nil
	})
	return ms, err
}

// ParseMap reads one module's .map text (see Map).
func ParseMap(module, text string) (Map, error) {
	type node struct {
		problem, note string
		core          bool
		lane          int
		parents       []string
		order         int
	}
	nodes := map[string]*node{}
	var order []string
	place := func(id string) *node {
		n, ok := nodes[id]
		if !ok {
			n = &node{problem: id, lane: -1, order: len(order)}
			nodes[id] = n
			order = append(order, id)
		}
		return n
	}
	addParent := func(n *node, p string) {
		if !slices.Contains(n.parents, p) {
			n.parents = append(n.parents, p)
		}
	}
	m := Map{Module: module}
	type anchor struct {
		id   string
		line int
	}
	var anchors []anchor
	sideOf := map[string]int{} // side node -> the line that placed it
	for i, line := range strings.Split(text, "\n") {
		ln := i + 1
		code, note, _ := strings.Cut(line, "#")
		code, note = strings.TrimSpace(code), strings.TrimSpace(note)
		if code == "" {
			continue
		}
		kind, rest, _ := strings.Cut(code, " ")
		var chain []string
		for _, id := range strings.Split(rest, ">") {
			if id = strings.TrimSpace(id); id == "" {
				return Map{}, fmt.Errorf("line %d: empty step in %q", ln, rest)
			}
			chain = append(chain, id)
		}
		switch kind {
		case "core":
			lane := len(m.Lanes)
			m.Lanes = append(m.Lanes, note)
			for j, id := range chain {
				if _, side := sideOf[id]; side {
					return Map{}, fmt.Errorf("line %d: %s is side and core", ln, id)
				}
				n := place(id)
				if !n.core {
					n.core, n.lane, n.note = true, lane, note
				}
				if j > 0 {
					addParent(n, chain[j-1])
				}
			}
		case "side":
			if len(chain) < 2 {
				return Map{}, fmt.Errorf("line %d: a side chain needs an anchor and a problem", ln)
			}
			anchors = append(anchors, anchor{chain[0], ln})
			for j, id := range chain[1:] {
				if n, ok := nodes[id]; ok && n.core {
					return Map{}, fmt.Errorf("line %d: %s is core and side", ln, id)
				}
				if at, ok := sideOf[id]; ok {
					return Map{}, fmt.Errorf("line %d: %s already follows a side anchor on line %d", ln, id, at)
				}
				sideOf[id] = ln
				n := place(id)
				n.note = note
				addParent(n, chain[j])
			}
		case "skip":
			if len(chain) != 1 {
				return Map{}, fmt.Errorf("line %d: skip names one problem", ln)
			}
			m.Skip = append(m.Skip, chain[0])
		default:
			return Map{}, fmt.Errorf("line %d: %q is not core, side or skip", ln, kind)
		}
	}
	for _, a := range anchors {
		if _, ok := nodes[a.id]; !ok {
			return Map{}, fmt.Errorf("line %d: anchor %s is placed by no chain", a.line, a.id)
		}
	}
	for _, id := range m.Skip {
		if _, ok := nodes[id]; ok {
			return Map{}, fmt.Errorf("%s is skipped and placed", id)
		}
	}

	// Parents first, ties in the order the file placed them; a side
	// node takes its anchor's lane.
	at := map[string]int{}
	for len(at) < len(order) {
		progress := false
		for _, id := range order {
			n := nodes[id]
			if _, done := at[id]; done || slices.ContainsFunc(n.parents, func(p string) bool { _, ok := at[p]; return !ok }) {
				continue
			}
			x := Node{Problem: id, ID: CatalogID(id), Core: n.core, Lane: n.lane, Note: n.note}
			for _, p := range n.parents {
				x.Parents = append(x.Parents, at[p])
			}
			slices.Sort(x.Parents)
			if !x.Core {
				x.Lane = m.Nodes[x.Parents[0]].Lane
			}
			at[id] = len(m.Nodes)
			m.Nodes = append(m.Nodes, x)
			progress = true
			break
		}
		if !progress {
			return Map{}, fmt.Errorf("the chains loop")
		}
	}
	return m, nil
}
