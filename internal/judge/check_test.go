package judge_test

import (
	"strings"
	"testing"

	"katharsis/internal/judge"
)

func TestCheck(t *testing.T) {
	t.Parallel()
	long := func(c byte) string { return strings.Repeat(string(c), 200) }
	tests := []struct {
		name      string
		want, got string
		diff      *judge.Diff // nil means accepted
	}{
		{"exact", "1\n2\n", "1\n2\n", nil},
		{"missing final newline", "1\n2\n", "1\n2", nil},
		{"trailing spaces and tabs", "1 2\n3\n", "1 2  \n3\t\n", nil},
		{"crlf", "1\n2\n", "1\r\n2\r\n", nil},
		{"crlf in expected", "1\r\n2\r\n", "1\n2\n", nil},
		{"trailing blank lines", "1\n", "1\n\n\n", nil},
		{"trailing whitespace-only lines", "1\n", "1\n  \n\t\r\n", nil},
		{"both empty", "", "", nil},
		{"empty vs blank lines", "", "\n\n", nil},

		{"line 1 differs", "3\n", "4\n", &judge.Diff{Line: 1, Want: "3", Got: "4"}},
		{"line 2 differs", "1\n2\n3\n", "1\n5\n3\n", &judge.Diff{Line: 2, Want: "2", Got: "5"}},
		{"leading space matters", "1\n", " 1\n", &judge.Diff{Line: 1, Want: "1", Got: " 1"}},
		{"inner spaces matter", "1 2\n", "1  2\n", &judge.Diff{Line: 1, Want: "1 2", Got: "1  2"}},
		{"blank line in the middle", "1\n\n2\n", "1\n2\n", &judge.Diff{Line: 2, Want: "", Got: "2"}},
		{"got shorter", "1\n2\n", "1\n", &judge.Diff{Line: 2, Want: "2", Got: "<EOF>"}},
		{"want shorter", "1\n", "1\n2\n", &judge.Diff{Line: 2, Want: "<EOF>", Got: "2"}},
		{"no output", "7\n", "", &judge.Diff{Line: 1, Want: "7", Got: "<EOF>"}},
		{"long lines clipped", long('a'), long('b'),
			&judge.Diff{Line: 1, Want: long('a')[:120] + "…", Got: long('b')[:120] + "…"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, d := judge.Check([]byte(tc.want), []byte(tc.got))
			if tc.diff == nil {
				if !ok || d != nil {
					t.Fatalf("Check = %v, %+v; want accepted", ok, d)
				}
				return
			}
			if ok || d == nil {
				t.Fatalf("Check = %v, %+v; want %+v", ok, d, *tc.diff)
			}
			if *d != *tc.diff {
				t.Errorf("diff = %+v, want %+v", *d, *tc.diff)
			}
		})
	}
}

func TestCheckFloat(t *testing.T) {
	t.Parallel()
	d := func(line int, want, got string) *judge.Diff { return &judge.Diff{Line: line, Want: want, Got: got} }
	const tol = 1e-6
	tests := []struct {
		name      string
		want, got string
		tol       float64
		diff      *judge.Diff // nil means accepted
	}{
		{"exact", "0.5\n", "0.5\n", tol, nil},
		{"abs error", "0.333333\n", "0.3333331\n", tol, nil},
		{"abs error at the bound", "1.000000\n", "1.000001\n", tol, nil},
		{"abs error past it", "1.000000\n", "1.000003\n", tol, d(1, "1.000000", "1.000003")},
		{"rel error on a big value", "1000000000\n", "1000000500\n", tol, nil},
		{"rel error past it", "1000000000\n", "1000002000\n", tol, d(1, "1000000000", "1000002000")},
		{"integer against float", "3\n", "3.0000000001\n", tol, nil},
		{"exponent notation", "0.00000001\n", "1e-8\n", tol, nil},
		{"negative", "-2.5\n", "-2.5000001\n", tol, nil},
		{"sign matters", "2.5\n", "-2.5\n", tol, d(1, "2.5", "-2.5")},
		{"line breaks and blanks ignored", "1 2\n3\n", "1\n2   3", tol, nil},
		{"words are exact", "YES 0.5\n", "yes 0.5\n", tol, d(1, "YES", "yes")},
		{"word against number", "impossible\n", "0\n", tol, d(1, "impossible", "0")},
		{"nan matches nan", "nan\n", "NaN\n", tol, nil},
		{"inf is not huge", "inf\n", "1e308\n", tol, d(1, "inf", "1e308")},
		{"missing token", "1.5 2.5\n", "1.5\n", tol, d(1, "2.5", "<EOF>")},
		{"extra token", "1.5\n", "1.5\n2.5\n", tol, d(2, "<EOF>", "2.5")},
		{"diff names the expected line", "1\n2\n3.0\n", "1 2 3.5\n", tol, d(3, "3.0", "3.5")},
		{"zero tol is exact", "0.5\n", "0.50\n", 0, d(1, "0.5", "0.50")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, d := judge.CheckFloat([]byte(tc.want), []byte(tc.got), tc.tol)
			if tc.diff == nil {
				if !ok || d != nil {
					t.Fatalf("CheckFloat = %v, %+v; want accepted", ok, d)
				}
				return
			}
			if ok || d == nil {
				t.Fatalf("CheckFloat = %v, %+v; want %+v", ok, d, *tc.diff)
			}
			if *d != *tc.diff {
				t.Errorf("diff = %+v, want %+v", *d, *tc.diff)
			}
		})
	}
}

func TestBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0B"},
		{312, "312B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1 << 20, "1M"},
		{48 << 20, "48M"},
		{1 << 30, "1.0G"},
		{3 << 29, "1.5G"},
	}
	for _, tc := range tests {
		if got := judge.Bytes(tc.n); got != tc.want {
			t.Errorf("Bytes(%d) = %s, want %s", tc.n, got, tc.want)
		}
	}
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
