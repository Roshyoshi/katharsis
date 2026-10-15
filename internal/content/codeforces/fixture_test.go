package codeforces

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"
)

// The fixture is a small copy of the dataset's layout, served by a fake
// Hugging Face. testdata/rows.json holds six real problem rows from
// open-r1/codeforces (CC BY 4.0; editorials cut to 700 characters); the
// rest are made up, one per rule.

// shardRow is a table row as the dataset stores it, official_tests (the
// column Open must never download) in its real place, before
// official_tests_complete.
type shardRow struct {
	ID                    string    `parquet:"id"`
	Aliases               []string  `parquet:"aliases,list"`
	ContestID             string    `parquet:"contest_id"`
	ContestName           string    `parquet:"contest_name"`
	ContestType           string    `parquet:"contest_type"`
	Index                 string    `parquet:"index"`
	Title                 string    `parquet:"title"`
	TimeLimit             *float64  `parquet:"time_limit"`
	MemoryLimit           *float64  `parquet:"memory_limit"`
	Description           string    `parquet:"description,optional"`
	InputFormat           string    `parquet:"input_format,optional"`
	OutputFormat          string    `parquet:"output_format,optional"`
	InteractionFormat     string    `parquet:"interaction_format,optional"`
	Note                  string    `parquet:"note,optional"`
	Examples              []example `parquet:"examples,list"`
	Editorial             string    `parquet:"editorial,optional"`
	Rating                *int64    `parquet:"rating"`
	Tags                  []string  `parquet:"tags,list"`
	TestsetSize           int64     `parquet:"testset_size,optional"`
	OfficialTests         []example `parquet:"official_tests,list"`
	OfficialTestsComplete bool      `parquet:"official_tests_complete,optional"`
	InputMode             string    `parquet:"input_mode,optional"`
	GeneratedChecker      string    `parquet:"generated_checker,optional"`
	Executable            bool      `parquet:"executable,optional"`
	GeneratedTests        int64     `parquet:"generated_tests,optional"`
}

func toShard(r tableRow) shardRow {
	contest, index, _ := strings.Cut(r.ID, "/")
	return shardRow{
		ID: r.ID, ContestID: contest, ContestName: r.ContestName, ContestType: "CF",
		Index: index, Title: r.Title, TimeLimit: r.TimeLimit,
		MemoryLimit: r.MemoryLimit, Description: r.Description, InputFormat: r.InputFormat,
		OutputFormat: r.OutputFormat, InteractionFormat: r.InteractionFormat, Note: r.Note,
		Examples: r.Examples, Editorial: r.Editorial, Rating: r.Rating, Tags: r.Tags,
		TestsetSize: r.TestsetSize, OfficialTestsComplete: r.OfficialTestsComplete,
		InputMode: r.InputMode, GeneratedChecker: r.GeneratedChecker,
		Executable: r.Executable, GeneratedTests: r.GeneratedTests,
	}
}

// Arrow-like rows: dictionary-encoded byte arrays, as pyarrow writes them.
type dictExample struct {
	Input  string `parquet:"input,optional,dict"`
	Output string `parquet:"output,optional,dict"`
}

type vtestRow struct {
	ID            string        `parquet:"id,dict"`
	OfficialTests []dictExample `parquet:"official_tests,list"`
}

type genFileRow struct {
	ProblemID string `parquet:"problem_id,dict"`
	Input     string `parquet:"input,optional,dict"`
	Output    string `parquet:"output,optional,dict"`
	TestI     int64  `parquet:"test_i,optional"`
}

// arrowLike are the writer options that match the real files: Snappy, v1
// data pages.
var arrowLike = []parquet.WriterOption{parquet.Compression(&parquet.Snappy), parquet.DataPageVersion(1)}

func writeParquet[T any](t testing.TB, rows []T, opts ...parquet.WriterOption) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := parquet.NewGenericWriter[T](&buf, opts...)
	if _, err := w.Write(rows); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func ptr[T any](v T) *T { return &v }

// realRows reads testdata/rows.json.
func realRows(t testing.TB) map[string]tableRow {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "rows.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []tableRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	out := map[string]tableRow{}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

// made builds a made-up row; fields default to a judgeable problem with
// complete official tests.
func made(id string, f func(*tableRow)) tableRow {
	contest, _, _ := strings.Cut(id, "/")
	r := tableRow{
		ID: id, ContestName: "Codeforces Round " + contest + " (Div. 2)",
		Title: "Problem " + id, TimeLimit: ptr(2.0), MemoryLimit: ptr(256.0),
		Description: "Find $$$x$$$.", InputFormat: "One integer $$$n$$$.", OutputFormat: "Print the answer.",
		Examples: []example{{Input: "1", Output: "1"}}, Rating: ptr(int64(1200)), Tags: []string{"math"},
		TestsetSize: 2, OfficialTestsComplete: true, InputMode: "stdio", Executable: true,
	}
	if f != nil {
		f(&r)
	}
	return r
}

// fixture is the fake dataset: files by repo path, plus the listing.
type fixture struct {
	files   map[string][]byte
	listing [][]treeEntry // pages
	rows    map[string]tableRow
	// officialSpan is where each table shard keeps official_tests, which
	// Open must never ask for.
	officialSpan map[string][2]int64
}

var fixtureLayout = layout{
	Shards: []shard{
		{"test", "test-00000-of-00001"},
		{"train", "train-00000-of-00002"},
		{"train", "train-00001-of-00002"},
	},
	VTests: map[string]int{"test": 1, "train": 2},
}

func newFixture(t testing.TB) *fixture {
	t.Helper()
	real := realRows(t)
	fx := &fixture{files: map[string][]byte{}, rows: map[string]tableRow{}, officialSpan: map[string][2]int64{}}
	gen := func(r *tableRow) { r.OfficialTestsComplete, r.GeneratedTests = false, 3 }
	shards := [][]tableRow{
		{real["2028/B"]},
		{
			real["4/A"], real["919/E"], real["1520/E"], real["1791/A"],
			made("1899/B", func(r *tableRow) { r.OfficialTestsComplete, r.GeneratedTests = false, 2 }),
			made("1899/C", func(r *tableRow) {
				gen(r)
				r.Title, r.Rating, r.Tags = "Yarik and Array", ptr(int64(1100)), []string{"dp", "greedy", "two pointers"}
				r.Editorial = "Keep the best sum ending here."
			}),
		},
		{
			real["2038/M"],
			edgeRow(), // prose edge cases, not executable: statement_test.go
			made("1000/A", func(r *tableRow) { r.InteractionFormat = "Ask at most 20 queries." }),
			made("1001/A", func(r *tableRow) { r.Rating = nil }),
			made("1002/A", func(r *tableRow) { r.InputMode = "file" }),
			made("1003/A", gen), // contest file listed over maxGenFile
			made("1004/A", func(r *tableRow) { r.ContestName = "April Fools Day Contest 2020" }),
			made("1005/A", func(r *tableRow) {
				r.GeneratedChecker = "import sys"
				r.OutputFormat = "If there are multiple answers, print any of them."
			}),
			made("1006/A", gen), // no contest file at all
			made("1007/A", nil), // missing from its verifiable_tests file
		},
	}
	// Official outputs dwarf the rest, as in the real table; random digits
	// so Snappy cannot shrink them below the merge gap.
	rng := rand.New(rand.NewPCG(1, 2))
	big := make([]byte, 2<<20)
	for i := range big {
		big[i] = byte('0' + rng.IntN(10))
	}
	for i, rows := range shards {
		var srows []shardRow
		for j, r := range rows {
			fx.rows[r.ID] = r
			s := toShard(r)
			if j == 0 {
				s.OfficialTests = []example{{Input: "1", Output: string(big)}}
			}
			srows = append(srows, s)
		}
		path := "data/" + fixtureLayout.Shards[i].Name + ".parquet"
		fx.files[path] = writeParquet(t, srows, arrowLike...)
		fx.officialSpan[path] = columnSpan(t, fx.files[path], "official_tests.")
	}

	// verifiable_tests: the verifiable rows of each split in table order,
	// 14 train rows cut 7 and 7. The first file has 4/A and 919/E
	// swapped and lacks 1899/B, so computed rows are off and the id
	// search must find them; the second lacks 1007/A. The test split's
	// file is in parquet-go's own layout, for the fallback.
	official := func(id string, n int) vtestRow {
		r := vtestRow{ID: id}
		for i := range n {
			r.OfficialTests = append(r.OfficialTests, dictExample{
				Input:  fmt.Sprintf("%s test %d\r\n", id, i+1),
				Output: fmt.Sprintf("%d\r\n", i+1),
			})
		}
		return r
	}
	sample4A := vtestRow{ID: "4/A", OfficialTests: []dictExample{{Input: "8\r\n", Output: "YES\r\n"}, {Input: "5\r\n", Output: "NO\r\n"}, {Input: "4\r\n", Output: "YES\r\n"}}}
	vt := map[string][]vtestRow{
		"verifiable_tests/test-00000-of-00001.parquet": {official("2028/B", 2)},
		"verifiable_tests/train-00000-of-00002.parquet": {
			official("919/E", 3), sample4A, official("1791/A", 2),
			official("1899/C", 1), official("2038/M", 2), official("1000/A", 1),
		},
		"verifiable_tests/train-00001-of-00002.parquet": {
			official("1001/A", 1), official("1002/A", 1), official("1003/A", 1),
			official("1004/A", 1), official("1005/A", 1), official("1006/A", 1),
		},
	}
	for p, rows := range vt {
		if strings.Contains(p, "/test-") {
			fx.files[p] = writeParquet(t, rows)
			continue
		}
		fx.files[p] = writeParquet(t, rows, arrowLike...)
	}

	// Generated tests: 1899 Arrow-like, with 1899/C's rows out of test_i
	// order and a sibling's rows around them; 2028 in parquet-go's own
	// layout (v2 pages, no dictionary), which streamColumn does not read.
	fx.files[genFile(1899)] = writeParquet(t, []genFileRow{
		{"1899/B", "b1\n", "B1\n", 1},
		{"1899/C", "c3 in\n", "c3 out\n", 3},
		{"1899/C", "c1 in\r\n", "c1 out\r\n", 1},
		{"1899/C", "c2 in", "c2 out", 2},
		{"1899/B", "b2\n", "B2\n", 2},
	}, arrowLike...)
	fx.files[genFile(2028)] = writeParquet(t, []genFileRow{
		{"2028/A", "a\n", "A\n", 1},
		{"2028/B", "7\n10 1 0\n", "0\n", 1},
	})
	fx.listing = [][]treeEntry{
		{
			{"file", genFile(1899), int64(len(fx.files[genFile(1899)]))},
			{"file", genFile(1003), maxGenFile + 1},
			{"directory", "generated_tests/old", 0},
		},
		{
			{"file", genFile(2028), int64(len(fx.files[genFile(2028)]))},
			{"file", "generated_tests/README.md", 10},
		},
	}
	return fx
}

// edgeRow is a made-up statement with every prose case the renderer must
// handle; see statement_test.go.
func edgeRow() tableRow {
	return made("9999/Z", func(r *tableRow) {
		r.Executable = false
		r.Title = "Edge *Cases* <b>"
		r.Description = "Costs $5 and $$$x$$$, or 3$ flat. Bold \"**.*..\" stays literal_text.\n" +
			"#define f(x) is not a heading\n" +
			"   indented line\n" +
			"> not a quote\n" +
			"No break spaces and `ticks`.\n\n" +
			"Footnote $$$[0, \\ldots, n-1]$$$$$$^{\\text{∗}}$$$ here.\n" +
			"Display: $$$$$$\\sum_{i=1}^{n} a_i$$$$$$ inline-ish.\n" +
			"OCR $$i,j\\in\\{1,2\\}$$ inline, and\n$$x^2$$\nalone.\n" +
			"Escaped $$$a \\$ b$$$ and $$$ spaced $$$5 digit after."
		r.InputFormat = ""
		r.OutputFormat = "Print YES or NO. You can output the answer in any case."
		r.Note = "<spoiler summary=\"Hint\"><p>Try small cases.</p></spoiler>"
		r.Examples = []example{{Input: "2\r\n1 2", Output: "YES"}, {Input: "```\n", Output: ""}}
		r.Editorial = "Line one.\nLine two with $$$O(n)$$$."
	})
}

// columnSpan finds the byte range of the columns under prefix.
func columnSpan(t testing.TB, b []byte, prefix string) [2]int64 {
	t.Helper()
	f, err := parquet.OpenFile(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	span := [2]int64{-1, -1}
	for _, rg := range f.Metadata().RowGroups {
		for _, cc := range rg.Columns {
			m := cc.MetaData
			if !strings.HasPrefix(strings.Join(m.PathInSchema, "."), prefix) {
				continue
			}
			off := chunkStart(m)
			if span[0] < 0 || off < span[0] {
				span[0] = off
			}
			span[1] = max(span[1], off+m.TotalCompressedSize)
		}
	}
	return span
}

// fakeHF serves a fixture the way huggingface.co lays it out, honoring
// Range headers, and records what was asked for.
type fakeHF struct {
	fx   *fixture
	srv  *httptest.Server
	mu   sync.Mutex
	hits map[string]int
	// ranges asked per file path ("" for a whole-body GET)
	ranges map[string][]string
	fail   bool // answer everything 500
}

func newFakeHF(t testing.TB, fx *fixture) *fakeHF {
	t.Helper()
	h := &fakeHF{fx: fx, hits: map[string]int{}, ranges: map[string][]string{}}
	tree := "/api/datasets/" + repo + "/tree/" + rev + "/generated_tests"
	resolve := "/datasets/" + repo + "/resolve/" + rev + "/"
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "katharsis/") {
			t.Errorf("User-Agent = %q", r.UserAgent())
		}
		h.mu.Lock()
		h.hits[r.URL.Path]++
		fail := h.fail
		h.mu.Unlock()
		if fail {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		switch {
		case r.URL.Path == tree:
			page, _ := strconv.Atoi(r.URL.Query().Get("cursor")) // 0 when absent
			if page >= len(fx.listing) {
				http.NotFound(w, r)
				return
			}
			if page+1 < len(fx.listing) {
				w.Header().Set("Link", fmt.Sprintf(`<%s%s?cursor=%d>; rel="next"`, h.srv.URL, tree, page+1))
			}
			_ = json.NewEncoder(w).Encode(fx.listing[page])
		case strings.HasPrefix(r.URL.Path, resolve):
			p := strings.TrimPrefix(r.URL.Path, resolve)
			b, ok := fx.files[p]
			if !ok {
				http.NotFound(w, r)
				return
			}
			h.mu.Lock()
			h.ranges[p] = append(h.ranges[p], r.Header.Get("Range"))
			h.mu.Unlock()
			http.ServeContent(w, r, p, time.Time{}, bytes.NewReader(b))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(h.srv.Close)
	return h
}

func (h *fakeHF) total() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, v := range h.hits {
		n += v
	}
	return n
}

func (h *fakeHF) count(path string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hits["/datasets/"+repo+"/resolve/"+rev+"/"+path]
}

func (h *fakeHF) setFail(v bool) {
	h.mu.Lock()
	h.fail = v
	h.mu.Unlock()
}

func testClient(h *fakeHF) *politeClient {
	return &politeClient{http: h.srv.Client(), gap: time.Millisecond, retries: 1, backoff: time.Millisecond}
}

// openFixture builds a catalog over a fresh fixture in a temp cache.
func openFixture(t *testing.T) (*Catalog, *fakeHF) {
	t.Helper()
	h := newFakeHF(t, newFixture(t))
	c, err := open(t.Context(), t.TempDir(), h.srv.URL, fixtureLayout, testClient(h))
	if err != nil {
		t.Fatal(err)
	}
	return c, h
}

func (h *fakeHF) rangesOf(path string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.ranges[path]...)
}
