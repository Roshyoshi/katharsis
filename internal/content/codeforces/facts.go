package codeforces

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// The judge needs three facts the dataset has no column for. They are
// stated in prose, almost always in the output format, and the regexes
// below cover the phrasings in the pinned revision (surveyed 2026-10-14:
// 334 output sections mention an error bound; the OCR of old statements
// writes 10⁻⁶ as "10 - 6").

var (
	// floatRE is the "absolute or relative error" family, in either order
	// and with or without the second word.
	floatRE = regexp.MustCompile(`(?i)\b(?:absolute|relative)\s+(?:or\s+(?:absolute|relative)\s+)?error\b`)
	// precisionRE is "print one real number with precision 10 - 6".
	precisionRE = regexp.MustCompile(`(?i)\bprecision\s+(?:of\s+)?(?:at\s+least\s+)?(?:10|1e|0\.0)`)
	// digitsRE is "with at least 6 digits after the decimal point".
	digitsRE = regexp.MustCompile(`(?i)\b(\d{1,2}|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve)\s+(?:digits?|decimal\s+places|signs?)\s+after\s+the\s+(?:decimal\s+)?(?:point|dot)`)

	// The bound itself, in the forms found after one of the phrases:
	// 10^{-6}, 2 \cdot 10^{-6}, 10^-6, 1e-6, OCR's "10 - 6", 0.000001.
	powRE = regexp.MustCompile(`(?:(\d+(?:\.\d+)?)\s*(?:\\cdot|\\times|·|×|\*)\s*)?10\s*\^\s*\{?\s*[-−]\s*(\d{1,2})\b`)
	expRE = regexp.MustCompile(`\b(\d+(?:\.\d+)?)\s*[eE]\s*[-−]\s*(\d{1,2})\b`)
	ocrRE = regexp.MustCompile(`\b10\s+[-−]\s+(\d{1,2})\b`)
	decRE = regexp.MustCompile(`\b0\.(0{1,17})1\b`)

	// multiRE is how statements say any of several answers is accepted.
	multiRE = regexp.MustCompile(`(?i)\b(?:any\s+of\s+them|print\s+any|output\s+any|(?:multiple|several|many)\s+(?:possible\s+|valid\s+|correct\s+|optimal\s+)?(?:answers|solutions)|any\s+(?:such|valid|correct|possible|suitable|optimal)\b)`)
	// caseRE is "you may print YES/NO in any case".
	caseRE = regexp.MustCompile(`(?i)\b(?:in\s+any\s+(?:letter\s+)?case|any\s+register|case[- ]insensitive|(?:upper|lower)\s*case\s+or\s+(?:upper|lower)\s*case|(?:upper|lower)\s+or\s+(?:upper|lower)\s+case)\b`)
)

var numberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
}

// floatTolerance returns the allowed error for a real-valued answer, 0 for
// exact match. The bound is read from the text after the phrase; a phrase
// with no readable bound gets 1e-6, the usual contest tolerance. A
// statement that only asks for N digits after the point gets 10⁻ᴺ: an
// exact checker would fail a correct answer on its last rounded digit, and
// Codeforces checks those with a tolerance too.
func floatTolerance(sections ...string) float64 {
	text := plainMath(strings.Join(sections, "\n"))
	for _, re := range []*regexp.Regexp{floatRE, precisionRE} {
		if loc := re.FindStringIndex(text); loc != nil {
			after := text[loc[0]:min(len(text), loc[1]+200)]
			if tol, ok := bound(after); ok {
				return tol
			}
			return 1e-6
		}
	}
	if m := digitsRE.FindStringSubmatch(text); m != nil {
		d, err := strconv.Atoi(m[1])
		if err != nil {
			d = numberWords[strings.ToLower(m[1])]
		}
		if d >= 1 && d <= 18 {
			return math.Pow(10, -float64(d))
		}
	}
	return 0
}

// bound finds the first number that reads as a tolerance in s.
func bound(s string) (float64, bool) {
	best, bestAt := 0.0, len(s)+1
	try := func(at int, v float64) {
		if at < bestAt && v > 0 && v < 1 {
			best, bestAt = v, at
		}
	}
	if m := powRE.FindStringSubmatchIndex(s); m != nil {
		mant := 1.0
		if m[2] >= 0 {
			mant, _ = strconv.ParseFloat(s[m[2]:m[3]], 64)
		}
		e, _ := strconv.Atoi(s[m[4]:m[5]])
		try(m[0], mant*math.Pow(10, -float64(e)))
	}
	if m := expRE.FindStringSubmatchIndex(s); m != nil {
		mant, _ := strconv.ParseFloat(s[m[2]:m[3]], 64)
		e, _ := strconv.Atoi(s[m[4]:m[5]])
		try(m[0], mant*math.Pow(10, -float64(e)))
	}
	if m := ocrRE.FindStringSubmatchIndex(s); m != nil {
		e, _ := strconv.Atoi(s[m[2]:m[3]])
		try(m[0], math.Pow(10, -float64(e)))
	}
	if m := decRE.FindStringSubmatchIndex(s); m != nil {
		try(m[0], math.Pow(10, -float64(m[3]-m[2]+1)))
	}
	return best, bestAt <= len(s)
}

// plainMath drops math delimiters and spacing commands so the regexes see
// "10^{-6}" whether the statement wrote $$$10^{-6}$$$ or 10^{-6}.
func plainMath(s string) string {
	return strings.NewReplacer("$", "", `\,`, " ", `\;`, " ", `\ `, " ", " ", " ").Replace(s)
}

// multiAnswer reports whether the statement accepts any of several
// answers, which needs a checker no matter what else is true.
func multiAnswer(sections ...string) bool {
	return multiRE.MatchString(plainMath(strings.Join(sections, "\n")))
}

// caseInsensitive reports "YES/NO in any case" answers. The judge compares
// exactly, so a "Yes" against an expected "YES" fails; the statement gets
// a warning (see statementMarkdown) until the judge learns the rule.
func caseInsensitive(sections ...string) bool {
	return caseRE.MatchString(plainMath(strings.Join(sections, "\n")))
}
