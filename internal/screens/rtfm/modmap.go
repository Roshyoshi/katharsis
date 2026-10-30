package rtfm

import (
	"katharsis/internal/curriculum"
	"katharsis/internal/ui"
)

// A module map (curriculum.Map) on screen: READ in column 0, each problem
// in the column of its depth, the longest chain of parents back to READ,
// and the trial one column past the deepest core problem. Each core lane
// gets a row, in the file's order, and its side problems the rows under
// it, a side chain along one row while it fits:
//
//	▪ READ ┬ ✓ Paired Up ────────────┬ ○ Diamond Collector ───── ⚔ TRIAL
//	       │                         ├ ☆ Cow Checkups
//	       ╰ ● Sleepy Cow Herding ───╯
//
// An edge runs along its parent's row to the gutter before the child's
// column, then up or down to the child's row; where that row is taken, it
// turns at the parent's own gutter instead and runs along the child's row.
// Paths into one column share its gutter, so they merge: above, ├ is both
// Paired Up's path into Cow Checkups and Sleepy Cow Herding's on up.

// mapItem indexes a map's items: node i of the map is i, READ and the trial
// are these.
const (
	readItem  = -1
	trialItem = -2
)

// mapLayout is where a map's items sit on the grid and how its edges run.
type mapLayout struct {
	cols, rows int
	at         [][2]int // node i's row and column
	trial      [2]int   // the trial's cell, when the map has one
	hasTrial   bool
	edges      []mapEdge
}

type mapEdge struct {
	from, to int  // items: a node index, readItem or trialItem
	drop     bool // turn at the parent and run along the child's row
}

// cell is item i's row and column.
func (l *mapLayout) cell(i int) [2]int {
	switch i {
	case readItem:
		return [2]int{0, 0}
	case trialItem:
		return l.trial
	}
	return l.at[i]
}

// layoutMap places m's items; trial says whether the level has a trial to
// end the map in.
func layoutMap(m curriculum.Map, trial bool) mapLayout {
	n := len(m.Nodes)
	l := mapLayout{at: make([][2]int, n), hasTrial: trial}
	depth := make([]int, n)
	coreDepth := 0
	for i, x := range m.Nodes {
		d := 1
		for _, p := range x.Parents {
			d = max(d, depth[p]+1)
		}
		depth[i] = d
		if x.Core {
			coreDepth = max(coreDepth, d)
		}
	}

	var taken []map[int]bool // row -> columns taken
	newRow := func() int {
		taken = append(taken, map[int]bool{})
		return len(taken) - 1
	}
	put := func(i, r int) {
		l.at[i] = [2]int{r, depth[i]}
		taken[r][depth[i]] = true
	}
	lanes := 0
	for _, x := range m.Nodes {
		lanes = max(lanes, x.Lane+1)
	}
	for lane := range lanes {
		var core, side []int
		for i, x := range m.Nodes {
			switch {
			case x.Lane != lane:
			case x.Core:
				core = append(core, i)
			default:
				side = append(side, i)
			}
		}
		if len(core)+len(side) == 0 {
			continue
		}
		block := newRow()
		for _, i := range core {
			put(i, block)
		}
		for _, i := range side {
			p := m.Nodes[i].Parents[0] // a side problem has one
			if r := l.at[p][0]; !m.Nodes[p].Core && !taken[r][depth[i]] {
				put(i, r) // the chain goes on along its row
				continue
			}
			r := -1
			for rr := block + 1; rr < len(taken); rr++ {
				if !taken[rr][depth[i]] {
					r = rr
					break
				}
			}
			if r < 0 {
				r = newRow()
			}
			put(i, r)
		}
	}
	if len(taken) == 0 {
		newRow()
	}
	l.rows = len(taken)
	l.cols = coreDepth + 1
	for _, d := range depth {
		l.cols = max(l.cols, d+1)
	}
	if trial {
		l.trial = [2]int{0, coreDepth + 1}
		l.cols = max(l.cols, coreDepth+2)
	}

	// Edges: READ to the roots, parent to child, the core's ends to the
	// trial (READ's own when there is no core).
	hasCoreChild := make([]bool, n)
	for i, x := range m.Nodes {
		if len(x.Parents) == 0 {
			l.edges = append(l.edges, mapEdge{from: readItem, to: i})
		}
		for _, p := range x.Parents {
			l.edges = append(l.edges, mapEdge{from: p, to: i})
			if x.Core {
				hasCoreChild[p] = true
			}
		}
	}
	if trial {
		ends := 0
		for i, x := range m.Nodes {
			if x.Core && !hasCoreChild[i] {
				l.edges = append(l.edges, mapEdge{from: i, to: trialItem})
				ends++
			}
		}
		if ends == 0 {
			l.edges = append(l.edges, mapEdge{from: readItem, to: trialItem})
		}
	}
	for k, e := range l.edges {
		if l.blocked(e, false) && !l.blocked(e, true) {
			l.edges[k].drop = true
		}
	}
	return l
}

// blocked reports whether edge e's run along a row, the parent's or with
// drop the child's, crosses another item's cell.
func (l *mapLayout) blocked(e mapEdge, drop bool) bool {
	a, b := l.cell(e.from), l.cell(e.to)
	row := a[0]
	if drop {
		row = b[0]
	}
	for c := a[1] + 1; c < b[1]; c++ {
		if l.taken(row, c) {
			return true
		}
	}
	return false
}

// taken reports whether an item sits at row r, column c.
func (l *mapLayout) taken(r, c int) bool {
	if r == 0 && c == 0 || l.hasTrial && l.trial == [2]int{r, c} {
		return true
	}
	for _, at := range l.at {
		if at == [2]int{r, c} {
			return true
		}
	}
	return false
}

// ---- drawing ----------------------------------------------------------------

// draw adds edge e's path to lk, the parent's label lw cells wide at its
// column's x: along the parent's row from just past the label to the
// cell before the child's column, round to the child's row, and into the
// child. A dropped edge turns at the parent's own gutter instead and runs
// along the child's row.
func (l *mapLayout) draw(lk *links, xs []int, e mapEdge, lw int, lit bool) {
	a, b := l.cell(e.from), l.cell(e.to)
	sx, t := xs[a[1]]+lw+1, xs[b[1]]-2
	turn := t
	if e.drop {
		turn = xs[a[1]+1] - 2
	}
	for x := sx; x <= turn; x++ { // linked back to the label
		m := uint8(linkL)
		if x < turn {
			m |= linkR
		}
		lk.add(a[0], x, m, lit)
	}
	if a[0] != b[0] {
		lk.fall(turn, a[0], b[0], lit)
	}
	if turn < t {
		lk.run(b[0], turn, t, lit)
	}
	lk.add(b[0], t, linkR, lit)
}

// Connections out of a cell, which pick its box-drawing glyph.
const (
	linkL = 1 << iota
	linkR
	linkU
	linkD
)

// links is an edge layer: each cell's connections, and whether an edge
// that passes it is lit.
type links struct {
	mask [][]uint8
	lit  [][]bool
}

func newLinks(w, h int) *links {
	l := &links{mask: make([][]uint8, h), lit: make([][]bool, h)}
	for y := range h {
		l.mask[y], l.lit[y] = make([]uint8, w), make([]bool, w)
	}
	return l
}

func (l *links) add(y, x int, m uint8, lit bool) {
	if y < 0 || y >= len(l.mask) || x < 0 || x >= len(l.mask[y]) {
		return
	}
	l.mask[y][x] |= m
	l.lit[y][x] = l.lit[y][x] || lit
}

// run joins cells (y, a) to (y, b) along a row.
func (l *links) run(y, a, b int, lit bool) {
	if a > b {
		a, b = b, a
	}
	for x := a; x <= b; x++ {
		var m uint8
		if x > a {
			m |= linkL
		}
		if x < b {
			m |= linkR
		}
		l.add(y, x, m, lit)
	}
}

// fall joins cells (a, x) to (b, x) down a column.
func (l *links) fall(x, a, b int, lit bool) {
	if a > b {
		a, b = b, a
	}
	for y := a; y <= b; y++ {
		var m uint8
		if y > a {
			m |= linkU
		}
		if y < b {
			m |= linkD
		}
		l.add(y, x, m, lit)
	}
}

// glyph is the box-drawing cell for mask m, "" for none.
func linkGlyph(m uint8) string {
	t := ui.G.Thin
	switch m {
	case 0:
		return ""
	case linkL, linkR, linkL | linkR:
		return t.Top
	case linkU, linkD, linkU | linkD:
		return t.Left
	case linkR | linkD:
		return t.TopLeft
	case linkL | linkD:
		return t.TopRight
	case linkR | linkU:
		return t.BottomLeft
	case linkL | linkU:
		return t.BottomRight
	case linkL | linkR | linkD:
		return t.MiddleTop
	case linkL | linkR | linkU:
		return t.MiddleBottom
	case linkU | linkD | linkR:
		return t.MiddleLeft
	case linkU | linkD | linkL:
		return t.MiddleRight
	}
	return t.Middle
}
