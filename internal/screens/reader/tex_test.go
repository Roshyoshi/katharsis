package reader

import "testing"

func TestTeXToUnicode(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		// Constraints lines as USACO and Codeforces write them.
		{"bounds", `$1 \le N \le 10^5$`, `1 ≤ N ≤ 10⁵`},
		{"bounds tight", `$1\leq N\leq 2\cdot 10^5$`, `1 ≤ N ≤ 2 · 10⁵`},
		{"times", `($1 \le K \le 2 \times 10^5$)`, `(1 ≤ K ≤ 2 × 10⁵)`},
		{"braced exponent", `$0 \le a_i \le 10^{18}$`, `0 ≤ aᵢ ≤ 10¹⁸`},
		{"modulus", `modulo $10^9+7$.`, `modulo 10⁹+7.`},
		{"int max", `$2^{31}-1$`, `2³¹-1`},
		{"neq ge", `$a \neq b$, $x \geq 0$`, `a ≠ b, x ≥ 0`},
		{"floor", `Output $\lfloor S / 2 \rfloor$.`, `Output ⌊S / 2⌋.`},
		{"ceil frac", `$\left\lceil \frac{n}{2} \right\rceil$`, `⌈n/2⌉`},
		{"frac parens", `$\frac{n(n+1)}{2}$ and $\frac{a+b}{c}$`, `n(n+1)/2 and (a+b)/c`},
		// n!/k!(n-k)! reads as (n!/k!)·(n-k)!.
		{"frac product denominator", `$\frac{n!}{k!(n-k)!}$`, `n!/(k!(n-k)!)`},
		{"frac factor denominators", `$\frac{x}{10^9}$, $\frac{1}{a_i}$, $\frac{1}{dp[i][j]}$, $\frac{d}{\max(1, b)}$`,
			`x/10⁹, 1/aᵢ, 1/dp\[i]\[j], d/max(1, b)`},
		{"sqrt", `$\sqrt{N}$, $\sqrt{a+b}$`, `√N, √(a+b)`},

		// Subscripts: Unicode when every character has one.
		{"sub letter", `$h_i$`, `hᵢ`},
		{"sub digits", `$x_{10}$`, `x₁₀`},
		{"sub expr", `$a_{i+1} - a_i$`, `aᵢ₊₁ - aᵢ`},
		// ... otherwise the whole span uses brackets, never a mix.
		{"sub upper forces brackets", `$a_1, a_2, \ldots, a_N$`, `a\[1], a\[2], …, a\[N]`},
		{"sub comma", `$g_{i,j}$`, `g\[i,j]`},
		{"sub per span", `$a_i$ and $b_N$`, `aᵢ and b\[N]`},
		{"escaped underscore", `$a\_i \le b\_i$`, `aᵢ ≤ bᵢ`},

		// Big operators take limits without forcing brackets elsewhere.
		{"sum", `$\sum_{i=1}^{n} a_i$`, `∑ᵢ₌₁ⁿ aᵢ`},
		{"max limits", `$\max_{1 \le i \le n} c_i$`, `max\[1 ≤ i ≤ n] cᵢ`},
		{"log base", `$O(N \log_2 N)$`, `O(N log₂ N)`},
		{"gcd", `$\gcd(a,b)=1$`, `gcd(a, b) = 1`},
		{"log after letter", `$O(N\log N)$ and $O(\log n)$`, `O(N log N) and O(log n)`},
		{"max after equals", `$m=\max(a,b)$`, `m = max(a, b)`},

		// Superscripts fall back per script.
		{"sup upper", `$2^N$ subsets`, `2ᴺ subsets`},
		{"sup fallback", `$x^{\lfloor n/2 \rfloor}$`, `x^(⌊n/2⌋)`},
		{"sup prime", `$f'(x)$`, `f′(x)`},

		// Text, sets, sizing, misc.
		{"text", `$\text{dist}(u, v) \le k$`, `dist(u, v) ≤ k`},
		{"texttt underscore", `$\texttt{max\_flow}$`, `max\_flow`},
		{"set", `$S \subseteq \{1, 2, \ldots, n\}$`, `S ⊆ {1, 2, …, n}`},
		{"mid", `$\{x \mid x > 0\}$`, `{x \| x > 0}`},
		{"thin space", `$10\,000$`, `10 000`},
		{"braced comma", `$100{,}000$`, `100,000`},
		// A comma between digit groups is a thousands separator, not a
		// list comma.
		{"thousands", `$10,000$`, `10,000`},
		{"thousands bound", `$1 \le N \le 200,000$`, `1 ≤ N ≤ 200,000`},
		{"millions", `$1,000,000$`, `1,000,000`},
		{"thousands then decimal", `$12,345.5$`, `12,345.5`},
		{"digit list", `$1,2,3$`, `1, 2, 3`},
		{"digit list ellipsis", `$1,2,\ldots,n$`, `1, 2, …, n`},
		{"pair", `$(0,1)$`, `(0, 1)`},
		{"four digits after", `$1,2345$`, `1, 2345`},
		{"spaced list of thousands", `$1, 000$`, `1, 000`},
		{"pmod", `$a \equiv b \pmod{m}$`, `a ≡ b (mod m)`},
		{"bmod", `$a \bmod m$`, `a mod m`},
		{"greek", `$\alpha + \beta = \pi$`, `α + β = π`},
		{"mathbb", `$x \in \mathbb{Z}$`, `x ∈ ℤ`},
		{"abs", `$|a_i - a_j|$`, `\|aᵢ - aⱼ\|`},
		{"lt escapes html", `$a<b$`, `a \< b`},
		{"star", `$a * b$`, `a \* b`},
		{"binom", `$\binom{n}{k}$`, `C(n, k)`},
		{"implies", `$p \implies q$`, `p ⇒ q`},
		{"unknown kept", `$\heartsuit$`, `\\heartsuit`},

		// Delimiters.
		{"paren delims", `\(1 \le n \le 100\)`, `1 ≤ n ≤ 100`},
		// \[ is markdown's escaped bracket, which html-to-markdown and the
		// curriculum both write: it never opens math.
		{"escaped brackets", `A\[0...K\] and dp\[i\]\[j\]`, `A\[0...K\] and dp\[i\]\[j\]`},
		{"display", "Compute\n\n$$\n\\sum_{i=1}^{n} i\n$$\n", "Compute\n\n∑ᵢ₌₁ⁿ i\n"},
		{"codeforces", `$$$1 \le n \le 2 \cdot 10^5$$$`, `1 ≤ n ≤ 2 · 10⁵`},
		// Six dollars is one display span; read as three, it swapped math
		// and prose for the rest of the statement.
		{"codeforces display", `$$$$$$\sum_{i=1}^{n} i$$$$$$ where $$$a_i$$$ is big`, `∑ᵢ₌₁ⁿ i where aᵢ is big`},

		// Commands Codeforces statements use.
		{"color", `$\color{red}{aa}b \rightarrow \color{blue}bb$`, `aab → bb`},
		{"textcolor", `$\textcolor{red}{x}$`, `x`},
		{"daggers", `$^\dagger$ and $^\ddagger$`, `^† and ^‡`},
		{"xrightarrow", `$a \xrightarrow{\text{up}} b$`, `a —up→ b`},
		{"operatorname", `$\operatorname{gcd}(x, y)$`, `gcd(x, y)`},
		// The note over a symbol says something: the step, the relation's
		// name, the definition.
		{"overset", `$1 \overset{+7}{\longrightarrow} 8$ and $a \stackrel{\mathrm{def}}{=} b$`, `1 —+7⟶ 8 and a ≝ b`},
		{"stackrel relation", `$a \stackrel{\rho}{\sim} b$`, `a ∼\[ρ] b`},
		{"underset limits", `$\underset{1 \leq i \leq n}{\max} a_i$`, `max\[1 ≤ i ≤ n] aᵢ`},
		// An underbrace's label counts what it spans: it stays, as an index.
		{"underbrace", `$\underbrace{p \times p}_{k \text{ times}}$`, `(p × p)\[k times]`},
		{"underbrace index", `$[\underbrace{a, \dots, a}_{x}, b]$`, `\[(a, …, a)ₓ, b]`},
		{"cancel", `$\require{cancel}\cancel{a_i}$`, `aᵢ`},
		{"bigcirc", `$x \bigcirc y$`, `x ◯ y`},
		{"over", `$2\over 12$`, `2/12`},
		{"array", `$\begin{array}{|c|c|}\hline a & b\end{array}$`, `a, b`},
		{"font switch", `$\bf{1 \le t \le 5}$`, `1 ≤ t ≤ 5`},
		{"overrightarrow", `$\overrightarrow{y}$`, "y⃗"},

		// Things that are not math.
		{"price", `It costs $5 and $10.`, `It costs $5 and $10.`},
		{"escaped dollar", `\$x\$`, `\$x\$`},
		{"no close", `a $ sign`, `a $ sign`},
		{"no blank line crossing", "$a\n\nb$", "$a\n\nb$"},
		{"escaped backslash paren", `\\(not math\\)`, `\\(not math\\)`},
		{"inline code", "Use `$x_i$` and $x_i$", "Use `$x_i$` and xᵢ"},
		{"double backtick code", "``a `$b$` c`` $b$", "``a `$b$` c`` b"},
	}
	runTeX(t, tests)
}

type texCase struct{ name, in, want string }

func runTeX[T ~struct{ name, in, want string }](t *testing.T, tests []T) {
	t.Helper()
	for _, tt := range tests {
		c := texCase(tt)
		t.Run(c.name, func(t *testing.T) {
			if got := TeXToUnicode(c.in); got != c.want {
				t.Errorf("TeXToUnicode(%q)\n got  %q\n want %q", c.in, got, c.want)
			}
		})
	}
}

// Environments are laid out on one line, rows and cells joined into the
// shape that reads as the same structure. Rows ran together before: "y - x,
// if |x-y| > 1 0, if |x-y| ≤ 1" (cf:903D).
func TestTeXEnvironments(t *testing.T) {
	runTeX(t, []texCase{
		{"cases", `$d(x,y) = \begin{cases} y - x, & \text{if } |x-y| > 1 \\ 0, & \text{if } |x-y| \leq 1 \end{cases}$`,
			`d(x, y) = {y - x, if \|x-y\| > 1; 0, if \|x-y\| ≤ 1}`},
		{"cases value, condition", `$\begin{cases} n & n < 10 \\ g(f(n)) & \text{otherwise} \end{cases}$`,
			`{n, n \< 10; g(f(n)), otherwise}`},
		// A row's own punctuation stands in for the separator, and a final
		// period ends the sentence, outside the braces.
		{"cases punctuation", `$\begin{cases} a, & x \geq 0; \\ 0, & x < 0. \end{cases}$`, `{a, x ≥ 0; 0, x \< 0}.`},
		{"doc example", `$\begin{cases} x & x > 0 \\ 0 & \text{else} \end{cases}$`, `{x, x > 0; 0, else}`},
		{"pmatrix", `$A=\begin{pmatrix}a&b\\c&d\end{pmatrix}$`, `A = (a, b; c, d)`},
		{"bmatrix row", `$\begin{bmatrix} 2 & 1 & 3 \end{bmatrix} \rightarrow \begin{bmatrix} 1 & 0 & 3 \end{bmatrix}$`,
			`\[2, 1, 3] → \[1, 0, 3]`},
		{"vmatrix", `$\begin{vmatrix} a & b \\ c & d \end{vmatrix}$`, `\|a, b; c, d\|`},
		{"plain TeX matrix", `$\matrix{a & b \cr c & d}$`, `a, b; c, d`},
		{"array in brackets", `$\left(\begin{array}{cc} 1 & 2 \\ 3 & 4 \end{array}\right)$`, `(1, 2; 3, 4)`},
		{"array empty cells", `$\begin{array}{ccc} a & \to & b \\ & & c \end{array}$`, `a, →, b; c`},
		{"aligned", `$\begin{aligned} y &= x + p \\ y &= -x + q \end{aligned}$`, `y = x + p; y = -x + q`},
		{"align columns", `$\begin{align*} a &= 1 & b &= 2 \\ c &= 3 \end{align*}$`, `a = 1, b = 2; c = 3`},
		{"row spacing", `$\begin{aligned} a \\[2pt] b \\ \end{aligned}$`, `a; b`},
		{"nested", `$\begin{cases} \begin{pmatrix} 1 & 0 \end{pmatrix} & k = 0 \\ 0 & k > 0 \end{cases}$`,
			`{(1, 0), k = 0; 0, k > 0}`},
		{"substack", `$\sum_{\substack{i < j \\ j < k}} x$`, `∑\[i \< j, j \< k] x`},
		{"substack punctuation", `$\max_{\substack{i < k, \\ i \text{ odd}}} a_i$`, `max\[i \< k, i odd] aᵢ`},
	})
}

// A linear a/b binds to whatever touches it, where the stacked fraction
// did not: it is parenthesized when a factor or a power is next to it, and
// a whole number before a numeric fraction makes a mixed number.
func TestTeXFractionGrouping(t *testing.T) {
	runTeX(t, []texCase{
		{"factor after", `$r \leq \frac{3}{2}r$`, `r ≤ (3/2)r`},
		{"factor after a space", `$\frac{1}{2} k \frac{360}{n}$`, `(1/2) k (360/n)`},
		{"bracket after", `$\frac{n}{k}(k - 1) \ge 1$`, `(n/k)(k - 1) ≥ 1`},
		{"power", `$\frac{n}{2}^2$`, `(n/2)²`},
		{"factor before", `$x\frac{d}{dx}$`, `x(d/dx)`},
		{"function", `$4\sin^2\frac{\pi}{n}$`, `4 sin²(π/n)`},
		{"over a fraction", `$a/\frac{b}{c}$`, `a/(b/c)`},
		{"root denominator stays bare", `$\frac{1}{\sqrt{2}}(a+b)$`, `(1/√2)(a+b)`},
		{"mixed number", `$[2\frac{2}{3}; 3 \frac{1}{2}]$`, `\[2⅔; 3½]`},
		{"mixed number without a glyph", `$5\frac{3}{11}$`, `5³⁄₁₁`},
		{"decimal is not a whole number", `$0.5\frac{1}{2}$`, `0.5(1/2)`},
		{"big operator leaves room", `$\sum_{i=1}^{n} \frac{1}{i}$`, `∑ᵢ₌₁ⁿ 1/i`},
		{"apart stays bare", `$\frac{a}{b} + \frac{c}{d} \cdot e$`, `a/b + c/d · e`},
		{"in brackets", `$\left\lceil \frac{n}{2} \right\rceil$`, `⌈n/2⌉`},
	})
}

// \over, \choose and \atop take the group around them. Unbraced, that is
// the whole formula, which the author rarely meant: the operands stop at a
// relation, a comma or a bracket.
func TestTeXInfix(t *testing.T) {
	runTeX(t, []texCase{
		{"choose", `$\sum_{i=1}^{10} {10 \choose i} i^2$`, `∑ᵢ₌₁¹⁰ C(10, i) i²`},
		{"choose in a product", `${p \choose j} \cdot {q \choose B-1-j}$`, `C(p, j) · C(q, B-1-j)`},
		{"over group", `${a+b \over c}$`, `(a+b)/c`},
		{"over stops at a relation", `$(3 + 4) + (4 + 7) \over 2 = 9$`, `((3 + 4) + (4 + 7))/2 = 9`},
		{"choose in parentheses", `$s(A,N)*(A+B \choose A)$`, `s(A, N)\*(C(A+B, A))`},
		{"atop", `$\left\{ {n \atop k} \right\}$`, `{n¦k}`},
	})
}

// A script takes a command with its arguments, as MathJax reads it, and a
// few scripts are something else: a degree, a misplaced \limits, layout.
func TestTeXScripts(t *testing.T) {
	runTeX(t, []texCase{
		{"command argument", `$w_\mathrm{max}$`, `wₘₐₓ`},
		{"text argument", `$\delta_\text{mean of x}$`, `δ\[mean of x]`},
		{"frac argument", `$n^\frac43$`, `n^(4/3)`},
		{"color argument", `$a_\color{red}{p_1}$`, `a\[p\[1]]`},
		{"degree", `$0^\circ < \alpha < 90^{\circ}$`, `0° \< α \< 90°`},
		{"limits misplaced", `$\sum_\limits{i=1}^n a_i$`, `∑ᵢ₌₁ⁿ aᵢ`},
		{"empty script spaces", `$1_{\ \ }1$`, `1 1`},
		{"transpose", `$v^\mathrm{T}$`, `vᵀ`},
		{"script after a space", `$x _ { 1 } + d ^ 2$`, `x₁ + d²`},
		{"script on a relation", `$a \ominus_k b$`, `a ⊖ₖ b`},
		{"root index", `$\sqrt[\leftroot{-2}\uproot{2}4]{x}$`, `∜x`},
		// An escaped underscore with nothing before it is a blank to fill.
		{"blank", `$[998, \_, \_]$ and $\_ x^2$`, `\[998, \_, \_] and \_ x²`},
	})
}

// Commands statements use that the converter printed as TeX.
func TestTeXSymbols(t *testing.T) {
	runTeX(t, []texCase{
		{"relations", `$p \nmid i$, $a \perp b$, $A \subsetneq B$, $p \Longleftrightarrow q$`, `p ∤ i, a ⊥ b, A ⊊ B, p ⟺ q`},
		{"proof marks", `$\therefore x$, $\blacksquare$, $\Box$`, `∴ x, ■, □`},
		{"backslash", `$[1, N] \backslash S$`, `\[1, N] \\ S`},
		{"spacing", `$a \> \& \> b \hspace{2pt} c \mkern-3mu d$`, `a & b c d`},
		{"tag", `$x = 1 \tag{1}$`, `x = 1 (1)`},
		{"sizing", `$\Biggl\lceil x \Biggr\rceil$`, `⌈x⌉`},
		{"functions", `$\cot x + \sinh y$`, `cot x + sinh y`},
		{"Bbb", `$x \in \Bbb R$`, `x ∈ ℝ`},
		// Math nested in \text, its dollars escaped by the Codeforces
		// converter; a price in \text stays a price.
		{"text math", `$\text{if \$d_u \leq \ell\$}$ and $\text{costs \$5}$`, `if dᵤ ≤ ℓ and costs $5`},
	})
}

// TestTeXToUnicodeLeavesCodeBlocks feeds a statement whose sample block is
// full of characters TeX and markdown care about; the block must come back
// byte for byte while the prose around it converts.
func TestTeXToUnicodeLeavesCodeBlocks(t *testing.T) {
	in := "The grid has $N \\le 10$ rows.\n\n" +
		"### SAMPLE INPUT:\n\n" +
		"```\n3 $5$\n#_#\n\\le *a_i*\n```\n\n" +
		"~~~~\n$x$\n~~~\n```\n$y$\n~~~~\n\n" +
		"Then $a_i$.\n"
	want := "The grid has N ≤ 10 rows.\n\n" +
		"### SAMPLE INPUT:\n\n" +
		"```\n3 $5$\n#_#\n\\le *a_i*\n```\n\n" +
		"~~~~\n$x$\n~~~\n```\n$y$\n~~~~\n\n" +
		"Then aᵢ.\n"
	if got := TeXToUnicode(in); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTeXToUnicodeUnclosedFenceStaysCode(t *testing.T) {
	in := "$a$\n```\n$b$\n"
	want := "a\n```\n$b$\n"
	if got := TeXToUnicode(in); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
