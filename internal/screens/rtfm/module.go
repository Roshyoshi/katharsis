package rtfm

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/curriculum"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Module is one module's page, set as a man page: NAME (id, title, what it
// is about, a faint story line), TRIAL (what opens it, its unseen doors and
// their aims, what passing takes and grants, or why a page has no trial),
// SEE ALSO (prerequisites, authors), then the module's CC BY-NC-SA
// attribution. r reads the module, t starts its trial (any level but a
// page), o opens it on usaco.guide.
type Module struct {
	e      entry
	prereq []string // prerequisites as display names
	caps   map[string]bool
	passed bool // its trial is passed (Meta.TrialsPassed)

	top int // first body line shown, when the body does not fit
	h   int // body rows at the last View
}

// NewModule builds the page for mod. c resolves prerequisite ids to titles
// and may be nil; judgeable and lookup are as for NewBrowser.
func NewModule(c *curriculum.Curriculum, mod curriculum.Module, m game.Meta, judgeable Judgeable, lookup screens.Lookup) *Module {
	return newModule(c, newEntry(mod, judgeable, lookup), capSet(m), slices.Contains(m.TrialsPassed, mod.ID))
}

func newModule(c *curriculum.Curriculum, e entry, caps map[string]bool, passed bool) *Module {
	p := &Module{e: e, caps: caps, passed: passed}
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
		ui.Muted.Render("opens once the map's core is solved: "+units.Plural(t.Map.Core(), "problem")), text))
	for _, l := range p.trialFacts() {
		out = append(out, wrap(l, modIndent, w)...)
	}
	return append(out, p.seeAlso(text)...)
}

// trialFacts is the trial, a line each: its doors, what they aim at, what
// passing takes, what it grants.
func (p *Module) trialFacts() []string {
	t := p.e.trial
	out := []string{ui.G.Blind + "  " + units.Plural(curriculum.Doors, "unseen door") + " on " + p.e.biome +
		ui.G.Sep + ui.RatingBand(t.Band[0], t.Band[1]) + ui.Muted.Render(ui.G.Sep+"every door blind")}
	if a := aims(t); a != "" {
		out = append(out, ui.G.Rating+"  aimed "+a+ui.Muted.Render(": the core's middle, its hardest, a step past"))
	}
	lose := rules.Trial().TrialLostRooms
	out = append(out, fmt.Sprintf("%s  pass %d of %d", ui.G.Pass, trialPass, curriculum.Doors)+
		ui.Muted.Render(": "+units.Plural(lose, "door")+" may go to :q!"))
	return append(out, p.passLine())
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
