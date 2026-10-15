package curriculum

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ordering.ts is a TypeScript module, not JSON: it has comments, single
// quotes, trailing commas, a type annotation on the declaration and code
// after it. A regex over it would break on the first reformat, so this file
// tokenizes the module and parses only the object literals it needs. Any
// syntax the parser does not expect is an error naming file:line, so a guide
// update that changes the shape fails loudly instead of silently dropping
// modules.

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokString // '...', "..." and `...`
	tokNumber
	tokPunct
)

type token struct {
	kind tokKind
	text string // identifier name, decoded string, number or punctuation
	line int
	// tmpl marks a template literal with ${} substitutions, which is not a
	// constant; its text leaves the substitutions out.
	tmpl bool
}

func (t token) String() string {
	if t.kind == tokEOF {
		return "end of file"
	}
	return strconv.Quote(t.text)
}

// tsLexer produces tokens on demand, so a construct it cannot read after the
// declarations the parser needs never causes an error.
type tsLexer struct {
	file string
	src  string
	pos  int
	line int
}

func newTSLexer(file, src string) *tsLexer {
	return &tsLexer{file: file, src: src, line: 1}
}

func (l *tsLexer) errf(line int, format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", l.file, line, fmt.Sprintf(format, args...))
}

// multi-character punctuators that must not be split, so "=>" is never read
// as the "=" that ends a type annotation.
var tsPuncts = []string{
	"...", "===", "!==", "**=", "<<=", ">>=", ">>>",
	"=>", "==", "!=", "<=", ">=", "&&", "||", "??", "?.", "++", "--",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "**", "<<", ">>",
}

func (l *tsLexer) next() (token, error) {
	if err := l.skipSpace(); err != nil {
		return token{}, err
	}
	if l.pos >= len(l.src) {
		return token{kind: tokEOF, line: l.line}, nil
	}
	c := l.src[l.pos]
	line := l.line
	switch {
	case c == '\'' || c == '"':
		s, err := l.quoted(c)
		return token{kind: tokString, text: s, line: line}, err
	case c == '`':
		s, tmpl, err := l.template()
		return token{kind: tokString, text: s, line: line, tmpl: tmpl}, err
	case isIdentStart(c):
		start := l.pos
		for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
			l.pos++
		}
		return token{kind: tokIdent, text: l.src[start:l.pos], line: line}, nil
	case c >= '0' && c <= '9':
		start := l.pos
		for l.pos < len(l.src) && (isIdentPart(l.src[l.pos]) || l.src[l.pos] == '.') {
			l.pos++
		}
		return token{kind: tokNumber, text: l.src[start:l.pos], line: line}, nil
	}
	for _, p := range tsPuncts {
		if strings.HasPrefix(l.src[l.pos:], p) {
			l.pos += len(p)
			return token{kind: tokPunct, text: p, line: line}, nil
		}
	}
	l.pos++
	return token{kind: tokPunct, text: string(c), line: line}, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }

func (l *tsLexer) skipSpace() error {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.pos++
		case strings.HasPrefix(l.src[l.pos:], "//"):
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case strings.HasPrefix(l.src[l.pos:], "/*"):
			start := l.line
			end := strings.Index(l.src[l.pos+2:], "*/")
			if end < 0 {
				return l.errf(start, "unterminated block comment")
			}
			body := l.src[l.pos : l.pos+2+end+2]
			l.line += strings.Count(body, "\n")
			l.pos += len(body)
		default:
			return nil
		}
	}
	return nil
}

// quoted reads a '...' or "..." literal.
func (l *tsLexer) quoted(q byte) (string, error) {
	line := l.line
	l.pos++ // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		switch c := l.src[l.pos]; c {
		case q:
			l.pos++
			return b.String(), nil
		case '\n':
			return "", l.errf(line, "unterminated string literal")
		case '\\':
			if err := l.escape(&b, line); err != nil {
				return "", err
			}
		default:
			b.WriteByte(c)
			l.pos++
		}
	}
	return "", l.errf(line, "unterminated string literal")
}

// escape decodes the backslash escape at l.pos into b, covering the ones a
// guide string could plausibly contain. line is where the literal starts.
func (l *tsLexer) escape(b *strings.Builder, line int) error {
	if l.pos+1 >= len(l.src) {
		return l.errf(line, "unterminated string literal")
	}
	e := l.src[l.pos+1]
	l.pos += 2
	switch e {
	case 'n':
		b.WriteByte('\n')
	case 't':
		b.WriteByte('\t')
	case 'r':
		b.WriteByte('\r')
	case '0':
		b.WriteByte(0)
	case '\n': // line continuation
		l.line++
	case 'u', 'x':
		n := 4
		if e == 'x' {
			n = 2
		}
		if l.pos+n > len(l.src) {
			return l.errf(l.line, "bad \\%c escape", e)
		}
		v, err := strconv.ParseUint(l.src[l.pos:l.pos+n], 16, 32)
		if err != nil {
			return l.errf(l.line, "bad \\%c escape", e)
		}
		b.WriteRune(rune(v))
		l.pos += n
	default:
		b.WriteByte(e)
	}
	return nil
}

// template reads a `...` literal. Substitutions are skipped with brace
// matching and make the literal tmpl.
func (l *tsLexer) template() (string, bool, error) {
	line := l.line
	l.pos++
	var b strings.Builder
	tmpl := false
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '`':
			l.pos++
			return b.String(), tmpl, nil
		case c == '\\':
			if err := l.escape(&b, line); err != nil {
				return "", false, err
			}
		case c == '$' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '{':
			tmpl = true
			l.pos += 2
			for depth := 1; depth > 0; {
				t, err := l.next()
				if err != nil {
					return "", false, err
				}
				switch {
				case t.kind == tokEOF:
					return "", false, l.errf(line, "unterminated template literal")
				case t.kind == tokPunct && t.text == "{":
					depth++
				case t.kind == tokPunct && t.text == "}":
					depth--
				}
			}
		default:
			if c == '\n' {
				l.line++
			}
			b.WriteByte(c)
			l.pos++
		}
	}
	return "", false, l.errf(line, "unterminated template literal")
}

// tsParser parses constant object/array literals out of the token stream.
type tsParser struct {
	lx  *tsLexer
	tok token
}

func (p *tsParser) advance() error {
	t, err := p.lx.next()
	p.tok = t
	return err
}

func (p *tsParser) is(text string) bool {
	return p.tok.kind == tokPunct && p.tok.text == text
}

func (p *tsParser) expect(text string) error {
	if !p.is(text) {
		return p.lx.errf(p.tok.line, "unexpected %s, want %q", p.tok, text)
	}
	return p.advance()
}

// tsValue is a parsed constant: string, float64, bool, nil, []tsValue or
// *tsObject. Objects keep key order because division order is meaning.
type tsValue any

type tsObject struct {
	keys []string
	vals map[string]tsValue
	line map[string]int
}

func (p *tsParser) value() (tsValue, error) {
	t := p.tok
	switch {
	case p.is("{"):
		o, err := p.object()
		return o, err
	case p.is("["):
		a, err := p.array()
		return a, err
	case t.kind == tokString:
		if t.tmpl {
			return nil, p.lx.errf(t.line, "template literal with substitutions is not a constant")
		}
		return t.text, p.advance()
	case t.kind == tokNumber:
		f, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, p.lx.errf(t.line, "bad number %s", t)
		}
		return f, p.advance()
	case t.kind == tokIdent && (t.text == "true" || t.text == "false"):
		return t.text == "true", p.advance()
	case t.kind == tokIdent && t.text == "null":
		return nil, p.advance()
	}
	return nil, p.lx.errf(t.line, "unexpected %s, want a constant literal", t)
}

func (p *tsParser) object() (*tsObject, error) {
	o := &tsObject{vals: map[string]tsValue{}, line: map[string]int{}}
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	for !p.is("}") {
		var key string
		switch p.tok.kind {
		case tokIdent, tokNumber:
			key = p.tok.text
		case tokString:
			if p.tok.tmpl {
				return nil, p.lx.errf(p.tok.line, "computed key %s", p.tok)
			}
			key = p.tok.text
		default:
			return nil, p.lx.errf(p.tok.line, "unexpected %s, want an object key", p.tok)
		}
		line := p.tok.line
		if _, dup := o.vals[key]; dup {
			return nil, p.lx.errf(line, "duplicate key %q", key)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		if err := p.expect(":"); err != nil {
			return nil, err
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		o.keys = append(o.keys, key)
		o.vals[key] = v
		o.line[key] = line
		if p.is(",") {
			if err := p.advance(); err != nil {
				return nil, err
			}
		} else if !p.is("}") {
			return nil, p.lx.errf(p.tok.line, "unexpected %s, want \",\" or \"}\"", p.tok)
		}
	}
	return o, p.advance()
}

func (p *tsParser) array() ([]tsValue, error) {
	if err := p.expect("["); err != nil {
		return nil, err
	}
	var a []tsValue
	for !p.is("]") {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		a = append(a, v)
		if p.is(",") {
			if err := p.advance(); err != nil {
				return nil, err
			}
		} else if !p.is("]") {
			return nil, p.lx.errf(p.tok.line, "unexpected %s, want \",\" or \"]\"", p.tok)
		}
	}
	return a, p.advance()
}

// findConsts scans top-level tokens for `const NAME (: Type)? = <literal>`
// (also let/var, with or without export) and parses each wanted name. It
// stops as soon as every wanted name is found so trailing code is never
// tokenized. Missing names are absent from the result.
func findConsts(file, src string, names ...string) (map[string]tsValue, map[string]int, error) {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	p := &tsParser{lx: newTSLexer(file, src)}
	if err := p.advance(); err != nil {
		return nil, nil, err
	}
	out := map[string]tsValue{}
	lines := map[string]int{}
	prev := token{}
	for p.tok.kind != tokEOF && len(out) < len(want) {
		t := p.tok
		decl := prev.kind == tokIdent && (prev.text == "const" || prev.text == "let" || prev.text == "var")
		if t.kind == tokIdent && want[t.text] && decl {
			if _, dup := out[t.text]; dup {
				return nil, nil, p.lx.errf(t.line, "%s declared twice", t.text)
			}
			if err := p.advance(); err != nil {
				return nil, nil, err
			}
			if p.is(":") {
				if err := p.skipType(); err != nil {
					return nil, nil, err
				}
			}
			if err := p.expect("="); err != nil {
				return nil, nil, err
			}
			v, err := p.value()
			if err != nil {
				return nil, nil, fmt.Errorf("parsing %s: %w", t.text, err)
			}
			out[t.text] = v
			lines[t.text] = t.line
			// `as const` / `satisfies T` after the literal is harmless; the
			// scan simply continues past it.
			prev = token{}
			continue
		}
		prev = t
		if err := p.advance(); err != nil {
			return nil, nil, err
		}
	}
	return out, lines, nil
}

// skipType skips a type annotation up to the "=" that starts the
// initializer, balancing brackets so "=" inside a type is not mistaken for
// it.
func (p *tsParser) skipType() error {
	start := p.tok.line
	depth := 0
	for {
		if err := p.advance(); err != nil {
			return err
		}
		switch {
		case p.tok.kind == tokEOF:
			return p.lx.errf(start, "unterminated type annotation")
		case p.is("{") || p.is("[") || p.is("("):
			depth++
		case p.is("}") || p.is("]") || p.is(")"):
			depth--
			if depth < 0 {
				return p.lx.errf(p.tok.line, "unbalanced %s in type annotation", p.tok)
			}
		case p.is("=") && depth == 0:
			return nil
		case p.is(";") && depth == 0:
			return p.lx.errf(p.tok.line, "declaration has no initializer")
		}
	}
}

// Division labels used when ordering.ts has no SECTION_LABELS.
var defaultDivisionTitles = map[string]string{
	"general": "General",
	"bronze":  "Bronze",
	"silver":  "Silver",
	"gold":    "Gold",
	"plat":    "Platinum",
	"adv":     "Advanced",
}

// ParseOrdering parses the guide's content/ordering.ts into the curriculum.
func ParseOrdering(path string) ([]Division, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseOrdering(path, string(b))
}

func parseOrdering(file, src string) ([]Division, error) {
	consts, lines, err := findConsts(file, src, "MODULE_ORDERING", "SECTION_LABELS")
	if err != nil {
		return nil, err
	}
	raw, ok := consts["MODULE_ORDERING"]
	if !ok {
		return nil, fmt.Errorf("%s: no `const MODULE_ORDERING = {...}` declaration", file)
	}
	errAt := func(line int, format string, args ...any) error {
		return fmt.Errorf("%s:%d: %s", file, line, fmt.Sprintf(format, args...))
	}
	root, ok := raw.(*tsObject)
	if !ok {
		return nil, errAt(lines["MODULE_ORDERING"], "MODULE_ORDERING is not an object")
	}
	labels := map[string]string{}
	if lv, ok := consts["SECTION_LABELS"].(*tsObject); ok {
		for _, k := range lv.keys {
			s, ok := lv.vals[k].(string)
			if !ok {
				return nil, errAt(lv.line[k], "SECTION_LABELS.%s is not a string", k)
			}
			labels[k] = s
		}
	}
	seen := map[string]string{}
	var divs []Division
	for _, divID := range root.keys {
		title := labels[divID]
		if title == "" {
			title = defaultDivisionTitles[divID]
		}
		if title == "" {
			title = divID
		}
		d := Division{ID: divID, Title: title}
		chapters, ok := root.vals[divID].([]tsValue)
		if !ok {
			return nil, errAt(root.line[divID], "division %q is not an array of chapters", divID)
		}
		for i, cv := range chapters {
			co, ok := cv.(*tsObject)
			if !ok {
				return nil, errAt(root.line[divID], "division %q chapter %d is not an object", divID, i)
			}
			name, ok := co.vals["name"].(string)
			if !ok {
				return nil, errAt(root.line[divID], "division %q chapter %d has no string name", divID, i)
			}
			ch := Chapter{Title: name}
			if desc, ok := co.vals["description"]; ok {
				s, ok := desc.(string)
				if !ok {
					return nil, errAt(co.line["description"], "chapter %q description is not a string", name)
				}
				ch.Description = s
			}
			items, ok := co.vals["items"].([]tsValue)
			if !ok {
				return nil, errAt(co.line["name"], "chapter %q has no items array", name)
			}
			for _, iv := range items {
				id, ok := iv.(string)
				if !ok || id == "" {
					return nil, errAt(co.line["items"], "chapter %q has a non-string item", name)
				}
				if prev, dup := seen[id]; dup {
					return nil, errAt(co.line["items"], "module %q listed in both %s and %s", id, prev, divID)
				}
				seen[id] = divID
				ch.Modules = append(ch.Modules, ModuleRef{ID: id})
			}
			d.Chapters = append(d.Chapters, ch)
		}
		divs = append(divs, d)
	}
	if len(divs) == 0 {
		return nil, errAt(lines["MODULE_ORDERING"], "MODULE_ORDERING is empty")
	}
	return divs, nil
}
