package curriculum

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// A terminal cannot typeset TeX, and raw `\mathcal{O}(N \log N)` is noise
// next to prose. simplifyTeX rewrites the commands the guide actually uses
// into plain text with Unicode symbols (O(N log N), a ≤ b, ⌊x/2⌋) and keeps
// anything it does not know verbatim, so a formula is never lost, only left
// less pretty.

// texUnwrap commands print their single argument as-is.
var texUnwrap = map[string]bool{
	"texttt": true, "text": true, "textit": true, "textbf": true,
	"textrm": true, "textsf": true, "textnormal": true, "mathrm": true,
	"mathit": true, "mathbf": true, "mathtt": true, "mathsf": true,
	"mathcal": true, "mathscr": true, "mathfrak": true, "mathbb": true,
	"operatorname": true, "boldsymbol": true, "bm": true, "emph": true,
	"underline": true, "overline": true, "mbox": true, "hbox": true,
	"textup": true, "mathnormal": true,
}

// texDrop commands are sizing/spacing hints with no content.
var texDrop = map[string]bool{
	"left": true, "right": true, "big": true, "Big": true, "bigg": true,
	"Bigg": true, "bigl": true, "bigr": true, "Bigl": true, "Bigr": true,
	"biggl": true, "biggr": true, "displaystyle": true, "textstyle": true,
	"limits": true, "nolimits": true, "middle": true,
}

var texSymbols = map[string]string{
	"le": "≤", "leq": "≤", "ge": "≥", "geq": "≥", "ne": "≠", "neq": "≠",
	"lt": "<", "gt": ">", "leqslant": "≤", "geqslant": "≥",
	"cdot": "·", "cdotp": "·", "times": "×", "div": "÷", "pm": "±", "mp": "∓",
	"cdots": "…", "ldots": "…", "dots": "…", "dotsc": "…", "dotsb": "…",
	"vdots": "⋮", "ddots": "⋱",
	"to": "→", "rightarrow": "→", "leftarrow": "←", "gets": "←",
	"Rightarrow": "⇒", "Leftarrow": "⇐", "implies": "⇒", "impliedby": "⇐",
	"iff": "⇔", "Leftrightarrow": "⇔", "leftrightarrow": "↔", "mapsto": "↦",
	"longrightarrow": "→", "Longrightarrow": "⇒", "uparrow": "↑", "downarrow": "↓",
	"infty": "∞", "sum": "Σ", "prod": "Π", "int": "∫",
	"in": "∈", "notin": "∉", "ni": "∋", "subset": "⊂", "subseteq": "⊆",
	"supset": "⊃", "supseteq": "⊇", "subsetneq": "⊊", "cup": "∪", "cap": "∩",
	"bigcup": "⋃", "bigcap": "⋂", "emptyset": "∅", "varnothing": "∅",
	"setminus": "∖", "forall": "∀", "exists": "∃", "neg": "¬", "lnot": "¬",
	"land": "∧", "wedge": "∧", "lor": "∨", "vee": "∨", "oplus": "⊕",
	"bigoplus": "⊕", "otimes": "⊗", "approx": "≈", "equiv": "≡", "sim": "~",
	"simeq": "≃", "cong": "≅", "propto": "∝", "partial": "∂", "nabla": "∇",
	"lfloor": "⌊", "rfloor": "⌋", "lceil": "⌈", "rceil": "⌉",
	"langle": "⟨", "rangle": "⟩", "lvert": "|", "rvert": "|", "vert": "|",
	"mid": "|", "lVert": "‖", "rVert": "‖", "Vert": "‖", "ll": "≪", "gg": "≫",
	"prime": "′", "circ": "∘", "star": "⋆", "ast": "*", "colon": ":",
	"triangle": "△", "angle": "∠", "perp": "⊥", "parallel": "∥",
	"degree": "°", "checkmark": "✓", "quad": "  ", "qquad": "    ",
	"ominus": "⊖", "ell": "ℓ", "rarr": "→", "larr": "←", "bot": "⊥", "top": "⊤",
	"bigstar": "★", "blacksquare": "∎", "square": "□", "LaTeX": "LaTeX", "TeX": "TeX",
	"textemdash": "—", "textendash": "–", "dag": "†",
	"lbrace": "{", "rbrace": "}", "lbrack": "[", "rbrack": "]",
	"bmod": " mod ", "mod": " mod ", "choose": " choose ",
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε",
	"varepsilon": "ε", "zeta": "ζ", "eta": "η", "theta": "θ", "vartheta": "θ",
	"iota": "ι", "kappa": "κ", "lambda": "λ", "mu": "μ", "nu": "ν", "xi": "ξ",
	"pi": "π", "rho": "ρ", "sigma": "σ", "tau": "τ", "upsilon": "υ",
	"phi": "φ", "varphi": "φ", "chi": "χ", "psi": "ψ", "omega": "ω",
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ",
	"Pi": "Π", "Sigma": "Σ", "Phi": "Φ", "Psi": "Ψ", "Omega": "Ω",
}

// texWords are operator names printed as words.
var texWords = map[string]bool{
	"log": true, "ln": true, "lg": true, "exp": true, "sin": true,
	"cos": true, "tan": true, "max": true, "min": true, "gcd": true,
	"lcm": true, "det": true, "lim": true, "sup": true, "inf": true,
	"arg": true, "argmax": true, "argmin": true, "deg": true, "ker": true,
	"dim": true, "Pr": true, "limsup": true, "liminf": true, "tg": true,
	"arctg": true, "arctan": true, "arcsin": true, "arccos": true, "sec": true,
	"csc": true, "cot": true, "sinh": true, "cosh": true, "tanh": true,
}

type texParser struct {
	s    string
	i    int
	envs []string // open \begin{...} environments, innermost last
}

// texEnvs are environments written inline as delimited rows: a 2x2
// bmatrix becomes [a b; c d] and cases becomes {a, x>0; b, otherwise}. Other
// environments (align*, gathered, ...) keep one display line per row.
var texEnvs = map[string][2]string{
	"matrix": {"", ""}, "smallmatrix": {"", ""}, "bmatrix": {"[", "]"},
	"pmatrix": {"(", ")"}, "vmatrix": {"|", "|"}, "Vmatrix": {"‖", "‖"},
	"Bmatrix": {"{", "}"}, "cases": {"{", "}"}, "array": {"", ""},
}

func (p *texParser) env() string {
	if len(p.envs) == 0 {
		return ""
	}
	return p.envs[len(p.envs)-1]
}

func simplifyTeX(s string) string {
	p := &texParser{s: s}
	return p.seq(false)
}

func (p *texParser) seq(inBrace bool) string {
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch c {
		case '}':
			p.i++
			if inBrace {
				return b.String()
			}
		case '{':
			p.i++
			b.WriteString(p.seq(true))
		case '\\':
			p.spaced(&b)
		case '^', '_':
			p.i++
			b.WriteByte(c)
			b.WriteString(wrapAtom(p.arg()))
		case '~':
			p.i++
			b.WriteByte(' ')
		case '$': // math shift, as in \text{if $x$ is odd}; prints nothing
			p.i++
		case '&': // column separator
			p.i++
			if p.env() == "cases" {
				b.WriteString(", ")
			} else {
				b.WriteByte(' ')
			}
		case '\n', '\t': // source layout; TeX lines break only at \\
			p.i++
			b.WriteByte(' ')
		default:
			_, n := utf8.DecodeRuneInString(p.s[p.i:])
			b.WriteString(p.s[p.i : p.i+n])
			p.i += n
		}
	}
	return b.String()
}

// texRelations get spaces around them: TeX spaces "a\le b" itself, plain
// text has to ("a ≤ b", not "a≤ b").
var texRelations = map[string]bool{
	"≤": true, "≥": true, "≠": true, "<": true, ">": true, "≈": true,
	"≡": true, "≅": true, "≃": true, "∝": true, "~": true,
	"→": true, "←": true, "⇒": true, "⇐": true, "⇔": true, "↔": true, "↦": true,
	"∈": true, "∉": true, "∋": true, "⊂": true, "⊆": true, "⊃": true,
	"⊇": true, "⊊": true, "∪": true, "∩": true, "∖": true, "×": true,
	"·": true, "÷": true, "±": true, "∓": true, "⊕": true, "⊗": true,
	"∧": true, "∨": true, "≪": true, "≫": true,
}

// spaced writes the next command's output, adding the spaces TeX would
// have typeset: around relations and binary operators, and between an
// operator name and an adjacent letter ("N\log N" -> "N log N").
func (p *texParser) spaced(b *strings.Builder) {
	start := p.i
	out := p.command()
	isWord := texWords[strings.TrimPrefix(p.s[start:p.i], "\\")]
	if !isWord && !texRelations[out] {
		b.WriteString(out)
		return
	}
	prev, _ := utf8.DecodeLastRuneInString(b.String())
	var next rune
	if p.i < len(p.s) {
		next, _ = utf8.DecodeRuneInString(p.s[p.i:])
	}
	alnum := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	if isWord {
		if alnum(prev) || prev == '}' || prev == ')' {
			b.WriteByte(' ')
		}
		b.WriteString(out)
		if alnum(next) || next == '\\' {
			b.WriteByte(' ')
		}
		return
	}
	if b.Len() > 0 && !strings.ContainsRune(" ([{", prev) {
		b.WriteByte(' ')
	}
	b.WriteString(out)
	if p.i < len(p.s) && !strings.ContainsRune(" )]},.;:", next) {
		b.WriteByte(' ')
	}
}

// arg reads one macro argument: a braced group or a single token.
func (p *texParser) arg() string {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\n' || p.s[p.i] == '\t') {
		p.i++
	}
	if p.i >= len(p.s) {
		return ""
	}
	switch p.s[p.i] {
	case '{':
		p.i++
		return p.seq(true)
	case '\\':
		return p.command()
	}
	_, n := utf8.DecodeRuneInString(p.s[p.i:])
	r := p.s[p.i : p.i+n]
	p.i += n
	return r
}

func (p *texParser) command() string {
	p.i++ // backslash
	if p.i >= len(p.s) {
		return "\\"
	}
	if c := p.s[p.i]; !isLetter(c) {
		p.i++
		switch c {
		case ',', ':', ';', ' ', '>':
			return " "
		case '!':
			return ""
		case '\\': // row break
			if _, inline := texEnvs[p.env()]; inline {
				return "; "
			}
			return "\n"
		case '{', '}', '%', '_', '#', '&', '$':
			return string(c)
		case '|':
			return "‖"
		}
		// The byte, not string(c), which would mangle a UTF-8 lead byte.
		return p.s[p.i-2 : p.i]
	}
	start := p.i
	for p.i < len(p.s) && isLetter(p.s[p.i]) {
		p.i++
	}
	name := p.s[start:p.i]
	switch {
	case texUnwrap[name]:
		return p.arg()
	case texDrop[name]:
		if p.i < len(p.s) && p.s[p.i] == '.' {
			p.i++ // \left. is an invisible delimiter
		}
		return ""
	case name == "frac" || name == "dfrac" || name == "tfrac":
		a, b := p.arg(), p.arg()
		return wrapAtom(a) + "/" + wrapAtom(b)
	case name == "binom" || name == "dbinom" || name == "tbinom":
		a, b := p.arg(), p.arg()
		return "C(" + a + ", " + b + ")"
	case name == "sqrt":
		index := ""
		if p.i < len(p.s) && p.s[p.i] == '[' {
			if j := strings.IndexByte(p.s[p.i:], ']'); j >= 0 {
				index = strings.TrimSpace(simplifyTeX(p.s[p.i+1 : p.i+j]))
				p.i += j + 1
			}
		}
		x := wrapAtom(p.arg())
		switch index {
		case "", "2":
			return "√" + x
		case "3":
			return "∛" + x
		case "4":
			return "∜" + x
		}
		return x + "^(1/" + wrapAtom(index) + ")"
	case name == "tag": // equation number
		return " (" + p.arg() + ")"
	case name == "pmod":
		return " (mod " + p.arg() + ")"
	case name == "begin":
		env := p.arg()
		if env == "array" || env == "tabular" {
			p.arg() // column spec
		}
		p.envs = append(p.envs, env)
		return texEnvs[env][0]
	case name == "end":
		env := p.arg()
		if p.env() == env {
			p.envs = p.envs[:len(p.envs)-1]
		}
		return texEnvs[env][1]
	case texWords[name]:
		return name
	}
	if sym, ok := texSymbols[name]; ok {
		return sym
	}
	return "\\" + name
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// wrapAtom parenthesizes anything but a single symbol or a plain word, so
// 2^{n-1} becomes 2^(n-1) and \frac{a+b}{2} becomes (a+b)/2.
func wrapAtom(a string) string {
	a = strings.TrimSpace(a)
	if utf8.RuneCountInString(a) <= 1 {
		return a
	}
	for _, r := range a {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return "(" + a + ")"
		}
	}
	return a
}

// inlineTeX renders $...$ as escaped plain text on one line.
func inlineTeX(tex string) string {
	return escapeMarkdown(tidyTeX(simplifyTeX(tex)))
}

var texTidier = strings.NewReplacer(
	" ,", ",", " ;", ";", " )", ")", " ]", "]", " }", "}", " ⌋", "⌋", " ⌉", "⌉",
	"( ", "(", "[ ", "[", "{ ", "{", "⌊ ", "⌊", "⌈ ", "⌈",
)

// tidyTeX collapses the whitespace simplifyTeX leaves behind and drops
// spaces inside brackets and before punctuation, as TeX would set them.
func tidyTeX(line string) string {
	return texTidier.Replace(strings.Join(strings.Fields(line), " "))
}

var mdEscaper = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", `*`, `\*`, `_`, `\_`, `[`, `\[`, `]`, `\]`,
	`<`, `\<`, `>`, `\>`, `|`, `\|`, `~`, `\~`, `$`, `\$`,
)

// escapeMarkdown makes s render literally as inline markdown text.
func escapeMarkdown(s string) string { return mdEscaper.Replace(s) }
