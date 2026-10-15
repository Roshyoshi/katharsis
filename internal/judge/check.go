package judge

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"unicode/utf8"
)

// Diff locates the first mismatch between expected and actual output.
type Diff struct {
	Line int    // 1-based
	Want string // "<EOF>" when the expected output ended first
	Got  string // "<EOF>" when the program's output ended first
}

const eof = "<EOF>"

// Checker compares a program's output with the expected output the way
// testlib's standard checkers do: as two sequences of tokens separated by
// blanks (space, tab, CR, LF; testlib's isBlanks). Line breaks, runs of
// blanks, leading blanks and blank lines do not matter; every token does.
//
// Tokens are the default for every source, not just Codeforces. Codeforces
// judges exact-answer problems with wcmp, which is this comparison, so " 4"
// for "4", or one number per line for numbers on one line, is accepted
// there. USACO's grader is lenient about whitespace too (per the content QA
// brief; not checked against its source). And the costs are lopsided: a
// correct answer judged WA costs the player a life, while a whitespace
// layout a strict judge would reject costs nothing a player would notice.
// The price is the rare problem whose whitespace is part of the answer
// (ASCII art that must keep its indentation), which this accepts more
// often than its real judge.
type Checker struct {
	// Tol > 0 accepts a token that parses as a decimal number within Tol
	// of the expected one, absolutely or relatively: |got - want| <= Tol *
	// max(1, |want|), plus testlib's 1e-15 for rounding (rcmp). Integers
	// are numbers too, so under a loose Tol a large integer may be
	// slightly off and still pass; judges using these checkers behave the
	// same. Tokens that are not decimal numbers ("0x1p-1", "1_000") must
	// match exactly.
	Tol float64
	// AnyCase compares every token regardless of letter case, for
	// statements that accept answers in any case. Codeforces says so for
	// YES/NO and for word answers (Alice/Bob, First/Second, even/odd)
	// alike, and its checkers fold the whole word. Folding every token,
	// not only the words the statement names, can only accept more.
	AnyCase bool
}

// Check reports whether got matches want. On a mismatch, Diff.Line is the
// line of the expected token (of the program's token when the expected
// output ran out) and Want and Got are the two tokens, clipped and with
// non-printable bytes escaped, so the HUD can show them verbatim.
func (c Checker) Check(want, got []byte) (bool, *Diff) {
	w, g := tokenizer{b: want, line: 1}, tokenizer{b: got, line: 1}
	for {
		wt, wl, wok := w.next()
		gt, gl, gok := g.next()
		switch {
		case !wok && !gok:
			return true, nil
		case !gok:
			return false, &Diff{Line: wl, Want: show(wt), Got: eof}
		case !wok:
			return false, &Diff{Line: gl, Want: eof, Got: show(gt)}
		case !c.match(wt, gt):
			return false, &Diff{Line: wl, Want: show(wt), Got: show(gt)}
		}
	}
}

func (c Checker) match(want, got []byte) bool {
	switch {
	case bytes.Equal(want, got):
		return true
	case c.AnyCase && bytes.EqualFold(want, got):
		return true
	}
	return c.Tol > 0 && near(want, got, c.Tol)
}

// Check compares output token by token, exactly; see Checker.
func Check(want, got []byte) (bool, *Diff) {
	return Checker{}.Check(want, got)
}

// CheckFloat is the checker for problems whose statement accepts answers
// within an absolute or relative error tol; see Checker.Tol. With tol <= 0
// it is Check.
func CheckFloat(want, got []byte, tol float64) (bool, *Diff) {
	return Checker{Tol: tol}.Check(want, got)
}

// tokenizer walks b's tokens without copying them, so checking a 64 MB
// output allocates nothing.
type tokenizer struct {
	b    []byte
	i    int
	line int // of b[i]
}

func (t *tokenizer) next() (tok []byte, line int, ok bool) {
	for t.i < len(t.b) && blank(t.b[t.i]) {
		if t.b[t.i] == '\n' {
			t.line++
		}
		t.i++
	}
	if t.i == len(t.b) {
		return nil, t.line, false
	}
	start := t.i
	for t.i < len(t.b) && !blank(t.b[t.i]) {
		t.i++
	}
	return t.b[start:t.i], t.line, true
}

// blank is testlib's isBlanks. Form feed and vertical tab are not blanks
// there, so they are not here.
func blank(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func near(want, got []byte, tol float64) bool {
	w, okW := decimal(want)
	g, okG := decimal(got)
	switch {
	case !okW || !okG:
		return false
	case math.IsNaN(w) || math.IsNaN(g):
		return math.IsNaN(w) && math.IsNaN(g)
	case math.IsInf(w, 0) || math.IsInf(g, 0):
		return w == g
	}
	return math.Abs(g-w) <= tol*math.Max(1, math.Abs(w))+1e-15
}

var (
	// decimalRE is a decimal number as scanf reads one: "1", "-2.5",
	// ".5", "2.", "1e-8", "+3". strconv alone would also take hex floats
	// and digit separators, which no judge reads as numbers.
	decimalRE = regexp.MustCompile(`^[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?$`)
	// specialRE is what C++ streams print for infinities and NaNs.
	specialRE = regexp.MustCompile(`(?i)^[+-]?(?:inf|infinity|nan)$`)
)

// decimal parses a token as a number.
func decimal(b []byte) (float64, bool) {
	if len(b) > 400 {
		return 0, false
	}
	s := string(b)
	if !decimalRE.MatchString(s) && !specialRE.MatchString(s) {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	// A range error comes with ±Inf or 0, which is what the token means.
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	return f, true
}

// show makes a token safe to print in the HUD: control bytes, C1
// controls and invalid UTF-8 become \xNN, and a long token is cut after
// 120 bytes on a rune boundary. A program can print anything, escape
// sequences that would repaint the player's terminal included.
func show(tok []byte) string {
	const n = 120
	var b bytes.Buffer
	for i := 0; i < len(tok); {
		if b.Len() >= n {
			b.WriteString("…")
			break
		}
		r, size := utf8.DecodeRune(tok[i:])
		if r == utf8.RuneError && size <= 1 || r < 0x20 || r == 0x7f || r >= 0x80 && r < 0xa0 {
			for _, c := range tok[i : i+size] {
				fmt.Fprintf(&b, `\x%02x`, c)
			}
		} else {
			b.Write(tok[i : i+size])
		}
		i += size
	}
	return b.String()
}

var yesNo = regexp.MustCompile(`(?i)\b(yes|no)\b`)

// FoldYesNo upper-cases the words yes and no, for problems that accept
// them in any case ("YES", "yEs"). Only those words change, so the rest of
// an answer is still compared exactly.
//
// Deprecated: Run no longer uses it; Checker.AnyCase folds every token,
// word answers such as "Alice" included.
func FoldYesNo(b []byte) []byte {
	return yesNo.ReplaceAllFunc(b, bytes.ToUpper)
}
