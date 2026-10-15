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
		{"bracket delims", `\[\sum_{i=1}^{n} i\]`, `∑ᵢ₌₁ⁿ i`},
		{"display", "Compute\n\n$$\n\\sum_{i=1}^{n} i\n$$\n", "Compute\n\n∑ᵢ₌₁ⁿ i\n"},
		{"codeforces", `$$$1 \le n \le 2 \cdot 10^5$$$`, `1 ≤ n ≤ 2 · 10⁵`},

		// Things that are not math.
		{"price", `It costs $5 and $10.`, `It costs $5 and $10.`},
		{"escaped dollar", `\$x\$`, `\$x\$`},
		{"no close", `a $ sign`, `a $ sign`},
		{"no blank line crossing", "$a\n\nb$", "$a\n\nb$"},
		{"escaped backslash paren", `\\(not math\\)`, `\\(not math\\)`},
		{"inline code", "Use `$x_i$` and $x_i$", "Use `$x_i$` and xᵢ"},
		{"double backtick code", "``a `$b$` c`` $b$", "``a `$b$` c`` b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TeXToUnicode(tt.in); got != tt.want {
				t.Errorf("TeXToUnicode(%q)\n got  %q\n want %q", tt.in, got, tt.want)
			}
		})
	}
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
