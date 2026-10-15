//go:build qa

package judgeqa

import (
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

	"katharsis/internal/judge"
)

// TestQASurveyCF reads a Codeforces content cache (read-only) and, for the
// judgeable problems, counts the sample outputs the checker options would
// misjudge:
//
//   - AnyCase problems whose samples answer with words other than yes/no
//     (only yes/no are folded, so "ALICE" fails against "Alice");
//
//   - exact-match problems whose sample output re-flowed onto one line
//     (one token per line vs space-separated) would fail Check, which
//     Codeforces' token checkers accept.
//
//     JUDGEQA_CF_CACHE=~/Library/Caches/katharsis/codeforces
func TestQASurveyCF(t *testing.T) {
	root := os.Getenv("JUDGEQA_CF_CACHE")
	if root == "" {
		t.Skip("set JUDGEQA_CF_CACHE to a codeforces cache dir")
	}
	var idx struct {
		Entries []struct {
			ID             string
			Judgeable      bool
			AnyCase        bool
			FloatTolerance float64
			Text           [2]int64 `json:"text"`
		} `json:"entries"`
	}
	b, err := os.ReadFile(filepath.Join(root, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &idx); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(root, "text.bin"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	word := regexp.MustCompile(`^[A-Za-z]+$`)
	var anyCase, anyCaseOtherWords, multiLineExact, exact int
	otherWords := map[string]int{}
	var examples []string
	for _, e := range idx.Entries {
		if !e.Judgeable {
			continue
		}
		zr := flate.NewReader(io.NewSectionReader(f, e.Text[0], e.Text[1]))
		raw, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		var tx struct {
			Examples []struct{ In, Out string } `json:"examples"`
		}
		if err := json.Unmarshal(raw, &tx); err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		if e.AnyCase {
			anyCase++
			found := ""
			for _, ex := range tx.Examples {
				for _, tok := range strings.Fields(ex.Out) {
					if word.MatchString(tok) && !strings.EqualFold(tok, "yes") && !strings.EqualFold(tok, "no") {
						found = tok
						otherWords[tok]++
					}
				}
			}
			if found != "" {
				anyCaseOtherWords++
				if len(examples) < 12 {
					examples = append(examples, e.ID+":"+found)
				}
			}
		}
		if e.FloatTolerance == 0 && !e.AnyCase {
			exact++
			// A sample whose answer spans lines but has several tokens on
			// some line: a player printing them all on one line (or one
			// per line) fails Check.
			for _, ex := range tx.Examples {
				w := []byte(ex.Out)
				lines := strings.Count(strings.TrimRight(ex.Out, "\n"), "\n") + 1
				oneLine := []byte(strings.Join(strings.Fields(ex.Out), " ") + "\n")
				if lines > 1 && len(strings.Fields(ex.Out)) > lines {
					if ok, _ := judge.Check(w, oneLine); !ok {
						multiLineExact++
						break
					}
				}
			}
		}
	}
	type kv struct {
		k string
		v int
	}
	var top []kv
	for k, v := range otherWords {
		top = append(top, kv{k, v})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].v > top[j].v })
	if len(top) > 15 {
		top = top[:15]
	}
	note("survey_cf", "anycase", fmt.Sprintf("%d judgeable AnyCase problems; %d have sample answers with words other than yes/no, e.g. %v; top words %v",
		anyCase, anyCaseOtherWords, examples, top))
	note("survey_cf", "exact", fmt.Sprintf("%d judgeable exact-match problems; %d have a multi-line sample answer that a one-line re-flow of the same tokens fails",
		exact, multiLineExact))
}
