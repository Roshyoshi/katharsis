package rtfm

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Browser is the RTFM index: the guide's divisions, chapters and modules in
// guide order, set like a man page (section headings at the margin, entries
// indented under them) with less's prompt line at the bottom. Chapters and
// divisions fold; / filters modules by title or id. Enter on a module opens
// its page in place, and q or h there returns to the index, so the app
// keeps one screen on its stack for all of learn mode; a page another
// screen linked to (Open) goes back there on q. A module the path has not
// reached (curriculum.Atlas.Progress) is listed faint, and does not open.
type Browser struct {
	divs   []division
	all    []entry // every module, for column widths and the totals
	cur    *curriculum.Curriculum
	caps   map[string]bool
	passed map[string]bool // the modules whose trial is passed
	atlas  curriculum.Atlas
	prog   curriculum.Progress

	rows     []row // what is visible: folds and the filter applied
	sel, top int   // sel indexes rows; top is the first visible line
	h        int   // list rows at the last View, for paging
	// tail is set while the view is scrolled on past the last row, into
	// the page's tail, where it need not show the selection.
	tail bool

	typing bool
	input  string // what has been typed at the / prompt
	query  string // the committed filter
	active string // the filter rows were built with: input while typing
	mark   row    // the selection before filtering began

	page *Module // the open module page; nil shows the index
	// linked is set while page is the one Open showed, a link from another
	// screen, which q and esc go back to.
	linked bool
}

type division struct {
	title  string
	folded bool
	chaps  []chapter
}

type chapter struct {
	title  string
	folded bool
	mods   []entry
}

type rowKind int

const (
	divRow rowKind = iota
	chapRow
	modRow
)

// row is one selectable line: a division heading, a chapter heading or a
// module, by index into divs (c and m are -1 when they do not apply).
type row struct {
	kind    rowKind
	d, c, m int
}

// NewBrowser builds the index over c, with capability marks from m.
// judgeable picks the problems each module's map plays (see
// curriculum.TrialFor) and lookup resolves the ratings its trial aims by;
// either may be nil. A nil curriculum shows man's "No manual entry". The
// guide's pages about itself (curriculum.Module.Meta) are left out, and
// with them a chapter or division they would leave empty (UX J21).
func NewBrowser(c *curriculum.Curriculum, m game.Meta, judgeable Judgeable, lookup screens.Lookup) *Browser {
	b := &Browser{cur: c, caps: capSet(m), passed: passedSet(m)}
	if c != nil {
		b.atlas = c.Atlas(judgeable)
		b.prog = b.atlas.Progress(&m)
		for _, d := range c.Divisions {
			dv := division{title: d.Title}
			for _, ch := range d.Chapters {
				cv := chapter{title: ch.Title}
				for _, ref := range ch.Modules {
					mod, ok := c.Module(ref.ID)
					if !ok || mod.Meta {
						continue
					}
					e := newEntry(mod, judgeable, lookup)
					cv.mods = append(cv.mods, e)
					b.all = append(b.all, e)
				}
				if len(cv.mods) > 0 {
					dv.chaps = append(dv.chaps, cv)
				}
			}
			if len(dv.chaps) > 0 {
				b.divs = append(b.divs, dv)
			}
		}
	}
	b.build()
	b.sel = b.firstModule()
	return b
}

// Refresh swaps in the player's Meta (a passed trial grants a capability),
// keeping folds, filter, selection and an open module page.
func (b *Browser) Refresh(m game.Meta) {
	b.caps, b.passed, b.prog = capSet(m), passedSet(m), b.atlas.Progress(&m)
	if b.page != nil {
		b.page.caps, b.page.passed = b.caps, b.passed[b.page.e.mod.ID]
	}
}

// Typing reports whether the / prompt is open. The prompt wants every
// printable key for itself, so an app with global keys should pass them
// through while it is.
func (b *Browser) Typing() bool { return b.typing }

// Open shows the page of the module with id as a link from another screen
// (a locked exit's t, the next step after a boot or a trial): q and esc go
// straight back there (screens.Back), while h returns to the index, its
// row unfolded and selected. It reports whether it opened the page: the
// curriculum has the module, and the path has reached it; a locked one is
// only selected.
func (b *Browser) Open(id string) bool {
	if !b.Select(id) {
		return false
	}
	r, _ := b.selected()
	b.linked = b.open(r)
	return b.linked
}

// Select puts the index at the module with id, its row unfolded and
// selected, the page not opened: the map's i. It reports whether the
// curriculum has the module.
func (b *Browser) Select(id string) bool {
	for di, d := range b.divs {
		for ci, c := range d.chaps {
			for mi, e := range c.mods {
				if e.mod.ID == id {
					b.typing, b.query = false, ""
					b.applyFilter("", row{modRow, di, ci, mi})
					return true
				}
			}
		}
	}
	return false
}

func (b *Browser) entry(r row) entry { return b.divs[r.d].chaps[r.c].mods[r.m] }

func (b *Browser) selected() (row, bool) {
	if len(b.rows) == 0 {
		return row{}, false
	}
	return b.rows[b.sel], true
}

// matches reports whether a module passes the filter q (lower case): its
// title or its id, since ids are what the guide's URLs and prerequisites
// use ("PURS", "VT").
func matches(e entry, q string) bool {
	return strings.Contains(strings.ToLower(e.mod.Title), q) ||
		strings.Contains(strings.ToLower(e.mod.ID), q)
}

// build lists the visible rows. A filter shows every match with its
// headings and ignores folds: a match hidden in a folded chapter would
// look like no match at all.
func (b *Browser) build() {
	q := strings.ToLower(b.active)
	b.rows = b.rows[:0]
	for di, d := range b.divs {
		var drows []row
		for ci, c := range d.chaps {
			var crows []row
			for mi, e := range c.mods {
				if q == "" || matches(e, q) {
					crows = append(crows, row{modRow, di, ci, mi})
				}
			}
			if q != "" && len(crows) == 0 {
				continue
			}
			drows = append(drows, row{chapRow, di, ci, -1})
			if q != "" || !c.folded {
				drows = append(drows, crows...)
			}
		}
		if q != "" && len(drows) == 0 {
			continue
		}
		b.rows = append(b.rows, row{divRow, di, -1, -1})
		if q != "" || !d.folded {
			b.rows = append(b.rows, drows...)
		}
	}
	b.sel = min(max(b.sel, 0), max(len(b.rows)-1, 0))
}

func (b *Browser) firstModule() int {
	for i, r := range b.rows {
		if r.kind == modRow {
			return i
		}
	}
	return 0
}

// restore selects r in the unfiltered index, unfolding what hides it.
func (b *Browser) restore(r row) {
	if r.kind != divRow {
		d := &b.divs[r.d]
		if d.folded || (r.kind == modRow && d.chaps[r.c].folded) {
			d.folded = false
			if r.kind == modRow {
				d.chaps[r.c].folded = false
			}
			b.build()
		}
	}
	for i, x := range b.rows {
		if x == r {
			b.sel = i
			return
		}
	}
	b.sel = b.firstModule()
}

// applyFilter rebuilds the rows for filter q. A new filter selects its
// first match; clearing one selects keep, so the module found by the
// filter is shown in its place in the index.
func (b *Browser) applyFilter(q string, keep row) {
	b.active = q
	b.build()
	if q == "" {
		b.restore(keep)
	} else {
		b.sel = b.firstModule()
	}
	b.top = 0
}

// refold rebuilds after a fold change, keeping the selection on r.
func (b *Browser) refold(r row) {
	b.build()
	for i, x := range b.rows {
		if x == r {
			b.sel = i
			return
		}
	}
}

// open opens module row r's page, unless the path has yet to reach it,
// and reports whether it did.
func (b *Browser) open(r row) bool {
	e := b.entry(r)
	if b.locked(e.mod.ID) {
		return false
	}
	b.page, b.linked = newModule(b.cur, e, b.caps, b.passed[e.mod.ID]), false
	return true
}

// locked reports whether the path has yet to reach module id's level; a
// module off the atlas is never locked.
func (b *Browser) locked(id string) bool {
	lp, ok := b.prog[id]
	return ok && !lp.Open
}

// lockNote is why the selected module does not open, "" when it does:
// "□ locked: pass Two Pointers first".
func (b *Browser) lockNote() string {
	r, ok := b.selected()
	if !ok || r.kind != modRow {
		return ""
	}
	id := b.entry(r).mod.ID
	if !b.locked(id) {
		return ""
	}
	note := ui.G.CapLocked + " locked"
	if l, ok := b.atlas.Before(id); ok {
		note += ": pass " + l.Title + " first"
	}
	return note
}

func (b *Browser) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, isKey := msg.(tea.KeyPressMsg)
	if b.page != nil {
		if isKey {
			switch k.String() {
			case "q", "esc":
				if b.linked {
					return b, screens.Emit(screens.Back{})
				}
				b.page = nil
				return b, nil
			case "h", "left", "backspace":
				b.page = nil
				return b, nil
			}
		}
		_, cmd := b.page.Update(msg)
		return b, cmd
	}
	if !isKey {
		return b, nil
	}
	// Only the keys that scroll the tail keep the view there; any other
	// brings the selection back on screen.
	tail := b.tail
	b.tail = false
	if b.typing {
		b.prompt(k)
		return b, nil
	}
	last := max(len(b.rows)-1, 0)
	page := max(b.h-1, 1)
	r, ok := b.selected()
	// On the last row the page goes on past it, through the tail to the
	// colophon, the way man's does; back up, it scrolls until the row is
	// on screen again.
	atEnd := len(b.rows) > 0 && b.sel == last
	hidden := tail && b.top > b.sel
	switch k.String() {
	case "q":
		return b, screens.Emit(screens.Back{})
	case "esc":
		if b.query != "" {
			b.query = ""
			b.applyFilter("", r)
			return b, nil
		}
		return b, screens.Emit(screens.Back{})
	case "up", "k", "ctrl+p":
		if hidden {
			b.tail, b.top = true, b.top-1
		} else {
			b.sel = max(b.sel-1, 0)
		}
	case "down", "j", "ctrl+n":
		if atEnd {
			b.tail, b.top = true, b.top+1
		} else {
			b.sel = min(b.sel+1, last)
		}
	case "pgup", "b", "ctrl+u":
		if hidden {
			b.tail, b.top = true, max(b.top-page, b.sel)
		} else {
			b.sel = max(b.sel-page, 0)
		}
	case "pgdown", "space", "f", "ctrl+d":
		if atEnd {
			b.tail, b.top = true, b.top+page
		} else {
			b.sel = min(b.sel+page, last)
		}
	case "home", "g":
		b.sel = 0
	case "end", "G":
		b.sel = last
		b.tail, b.top = len(b.rows) > 0, 1<<30 // View clamps it to the end
	case "/":
		b.typing, b.input = true, ""
		if b.query == "" {
			b.mark = r // where esc or an emptied prompt returns to
		}
	case "enter", "l", "right":
		if !ok {
			break
		}
		switch {
		case r.kind == modRow:
			b.open(r)
		case k.String() == "enter":
			b.toggle(r)
		case b.folded(r):
			b.toggle(r)
		default: // l on an open heading steps into it
			b.sel = min(b.sel+1, last)
		}
	case "tab":
		if ok {
			b.toggle(r)
		}
	case "h", "left":
		if ok {
			b.collapse(r)
		}
	case "-":
		b.foldAll(true, r)
	case "+", "=":
		b.foldAll(false, r)
	}
	return b, nil
}

// prompt edits the / line, filtering as the player types.
func (b *Browser) prompt(k tea.KeyPressMsg) {
	switch k.String() {
	case "esc":
		b.typing = false
		b.applyFilter(b.query, b.mark)
	case "enter":
		b.typing = false
		b.query = b.input
		if b.query == "" {
			b.applyFilter("", b.mark)
		}
	case "backspace", "ctrl+h":
		if r := []rune(b.input); len(r) > 0 {
			b.input = string(r[:len(r)-1])
			b.applyFilter(b.input, b.mark)
		}
	case "ctrl+u":
		b.input = ""
		b.applyFilter("", b.mark)
	default:
		if k.Text != "" {
			b.input += k.Text
			b.applyFilter(b.input, b.mark)
		}
	}
}

func (b *Browser) folded(r row) bool {
	switch r.kind {
	case divRow:
		return b.divs[r.d].folded
	case chapRow:
		return b.divs[r.d].chaps[r.c].folded
	}
	return false
}

// toggle folds or unfolds the heading at r; on a module it folds the
// module's chapter. Folds mean nothing while a filter shows every match.
func (b *Browser) toggle(r row) {
	if b.active != "" {
		return
	}
	switch r.kind {
	case divRow:
		b.divs[r.d].folded = !b.divs[r.d].folded
	case chapRow:
		b.divs[r.d].chaps[r.c].folded = !b.divs[r.d].chaps[r.c].folded
	case modRow:
		b.divs[r.d].chaps[r.c].folded = true
		r = row{chapRow, r.d, r.c, -1}
	}
	b.refold(r)
}

// collapse is h: fold what the cursor is in, or step out to the heading
// above when it is already folded, as a file tree does.
func (b *Browser) collapse(r row) {
	if b.active != "" {
		return
	}
	switch {
	case r.kind == modRow:
		b.divs[r.d].chaps[r.c].folded = true
		r = row{chapRow, r.d, r.c, -1}
	case r.kind == chapRow && b.divs[r.d].chaps[r.c].folded:
		r = row{divRow, r.d, -1, -1}
	case r.kind == chapRow:
		b.divs[r.d].chaps[r.c].folded = true
	default:
		b.divs[r.d].folded = true
	}
	b.refold(r)
}

// foldAll folds every chapter (leaving divisions open, so the chapter list
// is an outline of the guide) or unfolds everything.
func (b *Browser) foldAll(fold bool, r row) {
	if b.active != "" {
		return
	}
	for di := range b.divs {
		if !fold {
			b.divs[di].folded = false
		}
		for ci := range b.divs[di].chaps {
			b.divs[di].chaps[ci].folded = fold
		}
	}
	if fold && r.kind == modRow {
		r = row{chapRow, r.d, r.c, -1}
	}
	b.refold(r)
}

// ---- view -------------------------------------------------------------------

const (
	manName   = "RTFM(7)"
	manTitle  = "USACO Guide · CC BY-NC-SA 4.0"
	modIndent = 7 // man's paragraph indent
	colGap    = 2

	// storyCredit is the story's sources, as the README credits them.
	storyCredit = "The story quotes Lewis Carroll's Alice's Adventures in Wonderland " +
		"(1865) and Through the Looking-Glass (1871), in the public domain, and " +
		"Structure and Interpretation of Computer Programs, 2nd ed., by Harold " +
		"Abelson and Gerald Jay Sussman with Julie Sussman (MIT Press, 1996), " +
		"licensed under CC BY-SA 4.0 (https://creativecommons.org/licenses/by-sa/4.0/)."

	// minTitle is the title room columns give way for: 30 cells show four
	// in five of the guide's module titles whole.
	minTitle = 30
	// maxTitle caps the title column; the longest titles (50 cells) would
	// push every other column off a 100-column terminal.
	maxTitle = 44
)

// columns sizes the module rows. Columns give way, least useful first,
// until the title has room to be read: the map's words ("4 core +2
// stretch" becomes "4+2"), then the frequency stars, the biome, the map
// size and last the capability, which is the progress the index exists to
// show.
type columns struct {
	title, biome, capw, size, stars int // cells; 0 drops the column
	compact                         bool
}

func (b *Browser) columns(w int) columns {
	var c columns
	var titleW, fullMap, compactMap int
	for _, e := range b.all {
		titleW = max(titleW, ansi.StringWidth(e.mod.Title))
		c.biome = max(c.biome, ansi.StringWidth(e.biome))
		if e.trial.Capability != "" {
			c.capw = max(c.capw, ansi.StringWidth(capCell(e.trial.Capability, false)))
		}
		fullMap = max(fullMap, ansi.StringWidth(mapLabel(e.trial.Map, false)))
		compactMap = max(compactMap, ansi.StringWidth(mapLabel(e.trial.Map, true)))
		if e.mod.Frequency >= 0 {
			c.stars = 4
		}
	}
	c.size = fullMap
	want := min(titleW, maxTitle)
	room := func() int {
		n := w - modIndent
		for _, x := range []int{c.biome, c.capw, c.size, c.stars} {
			if x > 0 {
				n -= x + colGap
			}
		}
		return n
	}
	steps := []func(){
		func() { c.size, c.compact = max(compactMap, len("MAP")), true },
		func() { c.stars = 0 },
		func() { c.biome = 0 },
		func() { c.size = 0 },
		func() { c.capw = 0 },
	}
	for _, step := range steps {
		if room() >= min(want, minTitle) {
			break
		}
		step()
	}
	c.title = max(min(room(), want), 1)
	return c
}

// width is the table's full width, for right-aligned heading notes.
func (c columns) width() int {
	n := modIndent + c.title
	for _, x := range []int{c.biome, c.capw, c.size, c.stars} {
		if x > 0 {
			n += colGap + x
		}
	}
	return n
}

// cells joins a row's cells at the column widths, dropping the cells of
// dropped columns.
func (c columns) cells(title, biome, capc, size, freq string) string {
	var s strings.Builder
	s.WriteString(strings.Repeat(" ", modIndent))
	s.WriteString(ui.Fit(title, c.title))
	for _, col := range []struct {
		w    int
		text string
	}{{c.biome, biome}, {c.capw, capc}, {c.size, size}, {c.stars, freq}} {
		if col.w > 0 {
			s.WriteString(strings.Repeat(" ", colGap) + ui.Fit(col.text, col.w))
		}
	}
	return strings.TrimRight(s.String(), " ")
}

func (c columns) header() string {
	label := func(long, short string, w int) string {
		if ansi.StringWidth(long) <= w {
			return long
		}
		return short
	}
	return ui.Muted.Render(c.cells("MODULE", "BIOME", label("CAPABILITY", "CAP", c.capw),
		"MAP", "FREQ"))
}

func (b *Browser) View(width, height int) string {
	if b.page != nil {
		return b.page.View(width, height)
	}
	w, h := max(width, 1), max(height, 1)
	cols := b.columns(w)
	rows := max(h-3, 0)
	if len(b.all) == 0 {
		rows++ // no column header
	}
	b.h = rows

	var body []string
	total := 0
	switch {
	case len(b.all) == 0:
		body = []string{"No manual entry for rtfm", "",
			ui.Muted.Render("The USACO Guide snapshot has no modules to list.")}
	case len(b.rows) == 0:
		body = []string{ui.Muted.Render("Pattern not found: " + b.active)}
	default:
		lines := b.lines(cols, w)
		total = len(lines)
		// Keep the selection on screen, unless the view was scrolled on
		// past the last row into the tail (see Update).
		if !b.tail {
			if b.sel < b.top {
				b.top = b.sel
			}
			if b.sel >= b.top+rows {
				b.top = b.sel - rows + 1
			}
		}
		b.top = max(min(b.top, len(lines)-rows), 0)
		body = lines[b.top:min(b.top+rows, len(lines))]
	}

	out := []string{manHeader(manName, manTitle, w)}
	if len(b.all) > 0 {
		out = append(out, cols.header())
	}
	return ui.Layout(w, h, append(out, body...), b.bar(w, total))
}

// lines renders every visible row, then the colophon; row i is line i.
func (b *Browser) lines(cols columns, w int) []string {
	var out []string
	q := b.active
	for i, r := range b.rows {
		var line string
		switch r.kind {
		case divRow:
			line = b.divLine(r, cols, w)
		case chapRow:
			line = b.chapLine(r)
		case modRow:
			e := b.entry(r)
			title := e.mod.Title
			if i != b.sel && ansi.StringWidth(title) <= cols.title {
				title = highlight(title, q)
			}
			size := mapLabel(e.trial.Map, cols.compact)
			if !e.trial.Kind.HasTrial() {
				// A page has no map to play (see Module.Update).
				size = ui.Muted.Render("-")
			}
			line = cols.cells(title, e.biome, capCell(e.trial.Capability, b.caps[e.trial.Capability]),
				size, stars(e.mod.Frequency))
			if b.locked(e.mod.ID) {
				line = ui.Muted.Render(ansi.Strip(line))
			}
		}
		if i == b.sel {
			line = ui.Select.Render(ui.Fit(ansi.Strip(line), w))
		}
		out = append(out, line)
	}
	out = append(out, "", ui.Strong.Render("COLOPHON"))
	for _, l := range wrap(curriculum.Attribution(), modIndent, min(w, 80)) {
		out = append(out, ui.Muted.Render(l))
	}
	// The story's sources (docs/story/story-bible.md, "Sources and
	// licenses"), next to the guide's.
	for _, l := range wrap(storyCredit, modIndent, min(w, 80)) {
		out = append(out, ui.Muted.Render(l))
	}
	// The game's own credits (music, licenses, sources) are a GRUB key.
	for _, l := range wrap("Music, licenses and sources: press c in the GRUB menu.", modIndent, min(w, 80)) {
		out = append(out, ui.Muted.Render(l))
	}
	out = append(out, "", ui.Spread("katharsis", manName, min(cols.width(), w)))
	return out
}

func (b *Browser) divLine(r row, cols columns, w int) string {
	d := b.divs[r.d]
	var es []entry
	for _, c := range d.chaps {
		es = append(es, c.mods...)
	}
	note := units.Plural(len(es), "module")
	if held, total := distinctCaps(es, b.caps); total > 0 {
		note += fmt.Sprintf("%s%s %d/%d", ui.G.Sep, ui.G.Cap, held, total)
	}
	head := foldMark(d.folded, b.active) + ui.Strong.Render(strings.ToUpper(d.title))
	return ui.Spread(head, ui.Muted.Render(note), min(max(cols.width(), 40), w))
}

func (b *Browser) chapLine(r row) string {
	c := b.divs[r.d].chaps[r.c]
	s := "   " + foldMark(c.folded, b.active) + c.title
	if c.folded && b.active == "" {
		s += ui.Muted.Render(" " + ui.G.Sep + units.Plural(len(c.mods), "module"))
	}
	return s
}

func foldMark(folded bool, filter string) string {
	if folded && filter == "" {
		return ui.Muted.Render(ui.G.FoldShut) + " "
	}
	return ui.Muted.Render(ui.G.FoldOpen) + " "
}

// bar is less's prompt line: where the page is, how many capabilities are
// held, and the keys; or the / prompt while typing. lines is the length of
// the page, colophon included.
func (b *Browser) bar(w, lines int) string {
	n := 0
	for _, r := range b.rows {
		if r.kind == modRow {
			n++
		}
	}
	// A locked module takes the left of the bar for why, and enter is not
	// offered.
	lock := b.lockNote()
	open := []string{"enter", "open"}
	if lock != "" {
		open = nil
	}
	switch {
	case b.typing:
		return ui.Spread("/"+b.input+"█", ui.Muted.Render(units.Plural(n, "match")), w)
	case b.query != "":
		left := "/" + b.query + ": " + units.Plural(n, "match")
		switch {
		case n == 0:
			left = "Pattern not found: /" + b.query
		case lock != "":
			left = lock
		}
		return ui.StatusBar(w, left, slices.Concat([]string{"esc", "clear"}, open, []string{"q", "quit"})...)
	}
	if len(b.all) == 0 {
		return ui.StatusBar(w, "Manual page rtfm(7) (press q to quit)")
	}
	held, total := distinctCaps(b.all, b.caps)
	pos := ui.PagerLine(b.top, b.h, lines)
	caps := fmt.Sprintf("%s %d/%d held", ui.G.Cap, held, total)
	keys := slices.Concat(open, []string{"tab", "fold", "/", "filter", "q", "quit"})
	lefts := []string{
		"Manual page rtfm(7) " + pos + ui.G.Sep + caps,
		"rtfm(7) " + pos + ui.G.Sep + caps,
		pos + ui.G.Sep + caps,
	}
	if lock != "" {
		lefts = []string{lock + ui.G.Sep + pos, lock}
	}
	for _, left := range lefts {
		if ui.StatusBarFits(w, left, keys...) {
			return ui.StatusBar(w, left, keys...)
		}
	}
	return ui.StatusBar(w, caps, keys...)
}
