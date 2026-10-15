package judge

import (
	"bytes"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Diff locates the first mismatch between expected and actual output.
type Diff struct {
	Line int    // 1-based
	Want string // "<EOF>" when the expected output ended first
	Got  string // "<EOF>" when the program's output ended first
}

const eof = "<EOF>"

// Check compares output line by line, ignoring trailing whitespace on each
// line and trailing blank lines, which is how USACO and most judges treat
// exact-match problems.
func Check(want, got []byte) (bool, *Diff) {
	w, g := lines(want), lines(got)
	for i := range max(len(w), len(g)) {
		switch {
		case i >= len(g):
			return false, &Diff{Line: i + 1, Want: clip(w[i]), Got: eof}
		case i >= len(w):
			return false, &Diff{Line: i + 1, Want: eof, Got: clip(g[i])}
		case w[i] != g[i]:
			return false, &Diff{Line: i + 1, Want: clip(w[i]), Got: clip(g[i])}
		}
	}
	return true, nil
}

// CheckFloat is the checker for problems whose statement accepts answers
// within an absolute or relative error tol. With tol <= 0 it is Check.
//
// Output is compared token by token, so line breaks and runs of blanks do
// not matter, as with testlib's rcmp checkers. Two tokens that both parse as
// numbers match when |got - want| <= tol * max(1, |want|), which is "within
// tol absolutely or relatively", plus testlib's 1e-15 for rounding. Integers
// are numbers too, so under a loose tol a large integer may be slightly off
// and still pass; judges using these checkers behave the same. Any other
// token must match exactly.
//
// Diff.Line is the line of the expected token (of the program's token when
// the expected output ran out); Want and Got are the two tokens.
func CheckFloat(want, got []byte, tol float64) (bool, *Diff) {
	if tol <= 0 {
		return Check(want, got)
	}
	w, g := tokens(want), tokens(got)
	for i := range max(len(w), len(g)) {
		switch {
		case i >= len(g):
			return false, &Diff{Line: w[i].line, Want: clip(w[i].s), Got: eof}
		case i >= len(w):
			return false, &Diff{Line: g[i].line, Want: eof, Got: clip(g[i].s)}
		case !near(w[i].s, g[i].s, tol):
			return false, &Diff{Line: w[i].line, Want: clip(w[i].s), Got: clip(g[i].s)}
		}
	}
	return true, nil
}

type token struct {
	s    string
	line int // 1-based
}

func tokens(b []byte) []token {
	var ts []token
	for i, l := range strings.Split(string(b), "\n") {
		for _, f := range strings.Fields(l) {
			ts = append(ts, token{f, i + 1})
		}
	}
	return ts
}

func near(want, got string, tol float64) bool {
	if want == got {
		return true
	}
	w, errW := strconv.ParseFloat(want, 64)
	g, errG := strconv.ParseFloat(got, 64)
	switch {
	case errW != nil || errG != nil:
		return false
	case math.IsNaN(w) || math.IsNaN(g):
		return math.IsNaN(w) && math.IsNaN(g)
	case math.IsInf(w, 0) || math.IsInf(g, 0):
		return w == g
	}
	return math.Abs(g-w) <= tol*math.Max(1, math.Abs(w))+1e-15
}

func lines(b []byte) []string {
	ls := strings.Split(string(b), "\n")
	for i, l := range ls {
		ls[i] = strings.TrimRight(l, " \t\r")
	}
	for len(ls) > 0 && ls[len(ls)-1] == "" {
		ls = ls[:len(ls)-1]
	}
	return ls
}

func clip(s string) string {
	const n = 120
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

var yesNo = regexp.MustCompile(`(?i)\b(yes|no)\b`)

// FoldYesNo upper-cases the words yes and no, for problems that accept
// them in any case ("YES", "yEs"). Only those words change, so the rest of
// an answer is still compared exactly.
func FoldYesNo(b []byte) []byte {
	return yesNo.ReplaceAllFunc(b, bytes.ToUpper)
}
