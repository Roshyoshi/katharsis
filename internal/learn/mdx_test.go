package learn

import (
	"strings"
	"testing"
)

var testLists = []ProblemList{
	{Key: "sample", Problems: []ProblemRef{
		{UniqueID: "usaco-572", ID: "usaco:572", Name: "Breed Counting", URL: "http://usaco.org/572", Source: "Silver", Difficulty: "Easy"},
	}},
	{Key: "general", Problems: []ProblemRef{
		{UniqueID: "usaco-572", ID: "usaco:572", Name: "Breed Counting", URL: "http://usaco.org/572", Source: "Silver", Difficulty: "Easy", Starred: true},
		{UniqueID: "cf-1|2", Name: "Pipe | Name", URL: "https://cf/1", Source: "CF", Difficulty: "Hard"},
	}},
}

func convertSnippet(src string) (string, *renderer, error) {
	nodes, _, err := parseMDX("m.mdx", src, 1)
	if err != nil {
		return "", nil, err
	}
	r := &renderer{file: "m.mdx", moduleURL: "https://usaco.guide/silver/m", lists: map[string]ProblemList{}, unknown: map[string]int{}}
	for _, l := range testLists {
		r.lists[l.Key] = l
	}
	md, err := r.block(nodes)
	return md, r, err
}

func TestConvert(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{
			name: "prose, headings and lists pass through",
			src:  "## Intro\n\nSome **bold** text.\n\n- a\n  - b\n",
			want: "## Intro\n\nSome **bold** text.\n\n- a\n  - b",
		},
		{
			name: "code fence is verbatim, JSX-looking text included",
			src:  "```cpp\n#include <bits/stdc++.h>\nint main() { return 0; }\n```\n",
			want: "```cpp\n#include <bits/stdc++.h>\nint main() { return 0; }\n```",
		},
		{
			name: "fence longer than backticks inside the body",
			src:  "````md\n```cpp\nx\n```\n````\n",
			want: "````md\n```cpp\nx\n```\n````",
		},
		{
			name: "indented fence inside JSX is dedented to its fence",
			src:  "<Spoiler title=\"Code\">\n\t\t```cpp\n\t\tint x;\n\t\t\tint y;\n\t\t```\n</Spoiler>\n",
			want: "**▸ Spoiler: Code**\n\n> ```cpp\n> int x;\n> \tint y;\n> ```",
		},
		{
			name: "LanguageSection keeps C++",
			src: "<LanguageSection>\n<CPPSection>\n\nC++ text.\n\n```cpp\nA\n```\n\n</CPPSection>\n" +
				"<JavaSection>\n\n```java\nB\n```\n\n</JavaSection>\n<PySection>\n\n```py\nC\n```\n\n</PySection>\n</LanguageSection>\n",
			want: "C++ text.\n\n```cpp\nA\n```",
		},
		{
			name: "LanguageSection without C++ falls back to Java",
			src:  "<LanguageSection>\n<PySection>\n\n```py\nC\n```\n\n</PySection>\n<JavaSection>\n\n```java\nB\n```\n\n</JavaSection>\n</LanguageSection>\n",
			want: "*(Not available in C++; showing Java.)*\n\n```java\nB\n```",
		},
		{
			name: "CPPOnly kept, JavaOnly and PyOnly dropped",
			src:  "<CPPOnly>\n\nfor C++\n\n</CPPOnly>\n\n<JavaOnly>\n\nfor Java\n\n</JavaOnly>\n<PyOnly>\n\nfor Py\n\n</PyOnly>\n",
			want: "for C++",
		},
		{
			name: "callouts",
			src:  "<Info title=\"Pro Tip\">\n\nRead this.\n\n</Info>\n\n<Warning>\n\nCareful!\n\n</Warning>\n\n<Optional>\n\nExtra.\n\n</Optional>\n",
			want: "> **Info: Pro Tip**\n>\n> Read this.\n\n> **Warning**\n>\n> Careful!\n\n> **Optional**\n>\n> Extra.",
		},
		{
			name: "spoiler without title",
			src:  "<Spoiler>\n\nx\n\n</Spoiler>\n",
			want: "**▸ Spoiler**\n\n> x",
		},
		{
			name: "nested callout in spoiler",
			src:  "<Spoiler title=\"Hint\">\n\n<Info title=\"x\">\n\ny\n\n</Info>\n\n</Spoiler>\n",
			want: "**▸ Spoiler: Hint**\n\n> > **Info: x**\n> >\n> > y",
		},
		{
			name: "incomplete section",
			src:  "<IncompleteSection />\n",
			want: "> **Incomplete section**\n>\n> This section is not complete. Any help would be appreciated!",
		},
		{
			name: "focus problem and problem table",
			src:  "<FocusProblem problem=\"sample\" />\n\n<Problems problems={\"general\"} hideSuggestProblemButton />\n",
			want: "> **Focus Problem**\n>\n> [Breed Counting](http://usaco.org/572) · Silver · Easy\n\n" +
				"| Problem | Source | Difficulty |\n| --- | --- | --- |\n" +
				"| ★ [Breed Counting](http://usaco.org/572) | Silver | Easy |\n" +
				"| [Pipe \\| Name](https://cf/1) | CF | Hard |",
		},
		{
			name: "resources",
			src: "<Resources>\n\t<Resource source=\"CPH\" title=\"9.1 - Sum Queries\" starred>\n\t\tBrief *description*\n\t</Resource>\n" +
				"\t<Resource source=\"CF\" title=\"Blog\" url=\"1234\" />\n\t<Resource source=\"X\" title=\"Y\" url=\"https://y.example\" />\n</Resources>\n",
			want: "**Resources**\n\n- ★ [9.1 - Sum Queries](https://usaco.guide/CPH.pdf) (CPH) — Brief *description*\n" +
				"- [Blog](https://codeforces.com/blog/entry/1234) (CF)\n- [Y](https://y.example) (X)",
		},
		{
			name: "inline math becomes escaped text, display math a math block",
			src:  "Runs in $\\mathcal{O}(N \\log N)$ for $a_i \\le 10^{9}$.\n\n$$\n\\sum_{i=1}^{n} \\frac{a+b}{2}\n$$\n",
			want: "Runs in O(N log N) for a\\_i ≤ 10^9.\n\n```math\nΣ_(i=1)^n (a+b)/2\n```",
		},
		{
			name: "escaped dollar and unclosed dollar stay literal",
			src:  "Costs \\$5 and $ more.\n",
			want: "Costs \\$5 and $ more.",
		},
		{
			name: "expressions and comments",
			src:  "a{' '}b{/* gone */}c <!-- hidden <b>x</b> -->d{// line\n'\\u00e9'}\n",
			want: "a bc dé",
		},
		{
			name: "non-ASCII prose is kept byte for byte",
			src:  "Maksim’s blog — naïve “quotes” …\n",
			want: "Maksim’s blog — naïve “quotes” …",
		},
		{
			name: "inline code is not parsed",
			src:  "Use `vector<int>` and `{x}` and `` a`b ``.\n",
			want: "Use `vector<int>` and `{x}` and `` a`b ``.",
		},
		{
			name: "images and root-relative links",
			src:  "See ![Treap](/content/treap.png) and [this](/silver/prefix-sums#x), [ext](https://e.x), [wiki](<https://w/a_(b)>).\n\n<img src=\"https://i/x.svg\" alt=\"Join *us*\" />\n",
			want: "See [image: Treap](https://usaco.guide/content/treap.png) and [this](https://usaco.guide/silver/prefix-sums#x), [ext](https://e.x), [wiki](<https://w/a_(b)>).\n\n[image: Join \\*us\\*](https://i/x.svg)",
		},
		{
			name: "media and widgets",
			src:  "<YouTube id=\"abc\" />\n\n<video controls>\n\t<source src=\"/animations/x.mp4\" type=\"video/mp4\"/>\n</video>\n\n<iframe src=\"https://e/1\" title=\"Demo\"></iframe>\n\n<PrefixSumInteractive />\n\n<svg viewBox=\"0 0 1 1\">\n\t<line x1=\"0\" style={{ stroke: 'red' }} />\n</svg>\n",
			want: "[video: YouTube](https://www.youtube.com/watch?v=abc)\n\n[video](https://usaco.guide/animations/x.mp4)\n\n[embed: Demo](https://e/1)\n\n" +
				"[interactive demo: open in browser](https://usaco.guide/silver/m)\n\n[figure: open in browser](https://usaco.guide/silver/m)",
		},
		{
			name: "inline components",
			src:  "Fast<Asterisk>usually</Asterisk> and <TextTooltip content=\"popup\">word</TextTooltip>, <b>bold</b>, <a href=\"/x\">a</a>.\n",
			want: "Fast (note: usually) and word (popup), **bold**, [a](https://usaco.guide/x).",
		},
		{
			name: "HTML table",
			src: "<center>\n\n<table>\n\t<thead>\n\t\t<tr>\n\t\t\t<th>Index $i$</th>\n\t\t\t<th>1</th>\n\t\t</tr>\n\t</thead>\n" +
				"\t<tbody>\n\t\t<tr>\n\t\t\t<td>$|x|$</td>\n\t\t\t<td className=\"bg\">6</td>\n\t\t</tr>\n\t</tbody>\n</table>\n\n</center>\n",
			want: "| Index i | 1 |\n| --- | --- |\n| \\|x\\| | 6 |",
		},
		{
			name: "unknown tags keep inner text",
			src:  "<Mystery foo=\"bar\">\n\nInner **text**.\n\n</Mystery>\n\nand <Odd>inline</Odd> too\n",
			want: "Inner **text**.\n\nand inline too",
		},
		{
			name: "quiz",
			src: "<Quiz>\n  <Quiz.Question>\n    Which is $O(1)$?\n    <Quiz.Answer>\n      Loop\n      <Quiz.Explanation>\n        No.\n      </Quiz.Explanation>\n    </Quiz.Answer>\n" +
				"    <Quiz.Answer correct>\n      Lookup\n      <Quiz.Explanation>\n        Yes.\n      </Quiz.Explanation>\n    </Quiz.Answer>\n  </Quiz.Question>\n</Quiz>\n",
			want: "**Question 1.** Which is O(1)?\n\n**(a)** Loop\n\n**(b)** Lookup\n\n**▸ Spoiler: Answer 1**\n\n> **(a)** ✗ No.\n>\n> **(b)** ✓ Yes.",
		},
		{
			name: "indented JSX content does not become indented code",
			src:  "<Warning>\n        Deeply indented prose\n        on two lines.\n\n        - list\n          - nested\n</Warning>\n",
			want: "> **Warning**\n>\n> Deeply indented prose\n> on two lines.\n>\n> - list\n>   - nested",
		},
		{
			name: "fenced code inside a blockquote is kept verbatim",
			src:  "> Quote:\n> ```cpp\n> if (a<b) { x(); }\n> ```\n",
			want: "> Quote:\n> ```cpp\n> if (a<b) { x(); }\n> ```",
		},
		{
			name: "ESM lines are dropped at top level",
			src:  "import X from './x';\nexport const meta = {};\n\nText.\n",
			want: "Text.",
		},
		{
			name: "import inside a paragraph is prose",
			src:  "We then\nimport the data.\n",
			want: "We then\nimport the data.",
		},
		{
			name: "fragments and br",
			src:  "<>\n\nIn a fragment.\n\n</>\n\nline one<br/>line two\n",
			want: "In a fragment.\n\nline one\\\nline two",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := convertSnippet(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got:\n%s\n---- want:\n%s", got, tc.want)
			}
		})
	}
}

func TestConvertUnknownTagsCounted(t *testing.T) {
	_, r, err := convertSnippet("<Mystery>\n\nx\n\n</Mystery>\n<Mystery />\n")
	if err != nil {
		t.Fatal(err)
	}
	if r.unknown["Mystery"] != 2 {
		t.Errorf("unknown = %v", r.unknown)
	}
}

func TestConvertErrors(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"unclosed element", "text\n\n<Spoiler title=\"x\">\n\nbody\n", "m.mdx:3: <Spoiler> is never closed"},
		{"mismatched close", "<Info>\n\nx\n\n</Warning>\n", "m.mdx:5: </Warning> closes <Info>"},
		{"stray close", "x\n</Info>\n", "m.mdx:2: </Info> closes nothing"},
		{"unclosed fence", "a\n\n```cpp\nint x;\n", "m.mdx:3: code fence ``` is never closed"},
		{"unclosed tag", "<Spoiler title=\"x\"\n", "m.mdx:1: tag <Spoiler is never closed"},
		{"bad attribute", "<Spoiler title=x>\n</Spoiler>\n", `m.mdx:1: attribute title: unexpected 'x'`},
		{"unclosed expression", "a {' '\n", "m.mdx:1: {expression} is never closed"},
		{"unclosed comment", "a <!-- b\n", "m.mdx:1: HTML comment is never closed"},
		{"unclosed display math", "$$\nx\n", "m.mdx:1: $$ display math is never closed"},
		{"unknown problem list", "\n<Problems problems=\"nope\" />\n", `m.mdx:2: <Problems problems="nope">: no such list`},
		{"focus list size", "<FocusProblem problem=\"general\" />\n", `focus problem list "general" has 2 problems`},
		{"code in inline element", "<Asterisk>\n```cpp\nx\n```\n</Asterisk> tail\n", "code block inside an inline element"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := convertSnippet(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSimplifyTeX(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`\mathcal{O}(N\log^2 N)`, "O(N log^2 N)"},
		{`\texttt{prefix}[k]`, "prefix[k]"},
		{`1\le a\le b\le N`, "1 ≤ a ≤ b ≤ N"},
		{`\lfloor \frac{n}{2} \rfloor`, "⌊ n/2 ⌋"},
		{`2^{n-1} \cdot 10^{5}`, "2^(n-1) · 10^5"},
		{`\binom{n}{k} \pmod{p}`, "C(n, k)  (mod p)"},
		{`\left( \sum_{i} a_i \right)`, "( Σ_i a_i )"},
		{`\{1, 2\} \cup \{3\}`, "{1, 2} ∪ {3}"},
		{`\unknowncmd{x}`, `\unknowncmdx`},
		{`\sqrt{n} + \sqrt x`, "√n + √x"},
		{`\sqrt[3]{\log n} \sqrt[k]{x}`, "∛(log n) x^(1/k)"},
		{`\é`, `\é`},
	} {
		if got := simplifyTeX(tc.in); got != tc.want {
			t.Errorf("simplifyTeX(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDisplayTeX(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"\\begin{align*}\nF_0 &= 0 \\\\\nF_{n+1} &= F_{n} + F_{n-1}\n\\end{align*}", "F_0 = 0\nF_(n+1) = F_n + F_(n-1)"},
		{`A = \begin{bmatrix}1&1\\1&0\end{bmatrix}^n`, "A = [1 1; 1 0]^n"},
		{"\\begin{pmatrix}\n  a & b \\\\\n  c & d\n\\end{pmatrix}", "(a b; c d)"},
		{`f(x) = \begin{cases} 1 & x > 0 \\ 0 & \text{otherwise} \end{cases}`, "f(x) = {1, x > 0; 0, otherwise}"},
		{"a\n+ b", "a + b"},
		{`d = \begin{cases} 1 & \text{if $x \in S$} \end{cases}`, "d = {1, if x ∈ S}"},
	} {
		if got := displayTeX(tc.in); got != tc.want {
			t.Errorf("displayTeX(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
