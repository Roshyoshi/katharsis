//go:build qa

package content

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"katharsis/internal/screens/reader"
)

// The surveys read the cached Codeforces index and statement text directly
// (no network) and look across every judgeable problem for what the
// per-problem audit can only sample.

type cfEntry struct {
	ID             string   `json:"ID"`
	Title          string   `json:"Title"`
	Rating         int      `json:"Rating"`
	Judgeable      bool     `json:"Judgeable"`
	AnyCase        bool     `json:"AnyCase"`
	FloatTolerance float64  `json:"FloatTolerance"`
	HasChecker     bool     `json:"has_checker"`
	Complete       bool     `json:"complete"`
	GenTests       int      `json:"gen_tests"`
	TestsetSize    int      `json:"testset_size"`
	Text           [2]int64 `json:"text"`
	Tags           []string `json:"Tags"`
}

type cfText struct {
	Description string `json:"description"`
	Input       string `json:"input"`
	Output      string `json:"output"`
	Note        string `json:"note"`
	Examples    []struct {
		In  string `json:"in"`
		Out string `json:"out"`
	} `json:"examples"`
}

func cfIndex(t *testing.T) ([]cfEntry, func(cfEntry) cfText) {
	t.Helper()
	cache, _ := os.UserCacheDir()
	root := filepath.Join(cache, "katharsis", "codeforces")
	b, err := os.ReadFile(filepath.Join(root, "index.json"))
	if err != nil {
		t.Skip("no cached codeforces index")
	}
	var idx struct {
		Entries []cfEntry `json:"entries"`
	}
	if err := json.Unmarshal(b, &idx); err != nil {
		t.Fatal(err)
	}
	blob, err := os.ReadFile(filepath.Join(root, "text.bin"))
	if err != nil {
		t.Fatal(err)
	}
	text := func(e cfEntry) cfText {
		var x cfText
		zr := flate.NewReader(bytes.NewReader(blob[e.Text[0] : e.Text[0]+e.Text[1]]))
		raw, _ := io.ReadAll(zr)
		_ = json.Unmarshal(raw, &x)
		return x
	}
	var out []cfEntry
	for _, e := range idx.Entries {
		if e.Judgeable {
			out = append(out, e)
		}
	}
	return out, text
}

// TestSurveyTeX runs the reader's TeX conversion over every judgeable
// statement and counts TeX commands it leaves behind as raw text.
func TestSurveyTeX(t *testing.T) {
	es, text := cfIndex(t)
	cmds := map[string]int{}
	ex := map[string]string{}
	probs := 0
	for _, e := range es {
		x := text(e)
		all := x.Description + "\n\n" + x.Input + "\n\n" + x.Output + "\n\n" + x.Note
		conv := reader.TeXToUnicode(all)
		seen := map[string]bool{}
		for _, m := range leftoverCmdRE.FindAllString(conv, -1) {
			if !seen[m] {
				seen[m] = true
				cmds[m]++
				if _, ok := ex[m]; !ok {
					ex[m] = e.ID
				}
			}
		}
		if len(seen) > 0 {
			probs++
		}
	}
	type kv struct {
		k string
		v int
	}
	var kvs []kv
	for k, v := range cmds {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].v > kvs[j].v })
	var lines []string
	for i, p := range kvs {
		if i >= 40 {
			break
		}
		lines = append(lines, fmt.Sprintf("%-16s %4d problems (e.g. %s)", p.k, p.v, ex[p.k]))
	}
	t.Logf("%d of %d judgeable statements keep raw TeX commands after TeXToUnicode:\n%s", probs, len(es), strings.Join(lines, "\n"))
}

var decimalRE = regexp.MustCompile(`(?m)(?:^|\s)-?\d+\.\d+(?:\s|$)`)
var yesnoRE = regexp.MustCompile(`(?m)^(?:YES|NO|Yes|No|yes|no)\s*$`)

// TestSurveyCheckers looks for judge settings the statement contradicts:
// real-valued samples judged exactly, and YES/NO answers judged
// case-sensitively although the output section talks about case.
func TestSurveyCheckers(t *testing.T) {
	es, text := cfIndex(t)
	var flt, cas []string
	for _, e := range es {
		x := text(e)
		outs := ""
		for _, s := range x.Examples {
			outs += s.Out + "\n"
		}
		if e.FloatTolerance == 0 && decimalRE.MatchString(outs) {
			o := strings.ReplaceAll(x.Output, "\n", " ")
			flt = append(flt, fmt.Sprintf("%-9s r=%-4d checker=%-5v out: %.160q", e.ID, e.Rating, e.HasChecker, o))
		}
		if !e.AnyCase && yesnoRE.MatchString(outs) {
			o := strings.ToLower(x.Output + " " + x.Note)
			if strings.Contains(o, "case") || strings.Contains(o, "register") || strings.Contains(o, "lowercase") || strings.Contains(o, "uppercase") {
				cas = append(cas, fmt.Sprintf("%-9s r=%-4d out: %.200q", e.ID, e.Rating, strings.ReplaceAll(x.Output, "\n", " ")))
			}
		}
	}
	t.Logf("%d judgeable problems with decimal sample answers and FloatTolerance=0:\n%s", len(flt), strings.Join(flt, "\n"))
	t.Logf("%d judgeable YES/NO problems not AnyCase whose output section mentions case:\n%s", len(cas), strings.Join(cas, "\n"))
}

// TestSurveyTests counts thin test sets: what a full :Submit judges
// against compared with Codeforces' own test count.
func TestSurveyTests(t *testing.T) {
	es, _ := cfIndex(t)
	buckets := map[string]int{}
	for _, e := range es {
		if e.Complete {
			buckets["complete official"]++
			continue
		}
		switch {
		case e.GenTests <= 5:
			buckets["generated <= 5"]++
		case e.GenTests <= 10:
			buckets["generated 6-10"]++
		default:
			buckets["generated > 10"]++
		}
	}
	t.Logf("%d judgeable: %v", len(es), buckets)
}

var wordAnswerRE = regexp.MustCompile(`(?m)^([A-Za-z]+)\s*$`)

// TestSurveyAnyCaseWords lists AnyCase problems whose one-word answers are
// not yes/no (Alice, Bob, First, ...): FoldYesNo folds only yes/no, so
// "alice" against "Alice" stays WA although the statement allows it.
func TestSurveyAnyCaseWords(t *testing.T) {
	es, text := cfIndex(t)
	var lines []string
	for _, e := range es {
		if !e.AnyCase {
			continue
		}
		x := text(e)
		words := map[string]bool{}
		for _, s := range x.Examples {
			for _, m := range wordAnswerRE.FindAllStringSubmatch(s.Out, -1) {
				w := strings.ToLower(m[1])
				if w != "yes" && w != "no" {
					words[m[1]] = true
				}
			}
		}
		if len(words) == 0 {
			continue
		}
		var ws []string
		for w := range words {
			ws = append(ws, w)
		}
		sort.Strings(ws)
		lines = append(lines, fmt.Sprintf("%-9s r=%-4d answers %v", e.ID, e.Rating, ws))
	}
	t.Logf("%d AnyCase problems answer with words other than yes/no:\n%s", len(lines), strings.Join(lines, "\n"))
}

var arbitraryCaseRE = regexp.MustCompile(`(?i)(?:arbitrary|either)\s+case|upper-?\s*or\s+lower-?\s*case|(?:upper|lower)\s+or\s+(?:upper|lower)|small\s+or\s+large|choose\s+the\s+case|lower\s+case\s+or\s+in\s+upper\s+case`)

// TestSurveyCaseMisses lists YES/NO problems not marked AnyCase whose
// output section still says the case is free, in words caseRE misses.
func TestSurveyCaseMisses(t *testing.T) {
	es, text := cfIndex(t)
	var lines []string
	for _, e := range es {
		if e.AnyCase {
			continue
		}
		x := text(e)
		s := strings.ReplaceAll(x.Output+" "+x.Note, "\n", " ")
		if loc := arbitraryCaseRE.FindStringIndex(s); loc != nil {
			lines = append(lines, fmt.Sprintf("%-9s r=%-4d %q", e.ID, e.Rating, s[max(0, loc[0]-60):min(len(s), loc[1]+20)]))
		}
	}
	t.Logf("%d judgeable problems say the case is free but are not AnyCase:\n%s", len(lines), strings.Join(lines, "\n"))
}

// TestSurveyLanguage counts judgeable statements written in Russian.
func TestSurveyLanguage(t *testing.T) {
	es, text := cfIndex(t)
	var lines []string
	for _, e := range es {
		if n := len(cyrillicRE.FindAllString(text(e).Description, -1)); n > 20 {
			lines = append(lines, fmt.Sprintf("%-9s r=%-4d %s", e.ID, e.Rating, e.Title))
		}
	}
	t.Logf("%d judgeable statements are in Russian:\n%s", len(lines), strings.Join(lines, "\n"))
}
