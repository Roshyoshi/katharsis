package rtfm

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/screens"
)

// Browser is the RTFM index: the guide's divisions, chapters and modules in
// guide order, set like a man page (section headings at the margin, entries
// indented under them) with less's prompt line at the bottom. Chapters and
// divisions fold; / filters modules by title or id. Enter on a module opens
// its page in place, and q or h there returns to the index, so the app
// keeps one screen on its stack for all of learn mode.
type Browser struct {
	divs   []division
	all    []entry // every module, for column widths and the totals
	cur    *curriculum.Curriculum
	caps   map[string]bool
	lookup screens.Lookup

	rows     []row // what is visible: folds and the filter applied
	sel, top int   // sel indexes rows; top is the first visible line
	h        int   // list rows at the last View, for paging

	typing bool
	input  string // what has been typed at the / prompt
	query  string // the committed filter
	active string // the filter rows were built with: input while typing
	mark   row    // the selection before filtering began

	page *Module // the open module page; nil shows the index
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
// judgeable picks each module's trial unit (see curriculum.TrialFor) and
// lookup resolves problem titles and ratings for module pages, which show
// the guide's own name for a problem it misses; either may be nil. A nil
// curriculum shows man's "No manual entry".
func NewBrowser(c *curriculum.Curriculum, m game.Meta, judgeable Judgeable, lookup screens.Lookup) *Browser {
	b := &Browser{cur: c, caps: capSet(m), lookup: lookup}
	if c != nil {
		for _, d := range c.Divisions {
			dv := division{title: d.Title}
			for _, ch := range d.Chapters {
				cv := chapter{title: ch.Title}
				for _, ref := range ch.Modules {
					mod, ok := c.Module(ref.ID)
					if !ok {
						continue
					}
					e := newEntry(mod, judgeable)
					cv.mods = append(cv.mods, e)
					b.all = append(b.all, e)
				}
				dv.chaps = append(dv.chaps, cv)
			}
			b.divs = append(b.divs, dv)
		}
	}
	b.build()
	b.sel = b.firstModule()
	return b
}

// Refresh swaps in the player's Meta (a passed trial grants a capability),
// keeping folds, filter, selection and an open module page.
func (b *Browser) Refresh(m game.Meta) {
	b.caps = capSet(m)
	if b.page != nil {
		b.page.caps = b.caps
	}
}

// Typing reports whether the / prompt is open. The prompt wants every
// printable key for itself, so an app with global keys should pass them
// through while it is.
func (b *Browser) Typing() bool { return b.typing }

// Open shows the page of the module with id, as enter on its row would,
// unfolding and selecting the row so q returns to it. It reports whether
// the curriculum has the module.
func (b *Browser) Open(id string) bool {
	for di, d := range b.divs {
		for ci, c := range d.chaps {
			for mi, e := range c.mods {
				if e.mod.ID == id {
					b.typing, b.query = false, ""
					b.applyFilter("", row{modRow, di, ci, mi})
					b.open(row{modRow, di, ci, mi})
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

func (b *Browser) open(r row) {
	b.page = newModule(b.cur, b.entry(r), b.caps, b.lookup)
}

func (b *Browser) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, isKey := msg.(tea.KeyPressMsg)
	if b.page != nil {
		if isKey {
			switch k.String() {
			case "q", "esc", "h", "left", "backspace":
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
	if b.typing {
		b.prompt(k)
		return b, nil
	}
	last := max(len(b.rows)-1, 0)
	page := max(b.h-1, 1)
	r, ok := b.selected()
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
		b.sel = max(b.sel-1, 0)
	case "down", "j", "ctrl+n":
		b.sel = min(b.sel+1, last)
	case "pgup", "b", "ctrl+u":
		b.sel = max(b.sel-page, 0)
	case "pgdown", "space", "f", "ctrl+d":
		b.sel = min(b.sel+page, last)
	case "home", "g":
		b.sel = 0
	case "end", "G":
		b.sel = last
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

	// minTitle is the title room columns give way for: 30 cells show four
	// in five of the guide's module titles whole.
	minTitle = 30
	// maxTitle caps the title column; the longest titles (50 cells) would
	// push every other column off a 100-column terminal.
	maxTitle = 44
)

// columns sizes the module rows. Columns give way, least useful first,
// until the title has room to be read: the trial's words ("2+2 topped up"
// becomes "2+2"), then the frequency stars, the biome, the trial size and
// last the capability, which is the progress the index exists to show.
type columns struct {
	title, biome, capw, unit, stars int // cells; 0 drops the column
	compact                         bool
}

func (b *Browser) columns(w int) columns {
	var c columns
	var titleW, fullUnit, compactUnit int
	for _, e := range b.all {
		titleW = max(titleW, ansi.StringWidth(e.mod.Title))
		c.biome = max(c.biome, ansi.StringWidth(e.biome))
		if e.trial.Capability != "" {
			c.capw = max(c.capw, ansi.StringWidth(capCell(e.trial.Capability, false)))
		}
		fullUnit = max(fullUnit, ansi.StringWidth(unitLabel(len(e.trial.Unit), e.short(), false)))
		compactUnit = max(compactUnit, ansi.StringWidth(unitLabel(len(e.trial.Unit), e.short(), true)))
		if e.mod.Frequency >= 0 {
			c.stars = 4
		}
	}
	c.unit = fullUnit
	want := min(titleW, maxTitle)
	room := func() int {
		n := w - modIndent
		for _, x := range []int{c.biome, c.capw, c.unit, c.stars} {
			if x > 0 {
				n -= x + colGap
			}
		}
		return n
	}
	steps := []func(){
		func() { c.unit, c.compact = max(compactUnit, len("TRIAL")), true },
		func() { c.stars = 0 },
		func() { c.biome = 0 },
		func() { c.unit = 0 },
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
	for _, x := range []int{c.biome, c.capw, c.unit, c.stars} {
		if x > 0 {
			n += colGap + x
		}
	}
	return n
}

// cells joins a row's cells at the column widths, dropping the cells of
// dropped columns.
func (c columns) cells(title, biome, capc, unit, freq string) string {
	var s strings.Builder
	s.WriteString(strings.Repeat(" ", modIndent))
	s.WriteString(pad(title, c.title))
	for _, col := range []struct {
		w    int
		text string
	}{{c.biome, biome}, {c.capw, capc}, {c.unit, unit}, {c.stars, freq}} {
		if col.w > 0 {
			s.WriteString(strings.Repeat(" ", colGap) + pad(col.text, col.w))
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
	return faint.Render(c.cells("MODULE", "BIOME", label("CAPABILITY", "CAP", c.capw),
		"TRIAL", "FREQ"))
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
			faint.Render("The USACO Guide snapshot has no modules to list.")}
	case len(b.rows) == 0:
		body = []string{faint.Render("Pattern not found: " + b.active)}
	default:
		lines := b.lines(cols, w)
		total = len(lines)
		// Keep the selection on screen; on the last row show the end of
		// the page, so G reaches the colophon as it does in man.
		if b.sel == len(b.rows)-1 {
			b.top = len(lines) - rows
		}
		if b.sel < b.top {
			b.top = b.sel
		}
		if b.sel >= b.top+rows {
			b.top = b.sel - rows + 1
		}
		b.top = max(min(b.top, len(lines)-rows), 0)
		body = lines[b.top:min(b.top+rows, len(lines))]
	}

	out := []string{manHeader(manName, manTitle, w)}
	if len(b.all) > 0 {
		out = append(out, cols.header())
	}
	out = append(out, body...)
	for len(out) < h-1 {
		out = append(out, "")
	}
	out = append(out[:h-1], b.bar(w, total))
	return frame(out, w, h)
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
			unit := unitLabel(len(e.trial.Unit), e.short(), cols.compact)
			if e.mod.Topic == "" {
				// No topic: the "trial" is Userland filler, not a test of
				// the module, so its size is not worth the eye.
				unit = faint.Render(ansi.Strip(unit))
			}
			line = cols.cells(title, e.biome, capCell(e.trial.Capability, b.caps[e.trial.Capability]),
				unit, stars(e.mod.Frequency))
		}
		if i == b.sel {
			line = reverse.Render(pad(ansi.Strip(line), w))
		}
		out = append(out, line)
	}
	out = append(out, "", bold.Render("COLOPHON"))
	for _, l := range wrap(curriculum.Attribution(), modIndent, min(w, 80)) {
		out = append(out, faint.Render(l))
	}
	out = append(out, "", spread("katharsis", manName, min(cols.width(), w)))
	return out
}

func (b *Browser) divLine(r row, cols columns, w int) string {
	d := b.divs[r.d]
	var es []entry
	for _, c := range d.chaps {
		es = append(es, c.mods...)
	}
	note := plural(len(es), "module")
	if held, total := distinctCaps(es, b.caps); total > 0 {
		note += fmt.Sprintf(" · %s %d/%d", heldMark, held, total)
	}
	head := foldMark(d.folded, b.active) + bold.Render(strings.ToUpper(d.title))
	return spread(head, faint.Render(note), min(max(cols.width(), 40), w))
}

func (b *Browser) chapLine(r row) string {
	c := b.divs[r.d].chaps[r.c]
	s := "   " + foldMark(c.folded, b.active) + c.title
	if c.folded && b.active == "" {
		s += faint.Render("  · " + plural(len(c.mods), "module"))
	}
	return s
}

func foldMark(folded bool, filter string) string {
	if folded && filter == "" {
		return "▸ "
	}
	return "▾ "
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
	switch {
	case b.typing:
		return spread("/"+b.input+"█", faint.Render(plural(n, "match")), w)
	case b.query != "":
		left := " /" + b.query + ": " + plural(n, "match")
		if n == 0 {
			left = " Pattern not found: /" + b.query
		}
		return reverse.Render(pad(spread(left, "esc clear · enter open · q quit ", w), w))
	}
	if len(b.all) == 0 {
		return reverse.Render(pad(" Manual page rtfm(7) (press q to quit)", w))
	}
	held, total := distinctCaps(b.all, b.caps)
	lines = max(lines, 1)
	pos := fmt.Sprintf("line %d/%d", min(b.top+1, lines), lines)
	caps := fmt.Sprintf("%s %d/%d held", heldMark, held, total)
	keys := "enter open · tab fold · / filter · q quit "
	for _, left := range []string{
		" Manual page rtfm(7) " + pos + " · " + caps,
		" rtfm(7) " + pos + " · " + caps,
		" " + caps,
	} {
		if ansi.StringWidth(left)+1+ansi.StringWidth(keys) <= w {
			return reverse.Render(spread(left, keys, w))
		}
	}
	return reverse.Render(pad(" "+caps+" · / filter · q quit", w))
}
