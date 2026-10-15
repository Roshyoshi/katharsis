package codeforces

import (
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/web"
)

func TestOpenBuildsIndex(t *testing.T) {
	t.Parallel()
	c, h := openFixture(t)

	// Footer and column ranges per shard, two listing pages, and no
	// byte of any shard's official_tests.
	for path, span := range h.fx.officialSpan {
		for _, rng := range h.rangesOf(path) {
			off, end := parseRange(t, rng, int64(len(h.fx.files[path])))
			if off < span[1] && span[0] < end {
				t.Errorf("%s: fetched %s, which overlaps official_tests [%d, %d)", path, rng, span[0], span[1])
			}
		}
		if n := h.count(path); n < 2 || n > 4 {
			t.Errorf("%s: %d requests, want 2-4", path, n)
		}
	}
	if n := h.total(); n > 2+4*len(fixtureLayout.Shards) {
		t.Errorf("open took %d requests", n)
	}

	tests := []struct {
		id    catalog.ID
		skip  string
		topic string
		vfile string
		vrow  int
	}{
		{"cf:4A", "", "math", "verifiable_tests/train-00000-of-00002.parquet", 0},
		{"cf:919E", "", "math", "verifiable_tests/train-00000-of-00002.parquet", 1},
		{"cf:1520E", skipNoTests, "math", "", 0},
		{"cf:1791A", "", "strings", "verifiable_tests/train-00000-of-00002.parquet", 2},
		{"cf:1899C", "", "dp", "verifiable_tests/train-00000-of-00002.parquet", 4},
		{"cf:2028B", "", "binary-search", "verifiable_tests/test-00000-of-00001.parquet", 0},
		{"cf:2038M", "", "dp", "verifiable_tests/train-00000-of-00002.parquet", 5},
		{"cf:9999Z", skipNotExecutable, "math", "", 0},
		{"cf:1000A", skipInteractive, "math", "verifiable_tests/train-00000-of-00002.parquet", 6},
		{"cf:1001A", skipUnrated, "math", "verifiable_tests/train-00001-of-00002.parquet", 0},
		{"cf:1002A", skipFileIO, "math", "verifiable_tests/train-00001-of-00002.parquet", 1},
		{"cf:1003A", skipTestsTooBig, "math", "verifiable_tests/train-00001-of-00002.parquet", 2},
		{"cf:1004A", skipJoke, "math", "verifiable_tests/train-00001-of-00002.parquet", 3},
		{"cf:1005A", skipChecker, "math", "verifiable_tests/train-00001-of-00002.parquet", 4},
		{"cf:1006A", skipNoTests, "math", "verifiable_tests/train-00001-of-00002.parquet", 5},
		{"cf:1007A", "", "math", "verifiable_tests/train-00001-of-00002.parquet", 6},
	}
	if got := len(c.All()); got != len(tests)+1 { // + 1899/B
		t.Errorf("All() has %d problems, want %d", got, len(tests)+1)
	}
	for _, tc := range tests {
		e, ok := c.entry(tc.id)
		if !ok {
			t.Errorf("%s missing", tc.id)
			continue
		}
		if e.Skip != tc.skip || e.Judgeable != (tc.skip == "") || e.Topic != tc.topic || e.VFile != tc.vfile || e.VRow != tc.vrow {
			t.Errorf("%s: skip %q judgeable %v topic %q vfile %q row %d; want %q %q %q %d",
				tc.id, e.Skip, e.Judgeable, e.Topic, e.VFile, e.VRow, tc.skip, tc.topic, tc.vfile, tc.vrow)
		}
	}

	// One problem in full.
	p, ok := c.Get("cf:2038M")
	want := catalog.Problem{
		ID: "cf:2038M", Source: catalog.CF, Title: "Royal Flush",
		URL:      "https://codeforces.com/contest/2038/problem/M",
		Contest:  "2024-2025 ICPC, NERC, Southern and Volga Russian Regional Contest (Unrated, Online Mirror, ICPC Rules, Preferably Teams) · M",
		Position: 13, Tags: []string{"dp", "implementation"}, Topic: "dp",
		Rating: 2800, RatingSource: "cf", TimeLimit: 3 * time.Second, MemLimit: 512 << 20,
		Judgeable: true, FloatTolerance: 1e-6,
	}
	if !ok || !reflect.DeepEqual(p, want) {
		t.Errorf("Get(cf:2038M) =\n%+v\nwant\n%+v", p, want)
	}
	if e, _ := c.entry("cf:2038M"); !e.HasChecker || e.NeedsChecker {
		t.Errorf("2038M: a float-only checker should not be needed: %+v", e)
	}
	if e, _ := c.entry("cf:1003A"); e.GenBytes != maxGenFile+1 {
		t.Errorf("1003A: GenBytes %d from the second listing entry", e.GenBytes)
	}
	if e, _ := c.entry("cf:2028B"); e.GenBytes == 0 {
		t.Errorf("2028B: GenBytes from the second listing page missing")
	}
	// Get copies Tags.
	p.Tags[0] = "mutated"
	if q, _ := c.Get("cf:2038M"); q.Tags[0] != "dp" {
		t.Error("Get aliases the index's Tags")
	}

	// Contest order, then problem index.
	var ids []string
	for _, p := range c.All() {
		ids = append(ids, string(p.ID))
	}
	if !slices.IsSortedFunc(ids, func(a, b string) int {
		x, _ := strconv.Atoi(digits(a))
		y, _ := strconv.Atoi(digits(b))
		return x - y
	}) {
		t.Errorf("All() not in contest order: %v", ids)
	}

	// A second Open reads the cache and needs no network.
	h.setFail(true)
	before := h.total()
	c2, err := open(t.Context(), c.root, h.srv.URL, fixtureLayout, testClient(h))
	if err != nil {
		t.Fatal(err)
	}
	if h.total() != before {
		t.Error("reopening touched the network")
	}
	if len(c2.All()) != len(c.All()) {
		t.Error("reopened index differs")
	}
}

// digits is the contest number of "cf:1899C".
func digits(id string) string {
	s := catalog.ID(id).Native()
	return s[:strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })]
}

func parseRange(t *testing.T, rng string, size int64) (int64, int64) {
	t.Helper()
	if rng == "" {
		return 0, size
	}
	spec, ok := strings.CutPrefix(rng, "bytes=")
	a, b, ok2 := strings.Cut(spec, "-")
	if !ok || !ok2 {
		t.Fatalf("range %q", rng)
	}
	if a == "" {
		n, _ := strconv.ParseInt(b, 10, 64)
		return max(0, size-n), size
	}
	x, _ := strconv.ParseInt(a, 10, 64)
	y, _ := strconv.ParseInt(b, 10, 64)
	return x, y + 1
}

func TestOpenRebuildsStaleCache(t *testing.T) {
	t.Parallel()
	c, h := openFixture(t)
	tests := []struct {
		name   string
		break_ func(t *testing.T)
	}{
		{"text.bin truncated", func(t *testing.T) {
			if err := web.WriteFile(c.root+"/text.bin", []byte("x")); err != nil {
				t.Fatal(err)
			}
		}},
		{"index from another build", func(t *testing.T) {
			idx := c.idx
			idx.Key = "rev=old"
			for _, id := range c.order {
				idx.Entries = append(idx.Entries, *c.entries[id])
			}
			b := mustJSON(t, idx)
			if err := web.WriteFile(c.root+"/index.json", b); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range tests {
		// Sequential: they share one cache dir.
		tc.break_(t)
		before := h.total()
		c2, err := open(t.Context(), c.root, h.srv.URL, fixtureLayout, testClient(h))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if h.total() == before {
			t.Errorf("%s: the stale cache was used", tc.name)
		}
		if _, err := c2.readText(*c2.entries["cf:4A"]); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

func TestVtestFile(t *testing.T) {
	t.Parallel()
	// The real splits at rev, checked against the files' id columns: the
	// test split's 422 verifiable rows in 40 files (22 of 11, then 10),
	// train's 8338 in 429 (187 of 20, then 19).
	tests := []struct {
		split       string
		pos, n, fil int
		want        string
		row         int
	}{
		{"test", 0, 422, 40, "verifiable_tests/test-00000-of-00040.parquet", 0},
		{"test", 11, 422, 40, "verifiable_tests/test-00001-of-00040.parquet", 0},   // 2028/E
		{"test", 241, 422, 40, "verifiable_tests/test-00021-of-00040.parquet", 10}, // last 11-row file
		{"test", 242, 422, 40, "verifiable_tests/test-00022-of-00040.parquet", 0},  // 1993/F1
		{"test", 421, 422, 40, "verifiable_tests/test-00039-of-00040.parquet", 9},
		{"train", 19, 8338, 429, "verifiable_tests/train-00000-of-00429.parquet", 19},  // 1347/A
		{"train", 3740, 8338, 429, "verifiable_tests/train-00187-of-00429.parquet", 0}, // 177/B1
		{"train", 8337, 8338, 429, "verifiable_tests/train-00428-of-00429.parquet", 18},
		{"train", 8338, 8338, 429, "", 0},
		{"train", 0, 3, 5, "", 0}, // fewer rows than files: no layout
	}
	for _, tc := range tests {
		got, row := vtestFile(tc.split, tc.pos, tc.n, tc.fil)
		if got != tc.want || row != tc.row {
			t.Errorf("vtestFile(%s, %d, %d, %d) = %s, %d; want %s, %d", tc.split, tc.pos, tc.n, tc.fil, got, row, tc.want, tc.row)
		}
	}
}

func TestTopicFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tags []string
		want string
	}{
		{[]string{"dp", "greedy", "two pointers"}, "dp"},
		{[]string{"binary search", "data structures", "greedy"}, "binary-search"},
		{[]string{"combinatorics", "dfs and similar", "dp", "trees"}, "trees"},
		{[]string{"constructive algorithms", "greedy", "math"}, "math"},
		{[]string{"constructive algorithms"}, "mixed"},
		{[]string{"implementation"}, "simulation"},
		{[]string{"bitmasks", "dp"}, "dp"},
		{[]string{"bitmasks"}, "math"},
		{[]string{"dsu", "graphs", "trees"}, "dsu"},
		{[]string{"geometry", "implementation"}, "geometry"},
		{[]string{"hashing", "string suffix structures"}, "strings"},
		{[]string{"shortest paths", "graphs"}, "shortest-paths"},
		{[]string{"*special"}, "mixed"},
		{nil, "mixed"},
	}
	for _, tc := range tests {
		if got := topicFor(tc.tags); got != tc.want {
			t.Errorf("topicFor(%v) = %q, want %q", tc.tags, got, tc.want)
		}
	}
	for tag, topic := range tagTopic {
		if !slices.Contains(catalog.Topics, topic) {
			t.Errorf("tag %q maps to %q, not a catalog topic", tag, topic)
		}
	}
}

func TestNewEntry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  tableRow
		ok   bool
		id   catalog.ID
		pos  int
	}{
		{"plain", made("1899/C", nil), true, "cf:1899C", 3},
		{"numbered index", made("2063/F1", nil), true, "cf:2063F1", 6},
		{"leading zero contest", made("0042/B", nil), true, "cf:42B", 2},
		{"no slash", made("1899C", nil), false, "", 0},
		{"lower-case index", made("1899/c", nil), false, "", 0},
		{"path in index", made("1899/../x", nil), false, "", 0},
		{"zero contest", made("0/A", nil), false, "", 0},
	}
	for _, tc := range tests {
		e, ok := newEntry(tc.row)
		if ok != tc.ok || ok && (e.ID != tc.id || e.Position != tc.pos) {
			t.Errorf("%s: newEntry = %v %s pos %d, want %v %s %d", tc.name, ok, e.ID, e.Position, tc.ok, tc.id, tc.pos)
		}
	}
	r := made("1/A", func(r *tableRow) { r.TimeLimit, r.MemoryLimit = nil, nil })
	if e, _ := newEntry(r); e.TimeLimit != defaultTime || e.MemLimit != defaultMem {
		t.Errorf("missing limits: %v %d", e.TimeLimit, e.MemLimit)
	}
	r = made("1/A", func(r *tableRow) { r.TimeLimit, r.MemoryLimit = ptr(1.5), ptr(64.0) })
	if e, _ := newEntry(r); e.TimeLimit != 1500*time.Millisecond || e.MemLimit != 64<<20 {
		t.Errorf("limits: %v %d", e.TimeLimit, e.MemLimit)
	}
	r = made("1/A", func(r *tableRow) { r.Tags = []string{"*special", "interactive"} })
	if e, _ := newEntry(r); !e.Interactive || slices.Contains(e.Tags, "*special") {
		t.Errorf("tags: %+v", e.Problem)
	}
}

func TestIndexCompare(t *testing.T) {
	t.Parallel()
	in := []string{"D", "C2", "A", "C10", "C", "B", "C1"}
	slices.SortFunc(in, indexCompare)
	if want := []string{"A", "B", "C", "C1", "C2", "C10", "D"}; !slices.Equal(in, want) {
		t.Errorf("sorted %v, want %v", in, want)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The rules newEntry and judgeable apply to a row, on made rows shaped like
// the problems that were judged unfairly.
func TestJudgeRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		row     func(*tableRow)
		skip    string
		anyCase bool
	}{
		{"plain", nil, "", false},
		// cf:233A: no checker in the dataset, so it was judged against one
		// of many valid permutations.
		{"several answers, no checker", func(r *tableRow) {
			r.OutputFormat = "Print the permutation. If there are multiple solutions, print any of them."
		}, skipChecker, false},
		{"several answers, a float checker", func(r *tableRow) {
			r.GeneratedChecker = "import sys"
			r.OutputFormat = "Print the point with absolute or relative error at most $$$10^{-6}$$$. If there are multiple answers, print any."
		}, skipChecker, false},
		// cf:306D: the loose phrase is in the description.
		{"a float checker, any polygon", func(r *tableRow) {
			r.GeneratedChecker = "import sys"
			r.Description = "Draw for him any such polygon with the given number of vertexes."
			r.OutputFormat = "Print the coordinates with absolute or relative error at most $$$10^{-3}$$$."
		}, skipChecker, false},
		{"no checker, any such in the story", func(r *tableRow) {
			r.Description = "Divisor of n is any such natural number that divides n."
		}, "", false},
		{"a float-only checker", func(r *tableRow) {
			r.GeneratedChecker = "import sys"
			r.OutputFormat = "Print the answer with absolute or relative error at most $$$10^{-6}$$$."
		}, "", false},
		{"several answers, one picked", func(r *tableRow) {
			r.OutputFormat = "If there are several solutions, print the lexicographically minimal one."
		}, "", false},
		{"any order", func(r *tableRow) {
			r.OutputFormat = "Print the indices in any order."
		}, skipChecker, false},
		// cf:832A said "arbitrary case", which caseRE missed.
		{"arbitrary case", func(r *tableRow) {
			r.OutputFormat = `Print "YES" or "NO". You can print each letter in arbitrary case (upper or lower).`
		}, "", true},
		// cf:646A.
		{"russian", func(r *tableRow) {
			r.Description = "У Васи есть $$$n$$$ братьев. Найдите, какой из них опоздал на встречу, если известны номера двух пришедших."
		}, skipRussian, false},
		{"english with a russian name", func(r *tableRow) {
			r.Description = "Vasya (Вася) has $$$n$$$ brothers and wants to know which one of them came late to the meeting."
		}, "", false},
		{"no statement", func(r *tableRow) { r.Description = " " }, skipNoStatement, false},
	}
	for _, tc := range tests {
		r := made("1000/A", tc.row)
		e, ok := newEntry(r)
		if !ok {
			t.Fatalf("%s: newEntry rejected the row", tc.name)
		}
		e.VFile = "verifiable_tests/x.parquet"
		e.judgeable(r.Description)
		if e.Skip != tc.skip || e.AnyCase != tc.anyCase {
			t.Errorf("%s: skip %q any-case %v, want %q %v", tc.name, e.Skip, e.AnyCase, tc.skip, tc.anyCase)
		}
	}
}
