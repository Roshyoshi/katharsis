package codeforces

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// The source is the Hugging Face dataset open-r1/codeforces at a pinned
// revision, so every cached byte stays valid: a newer revision is a code
// change (bump rev, which rebuilds the index and refetches problem dirs).
// The dataset is CC BY 4.0; statement.md and editorial.md credit it.
const (
	repo = "open-r1/codeforces"
	rev  = "fbe3f6e903ee854eec2e69e9d96d0306cde59baf" // 2025-05-19
)

// layout says where the pinned revision keeps the three things Fetch and
// Open read. Tests swap in a small one.
//
//	data/<split>-NNNNN-of-NNNNN.parquet   problem table, default config
//	verifiable_tests/<split>-...parquet   the verifiable rows with their
//	                                      official tests, ~20 rows a file
//	generated_tests/test_cases_NNNN.parquet  generated tests per contest
type layout struct {
	Shards []shard
	// VTests is the number of verifiable_tests files per split. The
	// verifiable config is the default table filtered to executable rows
	// with a full test set (complete official tests or generated ones),
	// in table order, and the files cut it into contiguous runs the way
	// the datasets library shards (see vtestFile). Checked against the
	// files for the first, a middle and the last run of each split.
	VTests map[string]int
}

type shard struct {
	Split string // "train" or "test"
	Name  string // "train-00003-of-00011"
}

var pinned = func() layout {
	l := layout{
		Shards: []shard{{"test", "test-00000-of-00001"}},
		VTests: map[string]int{"test": 40, "train": 429},
	}
	for i := range 11 {
		l.Shards = append(l.Shards, shard{"train", fmt.Sprintf("train-%05d-of-00011", i)})
	}
	return l
}()

func (c *Catalog) fileURL(path string) string {
	return c.base + "/datasets/" + repo + "/resolve/" + rev + "/" + path
}

// vtestFile names the verifiable_tests file holding row pos of a split of
// n rows cut into files: the first n%files files get one extra row
// (datasets' Dataset.shard(contiguous=True)). It returns the row's offset
// in that file.
func vtestFile(split string, pos, n, files int) (string, int) {
	div, mod := n/files, n%files
	if div == 0 || pos < 0 || pos >= n {
		return "", 0
	}
	var k, off int
	if b := mod * (div + 1); pos < b {
		k, off = pos/(div+1), pos%(div+1)
	} else {
		k, off = mod+(pos-b)/div, (pos-b)%div
	}
	return fmt.Sprintf("verifiable_tests/%s-%05d-of-%05d.parquet", split, k, files), off
}

func genFile(contest int) string {
	return fmt.Sprintf("generated_tests/test_cases_%04d.parquet", contest)
}

// ---- rows --------------------------------------------------------------------

// tableRow is the part of a problem-table row the catalog keeps. Its
// field set decides which columns parquet-go decodes; Open downloads every
// column but official_tests.
type tableRow struct {
	ID                    string    `parquet:"id"` // "1899/C"
	ContestName           string    `parquet:"contest_name"`
	Title                 string    `parquet:"title"`
	TimeLimit             *float64  `parquet:"time_limit"`   // seconds
	MemoryLimit           *float64  `parquet:"memory_limit"` // MB
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
	OfficialTestsComplete bool      `parquet:"official_tests_complete,optional"`
	InputMode             string    `parquet:"input_mode,optional"` // "stdio" or "file"
	GeneratedChecker      string    `parquet:"generated_checker,optional"`
	Executable            bool      `parquet:"executable,optional"`
	GeneratedTests        int64     `parquet:"generated_tests,optional"`
}

type example struct {
	Input  string `parquet:"input,optional" json:"in"`
	Output string `parquet:"output,optional" json:"out"`
}

func tableColumn(path string) bool { return !strings.HasPrefix(path, "official_tests.") }

// officialRow is a verifiable_tests row, read for its official tests only:
// those columns are small there (~20 rows a file) while the same column
// in the problem table is gigabytes.
type officialRow struct {
	ID            string    `parquet:"id"`
	OfficialTests []example `parquet:"official_tests,list"`
}

func officialColumn(path string) bool {
	return path == "id" || strings.HasPrefix(path, "official_tests.")
}

// genRow is a generated test. The card calls the index test_case_i; the
// files at rev name it test_i.
type genRow struct {
	ProblemID string `parquet:"problem_id"`
	Input     string `parquet:"input,optional"`
	Output    string `parquet:"output,optional"`
	TestI     int64  `parquet:"test_i,optional"`
}

// ---- reading the table -------------------------------------------------------

// readShard downloads the wanted columns of one table shard and decodes
// its rows.
func (c *Catalog) readShard(ctx context.Context, s shard) ([]tableRow, error) {
	u := c.fileURL("data/" + s.Name + ".parquet")
	f, _, err := c.openRemote(ctx, u, tableColumn)
	if err != nil {
		return nil, err
	}
	rows, err := readAll[tableRow](f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	return rows, nil
}

type treeEntry struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

var nextLinkRE = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// genSizes lists generated_tests/ at rev: contest id -> file size. Fetch
// downloads a whole contest file for one problem, so its size decides
// whether the problem is worth offering (see maxGenFile). The API pages
// the listing (1000 entries a page, 1705 files) through Link headers.
func (c *Catalog) genSizes(ctx context.Context) (map[int]int64, error) {
	out := map[int]int64{}
	next := c.base + "/api/datasets/" + repo + "/tree/" + rev + "/generated_tests"
	for pages := 0; next != ""; pages++ {
		if pages > 100 {
			return nil, fmt.Errorf("generated_tests listing: over %d pages", pages)
		}
		body, hdr, err := c.client.Bytes(ctx, next, 16<<20)
		if err != nil {
			return nil, err
		}
		var es []treeEntry
		if err := json.Unmarshal(body, &es); err != nil {
			return nil, fmt.Errorf("generated_tests listing: %w", err)
		}
		for _, e := range es {
			name, ok := strings.CutPrefix(e.Path, "generated_tests/test_cases_")
			name, ok2 := strings.CutSuffix(name, ".parquet")
			if id, err := strconv.Atoi(name); ok && ok2 && err == nil && e.Type == "file" {
				out[id] = e.Size
			}
		}
		// The link is absolute; follow it only back to the same server.
		if next = nextLink(hdr); next != "" && !strings.HasPrefix(next, c.base+"/") {
			return nil, fmt.Errorf("generated_tests listing: next page %q is off-site", next)
		}
	}
	return out, nil
}

func nextLink(h http.Header) string {
	for _, v := range h.Values("Link") {
		if m := nextLinkRE.FindStringSubmatch(v); m != nil {
			return m[1]
		}
	}
	return ""
}
