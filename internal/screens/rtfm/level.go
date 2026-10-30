package rtfm

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Level is what opens on a node of the RTFM map: the module's own map
// (curriculum.Map, laid out by layoutMap), its problems on paths from
// READ to the TRIAL. A problem opens once every problem with a path into
// it is solved, and the trial once every core one is; the stretch
// problems (☆) hang off the core, and nothing waits on them:
//
//	▪ READ ┬ ✓ Paired Up ────────────┬ ○ Diamond Collector ───── ⚔ TRIAL
//	       │                         ├ ☆ Cow Checkups
//	       ╰ ● Sleepy Cow Herding ───╯
//
// Under the map, a card on the item under the cursor: what it is, why it
// is open or locked, what it leads to. Then the later modules the level
// leads on to (its warps), SEE ALSO and the module's CC BY-NC-SA
// attribution.
//
// The arrows move across the map. Enter acts on the item under the
// cursor: READ reads the module, an open or solved problem is a practice
// door on its clock (screens.Upsolve, game.PracticeClock), the trial
// starts (screens.StartTrial, which the
// app refuses while the core is unsolved). A locked problem does nothing,
// not even o: its statement waits with it. Only a level the path has
// reached (curriculum.Atlas.Progress) is shown at all; Map keeps the rest
// shut. Leaving is the host's: it takes q, esc and backspace before they
// reach the level, as Browser does for its page.
type Level struct {
	l      curriculum.Level
	region string
	e      entry
	page   *Module // the module page, for its pass line
	caps   map[string]bool
	opened bool
	passed bool     // its trial
	prereq []string // prerequisites as display names, ✓ when passed
	warps  string   // the later modules it leads on to, "" for none
	nodes  []node
	lay    mapLayout

	cur       int // the item under the cursor: a node, readItem or trialItem
	top, left int // the map's first row and column shown, when it does not fit
}

// node is a problem of the level's map as the screen shows it.
type node struct {
	curriculum.Node
	title, source, label, contest, url string
	rating                             int // 0 when not shown (catalog.ShowsRating)
	solved                             bool
	open                               bool // every parent solved
	clock                              game.PracticeClock
	edLeft                             time.Duration // game.EditorialLeft
}

// NewLevel builds the level screen for l, a level of c's atlas in the
// region titled region that the path has reached
// (curriculum.LevelProgress.Open).
func NewLevel(c *curriculum.Curriculum, l curriculum.Level, region string, m game.Meta, judgeable Judgeable, lookup screens.Lookup) *Level {
	mod, ok := curriculum.Module{}, false
	if c != nil {
		mod, ok = c.Module(l.Module)
	}
	if !ok {
		mod = curriculum.Module{ID: l.Module, Title: l.Title, Frequency: -1}
	}
	caps := capSet(m)
	e := newEntry(mod, judgeable, lookup)
	passed := slices.Contains(m.TrialsPassed, l.Module)
	v := &Level{
		l: l, region: region, e: e, page: newModule(c, e, caps, passed), caps: caps,
		opened: slices.Contains(m.Opened, l.Module), passed: passed,
		cur: readItem,
	}
	for _, id := range l.Prereqs {
		name := id
		if c != nil {
			if pm, ok := c.Module(id); ok {
				name = pm.Title + " (" + pm.DivisionTitle + ")"
			}
		}
		if slices.Contains(m.TrialsPassed, id) {
			name += " " + ui.OK.Render(ui.G.Pass)
		}
		v.prereq = append(v.prereq, name)
	}
	v.warps = warpText(c, l.Warps)
	v.nodes = mapNodes(mod, l.Map, game.Solved(&m), lookup)
	for i, n := range v.nodes {
		v.nodes[i].clock, v.nodes[i].edLeft = game.Practice(&m, n.ID), game.EditorialLeft(&m, n.ID)
	}
	v.lay = layoutMap(l.Map, v.hasTrial())
	return v
}

// warpText names the modules ids, a level's warps, grouped by
// division: "Eulerian Tours, String Searching (Advanced)".
func warpText(c *curriculum.Curriculum, ids []string) string {
	var divs []string
	names := map[string][]string{}
	for _, id := range ids {
		name, div := id, ""
		if c != nil {
			if wm, ok := c.Module(id); ok {
				name, div = wm.Title, wm.DivisionTitle
			}
		}
		if _, ok := names[div]; !ok {
			divs = append(divs, div)
		}
		names[div] = append(names[div], name)
	}
	var parts []string
	for _, d := range divs {
		s := strings.Join(names[d], ", ")
		if d != "" {
			s += " (" + d + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ui.G.Sep)
}

// mapNodes is mp's nodes with what the guide's entry and the catalog say
// of each, and whether solved names it.
func mapNodes(mod curriculum.Module, mp curriculum.Map, solved map[catalog.ID]bool, lookup screens.Lookup) []node {
	refs := map[string]curriculum.ProblemRef{}
	for _, l := range mod.Problems {
		for _, r := range l.Problems {
			if _, dup := refs[r.UniqueID]; !dup {
				refs[r.UniqueID] = r
			}
		}
	}
	out := make([]node, len(mp.Nodes))
	for i, n := range mp.Nodes {
		r := refs[n.Problem]
		x := node{Node: n, title: cmp.Or(r.Name, n.Problem), source: r.Source, label: r.Difficulty,
			url: r.URL, solved: solved[n.ID]}
		if lookup != nil {
			if p, ok := lookup(n.ID); ok {
				x.title, x.contest = cmp.Or(p.Title, x.title), p.Contest
				if catalog.ShowsRating(n.ID) {
					x.rating = p.Rating
				}
			}
		}
		out[i] = x
	}
	for i := range out {
		out[i].open = !slices.ContainsFunc(out[i].Parents, func(p int) bool { return !out[p].solved })
	}
	return out
}

// hasTrial reports whether the map ends in a trial: trial levels and
// castles do, a page does not.
func (v *Level) hasTrial() bool { return v.l.Kind != curriculum.PageLevel }

// tally counts the map's core and stretch problems and those solved.
func (v *Level) tally() (core, coreSolved, side, sideSolved int) {
	for _, n := range v.nodes {
		switch {
		case n.Core:
			core++
			if n.solved {
				coreSolved++
			}
		default:
			side++
			if n.solved {
				sideSolved++
			}
		}
	}
	return core, coreSolved, side, sideSolved
}

// trialOpen reports whether the trial may start: every core problem is
// solved.
func (v *Level) trialOpen() bool {
	core, solved, _, _ := v.tally()
	return solved == core
}

// playable reports whether node i may be played: it is open. A solved one
// stays playable, to try again.
func (v *Level) playable(i int) bool { return v.nodes[i].open || v.nodes[i].solved }

// items is every item of the map, READ first.
func (v *Level) items() []int {
	out := []int{readItem}
	for i := range v.nodes {
		out = append(out, i)
	}
	if v.hasTrial() {
		out = append(out, trialItem)
	}
	return out
}

// Practice puts the cursor on what to do next: the first open core
// problem not solved, then the trial while it waits to be passed, then
// the first open stretch problem; READ once nothing is left.
func (v *Level) Practice() { v.cur = v.next() }

func (v *Level) next() int {
	todo := func(core bool) int {
		return slices.IndexFunc(v.nodes, func(n node) bool { return n.Core == core && n.open && !n.solved })
	}
	switch {
	case todo(true) >= 0:
		return todo(true)
	case v.hasTrial() && !v.passed:
		return trialItem
	case todo(false) >= 0:
		return todo(false)
	}
	return readItem
}

// move is an arrow: to the nearest item that way. Across (←→) is deeper
// or shallower on the map, the nearest column first, then the nearest
// row; up and down (↑↓) change lane, the nearest column first again, so
// the cursor keeps to its depth while it can.
func (v *Level) move(dr, dc int) {
	at := v.lay.cell(v.cur)
	best, bestKey := v.cur, [3]int{}
	for _, i := range v.items() {
		c := v.lay.cell(i)
		dRow, dCol := c[0]-at[0], c[1]-at[1]
		var key [3]int
		switch {
		case dc != 0 && dCol*dc > 0:
			key = [3]int{dCol * dc, abs(dRow), c[0]}
		case dr != 0 && dRow*dr > 0:
			key = [3]int{abs(dCol), dRow * dr, c[1]}
		default:
			continue
		}
		if best == v.cur || slices.Compare(key[:], bestKey[:]) < 0 {
			best, bestKey = i, key
		}
	}
	v.cur = best
}

func abs(x int) int { return max(x, -x) }

func (v *Level) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return v, nil
	}
	id := v.l.Module
	switch k.String() {
	case "up", "k":
		v.move(-1, 0)
	case "down", "j":
		v.move(1, 0)
	case "left", "h":
		v.move(0, -1)
	case "right", "l":
		v.move(0, 1)
	case "home", "g":
		v.cur = readItem
	case "end", "G":
		if v.hasTrial() {
			v.cur = trialItem
		}
	case "n", "p":
		v.Practice()
	case "r":
		return v, screens.Emit(screens.OpenModule{Module: id})
	case "t":
		if v.hasTrial() {
			return v, screens.Emit(screens.StartTrial{Module: id})
		}
	case "o":
		u := v.e.mod.URL
		if v.cur >= 0 {
			if !v.playable(v.cur) {
				break // a locked problem's statement waits with it
			}
			u = cmp.Or(v.nodes[v.cur].url, u)
		}
		if u != "" {
			return v, screens.Emit(reader.OpenURL{URL: u})
		}
	case "enter":
		return v, v.act()
	}
	return v, nil
}

// act is enter on the item under the cursor. A locked problem does
// nothing: its card says what it waits on.
func (v *Level) act() tea.Cmd {
	switch {
	case v.cur == readItem:
		return screens.Emit(screens.OpenModule{Module: v.l.Module})
	case v.cur == trialItem:
		return screens.Emit(screens.StartTrial{Module: v.l.Module})
	case v.playable(v.cur):
		return screens.Emit(screens.Upsolve{Problem: v.nodes[v.cur].ID})
	}
	return nil
}

func (v *Level) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	m := v.e.mod
	crumb := strings.Join([]string{ui.Strong.Render("RTFM"), m.DivisionTitle, v.region, v.l.Title},
		ui.Muted.Render(ui.G.Crumb))
	head := []string{ui.Spread(crumb, capCell(v.l.Capability, v.caps[v.l.Capability]), w), ui.Rule(w), v.nameLine()}
	var desc []string
	for _, l := range ui.WrapN(m.Description, max(w-2, 10), 2) {
		desc = append(desc, " "+l)
	}
	card, foot := v.card(w), v.footLines(w)

	// The map gets the rows the head, the card and the foot leave, with a
	// blank line on each side when they fit; then the description, and
	// the blank lines that keep the foot at the bottom.
	rows := max(h-1, 0) // the status bar
	room := rows - len(head) - len(card) - len(foot)
	gap := room >= min(v.lay.rows, 3)+2
	if gap {
		room -= 2
	}
	n := max(min(v.lay.rows, room), 1)
	spare := room - n

	body := slices.Clone(head)
	if len(desc) > 0 && spare >= len(desc)+1 {
		body = append(body, desc...)
		spare -= len(desc)
	}
	if gap {
		body = append(body, "")
	}
	body = append(body, v.mapLines(w, n)...)
	if gap {
		body = append(body, "")
	}
	body = append(body, card...)
	for ; spare > 0; spare-- {
		body = append(body, "")
	}
	body = append(body, foot...)
	return ui.Layout(w, h, body, v.statusBar(w))
}

// nameLine is the level's title, the guide's frequency stars and how far
// the map is solved: "core 2/4 · ★ 0/2".
func (v *Level) nameLine() string {
	name := " " + ui.Strong.Render(v.l.Title)
	if v.l.Kind == curriculum.CastleLevel {
		// A conclusion is the world's gauntlet: the boss door's glyph.
		name = " " + ui.Special.Render(ui.G.Boss) + name
	}
	if f := v.e.mod.Frequency; f >= 0 {
		name += "  " + stars(f) + " " + ui.Muted.Render(frequencyText(f))
	}
	core, coreSolved, side, sideSolved := v.tally()
	if core > 0 {
		name += ui.Muted.Render(ui.G.Sep) + fmt.Sprintf("core %d/%d", coreSolved, core)
	}
	if side > 0 {
		name += ui.Muted.Render(ui.G.Sep) + ui.Warn.Render(ui.G.Star) + fmt.Sprintf(" %d/%d", sideSolved, side)
	}
	return name
}

// ---- the map ----------------------------------------------------------------

// Column widths. Each column's items start at its x; after an item's label
// come a space, the cell where paths turn into the next column, and the
// space before the next item, where the cursor sits.
const (
	gutter  = 3
	minCol  = 12 // the narrowest a problem's column gets before the map pans
	maxCol  = 26
	padLeft = 1 // READ's cursor
)

// mapCols is each column's x and width in cells: READ's column fits READ,
// the trial's fits the trial when it holds nothing else, and the rest
// share what w leaves, between minCol and maxCol.
func (v *Level) mapCols(w int) (xs, cw []int) {
	cols := v.lay.cols
	cw = make([]int, cols)
	cw[0] = ansi.StringWidth(readLabel()) + gutter
	share, rest := w-padLeft-cw[0], 0
	for c := 1; c < cols; c++ {
		if v.lay.hasTrial && c == v.lay.trial[1] && !v.colHolds(c) {
			cw[c] = ansi.StringWidth(trialLabel()) + 1
			share -= cw[c]
			continue
		}
		rest++
	}
	each := maxCol
	if rest > 0 {
		each = min(max(share/rest, minCol), maxCol)
	}
	xs = make([]int, cols)
	x := padLeft
	for c := range cols {
		if cw[c] == 0 {
			cw[c] = each
		}
		xs[c] = x
		x += cw[c]
	}
	return xs, cw
}

// colHolds reports whether a problem sits in column c.
func (v *Level) colHolds(c int) bool {
	return slices.ContainsFunc(v.lay.at, func(at [2]int) bool { return at[1] == c })
}

func readLabel() string  { return ui.G.Page + " READ" }
func trialLabel() string { return ui.G.Boss + " TRIAL" }

// label is item i drawn in w cells: its glyph, the state's colour, and
// its title cut to fit.
func (v *Level) label(i, w int) string {
	g, tok, text := v.glyph(i)
	title := ui.Clip(text, max(w-2, 1))
	switch {
	case i == v.cur:
		title = ui.Focus.Render(title)
	case tok == ui.Muted:
		title = ui.Muted.Render(title)
	}
	return tok.Render(g) + " " + title
}

// glyph is item i's mark and colour, and its title: ▪ READ; ✓ a solved
// core problem, ● one open, ○ one locked; ★ a solved stretch problem, ☆
// one not; ⚔ the trial, faint until the core is solved, ✓ once passed.
// Anything locked is faint.
func (v *Level) glyph(i int) (string, ui.Token, string) {
	switch i {
	case readItem:
		return ui.G.Page, ui.FG, "READ"
	case trialItem:
		switch {
		case v.passed:
			return ui.G.Pass, ui.OK, "TRIAL"
		case v.trialOpen():
			return ui.G.Boss, ui.Special, "TRIAL"
		}
		return ui.G.Boss, ui.Muted, "TRIAL"
	}
	n := v.nodes[i]
	tok := ui.Muted
	if v.playable(i) {
		tok = ui.FG
	}
	switch {
	case n.Core && n.solved:
		return ui.G.Pass, ui.OK, n.title
	case n.Core && tok == ui.FG:
		return ui.G.Here, ui.FG, n.title
	case n.Core:
		return ui.G.Ahead, ui.Muted, n.title
	case n.solved:
		return ui.G.Star, ui.Warn, n.title
	}
	return ui.G.StarOff, tok, n.title
}

// mapLines draws the map in n lines w cells wide: the items in their
// columns, the paths between them, lit where they leave something solved
// (READ's always), and the cursor before the item under it. A map larger
// than that pans to keep the cursor in view.
func (v *Level) mapLines(w, n int) []string {
	xs, cw := v.mapCols(w)
	full := xs[len(xs)-1] + cw[len(cw)-1]
	labels := map[int]string{}
	for _, i := range v.items() {
		// Before the gutter, but for the trial, which no path leaves.
		room := cw[v.lay.cell(i)[1]] - gutter
		if i == trialItem {
			room = max(room, ansi.StringWidth(trialLabel()))
		}
		labels[i] = v.label(i, room)
	}
	lk := newLinks(full, v.lay.rows)
	for _, e := range v.lay.edges {
		lit := e.from == readItem || e.from >= 0 && v.nodes[e.from].solved
		v.lay.draw(lk, xs, e, ansi.StringWidth(labels[e.from]), lit)
	}
	c := newCanvas(full, v.lay.rows)
	for y, row := range lk.mask {
		for x, m := range row {
			if g := linkGlyph(m); g != "" {
				tok := ui.Muted
				if lk.lit[y][x] {
					tok = ui.FG
				}
				c.put(y, x, tok.Render(g))
			}
		}
	}
	for i, l := range labels {
		at := v.lay.cell(i)
		c.put(at[0], xs[at[1]], l)
	}
	at := v.lay.cell(v.cur)
	c.put(at[0], xs[at[1]]-1, ui.Focus.Render(ui.G.Cursor))

	// Pan: the cursor's column and row in view, as little moved as can be.
	v.left = min(v.left, at[1])
	if full <= w {
		v.left = 0
	}
	for v.left < at[1] && xs[at[1]]+cw[at[1]]-(xs[v.left]-padLeft) > w {
		v.left++
	}
	v.top = max(min(v.top, at[0]), at[0]-n+1)
	v.top = max(min(v.top, v.lay.rows-n), 0)
	x0 := xs[v.left] - padLeft
	var out []string
	for y := v.top; y < min(v.top+n, v.lay.rows); y++ {
		cells := c.cells[y][x0:min(x0+w, full)]
		out = append(out, strings.TrimRight(strings.Join(cells, ""), " "))
	}
	return out
}

// ---- the card ---------------------------------------------------------------

// card is the facts of the item under the cursor, a line each: what it
// is, where it stands, and the note of the chain that placed it.
func (v *Level) card(w int) []string {
	ind := strings.Repeat(" ", gutter)
	g, tok, _ := v.glyph(v.cur)
	mark := " " + tok.Render(g) + " "
	switch v.cur {
	case readItem:
		out := []string{ui.Spread(mark+ui.Strong.Render("READ")+"  the module, in the reader", v.openedWord(), w)}
		if len(v.nodes) > 0 {
			out = append(out, ind+ui.Muted.Render("nothing waits on it: the first problems are open from the start"))
		}
		if !v.hasTrial() {
			out = append(out, ind+v.page.passLine())
		}
		return out
	case trialItem:
		t := v.e.trial
		what := mark + ui.Strong.Render("TRIAL") + "  " + units.Plural(curriculum.Doors, "unseen door") + " on " +
			v.e.biome + ui.Muted.Render(ui.G.Sep+"blind"+ui.G.Sep) + fmt.Sprintf("pass %d of %d", trialPass, curriculum.Doors)
		aimed := ui.Muted.Render("spread over ") + ui.RatingBand(t.Band[0], t.Band[1])
		if a := aims(t); a != "" {
			aimed = ui.Muted.Render("aimed ") + a + ui.Muted.Render(": the core's middle, its hardest, a step past")
		}
		core, solved, _, _ := v.tally()
		var state string
		switch {
		case v.passed:
			state = ui.OK.Render(ui.G.Pass+" passed") + ui.Muted.Render(": t plays it again")
		case solved < core:
			state = fmt.Sprintf("opens once the core is solved: %d of %d so far", solved, core)
		default:
			state = "open" + ui.Muted.Render(": enter or t starts it")
		}
		return []string{what, ind + aimed, ind + state, ind + v.page.passLine()}
	}

	n := v.nodes[v.cur]
	var facts []string
	for _, f := range []string{n.source, n.label} {
		if f != "" {
			facts = append(facts, f)
		}
	}
	right := ui.Muted.Render(strings.Join(facts, ui.G.Sep))
	if n.rating > 0 {
		right += " " + ui.Rating(n.rating)
	}
	kind := "core"
	if !n.Core {
		kind = "stretch"
	}
	var state string
	switch {
	case n.solved:
		state = ui.OK.Render(ui.G.Pass+" solved") + ui.Muted.Render(": enter plays it again")
	case n.open && n.clock.Spent > 0:
		state = "open" + ui.Muted.Render(": enter goes on")
	case n.open:
		state = "open" + ui.Muted.Render(": enter starts the clock")
	default:
		state = "locked: solve " + v.titles(slices.DeleteFunc(slices.Clone(n.Parents), func(p int) bool { return v.nodes[p].solved })) + " first"
	}
	if to := v.leadsTo(v.cur); to != "" {
		state += ui.Muted.Render(ui.G.Sep+"leads to ") + to
	}
	out := []string{ui.Spread(mark+ui.Strong.Render(n.title), right, w), ind + kind + ui.Muted.Render(ui.G.Sep) + state}
	if c := clockLine(n); c != "" {
		out = append(out, ind+c)
	}
	if n.Note != "" {
		out = append(out, ind+ui.Muted.Render(n.Note))
	}
	return out
}

// clockLine is problem n's clock on its card (ClockFace) and what it
// says: how long the editorial still waits, or where the clock stopped;
// "" for a locked problem whose clock never ran, and for one solved
// before its clock ran.
func clockLine(n node) string {
	face := ClockFace(n.clock, n.solved, ui.Token.Render)
	var say string
	switch {
	case face == "", !n.open && !n.solved && n.clock.Spent == 0:
		return ""
	case n.solved && n.clock.Editorial > 0:
		say = ": the editorial at the red, then the yellow to the AC"
	case n.solved:
		say = " to the AC, no editorial"
	case n.clock.Editorial > 0:
		say = ": stopped at the editorial; the yellow runs to the AC"
	case n.edLeft > 0:
		say = ui.G.Sep + "editorial in " + units.Countdown(n.edLeft)
	default:
		say = ui.G.Sep + "editorial open: e in the door shows it, and stops the clock"
	}
	return face + ui.Muted.Render(say)
}

// titles names nodes is, comma-separated.
func (v *Level) titles(is []int) string {
	var out []string
	for _, i := range is {
		out = append(out, v.nodes[i].title)
	}
	return strings.Join(out, ", ")
}

// leadsTo names what node i opens the way to: its children, and the trial
// for a core problem no other core problem follows.
func (v *Level) leadsTo(i int) string {
	var kids []int
	core := false
	for j, n := range v.nodes {
		if slices.Contains(n.Parents, i) {
			kids = append(kids, j)
			core = core || n.Core
		}
	}
	out := v.titles(kids)
	if v.nodes[i].Core && !core && v.hasTrial() {
		out = strings.Join(slices.DeleteFunc([]string{out, "the trial"}, func(s string) bool { return s == "" }), ", ")
	}
	return out
}

// openedWord is READ's state: "opened" once the page was opened in the
// reader, else nothing (no "not read yet": plan GR8, GR10).
func (v *Level) openedWord() string {
	if v.opened {
		return ui.Muted.Render("opened")
	}
	return ""
}

// footLines is the level's foot: the modules it leads on to, SEE ALSO (its
// prerequisites, ✓ for each whose trial is passed) and the module's
// attribution, which CC BY-NC-SA requires, as the module page prints it.
func (v *Level) footLines(w int) []string {
	text := min(w, 80)
	var out []string
	if v.warps != "" {
		out = append(out, ui.Clip(" "+ui.G.Warp+" leads on to: "+v.warps, w))
	}
	req := "nothing"
	if len(v.prereq) > 0 {
		req = strings.Join(v.prereq, ui.Muted.Render(ui.G.Sep))
	}
	const labelW = 10
	see := wrap(ui.Muted.Render("needs ")+req, 1+labelW, w)
	see[0] = " " + ui.Fit(ui.Strong.Render("SEE ALSO"), labelW) + strings.TrimLeft(see[0], " ")
	out = append(out, see...)
	for _, l := range wrap(v.e.mod.Attribution(), 0, text) {
		out = append(out, ui.Muted.Render(l))
	}
	return out
}

// statusBar is the pager's bar, as on the module page: the level on the
// left, its keys on the right.
func (v *Level) statusBar(w int) string {
	keys := []string{"hjkl", "move", "enter", "go", "n", "next", "r", "read"}
	if v.hasTrial() {
		keys = append(keys, "t", "trial")
	}
	switch {
	case v.cur >= 0 && !v.playable(v.cur):
	case v.cur >= 0 && v.nodes[v.cur].url != "":
		keys = append(keys, "o", "open problem")
	case v.e.mod.URL != "":
		keys = append(keys, "o", "usaco.guide")
	}
	keys = append(keys, "q", "map")
	return ui.StatusBar(w, v.l.Module+"(7)", keys...)
}
