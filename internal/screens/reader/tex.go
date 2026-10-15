package reader

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TeXToUnicode rewrites the TeX math in a markdown document into plain
// Unicode, so statements read well in a terminal that cannot run MathJax:
//
//	$1 \le N \le 2 \cdot 10^5$    →  1 ≤ N ≤ 2 · 10⁵
//	$\lfloor a_i / 2 \rfloor$     →  ⌊aᵢ / 2⌋
//	$a_1, a_2, \ldots, a_N$       →  a[1], a[2], …, a[N]
//
// Math is whatever sits between $…$, $$…$$, $$$…$$$ or $$$$$$…$$$$$$
// (Codeforces inline and display), or \(…\). MathJax's \[…\] is not math
// here: in markdown \[ is an escaped bracket, and html-to-markdown and the
// curriculum write it for every literal "[" (A\[i\] read as math was "Ai").
// Text outside math is left alone, and so are fenced code blocks and inline
// code spans, because sample input must reach the judge's reader byte for
// byte.
//
// Subscripts become Unicode subscripts when every character has one (aᵢ,
// x₁₀); otherwise the whole math span switches to brackets (a[N], b[i,j]) so
// one formula never mixes the two styles. Superscripts fall back per script
// (2^Q, 2^(⌊n/2⌋)) since mixing is less jarring there.
//
// What a terminal line cannot stack is set linearly so it reads the same:
//
//	$\frac{3}{2}r$, $2\frac{2}{3}$         →  (3/2)r, 2⅔
//	${n \choose k}$                     →  C(n, k)
//	$\begin{cases} x & x > 0 \\ 0 & \text{else} \end{cases}$  →  {x, x > 0; 0, else}
//	$\begin{pmatrix} a & b \\ c & d \end{pmatrix}$          →  (a, b; c, d)
//
// Where Unicode has no clean form the result stays readable, and the page
// online (o) has the typeset original.
//
// The output is markdown: characters the conversion produces that markdown
// would read as syntax (*, _, <, [, |, `, \) are backslash-escaped.
func TeXToUnicode(s string) string {
	var b strings.Builder
	for _, seg := range splitFences(s) {
		if seg.code {
			b.WriteString(seg.text)
			continue
		}
		b.WriteString(convertProse(seg.text))
	}
	return b.String()
}

// segment is a run of markdown lines that is either inside a fenced code
// block (code) or not.
type segment struct {
	code bool
	text string
}

// splitFences cuts markdown into fenced code blocks and everything else. A
// fence opens on a line of three or more backticks or tildes (indented at
// most three spaces) and closes on a line of at least as many of the same
// character; an unclosed fence runs to the end, as in CommonMark. Lines keep
// their newlines so joining the segments gives back the input.
func splitFences(s string) []segment {
	var segs []segment
	var cur strings.Builder
	inCode := false
	var fenceCh byte
	fenceLen := 0
	flush := func(code bool) {
		if cur.Len() > 0 {
			segs = append(segs, segment{code: code, text: cur.String()})
			cur.Reset()
		}
	}
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		line := s
		if i >= 0 {
			line = s[:i+1]
		}
		s = s[len(line):]
		ch, n, info := fenceLine(line)
		// A backtick fence cannot have a backtick in its info string.
		opens := n >= 3 && (ch != '`' || !strings.ContainsRune(info, '`'))
		switch {
		case !inCode && opens:
			flush(false)
			inCode, fenceCh, fenceLen = true, ch, n
			cur.WriteString(line)
		case inCode && ch == fenceCh && n >= fenceLen && strings.TrimSpace(info) == "":
			cur.WriteString(line)
			flush(true)
			inCode = false
		default:
			cur.WriteString(line)
		}
	}
	flush(inCode)
	return segs
}

// fenceLine reports the fence character, the run length and the rest of the
// line when line looks like a code fence; n is 0 otherwise.
func fenceLine(line string) (ch byte, n int, info string) {
	t := strings.TrimRight(line, "\r\n")
	indent := len(t) - len(strings.TrimLeft(t, " "))
	if indent > 3 {
		return 0, 0, ""
	}
	t = t[indent:]
	if t == "" || (t[0] != '`' && t[0] != '~') {
		return 0, 0, ""
	}
	ch = t[0]
	for n < len(t) && t[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, ""
	}
	return ch, n, t[n:]
}

// convertProse rewrites math in markdown text that holds no fenced code.
func convertProse(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '`':
			// Inline code: copy through the matching run of backticks.
			n := runLen(s, i, '`')
			end := findRun(s, i+n, '`', n)
			if end < 0 {
				b.WriteString(s[i : i+n])
				i += n
				continue
			}
			b.WriteString(s[i : end+n])
			i = end + n
		case strings.HasPrefix(s[i:], `\(`):
			end := findCloser(s, i+2, `\)`)
			if end < 0 {
				b.WriteString(s[i : i+2])
				i += 2
				continue
			}
			b.WriteString(mathToMarkdown(s[i+2 : end]))
			i = end + 2
		case c == '\\' && i+1 < len(s):
			// A markdown escape (\$, \\, \*): copy both bytes so the
			// escaped character never opens math.
			b.WriteString(s[i : i+2])
			i += 2
		case c == '$':
			// Six dollars is Codeforces display math; read as three, it would
			// open an empty span and swap math and prose for the rest of the
			// statement.
			n := runLen(s, i, '$')
			if n != 6 {
				n = min(n, 3)
			}
			inner, end, ok := dollarMath(s, i, n)
			if !ok {
				b.WriteString(s[i : i+n])
				i += n
				continue
			}
			b.WriteString(mathToMarkdown(inner))
			i = end
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// dollarMath finds the math opened by n dollars at s[i]. A single $ follows
// pandoc's rule so prices survive: the opener must be followed by a
// non-space, the closer preceded by a non-space and not followed by a digit.
// Math never crosses a blank line.
func dollarMath(s string, i, n int) (inner string, end int, ok bool) {
	open := i + n
	delim := strings.Repeat("$", n)
	if n == 1 && (open >= len(s) || s[open] == ' ' || s[open] == '\n' || s[open] == '\t') {
		return "", 0, false
	}
	for j := open; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++ // skip the escaped character, \$ included
		case s[j] == '\n' && strings.HasPrefix(strings.TrimLeft(s[j+1:], " \t"), "\n"):
			return "", 0, false
		case strings.HasPrefix(s[j:], delim):
			if n == 1 {
				if s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' {
					continue
				}
				if j+1 < len(s) && s[j+1] >= '0' && s[j+1] <= '9' {
					continue
				}
			}
			if j == open {
				return "", 0, false
			}
			return s[open:j], j + n, true
		}
	}
	return "", 0, false
}

// findCloser finds closer from i, giving up at a blank line.
func findCloser(s string, i int, closer string) int {
	for j := i; j < len(s); j++ {
		if strings.HasPrefix(s[j:], closer) {
			return j
		}
		if s[j] == '\\' {
			j++
			continue
		}
		if s[j] == '\n' && strings.HasPrefix(strings.TrimLeft(s[j+1:], " \t"), "\n") {
			return -1
		}
	}
	return -1
}

func runLen(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// findRun finds the next run of exactly n copies of c at or after i.
func findRun(s string, i int, c byte, n int) int {
	for i < len(s) {
		j := strings.IndexByte(s[i:], c)
		if j < 0 {
			return -1
		}
		j += i
		if k := runLen(s, j, c); k == n {
			return j
		} else {
			i = j + k
		}
	}
	return -1
}

// mathToMarkdown converts one math span and escapes what markdown would
// misread.
func mathToMarkdown(tex string) string {
	return escapeMarkdown(mathToUnicode(tex))
}

func escapeMarkdown(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '`', '*', '_', '<', '[', '|':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// mathToUnicode converts the inside of one math span (no delimiters) to
// plain Unicode text; see TeXToUnicode for the rules.
func mathToUnicode(tex string) string {
	p := &texParser{src: tex}
	out := p.seq(0, false, false)
	if p.subFallback {
		// One subscript needed brackets: render every index the same way.
		p = &texParser{src: tex, brackets: true}
		out = p.seq(0, false, false)
	}
	return tidy(out)
}

type texParser struct {
	src      string
	pos      int
	brackets bool // render index subscripts as a[i]

	subFallback bool // an index subscript had no Unicode form

	// frac is the fraction command just rendered, for seq to place among
	// its neighbours (placeFraction).
	frac *fraction
}

type fraction struct{ num, den string }

// tidy collapses the runs of spaces that relation padding leaves behind. No
// space goes between an opening bracket and its contents, nor before closing
// punctuation: \lfloor x \rfloor reads ⌊x⌋.
func tidy(s string) string {
	var b strings.Builder
	space := false
	var last rune
	for _, r := range strings.TrimSpace(s) {
		if r == ' ' {
			space = true
			continue
		}
		if space && !strings.ContainsRune(")]}⌋⌉⟩,;", r) && !strings.ContainsRune("([{⌊⌈⟨", last) {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
		last = r
	}
	return b.String()
}

// seq renders tokens until a closing brace (consumed) or the end. depth > 0
// means inside braces; script means inside a sub/superscript, where
// relations get no padding; text means \text{} mode, where only braces and
// a few escapes are special.
func (p *texParser) seq(depth int, script, text bool) string {
	var b strings.Builder
	prevOp := ""       // the big operator just rendered, so its scripts are limits
	prevEmpty := false // an empty group just rendered: {}^{27}Co's base
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		op, empty := "", false
		switch {
		case c == '}':
			p.pos++
			if depth > 0 {
				return b.String()
			}
		case c == '{':
			p.pos++
			g := p.seq(depth+1, script, text)
			b.WriteString(g)
			empty = g == ""
		case text && c == '\\':
			b.WriteString(p.textEscape())
		case text:
			r, n := utf8.DecodeRuneInString(p.src[p.pos:])
			p.pos += n
			b.WriteRune(r)
		case strings.HasPrefix(p.src[p.pos:], `\_`) && !endsFactor(b.String()):
			// An underscore with nothing to index is a blank to fill in,
			// as in [998, \_, \_]; after a name it is a subscript (command).
			p.pos += 2
			b.WriteByte('_')
		case c == '\\' && p.infix() != "":
			// \over and \choose split the group they sit in, so they take
			// what came before them in it as well as the rest.
			name := p.infix()
			p.pos += 1 + len(name)
			left, right := b.String(), p.seq(depth, script, text)
			if depth > 0 {
				return infix(name, tidy(left), tidy(right))
			}
			// Unbraced, the group is the whole formula, which TeX takes
			// as written and the author rarely meant: (a+b) \times 2
			// \over 12 \approx 16 is not over "12 ≈ 16". The operands stop
			// at a relation, a comma or a bracket around them instead.
			i, j := operandStart(left), operandEnd(right)
			return left[:i] + " " + infix(name, tidy(left[i:]), tidy(right[:j])) + " " + right[j:]
		case c == '\\':
			s, isOp := p.command(script)
			if f := p.frac; f != nil {
				p.frac = nil
				s = p.placeFraction(&b, *f, bigOpSymbols[prevOp])
			}
			b.WriteString(s)
			switch {
			case isOp:
				op = s
			case s == "":
				op = prevOp // \sum\limits_{i}: sizing keeps the operator
			}
		case c == '&':
			p.pos++ // alignment tab in cases/aligned
			b.WriteString("  ")
		case c == '^' || c == '_':
			p.pos++
			arg := p.rawArg()
			if arg == `\limits` || arg == `\nolimits` {
				arg = p.rawArg() // \sum_\limits{i}, the switch misplaced
			}
			s := p.script(c, arg, prevOp != "")
			before := strings.TrimRight(b.String(), " ")
			last, _ := utf8.DecodeLastRuneInString(before)
			rel := relationSymbols[string(last)]
			if len(before) < b.Len() && !prevEmpty && (rel || bigOpSymbols[string(last)] || endsFactor(before)) {
				// A script sits on what it follows, as math ignores spaces
				// (x _ { 1 } is x₁), and a relation's padding goes after
				// its script: a ⊖ₖ b. After "+ " it has no base: {}^{27}Co.
				b.Reset()
				b.WriteString(before)
				if rel {
					s += " "
				}
			}
			b.WriteString(s)
			if prevOp != "" {
				op = prevOp // ∑_{i=1}^{n}: both scripts are limits
			}
		case c == '~':
			p.pos++
			b.WriteByte(' ')
		case c == '\'':
			p.pos++
			if p.pos < len(p.src) && p.src[p.pos] == '\'' {
				p.pos++
				b.WriteString("″")
			} else {
				b.WriteString("′")
			}
		case c == '\n' || c == '\t' || c == '\r':
			p.pos++
			b.WriteByte(' ')
		case c == ' ':
			p.pos++
			if !script {
				b.WriteByte(' ')
			}
			op = prevOp // math ignores spaces: \sum_i \frac1i keeps its limits
		case !script && (c == '=' || c == '<' || c == '>'):
			p.pos++
			b.WriteString(" " + string(c) + " ")
		case !script && c == ',' && depth == 0 && !digitGroups(b.String(), p.src[p.pos+1:]):
			p.pos++
			b.WriteString(", ")
		default:
			r, n := utf8.DecodeRuneInString(p.src[p.pos:])
			p.pos += n
			b.WriteRune(r)
		}
		prevOp, prevEmpty = op, empty
	}
	return b.String()
}

// script renders a sub- or superscript from its source. limits marks the
// scripts of a big operator (∑, max), whose fallback must not force brackets
// on the whole formula.
func (p *texParser) script(kind byte, raw string, limits bool) string {
	if kind == '^' && (raw == `\circ` || raw == `\degree`) {
		return "°" // 90^\circ is an angle, not 90 composed with something
	}
	tight := strings.ReplaceAll(p.sub(raw, 1, true, false), " ", "")
	table := superscripts
	if kind == '_' {
		table = subscripts
	}
	brackets := kind == '_' && p.brackets && !limits
	if u, ok := mapAll(tight, table); ok && !brackets {
		return u
	}
	// The fallback reads as ordinary math, relations spaced: [1 ≤ i ≤ n].
	loose := tidy(p.sub(raw, 1, false, false))
	if loose == "" {
		return " " // 1_{\ \ }1: an empty script spaces a hand-set layout
	}
	if kind == '^' {
		if utf8.RuneCountInString(tight) == 1 {
			return "^" + tight
		}
		return "^(" + loose + ")"
	}
	if !limits {
		p.subFallback = true
	}
	return "[" + loose + "]"
}

// sub renders src with a fresh parser that shares this one's mode, so an
// argument can be rendered twice (tight and loose) from its source.
func (p *texParser) sub(src string, depth int, script, text bool) string {
	q := &texParser{src: src, brackets: p.brackets}
	out := q.seq(depth, script, text)
	p.subFallback = p.subFallback || q.subFallback
	return out
}

// rawArg returns the source of the next argument and moves past it: the
// inside of a braced group, a control sequence, or one character, as TeX
// reads one (10^18 is 10¹8 in TeX too).
func (p *texParser) rawArg() string {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return ""
	}
	start := p.pos
	switch p.src[p.pos] {
	case '{':
		depth := 0
		for ; p.pos < len(p.src); p.pos++ {
			switch p.src[p.pos] {
			case '\\':
				p.pos++
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					p.pos++
					return p.src[start+1 : p.pos-1]
				}
			}
		}
		return p.src[start+1:] // unclosed: take the rest
	case '\\':
		p.pos++
		if p.pos < len(p.src) && !isLetter(p.src[p.pos]) {
			_, n := utf8.DecodeRuneInString(p.src[p.pos:])
			p.pos += n
			return p.src[start:p.pos]
		}
		// A command takes its arguments along, as MathJax reads it:
		// w_\mathrm{max} is wₘₐₓ, not w with an empty index before "max".
		name := p.letters()
		if name == "sqrt" {
			p.optArg()
		}
		for range macroArgs[name] {
			p.rawArg()
		}
		return p.src[start:p.pos]
	}
	_, n := utf8.DecodeRuneInString(p.src[p.pos:])
	p.pos += n
	return p.src[start:p.pos]
}

func mapAll(s string, table map[rune]rune) (string, bool) {
	if s == "" {
		return "", false
	}
	var b strings.Builder
	for _, r := range s {
		u, ok := table[r]
		if !ok {
			return "", false
		}
		b.WriteRune(u)
	}
	return b.String(), true
}

// textEscape handles a backslash inside \text{}.
func (p *texParser) textEscape() string {
	p.pos++ // the backslash
	if p.pos >= len(p.src) {
		return `\`
	}
	c := p.src[p.pos]
	if c == '$' {
		if s, ok := p.textMath(); ok {
			return s
		}
	}
	if strings.IndexByte(`{}_%$#&\ `, c) >= 0 {
		p.pos++
		if c == '\\' {
			return " "
		}
		return string(c)
	}
	name := p.letters()
	if s, ok := symbols[name]; ok {
		return s
	}
	if s, ok := relations[name]; ok {
		return s
	}
	return `\` + name
}

// textMath renders math nested in text, \text{if \$a \le b\$}, at p.pos (the
// first $). The Codeforces converter escapes the dollars of math inside a
// formula's \text, and a price would not read as math: the inside must not
// start or end with a space, nor start with a digit.
func (p *texParser) textMath() (string, bool) {
	rest := p.src[p.pos+1:]
	end := strings.Index(rest, `\$`)
	if end <= 0 {
		return "", false
	}
	inner := rest[:end]
	if strings.ContainsAny(inner[:1]+inner[len(inner)-1:], " \t") || isDigit(inner[0]) {
		return "", false
	}
	p.pos += 1 + end + 2
	return tidy(p.sub(inner, 1, false, false)), true
}

func (p *texParser) skipSpaces() {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
}

func (p *texParser) letters() string {
	start := p.pos
	for p.pos < len(p.src) && isLetter(p.src[p.pos]) {
		p.pos++
	}
	return p.src[start:p.pos]
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// digitGroups reports whether a comma between before and after separates
// the digit groups of one number, as in $10,000$: a digit before it and
// exactly three digits after. Statements write thousands that way although
// TeX would set it as a list; a real list of digits ($1,2,3$) keeps the
// space a list gets.
func digitGroups(before, after string) bool {
	if before == "" || !isDigit(before[len(before)-1]) || len(after) < 3 {
		return false
	}
	for i := range 3 {
		if !isDigit(after[i]) {
			return false
		}
	}
	return len(after) == 3 || !isDigit(after[3])
}

// arg reads and renders one macro argument: a braced group or a single
// token.
func (p *texParser) arg(script, text bool) string {
	return p.sub(p.rawArg(), 1, script, text)
}

// optArg reads an optional [..] argument, as in \sqrt[3]{x}.
func (p *texParser) optArg() string {
	if p.pos < len(p.src) && p.src[p.pos] == '[' {
		end := strings.IndexByte(p.src[p.pos:], ']')
		if end > 0 {
			s := p.src[p.pos+1 : p.pos+end]
			p.pos += end + 1
			return s
		}
	}
	return ""
}

// command renders the control sequence at p.pos (a backslash). isOp is true
// for big operators whose scripts are limits.
func (p *texParser) command(script bool) (out string, isOp bool) {
	p.pos++ // the backslash
	if p.pos >= len(p.src) {
		return `\`, false
	}
	c := p.src[p.pos]
	if !isLetter(c) {
		p.pos++
		switch c {
		case ',', ':', ';', '>', ' ':
			return " ", false
		case '!', '/', '-':
			return "", false // negative space, italic correction, soft hyphen
		case '\\':
			return " ", false // line break in display math
		case '{', '}', '%', '$', '#', '&', '*':
			return string(c), false
		case '|':
			return "‖", false
		case '_':
			// A converter that escapes markdown turns a_i into a\_i; a
			// literal underscore in math is rare enough to read it back as
			// a subscript.
			return p.script('_', p.rawArg(), false), false
		}
		return `\` + string(c), false
	}
	name := p.letters()
	pad := func(s string) string {
		if script {
			return s
		}
		return " " + s + " "
	}
	switch name {
	case "frac", "dfrac", "tfrac", "cfrac":
		f := fraction{tidy(p.arg(script, false)), tidy(p.arg(script, false))}
		p.frac = &f
		return f.String(), false
	case "binom", "dbinom", "tbinom":
		n, k := p.arg(script, false), p.arg(script, false)
		return "C(" + tidy(n) + ", " + tidy(k) + ")", false
	case "sqrt":
		root := tidy(p.sub(p.optArg(), 1, true, false)) // \sqrt[\leftroot{-2}3]{x}
		x := wrapOperand(tidy(p.arg(script, false)))
		switch root {
		case "":
			return "√" + x, false
		case "3":
			return "∛" + x, false
		case "4":
			return "∜" + x, false
		}
		return x + "^(1/" + root + ")", false
	case "text", "textrm", "textit", "textbf", "texttt", "textsf", "mbox",
		"mathrm", "mathit", "mathbf", "mathtt", "mathsf", "operatorname",
		"boldsymbol", "bm", "emph", "textnormal", "cancel",
		"mathbin", "mathrel", "mathop", "smash", "boxed", "fbox", "hbox",
		"pmb", "textup", "textsl", "mathnormal", "ensuremath":
		textMode := strings.HasPrefix(name, "text") || name == "mbox" || name == "emph" ||
			name == "fbox" || name == "hbox"
		return p.arg(script, textMode), false
	case "substack":
		// \sum_{\substack{i < j \\ j < k}}: the lines are conditions, all
		// of which hold.
		return joinWith(p.cells(p.rawArg(), rowBreak, script), ", "), false
	case "href":
		p.rawArg() // the URL; the text is what the page shows
		return p.arg(script, true), false
	case "textcircled":
		return "(" + tidy(p.arg(script, true)) + ")", false
	case "tag":
		return "  (" + tidy(p.arg(script, true)) + ")", false
	case "hspace", "hphantom", "phantom", "hskip":
		p.rawArg()
		return " ", false
	case "vspace", "vphantom", "label", "cline", "leftroot", "uproot":
		p.rawArg()
		return "", false
	case "mkern", "kern", "mskip":
		// A dimension follows unbraced: \mkern-3mu.
		p.pos += len(dimensionRE.FindString(p.src[p.pos:]))
		return " ", false
	case "rule":
		p.optArg()
		p.rawArg()
		p.rawArg()
		return "", false
	case "verb":
		// \verb|x| quotes x verbatim between two copies of any character.
		if p.pos < len(p.src) {
			d := p.src[p.pos]
			if end := strings.IndexByte(p.src[p.pos+1:], d); end >= 0 {
				s := p.src[p.pos+1 : p.pos+1+end]
				p.pos += end + 2
				return s, false
			}
		}
		return "", false
	case "mathbb", "Bbb":
		a := p.arg(script, false)
		if s, ok := blackboard[a]; ok {
			return s, false
		}
		return a, false
	case "mathcal", "mathscr", "mathfrak":
		return p.arg(script, false), false
	case "overline", "bar", "hat", "widehat", "tilde", "widetilde", "vec",
		"dot", "ddot", "overrightarrow", "overleftarrow":
		return combine(p.arg(script, false), accents[name]), false
	case "underline":
		return p.arg(script, false), false
	case "underbrace", "overbrace":
		// The brace has no linear form, but its label says what it spans
		// (k times, x of them), so it indexes the group, over or under:
		// (a, a, …, a)ₓ. A superscript would read as a power.
		x := tidy(p.arg(script, false))
		p.skipSpaces()
		if p.pos < len(p.src) && (p.src[p.pos] == '_' || p.src[p.pos] == '^') {
			p.pos++
			note := p.rawArg()
			if words := tidy(p.sub(note, 1, false, false)); strings.Contains(words, " ") {
				return "(" + x + ")[" + words + "]", false // ₖₜᵢₘₑₛ would not read
			}
			return "(" + x + ")" + p.script('_', note, true), false
		}
		return x, false
	case "overset", "underset", "stackrel":
		// The note set over or under is part of what the symbol says: the
		// step on an arrow, the name of a relation, the range of a max.
		note, base := p.rawArg(), tidy(p.arg(script, false))
		label := tidy(p.sub(note, 1, true, false))
		r, n := utf8.DecodeRuneInString(base)
		arrow := n == len(base) && base != ""
		switch {
		case base == "=" && label == "def":
			return pad("≝"), false
		case arrow && strings.ContainsRune("→⟶⇒↦", r):
			return pad("—" + label + base), false
		case arrow && strings.ContainsRune("←⟵⇐", r):
			return pad(base + label + "—"), false
		}
		s := base + p.script('_', note, true)
		if relationSymbols[base] {
			return pad(s), false
		}
		return s, false
	case "xrightarrow", "xleftarrow":
		p.optArg() // the label below
		label := tidy(p.arg(true, false))
		if name == "xleftarrow" {
			return pad("←" + label + "—"), false
		}
		return pad("—" + label + "→"), false
	case "color", "textcolor", "require":
		// \color{red}x and \textcolor{red}{x} print x; \require{cancel}
		// loads a MathJax extension.
		p.rawArg()
		return "", false
	case "hline", "nonumber", "notag":
		return "", false
	case "pmod":
		return " (mod " + tidy(p.arg(script, false)) + ")", false
	case "bmod", "mod":
		return " mod ", false
	case "left", "right", "big", "Big", "bigg", "Bigg", "bigl", "bigr",
		"Bigl", "Bigr", "biggl", "biggr", "Biggl", "Biggr", "bigm", "Bigm",
		"biggm", "Biggm", "middle", "displaystyle", "scriptscriptstyle",
		"textstyle", "scriptstyle", "limits", "nolimits", "tiny", "small",
		"normalsize", "large", "Large", "LARGE", "huge", "Huge",
		"bf", "it", "rm", "tt", "sf", "cal":
		// Sizing and font switches only; the delimiter or text that follows
		// renders on its own. A \left. is an invisible delimiter.
		if p.pos < len(p.src) && p.src[p.pos] == '.' {
			p.pos++
		}
		return "", false
	case "not":
		p.skipSpaces()
		if strings.HasPrefix(p.src[p.pos:], "=") {
			p.pos++
			return pad("≠"), false
		}
		if strings.HasPrefix(p.src[p.pos:], `\in`) {
			p.pos += 3
			return pad("∉"), false
		}
		return "¬", false
	case "matrix", "pmatrix":
		// Plain TeX's \matrix{a & b \cr c & d}, which MathJax keeps.
		return p.grid(name, p.rawArg(), script), false
	case "begin":
		return " " + p.environment(p.rawArg(), script) + " ", false
	case "end":
		p.rawArg() // an \end whose \begin is not in this span
		return " ", false
	case "newline", "cr":
		return " ", false // line breaks outside an environment
	case "quad":
		return "  ", false
	case "enspace", "space":
		return " ", false
	case "qquad":
		return "    ", false
	}
	if s, ok := relations[name]; ok {
		return pad(s), false
	}
	if s, ok := bigOps[name]; ok {
		return s, true
	}
	if s, ok := symbols[name]; ok {
		return s, false
	}
	if functions[name] {
		// Upright operator words are set off from what precedes them, as
		// TeX does: N\log N reads N log N, not Nlog N. tidy drops the space
		// at the start of a formula or after an opening bracket.
		if script {
			return name, true
		}
		return " " + name, true
	}
	return `\` + name, false
}

// wrapOperand parenthesizes a fraction or root operand that is more than one
// term, so \frac{a+b}{2} reads (a+b)/2 rather than a+b/2.
func wrapOperand(s string) string {
	depth := 0
	for _, r := range s {
		switch r {
		case '(', '[', '{', '⌊', '⌈':
			depth++
		case ')', ']', '}', '⌋', '⌉':
			depth--
		case ' ', '+', '-', '·', '×', '/', '−', '±':
			if depth == 0 {
				return "(" + s + ")"
			}
		}
	}
	return s
}

// wrapDenominator also parenthesizes a product, since n!/k!(n-k)! reads as
// (n!/k!)·(n-k)!. One factor stays bare: a symbol, or a name or number
// followed only by bracketed groups (10⁹, aᵢ, dp[i][j], max(1, |B|)).
func wrapDenominator(s string) string {
	if w := wrapOperand(s); w != s || utf8.RuneCountInString(s) <= 1 {
		return w
	}
	// A root of one factor is one factor too: 1/√2, 1/√(a+b).
	rest := strings.TrimLeftFunc(strings.TrimLeft(s, "√∛∜"), func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)
	})
	depth := 0
	for _, r := range rest {
		open := strings.ContainsRune("([{⌊⌈", r)
		if depth == 0 && !open {
			return "(" + s + ")"
		}
		switch {
		case open:
			depth++
		case strings.ContainsRune(")]}⌋⌉", r):
			depth--
		}
	}
	return s
}

func (f fraction) String() string { return wrapOperand(f.num) + "/" + wrapDenominator(f.den) }

// placeFraction sets the fraction just rendered among its neighbours. A
// linear a/b binds to what touches it, where the stacked one did not:
//
//	\frac{3}{2}r     (3/2)r     not 3/2r, r over 2
//	\frac{n}{2}^2    (n/2)²     not n/2², n over 4
//	x\frac{d}{dx}    x(d/dx)    not xd/dx
//	2\frac{2}{3}     2⅔         a mixed number, not 22/3
//
// b holds the group so far. afterBigOp says it ends in a big operator and
// its limits, which leave room enough: ∑ᵢ 1/i.
func (p *texParser) placeFraction(b *strings.Builder, f fraction, afterBigOp bool) string {
	before := strings.TrimRight(b.String(), " ")
	if wholeNumberEnd(before) && isInteger(f.num) && isInteger(f.den) {
		b.Reset()
		b.WriteString(before)
		return vulgar(f.num, f.den)
	}
	if !afterBigOp && endsFactor(before) || startsFactor(p.src[p.pos:]) {
		return "(" + f.String() + ")"
	}
	return f.String()
}

// wholeNumberEnd reports whether s ends in a whole number: digits, not
// after a decimal point or as part of a name.
func wholeNumberEnd(s string) bool {
	i := len(s)
	for i > 0 && isDigit(s[i-1]) {
		i--
	}
	if i == len(s) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return i == 0 || r != '.' && r != ',' && !unicode.IsLetter(r) && !unicode.IsNumber(r)
}

func isInteger(s string) bool {
	for i := range len(s) {
		if !isDigit(s[i]) {
			return false
		}
	}
	return s != ""
}

// vulgar is the fraction num/den of a mixed number: one character where
// Unicode has it, else superscript over subscript (³⁄₇).
func vulgar(num, den string) string {
	if s, ok := vulgarFractions[num+"/"+den]; ok {
		return s
	}
	n, _ := mapAll(num, superscripts)
	d, _ := mapAll(den, subscripts)
	return n + "⁄" + d
}

var vulgarFractions = map[string]string{
	"1/2": "½", "1/3": "⅓", "2/3": "⅔", "1/4": "¼", "3/4": "¾", "1/5": "⅕",
	"2/5": "⅖", "3/5": "⅗", "4/5": "⅘", "1/6": "⅙", "5/6": "⅚", "1/7": "⅐",
	"1/8": "⅛", "3/8": "⅜", "5/8": "⅝", "7/8": "⅞", "1/9": "⅑", "1/10": "⅒",
}

// endsFactor reports whether rendered math ends in something a fraction
// after it would multiply: a name, a number, a closing bracket, a script.
// A slash counts too, since a/b/c reads (a/b)/c.
func endsFactor(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(s)
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) ||
		strings.ContainsRune(")]⌋⌉′″!/", r)
}

// startsFactor reports whether TeX source s starts with something a
// fraction before it would multiply, or a superscript that would take only
// its denominator.
func startsFactor(s string) bool {
	s = strings.TrimLeft(s, " ")
	for len(s) > 1 && s[0] == '\\' && strings.IndexByte(",;:! ", s[1]) >= 0 {
		s = strings.TrimLeft(s[2:], " ") // spacing: \frac{1}{2}\,n
	}
	if s == "" {
		return false
	}
	if c := s[0]; isLetter(c) || isDigit(c) || strings.IndexByte("([{^!'", c) >= 0 {
		return true
	}
	if s[0] != '\\' {
		return false
	}
	name := s[1:]
	if i := strings.IndexFunc(name, func(r rune) bool { return r > 127 || !isLetter(byte(r)) }); i >= 0 {
		name = name[:i]
	}
	switch name {
	case "frac", "dfrac", "tfrac", "sqrt", "binom", "dbinom", "left", "bigl", "Bigl",
		"biggl", "Biggl", "mathrm", "mathbf", "mathit", "operatorname",
		"lfloor", "lceil", "langle", "lvert", "lVert":
		return true
	}
	if functions[name] || bigOps[name] != "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(symbols[name])
	return unicode.IsLetter(r) // \pi, \alpha
}

// infix names the command at p.pos when it is one of TeX's infix
// fractions, which take the whole group they sit in: {n \choose k}.
func (p *texParser) infix() string {
	i := p.pos + 1
	for i < len(p.src) && isLetter(p.src[i]) {
		i++
	}
	switch name := p.src[p.pos+1 : i]; name {
	case "over", "choose", "atop":
		return name
	}
	return ""
}

func infix(name, left, right string) string {
	switch name {
	case "choose":
		return "C(" + left + ", " + right + ")"
	case "atop":
		// left stacked on right, with no rule: Stirling numbers {n \atop k}.
		return left + "¦" + right
	}
	return fraction{left, right}.String()
}

// environment renders \begin{env}…\end{env} on one line, as a reader's
// paragraph cannot hold rows. Rows and cells join into the shape that
// reads as the same structure:
//
//	pmatrix          (a, b; c, d)
//	cases            {x, if x ≥ 0; -x, otherwise}
//	aligned, split   a = b + c; c = d
//	array            what the cells say, in order
//
// A row that ends in its own punctuation (",", ";", ".") keeps it in place
// of the "; ".
func (p *texParser) environment(env string, script bool) string {
	switch env {
	case "array", "tabular", "subarray", "alignat", "alignat*", "alignedat":
		p.rawArg() // the column spec, or how many columns
	}
	return p.grid(env, p.envBody(env), script)
}

// grid lays out the body of an environment named env.
func (p *texParser) grid(env, body string, script bool) string {
	var rows [][]string
	for _, r := range splitTop(body, rowBreak) {
		if cells := p.cells(r, cellBreak, script); strings.Join(cells, "") != "" {
			rows = append(rows, cells)
		}
	}
	var lines []string
	if d, ok := matrixDelims[env]; ok {
		for _, r := range rows {
			lines = append(lines, strings.Join(r, ", "))
		}
		return d[0] + joinLines(lines) + d[1]
	}
	switch strings.TrimSuffix(env, "*") {
	case "cases", "dcases", "rcases":
		for _, r := range rows {
			lines = append(lines, caseRow(r))
		}
		s := joinLines(lines)
		// What ends the last row ends the sentence: {…, otherwise}.
		if r, n := utf8.DecodeLastRuneInString(s); strings.ContainsRune(".,;", r) {
			return "{" + s[:len(s)-n] + "}" + string(r)
		}
		return "{" + s + "}"
	case "align", "aligned", "alignat", "alignedat", "flalign":
		// & alternates alignment points and columns: a &= b & c &= d.
		for _, r := range rows {
			var eqs []string
			for i := 0; i < len(r); i += 2 {
				if eq := tidy(strings.Join(r[i:min(i+2, len(r))], " ")); eq != "" {
					eqs = append(eqs, eq)
				}
			}
			lines = append(lines, strings.Join(eqs, ", "))
		}
		return joinLines(lines)
	case "eqnarray", "gather", "gathered", "split", "multline", "equation":
		for _, r := range rows {
			lines = append(lines, tidy(strings.Join(r, " ")))
		}
		return joinLines(lines)
	}
	// array, tabular, subarray: a grid whose meaning only the page has.
	for _, r := range rows {
		var cells []string
		for _, c := range r {
			if c != "" {
				cells = append(cells, c)
			}
		}
		lines = append(lines, strings.Join(cells, ", "))
	}
	return joinLines(lines)
}

var matrixDelims = map[string][2]string{
	"matrix": {"", ""}, "smallmatrix": {"", ""}, "pmatrix": {"(", ")"},
	"bmatrix": {"[", "]"}, "Bmatrix": {"{", "}"}, "vmatrix": {"|", "|"},
	"Vmatrix": {"‖", "‖"},
}

// caseRow joins a cases row: the value, then its condition.
func caseRow(cells []string) string {
	if len(cells) < 2 {
		return strings.Join(cells, "")
	}
	v, cond := cells[0], tidy(strings.Join(cells[1:], " "))
	last, _ := utf8.DecodeLastRuneInString(v)
	first, _ := utf8.DecodeRuneInString(cond)
	if v == "" || cond == "" || strings.ContainsRune(",;:.", last) || strings.ContainsRune(",;:.(", first) {
		return tidy(v + " " + cond)
	}
	return v + ", " + cond
}

func joinLines(lines []string) string { return joinWith(lines, "; ") }

// joinWith joins lines with sep, or a space after a line that ends in its
// own punctuation.
func joinWith(lines []string, sep string) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			if r, _ := utf8.DecodeLastRuneInString(lines[i-1]); strings.ContainsRune(",;.", r) {
				b.WriteString(" ")
			} else {
				b.WriteString(sep)
			}
		}
		b.WriteString(l)
	}
	return b.String()
}

// cells renders each part of src cut at sep.
func (p *texParser) cells(src, sep string, script bool) []string {
	var out []string
	for _, c := range splitTop(src, sep) {
		out = append(out, tidy(p.sub(c, 0, script, false))) // a cell is a formula of its own
	}
	return out
}

// The separators splitTop cuts at: an environment's rows and cells.
const (
	rowBreak  = `\\`
	cellBreak = `&`
)

// splitTop cuts s at sep where it is not inside braces or a nested
// environment. A row break's spacing (\\[2pt]) goes with it, and plain
// TeX's \cr breaks a row too.
func splitTop(s, sep string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '{':
			depth++
		case c == '}':
			depth--
		case c == '&' && sep == cellBreak && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		case c == '\\' && i+1 < len(s):
			j := i + 2
			if isLetter(s[i+1]) {
				for j < len(s) && isLetter(s[j]) {
					j++
				}
			}
			switch name := s[i+1 : j]; {
			case name == "begin":
				depth++
			case name == "end":
				depth--
			case depth == 0 && sep == rowBreak && (name == `\` || name == "cr"):
				out = append(out, s[start:i])
				j += len(rowSpacingRE.FindString(s[j:]))
				start = j
			}
			i = j - 1
		}
	}
	return append(out, s[start:])
}

var (
	rowSpacingRE = regexp.MustCompile(`^\s*\[\s*-?[\d.]+\s*(?:pt|em|ex|mm|cm|in|mu|px)\s*\]`)
	dimensionRE  = regexp.MustCompile(`^\s*-?[\d.]+\s*(?:mu|pt|em|ex|mm|cm|in|px)`)
)

// envBody returns the source up to the \end{env} that closes the \begin
// just read, and moves past it.
func (p *texParser) envBody(env string) string {
	begin, end := `\begin{`+env+`}`, `\end{`+env+`}`
	depth := 1
	for i := p.pos; i < len(p.src); {
		switch {
		case strings.HasPrefix(p.src[i:], begin):
			depth++
			i += len(begin)
		case strings.HasPrefix(p.src[i:], end):
			if depth--; depth == 0 {
				body := p.src[p.pos:i]
				p.pos = i + len(end)
				return body
			}
			i += len(end)
		default:
			i++
		}
	}
	body := p.src[p.pos:] // unclosed: take the rest
	p.pos = len(p.src)
	return body
}

// combine puts a combining accent after every character of s.
func combine(s, mark string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(r)
		if !unicode.IsSpace(r) {
			b.WriteString(mark)
		}
	}
	return b.String()
}

var accents = map[string]string{
	"overline": "̅", "bar": "̄", "hat": "̂", "widehat": "̂",
	"tilde": "̃", "widetilde": "̃", "vec": "⃗", "dot": "̇",
	"ddot": "̈", "overrightarrow": "⃗", "overleftarrow": "⃖",
}

// relations and binary operators get a space on each side, as TeX sets them.
var relations = map[string]string{
	"le": "≤", "leq": "≤", "leqslant": "≤", "ge": "≥", "geq": "≥", "geqslant": "≥",
	"ne": "≠", "neq": "≠", "lt": "<", "gt": ">", "approx": "≈", "equiv": "≡",
	"sim": "∼", "simeq": "≃", "cong": "≅", "propto": "∝", "ll": "≪", "gg": "≫",
	"in": "∈", "notin": "∉", "ni": "∋", "subset": "⊂", "subseteq": "⊆",
	"supset": "⊃", "supseteq": "⊇", "to": "→", "rightarrow": "→",
	"leftarrow": "←", "gets": "←", "Rightarrow": "⇒", "Leftarrow": "⇐",
	"implies": "⇒", "iff": "⇔", "Leftrightarrow": "⇔", "leftrightarrow": "↔",
	"mapsto": "↦", "longrightarrow": "⟶", "longleftrightarrow": "⟷", "leadsto": "⇝",
	"uparrow": "↑", "downarrow": "↓", "ngtr": "≯", "nless": "≮",
	"times": "×", "cdot": "·", "div": "÷", "pm": "±", "mp": "∓",
	"oplus": "⊕", "otimes": "⊗", "cup": "∪", "cap": "∩", "setminus": "∖",
	"land": "∧", "wedge": "∧", "lor": "∨", "vee": "∨", "mid": "|",
	"circ": "∘", "ast": "∗", "star": "⋆", "bullet": "•", "bigcirc": "◯",
	"odot": "⊙", "diamond": "⋄",
	"nmid": "∤", "perp": "⊥", "parallel": "∥", "nparallel": "∦", "subsetneq": "⊊",
	"supsetneq": "⊋", "nsubseteq": "⊈", "nsupseteq": "⊉", "nleq": "≰", "ngeq": "≱",
	"lneq": "⪇", "gneq": "⪈", "prec": "≺", "succ": "≻", "preceq": "⪯", "succeq": "⪰",
	"lesssim": "≲", "gtrsim": "≳", "asymp": "≍", "doteq": "≐", "triangleq": "≜",
	"thicksim": "∼", "nsim": "≁", "models": "⊨", "vdash": "⊢", "dashv": "⊣",
	"Longrightarrow": "⟹", "Longleftarrow": "⟸", "Longleftrightarrow": "⟺",
	"longleftarrow": "⟵", "dashrightarrow": "⇢", "dashleftarrow": "⇠",
	"hookrightarrow": "↪", "hookleftarrow": "↩", "updownarrow": "↕",
	"Uparrow": "⇑", "Downarrow": "⇓", "Updownarrow": "⇕", "nearrow": "↗",
	"searrow": "↘", "swarrow": "↙", "nwarrow": "↖", "nrightarrow": "↛",
	"nleftarrow": "↚", "rightleftharpoons": "⇌", "rightrightarrows": "⇉",
	"sqcup": "⊔", "sqcap": "⊓", "uplus": "⊎", "ominus": "⊖", "oslash": "⊘",
	"triangleleft": "◁", "triangleright": "▷", "therefore": "∴", "because": "∵",
}

// bigOps take limits as scripts.
var bigOps = map[string]string{
	"sum": "∑", "prod": "∏", "coprod": "∐", "bigcup": "⋃", "bigcap": "⋂",
	"bigoplus": "⨁", "bigotimes": "⨂", "bigvee": "⋁", "bigwedge": "⋀",
	"int": "∫", "oint": "∮",
}

// functions are upright operator names; their scripts are limits too (max
// over i, lim as n → ∞).
var functions = map[string]bool{
	"log": true, "ln": true, "lg": true, "exp": true, "min": true, "max": true,
	"gcd": true, "lcm": true, "sin": true, "cos": true, "tan": true,
	"lim": true, "sup": true, "inf": true, "deg": true, "det": true,
	"dim": true, "arg": true, "argmin": true, "argmax": true, "Pr": true,
	"arcsin": true, "arccos": true, "arctan": true, "sec": true, "csc": true,
	"cot": true, "sinh": true, "cosh": true, "tanh": true, "coth": true, "ker": true,
	"liminf": true, "limsup": true,
}

var symbols = map[string]string{
	"lfloor": "⌊", "rfloor": "⌋", "lceil": "⌈", "rceil": "⌉",
	"langle": "⟨", "rangle": "⟩", "lvert": "|", "rvert": "|", "vert": "|",
	"lVert": "‖", "rVert": "‖", "Vert": "‖", "lbrace": "{", "rbrace": "}",
	"ldots": "…", "dots": "…", "dotsc": "…", "dotsb": "⋯", "cdots": "⋯",
	"vdots": "⋮", "ddots": "⋱", "infty": "∞", "partial": "∂", "nabla": "∇",
	"forall": "∀", "exists": "∃", "neg": "¬", "lnot": "¬", "emptyset": "∅",
	"varnothing": "∅", "prime": "′", "ell": "ℓ", "angle": "∠", "degree": "°",
	"triangle": "△", "square": "□", "checkmark": "✓", "dagger": "†", "ddagger": "‡",
	"aleph": "ℵ", "hbar": "ℏ", "Re": "ℜ", "Im": "ℑ", "wp": "℘",
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε",
	"varepsilon": "ε", "zeta": "ζ", "eta": "η", "theta": "θ", "vartheta": "ϑ",
	"iota": "ι", "kappa": "κ", "lambda": "λ", "mu": "μ", "nu": "ν", "xi": "ξ",
	"pi": "π", "varpi": "ϖ", "rho": "ρ", "varrho": "ϱ", "sigma": "σ",
	"varsigma": "ς", "tau": "τ", "upsilon": "υ", "phi": "φ", "varphi": "φ",
	"chi": "χ", "psi": "ψ", "omega": "ω",
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ",
	"Pi": "Π", "Sigma": "Σ", "Upsilon": "Υ", "Phi": "Φ", "Psi": "Ψ", "Omega": "Ω",
	"textasciitilde": "~", "textbackslash": `\`, "textunderscore": "_",
	"S": "§", "P": "¶", "ldotp": ".", "cdotp": "·", "colon": ":",
	"blacksquare": "■", "Box": "□", "Diamond": "◇", "lozenge": "◊", "bigstar": "★",
	"blacktriangle": "▲", "triangledown": "▽", "backslash": `\`, "exist": "∃",
	"nexists": "∄", "top": "⊤", "bot": "⊥", "complement": "∁", "dotsi": "⋯",
	"dotsm": "⋯", "dotso": "…", "surd": "√", "imath": "ı", "jmath": "ȷ",
	"lbrack": "[", "rbrack": "]", "dag": "†", "ddag": "‡", "LaTeX": "LaTeX", "TeX": "TeX",
	"clubsuit": "♣", "spadesuit": "♠", "diamondsuit": "♢", "flat": "♭", "sharp": "♯",
}

var blackboard = map[string]string{
	"N": "ℕ", "Z": "ℤ", "Q": "ℚ", "R": "ℝ", "C": "ℂ", "P": "ℙ",
}

var superscripts = runeTable(
	"0123456789+-=()",
	"⁰¹²³⁴⁵⁶⁷⁸⁹⁺⁻⁼⁽⁾",
	"abcdefghijklmnoprstuvwxyz",
	"ᵃᵇᶜᵈᵉᶠᵍʰⁱʲᵏˡᵐⁿᵒᵖʳˢᵗᵘᵛʷˣʸᶻ",
	"ABDEGHIJKLMNOPRTUVW",
	"ᴬᴮᴰᴱᴳᴴᴵᴶᴷᴸᴹᴺᴼᴾᴿᵀᵁⱽᵂ",
	"′*",
	"′*",
)

var subscripts = runeTable(
	"0123456789+-=()",
	"₀₁₂₃₄₅₆₇₈₉₊₋₌₍₎",
	"aehijklmnoprstuvx",
	"ₐₑₕᵢⱼₖₗₘₙₒₚᵣₛₜᵤᵥₓ",
)

// runeTable zips pairs of equal-length strings into a rune map.
func runeTable(pairs ...string) map[rune]rune {
	m := map[rune]rune{}
	for i := 0; i+1 < len(pairs); i += 2 {
		from, to := []rune(pairs[i]), []rune(pairs[i+1])
		if len(from) != len(to) {
			panic("reader: runeTable length mismatch: " + pairs[i])
		}
		for j := range from {
			m[from[j]] = to[j]
		}
	}
	return m
}

// bigOpSymbols are the big operators as rendered, for seq to tell them from
// other operators by their output.
var bigOpSymbols = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range bigOps {
		m[s] = true
	}
	return m
}()

// macroArgs is how many arguments a command takes, for rawArg to read a
// command and its arguments as one script: w_\mathrm{max}, x^\frac12.
var macroArgs = map[string]int{
	"frac": 2, "dfrac": 2, "tfrac": 2, "cfrac": 2, "binom": 2, "dbinom": 2,
	"tbinom": 2, "overset": 2, "underset": 2, "stackrel": 2, "textcolor": 2,
	"href": 2,
	// MathJax 2, which Codeforces runs, reads \color{red}{x} as \textcolor.
	"color": 2,

	"sqrt": 1, "text": 1, "textrm": 1, "textit": 1, "textbf": 1, "texttt": 1,
	"textsf": 1, "textup": 1, "textsl": 1, "textnormal": 1, "mbox": 1, "hbox": 1,
	"fbox": 1, "mathrm": 1, "mathit": 1, "mathbf": 1, "mathtt": 1, "mathsf": 1,
	"mathnormal": 1, "mathbb": 1, "Bbb": 1, "mathcal": 1, "mathscr": 1,
	"mathfrak": 1, "operatorname": 1, "boldsymbol": 1, "bm": 1, "pmb": 1,
	"emph": 1, "cancel": 1, "boxed": 1, "overline": 1, "underline": 1,
	"bar": 1, "hat": 1, "widehat": 1, "tilde": 1, "widetilde": 1, "vec": 1,
	"dot": 1, "ddot": 1, "overrightarrow": 1, "overleftarrow": 1,
	"underbrace": 1, "overbrace": 1, "pmod": 1, "substack": 1,
	"textcircled": 1,
}

// relationRunes are the relations as rendered, where an unbraced \over's
// operands end.
const relationRunes = "=<>≤≥≠≈≡∼≃≅∝≪≫∈∉∋⊂⊆⊃⊇⊊⊋⊈⊉→←⇒⇐⇔↔↦⟶⟵⟷⟹⟸⟺⇝≯≮≰≱⪇⪈≺≻⪯⪰≲≳≍≐≜⊨⊢⊣,;"

// operandStart is where the operand that ends s begins: after the last
// relation or comma outside brackets, or after a bracket left open.
func operandStart(s string) int {
	depth := 0
	for i := len(s); i > 0; {
		r, n := utf8.DecodeLastRuneInString(s[:i])
		i -= n
		switch {
		case strings.ContainsRune(")]}⌋⌉⟩", r):
			depth++
		case strings.ContainsRune("([{⌊⌈⟨", r):
			if depth--; depth < 0 {
				return i + n
			}
		case depth == 0 && strings.ContainsRune(relationRunes, r):
			return i + n
		}
	}
	return 0
}

// operandEnd is where the operand that starts s ends: at the first
// relation or comma outside brackets, or at a bracket it did not open.
func operandEnd(s string) int {
	depth := 0
	for i, r := range s {
		switch {
		case strings.ContainsRune("([{⌊⌈⟨", r):
			depth++
		case strings.ContainsRune(")]}⌋⌉⟩", r):
			if depth--; depth < 0 {
				return i
			}
		case depth == 0 && strings.ContainsRune(relationRunes, r):
			return i
		}
	}
	return len(s)
}

// relationSymbols are relations and binary operators as rendered, which
// TeX spaces on both sides.
var relationSymbols = func() map[string]bool {
	m := map[string]bool{"=": true, "<": true, ">": true}
	for _, s := range relations {
		m[s] = true
	}
	return m
}()
