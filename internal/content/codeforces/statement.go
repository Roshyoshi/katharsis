package codeforces

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The dataset's statements are plain text taken from the Codeforces page:
// paragraphs split by newlines, "- " and "1. " lists, and TeX in
// Codeforces' own delimiters, $$$x$$$ inline and $$$$$$x$$$$$$ display.
// Formulas that were images on the page were OCR'd into $$x$$ (used
// inline) and, rarely, $x$. The markdown here keeps every formula's TeX
// as written and only rewrites delimiters to the $x$ and $$x$$ the rest
// of the game uses; screens/reader turns the TeX into Unicode. Prose is
// escaped so markdown does not read "**.*.." (a game level in 1185 E) as
// emphasis or "#define" as a heading.

// data normalizes sample and test bytes: Codeforces serves tests with
// CRLF, which getline on macOS would hand to the player's program, and the
// dataset strips the final newline from samples. Statement fences and the
// files on disk both come from this, so they stay byte-identical.
func data(s string) string { return string(dataBytes([]byte(s))) }

// dataBytes is data for a test value, which can be tens of MB: it copies
// only when it must. v may be shared, so it is never written in place.
func dataBytes(v []byte) []byte {
	if bytes.Contains(v, []byte("\r\n")) {
		v = bytes.ReplaceAll(v, []byte("\r\n"), []byte("\n"))
	}
	if len(v) > 0 && v[len(v)-1] != '\n' {
		v = append(v[:len(v):len(v)], '\n')
	}
	return v
}

// statementMarkdown renders statement.md.
func statementMarkdown(e entry, t text) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", escapeText(e.Title))
	fmt.Fprintf(&b, "*%s, problem %s*\n\n", escapeText(e.Name), e.Index)
	fmt.Fprintf(&b, "*Time limit: %s s · Memory limit: %d MB*\n\n",
		strconv.FormatFloat(e.TimeLimit.Seconds(), 'f', -1, 64), e.MemLimit>>20)
	section(&b, "", t.Description)
	section(&b, "Input", t.Input)
	section(&b, "Output", t.Output)
	section(&b, "Interaction", t.Interaction)
	for i, ex := range t.Examples {
		n := i + 1
		fmt.Fprintf(&b, "#### Sample input %d\n\n%s\n", n, fence(data(ex.Input)))
		fmt.Fprintf(&b, "#### Sample output %d\n\n%s\n", n, fence(data(ex.Output)))
	}
	section(&b, "Note", t.Note)
	fmt.Fprintf(&b, "---\n\nSource: <%s> · via open-r1/codeforces, CC BY 4.0\n", e.URL)
	return b.String()
}

// editorialMarkdown renders editorial.md.
func editorialMarkdown(e entry, t text) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Editorial: %s\n\n", escapeText(e.Title))
	fmt.Fprintf(&b, "*%s, problem %s*\n\n", escapeText(e.Name), e.Index)
	section(&b, "", t.Editorial)
	fmt.Fprintf(&b, "---\n\nProblem: <%s> · editorial via open-r1/codeforces, CC BY 4.0\n", e.URL)
	return b.String()
}

func section(b *strings.Builder, heading, body string) {
	md := prose(body)
	if md == "" {
		return
	}
	if heading != "" {
		fmt.Fprintf(b, "#### %s\n\n", heading)
	}
	b.WriteString(md)
	b.WriteString("\n\n")
}

// fence wraps sample data in a ```text block, lengthening the fence if the
// data itself has a line of backticks.
func fence(d string) string {
	n := 3
	for line := range strings.Lines(d) {
		if k := len(line) - len(strings.TrimLeft(line, "`")); k >= n {
			n = k + 1
		}
	}
	f := strings.Repeat("`", n)
	return f + "text\n" + d + f + "\n"
}

// ---- prose -------------------------------------------------------------------

type segKind int

const (
	segText segKind = iota
	segInline
	segDisplay
)

type seg struct {
	kind segKind
	s    string // text as written, or the TeX inside the delimiters
	// digitAfter is true when the source has a digit right after the
	// closing delimiter: "$x$5" would not close for pandoc-style readers.
	digitAfter bool
}

// prose converts one statement field to markdown.
func prose(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	// Codeforces uses no-break spaces as ordinary ones; glamour would
	// never wrap a paragraph of them.
	s = strings.ReplaceAll(s, "\u00a0", " ")
	segs := mergeInline(splitMath(s))
	var b strings.Builder
	for _, sg := range segs {
		switch sg.kind {
		case segText:
			writeText(&b, sg.s)
		case segInline:
			tex := literalDollars(strings.Join(strings.Fields(sg.s), " "))
			if tex == "" {
				continue
			}
			if sg.digitAfter {
				b.WriteString(`\(` + tex + `\)`)
			} else {
				b.WriteString("$" + tex + "$")
			}
		case segDisplay:
			tex := literalDollars(strings.TrimSpace(blankLinesRE.ReplaceAllString(sg.s, "\n")))
			if tex == "" {
				continue
			}
			b.WriteString("\n\n$$" + tex + "$$\n\n")
		}
	}
	md := trailingSpaceRE.ReplaceAllString(b.String(), "\n")
	return strings.TrimSpace(manyNewlinesRE.ReplaceAllString(md, "\n\n"))
}

var (
	blankLinesRE    = regexp.MustCompile(`\n[ \t]*\n[\s]*`)
	manyNewlinesRE  = regexp.MustCompile(`\n[ \t]*\n(?:[ \t]*\n)+`)
	trailingSpaceRE = regexp.MustCompile(`[ \t]+\n`)
)

// splitMath cuts s into text and math the way Codeforces' MathJax reads
// it: outside math, $$$$$$ opens display math and $$$ inline math, and
// the first matching run closes it, so "$$$a$$$$$$^{b}$$$" is two inline
// spans. $$x$$ is the OCR's formula, display when alone on its line. A
// lone $ is math under pandoc's rule (no space inside the delimiters, no
// digit after the closer), otherwise a literal dollar.
func splitMath(s string) []seg {
	var out []seg
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			out = append(out, seg{kind: segText, s: text.String()})
			text.Reset()
		}
	}
	emit := func(k segKind, tex string, after int) {
		flush()
		out = append(out, seg{kind: k, s: tex, digitAfter: after < len(s) && s[after] >= '0' && s[after] <= '9'})
	}
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '$' {
			text.WriteString(`\$`) // already a literal dollar
			i += 2
			continue
		}
		if s[i] != '$' {
			text.WriteByte(s[i])
			i++
			continue
		}
		run := runLen(s, i, '$')
		if run >= 6 {
			if j := closeAt(s, i+6, "$$$$$$"); j >= 0 {
				emit(segDisplay, s[i+6:j], j+6)
				i = j + 6
				continue
			}
		}
		switch {
		case run >= 3:
			if j := closeAt(s, i+3, "$$$"); j >= 0 {
				emit(segInline, s[i+3:j], j+3)
				i = j + 3
				continue
			}
			text.WriteString(`\$\$\$`)
			i += 3
		case run == 2:
			if j := closeAt(s, i+2, "$$"); j >= 0 {
				k := segInline
				if lineStart(s, i) && lineEnd(s, j+2) {
					k = segDisplay
				}
				emit(k, s[i+2:j], j+2)
				i = j + 2
				continue
			}
			text.WriteString(`\$\$`)
			i += 2
		default:
			if j := singleClose(s, i); j >= 0 {
				emit(segInline, s[i+1:j], j+1)
				i = j + 1
				continue
			}
			text.WriteString(`\$`)
			i++
		}
	}
	flush()
	return out
}

// mergeInline joins inline spans with nothing between them. After the
// delimiter rewrite "$a$$^{b}$" would read as a $$ display opener;
// "$a^{b}$" is the same formula.
func mergeInline(segs []seg) []seg {
	var out []seg
	for _, sg := range segs {
		if n := len(out); n > 0 && sg.kind == segInline && out[n-1].kind == segInline {
			prev := &out[n-1]
			a, b := strings.TrimSpace(prev.s), strings.TrimSpace(sg.s)
			sep := " "
			if strings.HasPrefix(b, "^") || strings.HasPrefix(b, "_") || a == "" || b == "" {
				sep = ""
			}
			prev.s, prev.digitAfter = a+sep+b, sg.digitAfter
			continue
		}
		out = append(out, sg)
	}
	return out
}

func runLen(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// closeAt finds delim at or after i, skipping backslash escapes (so "\$"
// inside math is not a closer) and giving up at a blank line, which TeX
// math never crosses.
func closeAt(s string, i int, delim string) int {
	for j := i; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++
		case s[j] == '\n' && strings.HasPrefix(strings.TrimLeft(s[j+1:], " \t"), "\n"):
			return -1
		case strings.HasPrefix(s[j:], delim):
			return j
		}
	}
	return -1
}

// singleClose applies pandoc's rule to a $ at s[i].
func singleClose(s string, i int) int {
	if i+1 >= len(s) || strings.ContainsRune(" \t\n$", rune(s[i+1])) {
		return -1
	}
	for j := i + 1; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++
		case s[j] == '\n' && strings.HasPrefix(strings.TrimLeft(s[j+1:], " \t"), "\n"):
			return -1
		case s[j] == '$':
			if j+1 < len(s) && s[j+1] == '$' {
				return -1 // the start of a longer run: not this span's end
			}
			if strings.ContainsRune(" \t\n", rune(s[j-1])) || j+1 < len(s) && s[j+1] >= '0' && s[j+1] <= '9' {
				continue
			}
			return j
		}
	}
	return -1
}

func lineStart(s string, i int) bool {
	return strings.TrimLeft(s[strings.LastIndexByte(s[:i], '\n')+1:i], " \t") == ""
}

func lineEnd(s string, i int) bool {
	rest := s[i:]
	if k := strings.IndexByte(rest, '\n'); k >= 0 {
		rest = rest[:k]
	}
	return strings.TrimSpace(rest) == ""
}

// htmlTagRE matches the few tags left in editorials (18 at rev): spoiler
// blocks with paragraphs of code. Block tags become line breaks.
var htmlTagRE = regexp.MustCompile(`(?i)</?(p|div|br|spoiler|pre|li|ul|ol|b|i|em|strong|code|span|sup|sub|a)\b[^<>]*>`)

// writeText appends a text segment: tags dropped, each source line a
// paragraph (the dataset breaks lines only between paragraphs and list
// items), leading indentation removed so nothing becomes a code block,
// and markdown syntax escaped.
func writeText(b *strings.Builder, s string) {
	s = htmlTagRE.ReplaceAllStringFunc(s, func(tag string) string {
		switch strings.ToLower(htmlTagRE.FindStringSubmatch(tag)[1]) {
		case "p", "div", "br", "spoiler", "pre", "li", "ul", "ol":
			return "\n"
		}
		return ""
	})
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		atStart := i > 0 || endsLine(b)
		if i > 0 {
			b.WriteString("\n\n")
		}
		if atStart {
			line = strings.TrimLeft(line, " \t")
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">") {
				b.WriteByte('\\')
			}
		}
		if i < len(lines)-1 {
			line = strings.TrimRight(line, " \t")
		}
		b.WriteString(escapeText(line))
	}
}

func endsLine(b *strings.Builder) bool {
	s := b.String()
	return s == "" || strings.HasSuffix(s, "\n")
}

// escapeText backslash-escapes what markdown would read as syntax inside
// a line. "[" and "(" stay: "\[" and "\(" open math for the reader, and a
// bracket only makes a link when "](" touch.
func escapeText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			if i+1 < len(s) && s[i+1] == '$' {
				b.WriteString(`\$`) // from splitMath: keep the escape
				i++
				continue
			}
			b.WriteByte('\\')
		case '*', '_', '`', '<', '~':
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

// literalDollars escapes a bare $ inside math ("$$$$100$$$" is a dollar
// sign then the formula 100), which would otherwise end the span early.
func literalDollars(tex string) string {
	if !strings.Contains(tex, "$") {
		return tex
	}
	var b strings.Builder
	for i := 0; i < len(tex); i++ {
		switch {
		case tex[i] == '\\' && i+1 < len(tex):
			b.WriteString(tex[i : i+2])
			i++
		case tex[i] == '$':
			b.WriteString(`\$`)
		default:
			b.WriteByte(tex[i])
		}
	}
	return b.String()
}
