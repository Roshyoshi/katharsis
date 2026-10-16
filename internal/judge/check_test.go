package judge_test

import (
	"strings"
	"testing"

	"katharsis/internal/judge"
)

// checkCase is a checker input and the expected result; a nil diff means
// accepted.
type checkCase struct {
	name      string
	want, got string
	diff      *judge.Diff
}

func runCheck(t *testing.T, tests []checkCase, check func(want, got []byte) (bool, *judge.Diff)) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, d := check([]byte(tc.want), []byte(tc.got))
			if tc.diff == nil {
				if !ok || d != nil {
					t.Fatalf("check(%q, %q) = %v, %+v; want accepted", tc.want, tc.got, ok, d)
				}
				return
			}
			if ok || d == nil {
				t.Fatalf("check(%q, %q) = %v, %+v; want %+v", tc.want, tc.got, ok, d, *tc.diff)
			}
			got := *d
			if tc.diff.GotLine == 0 {
				got.GotLine = 0 // a case that pins only the expected side
			}
			if got != *tc.diff {
				t.Errorf("diff = %+v, want %+v", *d, *tc.diff)
			}
		})
	}
}

func diff(line int, want, got string) *judge.Diff {
	return &judge.Diff{Line: line, Want: want, Got: got}
}

func TestCheck(t *testing.T) {
	t.Parallel()
	long := func(c byte) string { return strings.Repeat(string(c), 200) }
	runCheck(t, []checkCase{
		{"exact", "1\n2\n", "1\n2\n", nil},
		{"missing final newline", "1\n2\n", "1\n2", nil},
		{"trailing spaces and tabs", "1 2\n3\n", "1 2  \n3\t\n", nil},
		{"crlf", "1\n2\n", "1\r\n2\r\n", nil},
		{"crlf in expected", "1\r\n2\r\n", "1\n2\n", nil},
		{"lone cr", "1\n2\n", "1\r2\r", nil},
		{"trailing blank lines", "1\n", "1\n\n\n", nil},
		{"trailing whitespace-only lines", "1\n", "1\n  \n\t\r\n", nil},
		{"both empty", "", "", nil},
		{"empty vs blank lines", "", "\n\n", nil},
		// What wcmp accepts and a line checker does not (cf:1A, cf:1971A).
		{"leading space", "4\n", " 4\n", nil},
		{"inner spaces", "1 2\n", "1  2\n", nil},
		{"blank line in the middle", "1\n\n2\n", "1\n2\n", nil},
		{"one per line for one line", "1 9\n2 5\n", "1\n9\n2\n5\n", nil},
		{"one line for one per line", "1\n2\n3\n", "1 2 3", nil},

		{"line 1 differs", "3\n", "4\n", diff(1, "3", "4")},
		{"line 2 differs", "1\n2\n3\n", "1\n5\n3\n", diff(2, "2", "5")},
		{"diff names the token", "1 2 3\n", "1 2 4\n", diff(1, "3", "4")},
		{"diff names the expected line", "1\n2\n3\n", "1 2 4\n", diff(3, "3", "4")},
		{"got shorter", "1\n2\n", "1\n", diff(2, "2", "<EOF>")},
		{"want shorter", "1\n", "1\n2\n", diff(2, "<EOF>", "2")},
		{"no output", "7\n", "", diff(1, "7", "<EOF>")},
		{"missing line that reads <EOF>", "1\n<EOF>\n", "1\n", diff(2, "<EOF>", "<EOF>")},
		{"case matters", "YES\n", "yes\n", diff(1, "YES", "yes")},
		{"form feed is not a blank", "1\n", "1\f\n", diff(1, "1", `1\x0c`)},
		{"numbers are compared as text", "0.5\n", "0.50\n", diff(1, "0.5", "0.50")},
		{"long tokens clipped", long('a'), long('b'), diff(1, long('a')[:120]+"…", long('b')[:120]+"…")},
		{"clipped on a rune boundary", "x\n", strings.Repeat("é", 100), diff(1, "x", strings.Repeat("é", 60)+"…")},
		{"escape sequences escaped", "x\n", "\x1b[31mred\n", diff(1, "x", `\x1b[31mred`)},
		{"invalid utf-8 escaped", "x\n", "\xff\xfe\n", diff(1, "x", `\xff\xfe`)},
		{"C1 control escaped", "x\n", "a\u009bb\n", diff(1, "x", `a\xc2\x9bb`)},
		{"NUL escaped", "1\n", "1\x00\n", diff(1, "1", `1\x00`)},
	}, judge.Check)
}

func TestCheckAnyCase(t *testing.T) {
	t.Parallel()
	runCheck(t, []checkCase{
		{"yes/no", "YES\nNO\n", "yes\nnO\n", nil},
		// cf:1110A answers even/odd, cf:1747C Alice/Bob.
		{"word answers", "even\nAlice\nFirst\n", "EVEN\nALICE\nfirst\n", nil},
		{"tokens of any kind", "yes-no YES_\n", "YES-NO yes_\n", nil},
		{"letters still matter", "YES\n", "NO\n", diff(1, "YES", "NO")},
		{"a look-alike is another letter", "YES\n", "YЕS\n", diff(1, "YES", "YЕS")}, // Cyrillic Е
		{"numbers still matter", "YES 3\n", "yes 4\n", diff(1, "3", "4")},
	}, judge.Checker{AnyCase: true}.Check)
}

func TestCheckFloat(t *testing.T) {
	t.Parallel()
	const tol = 1e-6
	float := func(tol float64) func(want, got []byte) (bool, *judge.Diff) {
		return func(want, got []byte) (bool, *judge.Diff) { return judge.CheckFloat(want, got, tol) }
	}
	runCheck(t, []checkCase{
		{"exact", "0.5\n", "0.5\n", nil},
		{"abs error", "0.333333\n", "0.3333331\n", nil},
		{"abs error at the bound", "1.000000\n", "1.000001\n", nil},
		{"abs error past it", "1.000000\n", "1.000003\n", diff(1, "1.000000", "1.000003")},
		{"rel error on a big value", "1000000000\n", "1000000500\n", nil},
		{"rel error past it", "1000000000\n", "1000002000\n", diff(1, "1000000000", "1000002000")},
		{"integer against float", "3\n", "3.0000000001\n", nil},
		{"exponent notation", "0.00000001\n", "1e-8\n", nil},
		{"leading dot and plus", "0.5 1.5\n", ".5 +1.5\n", nil},
		{"trailing dot", "2\n", "2.\n", nil},
		{"negative", "-2.5\n", "-2.5000001\n", nil},
		{"sign matters", "2.5\n", "-2.5\n", diff(1, "2.5", "-2.5")},
		{"line breaks and blanks ignored", "1 2\n3\n", "1\n2   3", nil},
		{"words are exact", "YES 0.5\n", "yes 0.5\n", diff(1, "YES", "yes")},
		{"word against number", "impossible\n", "0\n", diff(1, "impossible", "0")},
		{"nan matches nan", "nan\n", "NaN\n", nil},
		{"inf is not huge", "inf\n", "1e308\n", diff(1, "inf", "1e308")},
		{"overflow reads as inf", "inf\n", "1e400\n", nil},
		{"hex float is not a number", "0.5\n", "0x1p-1\n", diff(1, "0.5", "0x1p-1")},
		{"digit separators are not a number", "1000\n", "1_000\n", diff(1, "1000", "1_000")},
		{"comma is not a decimal point", "1.5\n", "1,5\n", diff(1, "1.5", "1,5")},
		{"missing token", "1.5 2.5\n", "1.5\n", diff(1, "2.5", "<EOF>")},
		{"extra token", "1.5\n", "1.5\n2.5\n", diff(2, "<EOF>", "2.5")},
		{"diff names the expected line", "1\n2\n3.0\n", "1 2 3.5\n", diff(3, "3.0", "3.5")},
	}, float(tol))
	runCheck(t, []checkCase{
		{"zero tol is exact", "0.5\n", "0.50\n", diff(1, "0.5", "0.50")},
	}, float(0))
	// Tol and AnyCase together, as a float problem with a YES/NO line.
	runCheck(t, []checkCase{
		{"both", "YES 0.5\n", "yes 0.5000001\n", nil},
	}, judge.Checker{Tol: tol, AnyCase: true}.Check)
}

func TestFoldYesNo(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"yes\nNo\nyEs 3\n", "YES\nNO\nYES 3\n"},
		{"yesterday nothing\n", "yesterday nothing\n"},
		{"Alice\n", "Alice\n"},
	} {
		if got := string(judge.FoldYesNo([]byte(tc.in))); got != tc.want {
			t.Errorf("FoldYesNo(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Matching outputs are checked without copying a token: a 64 MB answer
// costs no garbage.
func TestCheckAllocs(t *testing.T) {
	out := []byte(strings.Repeat("123456 7\n", 1<<16))
	if n := testing.AllocsPerRun(5, func() { judge.Check(out, out) }); n > 0 {
		t.Errorf("Check allocated %v times on matching output, want 0", n)
	}
}

// GotLine is the program's side of the mismatch, which the samples view
// marks: its own line when it breaks lines unlike the expected output,
// the line it ended on when it ended first.
func TestCheckGotLine(t *testing.T) {
	t.Parallel()
	runCheck(t, []checkCase{
		{"same lines", "1\n2\n3\n", "1\n2\n4\n", &judge.Diff{Line: 3, Want: "3", Got: "4", GotLine: 3}},
		{"one line for three", "1\n2\n3\n", "1 2 4\n", &judge.Diff{Line: 3, Want: "3", Got: "4", GotLine: 1}},
		{"nothing printed", "YES\nNO\n", "", &judge.Diff{Line: 1, Want: "YES", Got: judge.EOF, GotLine: 1}},
		{"ended early", "1\n2\n", "1\n", &judge.Diff{Line: 2, Want: "2", Got: judge.EOF, GotLine: 2}},
	}, judge.Checker{}.Check)
}
