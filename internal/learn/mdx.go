package learn

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The MDX scanner splits a module body into markdown text, fenced code,
// display math and JSX elements. It is not a markdown parser: markdown text
// is passed through, except for the inline constructs that would otherwise
// be mistaken for JSX or mangled by a terminal renderer (code spans, math,
// escapes, images, root-relative links). The guide is compiled by MDX on
// every deploy, so its sources are well-formed MDX; anything this scanner
// cannot classify is an error with a line number rather than a guess, and
// the full-repo test keeps the error count at zero.

type nodeKind int

const (
	nodeText nodeKind = iota // markdown, inline constructs already converted
	nodeRaw                  // verbatim source (a fenced block inside ">")
	nodeCode                 // fenced code block
	nodeMath                 // $$ display math $$
	nodeElem                 // JSX element
)

type node struct {
	kind nodeKind
	text string // text, raw, code body, math body
	info string // code info string ("cpp", "java", ...)
	line int    // source line of the construct's start

	// bol: a text/raw node whose first line starts at a line start, so its
	// indentation is real indentation and not the tail of a previous line.
	bol bool
	// indent is the column of a code fence, $$ line or element tag that
	// starts its line; -1 when something precedes it on the line.
	indent int

	name     string
	attrs    []jsxAttr
	children []*node
	inline   bool // element shares its line with text
}

type jsxAttr struct {
	name  string
	value string // decoded string literal, or expression source if expr
	expr  bool
	bare  bool // `<Problems hideSuggestProblemButton />`
}

func (n *node) attr(name string) (string, bool) {
	for _, a := range n.attrs {
		if a.name == name {
			return a.value, true
		}
	}
	return "", false
}

type mdxScanner struct {
	file      string
	src       string
	pos       int
	lineStart []int // byte offset of each line, for error positions
	firstLine int   // source line of src[0]
	buf       strings.Builder
	bufBOL    bool
	// dropped counts MDX expressions that were not plain strings or
	// comments; they carry no prose in the guide but are worth knowing.
	dropped []string
}

func (s *mdxScanner) lineOf(pos int) int {
	i := sort.Search(len(s.lineStart), func(i int) bool { return s.lineStart[i] > pos })
	return s.firstLine + i - 1
}

func (s *mdxScanner) errf(pos int, format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", s.file, s.lineOf(pos), fmt.Sprintf(format, args...))
}

func parseMDX(file, body string, firstLine int) ([]*node, *mdxScanner, error) {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	s := &mdxScanner{file: file, src: body, firstLine: firstLine, lineStart: []int{0}}
	for i := 0; i < len(body); i++ {
		if body[i] == '\n' {
			s.lineStart = append(s.lineStart, i+1)
		}
	}
	nodes, err := s.parseChildren("", -1, 0)
	return nodes, s, err
}

func (s *mdxScanner) atLineStart() bool { return s.pos == 0 || s.src[s.pos-1] == '\n' }

// onlySpaceBefore reports whether only blanks precede pos on its line.
func (s *mdxScanner) onlySpaceBefore(pos int) bool {
	for i := pos - 1; i >= 0 && s.src[i] != '\n'; i-- {
		if s.src[i] != ' ' && s.src[i] != '\t' {
			return false
		}
	}
	return true
}

func (s *mdxScanner) lineEnd(pos int) int {
	if i := strings.IndexByte(s.src[pos:], '\n'); i >= 0 {
		return pos + i
	}
	return len(s.src)
}

func (s *mdxScanner) write(str string) {
	if s.buf.Len() == 0 {
		s.bufBOL = s.atLineStart() || s.onlySpaceBefore(s.pos)
	}
	s.buf.WriteString(str)
}

func (s *mdxScanner) flush(out *[]*node) {
	if s.buf.Len() == 0 {
		return
	}
	*out = append(*out, &node{kind: nodeText, text: s.buf.String(), bol: s.bufBOL})
	s.buf.Reset()
}

// parseChildren scans until the closing tag of parent ("" = top level).
func (s *mdxScanner) parseChildren(parent string, openPos, depth int) ([]*node, error) {
	var out []*node
	for s.pos < len(s.src) {
		if s.atLineStart() {
			ok, err := s.blockConstruct(&out, depth)
			if err != nil {
				return nil, err
			}
			if ok {
				continue
			}
		}
		c := s.src[s.pos]
		switch c {
		case '\\':
			if s.pos+1 < len(s.src) && isASCIIPunct(s.src[s.pos+1]) {
				s.write(s.src[s.pos : s.pos+2])
				s.pos += 2
				continue
			}
		case '`':
			if s.codeSpan() {
				continue
			}
		case '$':
			if s.inlineMath() {
				continue
			}
		case '!':
			if s.image() {
				continue
			}
		case '(':
			if s.pos+2 < len(s.src) && s.src[s.pos+1] == '/' && s.src[s.pos+2] != '/' &&
				strings.HasSuffix(s.buf.String(), "]") {
				s.write("(" + siteBase)
				s.pos++
				continue
			}
		case '{':
			if err := s.expression(); err != nil {
				return nil, err
			}
			continue
		case '<':
			done, err := s.angle(&out, parent, depth)
			if err != nil {
				return nil, err
			}
			if done {
				s.flush(&out)
				return out, nil
			}
			continue
		}
		s.write(string(c))
		s.pos++
	}
	if parent != "" {
		return nil, s.errf(openPos, "<%s> is never closed", parent)
	}
	s.flush(&out)
	return out, nil
}

func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}

// indentCols measures leading blanks with tabs at 4-column stops, the
// CommonMark rule.
func indentCols(l string) int {
	col := 0
	for i := 0; i < len(l); i++ {
		switch l[i] {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return col
		}
	}
	return col
}

// stripCols removes up to n columns of leading blanks; a tab that straddles
// the boundary leaves its remainder as spaces.
func stripCols(l string, n int) string {
	col := 0
	for i := 0; i < len(l); i++ {
		if col >= n {
			return l[i:]
		}
		switch l[i] {
		case ' ':
			col++
		case '\t':
			next := col + 4 - col%4
			if next > n {
				return strings.Repeat(" ", next-n) + l[i+1:]
			}
			col = next
		default:
			return l[i:]
		}
	}
	return ""
}

var fenceOpen = regexp.MustCompile("^(`{3,}|~{3,})(.*)$")

// blockConstruct handles constructs recognized only at a line start: code
// fences (also inside blockquotes), $$ display math, and ESM lines.
func (s *mdxScanner) blockConstruct(out *[]*node, depth int) (bool, error) {
	start := s.pos
	end := s.lineEnd(start)
	line := s.src[start:end]
	trimmed := strings.TrimLeft(line, " \t")
	ind := indentCols(line)

	// Blockquote prefix: "> ```".
	quoted := false
	rest := trimmed
	for strings.HasPrefix(rest, ">") {
		quoted = true
		rest = strings.TrimLeft(rest[1:], " \t")
	}

	if m := fenceOpen.FindStringSubmatch(rest); m != nil {
		fence, info := m[1], strings.TrimSpace(m[2])
		if fence[0] == '`' && strings.Contains(info, "`") {
			return false, nil // inline code at line start, not a fence
		}
		if quoted {
			return true, s.quotedFence(out, fence)
		}
		return true, s.codeFence(out, fence, info, ind)
	}

	if !quoted && strings.HasPrefix(trimmed, "$$") {
		return true, s.displayMath(out, ind)
	}

	if depth == 0 && ind == 0 && s.afterBlankLine(start) &&
		(strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "export ")) {
		// ESM starts a paragraph (it cannot interrupt one) and runs to the
		// next blank line.
		s.flush(out)
		for s.pos < len(s.src) {
			e := s.lineEnd(s.pos)
			blank := strings.TrimSpace(s.src[s.pos:e]) == ""
			s.pos = min(e+1, len(s.src))
			if blank {
				break
			}
		}
		return true, nil
	}
	return false, nil
}

func isClosingFence(l, fence string) bool {
	t := strings.TrimLeft(l, " \t")
	n := 0
	for n < len(t) && t[n] == fence[0] {
		n++
	}
	return n >= len(fence) && strings.TrimSpace(t[n:]) == ""
}

func (s *mdxScanner) codeFence(out *[]*node, fence, info string, ind int) error {
	open := s.pos
	s.flush(out)
	s.pos = min(s.lineEnd(s.pos)+1, len(s.src))
	var body []string
	for {
		if s.pos >= len(s.src) {
			return s.errf(open, "code fence %s is never closed", fence)
		}
		e := s.lineEnd(s.pos)
		l := s.src[s.pos:e]
		s.pos = min(e+1, len(s.src))
		if isClosingFence(l, fence) {
			break
		}
		body = append(body, stripCols(l, ind))
	}
	*out = append(*out, &node{
		kind: nodeCode, text: strings.Join(body, "\n"), info: info,
		line: s.lineOf(open), indent: ind,
	})
	return nil
}

// quotedFence keeps a fenced block inside a blockquote verbatim; the
// blockquote ends the block if the fence is never closed.
func (s *mdxScanner) quotedFence(out *[]*node, fence string) error {
	s.flush(out)
	start := s.pos
	s.pos = min(s.lineEnd(s.pos)+1, len(s.src))
	for s.pos < len(s.src) {
		e := s.lineEnd(s.pos)
		l := strings.TrimLeft(s.src[s.pos:e], " \t")
		if !strings.HasPrefix(l, ">") {
			break
		}
		s.pos = min(e+1, len(s.src))
		for strings.HasPrefix(l, ">") {
			l = strings.TrimLeft(l[1:], " \t")
		}
		if isClosingFence(l, fence) {
			break
		}
	}
	*out = append(*out, &node{kind: nodeRaw, text: s.src[start:s.pos], bol: true, line: s.lineOf(start)})
	return nil
}

func (s *mdxScanner) displayMath(out *[]*node, ind int) error {
	open := s.pos
	s.flush(out)
	e := s.lineEnd(s.pos)
	first := strings.TrimSpace(s.src[s.pos:e])[2:]
	s.pos = min(e+1, len(s.src))
	if i := strings.Index(first, "$$"); i >= 0 {
		if strings.TrimSpace(first[i+2:]) != "" {
			return s.errf(open, "text after closing $$")
		}
		*out = append(*out, &node{kind: nodeMath, text: strings.TrimSpace(first[:i]), line: s.lineOf(open), indent: ind})
		return nil
	}
	var body []string
	if strings.TrimSpace(first) != "" {
		body = append(body, strings.TrimSpace(first))
	}
	for {
		if s.pos >= len(s.src) {
			return s.errf(open, "$$ display math is never closed")
		}
		e := s.lineEnd(s.pos)
		l := s.src[s.pos:e]
		s.pos = min(e+1, len(s.src))
		t := strings.TrimSpace(l)
		if strings.HasSuffix(t, "$$") {
			if t = strings.TrimSpace(strings.TrimSuffix(t, "$$")); t != "" {
				body = append(body, t)
			}
			break
		}
		body = append(body, stripCols(l, ind))
	}
	*out = append(*out, &node{kind: nodeMath, text: strings.Join(body, "\n"), line: s.lineOf(open), indent: ind})
	return nil
}

// runLen counts repeats of c at pos.
func (s *mdxScanner) runLen(pos int, c byte) int {
	n := 0
	for pos+n < len(s.src) && s.src[pos+n] == c {
		n++
	}
	return n
}

// findRun finds the next run of exactly n copies of c after from, without
// crossing a blank line (a paragraph break ends inline constructs).
func (s *mdxScanner) findRun(from, n int, c byte) int {
	for i := from; i < len(s.src); i++ {
		switch s.src[i] {
		case c:
			r := s.runLen(i, c)
			if r == n {
				return i
			}
			i += r - 1
		case '\n':
			j := i + 1
			for j < len(s.src) && (s.src[j] == ' ' || s.src[j] == '\t') {
				j++
			}
			if j >= len(s.src) || s.src[j] == '\n' {
				return -1
			}
		case '\\':
			if c == '$' {
				i++ // an escaped dollar does not close math
			}
		}
	}
	return -1
}

func (s *mdxScanner) codeSpan() bool {
	n := s.runLen(s.pos, '`')
	end := s.findRun(s.pos+n, n, '`')
	if end < 0 {
		s.write(strings.Repeat("`", n))
		s.pos += n
		return true
	}
	s.write(s.src[s.pos : end+n])
	s.pos = end + n
	return true
}

func (s *mdxScanner) inlineMath() bool {
	n := s.runLen(s.pos, '$')
	if n > 2 {
		return false
	}
	end := s.findRun(s.pos+n, n, '$')
	if end < 0 || strings.TrimSpace(s.src[s.pos+n:end]) == "" {
		s.write(strings.Repeat("$", n))
		s.pos += n
		return true
	}
	tex := s.src[s.pos+n : end]
	s.write(inlineTeX(tex))
	s.pos = end + n
	return true
}

// image rewrites ![alt](url "title") to [image: alt](absolute url): a
// terminal cannot show it, but the link can be opened.
func (s *mdxScanner) image() bool {
	if s.pos+1 >= len(s.src) || s.src[s.pos+1] != '[' {
		return false
	}
	altEnd := matchBracket(s.src, s.pos+1, '[', ']')
	if altEnd < 0 || altEnd+1 >= len(s.src) || s.src[altEnd+1] != '(' {
		return false
	}
	destEnd := matchBracket(s.src, altEnd+1, '(', ')')
	if destEnd < 0 {
		return false
	}
	alt := strings.Join(strings.Fields(s.src[s.pos+2:altEnd]), " ")
	dest := strings.TrimSpace(s.src[altEnd+2 : destEnd])
	if strings.HasPrefix(dest, "<") {
		if i := strings.IndexByte(dest, '>'); i > 0 {
			dest = dest[1:i]
		}
	} else if i := strings.IndexAny(dest, " \t\n"); i >= 0 {
		dest = dest[:i]
	}
	s.write(imageLink(escapeLinkText(alt), dest))
	s.pos = destEnd + 1
	return true
}

// imageLink takes alt as markdown (already escaped), since ![alt] text is
// markdown in the source.
func imageLink(alt, src string) string {
	label := "image"
	if alt != "" {
		label = "image: " + alt
	}
	return "[" + label + "](" + absURL(src) + ")"
}

// matchBracket returns the index of the bracket closing the one at open,
// honoring backslash escapes and stopping at a blank line.
func matchBracket(src string, open int, o, c byte) int {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case o:
			depth++
		case c:
			depth--
			if depth == 0 {
				return i
			}
		case '\n':
			if i+1 < len(src) && src[i+1] == '\n' {
				return -1
			}
		}
	}
	return -1
}

var autolink = regexp.MustCompile(`^<[A-Za-z][A-Za-z0-9+.-]{1,31}:[^\s<>]*>`)

// angle handles "<": comments, autolinks, <...> link destinations and JSX
// tags. done is true when the parent's closing tag was consumed.
func (s *mdxScanner) angle(out *[]*node, parent string, depth int) (done bool, err error) {
	rest := s.src[s.pos:]
	switch {
	case strings.HasPrefix(rest, "<!--"):
		end := strings.Index(rest, "-->")
		if end < 0 {
			return false, s.errf(s.pos, "HTML comment is never closed")
		}
		s.pos += end + 3
		return false, nil
	case strings.HasSuffix(s.buf.String(), "]("):
		end := strings.IndexAny(rest, ">\n")
		if end < 0 || rest[end] != '>' {
			return false, s.errf(s.pos, "unterminated <link destination>")
		}
		s.write(rest[:end+1])
		s.pos += end + 1
		return false, nil
	case autolink.MatchString(rest):
		m := autolink.FindString(rest)
		s.write(m)
		s.pos += len(m)
		return false, nil
	}
	if len(rest) < 2 || !(isJSXNameStart(rest[1]) || rest[1] == '>' || rest[1] == '/') {
		s.write("<")
		s.pos++
		return false, nil
	}
	tagPos := s.pos
	t, err := s.tag()
	if err != nil {
		return false, err
	}
	if t.name == "" {
		t.name = fragment
	}
	if t.closing {
		if parent == "" {
			return false, s.errf(tagPos, "</%s> closes nothing", t.name)
		}
		if t.name != parent {
			return false, s.errf(tagPos, "</%s> closes <%s>", t.name, parent)
		}
		return true, nil
	}
	s.flush(out)
	n := &node{kind: nodeElem, name: t.name, attrs: t.attrs, line: s.lineOf(tagPos), indent: -1}
	if s.onlySpaceBefore(tagPos) {
		n.indent = indentCols(s.src[s.lineStart[s.lineOf(tagPos)-s.firstLine]:tagPos])
		// A tag followed by text on its own line is inline in MDX.
		after := strings.TrimSpace(s.src[s.pos:s.lineEnd(s.pos)])
		n.inline = after != "" && !strings.HasPrefix(after, "<") && !strings.HasPrefix(after, "{")
	} else {
		n.inline = true
	}
	if !t.selfClosing {
		kids, err := s.parseChildren(t.name, tagPos, depth+1)
		if err != nil {
			return false, err
		}
		n.children = kids
	}
	*out = append(*out, n)
	return false, nil
}

// fragment names <>...</> so it cannot collide with the top level's "".
const fragment = "#fragment"

func isJSXNameStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isJSXNamePart(c byte) bool {
	return isJSXNameStart(c) || c >= '0' && c <= '9' || c == '.' || c == '-' || c == ':'
}

type jsxTag struct {
	name        string
	attrs       []jsxAttr
	closing     bool
	selfClosing bool
}

func (s *mdxScanner) skipBlanks() {
	for s.pos < len(s.src) && strings.IndexByte(" \t\n", s.src[s.pos]) >= 0 {
		s.pos++
	}
}

func (s *mdxScanner) jsxName() string {
	start := s.pos
	if s.pos < len(s.src) && isJSXNameStart(s.src[s.pos]) {
		for s.pos < len(s.src) && isJSXNamePart(s.src[s.pos]) {
			s.pos++
		}
	}
	return s.src[start:s.pos]
}

func (s *mdxScanner) tag() (jsxTag, error) {
	open := s.pos
	s.pos++ // <
	var t jsxTag
	if s.pos < len(s.src) && s.src[s.pos] == '/' {
		t.closing = true
		s.pos++
	}
	s.skipBlanks()
	t.name = s.jsxName()
	for {
		s.skipBlanks()
		if s.pos >= len(s.src) {
			return t, s.errf(open, "tag <%s is never closed", t.name)
		}
		c := s.src[s.pos]
		switch {
		case c == '>':
			s.pos++
			return t, nil
		case c == '/' && !t.closing && s.pos+1 < len(s.src) && s.src[s.pos+1] == '>':
			s.pos += 2
			t.selfClosing = true
			return t, nil
		case t.closing:
			return t, s.errf(s.pos, "unexpected %q in closing tag </%s>", c, t.name)
		case c == '{':
			// {...spread} attribute: no prose to keep.
			src, err := s.exprSource()
			if err != nil {
				return t, err
			}
			t.attrs = append(t.attrs, jsxAttr{name: "...", value: src, expr: true})
		case isJSXNameStart(c):
			a := jsxAttr{name: s.jsxName()}
			s.skipBlanks()
			if s.pos < len(s.src) && s.src[s.pos] == '=' {
				s.pos++
				s.skipBlanks()
				if s.pos >= len(s.src) {
					return t, s.errf(open, "attribute %s has no value", a.name)
				}
				switch q := s.src[s.pos]; q {
				case '"', '\'':
					end := strings.IndexByte(s.src[s.pos+1:], q)
					if end < 0 {
						return t, s.errf(s.pos, "attribute %s: unterminated string", a.name)
					}
					a.value = s.src[s.pos+1 : s.pos+1+end]
					s.pos += end + 2
				case '{':
					src, err := s.exprSource()
					if err != nil {
						return t, err
					}
					a.expr = true
					a.value = src
					if v, ok := jsString(src); ok {
						a.value, a.expr = v, false
					}
				default:
					return t, s.errf(s.pos, "attribute %s: unexpected %q", a.name, q)
				}
			} else {
				a.bare = true
			}
			t.attrs = append(t.attrs, a)
		default:
			return t, s.errf(s.pos, "unexpected %q in tag <%s>", c, t.name)
		}
	}
}

// exprSource reads a balanced {...} JS expression starting at "{" and
// returns its inside, skipping strings, template literals and comments.
func (s *mdxScanner) exprSource() (string, error) {
	open := s.pos
	depth := 0
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				s.pos++
				return s.src[open+1 : s.pos-1], nil
			}
		case c == '"' || c == '\'' || c == '`':
			for s.pos++; s.pos < len(s.src) && s.src[s.pos] != c; s.pos++ {
				if s.src[s.pos] == '\\' {
					s.pos++
				}
			}
		case strings.HasPrefix(s.src[s.pos:], "/*"):
			end := strings.Index(s.src[s.pos+2:], "*/")
			if end < 0 {
				return "", s.errf(open, "comment in {expression} is never closed")
			}
			s.pos += 2 + end + 1
		case strings.HasPrefix(s.src[s.pos:], "//"):
			s.pos = s.lineEnd(s.pos) - 1
		}
		s.pos++
	}
	return "", s.errf(open, "{expression} is never closed")
}

var blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// jsString evaluates an expression that is just a string literal.
func jsString(src string) (string, bool) {
	t := strings.TrimSpace(blockComment.ReplaceAllString(src, ""))
	if len(t) < 2 {
		return "", false
	}
	q := t[0]
	if (q != '"' && q != '\'' && q != '`') || t[len(t)-1] != q {
		return "", false
	}
	inner := t[1 : len(t)-1]
	if q == '`' && strings.Contains(inner, "${") {
		return "", false
	}
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c == q {
			return "", false // two literals concatenated, or similar
		}
		if c == '\\' && i+1 < len(inner) {
			i++
			switch inner[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(inner[i])
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String(), true
}

// expression handles a {...} in prose: comments vanish, string literals
// (the guide's `{' '}`) become text, anything else is dropped and noted.
func (s *mdxScanner) expression() error {
	src, err := s.exprSource()
	if err != nil {
		return err
	}
	if strings.TrimSpace(blockComment.ReplaceAllString(src, "")) == "" {
		return nil
	}
	if v, ok := jsString(src); ok {
		s.write(v)
		return nil
	}
	s.dropped = append(s.dropped, src)
	return nil
}

// afterBlankLine reports whether the line starting at pos is the first line
// of the input or follows a blank line.
func (s *mdxScanner) afterBlankLine(pos int) bool {
	if pos == 0 {
		return true
	}
	prevStart := strings.LastIndexByte(s.src[:pos-1], '\n') + 1
	return strings.TrimSpace(s.src[prevStart:pos-1]) == ""
}
