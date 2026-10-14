package usaco

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// USACO's defaults, stated on its FAQ: 2 s per test for C/C++ (Java and
// Python get twice that) and 256 MB.
const (
	defaultTime = 2 * time.Second
	defaultMem  = 256 << 20
)

var (
	// "INPUT FORMAT (file paint.in):" since 2015; "PROBLEM NAME: crazy" and
	// "SAMPLE INPUT (file crazy.in):" before, and once (cpid 532) just
	// "INPUT FORMAT: (hopscotch.in)". Stdin statements say "input
	// arrives from the terminal / stdin" and match neither.
	fileIORE = regexp.MustCompile(`\((?:file )?([A-Za-z0-9_]+)\.in\)|PROBLEM NAME: ([A-Za-z0-9_]+)`)

	timeTopicRE = regexp.MustCompile(`(?i)\btime (?:and memory )?limits?\b`)
	memTopicRE  = regexp.MustCompile(`(?i)\bmemory limits?\b`)
	secondsRE   = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?) ?(?:s|secs?|seconds?)\b`)
	megabytesRE = regexp.MustCompile(`(?i)\b(\d+) ?(MB|MiB|GB|GiB)\b`)
	langRE      = regexp.MustCompile(`(?i)\b(?:java|python)\b`)

	// Limits stated relative to the default: "twice the default", "1.5x the
	// default", "50 percent higher than the default", "2.5 times".
	factorRE = regexp.MustCompile(`(?i)\b(?:(\d+(?:\.\d+)?) ?(?:x|times)\b|(twice|double|thrice)\b|` +
		`(two|three|four|five) times\b|(\d+) ?(?:%|percent) (?:higher|larger|more|greater))`)

	// "This is an interactive problem", "You can interactively query",
	// "graded by an interactive grader". "interaction(s)" is common prose
	// (cows interact) and deliberately not matched.
	interactiveRE = regexp.MustCompile(`(?i)\binteractive(?:ly)?\b`)

	// Statements that accept any of several answers: "Any valid output will
	// be accepted", "any one will be accepted", "If there are multiple
	// solutions, output any.", "your program may output any of them", "the
	// array [4, 3, 2, 1] would also be accepted". Phrases that then pick
	// one answer ("output the lexicographically smallest") do not match,
	// and neither does "make sure not to output any extraneous spaces".
	checkerRE = regexp.MustCompile(`(?i)\bany\b[^.]{0,60}?\b(?:will|would|is|are) (?:also )?(?:be )?accepted|` +
		`\b(?:output|print) any(?: (?:one|of|them|such|valid|correct|possible|answer|solution))*(?:[.;,)]|$)|` +
		`\b(?:will|would) also be accepted`)

	floatRE    = regexp.MustCompile(`(?i)\b(?:absolute|relative) (?:or (?:absolute|relative) )?error\b`)
	floatExpRE = regexp.MustCompile(`10\^\{?-(\d+)\}?|1e-(\d+)`)
)

// scanFacts reads the prose for I/O style, limits and the answer format.
// text is the plain text of the statement after math rewriting.
func (st *statement) scanFacts(text string) {
	flat := squash(text)
	if m := fileIORE.FindStringSubmatch(flat); m != nil {
		st.FileIO = m[1] + m[2]
	}
	st.TimeLimit, st.MemLimit = limits(flat)
	st.Interactive = interactiveRE.MatchString(flat)
	st.NeedsChecker = checkerRE.MatchString(flat)
	st.FloatTolerance = floatTolerance(flat)
}

// limits finds non-default time and memory limits. USACO words these
// notes a dozen ways (see the golden tests), so instead of one regex per
// wording it looks at each sentence that mentions a limit: an explicit
// number with a unit wins ("4s", "512MB"), else a factor of the default
// ("twice the default"). A number stated for Java or Python only ("The time
// limit for this problem in Python is 15s.") is skipped, and so is a
// sentence with no number or factor ("will not run within the time
// limit").
func limits(flat string) (t time.Duration, mem int64) {
	for _, s := range sentences(flat) {
		if loc := timeTopicRE.FindStringIndex(s); loc != nil && t == 0 {
			seg := s[loc[0]:]
			if m := secondsRE.FindStringSubmatchIndex(seg); m != nil {
				if !langRE.MatchString(seg[:m[0]]) {
					secs, _ := strconv.ParseFloat(seg[m[2]:m[3]], 64)
					if secs > 0 && secs <= 60 {
						t = time.Duration(secs * float64(time.Second))
					}
				}
			} else if f := factor(beforeMemory(seg)); f > 0 {
				t = time.Duration(f * float64(defaultTime))
			}
		}
		if loc := memTopicRE.FindStringIndex(s); loc != nil && mem == 0 {
			seg := s[loc[0]:]
			if m := megabytesRE.FindStringSubmatch(s); m != nil {
				n, _ := strconv.ParseInt(m[1], 10, 64)
				unit := int64(1 << 20)
				if strings.HasPrefix(strings.ToUpper(m[2]), "G") {
					unit = 1 << 30
				}
				mem = n * unit
			} else if f := factor(seg); f > 0 {
				mem = int64(f * defaultMem)
			}
		}
	}
	return t, mem
}

// beforeMemory cuts a time-limit clause where the memory clause starts, so
// "time limit is 50 percent higher than the default, and the memory limit
// is twice the default" reads 1.5, not 2.
func beforeMemory(seg string) string {
	if loc := memTopicRE.FindStringIndex(seg); loc != nil && loc[0] > 0 {
		return seg[:loc[0]]
	}
	return seg
}

func factor(s string) float64 {
	m := factorRE.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	switch {
	case m[1] != "":
		f, _ := strconv.ParseFloat(m[1], 64)
		return f
	case m[2] != "":
		return map[string]float64{"twice": 2, "double": 2, "thrice": 3}[strings.ToLower(m[2])]
	case m[3] != "":
		return map[string]float64{"two": 2, "three": 3, "four": 4, "five": 5}[strings.ToLower(m[3])]
	case m[4] != "":
		p, _ := strconv.ParseFloat(m[4], 64)
		return 1 + p/100
	}
	return 0
}

// sentences splits on ". " and the "**" USACO wraps its notes in. Decimal
// points ("1.5x", "2.5s") have no space after them and survive.
func sentences(flat string) []string {
	flat = strings.ReplaceAll(flat, "**", ". ")
	var out []string
	for s := range strings.SplitSeq(flat, ". ") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// floatTolerance returns the allowed error for a real-valued answer. USACO
// nearly always asks for a rounded or truncated integer instead; 2022
// February Gold P2 (cpid 1210) is the one statement in the index that
// states "at most 10⁻⁶ absolute or relative error".
func floatTolerance(flat string) float64 {
	loc := floatRE.FindStringIndex(flat)
	if loc == nil {
		return 0
	}
	win := flat[max(0, loc[0]-120):min(len(flat), loc[1]+120)]
	for _, g := range floatExpRE.FindAllStringSubmatch(win, -1) {
		exp := g[1] + g[2]
		if n, err := strconv.Atoi(exp); err == nil && n > 0 && n < 20 {
			return math.Pow10(-n)
		}
	}
	return 1e-6 // stated but not parsed: the usual contest tolerance
}
