package rtfm

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
)

// Module is one module's page, set as a man page: NAME (id, title, what it
// is about), TRIAL (the unit doors in the order a trial boot plays them,
// any the catalog tops up, the blind integration doors, and what passing
// grants), SEE ALSO (prerequisites, authors), then the module's CC BY-NC-SA
// attribution. r reads the module, t starts its trial, o opens it on
// usaco.guide.
type Module struct {
	e      entry
	prereq []string // prerequisites as display names
	caps   map[string]bool
	lookup Lookup

	top int // first body line shown, when the body does not fit
	h   int // body rows at the last View
}

// NewModule builds the page for mod. c resolves prerequisite ids to titles
// and may be nil; judgeable and lookup are as for NewBrowser.
func NewModule(c *curriculum.Curriculum, mod curriculum.Module, m game.Meta, judgeable Judgeable, lookup Lookup) *Module {
	return newModule(c, newEntry(mod, judgeable), capSet(m), lookup)
}

func newModule(c *curriculum.Curriculum, e entry, caps map[string]bool, lookup Lookup) *Module {
	p := &Module{e: e, caps: caps, lookup: lookup}
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
		// A module with no topic grants nothing, so it has no trial to play.
		if p.e.mod.Topic != "" {
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
	crumb := bold.Render("RTFM") + " ▸ " + m.DivisionTitle + " ▸ " + m.Chapter
	head := spread(crumb, capCell(p.e.trial.Capability, p.caps[p.e.trial.Capability]), w)

	body := p.body(w)
	rows := max(h-3, 0)
	p.h = rows
	p.top = max(min(p.top, len(body)-rows), 0)
	shown := body[p.top:min(p.top+rows, len(body))]

	out := append([]string{head, ""}, shown...)
	for len(out) < h-1 {
		out = append(out, "")
	}
	out = append(out[:h-1], p.footer(w, len(body), rows))
	return frame(out, w, h)
}

func (p *Module) body(w int) []string {
	m := p.e.mod
	ind := strings.Repeat(" ", modIndent)
	text := min(w, 80) // prose measure; the trial table may use more

	var out []string
	out = append(out, bold.Render("NAME"))
	name := ind + m.ID + " - " + bold.Render(m.Title)
	if f := m.Frequency; f >= 0 {
		name = spread(name, stars(f)+" "+faint.Render(frequencyText(f)), text)
	}
	out = append(out, name)
	if m.Description != "" {
		out = append(out, wrap(m.Description, modIndent, text)...)
	}

	out = append(out, "", spread(bold.Render("TRIAL"),
		faint.Render("unit: Easy → Hard, tags shown · integration: blind"), text))
	out = append(out, p.unitLines(w)...)
	t := p.e.trial
	where := p.e.biome + " · " + band(t.IntegBand)
	if n := p.e.short(); n > 0 {
		out = append(out, ind+fmt.Sprintf("+  %d topped up from the catalog · %s", n, where))
	}
	out = append(out, ind+fmt.Sprintf("+  %d blind integration doors · %s", integrationDoors, where))
	out = append(out, ind+p.passLine())

	out = append(out, "", bold.Render("SEE ALSO"))
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
		out = append(out, faint.Render(l))
	}
	return out
}

// facts is one "label  value" line of SEE ALSO, the value wrapped under
// itself.
func facts(label, value string, w int) []string {
	const labelW = 10
	lines := wrap(value, modIndent+labelW, w)
	lines[0] = strings.Repeat(" ", modIndent) + pad(label, labelW) + strings.TrimLeft(lines[0], " ")
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
				d.rating, d.contest = pr.Rating, pr.Contest
			}
		}
		ds = append(ds, d)
		titleW = max(titleW, ansi.StringWidth(d.title))
	}
	const labelW, ratingW = 9, 4
	fixed := modIndent + 3 + colGap + labelW + colGap + ratingW
	titleW = max(min(titleW, w-fixed-colGap), 8)
	var out []string
	for i, d := range ds {
		rating := ""
		if d.rating > 0 {
			rating = ratingStyle(float64(d.rating)).Render(fmt.Sprint(d.rating))
		}
		line := strings.Repeat(" ", modIndent) + pad(fmt.Sprint(i+1), 3) + pad(d.title, titleW) +
			strings.Repeat(" ", colGap) + pad(d.label, labelW) +
			strings.Repeat(" ", colGap) + padLeft(rating, ratingW)
		if d.contest != "" {
			line += strings.Repeat(" ", colGap) + faint.Render(d.contest)
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

// passLine says what passing the trial grants.
func (p *Module) passLine() string {
	c := p.e.trial.Capability
	switch {
	case c == "":
		return faint.Render("·  grants no capability: " + p.e.biome + " is open to every boot")
	case p.caps[c]:
		return green.Render(heldMark+"  "+c+" held") + ": " + p.e.biome + " is already on your routes"
	}
	return lockedMark + "  pass → " + green.Render(heldMark+" "+c) + ": " + p.e.biome + " joins your route choices"
}

// footer is the action line, crash-screen style, with vim's ruler at the
// right when the page scrolls.
func (p *Module) footer(w, lines, rows int) string {
	keys := "[r] read"
	if p.e.mod.Topic != "" {
		keys += "  [t] start trial"
	}
	if p.e.mod.URL != "" {
		keys += "  [o] usaco.guide"
	}
	keys += "  [q] back"
	if lines <= rows {
		return keys
	}
	pos := fmt.Sprintf("%d%%", 100*p.top/max(lines-rows, 1))
	switch {
	case p.top == 0:
		pos = "Top"
	case p.top >= lines-rows:
		pos = "Bot"
	}
	return spread(keys, faint.Render("j/k scroll ")+pos, w)
}
