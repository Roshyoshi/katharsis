package rtfm

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/ui"
)

// Module is one module's page, set as a man page: NAME (id, title, what it
// is about, a faint story line), TRIAL (the unit doors in the order a trial
// boot plays them, any the catalog tops up, the blind integration doors, and
// what passing grants, or why a page has no trial), SEE ALSO (prerequisites,
// authors), then the module's CC BY-NC-SA attribution. r reads the module, t
// starts its trial (any level but a page), o opens it on usaco.guide.
type Module struct {
	e      entry
	prereq []string // prerequisites as display names
	caps   map[string]bool
	passed bool // its trial is passed (Meta.TrialsPassed)
	lookup screens.Lookup

	top int // first body line shown, when the body does not fit
	h   int // body rows at the last View
}

// NewModule builds the page for mod. c resolves prerequisite ids to titles
// and may be nil; judgeable and lookup are as for NewBrowser.
func NewModule(c *curriculum.Curriculum, mod curriculum.Module, m game.Meta, judgeable Judgeable, lookup screens.Lookup) *Module {
	return newModule(c, newEntry(mod, judgeable), capSet(m), slices.Contains(m.TrialsPassed, mod.ID), lookup)
}

func newModule(c *curriculum.Curriculum, e entry, caps map[string]bool, passed bool, lookup screens.Lookup) *Module {
	p := &Module{e: e, caps: caps, passed: passed, lookup: lookup}
	for _, id := range e.mod.Prerequisites {
		name := id
		if c != nil {
			if pm, ok := c.Module(id); ok {
				name = pm.Title + " (" + pm.DivisionTitle + ")"
			}
		}
		p.prereq = append(p.prereq, name)
	}
	return p
}

func (p *Module) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	id := p.e.mod.ID
	switch k.String() {
	case "q", "esc", "h", "left", "backspace":
		return p, screens.Emit(screens.Back{})
	case "r", "enter":
		return p, screens.Emit(screens.OpenModule{Module: id})
	case "t":
		// A page has no trial; the app would refuse one.
		if p.e.trial.Kind.HasTrial() {
			return p, screens.Emit(screens.StartTrial{Module: id})
		}
	case "o":
		if p.e.mod.URL != "" {
			return p, screens.Emit(reader.OpenURL{URL: p.e.mod.URL})
		}
	case "up", "k":
		p.top = max(p.top-1, 0)
	case "down", "j":
		p.top++
	case "pgup", "b", "ctrl+u":
		p.top = max(p.top-max(p.h-1, 1), 0)
	case "pgdown", "space", "f", "ctrl+d":
		p.top += max(p.h-1, 1)
	case "home", "g":
		p.top = 0
	case "end", "G":
		p.top = 1 << 30 // View clamps it
	}
	return p, nil
}

func (p *Module) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	m := p.e.mod
	crumb := strings.Join([]string{ui.Strong.Render("RTFM"), m.DivisionTitle, m.Chapter}, ui.Muted.Render(ui.G.Crumb))
	head := ui.Spread(crumb, capCell(p.e.trial.Capability, p.caps[p.e.trial.Capability]), w)

	body := p.body(w)
	rows := max(h-3, 0)
	p.h = rows
	p.top = max(min(p.top, len(body)-rows), 0)
	shown := body[p.top:min(p.top+rows, len(body))]

	return ui.Layout(w, h, append([]string{head, ""}, shown...), p.footer(w, len(body), rows))
}

func (p *Module) body(w int) []string {
	m := p.e.mod
	ind := strings.Repeat(" ", modIndent)
	text := min(w, 80) // prose measure; the trial table may use more

	var out []string
	out = append(out, ui.Strong.Render("NAME"))
	name := ind + m.ID + " - " + ui.Strong.Render(m.Title)
	if f := m.Frequency; f >= 0 {
		name = ui.Spread(name, stars(f)+" "+ui.Muted.Render(frequencyText(f)), text)
	}
	out = append(out, name)
	if m.Description != "" {
		out = append(out, wrap(m.Description, modIndent, text)...)
	}
	// The Mock Turtle's school has an opinion on every subject, faint
	// under the module's own words (rtfm.module.intro).
	for _, l := range wrap(introLine(m), modIndent, text) {
		out = append(out, ui.Voice.Render(ui.Muted.Render(l)))
	}

	t := p.e.trial
	if !t.Kind.HasTrial() {
		out = append(out, "", ui.Strong.Render("TRIAL"), ind+p.passLine())
		return append(out, p.seeAlso(text)...)
	}
	out = append(out, "", ui.Spread(ui.Strong.Render("TRIAL"),
		ui.Muted.Render("unit: Easy → Hard, tags shown · integration: blind"), text))
	out = append(out, p.unitLines(w)...)
	where := p.e.biome + ui.G.Sep + ui.RatingBand(t.IntegBand[0], t.IntegBand[1])
	if n := p.e.short(); n > 0 {
		out = append(out, ind+fmt.Sprintf("+  %d topped up from the catalog · %s", n, where))
	}
	out = append(out, ind+fmt.Sprintf("+  %d blind integration doors · %s", integrationDoors, where))
	out = append(out, ind+p.passLine())
	return append(out, p.seeAlso(text)...)
}

// seeAlso is the page's tail: SEE ALSO, then the module's attribution,
// text cells wide.
func (p *Module) seeAlso(text int) []string {
	m := p.e.mod
	out := []string{"", ui.Strong.Render("SEE ALSO")}
	req := "nothing"
	if len(p.prereq) > 0 {
		req = strings.Join(p.prereq, ", ")
	}
	out = append(out, facts("requires", req, text)...)
	if len(m.Authors) > 0 {
		out = append(out, facts("authors", strings.Join(m.Authors, ", "), text)...)
	}

	out = append(out, "")
	for _, l := range wrap(m.Attribution(), 0, text) {
		out = append(out, ui.Muted.Render(l))
	}
	return out
}

// introLine is the module's line from the Mock Turtle's school, the same
// for a module every time: the pool's pick is seeded by the module's id.
// {module} is the title, cut to text.md's 24 cells; "" when the module has
// no name to put there.
func introLine(m curriculum.Module) string {
	name := cmp.Or(m.Title, m.ID)
	if name == "" {
		return ""
	}
	h := fnv.New64a()
	h.Write([]byte(m.ID))
	return flavor.Fill(flavor.Story("rtfm.module.intro", h.Sum64()), "module", ui.Clip(name, 24))
}

// facts is one "label  value" line of SEE ALSO, the value wrapped under
// itself.
func facts(label, value string, w int) []string {
	const labelW = 10
	lines := wrap(value, modIndent+labelW, w)
	lines[0] = strings.Repeat(" ", modIndent) + ui.Fit(label, labelW) + strings.TrimLeft(lines[0], " ")
	return lines
}

// unitLines lists the unit doors: number, title, the guide's difficulty
// label (relative to the module), the catalog rating when known, and the
// contest.
func (p *Module) unitLines(w int) []string {
	type door struct {
		title, label, contest string
		rating                int
	}
	var ds []door
	titleW := 0
	for _, id := range p.e.trial.Unit {
		d := door{title: string(id)}
		if ref, ok := p.ref(id); ok {
			d.title, d.label = ref.Name, ref.Difficulty
		}
		if p.lookup != nil {
			if pr, ok := p.lookup(id); ok {
				if pr.Title != "" {
					d.title = pr.Title
				}
				d.contest = pr.Contest
				if catalog.ShowsRating(id) {
					d.rating = pr.Rating
				}
			}
		}
		ds = append(ds, d)
		titleW = max(titleW, ansi.StringWidth(d.title))
	}
	const labelW, ratingW = 9, 5 // ratingW fits "◆3500"
	fixed := modIndent + 3 + colGap + labelW + colGap + ratingW
	titleW = max(min(titleW, w-fixed-colGap), 8)
	var out []string
	for i, d := range ds {
		rating := ""
		if d.rating > 0 {
			rating = ui.Rating(d.rating)
		}
		line := strings.Repeat(" ", modIndent) + ui.Fit(fmt.Sprint(i+1), 3) + ui.Fit(d.title, titleW) +
			strings.Repeat(" ", colGap) + ui.Fit(d.label, labelW) +
			strings.Repeat(" ", colGap) + ui.AlignRight(rating, ratingW)
		if d.contest != "" {
			line += strings.Repeat(" ", colGap) + ui.Muted.Render(d.contest)
		}
		out = append(out, strings.TrimRight(line, " "))
	}
	return out
}

// ref finds a unit problem's entry in the module's lists.
func (p *Module) ref(id catalog.ID) (curriculum.ProblemRef, bool) {
	for _, l := range p.e.mod.Problems {
		for _, r := range l.Problems {
			if r.ID == id {
				return r, true
			}
		}
	}
	return curriculum.ProblemRef{}, false
}

// passLine says what passing the trial grants, or for a page why it has no
// trial. A trial that grants no capability still clears its level: ✓.
func (p *Module) passLine() string {
	t := p.e.trial
	c := t.Capability
	switch {
	case !t.Kind.HasTrial() && p.e.mod.Division == "general":
		return ui.Muted.Render("·  no trial: " + p.e.mod.DivisionTitle + " is reading only")
	case !t.Kind.HasTrial() && c != "":
		return ui.Muted.Render("·  no trial of its own: no problem here the game can judge; " + c + " is earned in another module on " + p.e.biome)
	case !t.Kind.HasTrial():
		return ui.Muted.Render("·  no trial: no capability to grant, no problem the game can judge")
	case c == "" && p.passed:
		return ui.OK.Render(ui.G.Pass+"  passed") + ": grants no capability, " + p.e.biome + " is open to every boot"
	case c == "":
		return ui.G.Ahead + "  pass " + ui.G.Arrow + " " + ui.OK.Render(ui.G.Pass+" only") +
			": grants no capability, " + p.e.biome + " is open to every boot"
	case p.caps[c]:
		return ui.OK.Render(ui.G.Cap+"  "+c+" held") + ": " + p.e.biome + " is already on your routes"
	}
	return ui.G.CapLocked + "  pass " + ui.G.Arrow + " " + ui.OK.Render(ui.G.Cap+" "+c) + ": " + p.e.biome + " joins your route choices"
}

// footer is the pager's status bar, as on the index: the page and the line
// on the left, the page's keys on the right. The page name goes first when
// the bar is short.
func (p *Module) footer(w, lines, rows int) string {
	keys := []string{"r", "read"}
	if p.e.trial.Kind.HasTrial() {
		keys = append(keys, "t", "start trial")
	}
	if p.e.mod.URL != "" {
		keys = append(keys, "o", "usaco.guide")
	}
	keys = append(keys, "q", "back")
	pos := ui.PagerLine(p.top, rows, lines)
	if page := p.e.mod.ID + "(7) " + pos; ui.StatusBarFits(w, page, keys...) {
		pos = page
	}
	return ui.StatusBar(w, pos, keys...)
}
