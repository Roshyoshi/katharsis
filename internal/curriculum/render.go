package curriculum

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// siteBase resolves the guide's root-relative links and assets
// (/silver/prefix-sums, /CPH.pdf, /content/...png live under public/).
const siteBase = "https://usaco.guide"

func absURL(u string) string {
	switch {
	case strings.HasPrefix(u, "//"):
		return "https:" + u
	case strings.HasPrefix(u, "/"):
		return siteBase + u
	}
	return u
}

var linkTextEscaper = strings.NewReplacer(`[`, `\[`, `]`, `\]`)

// escapeLinkText escapes markdown that is already markdown for use as link
// text: only unbalanced brackets could break it.
func escapeLinkText(s string) string { return linkTextEscaper.Replace(s) }

// link makes a markdown link from literal text (a JSX attribute or JSON
// field, never markdown), so the text is fully escaped.
func link(text, url string) string {
	if url == "" {
		return escapeMarkdown(text)
	}
	return "[" + escapeMarkdown(text) + "](" + absURL(url) + ")"
}

// renderer turns the node tree of one module into markdown.
type renderer struct {
	file      string
	moduleURL string
	lists     map[string]ProblemList
	// unknown counts elements rendered as plain inner text because no
	// rule exists for them; the full-repo test reports them.
	unknown map[string]int
}

func (r *renderer) errf(n *node, format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", r.file, n.line, fmt.Sprintf(format, args...))
}

// childBase is the indentation shared by an element's content. MDX has no
// indented code blocks, so authors (and prettier) indent JSX children
// freely; stripping the common indent keeps nested lists intact while
// stopping a 4-space-indented paragraph from turning into code.
func childBase(nodes []*node) int {
	base := -1
	take := func(c int) {
		if base < 0 || c < base {
			base = c
		}
	}
	for _, n := range nodes {
		switch n.kind {
		case nodeText, nodeRaw:
			for i, l := range strings.Split(n.text, "\n") {
				if (i == 0 && !n.bol) || strings.TrimSpace(l) == "" {
					continue
				}
				take(indentCols(l))
			}
		default:
			if n.indent >= 0 {
				take(n.indent)
			}
		}
	}
	return max(base, 0)
}

// block renders nodes as a markdown block, normalized and trimmed.
func (r *renderer) block(nodes []*node) (string, error) {
	s, err := r.blockBase(nodes, childBase(nodes))
	if err != nil {
		return "", err
	}
	return normalize(s), nil
}

func dedent(text string, base int, bol bool) string {
	if base == 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if i == 0 && !bol {
			continue
		}
		lines[i] = stripCols(l, base)
	}
	return strings.Join(lines, "\n")
}

func endLine(b *strings.Builder) {
	if s := b.String(); s != "" && !strings.HasSuffix(s, "\n") {
		b.WriteByte('\n')
	}
}

func (r *renderer) blockBase(nodes []*node, base int) (string, error) {
	var b strings.Builder
	for _, n := range nodes {
		switch n.kind {
		case nodeText, nodeRaw:
			b.WriteString(dedent(n.text, base, n.bol))
		case nodeCode:
			endLine(&b)
			b.WriteString(fenceBlock(n.text, n.info, max(n.indent-base, 0)))
		case nodeMath:
			endLine(&b)
			b.WriteString(fenceBlock(displayTeX(n.text), "math", max(n.indent-base, 0)))
		case nodeElem:
			out, isBlock, err := r.elem(n)
			if err != nil {
				return "", err
			}
			if isBlock {
				b.WriteString("\n\n" + out + "\n\n")
			} else {
				b.WriteString(out)
			}
		}
	}
	return b.String(), nil
}

// inline renders nodes on one line, for table cells, list items and
// inline elements. A code block cannot live on one line, so it is an
// error rather than a silent mangle.
func (r *renderer) inline(nodes []*node) (string, error) {
	for _, n := range nodes {
		if n.kind == nodeCode {
			return "", r.errf(n, "code block inside an inline element")
		}
	}
	s, err := r.blockBase(nodes, 0)
	if err != nil {
		return "", err
	}
	s = strings.ReplaceAll(s, "\\\n", " ")
	return strings.Join(strings.Fields(s), " "), nil
}

func displayTeX(tex string) string {
	var lines []string
	for _, l := range strings.Split(simplifyTeX(tex), "\n") {
		if l = tidyTeX(l); l != "" {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

// fenceBlock emits a fenced code block whose fence is longer than any
// backtick run in the body, so the body can never close it early.
func fenceBlock(body, info string, indent int) string {
	longest, run := 0, 0
	for i := 0; i < len(body); i++ {
		if body[i] == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	pre := strings.Repeat(" ", indent)
	var b strings.Builder
	b.WriteString(pre + fence + info + "\n")
	if body != "" {
		for _, l := range strings.Split(body, "\n") {
			if l != "" {
				b.WriteString(pre)
			}
			b.WriteString(l + "\n")
		}
	}
	b.WriteString(pre + fence + "\n")
	return b.String()
}

func quote(body string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if l == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + l
		}
	}
	return strings.Join(lines, "\n")
}

// spoiler renders a collapsible section as a labelled blockquote; the TUI
// can fold everything under a "**▸ Spoiler" line. title is markdown.
func spoiler(title, body string) string {
	head := "**▸ Spoiler**"
	if title != "" {
		head = "**▸ Spoiler: " + title + "**"
	}
	if body == "" {
		return head
	}
	return head + "\n\n" + quote(body)
}

// callout renders Info/Warning/Optional and friends as a labelled
// blockquote.
func callout(label, body string) string {
	head := "> **" + label + "**"
	if body == "" {
		return head
	}
	return head + "\n>\n" + quote(body)
}

func (r *renderer) transparent(n *node) (string, bool, error) {
	if n.inline {
		s, err := r.inline(n.children)
		return s, false, err
	}
	s, err := r.block(n.children)
	return s, true, err
}

func (r *renderer) widget(what string) string {
	return "[" + what + ": open in browser](" + r.moduleURL + ")"
}

func (r *renderer) elem(n *node) (string, bool, error) {
	title, _ := n.attr("title")
	switch n.name {
	case "Spoiler":
		body, err := r.block(n.children)
		if err != nil {
			return "", false, err
		}
		return spoiler(escapeMarkdown(title), body), true, nil

	case "Info", "Warning", "Optional":
		body, err := r.block(n.children)
		if err != nil {
			return "", false, err
		}
		label := n.name
		if title != "" {
			label += ": " + escapeMarkdown(title)
		}
		return callout(label, body), true, nil

	case "IncompleteSection":
		body, err := r.block(n.children)
		if err != nil {
			return "", false, err
		}
		msg := "This section is not complete. Any help would be appreciated!"
		if body != "" {
			msg += "\n\n" + body
		}
		return callout("Incomplete section", msg), true, nil

	case "FocusProblem":
		return r.focusProblem(n)
	case "Problems":
		return r.problems(n)
	case "Resources":
		return r.resources(n)
	case "Resource":
		item, err := r.resource(n)
		return item, true, err
	case "LanguageSection":
		return r.languageSection(n)
	case "CPPSection", "JavaSection", "PySection", "CPPOnly":
		// Outside a LanguageSection the site renders these unfiltered.
		return r.transparent(n)
	case "JavaOnly", "PyOnly":
		return "", !n.inline, nil
	case "Quiz":
		return r.quiz(n)

	case "Asterisk":
		s, err := r.inline(n.children)
		return " (note: " + s + ")", false, err
	case "TextTooltip":
		s, err := r.inline(n.children)
		if c, _ := n.attr("content"); c != "" {
			s += " (" + escapeMarkdown(c) + ")"
		}
		return s, false, err

	case "YouTube":
		id, _ := n.attr("id")
		return link("video: YouTube", "https://www.youtube.com/watch?v="+id), !n.inline, nil
	case "PrefixSumInteractive":
		return r.widget("interactive demo"), !n.inline, nil
	case "DivisionList":
		return r.widget("problem list by contest"), !n.inline, nil
	case "svg":
		return r.widget("figure"), !n.inline, nil
	case "img":
		src, _ := n.attr("src")
		alt, _ := n.attr("alt")
		return imageLink(escapeMarkdown(alt), src), false, nil
	case "video":
		src, _ := n.attr("src")
		for _, c := range n.children {
			if c.kind == nodeElem && c.name == "source" && src == "" {
				src, _ = c.attr("src")
			}
		}
		return link("video", src), !n.inline, nil
	case "iframe":
		src, _ := n.attr("src")
		label := "embed"
		if title != "" {
			label += ": " + title
		}
		return link(label, src), !n.inline, nil
	case "source":
		return "", false, nil

	case "table":
		return r.table(n)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		s, err := r.inline(n.children)
		level, _ := strconv.Atoi(n.name[1:])
		return strings.Repeat("#", level) + " " + s, true, err
	case "hr":
		return "---", true, nil
	case "br":
		if n.inline {
			return "\\\n", false, nil
		}
		return "", true, nil
	case "a":
		s, err := r.inline(n.children)
		href, _ := n.attr("href")
		if href == "" {
			return s, false, err
		}
		return "[" + s + "](" + absURL(href) + ")", false, err
	case "b", "strong":
		s, err := r.inline(n.children)
		return wrapInline("**", s), false, err
	case "i", "em":
		s, err := r.inline(n.children)
		return wrapInline("*", s), false, err
	case "code", "kbd", "samp", "tt":
		s, err := r.inline(n.children)
		return codeSpan(s), false, err
	case "sup":
		s, err := r.inline(n.children)
		return "^" + s, false, err
	case "sub":
		s, err := r.inline(n.children)
		return "_" + s, false, err
	case "details":
		return r.details(n)
	case fragment, "center", "div", "p", "span", "section", "figure",
		"Quiz.Question", "Quiz.Answer", "Quiz.Explanation",
		"thead", "tbody", "tfoot", "tr", "td", "th", "u", "small", "mark":
		return r.transparent(n)
	}
	r.unknown[n.name]++
	return r.transparent(n)
}

func wrapInline(mark, s string) string {
	if s == "" {
		return ""
	}
	return mark + s + mark
}

func codeSpan(s string) string {
	if s == "" {
		return ""
	}
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		s = " " + s + " "
	}
	return fence + s + fence
}

func (r *renderer) list(n *node, attr string) (ProblemList, error) {
	key, ok := n.attr(attr)
	if !ok || key == "" {
		return ProblemList{}, r.errf(n, "<%s> has no %s attribute", n.name, attr)
	}
	l, ok := r.lists[key]
	if !ok {
		return ProblemList{}, r.errf(n, "<%s %s=%q>: no such list in the module's problems.json", n.name, attr, key)
	}
	return l, nil
}

func (r *renderer) focusProblem(n *node) (string, bool, error) {
	l, err := r.list(n, "problem")
	if err != nil {
		return "", false, err
	}
	if len(l.Problems) != 1 {
		return "", false, r.errf(n, "focus problem list %q has %d problems, want 1", l.Key, len(l.Problems))
	}
	p := l.Problems[0]
	line := link(p.Name, p.URL)
	for _, s := range []string{p.Source, p.Difficulty} {
		if s != "" {
			line += " · " + escapeMarkdown(s)
		}
	}
	return callout("Focus Problem", line), true, nil
}

func escapePipes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteByte('\\')
			if i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			}
		case '|':
			b.WriteString(`\|`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func mdTable(rows [][]string) string {
	cols := 0
	for _, row := range rows {
		cols = max(cols, len(row))
	}
	if cols == 0 {
		return ""
	}
	var b strings.Builder
	for i, row := range rows {
		cells := make([]string, cols)
		for j := range cells {
			if j < len(row) {
				cells[j] = escapePipes(row[j])
			}
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		if i == 0 {
			b.WriteString("|" + strings.Repeat(" --- |", cols) + "\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (r *renderer) problems(n *node) (string, bool, error) {
	l, err := r.list(n, "problems")
	if err != nil {
		return "", false, err
	}
	if len(l.Problems) == 0 {
		return "", true, nil
	}
	rows := [][]string{{"Problem", "Source", "Difficulty"}}
	for _, p := range l.Problems {
		name := link(p.Name, p.URL)
		if p.Starred {
			name = "★ " + name
		}
		rows = append(rows, []string{name, escapeMarkdown(p.Source), escapeMarkdown(p.Difficulty)})
	}
	return mdTable(rows), true, nil
}

// Resource source abbreviations, copied from the guide's
// src/utils/books.ts and src/components/markdown/ResourcesList.tsx. Books
// link to the whole book (the guide's per-section page table is not
// reproduced); module sources prefix a relative url.
var resourceBooks = map[string]string{
	"PAPS":   "/PAPS.pdf",
	"PAPS1":  "/PAPS1.pdf",
	"CP2":    "/CP2.pdf",
	"CPH":    "/CPH.pdf",
	"IUSACO": "https://darrenyao.com/usacobook/cpp.pdf",
	"GCP":    "https://link.springer.com/book/10.1007/978-3-319-72547-5",
	"MT":     "/matroid-theory.pdf",
}

var resourcePrefixes = map[string]string{
	"AryanshS": "https://sendtoaryansh.gitbook.io/informatics-notes/",
	"AoPS":     "http://aops.com/",
	"Benq":     "https://github.com/bqi343/USACO/blob/master/Implementations/content/",
	"Errichto": "https://www.youtube.com/c/Errichto/about",
	"CSA":      "https://csacademy.com/lesson/",
	"CF":       "https://codeforces.com/blog/entry/",
	"CF EDU":   "https://codeforces.com/edu/courses",
	"HE":       "https://hackerearth.com/",
	"KA":       "https://www.khanacademy.org/",
	"TC":       "https://www.topcoder.com/community/competitive-programming/tutorials/",
	"cp-algo":  "https://cp-algorithms.com/",
	"USACO":    "http://www.usaco.org/",
	"SO":       "https://stackoverflow.com/",
	"GFG":      "https://www.geeksforgeeks.org/",
	"CPC":      "https://github.com/SuprDewd/T-414-AFLV/tree/master/",
	"GCC":      "https://gcc.gnu.org/onlinedocs/gcc/",
	"CPP":      "http://www.cplusplus.com/reference/algorithm/",
	"CPPR":     "https://en.cppreference.com/w/cpp/language/",
	"LCPP":     "https://www.learncpp.com/cpp-tutorial/",
}

func resourceURL(source, url string) string {
	if book, ok := resourceBooks[source]; ok && url == "" {
		return absURL(book)
	}
	if prefix, ok := resourcePrefixes[source]; ok && !strings.HasPrefix(url, "http") {
		return prefix + url
	}
	return absURL(url)
}

func (r *renderer) resource(n *node) (string, error) {
	source, _ := n.attr("source")
	url, _ := n.attr("url")
	title, _ := n.attr("title")
	_, starred := n.attr("starred")
	desc, err := r.inline(n.children)
	if err != nil {
		return "", err
	}
	item := "- "
	if starred {
		item += "★ "
	}
	if title == "" {
		title = url
	}
	item += link(title, resourceURL(source, url))
	if source != "" {
		item += " (" + escapeMarkdown(source) + ")"
	}
	if desc != "" {
		item += " — " + desc
	}
	return item, nil
}

func isBlankText(n *node) bool {
	return (n.kind == nodeText || n.kind == nodeRaw) && strings.TrimSpace(n.text) == ""
}

func (r *renderer) resources(n *node) (string, bool, error) {
	head := "**Resources**"
	if t, _ := n.attr("title"); t != "" {
		head = "**Resources: " + escapeMarkdown(t) + "**"
	}
	var items []string
	for _, c := range n.children {
		switch {
		case isBlankText(c):
		case c.kind == nodeElem && c.name == "Resource":
			item, err := r.resource(c)
			if err != nil {
				return "", false, err
			}
			items = append(items, item)
		default:
			s, err := r.block([]*node{c})
			if err != nil {
				return "", false, err
			}
			if s != "" {
				items = append(items, s)
			}
		}
	}
	return head + "\n\n" + strings.Join(items, "\n"), true, nil
}

// languageSection keeps the C++ variant, falling back to Java then Python
// the way the site does when a language is missing, so no section is ever
// empty.
func (r *renderer) languageSection(n *node) (string, bool, error) {
	sections := map[string]*node{}
	var other []*node
	for _, c := range n.children {
		switch {
		case isBlankText(c):
		case c.kind == nodeElem && (c.name == "CPPSection" || c.name == "JavaSection" || c.name == "PySection"):
			if sections[c.name] != nil {
				return "", false, r.errf(c, "two <%s> in one <LanguageSection>", c.name)
			}
			sections[c.name] = c
		default:
			other = append(other, c)
		}
	}
	var parts []string
	if len(other) > 0 {
		s, err := r.block(other)
		if err != nil {
			return "", false, err
		}
		parts = append(parts, s)
	}
	for _, lang := range []struct{ tag, label string }{
		{"CPPSection", ""}, {"JavaSection", "Java"}, {"PySection", "Python"},
	} {
		sec := sections[lang.tag]
		if sec == nil {
			continue
		}
		s, err := r.block(sec.children)
		if err != nil {
			return "", false, err
		}
		if lang.label != "" {
			s = "*(Not available in C++; showing " + lang.label + ".)*\n\n" + s
		}
		parts = append(parts, s)
		break
	}
	return strings.Join(parts, "\n\n"), true, nil
}

// labelled puts a bold label in front of a block, on the same line when the
// block starts with a plain paragraph.
func labelled(label, body string) string {
	if body == "" {
		return label
	}
	first, _, _ := strings.Cut(body, "\n")
	structural := strings.HasPrefix(first, "```") || strings.HasPrefix(first, "|") ||
		strings.HasPrefix(first, ">") || strings.HasPrefix(first, "#") ||
		strings.HasPrefix(first, "**▸") || listItem.MatchString(first)
	if structural {
		return label + "\n\n" + body
	}
	return label + " " + body
}

func (r *renderer) quiz(n *node) (string, bool, error) {
	var parts []string
	q := 0
	for _, qn := range n.children {
		if isBlankText(qn) {
			continue
		}
		if qn.kind != nodeElem || qn.name != "Quiz.Question" {
			s, err := r.block([]*node{qn})
			if err != nil {
				return "", false, err
			}
			parts = append(parts, s)
			continue
		}
		q++
		var prompt, answers []*node
		for _, c := range qn.children {
			if c.kind == nodeElem && c.name == "Quiz.Answer" {
				answers = append(answers, c)
			} else {
				prompt = append(prompt, c)
			}
		}
		ps, err := r.block(prompt)
		if err != nil {
			return "", false, err
		}
		part := []string{labelled(fmt.Sprintf("**Question %d.**", q), ps)}
		var keys []string
		for j, a := range answers {
			var text, expl []*node
			for _, c := range a.children {
				if c.kind == nodeElem && c.name == "Quiz.Explanation" {
					expl = append(expl, c.children...)
				} else {
					text = append(text, c)
				}
			}
			as, err := r.block(text)
			if err != nil {
				return "", false, err
			}
			es, err := r.block(expl)
			if err != nil {
				return "", false, err
			}
			letter := fmt.Sprintf("**(%c)**", 'a'+j)
			part = append(part, labelled(letter, as))
			mark := "✗"
			if _, ok := a.attr("correct"); ok {
				mark = "✓"
			}
			keys = append(keys, labelled(letter+" "+mark, es))
		}
		if len(keys) > 0 {
			part = append(part, spoiler(fmt.Sprintf("Answer %d", q), strings.Join(keys, "\n\n")))
		}
		parts = append(parts, strings.Join(part, "\n\n"))
	}
	return strings.Join(parts, "\n\n"), true, nil
}

func (r *renderer) details(n *node) (string, bool, error) {
	summary := "Details"
	var body []*node
	for _, c := range n.children {
		if c.kind == nodeElem && c.name == "summary" {
			s, err := r.inline(c.children)
			if err != nil {
				return "", false, err
			}
			summary = s
			continue
		}
		body = append(body, c)
	}
	s, err := r.block(body)
	if err != nil {
		return "", false, err
	}
	return spoiler(summary, s), true, nil
}

func (r *renderer) table(n *node) (string, bool, error) {
	var rows [][]string
	var walk func([]*node) error
	walk = func(nodes []*node) error {
		for _, c := range nodes {
			if c.kind != nodeElem {
				continue
			}
			switch c.name {
			case "thead", "tbody", "tfoot":
				if err := walk(c.children); err != nil {
					return err
				}
			case "tr":
				var row []string
				for _, cell := range c.children {
					if cell.kind != nodeElem || (cell.name != "td" && cell.name != "th") {
						continue
					}
					s, err := r.inline(cell.children)
					if err != nil {
						return err
					}
					row = append(row, s)
					if span, _ := cell.attr("colSpan"); span != "" {
						k, _ := strconv.Atoi(span)
						for ; k > 1; k-- {
							row = append(row, "")
						}
					}
				}
				rows = append(rows, row)
			}
		}
		return nil
	}
	if err := walk(n.children); err != nil {
		return "", false, err
	}
	return mdTable(rows), true, nil
}

var (
	listItem = regexp.MustCompile(`^\s*([-*+]|\d{1,9}[.)])(\s|$)`)
	fenceRun = regexp.MustCompile("^(`{3,})([^`]*)$")
)

// normalize tidies rendered markdown: it un-indents lines MDX would never
// have read as indented code, trims trailing blanks, and collapses blank
// runs. Fenced blocks (always emitted by fenceBlock) pass through
// untouched apart from shifting with their fence.
func normalize(md string) string {
	var out []string
	inFence := false
	var fence string
	strip := 0 // columns removed from the current fence's lines
	inList := false
	blank := 0
	for _, l := range strings.Split(md, "\n") {
		if inFence {
			t := strings.TrimLeft(l, " \t")
			if strings.HasPrefix(t, fence) && strings.TrimSpace(strings.TrimLeft(t, "`")) == "" {
				inFence = false
			}
			out = append(out, stripCols(l, strip))
			continue
		}
		l = strings.TrimRight(l, " \t")
		t := strings.TrimLeft(l, " \t")
		if t == "" {
			blank++
			continue
		}
		ind := indentCols(l)
		switch {
		case fenceRun.MatchString(t):
			inFence = true
			fence = fenceRun.FindStringSubmatch(t)[1]
			strip = 0
			if ind >= 4 && !inList {
				strip = ind
			}
			l = stripCols(l, strip)
		case listItem.MatchString(l):
			inList = true
		case ind == 0:
			inList = false
		case ind >= 4 && !inList:
			l = t
		}
		if len(out) > 0 && blank > 0 {
			out = append(out, "")
		}
		blank = 0
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
