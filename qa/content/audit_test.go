//go:build qa

package content

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/judge"
	"katharsis/internal/screens/reader"
)

// auditIDs spans the eras and shapes the two sources serve. The comment
// on each line is why it is in the set.
var auditIDs = []catalog.ID{
	// USACO Nov-Dec 2011: I.N/O.N zip names.
	"usaco:84", "usaco:94", "usaco:91", "usaco:97",
	// USACO 2012-2015: plain-text statements, file I/O.
	"usaco:121", "usaco:205", "usaco:340", "usaco:346", "usaco:358",
	"usaco:412", "usaco:487", "usaco:508", "usaco:526", "usaco:545",
	// USACO 2015-2020: HTML statements, file I/O.
	"usaco:567", "usaco:568", "usaco:615", "usaco:639", "usaco:649",
	"usaco:663", "usaco:687", "usaco:711", "usaco:735", "usaco:759",
	"usaco:807", "usaco:855", "usaco:891", "usaco:963", "usaco:987",
	// USACO Dec 2020+: stdin, some non-default limits, the float one.
	"usaco:1059", "usaco:1083", "usaco:1089", "usaco:1131", "usaco:1155",
	"usaco:1179", "usaco:1210", "usaco:1211", "usaco:1251", "usaco:1279",
	"usaco:1299", "usaco:1325", "usaco:1347", "usaco:1395", "usaco:1398",
	"usaco:1443", "usaco:1467", "usaco:1515",

	// Codeforces 800-1100: complete official tests, generated tests,
	// AnyCase, float, multi-answer, Russian-only.
	"cf:4A", "cf:1722B", "cf:1986A", "cf:2007A", "cf:158A", "cf:71A",
	"cf:266A", "cf:236A", "cf:200B", "cf:1642A", "cf:1742A", "cf:1873A",
	"cf:233A", "cf:1791A", "cf:1971A", "cf:1328A", "cf:1004A", "cf:160A",
	"cf:127A", "cf:1A", "cf:131A", "cf:1293B", "cf:219A", "cf:1199B",
	"cf:118A", "cf:1899C", "cf:646A", "cf:832A", "cf:1110A",
	// Codeforces 1200-2400.
	"cf:1419B", "cf:1872D", "cf:9C", "cf:1305B", "cf:1350B", "cf:1443C",
	"cf:42A", "cf:1769B2", "cf:1395C", "cf:1840E", "cf:11B", "cf:1442A",
	"cf:1781D", "cf:3C", "cf:16C", "cf:20C", "cf:1344B", "cf:20B",
	"cf:10C", "cf:1333F", "cf:1695D1", "cf:28C", "cf:1340C", "cf:1469E",
	"cf:26D", "cf:32E",
}

type issue struct {
	sev   string // "HIGH", "MED", "LOW", "INFO"
	check string
	what  string
}

// TestAudit fetches every auditID and checks its statement, samples and
// tests. It fails on HIGH and MED issues and logs a row per problem.
func TestAudit(t *testing.T) {
	u := union(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	var rows []string
	for _, id := range auditIDs {
		if !only(id) {
			continue
		}
		if _, ok := u.Get(id); !ok {
			t.Errorf("%s: not in the catalog", id)
			continue
		}
		start := time.Now()
		a, err := u.Fetch(ctx, id)
		if err != nil {
			t.Errorf("%s: fetch: %v", id, err)
			rows = append(rows, fmt.Sprintf("%-12s FETCH ERROR %v", id, err))
			continue
		}
		p, _ := u.Get(id) // a fetch can refine the entry
		iss := audit(t, p, a)
		bad := 0
		for _, is := range iss {
			if is.sev == "HIGH" || is.sev == "MED" {
				bad++
				t.Errorf("%s [%s] %s: %s", id, is.sev, is.check, is.what)
			} else {
				t.Logf("%s [%s] %s: %s", id, is.sev, is.check, is.what)
			}
		}
		rows = append(rows, fmt.Sprintf("%-12s r=%-4d %-8s T=%-5s M=%-4d io=%-10q tol=%-6g any=%-5v samples=%-2d tests=%-3d issues=%d (%s)",
			id, p.Rating, p.Division, strconv.FormatFloat(p.TimeLimit.Seconds(), 'f', -1, 64), p.MemLimit>>20,
			p.FileIO, p.FloatTolerance, p.AnyCase, len(a.Samples), len(a.Tests), bad, time.Since(start).Round(time.Millisecond)))
	}
	t.Log("\n" + strings.Join(rows, "\n"))
}

var (
	leftoverCmdRE = regexp.MustCompile(`\\[A-Za-z]{2,}`)
	limitLineRE   = regexp.MustCompile(`(?i)[^.\n]*(?:time limit|memory limit)[^.\n]*`)
	cfLimitRE     = regexp.MustCompile(`\*Time limit: ([0-9.]+) s · Memory limit: (\d+) MB\*`)
	cyrillicRE    = regexp.MustCompile(`[А-Яа-яЁё]{4,}`)
	codeSpanRE    = regexp.MustCompile("`[^`\n]*`")
)

func audit(t *testing.T, p catalog.Problem, a catalog.Assets) []issue {
	var out []issue
	add := func(sev, check, f string, args ...any) {
		out = append(out, issue{sev, check, fmt.Sprintf(f, args...)})
	}

	// ---- statement -------------------------------------------------------
	raw, err := os.ReadFile(a.Statement)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		add("HIGH", "statement", "missing or empty: %v", err)
		return out
	}
	md := string(raw)
	conv := reader.TeXToUnicode(md)
	if strings.Contains(md, "DCVERB") || strings.Contains(conv, "DCVERB") {
		add("HIGH", "placeholder", "verbatim placeholder left in statement")
	}
	if strings.Contains(md, "\r") {
		add("LOW", "statement-cr", "CR in statement.md")
	}
	proseRaw, open := prose(md)
	if open {
		add("HIGH", "fences", "unbalanced code fences in statement.md")
	}
	proseConv, _ := prose(conv)
	if strings.Contains(proseConv, `\(`) || strings.Contains(proseConv, `\)`) {
		add("MED", "tex-delim", "\\( or \\) left after TeXToUnicode: %s", context80(proseConv, `\(`))
	}
	if i := bareDollar(proseConv); i >= 0 {
		add("MED", "tex-dollar", "bare $ left after TeXToUnicode: %q", around(proseConv, i))
	}
	if m := leftoverCmdRE.FindStringIndex(proseConv); m != nil {
		add("MED", "tex-cmd", "TeX command left after TeXToUnicode: %q", around(proseConv, m[0]))
	}
	// Brackets the converter swallowed: markdown-escaped "\[" ... "\]"
	// read as display math drops the brackets from the text.
	if b, c := strings.Count(proseRaw, "["), strings.Count(proseConv, "["); c < b {
		i := strings.Index(proseRaw, `\[`)
		add("MED", "tex-brackets", "%d of %d '[' lost in TeXToUnicode, e.g. %q", b-c, b, around(proseRaw, max(i, 0)))
	}

	// ---- samples ----------------------------------------------------------
	if len(a.Samples) == 0 && !p.Interactive {
		add("HIGH", "samples", "no samples")
	}
	for i, s := range a.Samples {
		in, outp := read(t, s.In), read(t, s.Out)
		if strings.TrimSpace(in) == "" || strings.TrimSpace(outp) == "" {
			add("HIGH", "samples", "sample %d has an empty side (in %dB, out %dB)", i+1, len(in), len(outp))
		}
		for _, side := range []struct{ name, data string }{{"in", in}, {"out", outp}} {
			if strings.Contains(side.data, "\r") {
				add("MED", "crlf", "sample %d.%s has CR", i+1, side.name)
			}
			f := "```text\n" + side.data + "```\n"
			if !strings.Contains(md, f) {
				add("MED", "sample-fence", "sample %d.%s is not byte-identical to a ```text fence in statement.md", i+1, side.name)
			} else if !strings.Contains(conv, f) {
				add("MED", "sample-fence", "sample %d.%s fence altered by TeXToUnicode", i+1, side.name)
			}
		}
	}

	// ---- tests ------------------------------------------------------------
	if p.Judgeable && len(a.Tests) == 0 {
		add("HIGH", "tests", "judgeable but no tests")
	}
	type tc struct{ in, out string }
	tests := make([]tc, len(a.Tests))
	for i, tt := range a.Tests {
		in, outp := read(t, tt.In), read(t, tt.Out)
		tests[i] = tc{in, outp}
		if strings.Contains(in, "\r") {
			add("MED", "crlf", "test %s.in has CR", tt.Name)
		}
		if strings.Contains(outp, "\r") {
			add("LOW", "crlf", "test %s.out has CR (checker trims it)", tt.Name)
		}
		if in != "" && !strings.HasSuffix(in, "\n") {
			add("LOW", "newline", "test %s.in has no final newline", tt.Name)
		}
		if strings.TrimSpace(outp) == "" {
			add("MED", "empty-out", "test %s.out is empty", tt.Name)
		}
		if truncated(in) || truncated(outp) {
			add("HIGH", "truncated", "test %s looks truncated (ends in ...)", tt.Name)
		}
	}
	// Every test whose input is a sample's must expect the sample's output,
	// under the problem's own checker settings.
	same := func(want, got string) bool {
		w, g := []byte(want), []byte(got)
		if p.AnyCase {
			w, g = judge.FoldYesNo(w), judge.FoldYesNo(g)
		}
		if p.FloatTolerance > 0 {
			ok, _ := judge.CheckFloat(w, g, p.FloatTolerance)
			return ok
		}
		return norm(string(w)) == norm(string(g))
	}
	for i, s := range a.Samples {
		sin, sout := norm(read(t, s.In)), norm(read(t, s.Out))
		found := -1
		for j, tt := range tests {
			if norm(tt.in) == sin {
				found = j
				if !same(sout, tt.out) {
					add("HIGH", "sample-vs-test", "test %d has sample %d's input but a different output:\n  sample: %.120q\n  test:   %.120q",
						j+1, i+1, sout, norm(tt.out))
				}
				break
			}
		}
		if i == 0 && len(tests) > 0 && found != 0 {
			sev := "INFO"
			if p.Source == catalog.USACO {
				sev = "LOW"
			}
			add(sev, "sample1-is-test1", "sample 1 found at test %d (0 = absent)", found+1)
		}
	}

	// ---- I/O and limits ---------------------------------------------------
	switch p.Source {
	case catalog.USACO:
		cpid, _ := strconv.Atoi(strings.TrimPrefix(string(p.ID), "usaco:"))
		switch {
		case cpid < 1059 && p.FileIO == "":
			add("HIGH", "fileio", "file-I/O era problem with no FileIO stem")
		case cpid >= 1059 && p.FileIO != "":
			add("HIGH", "fileio", "stdin era problem with FileIO %q", p.FileIO)
		case p.FileIO != "":
			if !strings.Contains(md, p.FileIO+".in") || !strings.Contains(md, p.FileIO+".out") {
				add("HIGH", "fileio", "statement does not name %s.in and %s.out", p.FileIO, p.FileIO)
			}
		}
		var lim []string
		for _, m := range limitLineRE.FindAllString(md, -1) {
			lim = append(lim, strings.TrimSpace(m))
		}
		if len(lim) > 0 {
			add("INFO", "limits", "T=%v M=%dMB; statement says: %s", p.TimeLimit, p.MemLimit>>20, strings.Join(lim, " | "))
		} else if p.TimeLimit != 2*time.Second || p.MemLimit != 256<<20 {
			add("MED", "limits", "non-default T=%v M=%dMB but statement names no limit", p.TimeLimit, p.MemLimit>>20)
		}
	case catalog.CF:
		if m := cfLimitRE.FindStringSubmatch(md); m == nil {
			add("MED", "limits", "no limits line in statement.md")
		} else {
			sec, _ := strconv.ParseFloat(m[1], 64)
			mb, _ := strconv.ParseInt(m[2], 10, 64)
			if time.Duration(sec*float64(time.Second)) != p.TimeLimit || mb<<20 != p.MemLimit {
				add("MED", "limits", "statement says %ss/%sMB, catalog %v/%dMB", m[1], m[2], p.TimeLimit, p.MemLimit>>20)
			}
		}
		if p.FileIO != "" {
			add("HIGH", "fileio", "codeforces problem with FileIO %q", p.FileIO)
		}
		if n := strings.Count(md, "#### Sample input "); n != len(a.Samples) {
			add("MED", "samples", "%d sample headings, %d sample files", n, len(a.Samples))
		}
		// The index keeps Russian-only statements out of play (Judgeable
		// false), so one is a finding only while it is still offered.
		if cyr := cyrillicRE.FindAllString(md, -1); len(cyr) > 20 {
			sev, what := "MED", "and offered as a door"
			if !p.Judgeable {
				sev, what = "INFO", "and kept out of play"
			}
			add(sev, "language", "statement is in Russian (%d Cyrillic words, e.g. %q) %s", len(cyr), cyr[0], what)
		}
	}
	// What the judge settings imply, for the report.
	if p.FloatTolerance > 0 {
		add("INFO", "float", "FloatTolerance=%g; statement: %s", p.FloatTolerance, context80(md, "error"))
	}
	if p.AnyCase {
		add("INFO", "anycase", "AnyCase; statement: %s", context80(md, "case"))
	}
	return out
}

// prose is md without fenced blocks and inline code spans; open reports a
// ``` or ~~~ fence left open.
func prose(md string) (text string, open bool) {
	var b strings.Builder
	in := false
	var ch byte
	n := 0
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimLeft(line, " ")
		k := 0
		for k < len(t) && (t[0] == '`' || t[0] == '~') && t[k] == t[0] {
			k++
		}
		if k >= 3 && len(line)-len(t) <= 3 {
			if !in {
				in, ch, n = true, t[0], k
				continue
			}
			if t[0] == ch && k >= n && strings.TrimSpace(t[k:]) == "" {
				in = false
				continue
			}
		}
		if !in {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return codeSpanRE.ReplaceAllString(b.String(), ""), in
}

// bareDollar is the index of the first $ not escaped as \$, or -1.
func bareDollar(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '$' {
			return i
		}
	}
	return -1
}

func around(s string, i int) string {
	lo, hi := max(0, i-40), min(len(s), i+40)
	return s[lo:hi]
}

func context80(s, sub string) string {
	i := strings.Index(strings.ToLower(s), strings.ToLower(sub))
	if i < 0 {
		return "(no " + sub + ")"
	}
	return strconv.Quote(strings.ReplaceAll(around(s, i), "\n", " "))
}

// TestShow prints statement.md after TeXToUnicode, as the reader gets it:
// QA_ONLY=usaco:97 go test -tags qa ./qa/content -run TestShow -v
func TestShow(t *testing.T) {
	if os.Getenv("QA_ONLY") == "" {
		t.Skip("set QA_ONLY")
	}
	u := union(t)
	for _, id := range auditIDs {
		if !only(id) {
			continue
		}
		a, err := u.Fetch(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("==== %s\n%s", id, reader.TeXToUnicode(read(t, a.Statement)))
	}
}

// truncated is Codeforces' display cut ("1 2 3..." or "…"), not a grid
// row of dots.
func truncated(s string) bool {
	s = strings.TrimRight(s, "\n")
	if strings.HasSuffix(s, "…") {
		return true
	}
	if !strings.HasSuffix(s, "...") || len(s) < 4 {
		return false
	}
	c := s[len(s)-4]
	return c >= '0' && c <= '9' || c == ' '
}
