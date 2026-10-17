package rtfm

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/ui"
)

// Level is what opens on a node of the RTFM map (.agents/work/rtfm-map-plan.md
// §3.4): one module in three panels, READ (the module in the reader),
// PRACTICE (its problems, each an untimed door through screens.Upsolve) and
// TRIAL (the module page's TRIAL section), then SEE ALSO, the shortcuts it
// opens and the module's CC BY-NC-SA attribution.
//
// The cursor moves over READ, PRACTICE's rows and TRIAL; enter acts on the
// row under it. Leaving is the host's: it takes q, esc, h, left and
// backspace before they reach the level, as Browser does for its page.
type Level struct {
	l      curriculum.Level
	region string
	e      entry
	page   *Module // the module page, for its TRIAL lines
	caps   map[string]bool
	opened bool
	prereq []string // prerequisites as display names, ✓ when passed
	warps  string   // what the level opens a shortcut to, "" for nothing
	items  []practiceItem
	solved int // of the playable items

	cur  int // 0 is READ, 1..len(items) PRACTICE's rows, then TRIAL
	ptop int // first PRACTICE row shown, when the list does not fit
}

// practiceItem is one PRACTICE row: a problem the game can judge, or one
// it cannot (external), which only opens its URL.
type practiceItem struct {
	id                   catalog.ID // "" for an external problem
	title, source, label string
	url                  string
	rating               int
	solved               bool
}

func (it practiceItem) playable() bool { return it.id != "" }

// NewLevel builds the level screen for l, a level of c's atlas in the
// region titled region.
func NewLevel(c *curriculum.Curriculum, l curriculum.Level, region string, m game.Meta, judgeable Judgeable, lookup screens.Lookup) *Level {
	mod, ok := curriculum.Module{}, false
	if c != nil {
		mod, ok = c.Module(l.Module)
	}
	if !ok {
		mod = curriculum.Module{ID: l.Module, Title: l.Title, Frequency: -1}
	}
	caps := capSet(m)
	e := newEntry(mod, judgeable)
	v := &Level{
		l: l, region: region, e: e, page: newModule(c, e, caps, slices.Contains(m.TrialsPassed, l.Module), lookup), caps: caps,
		opened: slices.Contains(m.Opened, l.Module),
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
	v.items, v.solved = practiceItems(mod, l.Playable, m, lookup)
	return v
}

// warpText names the modules ids opens a shortcut to, grouped by
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

// practiceItems lists mod's problems: the playable ones first, each once,
// marked solved when the player solved it anywhere, then the external
// ones, each once.
func practiceItems(mod curriculum.Module, playable []catalog.ID, m game.Meta, lookup screens.Lookup) (items []practiceItem, solved int) {
	done := map[catalog.ID]bool{}
	for _, s := range m.Solves {
		done[s.Problem] = true
	}
	var ext []practiceItem
	seen := map[string]bool{}
	for _, list := range mod.Problems {
		for _, r := range list.Problems {
			it := practiceItem{title: r.Name, source: r.Source, label: r.Difficulty, url: r.URL}
			key := r.UniqueID
			if slices.Contains(playable, r.ID) {
				key = string(r.ID)
				it.id, it.solved = r.ID, done[r.ID]
				if lookup != nil {
					if p, ok := lookup(r.ID); ok {
						it.title = cmp.Or(p.Title, it.title)
						it.rating = p.Rating
					}
				}
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			switch {
			case it.playable():
				items = append(items, it)
				if it.solved {
					solved++
				}
			default:
				ext = append(ext, it)
			}
		}
	}
	return append(items, ext...), solved
}

// hasTrial reports whether the level shows a TRIAL panel: trial levels and
// castles do, a page does not.
func (v *Level) hasTrial() bool { return v.l.Kind != curriculum.PageLevel }

// last is the cursor's last position.
func (v *Level) last() int {
	if v.hasTrial() {
		return len(v.items) + 1
	}
	return len(v.items)
}

// item is the PRACTICE row under the cursor.
func (v *Level) item() (practiceItem, bool) {
	if v.cur >= 1 && v.cur <= len(v.items) {
		return v.items[v.cur-1], true
	}
	return practiceItem{}, false
}

// Practice puts the cursor on PRACTICE's first row.
func (v *Level) Practice() {
	if len(v.items) > 0 {
		v.cur = 1
	}
}

func (v *Level) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return v, nil
	}
	id := v.l.Module
	switch k.String() {
	case "up", "k":
		v.cur = max(v.cur-1, 0)
	case "down", "j":
		v.cur = min(v.cur+1, v.last())
	case "home", "g":
		v.cur = 0
	case "end", "G":
		v.cur = v.last()
	case "p":
		v.Practice()
	case "r":
		return v, screens.Emit(screens.OpenModule{Module: id})
	case "t":
		if v.hasTrial() {
			return v, screens.Emit(screens.StartTrial{Module: id})
		}
	case "o":
		if it, ok := v.item(); ok && !it.playable() && it.url != "" {
			return v, screens.Emit(reader.OpenURL{URL: it.url})
		}
		if u := v.e.mod.URL; u != "" {
			return v, screens.Emit(reader.OpenURL{URL: u})
		}
	case "enter":
		return v, v.act()
	}
	return v, nil
}

// act is enter on the row under the cursor.
func (v *Level) act() tea.Cmd {
	if v.cur == 0 {
		return screens.Emit(screens.OpenModule{Module: v.l.Module})
	}
	it, ok := v.item()
	switch {
	case !ok:
		return screens.Emit(screens.StartTrial{Module: v.l.Module})
	case it.playable():
		return screens.Emit(screens.Upsolve{Problem: it.id})
	case it.url != "":
		return screens.Emit(reader.OpenURL{URL: it.url})
	}
	return nil
}

// labelW is the panels' label column ("PRACTICE  "), after the cursor's
// three cells; a panel's content starts at contentCol.
const (
	labelW     = 10
	contentCol = 3 + labelW
)

func (v *Level) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	m := v.e.mod
	crumb := strings.Join([]string{ui.Strong.Render("RTFM"), m.DivisionTitle, v.region, v.l.Title},
		ui.Muted.Render(ui.G.Crumb))
	head := []string{ui.Spread(crumb, capCell(v.l.Capability, v.caps[v.l.Capability]), w), ui.Rule(w)}
	name := " " + ui.Strong.Render(v.l.Title)
	if v.l.Kind == curriculum.CastleLevel {
		// A conclusion is the world's gauntlet: the boss door's glyph.
		name = " " + ui.Special.Render(ui.G.Boss) + name
	}
	if f := m.Frequency; f >= 0 {
		name += "  " + stars(f) + " " + ui.Muted.Render(frequencyText(f))
	}
	if v.hasTrial() {
		t := v.e.trial
		name += ui.Muted.Render(ui.G.Sep) + ui.RatingBand(t.IntegBand[0], t.IntegBand[1]) +
			ui.Muted.Render(ui.G.Sep) + v.e.biome
	}
	head = append(head, name)
	var desc []string
	for _, l := range ui.WrapN(m.Description, max(w-2, 10), 2) {
		desc = append(desc, " "+l)
	}

	read := []string{v.row(0, "READ", ui.Spread("the module, in the reader", v.openedWord(), w-contentCol))}
	trial := v.trialLines(w)
	foot := v.footLines(w)

	// PRACTICE gets the rows left over, its label row included; then the
	// description and the blank lines that set the panels apart, in that
	// order, take what remains.
	rows := max(h-1, 0) // the status bar
	fixed := len(head) + len(read) + len(trial) + len(foot)
	want := 1 + len(v.items)
	prows := max(min(want, rows-fixed), 2) - 1
	spare := rows - fixed - 1 - prows
	practice := v.practiceLines(w, prows)

	body := slices.Clone(head)
	if spare > 0 && len(desc) > 0 && spare >= len(desc)+1 {
		body = append(body, desc...)
		spare -= len(desc)
	}
	if spare > 0 {
		body = append(body, "")
		spare--
	}
	body = append(body, read...)
	body = append(body, practice...)
	body = append(body, trial...)
	for ; spare > 0; spare-- {
		body = append(body, "")
	}
	body = append(body, foot...)
	return ui.Layout(w, h, body, v.statusBar(w))
}

// openedWord is READ's state: "opened" once the page was opened in the
// reader, else nothing (no "not read yet": plan GR8, GR10).
func (v *Level) openedWord() string {
	if v.opened {
		return ui.Muted.Render("opened")
	}
	return ""
}

// row is a panel's line: the cursor's cell when at is under it, then the
// label and the content.
func (v *Level) row(at int, label, content string) string {
	cur := " "
	if v.cur == at {
		cur = ui.Focus.Render(ui.G.Cursor)
	}
	if label != "" {
		label = ui.Strong.Render(label)
	}
	return " " + cur + " " + ui.Fit(label, labelW) + content
}

// practiceLines is PRACTICE in n rows besides its label row: as many
// problems as fit, scrolled to keep the cursor on screen, "+N above" and
// "+N more" saying what does not show.
func (v *Level) practiceLines(w, n int) []string {
	playable := 0
	for _, it := range v.items {
		if it.playable() {
			playable++
		}
	}
	var head string
	switch {
	case playable > 0:
		head = fmt.Sprintf("%d/%d solved", v.solved, playable)
	case len(v.items) > 0:
		head = ui.Muted.Render("none the game can judge: o opens one")
	default:
		head = ui.Muted.Render("no problems listed")
	}
	out := []string{v.row(-1, "PRACTICE", head)}
	if len(v.items) == 0 {
		return out
	}
	n = min(n, len(v.items))
	if i := v.cur - 1; i >= 0 && i < len(v.items) {
		v.ptop = min(v.ptop, i)
		v.ptop = max(v.ptop, i-n+1)
	}
	v.ptop = max(min(v.ptop, len(v.items)-n), 0)

	const srcW, diffW, ratingW = 7, 9, 5
	titleW := 0
	for _, it := range v.items {
		titleW = max(titleW, ansi.StringWidth(it.title))
	}
	more := 0
	if n < len(v.items) {
		more = len(" +999 above")
	}
	fixed := contentCol + 2 + colGap + srcW + colGap + diffW + colGap + ratingW
	titleW = max(min(titleW, w-fixed-more), 8)
	for i := v.ptop; i < v.ptop+n; i++ {
		it := v.items[i]
		mark := " "
		if it.solved {
			mark = ui.OK.Render(ui.G.Pass)
		}
		rating := ""
		switch {
		case !it.playable():
			rating = "link"
		case it.rating > 0:
			rating = ui.Rating(it.rating)
		}
		line := mark + " " + ui.Fit(it.title, titleW) + strings.Repeat(" ", colGap) +
			ui.Fit(it.source, srcW) + strings.Repeat(" ", colGap) + ui.Fit(it.label, diffW) +
			strings.Repeat(" ", colGap) + ui.AlignRight(rating, ratingW)
		if !it.playable() {
			line = ui.Muted.Render(line)
		}
		switch {
		case i == v.ptop+n-1 && i < len(v.items)-1:
			line += ui.Muted.Render(fmt.Sprintf("  +%d more", len(v.items)-1-i))
		case i == v.ptop && i > 0:
			line += ui.Muted.Render(fmt.Sprintf("  +%d above", i))
		}
		out = append(out, strings.TrimRight(v.row(i+1, "", line), " "))
	}
	return out
}

// trialLines is TRIAL as the module page sets it (module.go): the unit
// doors, any the catalog tops up, the blind integration doors and what
// passing grants. A page has none, but one on a gated topic says, on a
// row the cursor never reaches, that the capability is earned elsewhere.
func (v *Level) trialLines(w int) []string {
	if !v.hasTrial() {
		if v.e.trial.Capability == "" {
			return nil
		}
		return []string{v.row(v.last()+1, "TRIAL", v.page.passLine())}
	}
	ind := strings.Repeat(" ", modIndent)
	var lines []string
	for _, l := range v.page.unitLines(w - (contentCol - modIndent)) {
		lines = append(lines, strings.TrimPrefix(l, ind))
	}
	t := v.e.trial
	where := v.e.biome + ui.G.Sep + ui.RatingBand(t.IntegBand[0], t.IntegBand[1])
	if n := v.e.short(); n > 0 {
		lines = append(lines, fmt.Sprintf("+  %d topped up from the catalog%s%s", n, ui.G.Sep, where))
	}
	lines = append(lines, fmt.Sprintf("+  %d blind integration doors%s%s", integrationDoors, ui.G.Sep, where))
	lines = append(lines, v.page.passLine())

	at := v.last()
	out := []string{v.row(at, "TRIAL", lines[0])}
	for _, l := range lines[1:] {
		out = append(out, strings.Repeat(" ", contentCol)+l)
	}
	return out
}

// footLines is the level's foot: the shortcuts it opens, SEE ALSO (its
// prerequisites, ✓ for each whose trial is passed) and the module's
// attribution, which CC BY-NC-SA requires, as the module page prints it.
func (v *Level) footLines(w int) []string {
	text := min(w, 80)
	var out []string
	if v.warps != "" {
		out = append(out, ui.Clip(" "+ui.G.Warp+" also opens: "+v.warps, w))
	}
	req := "nothing"
	if len(v.prereq) > 0 {
		req = strings.Join(v.prereq, ui.Muted.Render(ui.G.Sep))
	}
	see := wrap(ui.Muted.Render("needs ")+req, contentCol, w)
	see[0] = " " + ui.Fit(ui.Strong.Render("SEE ALSO"), contentCol-1) + strings.TrimLeft(see[0], " ")
	out = append(out, see...)
	for _, l := range wrap(v.e.mod.Attribution(), 0, text) {
		out = append(out, ui.Muted.Render(l))
	}
	return out
}

// statusBar is the pager's bar, as on the module page: the level on the
// left, its keys on the right.
func (v *Level) statusBar(w int) string {
	keys := []string{"j/k", "move", "enter", "go", "r", "read", "p", "practice"}
	if v.hasTrial() {
		keys = append(keys, "t", "trial")
	}
	if it, ok := v.item(); ok && !it.playable() && it.url != "" {
		keys = append(keys, "o", "open problem")
	} else if v.e.mod.URL != "" {
		keys = append(keys, "o", "usaco.guide")
	}
	keys = append(keys, "q", "map")
	return ui.StatusBar(w, v.l.Module+"(7)", keys...)
}
