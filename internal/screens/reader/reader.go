// Package reader is the full-screen markdown viewer for statements,
// editorials and learn-mode pages. Statements arrive as markdown with TeX
// math; the reader rewrites the math into Unicode (see TeXToUnicode),
// renders with Glamour at the terminal's width, and scrolls with a Bubbles
// viewport.
package reader

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/screens"
	"katharsis/internal/ui"
)

// Doc is one page to read.
type Doc struct {
	Title    string
	Source   string // a label for where it came from, e.g. "USACO 2024 December, Silver P2"
	URL      string // the page online; "" disables `o`
	Markdown string
}

// OpenURL asks the app to open URL in the browser. The reader emits it on
// `o`; the reader never runs a browser itself, so it stays testable and the
// app decides how (open(1), $BROWSER, or printing the link over ssh).
type OpenURL struct{ URL string }

// maxMeasure caps the wrap width. Prose wider than about 100 columns is hard
// to follow line to line; code blocks are not wrapped and may use the full
// width.
const maxMeasure = 100

type Model struct {
	doc Doc
	md  string // doc.Markdown after TeX and fence rewriting

	vp viewport.Model

	wrap      int      // wrap width of the current render, 0 before the first
	plain     []string // rendered lines without ANSI, for search
	renderErr error

	typing  bool   // the / prompt is open
	input   string // what has been typed at the prompt
	query   string // the committed search
	matches []int  // line indexes containing query
	cur     int    // index into matches
}

// New builds a reader for doc. Rendering waits for the first View, which is
// when the width is known.
func New(doc Doc) *Model {
	return &Model{doc: doc, md: prepare(doc.Markdown), vp: viewport.New()}
}

// prepare converts math, labels bare code fences as plain text, and expands
// tabs in code. Without a language Chroma guesses one from the content, and
// a sample grid of #s and .s can come out coloured as some unrelated
// language. Tabs must go because nothing downstream agrees on their width:
// Glamour pads lines counting a tab as zero cells, Lip Gloss turns it into
// four spaces, and the terminal jumps to the next multiple of eight.
func prepare(md string) string {
	var b strings.Builder
	for _, seg := range splitFences(TeXToUnicode(md)) {
		if !seg.code {
			b.WriteString(seg.text)
			continue
		}
		first, rest, _ := strings.Cut(seg.text, "\n")
		if _, n, info := fenceLine(first); n > 0 && strings.TrimSpace(info) == "" {
			first = strings.TrimRight(first, "\r") + "text"
		}
		b.WriteString(first)
		if strings.Contains(seg.text, "\n") {
			b.WriteString("\n" + expandTabs(rest, tabWidth))
		}
	}
	return b.String()
}

// tabWidth is the tab stop for code blocks: C++ in USACO Guide modules is
// tab-indented, and 4 keeps nested loops on screen at 80 columns.
const tabWidth = 4

// expandTabs replaces each tab with spaces to the next multiple of w,
// counting columns per line.
func expandTabs(s string, w int) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch r {
		case '\t':
			n := w - col%w
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case '\n':
			b.WriteRune(r)
			col = 0
		default:
			b.WriteRune(r)
			col += ansi.StringWidth(string(r))
		}
	}
	return b.String()
}

func (m *Model) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout(msg.Width, msg.Height)
	case tea.MouseWheelMsg:
		m.vp, _ = m.vp.Update(msg)
	case tea.KeyPressMsg:
		if m.typing {
			return m, m.prompt(msg)
		}
		switch msg.String() {
		case "q", "esc":
			if msg.String() == "esc" && m.query != "" {
				m.clearSearch()
				return m, nil
			}
			return m, screens.Emit(screens.Back{})
		case "o":
			if m.doc.URL != "" {
				return m, screens.Emit(OpenURL{URL: m.doc.URL})
			}
		case "g", "home":
			m.vp.GotoTop()
		case "G", "end":
			m.vp.GotoBottom()
		case "/":
			m.typing, m.input = true, ""
		case "n":
			m.step(1)
		case "N":
			m.step(-1)
		default:
			m.vp, _ = m.vp.Update(msg)
		}
	}
	return m, nil
}

// prompt edits the search line.
func (m *Model) prompt(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.typing = false
	case "enter":
		m.typing = false
		m.query = m.input
		m.findMatches()
		m.cur = m.firstMatchFrom(m.vp.YOffset())
		m.show()
	case "backspace":
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	default:
		if msg.Text != "" {
			m.input += msg.Text
		}
	}
	return nil
}

func (m *Model) clearSearch() { m.query, m.matches, m.cur = "", nil, 0 }

func (m *Model) findMatches() {
	m.matches = nil
	q := strings.ToLower(m.query)
	if q == "" {
		return
	}
	for i, l := range m.plain {
		if strings.Contains(strings.ToLower(l), q) {
			m.matches = append(m.matches, i)
		}
	}
}

func (m *Model) firstMatchFrom(line int) int {
	for i, l := range m.matches {
		if l >= line {
			return i
		}
	}
	return 0
}

func (m *Model) step(d int) {
	if len(m.matches) == 0 {
		return
	}
	m.cur = (m.cur + d + len(m.matches)) % len(m.matches)
	m.show()
}

// show scrolls the current match to a third of the way down, where the eye
// lands after a jump.
func (m *Model) show() {
	if len(m.matches) == 0 {
		return
	}
	m.vp.SetYOffset(m.matches[m.cur] - m.vp.Height()/3)
}

// layout sizes the viewport and re-renders when the wrap width changes,
// keeping the reading position at the same fraction of the document.
func (m *Model) layout(width, height int) {
	m.vp.SetWidth(max(width, 1))
	m.vp.SetHeight(max(height-3, 1))
	wrap := min(max(width, 20), maxMeasure)
	if wrap == m.wrap {
		return
	}
	frac := 0.0
	if n := m.vp.TotalLineCount(); n > 0 {
		frac = float64(m.vp.YOffset()) / float64(n)
	}
	m.wrap = wrap
	out, err := render(m.md, wrap)
	m.renderErr = err
	m.vp.SetContent(out)
	m.plain = strings.Split(ansi.Strip(out), "\n")
	m.vp.SetYOffset(int(frac*float64(m.vp.TotalLineCount()) + 0.5))
	if m.query != "" {
		m.findMatches()
		m.cur = min(m.cur, max(len(m.matches)-1, 0))
	}
}

// render runs Glamour, falling back to the prepared markdown wrapped as
// plain text so a renderer failure still leaves something to read.
func render(md string, wrap int) (string, error) {
	// The style is built from the palette, so the page follows the
	// terminal's theme rather than Glamour's fixed 256-color "dark" one.
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(ui.StyleConfig()),
		glamour.WithWordWrap(wrap),
	)
	var out string
	if err == nil {
		out, err = r.Render(md)
	}
	if err != nil {
		return ansi.Wrap(md, wrap, ""), err
	}
	// Glamour frames the document with blank lines; one at the top is
	// breathing room under the header, the rest is dead scroll.
	out = strings.TrimRight(out, " \n")
	lines := strings.Split(out, "\n")
	for len(lines) > 1 && strings.TrimSpace(ansi.Strip(lines[0])) == "" &&
		strings.TrimSpace(ansi.Strip(lines[1])) == "" {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n"), nil
}

func (m *Model) View(width, height int) string {
	m.layout(width, height)
	var out []string

	head := ui.Strong.Render(m.doc.Title)
	if m.doc.Source != "" {
		head += ui.Muted.Render(ui.G.Sep + m.doc.Source)
	}
	out = append(out, " "+head, ui.Rule(width))

	body := strings.Split(m.vp.View(), "\n")
	m.highlight(body)
	return ui.Layout(width, max(height, 1), append(out, body...), m.footer(width))
}

// highlight marks search hits on the visible lines.
func (m *Model) highlight(body []string) {
	if m.query == "" || len(m.matches) == 0 {
		return
	}
	q := strings.ToLower(m.query)
	top, x := m.vp.YOffset(), m.vp.XOffset()
	for i := range body {
		idx := top + i
		if idx >= len(m.plain) {
			break
		}
		low := strings.ToLower(m.plain[idx])
		var ranges []lipgloss.Range
		for off := 0; ; {
			j := strings.Index(low[off:], q)
			if j < 0 {
				break
			}
			j += off
			// Lower-casing can change byte lengths outside ASCII; cell
			// positions come from the original line.
			start := ansi.StringWidth(m.plain[idx][:min(j, len(m.plain[idx]))]) - x
			end := start + ansi.StringWidth(m.query)
			off = j + len(q)
			if end <= 0 || start < 0 {
				continue
			}
			st := ui.Hit.Style()
			if m.matches[m.cur] == idx {
				st = ui.HitCurrent.Style()
			}
			ranges = append(ranges, lipgloss.NewRange(start, end, st))
		}
		if len(ranges) > 0 {
			body[i] = lipgloss.StyleRanges(body[i], ranges...)
		}
	}
}

// footer is the pager's status bar, as in less: where the page is on the
// left, the keys on the right; or the / prompt while typing.
func (m *Model) footer(width int) string {
	if m.typing {
		return "/" + m.input + "█"
	}
	pos := ui.PagerLine(m.vp.YOffset(), m.vp.Height(), m.vp.TotalLineCount())
	switch {
	case m.renderErr != nil:
		pos = "render failed: " + m.renderErr.Error()
	case m.query != "" && len(m.matches) == 0:
		pos = "Pattern not found: /" + m.query
	case m.query != "":
		pos += ui.G.Sep + fmt.Sprintf("match %d/%d", m.cur+1, len(m.matches))
	}
	keys := []string{"j/k", "scroll", "space/b", "page", "g/G", "ends", "/", "search"}
	if len(m.matches) > 0 {
		keys = append(keys, "n/N", "next")
	}
	if m.doc.URL != "" {
		keys = append(keys, "o", "browser")
	}
	return ui.StatusBar(width, pos, append(keys, "q", "back")...)
}
