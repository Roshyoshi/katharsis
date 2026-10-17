package cses

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
)

func TestParseList(t *testing.T) {
	rows, err := parseList([]byte(fixture(t, "list.html")))
	if err != nil {
		t.Fatal(err)
	}
	want := []row{
		{ID: "1068", Title: "Weird Algorithm", Section: "Introductory Problems", Position: 1, Solvers: 181740, Attempts: 189773},
		{ID: "1070", Title: "Permutations", Section: "Introductory Problems", Position: 2, Solvers: 112214, Attempts: 115440},
		{ID: "1092", Title: "Two Sets", Section: "Introductory Problems", Position: 3, Solvers: 66966, Attempts: 71616},
		{ID: "1084", Title: "Apartments", Section: "Sorting and Searching", Position: 1, Solvers: 67888, Attempts: 78000},
		{ID: "3112", Title: "Hidden Integer", Section: "Interactive Problems", Position: 1, Solvers: 3057, Attempts: 3195},
		{ID: "3400", Title: "Least Solved", Section: "Additional Problems II", Position: 1, Solvers: 116, Attempts: 300},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
	if _, err := parseList([]byte(fixture(t, "home.html"))); err == nil {
		t.Error("a page without tasks parsed as a list")
	}
}

func TestRating(t *testing.T) {
	const top = 181740
	for _, tc := range []struct{ solvers, want int }{
		{181740, 800}, {67888, 1050}, {33173, 1250}, {3334, 1850}, {974, 2150},
		{116, 2700}, {1, 3000}, {0, 3000},
	} {
		if got := rating(tc.solvers, top); got != tc.want {
			t.Errorf("rating(%d) = %d, want %d", tc.solvers, got, tc.want)
		}
	}
	last := 0
	for s := top; s >= 1; s = s * 9 / 10 {
		r := rating(s, top)
		if r < last {
			t.Fatalf("rating not monotone: %d solvers rated %d after %d", s, r, last)
		}
		last = r
	}
}

func TestParseTask(t *testing.T) {
	for _, tc := range []struct {
		id      string
		title   string
		facts   facts
		samples int
		in      []string // substrings of the markdown body
	}{
		{"1068", "Weird Algorithm", facts{TimeLimit: time.Second, MemLimit: 512 << 20}, 1, []string{
			"positive integer $n$.",
			`$$3 \rightarrow 10 \rightarrow 5`,
			`costs \$1`,
			"a\\_[i]",
			"#### Input",
			"```text\n3\n```",
			"```text\n3 10 5 16 8 4 2 1\n```",
			`$1 \le n \le 10^6$`,
		}},
		{"1070", "Permutations", facts{TimeLimit: time.Second, MemLimit: 512 << 20, NeedsChecker: true}, 2, []string{"#### Example 2"}},
		{"1092", "Two Sets", facts{TimeLimit: time.Second, MemLimit: 512 << 20, NeedsChecker: true}, 1, nil},
		{"1084", "Apartments", facts{TimeLimit: 2500 * time.Millisecond, MemLimit: 256 << 20}, 1, nil},
		{"3112", "Hidden Integer", facts{TimeLimit: time.Second, MemLimit: 512 << 20, Interactive: true}, 0, []string{"```text\n? 3\nYES\n! 7\n```"}},
		{"3400", "Least Solved", facts{TimeLimit: time.Second, MemLimit: 512 << 20, FloatTolerance: 1e-9}, 1, nil},
	} {
		st, err := parseTask([]byte(fixture(t, "task_"+tc.id+".html")))
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		if st.Title != tc.title || st.facts != tc.facts || len(st.Samples) != tc.samples {
			t.Errorf("%s: got %q %+v %d samples, want %q %+v %d", tc.id, st.Title, st.facts, len(st.Samples), tc.title, tc.facts, tc.samples)
		}
		for _, s := range tc.in {
			if !strings.Contains(st.Body, s) {
				t.Errorf("%s: markdown lacks %q:\n%s", tc.id, s, st.Body)
			}
		}
		if strings.Contains(st.Body, verbMark) || strings.Contains(st.Body, "katex") {
			t.Errorf("%s: markdown has leftovers:\n%s", tc.id, st.Body)
		}
	}
	st, _ := parseTask([]byte(fixture(t, "task_1070.html")))
	if st.Samples[1] != (sample{In: "3\n", Out: "NO SOLUTION\n"}) {
		t.Errorf("sample 2 = %+v", st.Samples[1])
	}
	if _, err := parseTask([]byte(fixture(t, "login.html"))); err == nil {
		t.Error("the login page parsed as a task")
	}
}

func TestSeveralAnswers(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"If there are several solutions, you may print any of them.", true},
		{"You can print any valid solution.", true},
		{"If there are multiple answers, print any.", true},
		{"print an example of how to create the sets", true},
		{"Print one integer: the number of ways modulo 10^9+7.", false},
		{"If there are no solutions, print NO SOLUTION.", false},
		{"Print the minimum number of moves.", false},
		{"In how many ways can you do this?", false},
		{"On each swap round, you can choose any number of distinct pairs.", false},
		{"If there are several possibilities, you can choose any of them.", true},
		{"You can construct any solution that has at most n reversals.", true},
	} {
		if got := severalRE.MatchString(tc.text); got != tc.want {
			t.Errorf("%q: several = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestDerive(t *testing.T) {
	rows, _ := parseList([]byte(fixture(t, "list.html")))
	table := map[string]facts{"1070": {TimeLimit: time.Second, MemLimit: 512 << 20, NeedsChecker: true}}
	verified := map[string]facts{"1084": {TimeLimit: 2500 * time.Millisecond, MemLimit: 256 << 20}}
	es := derive(rows, table, verified)
	by := map[string]entry{}
	for _, e := range es {
		by[e.ID.Native()] = e
	}
	if e := by["1068"]; !e.Playable || e.Verified || e.TimeLimit != defaultTime || e.Rating != 800 ||
		e.Topic != catalog.Mixed || e.URL != "https://cses.fi/problemset/task/1068" || e.ID != "cses:1068" {
		t.Errorf("1068: %+v", e)
	}
	if e := by["1070"]; e.Playable || !e.NeedsChecker {
		t.Errorf("1070 should need a checker: %+v", e)
	}
	if e := by["1084"]; !e.Verified || e.TimeLimit != 2500*time.Millisecond || e.Topic != "sorting-greedy" || e.Position != 1 {
		t.Errorf("1084: %+v", e)
	}
	if e := by["3112"]; e.Playable || !e.Interactive {
		t.Errorf("3112 is in Interactive Problems: %+v", e)
	}
}

func TestUnzipTests(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		n     int
		err   string
	}{
		{"plain", map[string]string{"1.in": "a", "1.out": "b", "2.in": "c", "2.out": "d"}, 2, ""},
		{"nested and junk", map[string]string{"tests/1.in": "a", "tests/1.out": "b", "../../evil": "x", "__MACOSX/._1.in2": "y"}, 1, ""},
		{"missing out", map[string]string{"1.in": "a"}, 0, "without"},
		{"empty", map[string]string{"readme.txt": "x"}, 0, "no tests"},
		{"duplicate", map[string]string{"a/1.in": "a", "b/1.in": "a", "1.out": "b"}, 0, "two entries"},
	} {
		z := makeZip(t, tc.files)
		dst := t.TempDir()
		n, err := unzipTests(strings.NewReader(string(z)), int64(len(z)), dst)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%s: err = %v, want %q", tc.name, err, tc.err)
			}
			continue
		}
		if err != nil || n != tc.n {
			t.Errorf("%s: %d, %v; want %d", tc.name, n, err, tc.n)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(dst), "evil")); err == nil {
			t.Errorf("%s: wrote outside dst", tc.name)
		}
	}
}

func TestFactsTable(t *testing.T) {
	tab, err := loadFactsTable()
	if err != nil {
		t.Fatal(err)
	}
	if len(tab) < 300 {
		t.Fatalf("facts.json has %d tasks; regenerate it (TestRegenFacts)", len(tab))
	}
	seen := map[string]bool{}
	for _, r := range tab {
		if !validID(r.ID) || seen[r.ID] || r.Title == "" || r.Section == "" || r.TimeLimit <= 0 || r.MemLimit <= 0 {
			t.Errorf("bad row %+v", r)
		}
		seen[r.ID] = true
	}
	for _, id := range []string{"1068", "1084", "1640"} {
		if !seen[id] {
			t.Errorf("facts.json lacks %s", id)
		}
	}
}
