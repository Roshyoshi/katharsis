package usaco

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// The statement reader typesets TeX itself (screens/reader runs
// TeXToUnicode over the markdown), so math must leave this package exactly
// as USACO wrote it. html-to-markdown would escape it ("x_i" to "x\_i",
// "\{" to "\\{"), so each math span is swapped for an inert placeholder
// before conversion and swapped back after. Sample data is handled the
// same way: it must reach the markdown byte for byte, as a ```text fence.
type verbatim struct {
	spans []string
}

// placeholder is letters and digits only, which no markdown escaper
// touches; the trailing X keeps "DCVERB1X" from matching inside
// "DCVERB12X".
func (v *verbatim) placeholder(s string) string {
	v.spans = append(v.spans, s)
	return fmt.Sprintf("DCVERB%dX", len(v.spans)-1)
}

func (v *verbatim) restore(md string) string {
	if len(v.spans) == 0 {
		return md
	}
	pairs := make([]string, 0, 2*len(v.spans))
	for i, s := range v.spans {
		pairs = append(pairs, fmt.Sprintf("DCVERB%dX", i), s)
	}
	return strings.NewReplacer(pairs...).Replace(md)
}

// protectMath replaces the math in body's text with placeholders. MathJax
// on usaco.org is configured with $...$ and \(...\) inline, $$...$$ and
// \[...\] display, and processEscapes, so "\$" is a literal dollar; it is
// protected too, so the reader does not mistake it for a delimiter. Text
// in <pre> and <code> is not math.
func (v *verbatim) protectMath(body *html.Node) {
	for _, n := range findAll(body, func(n *html.Node) bool { return n.Type == html.TextNode }) {
		if inside(n, atom.Pre) || inside(n, atom.Code) {
			continue
		}
		n.Data = mapMath(n.Data, v.placeholder)
	}
}

func inside(n *html.Node, a atom.Atom) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.DataAtom == a {
			return true
		}
	}
	return false
}

// mapMath calls f on each math span of s, delimiters included, and on
// each "\$", and splices in the result. An unclosed delimiter is plain
// text, as in MathJax.
func mapMath(s string, f func(string) string) string {
	if !strings.ContainsAny(s, `$\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if n := mathSpan(s[i:]); n > 0 {
			b.WriteString(f(s[i : i+n]))
			i += n
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// mathSpan returns the length of the math span starting at s[0], or 0.
func mathSpan(s string) int {
	pairs := []struct{ open, close string }{
		{`\$`, ""}, // literal dollar, length 2
		{"$$", "$$"},
		{`\(`, `\)`},
		{`\[`, `\]`},
		{"$", "$"},
	}
	for _, p := range pairs {
		if !strings.HasPrefix(s, p.open) {
			continue
		}
		if p.close == "" {
			return len(p.open)
		}
		if j := closing(s[len(p.open):], p.close); j >= 0 {
			return len(p.open) + j + len(p.close)
		}
		return 0
	}
	return 0
}

// closing finds the delimiter that ends a span. A backslash escapes the
// next character, so "$a \$ b$" is one span.
func closing(s, delim string) int {
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], delim) {
			return i
		}
		if s[i] == '\\' {
			i++
		}
	}
	return -1
}
