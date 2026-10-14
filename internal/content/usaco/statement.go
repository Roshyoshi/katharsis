package usaco

import (
	"bytes"
	"errors"
	"fmt"
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

// facts are what the game needs to know about a problem before it opens
// it. They come from the statement page because nothing else has them: the
// I/O file name differs from the test-data stem (cpid 567 reads paint.in
// from paint_bronze_dec15.zip), and time limits, checkers and interactivity
// are stated only in prose.
type facts struct {
	Title    string `json:"title"`
	Contest  string `json:"contest"`  // "2015 December", "2016 US Open", "2026 First Contest"
	Division string `json:"division"` // "bronze", ..., "platinum"
	Position int    `json:"position"`

	FileIO    string        `json:"file_io,omitempty"`    // freopen stem, "" for stdin
	TimeLimit time.Duration `json:"time_limit,omitempty"` // 0: the default
	MemLimit  int64         `json:"mem_limit,omitempty"`  // 0: the default

	Interactive    bool    `json:"interactive,omitempty"`
	NeedsChecker   bool    `json:"needs_checker,omitempty"`
	FloatTolerance float64 `json:"float_tolerance,omitempty"`
}

type sample struct{ In, Out string }

type statement struct {
	facts
	Markdown string
	Samples  []sample
	Credits  string
}

// parseStatement reads a viewproblem2 page. USACO has served two formats:
// through the 2014-15 season the statement is one preformatted plain-text
// block (class "mono prewrap") with SAMPLE INPUT/OUTPUT sections; later
// statements are HTML with MathJax ($...$) and samples in <pre class=in>
// and <pre class=out>.
func parseStatement(page []byte, pageURL string) (*statement, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	body := find(doc, func(n *html.Node) bool {
		return n.DataAtom == atom.Span && attr(n, "id") == "probtext-text"
	})
	if body == nil {
		return nil, errors.New("no problem text on the page")
	}
	st := &statement{}
	if err := st.parseHeader(doc); err != nil {
		return nil, err
	}
	var text string
	if strings.Contains(attr(body, "class"), "prewrap") {
		text = textOf(body)
		st.Samples = plainSamples(text)
		st.Markdown, st.Credits = plainToMarkdown(text, st.Samples)
	} else {
		cleanHTML(body)
		st.Samples = htmlSamples(body)
		text = textOf(body) // TeX left in; the fact regexes allow for it
		st.Credits = credits(text)
		md, err := htmlToMarkdown(body, true)
		if err != nil {
			return nil, err
		}
		st.Markdown = md
	}
	// No samples is legal only for interactive problems (cpid 552 shows an
	// example dialogue instead); callers check.
	st.scanFacts(text)
	st.Markdown = st.render(pageURL)
	return st, nil
}

// cleanHTML drops markup noise from the body: the literal "**" USACO puts
// inside <strong> notes, and Cloudflare's obfuscated mailto links, which
// render as "[email protected]".
func cleanHTML(body *html.Node) {
	for _, n := range findAll(body, func(n *html.Node) bool { return n.Type == html.TextNode }) {
		if !inside(n, atom.Pre) {
			n.Data = strings.ReplaceAll(n.Data, "**", "")
		}
	}
	for _, a := range findAll(body, func(n *html.Node) bool {
		return n.DataAtom == atom.A && strings.Contains(attr(n, "href"), "email-protection")
	}) {
		a.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: "the USACO staff"}, a)
		a.Parent.RemoveChild(a)
	}
}

var (
	// "USACO 2011 November Contest, Bronze Division", "USACO 2016 US Open,
	// Gold", "USACO 2026 First Contest, Silver".
	contestRE = regexp.MustCompile(`^USACO (\d{4} .+?), (Bronze|Silver|Gold|Platinum)(?: Division)?$`)
	problemRE = regexp.MustCompile(`^Problem (\d+)\. (.+)$`)
	ordinalRE = regexp.MustCompile(`^\d{4} (First|Second|Third|Fourth|Fifth) Contest$`)
)

func (st *statement) parseHeader(doc *html.Node) error {
	for _, h := range findAll(doc, func(n *html.Node) bool { return n.DataAtom == atom.H2 }) {
		t := squash(textOf(h))
		if m := contestRE.FindStringSubmatch(t); m != nil {
			st.Contest = contestLabel(m[1])
			st.Division = strings.ToLower(m[2])
		} else if m := problemRE.FindStringSubmatch(t); m != nil {
			st.Position, _ = strconv.Atoi(m[1])
			st.Title = m[2]
		}
	}
	if st.Contest == "" || st.Title == "" {
		return errors.New("no contest/problem header on the page")
	}
	return nil
}

// contestLabel matches the guide's div_to_probs labels: "2015 December",
// "2016 US Open", and since 2025-26 "2026 First Contest".
func contestLabel(s string) string {
	if ordinalRE.MatchString(s) {
		return s
	}
	return strings.TrimSuffix(s, " Contest")
}

func (st *statement) render(pageURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", mdEscape(st.Title))
	fmt.Fprintf(&b, "*USACO %s, %s, Problem %d*\n\n", st.Contest, titleCase(st.Division), st.Position)
	b.WriteString(strings.TrimSpace(st.Markdown))
	b.WriteString("\n\n---\n\n")
	if st.Credits != "" {
		fmt.Fprintf(&b, "Problem credits: %s. ", mdEscape(st.Credits))
	}
	fmt.Fprintf(&b, "Source: <%s>\n", pageURL)
	return b.String()
}

// ---- modern HTML statements ------------------------------------------------

// htmlSamples collects <pre class=in> and <pre class=out> in page order.
// Early 2015 pages have bare <pre> right after an <h4>SAMPLE INPUT:</h4>
// or <h4>SAMPLE OUTPUT:</h4>; the heading decides for those.
func htmlSamples(body *html.Node) []sample {
	var ins, outs []string
	for _, pre := range findAll(body, func(n *html.Node) bool { return n.DataAtom == atom.Pre }) {
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
	if k := attr(pre, "class"); k == "in" || k == "out" {
		return k
	}
	if h := prevElement(pre); h != nil && h.DataAtom == atom.H4 {
		switch t := strings.TrimSpace(textOf(h)); {
		case strings.HasPrefix(t, "SAMPLE INPUT"):
			return "in"
		case strings.HasPrefix(t, "SAMPLE OUTPUT"):
			return "out"
		}
	}
	return ""
}

// prevElement is the closest preceding element sibling, skipping text.
func prevElement(n *html.Node) *html.Node {
	for p := n.PrevSibling; p != nil; p = p.PrevSibling {
		if p.Type == html.ElementNode {
			return p
		}
		if strings.TrimSpace(p.Data) != "" {
			return nil
		}
	}
	return nil
}

// sampleText drops the newline that follows <pre> in USACO's markup and
// ends the data with exactly one newline, as a test file would.
func sampleText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimLeft(s, "\n")
	s = strings.TrimRight(s, "\n \t")
	return s + "\n"
}

var creditsRE = regexp.MustCompile(`\[?Problem credits: ([^\]\n]+)\]?`)

func credits(text string) string {
	if m := creditsRE.FindStringSubmatch(text); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// htmlToMarkdown converts a statement or analysis body. Math is protected
// from the converter's escaping and restored verbatim (see verbatim). With
// textPre, every <pre> becomes a ```text fence holding its text exactly,
// samples included, which is what statements want; analysis pages pass
// false and keep the converter's code blocks with their guessed language.
func htmlToMarkdown(body *html.Node, textPre bool) (string, error) {
	// Credits move to the footer; drop them from the body.
	for _, n := range findAll(body, func(n *html.Node) bool {
		return n.Type == html.TextNode && strings.Contains(n.Data, "Problem credits:")
	}) {
		n.Data = creditsRE.ReplaceAllString(n.Data, "")
	}
	var v verbatim
	v.protectMath(body)
	if textPre {
		for _, pre := range findAll(body, func(n *html.Node) bool { return n.DataAtom == atom.Pre }) {
			data := strings.TrimRight(textOf(pre), "\n") + "\n"
			if sampleKind(pre) != "" {
				data = sampleText(textOf(pre)) // same bytes as samples/N.in
			}
			p := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.P}
			p.AppendChild(&html.Node{Type: html.TextNode, Data: v.placeholder(fence("text", data))})
			pre.Parent.InsertBefore(p, pre)
			pre.Parent.RemoveChild(pre)
		}
	}
	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(),
	))
	wrap := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	for c := body.FirstChild; c != nil; {
		next := c.NextSibling
		body.RemoveChild(c)
		wrap.AppendChild(c)
		c = next
	}
	md, err := conv.ConvertNode(wrap, converter.WithDomain("https://usaco.org/"))
	if err != nil {
		return "", err
	}
	return v.restore(string(md)), nil
}

// ---- old plain-text statements ---------------------------------------------

var (
	// sectionRE matches the all-caps headings of plain-text statements:
	// "INPUT FORMAT:", "SAMPLE INPUT (file crazy.in):", "OUTPUT DETAILS:",
	// and in 2014-15 "SAMPLE OUTPUT: (file grass.out)".
	sectionRE = regexp.MustCompile(`^[A-Z][A-Z0-9 ]*[A-Z0-9](?: \([^)]*\))?:(?: \([^)]*\))?$`)
	authorRE  = regexp.MustCompile(`\[([^\]]+)\]\s*$`)
	numberRE  = regexp.MustCompile(`^(\d+)([.)])`)
)

func plainSamples(text string) []sample {
	var ins, outs []string
	mode := "" // "in", "out" or "" while outside a SAMPLE section
	var buf []string
	end := func() {
		data := sampleText(strings.Join(buf, "\n"))
		switch mode {
		case "in":
			ins = append(ins, data)
		case "out":
			outs = append(outs, data)
		}
		buf = nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trim := strings.TrimSpace(line)
		if !sectionRE.MatchString(trim) {
			if mode != "" {
				buf = append(buf, strings.TrimRight(line, " \t"))
			}
			continue
		}
		end()
		switch {
		case strings.HasPrefix(trim, "SAMPLE INPUT"):
			mode = "in"
		case strings.HasPrefix(trim, "SAMPLE OUTPUT"):
			mode = "out"
		default:
			mode = ""
		}
	}
	end()
	var s []sample
	for i := range min(len(ins), len(outs)) {
		s = append(s, sample{In: ins[i], Out: outs[i]})
	}
	return s
}

// plainToMarkdown turns a plain-text statement into markdown that reflows
// in a narrow pane: prose paragraphs are joined, "* Line 1: ..." blocks
// become list items, sample data and anything indented (ASCII diagrams)
// stays verbatim in fenced blocks, and the all-caps section names become
// headings. It also returns the author from the "Problem 1: Title
// [Author, 2012]" first line, which the header already repeats.
func plainToMarkdown(text string, samples []sample) (md, author string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\t", "        ")
	blocks := splitBlocks(text)
	if len(blocks) > 0 && len(blocks[0]) == 1 && strings.HasPrefix(blocks[0][0], "Problem ") {
		if m := authorRE.FindStringSubmatch(blocks[0][0]); m != nil {
			author = m[1]
		}
		blocks = blocks[1:]
	}
	var b strings.Builder
	var ins, outs int // samples rendered so far
	skip := false     // blocks of a sample already rendered from samples
	verbatim := false // inside a SAMPLE section that samples did not cover
	for _, blk := range blocks {
		first := strings.TrimSpace(blk[0])
		switch {
		case len(blk) == 1 && sectionRE.MatchString(first):
			fmt.Fprintf(&b, "#### %s\n\n", mdEscape(strings.TrimSuffix(first, ":")))
			skip, verbatim = false, false
			// Sample data comes from the parsed samples, so the fence holds
			// the same bytes as samples/N.in and N.out.
			switch {
			case strings.HasPrefix(first, "SAMPLE INPUT") && ins < len(samples):
				b.WriteString(fence("text", samples[ins].In) + "\n")
				ins, skip = ins+1, true
			case strings.HasPrefix(first, "SAMPLE OUTPUT") && outs < len(samples):
				b.WriteString(fence("text", samples[outs].Out) + "\n")
				outs, skip = outs+1, true
			case strings.HasPrefix(first, "SAMPLE "):
				verbatim = true
			}
			continue
		case skip:
			continue
		case len(blk) == 1 && strings.HasPrefix(first, "PROBLEM NAME: "):
			fmt.Fprintf(&b, "**PROBLEM NAME:** %s\n\n", mdEscape(strings.TrimPrefix(first, "PROBLEM NAME: ")))
			continue
		case verbatim || looksPreformatted(blk):
			b.WriteString(fence("text", strings.Join(blk, "\n")))
		case strings.HasPrefix(blk[0], "* "):
			b.WriteString(listItem(blk))
		default:
			b.WriteString(paragraph(blk))
		}
		b.WriteString("\n")
	}
	return b.String(), author
}

func splitBlocks(text string) [][]string {
	var blocks [][]string
	var cur []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " ")
		if line == "" {
			if cur != nil {
				blocks = append(blocks, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, line)
	}
	if cur != nil {
		blocks = append(blocks, cur)
	}
	return blocks
}

// looksPreformatted is true for diagrams and tables: lines indented on
// their own (not as the hanging indent of a "* Line" item) or with runs of
// spaces inside.
func looksPreformatted(blk []string) bool {
	list := strings.HasPrefix(blk[0], "* ")
	for i, line := range blk {
		indented := line[0] == ' '
		if indented && (i == 0 || !list) {
			return true
		}
		if strings.Contains(strings.TrimSpace(line), "   ") && !list {
			return true
		}
	}
	return false
}

// fence wraps s in a fenced code block, with a fence longer than any run
// of backticks in s.
func fence(lang, s string) string {
	ticks := "```"
	for strings.Contains(s, ticks) {
		ticks += "`"
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return ticks + lang + "\n" + s + ticks + "\n"
}

func listItem(blk []string) string {
	return "- " + mdEscape(squash(strings.Join(blk, " "))[2:]) + "\n"
}

func paragraph(blk []string) string {
	p := mdEscape(squash(strings.Join(blk, " ")))
	// A paragraph that starts like a list or heading must not become one.
	if len(p) > 0 && strings.ContainsRune("#+-=>", rune(p[0])) {
		p = `\` + p
	}
	if m := numberRE.FindStringSubmatch(p); m != nil {
		p = m[1] + `\` + p[len(m[1]):]
	}
	return p + "\n"
}

var mdEscaper = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", `*`, `\*`, `_`, `\_`, `[`, `\[`, `]`, `\]`,
	`<`, `\<`, `>`, `\>`, `|`, `\|`, `$`, `\$`,
)

func mdEscape(s string) string { return mdEscaper.Replace(s) }

// ---- DOM helpers -------------------------------------------------------------

func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	if match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := find(c, match); f != nil {
			return f
		}
	}
	return nil
}

func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			if n.DataAtom == atom.Script || n.DataAtom == atom.Style {
				return
			}
			if n.DataAtom == atom.Br {
				b.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.P, atom.Div, atom.H1, atom.H2, atom.H3, atom.H4, atom.Li, atom.Pre:
				b.WriteByte('\n')
			}
		}
	}
	walk(n)
	return b.String()
}

// squash collapses all whitespace runs to one space, so prose regexes do
// not care where the statement wrapped its lines.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
