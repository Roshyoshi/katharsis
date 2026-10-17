package cses

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// facts are what the game needs from a task page before it offers the
// task: limits, and whether a byte-for-byte judge can judge it.
type facts struct {
	TimeLimit time.Duration `json:"time_limit,omitempty"`
	MemLimit  int64         `json:"mem_limit,omitempty"` // bytes

	Interactive    bool    `json:"interactive,omitempty"`
	NeedsChecker   bool    `json:"needs_checker,omitempty"`
	FloatTolerance float64 `json:"float_tolerance,omitempty"`
}

// Default limits, for a task no page has been read for yet: nearly every
// CSES task has these.
const (
	defaultTime = time.Second
	defaultMem  = 512 << 20
)

type sample struct{ In, Out string }

// statement is a parsed task page.
type statement struct {
	facts
	Title   string
	Body    string // markdown of the statement, without heading or footer
	Samples []sample
}

// parseTask reads a /problemset/task/<id> page. The page is server-side
// rendered markdown: a ul.task-constraints with the limits, then a div.md
// whose math is <span class="math math-inline"> (or math-display) holding
// TeX for KaTeX, and whose examples are <p>Input:</p><pre>..</pre>
// <p>Output:</p><pre>..</pre>, one pair per "Example N" heading.
// Interactive tasks show a dialogue in a single <pre> instead.
func parseTask(page []byte) (*statement, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	body := find(doc, elem(atom.Div, "md"))
	if body == nil {
		return nil, errors.New("no statement on the task page")
	}
	st := &statement{}
	if nav := find(doc, elem(atom.Div, "title-block")); nav != nil {
		if h := find(nav, elem(atom.H1, "")); h != nil {
			st.Title = squash(textOf(h))
		}
	}
	if st.Title == "" {
		return nil, errors.New("no title on the task page")
	}
	st.TimeLimit, st.MemLimit = defaultTime, defaultMem
	if ul := find(doc, elem(atom.Ul, "task-constraints")); ul != nil {
		lim := squash(textOf(ul))
		if m := timeRE.FindStringSubmatch(lim); m != nil {
			secs, _ := strconv.ParseFloat(m[1], 64)
			st.TimeLimit = time.Duration(math.Round(secs*1000)) * time.Millisecond
		}
		if m := memRE.FindStringSubmatch(lim); m != nil {
			mb, _ := strconv.ParseInt(m[1], 10, 64)
			st.MemLimit = mb << 20
		}
	}
	text := squash(textOf(body))
	st.Interactive = interactiveRE.MatchString(text) ||
		find(body, func(n *html.Node) bool { return n.DataAtom == atom.H1 && attr(n, "id") == "interaction" }) != nil
	st.NeedsChecker = severalRE.MatchString(text)
	st.FloatTolerance = floatTolerance(text)
	st.Samples = samples(body)
	if st.Body, err = toMarkdown(body); err != nil {
		return nil, err
	}
	return st, nil
}

var (
	timeRE = regexp.MustCompile(`Time limit: ([0-9.]+) s`)
	memRE  = regexp.MustCompile(`Memory limit: ([0-9]+) MB`)

	interactiveRE = regexp.MustCompile(`(?i)this is an interactive problem`)

	// severalRE matches the wordings CSES uses for a task with more than
	// one right answer. Checked against all 400 statements on 2026-10-17:
	// "If there are several solutions, you may print any of them" (most),
	// "you can print any valid solution", "you can choose any of them",
	// "you can construct any solution" and "print an example of how to ..."
	// (Two Sets). It avoids "in how many ways" (a count), "you can choose
	// any number of pairs" (a move) and "several routes ... each counted".
	severalRE = regexp.MustCompile(`(?i)` +
		`(several|multiple) (possible |valid |optimal |correct )?(solutions|answers)` +
		`|(print|output|give|construct|choose) any (valid|correct|optimal|such|of them|solution|answer)` +
		`|you (can|may) (print|output) any` +
		`|any (valid )?(solution|answer)s?\b` +
		`|print an example`)

	floatRE    = regexp.MustCompile(`(?i)(absolute|relative) (or relative )?error`)
	floatExpRE = regexp.MustCompile(`10\^\{?-(\d+)\}?`)
)

// floatTolerance is the allowed error a statement states, 0 when it
// states none. CSES mostly asks for answers rounded to a fixed number of
// decimals, which compare exactly; a tolerance is stated only as an
// absolute or relative error.
func floatTolerance(text string) float64 {
	loc := floatRE.FindStringIndex(text)
	if loc == nil {
		return 0
	}
	if m := floatExpRE.FindStringSubmatch(text[loc[0]:min(len(text), loc[1]+80)]); m != nil {
		if k, err := strconv.Atoi(m[1]); err == nil && k > 0 && k < 19 {
			return math.Pow10(-k)
		}
	}
	return 1e-6
}

// samples pairs each <pre> after a "Input:" paragraph with the next one
// after "Output:", in page order.
func samples(body *html.Node) []sample {
	var ins, outs []string
	for _, pre := range findAll(body, elem(atom.Pre, "")) {
		switch sampleKind(pre) {
		case "in":
			ins = append(ins, sampleText(textOf(pre)))
		case "out":
			outs = append(outs, sampleText(textOf(pre)))
		}
	}
	var s []sample
	for i := range min(len(ins), len(outs)) {
		s = append(s, sample{In: ins[i], Out: outs[i]})
	}
	return s
}

// sampleKind is "in" or "out" for a sample <pre>, "" for any other.
func sampleKind(pre *html.Node) string {
	p := prevElement(pre)
	if p == nil || p.DataAtom != atom.P {
		return ""
	}
	switch strings.TrimSpace(textOf(p)) {
	case "Input:":
		return "in"
	case "Output:":
		return "out"
	}
	return ""
}

// sampleText ends the data with exactly one newline, as a test file
// would, and drops CRs.
func sampleText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimLeft(s, "\n")
	s = strings.TrimRight(s, "\n \t")
	return s + "\n"
}

// toMarkdown converts the statement body. Math and sample data must reach
// the markdown exactly (screens/reader typesets the TeX itself), and the
// converter would escape them, so each is swapped for an inert
// placeholder first and swapped back after, as content/usaco does. A
// literal "$" in prose becomes "\$", which the reader does not take for a
// delimiter. The page's <h1> sections (Input, Output, Constraints,
// Example) become level-4 headings under the statement's title.
func toMarkdown(body *html.Node) (string, error) {
	var v verbatim
	for _, span := range findAll(body, elem(atom.Span, "math")) {
		delim := "$"
		if hasClass(span, "math-display") {
			delim = "$$"
		}
		tex := strings.TrimSpace(textOf(span))
		span.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: v.placeholder(delim + tex + delim)}, span)
		span.Parent.RemoveChild(span)
	}
	for _, n := range findAll(body, func(n *html.Node) bool { return n.Type == html.TextNode }) {
		if !inside(n, atom.Pre) && !inside(n, atom.Code) && strings.Contains(n.Data, "$") {
			n.Data = strings.ReplaceAll(n.Data, "$", v.placeholder(`\$`))
		}
	}
	for _, pre := range findAll(body, elem(atom.Pre, "")) {
		data := textOf(pre)
		if sampleKind(pre) != "" {
			data = sampleText(data) // same bytes as samples/N.in
		} else {
			data = strings.TrimRight(strings.TrimLeft(data, "\n"), "\n ") + "\n"
		}
		p := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.P}
		p.AppendChild(&html.Node{Type: html.TextNode, Data: v.placeholder(strings.TrimSuffix(fence(data), "\n"))})
		pre.Parent.InsertBefore(p, pre)
		pre.Parent.RemoveChild(pre)
	}
	for _, h := range findAll(body, func(n *html.Node) bool {
		return n.DataAtom == atom.H1 || n.DataAtom == atom.H2 || n.DataAtom == atom.H3
	}) {
		h.DataAtom, h.Data = atom.H4, "h4"
	}
	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(),
	))
	md, err := conv.ConvertNode(body, converter.WithDomain(site+"/"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v.restore(unescapeBrackets(string(md)))), nil
}

// render is statement.md: title, where the task sits, limits, the body,
// and a footer with the source.
func render(st *statement, section string, pageURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", mdEscape(st.Title))
	fmt.Fprintf(&b, "*CSES Problem Set, %s*\n\n", mdEscape(section))
	fmt.Fprintf(&b, "*Time limit: %s s · Memory limit: %d MB*\n\n",
		strconv.FormatFloat(st.TimeLimit.Seconds(), 'f', -1, 64), st.MemLimit>>20)
	b.WriteString(st.Body)
	fmt.Fprintf(&b, "\n\n---\n\nSource: <%s>\n", pageURL)
	return b.String()
}

// verbatim holds the spans toMarkdown protects from the converter.
type verbatim struct{ spans []string }

// verbMark starts every placeholder. Placeholders are letters and digits
// only, which no markdown escaper touches; the trailing X keeps
// "CSVERB1X" from matching inside "CSVERB12X".
const verbMark = "CSVERB"

func (v *verbatim) placeholder(s string) string {
	v.spans = append(v.spans, s)
	return fmt.Sprintf("%s%dX", verbMark, len(v.spans)-1)
}

func (v *verbatim) restore(md string) string {
	if len(v.spans) == 0 {
		return md
	}
	pairs := make([]string, 0, 2*len(v.spans))
	for i, s := range v.spans {
		pairs = append(pairs, fmt.Sprintf("%s%dX", verbMark, i), s)
	}
	return strings.NewReplacer(pairs...).Replace(md)
}

// fence wraps s in a ```text block, with a fence longer than any run of
// backticks in s.
func fence(s string) string {
	ticks := "```"
	for strings.Contains(s, ticks) {
		ticks += "`"
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return ticks + "text\n" + s + ticks + "\n"
}

// unescapeBrackets turns the converter's "\[" back into "[" outside code:
// screens/reader reads "\[" as the start of display math. Math is still a
// placeholder when this runs, so every "\[" left is an escaped bracket.
func unescapeBrackets(md string) string {
	if !strings.Contains(md, `\[`) {
		return md
	}
	var b strings.Builder
	fenced := ""
	for line := range strings.Lines(md) {
		if t := strings.TrimLeft(line, " "); strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			switch mark := t[:3]; fenced {
			case "":
				fenced = mark
			case mark:
				fenced = ""
			}
			b.WriteString(line)
			continue
		}
		if fenced != "" {
			b.WriteString(line)
			continue
		}
		code := false
		for i := 0; i < len(line); i++ {
			switch {
			case line[i] == '`':
				code = !code
			case !code && line[i] == '\\' && i+1 < len(line) && line[i+1] == '[':
				continue
			case !code && line[i] == '\\' && i+1 < len(line):
				b.WriteString(line[i : i+2])
				i++
				continue
			}
			b.WriteByte(line[i])
		}
	}
	return b.String()
}

// mdEscaper escapes what markdown reads as syntax in a plain title. "["
// stays: the reader takes "\[" for display math, and with "]" escaped no
// link can form.
var mdEscaper = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", `*`, `\*`, `_`, `\_`, `]`, `\]`,
	`<`, `\<`, `>`, `\>`, `|`, `\|`, `$`, `\$`,
)

func mdEscape(s string) string { return mdEscaper.Replace(s) }
