//go:build qa

package judgeqa

import (
	"fmt"
	"testing"

	"katharsis/internal/judge"
)

// TestQACheckFloat: each case says what testlib's rcmp-style checkers
// (doubleCompare: |d| <= tol or |d| <= tol*|want|, NaN/Inf handled) accept.
func TestQACheckFloat(t *testing.T) {
	const tol = 1e-6
	for _, tc := range []struct {
		name, want, got string
		tol             float64
		ok              bool
		why             string
	}{
		{"abs below 1", "0.5", "0.5000009", tol, true, "abs"},
		{"abs below 1 past", "0.5", "0.500002", tol, false, ""},
		{"rel above 1", "123456.0", "123456.1", tol, true, "rel 8e-7"},
		{"rel above 1 past", "123456.0", "123456.2", tol, false, ""},
		{"tiny want uses abs", "1e-9", "9e-7", tol, true, "abs 9e-7 <= 1e-6"},
		{"huge values rel", "1e300", "1.0000005e300", tol, true, ""},
		{"huge vs inf", "1e308", "inf", tol, false, ""},
		{"exp vs plain", "1e5", "100000", tol, true, ""},
		{"plain vs exp", "100000", "1e5", tol, true, ""},
		{"-0 vs 0", "0", "-0", tol, true, ""},
		{"-0.000000 vs 0.000000", "0.000000", "-0.000000", tol, true, ""},
		{"nan vs nan", "nan", "nan", tol, true, ""},
		{"nan vs number", "1", "nan", tol, false, ""},
		{"number vs nan", "nan", "1", tol, false, ""},
		{"inf vs inf", "inf", "Inf", tol, true, ""},
		{"-inf vs inf", "-inf", "inf", tol, false, ""},
		{"Infinity spelled out", "inf", "Infinity", tol, true, "Go parses it; testlib does not read it as a double"},
		{"hex float accepted", "0.5", "0x1p-1", tol, false, "testlib reads decimal only; a hex float is a PE"},
		{"leading plus", "1.5", "+1.5", tol, true, ""},
		{"comma decimal", "1.5", "1,5", tol, false, ""},
		{"trailing dot", "2", "2.", tol, true, ""},
		{"leading dot", "0.5", ".5", tol, true, ""},
		{"underscore digits", "1000", "1_000", tol, false, "not a number anywhere"},
		{"int64 max off by one", "9223372036854775807", "9223372036854775806", tol, true, "documented: integers are numbers under tol"},
		{"count off by one under loose tol", "1000000", "1000001", tol, true, "documented"},
		{"words exact", "YES 0.5", "yes 0.5", tol, false, ""},
		{"line structure ignored", "1 2\n3", "1\n2 3", tol, true, ""},
		{"negative tol is exact", "0.5", "0.50", -1, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, d := judge.CheckFloat([]byte(tc.want+"\n"), []byte(tc.got+"\n"), tc.tol)
			note("checkfloat/"+tc.name, "", fmt.Sprintf("ok=%v diff=%+v", ok, d))
			if ok != tc.ok {
				t.Errorf("CheckFloat(%q, %q, %g) = %v, want %v %s", tc.want, tc.got, tc.tol, ok, tc.ok, tc.why)
			}
		})
	}
}

// TestQAAnyCase: Run applies FoldYesNo to both sides, then Check. Each case
// says what a Codeforces "any case" checker accepts.
func TestQAAnyCase(t *testing.T) {
	for _, tc := range []struct {
		name, want, got string
		ok              bool
		why             string
	}{
		{"YES/yes", "YES\n", "yes\n", true, ""},
		{"Yes/YES", "Yes\n", "YES\n", true, ""},
		{"yEs/NO", "YES\n", "NO\n", false, ""},
		{"yesterday", "yesterday\n", "YESTERDAY\n", false, "only yes/no fold"},
		{"No inside a sentence", "No solution\n", "NO solution\n", true, ""},
		{"Alice in any case", "Alice\n", "ALICE\n", true, "CF 'any case' problems with Alice/Bob answers accept ALICE"},
		{"First/Second in any case", "First\n", "first\n", true, "same family"},
		{"yes-no hyphen", "yes-no\n", "YES-NO\n", true, ""},
		{"YES_ underscore", "YES_\n", "yes_\n", false, "\\b treats _ as a word char"},
		{"Cyrillic look-alike", "YES\n", "YЕS\n", false, "Е is U+0415"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, d := judge.Check(judge.FoldYesNo([]byte(tc.want)), judge.FoldYesNo([]byte(tc.got)))
			note("anycase/"+tc.name, "", fmt.Sprintf("ok=%v diff=%+v", ok, d))
			if ok != tc.ok {
				t.Errorf("AnyCase(%q, %q) = %v, want %v %s", tc.want, tc.got, ok, tc.ok, tc.why)
			}
		})
	}
}

func TestQACheckExact(t *testing.T) {
	for _, tc := range []struct {
		name, want, got string
		ok              bool
		line            int
		why             string
	}{
		{"crlf both", "1\r\n2\r\n", "1\r\n2\r\n", true, 0, ""},
		{"crlf got only", "1\n2\n", "1\r\n2\r\n", true, 0, ""},
		{"lone CR line ends", "1\n2\n", "1\r2\r", true, 0, "token checkers (wcmp) treat CR as a blank"},
		{"CRLF blank line in middle", "1\n\n2\n", "1\r\n\r\n2\r\n", true, 0, ""},
		{"trailing tab-space mix", "1\n", "1 \t \r\n", true, 0, ""},
		{"form feed at end", "1\n", "1\f\n", false, 1, "testlib isBlanks is LF CR SPACE TAB only"},
		{"NUL in output", "1\n", "1\x00\n", false, 1, ""},
		{"first differing line is 3", "a\nb\nc\nd\n", "a\nb\nX\nY\n", false, 3, ""},
		{"utf8 clip", "x\n", string(make([]byte, 0)) + repeat("é", 100) + "\n", false, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, d := judge.Check([]byte(tc.want), []byte(tc.got))
			note("check/"+tc.name, "", fmt.Sprintf("ok=%v diff=%+v", ok, d))
			if ok != tc.ok {
				t.Errorf("Check(%q, %q) = %v, want %v %s", tc.want, tc.got, ok, tc.ok, tc.why)
			}
			if !ok && tc.line > 0 && (d == nil || d.Line != tc.line) {
				t.Errorf("diff %+v, want line %d", d, tc.line)
			}
		})
	}
}

// The Diff is shown verbatim in the HUD; it must be valid UTF-8 and free
// of terminal control bytes.
func TestQADiffPrintable(t *testing.T) {
	for _, tc := range []struct{ name, got string }{
		{"clip mid-rune", repeat("é", 100)},
		{"escape sequence", "\x1b[2J\x1b[31mred"},
		{"carriage return", "123\rabc"},
		{"garbage bytes", "\xff\xfe\x01\x02"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, d := judge.Check([]byte("x\n"), []byte(tc.got+"\n"))
			if d == nil {
				t.Fatal("no diff")
			}
			bad := ""
			for i, r := range d.Got {
				if r == 0xFFFD || (r < 0x20 && r != '\t') || r == 0x7f {
					bad = fmt.Sprintf("byte %d: %U", i, r)
					break
				}
			}
			note("diff_printable/"+tc.name, "", fmt.Sprintf("Got=%q bad=%s", d.Got, bad))
			if bad != "" {
				t.Errorf("Diff.Got %q has a non-printable at %s", d.Got, bad)
			}
		})
	}
}

func repeat(s string, n int) string {
	out := ""
	for range n {
		out += s
	}
	return out
}
