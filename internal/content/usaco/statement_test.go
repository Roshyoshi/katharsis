package usaco

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the parser's output")

// golden compares got with testdata/golden/name, or rewrites it with
// -update. Review the diff of an update: the files are what a player
// reads.
func golden(t *testing.T, name, got string) {
	t.Helper()
	p := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from the parser's output; rerun with -update and review the diff.\n--- got ---\n%s", p, got)
	}
}

func readPage(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "pages", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The pages are saved from usaco.org (2026-10-14), one per format quirk.
func TestParseStatement(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cpid    string
		why     string
		want    facts
		samples int
		in, out string // first sample
	}{
		{"84", "Nov 2011 plain text; zip uses I.N/O.N",
			facts{Title: "Contest Timing", Contest: "2011 November", Division: "bronze", Position: 1, FileIO: "ctiming"},
			1, "12 13 14\n", "1563\n"},
		{"208", "plain text with list items and a long sample",
			facts{Title: "Crazy Fences", Contest: "2012 December", Division: "silver", Position: 1, FileIO: "crazy"},
			1, "10 4\n0 0 10 0\n10 0 10 10\n0 0 0 10\n10 10 0 10\n8 8 9 8\n9 8 8 9\n8 9 8 8\n2 7 3 2\n3 2 7 5\n7 5 2 7\n15 3\n1 4\n4 5\n7 1\n", "2\n"},
		{"516", "plain text, 'SAMPLE OUTPUT: (file grass.out)'",
			facts{Title: "Grass Cownoisseur", Contest: "2015 January", Division: "gold", Position: 3, FileIO: "grass"},
			1, "7 10\n1 2\n3 1\n2 5\n2 4\n3 7\n3 5\n3 6\n6 5\n7 2\n4 7\n", "6\n"},
		{"526", "early HTML: bare <pre> after <h4>SAMPLE INPUT:</h4>",
			facts{Title: "Censoring (Bronze)", Contest: "2015 February", Division: "bronze", Position: 1, FileIO: "censor"},
			1, "whatthemomooofun\nmoo\n", "whatthefun\n"},
		{"532", "'INPUT FORMAT: (hopscotch.in)' without the word file",
			facts{Title: "Cow Hopscotch (Gold)", Contest: "2015 February", Division: "gold", Position: 1, FileIO: "hopscotch"},
			1, "4 4 4\n1 1 1 1\n1 3 2 1\n1 2 4 1\n1 1 1 1\n", "5\n"},
		{"552", "interactive, no samples, 'running time limit of 4 seconds'",
			facts{Title: "Googol", Contest: "2015 US Open", Division: "gold", Position: 1,
				TimeLimit: 4 * time.Second, Interactive: true},
			0, "", ""},
		{"567", "file-I/O era HTML",
			facts{Title: "Fence Painting", Contest: "2015 December", Division: "bronze", Position: 1, FileIO: "paint"},
			1, "7 10\n4 8\n", "6\n"},
		{"1059", "first stdin contest",
			facts{Title: "Do You Know Your ABCs?", Contest: "2020 December", Division: "bronze", Position: 1},
			1, "2 2 11 4 9 7 9\n", "2 2 7\n"},
		{"1094", "'50 percent higher' time and 512MB memory in one sentence",
			facts{Title: "Paint by Letters", Contest: "2021 January", Division: "platinum", Position: 3,
				TimeLimit: 3 * time.Second, MemLimit: 512 << 20},
			1, "", ""},
		{"1210", "float answer with 10^{-6} absolute or relative error",
			facts{Title: "Cow Camp", Contest: "2022 February", Division: "gold", Position: 2,
				FloatTolerance: 1e-6},
			2, "", ""},
		{"1211", "'time limit for this problem is 4s' in <strong>**...**</strong>",
			facts{Title: "Moo Network", Contest: "2022 February", Division: "gold", Position: 3,
				TimeLimit: 4 * time.Second},
			1, "", ""},
		{"1254", "4s, memory 'also twice the default', 'output any'",
			facts{Title: "Barn Tree", Contest: "2022 December", Division: "silver", Position: 1,
				TimeLimit: 4 * time.Second, MemLimit: 512 << 20, NeedsChecker: true},
			1, "", ""},
		{"1327", "15s for Python only; the C++ limit is the default 2s",
			facts{Title: "Field Day", Contest: "2023 US Open", Division: "silver", Position: 2,
				TimeLimit: 2 * time.Second},
			1, "", ""},
		{"1141", "inline math with \\le, subscripts and braces; a sample with a blank line",
			facts{Title: "Routing Schemes", Contest: "2021 US Open", Division: "platinum", Position: 2},
			3, "", ""},
	}
	// Math must reach the reader verbatim, as USACO wrote it: the reader
	// typesets TeX itself. Each of these is copied from a page's source.
	verbatimMath := map[string][]string{
		"1059": {`$A\le B\le C$`, `$1 \ldots 10^9$`},
		"1211": {`$(x_i,y_i)$`, `$0 \leq x_i \leq 10^6$`, `$(x_i-x_j)^2 + (y_i-y_j)^2$`},
		"1141": {`$2\le N\le 100$`, `$\{1,2,3\}$`, `$v_i\to v_{i+1}$`, `$i<j$`},
		"1210": {`$10^{-6}$`},
	}
	for _, tc := range tests {
		t.Run(tc.cpid, func(t *testing.T) {
			t.Parallel()
			st, err := parseStatement(readPage(t, tc.cpid+".html"), statementURL(atoi(tc.cpid)))
			if err != nil {
				t.Fatal(err)
			}
			if st.facts != tc.want {
				t.Errorf("%s\nfacts = %+v\nwant    %+v", tc.why, st.facts, tc.want)
			}
			if len(st.Samples) != tc.samples {
				t.Fatalf("%d samples, want %d", len(st.Samples), tc.samples)
			}
			if tc.in != "" && (st.Samples[0].In != tc.in || st.Samples[0].Out != tc.out) {
				t.Errorf("sample 1 = %q -> %q, want %q -> %q", st.Samples[0].In, st.Samples[0].Out, tc.in, tc.out)
			}
			// Each sample appears as a text fence holding its exact bytes.
			for i, s := range st.Samples {
				for _, data := range []string{s.In, s.Out} {
					if !strings.Contains(st.Markdown, "```text\n"+data+"```\n") {
						t.Errorf("sample %d: no ```text fence with %q", i+1, data)
					}
				}
			}
			for _, m := range verbatimMath[tc.cpid] {
				if !strings.Contains(st.Markdown, m) {
					t.Errorf("markdown lacks verbatim math %s", m)
				}
			}
			for _, junk := range []string{`\\{`, `\\(`, `\*\*`, "DCVERB", "[email", "Return to Problem List", "Log in"} {
				if strings.Contains(st.Markdown, junk) {
					t.Errorf("markdown contains %q", junk)
				}
			}
			golden(t, tc.cpid+".md", st.Markdown)
		})
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func TestEditorialMarkdown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		page, statement string
		want            []string // substrings
	}{
		{"sol_ctiming.html", "84.html", []string{"Solution Notes:", "```cpp", `freopen ("ctiming.in", "r", stdin);`}},
		{"sol_prob1_bronze_dec20.html", "1059.html", []string{"(Analysis by Brian Dean and Benjamin Qi)", "```cpp", "```python", "$A+B+C$"}},
	}
	for _, tc := range tests {
		t.Run(tc.page, func(t *testing.T) {
			t.Parallel()
			st, err := parseStatement(readPage(t, tc.statement), "")
			if err != nil {
				t.Fatal(err)
			}
			md, err := editorialMarkdown(readPage(t, tc.page), st, "https://usaco.org/current/data/"+tc.page)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(md, w) {
					t.Errorf("editorial lacks %q", w)
				}
			}
			for _, junk := range []string{"MathJax", "__CF$cv$params", "<script", "DCVERB"} {
				if strings.Contains(md, junk) {
					t.Errorf("editorial contains %q", junk)
				}
			}
			golden(t, strings.TrimSuffix(tc.page, ".html")+".md", md)
		})
	}
	// A page with no <body> (a frameset) must not crash the fetch.
	t.Run("frameset", func(t *testing.T) {
		t.Parallel()
		st, err := parseStatement(readPage(t, "84.html"), "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := editorialMarkdown([]byte("<frameset><frame src=x></frameset>"), st, ""); err != nil {
			t.Error(err)
		}
	})
}

func TestMapMath(t *testing.T) {
	t.Parallel()
	mark := func(s string) string { return "<" + s + ">" }
	tests := []struct{ in, want string }{
		{`range $1 \ldots 10^9$ inclusive`, `range <$1 \ldots 10^9$> inclusive`},
		{`costs \$5 and $N$ cows`, `costs <\$>5 and <$N$> cows`},
		{`display $$x^2$$ math`, `display <$$x^2$$> math`},
		{`paren \(a_i\) and \[\sum_{i} a_i\]`, `paren <\(a_i\)> and <\[\sum_{i} a_i\]>`},
		{`escaped $a \$ b$ inside`, `escaped <$a \$ b$> inside`},
		{`two $a$$b$ spans`, `two <$a$><$b$> spans`},
		{`unclosed $ sign`, `unclosed $ sign`},
		{`no math here`, `no math here`},
	}
	for _, tc := range tests {
		if got := mapMath(tc.in, mark); got != tc.want {
			t.Errorf("mapMath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		t    time.Duration
		mem  int64
	}{
		{"Note: the time limit for this problem is 4s, twice the default.", 4 * time.Second, 0},
		{"Note that this problem has a time limit of 6 seconds per test case (and twice that for Java and Python submissions).", 6 * time.Second, 0},
		{"Note that the time limit for this problem is slightly larger than the default: 4 seconds for C/C++/Pascal, and 8 seconds for Java/Python.", 4 * time.Second, 0},
		{"Note that for this problem, the standard time limit is increased to 3 seconds per test case (6 seconds per case for Java and Python).", 3 * time.Second, 0},
		{"Note: the time limit per test case on this problem is twice the default.", 4 * time.Second, 0},
		{"Note: The time limit for this problem is 50 percent higher than the default, and the memory limit is 512MB, twice the default.", 3 * time.Second, 512 << 20},
		{"Note: the time and memory limits for this problem are 4s and 512MB, twice the defaults.", 4 * time.Second, 512 << 20},
		{"Note: The time limit and memory limit for this problem are 3s and 512MB, which are 1.5x and 2x the normal amount, respectively.", 3 * time.Second, 512 << 20},
		{"Note: the time limit for this problem is 5s, 2.5 times the default. The memory limit is twice the default.", 5 * time.Second, 512 << 20},
		{"Note: The time limit for this problem is 2.5s, 1.25 times the default.", 2500 * time.Millisecond, 0},
		{"Note: The time limit for this problem in Python is 15s. Other languages have the default time limit of 2s.", 2 * time.Second, 0},
		{"Note: This problem has an expanded memory limit of 512 MB.", 0, 512 << 20},
		{"The memory limit for this problem is set to 512MB, above the usual 256MB limit.", 0, 512 << 20},
		{"Do not forget the standard memory limit of 64MB for all problems.", 0, 64 << 20},
		{"than the 3 hour (180 minute) time limit for the contest.", 0, 0},
		{"Y (nearly 15,000^2 possibilities!) will not run within the time limit, so it would not receive full credit.", 0, 0},
	}
	for _, tc := range tests {
		gt, gm := limits(squash(tc.text))
		if gt != tc.t || gm != tc.mem {
			t.Errorf("limits(%q) = %v, %d; want %v, %d", tc.text, gt, gm, tc.t, tc.mem)
		}
	}
}

func TestChecker(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want bool
	}{
		{"If there are multiple permutations, any one will be accepted.", true},
		{"Any valid output will be accepted.", true},
		{"If there are multiple solutions, output any.", true},
		{"If there are multiple optimal instruction sequences, your program may output any of them.", true},
		{"so any valid construction with either M=2 or M=3 would be accepted.", true},
		{"For instance, the array [4, 3, 2, 1] would also be accepted.", true},
		{"If there are multiple solutions, output the solution with the smallest optimal tuition.", false},
		{"There may be multiple possible answers, in which case you should output the lexicographically smallest.", false},
		{"The grader is sensitive to whitespace, so make sure not to output any extraneous spaces.", false},
		{"lowest name of all possible cows that could appear first in any valid ordering.", false},
	}
	for _, tc := range tests {
		if got := checkerRE.MatchString(tc.text); got != tc.want {
			t.Errorf("checker(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

// "[" must reach the reader unescaped: it reads "\[" as display math, and
// "A\[1...N\]" showed as "A1...N" (cpids 91, 97, 508).
func TestBracketsStayLiteral(t *testing.T) {
	t.Parallel()
	md, _ := plainToMarkdown("Bessie keeps an array A[1...N], where A[j] is the j-th cow.\n", nil)
	if strings.Contains(md, `\[`) || !strings.Contains(md, `A[1...N\]`) {
		t.Errorf("plainToMarkdown = %q, want A[1...N\\] and no \\[", md)
	}
	for _, tc := range []struct{ in, want string }{
		{"P\\[j] sums A\\[1...j].\n", "P[j] sums A[1...j].\n"},
		{"keep \\* and \\\\[x]\n", "keep \\* and \\\\[x]\n"},
		{"code `a\\[i]` stays\n", "code `a\\[i]` stays\n"},
		{"```cpp\nx = \"\\[\";\n```\nP\\[i]\n", "```cpp\nx = \"\\[\";\n```\nP[i]\n"},
	} {
		if got := unescapeBrackets(tc.in); got != tc.want {
			t.Errorf("unescapeBrackets(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
