// Package samples is the samples view, w on a door: a sample's input,
// expected output and the program's own side by side, its stderr under
// them, and the build's warnings above, so a failing :w shows what the
// program printed and not only the first token it got wrong.
package samples

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Source is the pass to show. The view asks on every View, so a :w while
// it is open shows the new pass; ok is false once there is no door (it
// closed under the view).
type Source func() (p hud.Pass, ok bool)

// Wide is the narrowest view that sets input, expected output and output
// side by side. From Medium the two outputs are side by side, the input
// under them, and below it all three are stacked, the outputs first.
const (
	Wide   = 96
	Medium = 50
)

// maxWarnings is how many warnings show above the samples; the rest are
// counted.
const maxWarnings = 3

type Model struct {
	src Source
	// sel is the sample the player picked; until they pick one the view
	// follows the first failure, as the HUD does.
	sel    int
	picked bool
	off    int // the body's scroll offset
	room   int // the body's height at the last View, for paging
}

func New(src Source) *Model { return &Model{src: src} }

func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	p, _ := m.src()
	page := max(m.room-1, 1)
	switch key := k.String(); key {
	case "w", "q", "esc":
		return m, screens.Emit(screens.Back{})
	case "l", "right", "tab":
		m.pick(p, m.current(p)+1)
	case "h", "left", "shift+tab":
		m.pick(p, m.current(p)-1)
	case "j", "down":
		m.off++
	case "k", "up":
		m.off--
	case "ctrl+d", "pgdown", "space":
		m.off += page
	case "ctrl+u", "pgup":
		m.off -= page
	case "g", "home":
		m.off = 0
	case "G", "end":
		m.off = 1 << 30 // View clamps it to the last page
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			m.pick(p, int(key[0]-'1'))
		}
	}
	return m, nil
}

// pick shows sample i, if there is one, from its top.
func (m *Model) pick(p hud.Pass, i int) {
	if i < 0 || i >= len(p.Tests) {
		return
	}
	m.sel, m.picked, m.off = i, true, 0
}

// current is the sample shown: the one picked, else the first failure.
func (m *Model) current(p hud.Pass) int {
	if m.picked {
		return min(m.sel, max(len(p.Tests)-1, 0))
	}
	for i, t := range p.Tests {
		if t.Done && t.Result.Verdict != judge.AC {
			return i
		}
	}
	return 0
}

func (m *Model) View(w, h int) string {
	p, ok := m.src()
	head := []string{header(p, ok, w), ui.Rule(w)}
	var body []string
	sel := m.current(p)
	switch {
	case !ok:
		body = []string{ui.Muted.Render("no door is open")}
	case p.Err != nil:
		body = wrap(ui.Danger, p.Err.Error(), w)
	case p.Waiting:
		body = []string{ui.Muted.Render("not run" + ui.G.Sep + ":w judges")}
	case p.Build == nil:
		body = []string{ui.Muted.Render("compiling…")}
	case !p.Build.OK:
		body = append([]string{ui.Danger.Render("CE") + ui.Muted.Render("  compile error"), ""},
			lines(p.Build.Log)...)
	case p.Quiet && len(p.Tests) > 0:
		body = []string{quietWord(p), ui.Muted.Render("pass or fail is all :w says on this door")}
	case len(p.Tests) == 0:
		head = append(head, warnings(p.Build.Log)...)
		body = []string{ui.Muted.Render("no samples" + ui.G.Sep + ":Submit judges the full tests" + ui.G.Sep + ":Case adds your own")}
	default:
		if ws := warnings(p.Build.Log); len(ws) > 0 {
			head = append(head, ws...)
			head = append(head, ui.Rule(w))
		}
		head = append(head, tabs(p, sel, w), ui.Rule(w))
		body = detail(p.Tests[sel], w)
	}

	m.room = max(h-len(head)-1, 1)
	m.off = min(max(m.off, 0), max(len(body)-m.room, 0))
	shown := body[m.off:min(m.off+m.room, len(body))]
	where := ""
	if ok && len(p.Tests) > 0 {
		where = fmt.Sprintf("#%s of %d", p.Tests[sel].Name, len(p.Tests))
	}
	if len(body) > m.room {
		where += fmt.Sprintf("%slines %d–%d of %d", sep(where), m.off+1, m.off+len(shown), len(body))
	}
	bar := ui.StatusBar(w, where, "h/l", "sample", "j/k", "scroll", "w", "close")
	return ui.Layout(w, h, append(head, shown...), bar)
}

func sep(s string) string {
	if s == "" {
		return ""
	}
	return " · "
}

// header is the pass in one line: "milk2 · samples 0/1 · asan+ubsan ·
// built 0.6s".
func header(p hud.Pass, ok bool, w int) string {
	if !ok {
		return ui.Strong.Render("samples")
	}
	s := ui.Strong.Render(p.Name) + ui.Muted.Render(ui.G.Sep) + "samples"
	switch {
	case p.Quiet && len(p.Tests) > 0:
	case p.Build != nil && p.Build.OK:
		s += fmt.Sprintf(" %d/%d", passed(p), len(p.Tests))
	}
	if p.Running {
		s += ui.Muted.Render(" running…")
	}
	s += ui.Muted.Render(ui.G.Sep + p.Profile)
	if b := p.Build; b != nil {
		built := fmt.Sprintf("built %.1fs", b.Duration.Seconds())
		if b.Cached {
			built = "cached build"
		}
		s += ui.Muted.Render(ui.G.Sep + built)
	}
	return ui.Clip(s, w)
}

func passed(p hud.Pass) int {
	n := 0
	for _, t := range p.Tests {
		if t.Done && t.Result.Verdict == judge.AC {
			n++
		}
	}
	return n
}

func quietWord(p hud.Pass) string {
	for _, t := range p.Tests {
		if t.Done && t.Result.Verdict != judge.AC {
			return ui.Danger.Render(ui.G.Fail + " fail")
		}
	}
	if p.Running {
		return ui.Muted.Render("running…")
	}
	return ui.OK.Render(ui.G.Pass + " pass")
}

// warnings is the build's warnings, a count and the first few, each as
// its place and text (hud.ShortWarning).
func warnings(log string) []string {
	ws := judge.Warnings(log)
	if len(ws) == 0 {
		return nil
	}
	out := []string{ui.Warn.Render(units.Plural(len(ws), "warning"))}
	for i, w := range ws {
		if i == maxWarnings {
			out = append(out, ui.Muted.Render(fmt.Sprintf("  … %d more", len(ws)-i)))
			break
		}
		out = append(out, "  "+hud.ShortWarning(w))
	}
	return out
}

// tabs names every sample by its verdict, the one shown under the cursor:
// "▸ #1 WA   #2 AC   #3 ·".
func tabs(p hud.Pass, sel, w int) string {
	parts := make([]string, len(p.Tests))
	for i, t := range p.Tests {
		cur := "  "
		if i == sel {
			cur = ui.Focus.Render(ui.G.Cursor) + " "
		}
		v := ui.Muted.Render("·")
		if t.Done {
			v = ui.Verdict(t.Result.Label()).Render(t.Result.Label())
		}
		parts[i] = cur + "#" + t.Name + " " + v
	}
	return ui.Clip(strings.Join(parts, " "), w)
}

// detail is one sample: its verdict and costs, where the checker found it
// wrong, the three texts, then stderr.
func detail(t hud.PassTest, w int) []string {
	if !t.Done {
		return append([]string{ui.Muted.Render("#" + t.Name + " pending"), ""}, texts(t, w)...)
	}
	r := t.Result
	head := fmt.Sprintf("#%s %s", t.Name, r.Label())
	if r.Reason != "" {
		head += ui.G.Sep + r.Reason
	}
	out := []string{ui.Verdict(r.Label()).Render(head) + ui.Muted.Render(fmt.Sprintf("%scpu %s%swall %s%speak %s",
		ui.G.Sep, ms(r.CPU), ui.G.Sep, ms(r.Wall), ui.G.Sep, units.Mem(r.MaxRSS)))}
	if d := r.Diff; d != nil {
		out = append(out, ui.Muted.Render("want ")+token(d.Want)+ui.Muted.Render(fmt.Sprintf(" (line %d)", d.Line))+
			ui.Muted.Render(ui.G.Sep+"got ")+token(d.Got)+ui.Muted.Render(fmt.Sprintf(" (line %d)", d.GotLine)))
	}
	out = append(out, "")
	out = append(out, texts(t, w)...)
	if s := strings.TrimRight(r.Stderr, "\n"); s != "" {
		out = append(out, "", ui.Strong.Render("stderr"))
		out = append(out, lines(s)...)
	}
	return out
}

func token(s string) string {
	if s == judge.EOF {
		return ui.Danger.Render("end of output")
	}
	return ui.Danger.Render(s)
}

func ms(d time.Duration) string { return fmt.Sprintf("%dms", d.Milliseconds()) }

// text is one of a sample's three texts and the line in it the checker
// marked, 0 for none.
type text struct {
	name  string
	lines []string
	mark  int
	empty string // what an empty text says
}

// texts are the input, the expected output and the program's, laid out
// for w (Wide, Medium). The lines a WA's checker
// compared are marked in both outputs; an output that ended first is
// marked where it ended.
func texts(t hud.PassTest, w int) []string {
	in := text{name: "input", lines: lines(t.In), empty: "(empty)"}
	want := text{name: "expected", lines: lines(t.Want), empty: "(empty)"}
	if t.NoWant {
		want.empty = "(none: run only)"
	}
	got := text{name: "output", empty: "(nothing printed)"}
	if t.Done {
		got.lines = lines(t.Result.Stdout)
		if d := t.Result.Diff; d != nil {
			want.mark, got.mark = d.Line, d.GotLine
			if d.Got == judge.EOF {
				for len(got.lines) < d.GotLine {
					got.lines = append(got.lines, "")
				}
				end := " end of output"
				if strings.TrimSpace(t.Result.Stdout) == "" {
					end = " nothing printed"
				}
				got.lines[d.GotLine-1] += ui.Danger.Render(ui.G.Fail + end)
				got.mark = 0
			}
		}
	} else {
		got.empty = "(not run)"
	}
	switch {
	case w >= Wide:
		return beside(w, in, want, got)
	case w >= Medium:
		return append(append(beside(w, want, got), ""), stack(in)...)
	}
	out := stack(got)
	for _, c := range []text{want, in} {
		out = append(append(out, ""), stack(c)...)
	}
	return out
}

// stack is c under its name.
func stack(c text) []string {
	out := []string{ui.Strong.Render(c.name)}
	for j := range max(len(c.lines), 1) {
		out = append(out, "  "+cell(c, j))
	}
	return out
}

// beside sets cs side by side in w cells, each under its name.
func beside(w int, cs ...text) []string {
	const gap = 3 // " │ "
	cw := (w - gap*(len(cs)-1)) / len(cs)
	bar := ui.Muted.Render(" │ ")
	row := func(cell func(i int, c text) string) string {
		var b strings.Builder
		for i, c := range cs {
			if i > 0 {
				b.WriteString(bar)
			}
			width := cw
			if i == len(cs)-1 {
				width = w - (gap+cw)*(len(cs)-1)
			}
			b.WriteString(ui.Fit(cell(i, c), width))
		}
		return b.String()
	}
	out := []string{row(func(_ int, c text) string { return ui.Strong.Render(c.name) })}
	n := 1
	for _, c := range cs {
		n = max(n, len(c.lines))
	}
	for j := range n {
		out = append(out, row(func(_ int, c text) string { return cell(c, j) }))
	}
	return out
}

// cell is line j of c: marked if the checker compared it, the empty text's
// word on the first line of an empty one, "" past its end.
func cell(c text, j int) string {
	switch {
	case len(c.lines) == 0 && j == 0:
		return ui.Muted.Render(c.empty)
	case j >= len(c.lines):
		return ""
	case j+1 == c.mark:
		return ui.Danger.Render(ui.G.Fail + " " + c.lines[j])
	}
	return c.lines[j]
}

// lines splits s into lines for display, the newline that ends it not
// making one more, with tabs expanded and control bytes shown as escapes,
// so a program's output cannot move the cursor or recolour the screen.
func lines(s string) []string {
	s = strings.TrimSuffix(strings.ToValidUTF8(s, "�"), "\n")
	if s == "" {
		return nil
	}
	ls := strings.Split(s, "\n")
	for i, l := range ls {
		ls[i] = printable(strings.TrimSuffix(l, "\r"))
	}
	return ls
}

func printable(l string) string {
	var b strings.Builder
	col := 0
	for _, r := range l {
		switch {
		case r == '\t':
			n := 4 - col%4
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case unicode.Is(unicode.Cc, r):
			s := fmt.Sprintf("\\x%02x", r)
			b.WriteString(s)
			col += len(s)
		default:
			b.WriteRune(r)
			col += ansi.StringWidth(string(r))
		}
	}
	return b.String()
}

func wrap(t ui.Token, s string, w int) []string {
	var out []string
	for l := range strings.SplitSeq(ansi.Wrap(s, max(w, 10), ""), "\n") {
		out = append(out, t.Render(l))
	}
	return out
}
