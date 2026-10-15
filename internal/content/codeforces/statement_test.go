package codeforces

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the renderer's output")

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
		t.Errorf("%s differs from the renderer's output; rerun with -update and review the diff.\n--- got ---\n%s", p, got)
	}
}

// One statement per formatting case, rendered from the row as Open would
// keep it.
func TestStatementGolden(t *testing.T) {
	t.Parallel()
	rows := realRows(t)
	rows["9999/Z"] = edgeRow()
	tests := []struct {
		id  string
		why string
	}{
		{"4/A", "2010 statement: plain text, no math"},
		{"919/E", "display math inside a sentence; three samples; editorial"},
		{"1520/E", `"**.*.." and "- " lists must stay literal and lists`},
		{"1791/A", "YES/NO in any case: the exact-match warning"},
		{"2028/B", "footnote marks: $$$x$$$$$$^{∗}$$$ is two inline spans"},
		{"2038/M", "float answer, numbered list, long ICPC contest name"},
		{"9999/Z", "made up: every prose edge case"},
	}
	for _, tc := range tests {
		t.Run(strings.ReplaceAll(tc.id, "/", ""), func(t *testing.T) {
			t.Parallel()
			r := rows[tc.id]
			e, ok := newEntry(r)
			if !ok {
				t.Fatalf("%s: newEntry rejected the row", tc.id)
			}
			txt := text{
				Description: r.Description, Input: r.InputFormat, Output: r.OutputFormat,
				Interaction: r.InteractionFormat, Note: r.Note, Editorial: r.Editorial, Examples: r.Examples,
			}
			md := statementMarkdown(e, txt)
			name := strings.ReplaceAll(tc.id, "/", "")
			golden(t, name+".md", md)
			if e.HasEditorial {
				golden(t, name+".editorial.md", editorialMarkdown(e, txt))
			}
			// Every sample is in the statement byte for byte.
			for _, ex := range r.Examples {
				for _, d := range []string{data(ex.Input), data(ex.Output)} {
					if !strings.Contains(md, "text\n"+d+"```") {
						t.Errorf("sample %q is not fenced verbatim", d)
					}
				}
			}
		})
	}
}

func TestProse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
	}{
		{"inline", "Let $$$a_i$$$ be", "Let $a_i$ be"},
		{"inline spaces trimmed", "$$$ n $$$ items", "$n$ items"},
		{"digit after closer", "$$$10$$$5", `\(10\)5`},
		{"adjacent spans merge", "$$$[0, n)$$$$$$^{\\text{∗}}$$$", "$[0, n)^{\\text{∗}}$"},
		{"display", "so $$$$$$x^2$$$$$$ holds", "so\n\n$$x^2$$\n\nholds"},
		{"ocr inline", "for $$i \\in S$$, and", "for $i \\in S$, and"},
		{"ocr display", "a\n$$x$$\nb", "a\n\n$$x$$\n\nb"},
		{"single dollar math", "value $x$ here", "value $x$ here"},
		{"price", "costs $5 now", `costs \$5 now`},
		{"unclosed", "a $$$ b", `a \$\$\$ b`},
		{"escaped dollar in math", "$$$a \\$ b$$$", `$a \$ b$`},
		{"bare dollar in math", "$$$$100$$$", `$\$100$`},
		{"emphasis chars", `"**.*.." and a_b`, `"\*\*.\*.." and a\_b`},
		{"heading and quote", "#define x\n> y", `\#define x` + "\n\n" + `\> y`},
		{"lines are paragraphs", "one\ntwo\n\n\nthree", "one\n\ntwo\n\nthree"},
		{"indent dropped", "a\n    code?", "a\n\ncode?"},
		{"nbsp", "a\u00a0b", "a b"},
		{"tags", "<p>x</p><b>y</b>", "x\n\ny"},
		{"backslash outside math", `C:\dir \( not math`, `C:\\dir \\( not math`},
		{"crlf", "a\r\nb", "a\n\nb"},
		{"empty", "  \n ", ""},
	}
	for _, tc := range tests {
		if got := prose(tc.in); got != tc.want {
			t.Errorf("%s: prose(%q) =\n%q\nwant\n%q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestData(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"1 2", "1 2\n"},
		{"1 2\r\n3\r\n", "1 2\n3\n"},
		{"x\n", "x\n"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := data(tc.in); got != tc.want {
			t.Errorf("data(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if got := string(dataBytes([]byte(tc.in))); got != tc.want {
			t.Errorf("dataBytes(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	shared := []byte("ab")[:1]
	if dataBytes(shared); string(shared[:2]) != "ab" {
		t.Error("dataBytes wrote past the slice it was given")
	}
}

func TestFence(t *testing.T) {
	t.Parallel()
	if got := fence("1\n"); got != "```text\n1\n```\n" {
		t.Errorf("fence = %q", got)
	}
	if got := fence("```\n"); got != "````text\n```\n````\n" {
		t.Errorf("fence with backticks = %q", got)
	}
}
