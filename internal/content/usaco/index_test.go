package usaco

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
)

// testdata/guide is a miniature guide snapshot: real file shapes, a
// handful of rows chosen to exercise each derivation rule.
const testGuide = "testdata/guide"

func openTest(t *testing.T, root string) (*Catalog, *int) {
	t.Helper()
	calls := 0
	c, err := open(context.Background(), root, func(context.Context) (string, error) {
		calls++
		return testGuide, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return c, &calls
}

func TestIndex(t *testing.T) {
	t.Parallel()
	c, _ := openTest(t, t.TempDir())
	tests := []struct {
		id   catalog.ID
		want catalog.Problem
	}{
		{"usaco:50", catalog.Problem{
			// Unverified (not in facts.json) and file-I/O era: the I/O file
			// name is unknown, so it is listed but not judgeable.
			Title: "Old Thing", Division: "silver", Topic: "graphs",
			Rating: 1700, RatingSource: "division+guide-label",
			TimeLimit: 2 * time.Second, MemLimit: 256 << 20, HasEditorial: true,
		}},
		{"usaco:84", catalog.Problem{
			Title: "Contest Timing", Division: "bronze", Contest: "2011 November, Bronze P1", Position: 1,
			Tags: []string{"Simulation"}, Topic: "simulation",
			Rating: 850, RatingSource: "division+position+guide-label", // old bronze 1000, Very Easy -150
			TimeLimit: 2 * time.Second, MemLimit: 256 << 20, FileIO: "ctiming",
			Judgeable: true, HasEditorial: true,
		}},
		{"usaco:98", catalog.Problem{
			// rblock.zip holds the 2014 rerun's larger data.
			Title: "Roadblock", Division: "silver", Contest: "2011 December, Silver P2", Position: 2,
			Topic: "mixed", Rating: 1700, RatingSource: "division+position",
			TimeLimit: 2 * time.Second, MemLimit: 256 << 20, FileIO: "rblock", HasEditorial: true,
		}},
		{"usaco:552", catalog.Problem{
			Title: "Googol", Division: "gold", Contest: "2015 US Open, Gold P1", Position: 1,
			Topic: "mixed", Rating: 1950, RatingSource: "division+position",
			TimeLimit: 4 * time.Second, MemLimit: 256 << 20, Interactive: true, HasEditorial: true,
		}},
		{"usaco:1059", catalog.Problem{
			// Tags merge across modules; dp outranks sorting-greedy. Only the
			// Bronze module's "Easy" counts, not the Gold one's "Very Hard".
			// Solve rates 0.99 vs siblings 0.5, 0.9: -87.
			Title: "Do You Know Your ABCs?", Division: "bronze", Contest: "2020 December, Bronze P1", Position: 1,
			Tags: []string{"Sorting", "DP"}, Topic: "dp",
			Rating: 1000 - 75 - 87, RatingSource: "division+position+guide-label+solve-rate",
			TimeLimit: 2 * time.Second, MemLimit: 256 << 20, Judgeable: true, HasEditorial: true,
		}},
		{"usaco:1060", catalog.Problem{
			// Only in extraProblems.json: tags count, the "N/A" label does not.
			Title: "Daisy Chains", Division: "bronze", Contest: "2020 December, Bronze P2", Position: 2,
			Tags: []string{"Complete Search", "Prefix Sums"}, Topic: "prefix-sums",
			Rating: 1150 + 134, RatingSource: "division+position+solve-rate",
			TimeLimit: 2 * time.Second, MemLimit: 256 << 20, Judgeable: true, HasEditorial: true,
		}},
		{"usaco:1061", catalog.Problem{
			// No tags: the module (intro-sorting) names the topic.
			Title: "Stuck in a Rut", Division: "bronze", Contest: "2020 December, Bronze P3", Position: 3,
			Topic: "sorting-greedy", Rating: 1300 + 75 - 47, RatingSource: "division+position+guide-label+solve-rate",
			TimeLimit: 2 * time.Second, MemLimit: 256 << 20, Judgeable: true, HasEditorial: true,
		}},
		{"usaco:9999", catalog.Problem{
			// A problem newer than facts.json: div_to_probs supplies the
			// header; stdin era, so judgeable on the defaults until a fetch.
			Title: "Future Problem", Division: "gold", Contest: "2030 December, Gold P1", Position: 1,
			Topic: "mixed", Rating: 1700, RatingSource: "division+position",
			TimeLimit: 2 * time.Second, MemLimit: 256 << 20, Judgeable: true, HasEditorial: true,
		}},
	}
	var ids []catalog.ID
	for _, p := range c.All() {
		ids = append(ids, p.ID)
	}
	var want []catalog.ID
	for _, tc := range tests {
		want = append(want, tc.id)
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("All() ids = %v, want %v", ids, want)
	}
	for _, tc := range tests {
		got, ok := c.Get(tc.id)
		if !ok {
			t.Errorf("Get(%s) missing", tc.id)
			continue
		}
		w := tc.want
		w.ID, w.Source, w.URL = tc.id, catalog.USACO, statementURL(atoi(tc.id.Native()))
		if !equalProblem(got, w) {
			t.Errorf("Get(%s)\n got %+v\nwant %+v", tc.id, got, w)
		}
	}
	if _, ok := c.Get("usaco:1"); ok {
		t.Error("Get(usaco:1) found a problem not in the guide")
	}
}

func equalProblem(a, b catalog.Problem) bool {
	if len(a.Tags) == 0 {
		a.Tags = nil
	}
	if len(b.Tags) == 0 {
		b.Tags = nil
	}
	return reflect.DeepEqual(a, b)
}

func TestIndexCached(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	c1, calls := openTest(t, root)
	if *calls != 1 {
		t.Fatalf("first open: %d snapshot calls, want 1", *calls)
	}
	// The cached index must be used without the guide, so a deleted or
	// unreachable snapshot does not matter.
	c2, err := open(context.Background(), root, func(context.Context) (string, error) {
		return "", errors.New("snapshot must not be needed")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(c1.All(), c2.All(), equalProblem) {
		t.Error("the cached index differs from the one built")
	}

	// A stale key (older schema, other guide commit) is rebuilt.
	p := filepath.Join(root, "index.json")
	if err := os.WriteFile(p, []byte(`{"key":"old","entries":[{"cpid":1}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, calls := openTest(t, root); *calls != 1 {
		t.Errorf("stale index: %d snapshot calls, want 1", *calls)
	}
}

// An index the previous schema wrote, against the same guide commit and
// facts.json, holds topics from the old rules: it is rebuilt, not served.
func TestIndexOldSchemaRebuilt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	openTest(t, root)
	p := filepath.Join(root, "index.json")
	idx, ok := loadIndex(p)
	if !ok {
		t.Fatal("no index written")
	}
	cur := fmt.Sprintf("schema=%d ", schema)
	idx.Key = strings.Replace(idx.Key, cur, fmt.Sprintf("schema=%d ", schema-1), 1)
	if !strings.Contains(indexKey(), cur) || idx.Key == indexKey() {
		t.Fatalf("key %q does not carry the schema", indexKey())
	}
	for i := range idx.Entries {
		idx.Entries[i].Topic = "two-pointers" // what the old rules said
	}
	if err := saveIndex(p, idx); err != nil {
		t.Fatal(err)
	}
	c, calls := openTest(t, root)
	if *calls != 1 {
		t.Errorf("old-schema index: %d snapshot calls, want 1 (a rebuild)", *calls)
	}
	if got, _ := c.Get("usaco:1059"); got.Topic != "dp" {
		t.Errorf("usaco:1059 topic = %s, want dp from the rebuilt index", got.Topic)
	}
}

// Get must hand out copies: the engine must not be able to edit the
// catalog through a returned slice.
func TestGetCopies(t *testing.T) {
	t.Parallel()
	c, _ := openTest(t, t.TempDir())
	p, _ := c.Get("usaco:1059")
	p.Tags[0] = "mutated"
	if q, _ := c.Get("usaco:1059"); q.Tags[0] != "Sorting" {
		t.Errorf("Get aliases tags: %v", q.Tags)
	}
}

func TestEstimate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		era    string
		pos    int
		labels []string
		rating int
		src    string
		why    string
	}{
		{"bronze", 1, nil, 1000, "division+position", ""},
		{"silver", 3, nil, 1650, "division+position", ""},
		{"platinum", 2, []string{"Hard", "Very Hard"}, 2250 + 113, "division+position+guide-label", "mean of 75 and 150"},
		{"old gold", 4, nil, 2250, "division+position", "a fourth problem rates as P3"},
		{"gold", 0, nil, 1850, "division", "unknown position: middle of the band"},
		{"gold", 1, []string{"N/A", "Weird"}, 1700, "division+position", "unknown labels ignored"},
	}
	for _, tc := range tests {
		r, src := estimate(tc.era, tc.pos, tc.labels)
		if r != tc.rating || src != tc.src {
			t.Errorf("estimate(%s, %d, %v) = %d, %s; want %d, %s (%s)",
				tc.era, tc.pos, tc.labels, r, src, tc.rating, tc.src, tc.why)
		}
	}
}

func TestSolveRateDelta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		row  []float64
		pos  int
		want int
		ok   bool
	}{
		{[]float64{0.999, 0.474, 0.917}, 2, 145, true}, // 2016 US Open Bronze
		{[]float64{0.387, 0.964, 1}, 1, 150, true},     // 2024 January Platinum: clamped from 178
		{[]float64{0.99, 0.98, 0.97}, 1, -5, true},
		{[]float64{1, 1, 1}, 3, 0, true},
		{nil, 1, 0, false},
		{[]float64{1, 1, 1}, 0, 0, false},
	}
	for _, tc := range tests {
		got, ok := solveRateDelta(tc.row, tc.pos)
		if got != tc.want || ok != tc.ok {
			t.Errorf("solveRateDelta(%v, %d) = %d, %v; want %d, %v", tc.row, tc.pos, got, ok, tc.want, tc.ok)
		}
	}
}

func TestTopicFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tags, modules []string
		want          string
	}{
		{[]string{"Tree", "DP"}, nil, "trees"},
		{[]string{"Sorting", "Prefix Sums"}, nil, "prefix-sums"},
		{[]string{"Binary Search", "Graphs"}, nil, "graphs"},
		{[]string{"Convex", "DP"}, nil, "dp"},
		{[]string{"Convex"}, nil, "geometry"},
		{[]string{"Casework"}, []string{"ad-hoc"}, "mixed"},
		{nil, []string{"conclusion", "knapsack"}, "dp"},
		{nil, []string{"merging"}, "trees"}, // the guide files it under Trees
		{nil, nil, "mixed"},
		// Real rows: each sits in its module's biome, the topic that
		// module's trial grants.
		{[]string{"Sweep Line"}, []string{"sweep-line"}, "geometry"},                       // 943
		{[]string{"Set", "Sliding Window"}, []string{"sliding-window"}, "data-structures"}, // 767
		{[]string{"Bitset", "PIE"}, []string{"bitsets", "PIE", "combo"}, "math"},           // 862
		{[]string{"2D Prefix Sums", "Max Subarray Sum", "Sliding Window"},
			[]string{"more-prefix-sums", "sliding-window"}, "prefix-sums"}, // 923
		{[]string{"Indexed Set", "Merging"}, []string{"merging"}, "trees"}, // 696
	}
	for _, tc := range tests {
		if got := topicFor(tc.tags, tc.modules); got != tc.want {
			t.Errorf("topicFor(%v, %v) = %s, want %s", tc.tags, tc.modules, got, tc.want)
		}
	}
}

// A tag that names a guide module must map to the topic that module
// teaches (guide.ModuleTopic), or the module's own problems sit in another
// biome than the one its trial grants. Four once disagreed: Sliding
// Window, Sweep Line, Merging and Bitset.
func TestTagsAgreeWithModules(t *testing.T) {
	t.Parallel()
	for tag, module := range map[string]string{
		"Sliding Window": "sliding-window", "Sweep Line": "sweep-line",
		"Merging": "merging", "Small to Large": "merging", "Bitset": "bitsets",
		"Prefix Sums": "prefix-sums", "Two Pointers": "two-pointers",
		"Binary Search": "binary-search", "Ternary Search": "ternary-search",
		"Stack": "stacks", "Priority Queue": "priority-queues", "PURS": "PURS",
		"DSU": "dsu", "MST": "mst", "TopoSort": "toposort", "SCC": "SCC",
		"Euler Tour": "tree-euler", "Binary Jumping": "binary-jump", "HLD": "hld",
		"Centroid": "centroid", "Hashing": "hashing", "Knapsack": "knapsack",
		"Slope Trick": "slope-trick", "PIE": "PIE", "Matrix": "matrix-expo",
		"Meet in the Middle": "meet-in-the-middle", "Simulation": "simulation",
	} {
		want, ok := guide.ModuleTopic(module)
		if !ok {
			t.Errorf("guide.ModuleTopic(%q) unknown", module)
			continue
		}
		got, ok := tagTopic[tag]
		if !ok {
			got = catalog.Mixed
		}
		if got != want {
			t.Errorf("tag %q -> %s, but module %s teaches %s", tag, got, module, want)
		}
	}
}

// Every topic the tag table produces must be one the engine knows
// (guide tests its module table).
func TestTopicsAreCatalogTopics(t *testing.T) {
	t.Parallel()
	for k, v := range tagTopic {
		if !slices.Contains(catalog.Topics, v) {
			t.Errorf("%s maps to %q, not in catalog.Topics", k, v)
		}
	}
	for _, v := range topicRank {
		if !slices.Contains(catalog.Topics, v) {
			t.Errorf("topicRank has %q, not in catalog.Topics", v)
		}
	}
}

// facts.json must cover the pinned guide and agree with what the brief
// verified by hand: 719 problems, 717 zips (552 and 948 are interactive
// and have none), plus rblock's 2011 problem whose zip is the 2014 data.
func TestFactsTable(t *testing.T) {
	t.Parallel()
	table, err := loadFactsTable()
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != 719 {
		t.Errorf("facts.json has %d problems, want 719", len(table))
	}
	var noZip, interactive []int
	for id, r := range table {
		if !r.HasZip {
			noZip = append(noZip, id)
		}
		if r.Interactive {
			interactive = append(interactive, id)
		}
		if r.Title == "" || r.Contest == "" || r.Division == "" || r.Position < 1 || r.Position > 4 {
			t.Errorf("cpid %d: incomplete header %+v", id, r.facts)
		}
		if (id < firstStdinCPID) != (r.FileIO != "") && !r.Interactive {
			t.Errorf("cpid %d: file I/O %q outside its era", id, r.FileIO)
		}
	}
	slices.Sort(noZip)
	slices.Sort(interactive)
	if !slices.Equal(noZip, []int{98, 552, 948}) {
		t.Errorf("no zip: %v", noZip)
	}
	if !slices.Equal(interactive, []int{552, 841, 948}) {
		t.Errorf("interactive: %v", interactive)
	}
}

// A division's band runs from its P1 to its P3 rating, so a problem rated
// by position alone lands inside its division's band.
func TestDivisionBand(t *testing.T) {
	t.Parallel()
	for _, div := range []string{"bronze", "silver", "gold", "platinum"} {
		b, ok := DivisionBand(div)
		if !ok || b[0] >= b[1] {
			t.Fatalf("DivisionBand(%q) = %v, %v", div, b, ok)
		}
		for pos := 1; pos <= 3; pos++ {
			if r, _ := estimate(div, pos, nil); r < b[0] || r > b[1] {
				t.Errorf("%s P%d rated %d, outside its band %v", div, pos, r, b)
			}
		}
	}
	for _, div := range []string{"old gold", "plat", ""} {
		if b, ok := DivisionBand(div); ok {
			t.Errorf("DivisionBand(%q) = %v, want no band", div, b)
		}
	}
}
