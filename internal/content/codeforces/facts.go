package codeforces

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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

	// multiRE is how statements say any of several answers is accepted:
	// a permission ("print any", "find any", "construct any"); group 1, a condition alone
	// ("if there are multiple answers", "more than one solution"), which
	// also opens statements that then pick one (see picksOneRE); or group
	// 2, a looser phrase ("any of them", "any such"), which in a
	// description is as often about the story ("you may skip any of
	// them") and only counts in the output section.
	multiRE = regexp.MustCompile(`(?i)\b(?:(?:print|output|find|construct|restore)\s+any\b` +
		`|((?:multiple|several|many)\s+(?:possible\s+|valid\s+|correct\s+|optimal\s+)?(?:answers|solutions)|more\s+than\s+one\s+(?:answer|solution))` +
		`|(any\s+of\s+(?:them|those|these|the\s+(?:possible|optimal|valid|correct|suitable)\b)|any\s+(?:such|valid|correct|possible|suitable|optimal)\b))`)
	// orderFreeRE, in the output section, lets the answer's items come in
	// any order, which the reference fixes to one.
	orderFreeRE = regexp.MustCompile(`(?i)\bin\s+(?:any|arbitrary)\s+order\b`)
	// picksOneRE, in the rest of a condition's sentence, means the
	// statement wants one particular answer of the several: "If there are
	// multiple answers, print the lexicographically smallest", "... the
	// one with the minimum number of ...", "... print all of them".
	picksOneRE = regexp.MustCompile(`(?i)\b(?:lexicographically|minim\w*|maxim\w*|smallest|largest|least|greatest|shortest|longest|earliest|latest|first|last|the\s+one\s+with|all\s+of\s+them)\b`)
	// negatedRE ends the text before "there aren't multiple answers".
	negatedRE = regexp.MustCompile(`(?i)(?:\bnot|\bno|n't|\bnever)\s*$`)
	// caseRE is "you may print YES/NO in any case" in the phrasings at
	// rev: "in any (letter) case", "any register", "case-insensitive",
	// "upper or lower case", "either upper- or lowercase", "in arbitrary
	// case (small or large)", "you can choose the case (lower or upper)",
	// "in lower case or in upper case", "in any (upper or lower) case",
	// "letters in either case".
	caseRE = regexp.MustCompile(`(?i)\b(?:in\s+any\s+(?:letter\s+)?case|any\s+register|case[- ]insensitive` +
		`|(?:upper|lower)-?\s*(?:case\s+)?or\s+(?:in\s+)?(?:upper|lower)-?\s*case` +
		`|(?:in\s+)?(?:an\s+)?arbitrary\s+(?:letter\s+)?case` +
		`|choose\s+the\s+(?:letter\s+)?case` +
		`|in\s+any\s+\(\s*(?:upper|lower)\s+or\s+(?:upper|lower)\s*\)\s+case` +
		`|(?:letters?|characters?)\s+in\s+either\s+case` +
		`|\(\s*(?:upper|lower|small|large|capital)\s+or\s+(?:upper|lower|small|large|capital)\s*\))`)
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
	return strings.NewReplacer("$", "", `\,`, " ", `\;`, " ", `\ `, " ", "\u00a0", " ").Replace(s)
}

// multiAnswer reports whether the statement accepts any of several
// answers, which needs a checker no matter what else is true. A bare
// condition ("if there are multiple solutions") whose sentence goes on to
// pick one ("print the lexicographically smallest") does not count, and
// neither does a negated phrase ("there aren't multiple answers", "do not
// print any extra characters"); a permission ("print any of them")
// anywhere does, and so does an output that may come in any order.
func multiAnswer(output, description string) bool {
	return answersFree(output, true) || answersFree(description, false) || orderFree(output)
}

// maybeMultiAnswer is multiAnswer with the looser phrases counted in the
// description too ("Draw for him any such polygon", cf:306D). It decides
// whether a dataset checker may be only a float checker: there the cost
// of a missed answer set is a life, and of a false alarm one problem.
func maybeMultiAnswer(output, description string) bool {
	return multiAnswer(output, description) || answersFree(description, true)
}

// answersFree reports a phrase of multiRE in section that stands; loose
// says whether group 2's looser phrases count.
func answersFree(section string, loose bool) bool {
	text := plainMath(section)
	for _, m := range multiRE.FindAllStringSubmatchIndex(text, -1) {
		switch {
		case negatedRE.MatchString(text[max(0, m[0]-12):m[0]]):
		case m[2] >= 0 && picksOneRE.MatchString(restOfSentence(text, m[1])):
		case m[4] >= 0 && !loose:
		default:
			return true
		}
	}
	return false
}

// orderFree reports an output section that lets the answer's items come in
// any order ("print the indices in any order"): the reference fixes one,
// so it needs a checker too. Only the output section counts; elsewhere the
// phrase is about the input ("the numbers are given in arbitrary order").
func orderFree(output string) bool {
	return orderFreeRE.MatchString(plainMath(output))
}

// restOfSentence is text from i to the end of its sentence, at most 160
// bytes: up to a period, question mark or semicolon that ends a clause, or
// a line break.
func restOfSentence(text string, i int) string {
	s := text[i:min(len(text), i+160)]
	for j := 0; j < len(s); j++ {
		switch s[j] {
		case '\n', ';', '?', '!':
			return s[:j]
		case '.':
			if j+1 == len(s) || s[j+1] == ' ' || s[j+1] == '\n' {
				return s[:j]
			}
		}
	}
	return s
}

// caseInsensitive reports "YES/NO in any case" answers, which the judge
// folds before comparing (catalog.Problem.AnyCase).
func caseInsensitive(sections ...string) bool {
	return caseRE.MatchString(plainMath(strings.Join(sections, "\n")))
}

// russian reports a statement in Russian: more Cyrillic letters than Latin
// ones. Math in a Russian statement is Latin ($$$n$$$), so the prose has to
// outweigh it, and it always does.
func russian(s string) bool {
	var cyr, lat int
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Cyrillic, r):
			cyr++
		case r < utf8.RuneSelf && unicode.IsLetter(r):
			lat++
		}
	}
	return cyr > lat
}
