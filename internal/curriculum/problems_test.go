package curriculum

import (
	"strings"
	"testing"

	"katharsis/internal/catalog"
)

func TestCatalogID(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want catalog.ID
	}{
		{"usaco-615", "usaco:615"},
		{"cses-1094", "cses:1094"},
		{"cf-1398C", ""},
		{"usaco-", ""},
		{"usaco-12a", ""},
		{"cses-dp-1", ""},
		{"ys-StaticRangeSum", ""},
		{"usaco", ""},
		{"SPOJ-123", ""},
	} {
		if got := CatalogID(tc.in); got != tc.want {
			t.Errorf("CatalogID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseProblems(t *testing.T) {
	src := `{
  "MODULE_ID": "prefix-sums",
  "sample": [{"uniqueId": "ys-StaticRangeSum", "name": "Static Range Sum",
    "url": "https://judge.yosupo.jp/problem/static_range_sum", "source": "YS",
    "difficulty": "Very Easy", "isStarred": false, "tags": ["Prefix Sums"],
    "solutionMetadata": {"kind": "in-module", "moduleId": "prefix-sums"}}],
  "general": [
    {"uniqueId": "usaco-572", "name": "Breed Counting", "url": "u1", "source": "Silver",
     "difficulty": "Very Easy", "isStarred": true, "tags": ["Prefix Sums"],
     "solutionMetadata": {"kind": "internal"}},
    {"uniqueId": "cses-1662", "name": "Subarray Divisibility", "url": "u2", "source": "CSES",
     "difficulty": "Easy", "isStarred": false, "tags": [], "solutionMetadata": {"kind": "internal"}}
  ]
}`
	id, lists, err := parseProblems("p.json", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if id != "prefix-sums" || len(lists) != 2 || lists[0].Key != "sample" || lists[1].Key != "general" {
		t.Fatalf("id=%q lists=%+v", id, lists)
	}
	p := lists[1].Problems[0]
	if p.ID != "usaco:572" || p.Name != "Breed Counting" || p.Source != "Silver" ||
		p.Difficulty != "Very Easy" || !p.Starred || p.SolutionKind != "internal" {
		t.Errorf("usaco entry = %+v", p)
	}
	if q := lists[1].Problems[1]; q.ID != "cses:1662" {
		t.Errorf("cses entry = %+v", q)
	}
	if s := lists[0].Problems[0]; s.ID != "" || s.SolutionKind != "in-module" || s.URL == "" {
		t.Errorf("external entry = %+v", s)
	}
}

func TestParseProblemsErrors(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"not an object", `[]`, "want a JSON object"},
		{"no module id", `{"general": []}`, "no MODULE_ID"},
		{"no unique id", `{"MODULE_ID": "m", "g": [{"name": "x"}]}`, `list "g" entry 0 has no uniqueId`},
		{"list not array", `{"MODULE_ID": "m", "g": {}}`, `list "g"`},
		{"duplicate key", `{"MODULE_ID": "m", "g": [], "g": []}`, `duplicate key "g"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseProblems("p.json", []byte(tc.src))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestDifficultyRank(t *testing.T) {
	order := []string{"Very Easy", "Easy", "Medium", "Hard", "Very Hard", "Insane", "N/A"}
	for i := 1; i < len(order); i++ {
		if DifficultyRank(order[i-1]) >= DifficultyRank(order[i]) {
			t.Errorf("%s should sort before %s", order[i-1], order[i])
		}
	}
	if DifficultyRank("Bogus") != DifficultyRank("N/A") {
		t.Error("unknown labels should sort with N/A")
	}
}
