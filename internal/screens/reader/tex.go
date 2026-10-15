package reader

import (
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
// Math is whatever sits between $…$, $$…$$, $$$…$$$ (Codeforces), \(…\) or
// \[…\]. Text outside math is left alone, and so are fenced code blocks and
// inline code spans, because sample input must reach the judge's reader
// byte for byte.
//
// Subscripts become Unicode subscripts when every character has one (aᵢ,
// x₁₀); otherwise the whole math span switches to brackets (a[N], b[i,j]) so
// one formula never mixes the two styles. Superscripts fall back per script
// (2^Q, 2^(⌊n/2⌋)) since mixing is less jarring there.
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
		case c == '\\' && i+1 < len(s) && (s[i+1] == '(' || s[i+1] == '['):
			closer := `\)`
			if s[i+1] == '[' {
				closer = `\]`
			}
			end := findCloser(s, i+2, closer)
			if end < 0 {
				b.WriteString(s[i : i+2])
				i += 2
				continue
			}
			b.WriteString(mathToMarkdown(s[i+2 : end]))
			i = end + len(closer)
		case c == '\\' && i+1 < len(s):
			// A markdown escape (\$, \\, \*): copy both bytes so the
			// escaped character never opens math.
			b.WriteString(s[i : i+2])
			i += 2
		case c == '$':
			n := min(runLen(s, i, '$'), 3)
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
	return escapeMarkdown(MathToUnicode(tex))
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

// MathToUnicode converts the inside of one math span (no delimiters) to
// plain Unicode text; see TeXToUnicode for the rules.
func MathToUnicode(tex string) string {
	p := &texParser{src: tex}
	out := p.render()
	if p.subFallback {
		// One subscript needed brackets: render every index the same way.
		p = &texParser{src: tex, brackets: true}
		out = p.render()
	}
	return out
}

type texParser struct {
	src      string
	pos      int
	brackets bool // render index subscripts as a[i]

	subFallback bool // an index subscript had no Unicode form
}

func (p *texParser) render() string {
	p.pos = 0
	return tidy(p.seq(0, false, false))
}

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
	prevOp := "" // the big operator just rendered, so its scripts are limits
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		op := ""
		switch {
		case c == '}':
			p.pos++
			if depth > 0 {
				return b.String()
			}
		case c == '{':
			p.pos++
			b.WriteString(p.seq(depth+1, script, text))
		case text && c == '\\':
			b.WriteString(p.textEscape())
		case text:
			r, n := utf8.DecodeRuneInString(p.src[p.pos:])
			p.pos += n
			b.WriteRune(r)
		case c == '\\':
			s, isOp := p.command(script)
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
			arg := p.scriptArg()
			b.WriteString(p.script(c, arg, prevOp != ""))
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
		prevOp = op
	}
	return b.String()
}

// scriptArg reads the source of the argument of ^ or _: a braced group, a
// control sequence, or one character, as TeX does (10^18 is 10¹8 in TeX too).
func (p *texParser) scriptArg() string { return p.rawArg() }

// script renders a sub- or superscript from its source. limits marks the
// scripts of a big operator (∑, max), whose fallback must not force brackets
// on the whole formula.
func (p *texParser) script(kind byte, raw string, limits bool) string {
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
// inside of a braced group, a control sequence, or one character.
func (p *texParser) rawArg() string {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
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
		} else {
			p.letters()
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
	return `\` + name
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
		case ',', ':', ';', ' ':
			return " ", false
		case '!':
			return "", false
		case '\\':
			return " ", false // line break in display math
		case '{', '}', '%', '$', '#', '&', '|':
			if c == '|' {
				return "‖", false
			}
			return string(c), false
		case '_':
			// A converter that escapes markdown turns a_i into a\_i; a
			// literal underscore in math is rare enough to read it back as
			// a subscript.
			return p.script('_', p.scriptArg(), false), false
		case '*':
			return "*", false
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
	case "frac", "dfrac", "tfrac":
		num, den := p.arg(script, false), p.arg(script, false)
		return wrapOperand(tidy(num)) + "/" + wrapOperand(tidy(den)), false
	case "binom", "dbinom", "tbinom":
		n, k := p.arg(script, false), p.arg(script, false)
		return "C(" + tidy(n) + ", " + tidy(k) + ")", false
	case "sqrt":
		root := p.optArg()
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
		"boldsymbol", "bm", "emph", "textnormal":
		textMode := strings.HasPrefix(name, "text") || name == "mbox" || name == "emph"
		return p.arg(script, textMode), false
	case "mathbb":
		a := p.arg(script, false)
		if s, ok := blackboard[a]; ok {
			return s, false
		}
		return a, false
	case "mathcal", "mathscr", "mathfrak":
		return p.arg(script, false), false
	case "overline", "bar", "hat", "widehat", "tilde", "widetilde", "vec",
		"dot", "ddot":
		return combine(p.arg(script, false), accents[name]), false
	case "underline":
		return p.arg(script, false), false
	case "pmod":
		return " (mod " + tidy(p.arg(script, false)) + ")", false
	case "bmod", "mod":
		return " mod ", false
	case "left", "right", "big", "Big", "bigg", "Bigg", "bigl", "bigr",
		"Bigl", "Bigr", "biggl", "biggr", "middle", "displaystyle",
		"textstyle", "scriptstyle", "limits", "nolimits":
		// Sizing only; the delimiter that follows renders on its own. A
		// \left. is an invisible delimiter.
		if p.pos < len(p.src) && p.src[p.pos] == '.' {
			p.pos++
		}
		return "", false
	case "not":
		for p.pos < len(p.src) && p.src[p.pos] == ' ' {
			p.pos++
		}
		if strings.HasPrefix(p.src[p.pos:], "=") {
			p.pos++
			return pad("≠"), false
		}
		if strings.HasPrefix(p.src[p.pos:], `\in`) {
			p.pos += 3
			return pad("∉"), false
		}
		return "¬", false
	case "begin", "end":
		p.arg(script, true) // environment name: cases, aligned, ...
		return " ", false
	case "quad":
		return "  ", false
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
	"ddot": "̈",
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
	"mapsto": "↦", "longrightarrow": "⟶",
	"times": "×", "cdot": "·", "div": "÷", "pm": "±", "mp": "∓",
	"oplus": "⊕", "otimes": "⊗", "cup": "∪", "cap": "∩", "setminus": "∖",
	"land": "∧", "wedge": "∧", "lor": "∨", "vee": "∨", "mid": "|",
	"circ": "∘", "ast": "∗", "star": "⋆", "bullet": "•",
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
}

var symbols = map[string]string{
	"lfloor": "⌊", "rfloor": "⌋", "lceil": "⌈", "rceil": "⌉",
	"langle": "⟨", "rangle": "⟩", "lvert": "|", "rvert": "|", "vert": "|",
	"lVert": "‖", "rVert": "‖", "Vert": "‖", "lbrace": "{", "rbrace": "}",
	"ldots": "…", "dots": "…", "dotsc": "…", "dotsb": "⋯", "cdots": "⋯",
	"vdots": "⋮", "ddots": "⋱", "infty": "∞", "partial": "∂", "nabla": "∇",
	"forall": "∀", "exists": "∃", "neg": "¬", "lnot": "¬", "emptyset": "∅",
	"varnothing": "∅", "prime": "′", "ell": "ℓ", "angle": "∠", "degree": "°",
	"triangle": "△", "square": "□", "checkmark": "✓", "dagger": "†",
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
