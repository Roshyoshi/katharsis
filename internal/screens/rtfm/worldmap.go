package rtfm

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/curriculum"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Map is RTFM's world map (.agents/work/rtfm-map-plan.md): the overworld,
// a world per division of the guide, and each world, its modules as levels
// on one path in guide order, folded left and right down the screen the
// way a Mario map folds. A card names the level under the cursor. Enter
// opens the level in place and q there comes back, so the app keeps one
// screen for the map; i leaves for the man index (screens.OpenIndex),
// which searches. Nothing is locked: the path lights up behind what is
// cleared, and the selected level's prerequisites light up with it.
type Map struct {
	cur       *curriculum.Curriculum
	atlas     curriculum.Atlas
	meta      game.Meta
	prog      curriculum.Progress
	judgeable Judgeable
	lookup    screens.Lookup

	lv    [][]spot            // each world's levels in order
	from  map[string][]string // each warp target's sources
	world int                 // the world shown, -1 for the overworld
	ow    int                 // the overworld's selection
	at    []int               // each world's selected level, into lv

	// warpFrom is the level the last w left and warpTo the index in its
	// Warps it went to: w at that target goes back, and w at the source
	// again goes on to its next target.
	warpFrom string
	warpTo   int

	// flipped is the capabilities the switch palace fills in (flip.go), and
	// flip its clip; nil before Flip.
	flipped map[string]bool
	flip    *anim.Player

	level  *Level // the open level; nil shows the map
	linked bool   // opened at a level for another screen (Open): q goes Back
	grid   grid   // the world's layout at the last View, for ↑/↓
}

// spot is a level and the index of its region in the world.
type spot struct {
	curriculum.Level
	region int
}

// NewMap builds the map of c for the player m; judgeable and lookup are as
// for NewBrowser. It opens on the overworld at the first world with a
// trial left, and each world at its first level not yet cleared.
func NewMap(c *curriculum.Curriculum, m game.Meta, judgeable Judgeable, lookup screens.Lookup) *Map {
	return newMap(c, c.Atlas(judgeable), m, judgeable, lookup)
}

func newMap(c *curriculum.Curriculum, a curriculum.Atlas, m game.Meta, judgeable Judgeable, lookup screens.Lookup) *Map {
	p := &Map{cur: c, atlas: a, judgeable: judgeable, lookup: lookup,
		from: map[string][]string{}, world: -1, at: make([]int, len(a.Worlds))}
	for _, w := range a.Worlds {
		var s []spot
		for ri, r := range w.Regions {
			for _, l := range r.Levels {
				s = append(s, spot{l, ri})
				for _, to := range l.Warps {
					p.from[to] = append(p.from[to], l.Module)
				}
			}
		}
		p.lv = append(p.lv, s)
	}
	p.Refresh(m)
	p.ow = max(slices.IndexFunc(a.Worlds, func(w curriculum.World) bool {
		s := p.prog.Sum(w)
		return s.Passed < s.Trials
	}), 0)
	for wi, s := range p.lv {
		p.at[wi] = max(slices.IndexFunc(s, func(x spot) bool { return !p.cleared(x.Level) }), 0)
	}
	return p
}

// Refresh swaps in the player's Meta (a trial passed, a page opened),
// keeping the place on the map. An open level is built again.
func (p *Map) Refresh(m game.Meta) {
	p.meta, p.prog = m, p.atlas.Progress(&m)
	if p.level != nil {
		// The level is built again where its cursor was: back from a
		// practice problem, on that problem.
		cur, ptop := p.level.cur, p.level.ptop
		p.openLevel()
		p.level.cur, p.level.ptop = cur, ptop
	}
}

// Open shows the world of the module with id, its level selected, as a
// link from another screen (a locked exit's t, the next step after a
// trial): q goes straight back there (screens.Back). It reports whether
// the map has the module.
func (p *Map) Open(id string) bool {
	if !p.show(id) {
		return false
	}
	p.level, p.linked = nil, true
	return true
}

// show selects the level of module id, in its world.
func (p *Map) show(id string) bool {
	for wi, s := range p.lv {
		if i := slices.IndexFunc(s, func(x spot) bool { return x.Module == id }); i >= 0 {
			p.world, p.ow, p.at[wi] = wi, wi, i
			return true
		}
	}
	return false
}

// Selected is the module of the level the cursor is on, "" on the
// overworld.
func (p *Map) Selected() string {
	if p.world < 0 {
		return ""
	}
	return p.spot().Module
}

// spot is the selected level.
func (p *Map) spot() spot { return p.lv[p.world][p.at[p.world]] }

func (p *Map) openLevel() {
	s := p.spot()
	region := p.atlas.Worlds[p.world].Regions[s.region].Title
	p.level = NewLevel(p.cur, s.Level, region, p.meta, p.judgeable, p.lookup)
}

// cleared reports whether l is behind the player: its trial passed, or for
// a page, opened. It lights the path on from l.
func (p *Map) cleared(l curriculum.Level) bool {
	lp := p.prog[l.Module]
	return lp.Passed || l.Kind == curriculum.PageLevel && lp.Opened
}

// Update moves on the map. An open level gets every message but the keys
// that close it, as the index's module page does.
func (p *Map) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	if p.flip != nil {
		if cmd, used := p.flip.Update(msg); used {
			return p, cmd
		}
	}
	k, isKey := msg.(tea.KeyPressMsg)
	if p.level != nil {
		if isKey {
			switch k.String() {
			case "q", "esc", "h", "left", "backspace":
				p.level = nil
				return p, nil
			}
		}
		_, cmd := p.level.Update(msg)
		return p, cmd
	}
	if !isKey {
		return p, nil
	}
	if p.world < 0 {
		return p, p.overworldKey(k.String())
	}
	return p, p.worldKey(k.String())
}

func (p *Map) overworldKey(k string) tea.Cmd {
	switch k {
	case "up", "k":
		p.ow = max(p.ow-1, 0)
	case "down", "j":
		p.ow = min(p.ow+1, len(p.lv)-1)
	case "enter", "right", "l":
		if len(p.lv) > 0 {
			p.world = p.ow
		}
	case "i", "/":
		return screens.Emit(screens.OpenIndex{})
	case "q", "esc":
		return screens.Emit(screens.Back{})
	}
	return nil
}

func (p *Map) worldKey(k string) tea.Cmd {
	n := len(p.lv[p.world])
	i := &p.at[p.world]
	switch k {
	case "right", "l":
		*i = min(*i+1, n-1)
	case "left", "h":
		*i = max(*i-1, 0)
	case "down", "j":
		*i = p.grid.step(*i, 1, n)
	case "up", "k":
		*i = p.grid.step(*i, -1, n)
	case "home", "g":
		*i = 0
	case "end", "G":
		*i = n - 1
	case "tab":
		p.world = (p.world + 1) % len(p.lv)
	case "shift+tab":
		p.world = (p.world + len(p.lv) - 1) % len(p.lv)
	case "enter":
		p.openLevel()
	case "p":
		p.openLevel()
		p.level.Practice()
	case "w":
		p.warp()
	case "r":
		return screens.Emit(screens.OpenModule{Module: p.spot().Module})
	case "t":
		// A page whose capability is earned elsewhere goes to that trial.
		s := p.spot()
		if l, _, ok := p.atlas.EarnedIn(s.Module); ok {
			p.show(l.Module)
		} else if s.Kind != curriculum.PageLevel {
			return screens.Emit(screens.StartTrial{Module: s.Module})
		}
	case "i", "/":
		return screens.Emit(screens.OpenIndex{Module: p.spot().Module})
	case "q", "esc":
		if p.linked {
			return screens.Emit(screens.Back{})
		}
		p.ow, p.world = p.world, -1
	}
	return nil
}

// warp is w: from a level with warps to its next target in turn, and from
// the target the last w reached back to where it came from.
func (p *Map) warp() {
	s := p.spot()
	switch {
	case len(s.Warps) > 0:
		n := 0
		if p.warpFrom == s.Module {
			n = (p.warpTo + 1) % len(s.Warps)
		}
		p.warpFrom, p.warpTo = s.Module, n
		p.show(s.Warps[n])
	case p.warpFrom != "":
		if src, _, ok := p.atlas.Level(p.warpFrom); ok && p.warpTo < len(src.Warps) && src.Warps[p.warpTo] == s.Module {
			p.show(p.warpFrom)
		}
	}
}

// View draws the open level, the overworld or a world.
func (p *Map) View(width, height int) string {
	if p.level != nil {
		return p.level.View(width, height)
	}
	w, h := max(width, 1), max(height, 1)
	if p.world < 0 {
		return p.overworld(w, h)
	}
	return p.worldView(w, h)
}

// ---- a world ----------------------------------------------------------------

// wideMin is the width from which the card moves into a panel beside the
// map and each level's title shows under it.
const wideMin = 120

func (p *Map) worldView(w, h int) string {
	wd := p.atlas.Worlds[p.world]
	crumb := ui.Strong.Render("RTFM") + ui.Muted.Render(ui.G.Crumb) + wd.Title
	out := []string{ui.Spread(crumb, tally(p.prog.Sum(wd)), w), ui.Rule(w)}
	room := max(h-len(out)-1, 0)
	keys := []string{"←/→", "path", "↑/↓", "row", "enter", "level", "r", "read", "t", "trial",
		"w", "warp", "tab", "world", "i", "index", "q", "map"}
	if p.linked {
		keys[len(keys)-1] = "back"
	}
	bar := ui.StatusBar(w, wd.Title, keys...)

	s := p.spot()
	tag := wd.Title + ui.G.Crumb + wd.Regions[s.region].Title
	if w >= wideMin && h >= 30 {
		cw := min(48, w/3)
		mw := w - cw - 2
		p.grid = layout(len(p.lv[p.world]), mw, room, true)
		lines := p.card()
		var body []string
		for _, l := range lines {
			body = append(body, ui.WrapN(l, cw-4, 0)...)
		}
		card := ui.Frame(ui.FrameIdle, cw, s.Title, tag, body)
		out = append(out, ui.JoinH(2, p.drawWorld(mw, room), card)...)
		return ui.Layout(w, h, out, bar)
	}
	card := ui.Frame(ui.FrameIdle, w, s.Title, tag, p.card())
	mapRoom := max(room-len(card), 0)
	p.grid = layout(len(p.lv[p.world]), w, mapRoom, false)
	out = append(out, ui.PadLines(p.drawWorld(w, mapRoom), mapRoom)...)
	return ui.Layout(w, h, append(out, card...), bar)
}

// tally is a world's counts for the header, each with its real
// denominator; a world of pages counts what was opened.
func tally(s curriculum.WorldSum) string {
	var parts []string
	if s.Trials > 0 {
		parts = append(parts, fmt.Sprintf("trials %d/%d", s.Passed, s.Trials))
	}
	if s.Caps > 0 {
		mark := ui.G.CapLocked
		if s.CapsHeld > 0 {
			mark = ui.G.Cap
		}
		parts = append(parts, fmt.Sprintf("%s %d/%d", mark, s.CapsHeld, s.Caps))
	}
	if s.Playable > 0 {
		parts = append(parts, fmt.Sprintf("solved %d/%d", s.Solved, s.Playable))
	}
	if s.Trials == 0 {
		parts = append(parts, fmt.Sprintf("opened %d/%d", s.Opened, s.Levels))
	}
	return ui.Muted.Render(strings.Join(parts, ui.G.Sep))
}

// grid is a world's layout: n levels to a row, slot cells apart, on rows
// of per lines each (the region labels, the path, the marks, the titles
// and a blank when they show). Rows run left to right and right to left in
// turn, joined by a turn at the end, turn cells past a right-hand glyph:
// clear of what sits under it.
type grid struct {
	n, slot, per, rows, turn int
	titles                   bool
}

// firstCol is the column of a row's leftmost glyph. The cursor sits just
// before a glyph, and a left turn three before that, in column 1.
const firstCol = 5

// layout fits count levels into rows mw cells wide: as many to a row as
// leave room for the turn after the last, past its marks ("■ ↑", wider in
// ASCII) and its title when titles show. A gap line goes between rows when
// there is room cells for it.
func layout(count, mw, room int, titles bool) grid {
	g := grid{slot: 7, per: 3}
	g.turn = ansi.StringWidth(ui.G.Cap+" "+ui.G.Warp) + 1
	if titles {
		g.slot, g.per, g.titles = 13, 4, true
		g.turn = max(g.turn, g.slot)
	}
	g.n = max((mw-firstCol-g.turn-1)/g.slot+1, 1)
	g.rows = (count + g.n - 1) / g.n
	if titles && g.rows*(g.per+1) <= room {
		g.per++
	}
	return g
}

// pos is level i's row and its glyph's column.
func (g grid) pos(i int) (row, col int) {
	r, j := i/g.n, i%g.n
	if r%2 == 1 {
		j = g.n - 1 - j
	}
	return r, firstCol + j*g.slot
}

// step is ↑/↓: from level i to the level dr rows on nearest its column, i
// itself when there is no such row.
func (g grid) step(i, dr, count int) int {
	if g.n == 0 {
		return i
	}
	r, x := g.pos(i)
	r += dr
	if r < 0 || r*g.n >= count {
		return i
	}
	best, far := i, -1
	for j := r * g.n; j < min((r+1)*g.n, count); j++ {
		_, c := g.pos(j)
		if d := max(c-x, x-c); far < 0 || d < far {
			best, far = j, d
		}
	}
	return best
}

// drawWorld draws the world's path on rows mw cells wide, scrolled to keep
// the selection in view when it is more than room lines.
func (p *Map) drawWorld(mw, room int) []string {
	g, s, sel := p.grid, p.lv[p.world], p.at[p.world]
	c := newCanvas(mw, g.rows*g.per)
	need := map[string]bool{}
	for _, id := range s[sel].Prereqs {
		need[id] = true
	}
	for i, l := range s {
		r, x := g.pos(i)
		y := r*g.per + 1
		if i > 0 {
			p.path(c, i)
		}
		if i == sel {
			c.put(y, x-1, ui.Focus.Render(ui.G.Cursor))
		}
		c.put(y, x, p.glyph(l.Level, need[l.Module]))
		c.put(y+1, x, p.marks(i))
		if g.titles {
			tok := ui.Muted
			if i == sel {
				tok = ui.Focus
			}
			c.put(y+2, x, tok.Render(ui.Clip(l.Title, g.slot-1)))
		}
	}
	p.labels(c)

	lines := c.lines()
	if fit := room / g.per; len(lines) > room && fit > 0 {
		r, _ := g.pos(sel)
		top := min(max(r-fit/2, 0), g.rows-fit)
		lines = lines[top*g.per : (top+fit)*g.per]
	}
	return lines
}

// glyph is a level's node: ○ ahead, ✓ passed, ▪ a page (faint until
// opened, and never ticked: opening is not reading), ⚔ the castle. need
// marks a prerequisite of the selection, in the focus colour.
func (p *Map) glyph(l curriculum.Level, need bool) string {
	lp := p.prog[l.Module]
	g, tok := ui.G.Ahead, ui.FG
	switch {
	case l.Kind == curriculum.PageLevel:
		g, tok = ui.G.Page, ui.Muted
		if lp.Opened {
			tok = ui.FG
		}
	case lp.Passed:
		g, tok = ui.G.Pass, ui.OK
	case l.Kind == curriculum.CastleLevel:
		g, tok = ui.G.Boss, ui.Special
	}
	if need {
		tok = ui.Focus
	}
	return tok.Render(g)
}

// marks is the line under level i's node: ■ or □ for the capability its
// trial grants (□ still while the switch palace has yet to reach it), ↑
// when it opens a warp.
func (p *Map) marks(i int) string {
	l := p.lv[p.world][i]
	var m []string
	switch {
	case l.Capability == "":
	case p.prog[l.Module].CapHeld && !p.unflipped(i):
		m = append(m, ui.OK.Render(ui.G.Cap))
	default:
		m = append(m, ui.Muted.Render(ui.G.CapLocked))
	}
	if len(l.Warps) > 0 {
		m = append(m, ui.Special.Render(ui.G.Warp))
	}
	return strings.Join(m, " ")
}

// path draws the way into level i from the level before: along the row,
// or round the turn at its end. It is lit once the level before is
// cleared, and dotted into an optional level.
func (p *Map) path(c *canvas, i int) {
	g, s := p.grid, p.lv[p.world]
	tok := ui.Muted
	if p.cleared(s[i-1].Level) {
		tok = ui.FG
	}
	across, down := ui.G.Rule, ui.G.Thin.Left
	if s[i].Optional {
		across, down = ui.G.Dotted, ui.G.DottedV
	}
	run := func(y, a, b int) {
		for x := a; x <= b; x++ {
			c.put(y, x, tok.Render(across))
		}
	}
	r0, x0 := g.pos(i - 1)
	r1, x1 := g.pos(i)
	y0, y1 := r0*g.per+1, r1*g.per+1
	if r0 == r1 {
		run(y0, min(x0, x1)+2, max(x0, x1)-3)
		return
	}
	// A row that ran right turns down at its right end, one that ran left
	// at its left; the row below starts under the turn.
	t, top, bot := x0+g.turn, ui.G.Thin.TopRight, ui.G.Thin.BottomRight
	if r0%2 == 0 {
		run(y0, x0+2, t-1)
		run(y1, x1+2, t-1)
	} else {
		t, top, bot = firstCol-4, ui.G.Thin.TopLeft, ui.G.Thin.BottomLeft
		run(y0, t+1, x0-3)
		run(y1, t+1, x1-3)
	}
	c.put(y0, t, tok.Render(top))
	c.put(y1, t, tok.Render(bot))
	for y := y0 + 1; y < y1; y++ {
		c.put(y, t, tok.Render(down))
	}
}

// labels names the regions over each row, at each one's leftmost node: a
// region that goes on from the row before is named again, faint. With
// titles, each says how many of its trials are passed.
func (p *Map) labels(c *canvas) {
	g, s := p.grid, p.lv[p.world]
	regions := p.atlas.Worlds[p.world].Regions
	type run struct{ x, region, first int }
	for r := range g.rows {
		var runs []run
		for i := r * g.n; i < min((r+1)*g.n, len(s)); i++ {
			_, x := g.pos(i)
			if len(runs) == 0 || s[i].region != runs[len(runs)-1].region {
				runs = append(runs, run{x, s[i].region, i})
			}
			last := &runs[len(runs)-1]
			last.x = min(last.x, x)
		}
		slices.SortFunc(runs, func(a, b run) int { return a.x - b.x })
		// A row that came round a right turn has it in its label line.
		end := c.w
		if r%2 == 1 {
			_, x := g.pos(r*g.n - 1)
			end = x + g.turn
		}
		for k, ru := range runs {
			stop := end
			if k+1 < len(runs) {
				stop = runs[k+1].x
			}
			text := regions[ru.region].Title
			if g.titles {
				text += " " + p.regionTally(ru.region)
			}
			tok := ui.Strong
			if ru.first > 0 && s[ru.first-1].region == ru.region {
				tok = ui.Muted
			}
			// A label cut to a stub says nothing the card's tag does not.
			if room := stop - ru.x - 1; room >= min(ansi.StringWidth(text), 5) {
				c.put(r*g.per, ru.x, tok.Render(ui.Clip(text, room)))
			}
		}
	}
}

// regionTally is "k/n", a region's trials passed of those it has, or ""
// for a region of pages.
func (p *Map) regionTally(region int) string {
	passed, n := 0, 0
	for _, l := range p.atlas.Worlds[p.world].Regions[region].Levels {
		if l.Kind != curriculum.PageLevel {
			n++
			if p.prog[l.Module].Passed {
				passed++
			}
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", passed, n)
}

// card is the selected level's facts, a line each for the caller to clip
// or wrap: what it is and what passing it grants, what it needs, what has
// been done in it, and its warps.
func (p *Map) card() []string {
	s := p.spot()
	l, lp := s.Level, p.prog[s.Module]
	var what string
	switch {
	case l.Capability != "":
		what = capCell(l.Capability, lp.CapHeld) + " " + ui.G.Arrow + " " + flavor.BiomeName(l.Topic)
	case l.Kind == curriculum.CastleLevel:
		what = ui.Special.Render(ui.G.Boss) + " the " + p.atlas.Worlds[p.world].Title + " gauntlet: mixed topics"
	case l.Kind == curriculum.PageLevel:
		what = ui.G.Page + " reading only"
	default:
		what = ui.Muted.Render("grants no capability, " + ui.G.Pass + " only")
	}
	if f := l.Frequency; f >= 0 {
		what += ui.Muted.Render(ui.G.Sep) + stars(f) + " " + ui.Muted.Render(frequencyText(f))
	}
	out := []string{what, ui.Muted.Render("needs ") + p.needs(l)}

	var did []string
	if lp.Opened {
		did = append(did, "opened")
	}
	switch n := len(l.Playable); {
	case n > 0 && l.Kind.HasTrial():
		did = append(did, fmt.Sprintf("practice %d/%d solved", lp.Solved, n))
	case n > 0:
		did = append(did, units.Plural(n, "example")+" from the guide")
	}
	earned, _, earn := p.atlas.EarnedIn(l.Module)
	switch {
	case earn:
		did = append(did, "trial in "+p.titles([]string{earned.Module})+ui.Muted.Render(": t goes there"))
	case l.Kind == curriculum.PageLevel:
	case lp.Passed:
		did = append(did, ui.OK.Render(ui.G.Pass+" trial passed"))
	default:
		did = append(did, fmt.Sprintf("trial %s + %d blind", units.Plural(curriculum.DefaultUnitSize, "door"), integrationDoors))
	}
	if len(did) == 0 {
		did = append(did, ui.Muted.Render("r opens it in the pager"))
	}
	out = append(out, strings.Join(did, ui.Muted.Render(ui.G.Sep)))

	switch {
	case len(l.Warps) > 0:
		out = append(out, ui.Special.Render(ui.G.Warp)+" opens "+p.titles(l.Warps))
	case len(p.from[l.Module]) > 0:
		out = append(out, ui.Special.Render(ui.G.Warp)+" from "+p.titles(p.from[l.Module]))
	}
	return out
}

// needs lists l's prerequisites, each ticked when its trial is passed and
// named with its world when that is not this one.
func (p *Map) needs(l curriculum.Level) string {
	if len(l.Prereqs) == 0 {
		return ui.Muted.Render("nothing")
	}
	var parts []string
	for _, id := range l.Prereqs {
		q, _, _ := p.atlas.Level(id)
		t := p.titles([]string{id})
		if p.prog[q.Module].Passed {
			t += " " + ui.OK.Render(ui.G.Pass)
		}
		parts = append(parts, t)
	}
	return strings.Join(parts, ui.Muted.Render(ui.G.Sep))
}

// titles names levels by title, with their world's when it is not this
// one's.
func (p *Map) titles(ids []string) string {
	var out []string
	for _, id := range ids {
		l, wi, ok := p.atlas.Level(id)
		if !ok {
			out = append(out, id)
			continue
		}
		t := l.Title
		if wi != p.world {
			t += " (" + p.atlas.Worlds[wi].Title + ")"
		}
		out = append(out, t)
	}
	return strings.Join(out, ", ")
}

// ---- the overworld ----------------------------------------------------------

// overworld is the hub: the worlds down a path, each with its counts, a
// world of pages first and the world with no castle last, on a dotted
// path (Super Mario World's Special World).
func (p *Map) overworld(w, h int) string {
	out := []string{manHeader(manName, manTitle, w), ""}
	if len(p.atlas.Worlds) == 0 {
		out = append(out, "No manual entry for rtfm", "",
			ui.Muted.Render("The USACO Guide snapshot has no modules to map."))
		return ui.Layout(w, h, out, ui.StatusBar(w, "rtfm(7)", "q", "quit"))
	}
	tw := 0
	for _, wd := range p.atlas.Worlds {
		tw = max(tw, ansi.StringWidth(wd.Title))
	}
	const pad = "      " // to the glyph column
	note := ""           // the world above's, beside the path down from it
	for i, wd := range p.atlas.Worlds {
		sum := p.prog.Sum(wd)
		if i > 0 {
			down := ui.G.Thin.Left
			if p.special(i) {
				down = ui.G.DottedV
			}
			out = append(out, ui.Clip(pad+ui.Muted.Render(down)+note, w))
		}
		cur := "  "
		if i == p.ow {
			cur = ui.Focus.Render(ui.G.Cursor) + " "
		}
		title := ui.Strong.Render(wd.Title) + strings.Repeat(" ", tw-ansi.StringWidth(wd.Title))
		out = append(out, ui.Clip("    "+cur+worldGlyph(sum)+"  "+title+"  "+p.worldLine(i, sum), w))
		note = p.worldNote(i)
		if note != "" {
			note = strings.Repeat(" ", tw+2) + note
		}
	}
	if note != "" {
		out = append(out, ui.Clip(pad+" "+note, w))
	}
	held, total := p.capCount()
	out = append(out, "", "",
		ui.Clip("  "+capMark(held > 0)+fmt.Sprintf(" %d/%d capabilities held", held, total)+
			ui.Muted.Render(ui.G.Sep+"each opens topic exits on the boot map"), w))
	bar := ui.StatusBar(w, "rtfm(7)", "↑/↓", "world", "enter", "open", "i", "index", "q", "quit")
	return ui.Layout(w, h, out, bar)
}

// special reports whether world i is the special world: one of trials with
// no castle at its end.
func (p *Map) special(i int) bool {
	s := p.lv[i]
	return p.prog.Sum(p.atlas.Worlds[i]).Trials > 0 && s[len(s)-1].Kind != curriculum.CastleLevel
}

// worldGlyph is a world's node: ▪ for a world of pages, ✓ once every trial
// in it is passed, ○ before.
func worldGlyph(s curriculum.WorldSum) string {
	switch {
	case s.Trials == 0:
		return ui.G.Page
	case s.Passed == s.Trials:
		return ui.OK.Render(ui.G.Pass)
	}
	return ui.G.Ahead
}

// worldLine is a world's counts on the overworld: its levels, a bar and
// count of its trials passed, its castle, its capabilities held. A world
// of pages counts the ones opened.
func (p *Map) worldLine(i int, s curriculum.WorldSum) string {
	if s.Trials == 0 {
		return ui.Muted.Render(units.Plural(s.Levels, "page")+ui.G.Sep+"library") +
			"   " + fmt.Sprintf("opened %d/%d", s.Opened, s.Levels)
	}
	const cells = 12
	fill := s.Passed * cells / s.Trials
	bar := ui.OK.Render(strings.Repeat(ui.G.Fill, fill)) + ui.Muted.Render(strings.Repeat(ui.G.Empty, cells-fill))
	castle := "   "
	if l := p.lv[i][len(p.lv[i])-1]; l.Kind == curriculum.CastleLevel {
		castle = ui.Special.Render(ui.G.Boss) + "  "
		if p.prog[l.Module].Passed {
			castle = ui.Special.Render(ui.G.Boss) + " " + ui.OK.Render(ui.G.Pass)
		}
	}
	caps := ""
	if s.Caps > 0 {
		caps = capMark(s.CapsHeld > 0) + fmt.Sprintf(" %d/%d", s.CapsHeld, s.Caps)
	}
	return fmt.Sprintf("%2d levels  %s  %5s trials  %s  %s", s.Levels, bar,
		fmt.Sprintf("%d/%d", s.Passed, s.Trials), castle, caps)
}

// capMark is ■ when any is held, □ before.
func capMark(held bool) string {
	if held {
		return ui.OK.Render(ui.G.Cap)
	}
	return ui.Muted.Render(ui.G.CapLocked)
}

// worldNote is a line under a world on the overworld: how many levels of
// later worlds its warps open, or, for the special world, how many of its
// levels need nothing first.
func (p *Map) worldNote(i int) string {
	var to []string
	free := 0
	for _, l := range p.lv[i] {
		for _, t := range l.Warps {
			if !slices.Contains(to, t) {
				to = append(to, t)
			}
		}
		if len(l.Prereqs) == 0 {
			free++
		}
	}
	switch {
	case len(to) > 0:
		return ui.Special.Render(ui.G.Warp) + ui.Muted.Render(" "+units.Plural(len(to), "level")+" of later worlds open from here")
	case p.special(i):
		return ui.Muted.Render("the special world" + ui.G.Sep + units.Plural(free, "level") + " need nothing first")
	}
	return ""
}

// capCount is the capabilities the atlas teaches, and how many are held.
func (p *Map) capCount() (held, total int) {
	seen := map[string]bool{}
	for _, s := range p.lv {
		for _, l := range s {
			if l.Capability == "" || seen[l.Capability] {
				continue
			}
			seen[l.Capability] = true
			total++
			if p.prog[l.Module].CapHeld {
				held++
			}
		}
	}
	return held, total
}

// ---- drawing ----------------------------------------------------------------

// canvas is the cells a world is drawn on, each one cell of styled text,
// or "" where a wider string put before it covers it.
type canvas struct {
	w     int
	cells [][]string
}

func newCanvas(w, h int) *canvas {
	c := &canvas{w: w, cells: make([][]string, h)}
	for y := range c.cells {
		c.cells[y] = slices.Repeat([]string{" "}, w)
	}
	return c
}

// put writes s at row y from column x. s past the canvas is dropped
// whole: callers clip.
func (c *canvas) put(y, x int, s string) {
	n := ansi.StringWidth(s)
	if n == 0 || y < 0 || y >= len(c.cells) || x < 0 || x+n > c.w {
		return
	}
	c.cells[y][x] = s
	for i := 1; i < n; i++ {
		c.cells[y][x+i] = ""
	}
}

func (c *canvas) lines() []string {
	out := make([]string, len(c.cells))
	for y, row := range c.cells {
		out[y] = strings.Join(row, "")
	}
	return out
}
