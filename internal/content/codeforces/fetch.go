package codeforces

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/parquet-go/parquet-go"

	"katharsis/internal/catalog"
	"katharsis/internal/content/web"
	"katharsis/internal/problem"
)

const (
	// One generated input reaches about 30 MB at rev (max constraints);
	// the caps only stop a broken file from filling the disk.
	maxTestFile  = 256 << 20
	maxTestTotal = 2 << 30
)

// errTestsMissing means the file the index points at does not hold this
// problem's tests. Retrying cannot help, so the problem leaves door play.
var errTestsMissing = errors.New("tests not found where the index says")

// meta.json is written last into a problem dir; its presence means the
// dir is complete. Key is dirKey of the build that wrote it.
type meta struct {
	Key          string    `json:"key"`
	HasEditorial bool      `json:"has_editorial"`
	Samples      int       `json:"samples"`
	Official     int       `json:"official"`
	Generated    int       `json:"generated"`
	Fetched      time.Time `json:"fetched"`
}

// dirKey names what a problem dir's contents depend on, so a dir written
// under another revision or schema is fetched again, not served stale.
var dirKey = fmt.Sprintf("rev=%s schema=%d", rev, dirSchema)

// dirSchema changes whenever what Fetch writes into a problem dir changes.
// It was the index schema until that reached 3, hence the shared "schema"
// in dirKey, which keeps dirs written under schema 2 valid.
const dirSchema = 2

func loadAssets(dir string) (catalog.Assets, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return catalog.Assets{}, false
	}
	var m meta
	if json.Unmarshal(b, &m) != nil || m.Key != dirKey {
		return catalog.Assets{}, false
	}
	a := catalog.Assets{Statement: filepath.Join(dir, "statement.md")}
	if m.HasEditorial {
		a.Editorial = filepath.Join(dir, "editorial.md")
	}
	if a.Samples, err = pairs(filepath.Join(dir, "samples")); err != nil {
		return catalog.Assets{}, false
	}
	if a.Tests, err = pairs(filepath.Join(dir, "tests")); err != nil {
		return catalog.Assets{}, false
	}
	if len(a.Samples) != m.Samples || len(a.Tests) != m.Official+m.Generated {
		return catalog.Assets{}, false // tampered with; fetch again
	}
	return a, true
}

// pairs lists N.in/N.out in dir in natural order ("2" before "10").
func pairs(dir string) ([]problem.Test, error) {
	ins, err := filepath.Glob(filepath.Join(dir, "*.in"))
	if err != nil {
		return nil, err
	}
	var out []problem.Test
	for _, in := range ins {
		name := strings.TrimSuffix(filepath.Base(in), ".in")
		o := filepath.Join(dir, name+".out")
		if _, err := os.Stat(o); err != nil {
			return nil, fmt.Errorf("%s: %s.in without %s.out", dir, name, name)
		}
		out = append(out, problem.Test{Name: name, In: in, Out: o})
	}
	sort.Slice(out, func(i, j int) bool {
		x, _ := strconv.Atoi(out[i].Name)
		y, _ := strconv.Atoi(out[j].Name)
		return x < y || x == y && out[i].Name < out[j].Name
	})
	return out, nil
}

// materialize builds the problem dir in a temp dir and renames it into
// place, so a crash or a failed request never leaves a dir loadAssets
// would accept. Any failure returns an error and keeps nothing, so the
// next Fetch retries, except errTestsMissing, which also marks the
// problem unjudgeable.
func (c *Catalog) materialize(ctx context.Context, e entry, dir string) (catalog.Assets, error) {
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		return catalog.Assets{}, err
	}
	tmp, err := os.MkdirTemp(c.root, web.TempPrefix+filepath.Base(dir)+"-")
	if err != nil {
		return catalog.Assets{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	t, err := c.readText(e)
	if err != nil {
		return catalog.Assets{}, err
	}
	m := meta{Key: dirKey, Fetched: time.Now().UTC()}
	files := map[string]string{"statement.md": statementMarkdown(e, t)}
	if e.HasEditorial {
		files["editorial.md"] = editorialMarkdown(e, t)
		m.HasEditorial = true
	}
	for i, ex := range t.Examples {
		n := strconv.Itoa(i + 1)
		files["samples/"+n+".in"] = data(ex.Input)
		files["samples/"+n+".out"] = data(ex.Output)
	}
	m.Samples = len(t.Examples)
	if err := writeFiles(tmp, files); err != nil {
		return catalog.Assets{}, err
	}

	tests := filepath.Join(tmp, "tests")
	if err := os.MkdirAll(tests, 0o755); err != nil {
		return catalog.Assets{}, err
	}
	if e.Judgeable {
		if m.Official, m.Generated, err = c.fetchTests(ctx, e, tests); err != nil {
			if errors.Is(err, errTestsMissing) {
				if merr := c.markMissing(e.ID); merr != nil {
					return catalog.Assets{}, merr
				}
			}
			return catalog.Assets{}, fmt.Errorf("%s tests: %w", e.ID, err)
		}
	}

	mb, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return catalog.Assets{}, err
	}
	if err := os.WriteFile(filepath.Join(tmp, "meta.json"), mb, 0o644); err != nil {
		return catalog.Assets{}, err
	}
	if err := web.SyncDir(tmp); err != nil {
		return catalog.Assets{}, err
	}
	// A dir loadAssets rejects is debris from a crash or another build;
	// replace it, and so one written before the problem was judgeable.
	if a, ok := loadAssets(dir); !usable(a, ok, e) {
		_ = os.RemoveAll(dir)
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another process published first; its copy is as good.
		if a, ok := loadAssets(dir); ok {
			return a, nil
		}
		return catalog.Assets{}, err
	}
	a, ok := loadAssets(dir)
	if !ok {
		return catalog.Assets{}, fmt.Errorf("%s: cache dir %s unreadable after fetch", e.ID, dir)
	}
	return a, nil
}

// usable reports whether a problem dir loadAssets returned can be served
// for e. A dir fetched while e was not judgeable has no tests; once a
// rule change makes e judgeable, that dir must be fetched again, or a door
// would judge against nothing.
func usable(a catalog.Assets, ok bool, e entry) bool {
	return ok && (len(a.Tests) > 0 || !e.Judgeable)
}

func writeFiles(dir string, files map[string]string) error {
	for name, s := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// fetchTests writes dst/1.in, 1.out, ...: the dataset's official tests
// first, then for an incomplete set the generated ones. Official tests
// come first because they are small and test 1 is the first sample, so an
// early failure shows the player something readable.
func (c *Catalog) fetchTests(ctx context.Context, e entry, dst string) (official, generated int, err error) {
	w := &testWriter{dir: dst, budget: maxTestTotal}
	if e.VFile != "" {
		official, err = c.officialTests(ctx, e, w)
		switch {
		case err == nil:
		case errors.Is(err, errTestsMissing) && !e.Complete:
			// Generated tests are the full set; the official ones only
			// add a few small cases.
		default:
			return 0, 0, err
		}
	}
	if e.Complete {
		if official == 0 {
			return 0, 0, errTestsMissing
		}
		return official, 0, nil
	}
	generated, err = c.generatedTests(ctx, e, w, official+1)
	return official, generated, err
}

// testWriter writes N.in/N.out under one byte budget for the problem.
type testWriter struct {
	dir    string
	budget int64
}

func (w *testWriter) write(name string, v []byte) error {
	d := dataBytes(v)
	if len(d) > maxTestFile {
		return fmt.Errorf("test %s is over %d bytes", name, maxTestFile)
	}
	if w.budget -= int64(len(d)); w.budget < 0 {
		return fmt.Errorf("tests over %d bytes", maxTestTotal)
	}
	return os.WriteFile(filepath.Join(w.dir, name), d, 0o644)
}

func (w *testWriter) reset() error {
	if err := os.RemoveAll(w.dir); err != nil {
		return err
	}
	w.budget = maxTestTotal
	return os.MkdirAll(w.dir, 0o755)
}

// count checks that tests first .. first+n-1 were written in full and
// returns n.
func (w *testWriter) count(first, n int) (int, error) {
	for i := first; i < first+n; i++ {
		for _, ext := range []string{".in", ".out"} {
			if _, err := os.Stat(filepath.Join(w.dir, strconv.Itoa(i)+ext)); err != nil {
				return 0, fmt.Errorf("test %d%s was not written", i, ext)
			}
		}
	}
	return n, nil
}

type idRow struct {
	ID string `parquet:"id"`
}

var officialCols = map[string][]string{
	".in":  {"official_tests", "list", "element", "input"},
	".out": {"official_tests", "list", "element", "output"},
}

// officialTests reads e's row of its verifiable_tests file: three ranged
// requests (footer, then the id and official_tests columns), usually well
// under a megabyte; a sibling with outputs of many MB can make it tens.
// The row is checked by id, since VRow is computed from the table order
// rather than read from the file.
func (c *Catalog) officialTests(ctx context.Context, e entry, w *testWriter) (int, error) {
	u := c.fileURL(e.VFile)
	f, sf, err := c.openRemote(ctx, u, officialColumn)
	if errors.Is(err, web.ErrNotFound) {
		return 0, fmt.Errorf("%s: %w", u, errTestsMissing)
	}
	if err != nil {
		return 0, err
	}
	ids, err := readAll[idRow](f)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", u, err)
	}
	row := e.VRow
	if row >= len(ids) || ids[row].ID != e.Key {
		row = slices.IndexFunc(ids, func(r idRow) bool { return r.ID == e.Key })
	}
	if row < 0 {
		return 0, fmt.Errorf("%s has no row %s: %w", u, e.Key, errTestsMissing)
	}
	rg, local := rowGroupOf(f, int64(row))
	seen := map[string]int{}
	for ext, col := range officialCols {
		err := streamColumn(sf, f, rg, col, func(r int64) bool { return r == local },
			func(_ int64, seq int, v []byte) error {
				seen[ext] = max(seen[ext], seq+1)
				return w.write(strconv.Itoa(seq+1)+ext, v)
			})
		if errors.Is(err, errUnsupported) {
			return c.officialFallback(f, row, w, u)
		}
		if err != nil {
			return 0, fmt.Errorf("%s: %w", u, err)
		}
	}
	if seen[".in"] != seen[".out"] {
		return 0, fmt.Errorf("%s: %d inputs, %d outputs", u, seen[".in"], seen[".out"])
	}
	return w.count(1, seen[".in"])
}

// officialFallback decodes the official tests with parquet-go, for a
// layout streamColumn does not handle. The columns are already in memory
// and small enough for that.
func (c *Catalog) officialFallback(f *parquet.File, row int, w *testWriter, u string) (int, error) {
	rows, err := readAll[officialRow](f)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", u, err)
	}
	if err := w.reset(); err != nil {
		return 0, err
	}
	for i, t := range rows[row].OfficialTests {
		n := strconv.Itoa(i + 1)
		if err := w.write(n+".in", []byte(t.Input)); err != nil {
			return 0, err
		}
		if err := w.write(n+".out", []byte(t.Output)); err != nil {
			return 0, err
		}
	}
	return len(rows[row].OfficialTests), nil
}

func rowGroupOf(f *parquet.File, row int64) (int, int64) {
	for i, rg := range f.Metadata().RowGroups {
		if row < rg.NumRows {
			return i, row
		}
		row -= rg.NumRows
	}
	return 0, -1
}

type genKey struct {
	ProblemID string `parquet:"problem_id"`
	TestI     int64  `parquet:"test_i,optional"`
}

// fallbackMax caps the contest file parquet-go may decode whole when
// streamColumn cannot read it; see colstream.go for the memory cost.
const fallbackMax = 32 << 20

// generatedTests downloads e's contest file into a new empty dir, writes
// this problem's rows to w as first.in, first.out, ... in test_i order,
// and deletes the file. The file is the unit of download: its inputs are
// one dictionary page, so no byte range holds just this problem's.
func (c *Catalog) generatedTests(ctx context.Context, e entry, w *testWriter, first int) (int, error) {
	dl, err := os.MkdirTemp(c.root, ".dl-")
	if err != nil {
		return 0, err
	}
	defer func() { _ = os.RemoveAll(dl) }()
	f, err := os.Create(filepath.Join(dl, "tests.parquet"))
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	u := c.fileURL(genFile(e.ContestID))
	size, err := c.client.Spool(ctx, u, f, maxGenFile)
	if errors.Is(err, web.ErrNotFound) {
		return 0, fmt.Errorf("%s: %w", u, errTestsMissing)
	}
	if err != nil {
		return 0, err
	}
	pf, err := parquet.OpenFile(f, size, parquet.SkipPageIndex(true), parquet.SkipBloomFilters(true))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", u, err)
	}

	// The key columns are a few hundred bytes; parquet-go reads them and
	// nothing else.
	keys, err := readAll[genKey](pf)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", u, err)
	}
	var rows []int64
	for i, k := range keys {
		if k.ProblemID == e.Key {
			rows = append(rows, int64(i))
		}
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("%s has no rows for %s: %w", u, e.Key, errTestsMissing)
	}
	slices.SortStableFunc(rows, func(a, b int64) int { return cmp.Compare(keys[a].TestI, keys[b].TestI) })
	name := map[int64]string{} // global row -> test number
	for i, r := range rows {
		name[r] = strconv.Itoa(first + i)
	}

	var base int64
	for rg, g := range pf.Metadata().RowGroups {
		lo := base
		base += g.NumRows
		want := func(r int64) bool { _, ok := name[lo+r]; return ok }
		if !slices.ContainsFunc(rows, func(r int64) bool { return r >= lo && r < base }) {
			continue
		}
		for _, col := range []struct{ name, ext string }{{"input", ".in"}, {"output", ".out"}} {
			err := streamColumn(f, pf, rg, []string{col.name}, want, func(r int64, _ int, v []byte) error {
				return w.write(name[lo+r]+col.ext, v)
			})
			if errors.Is(err, errUnsupported) && size <= fallbackMax {
				return c.generatedFallback(pf, name, w, u, first)
			}
			if err != nil {
				return 0, fmt.Errorf("%s: %w", u, err)
			}
		}
	}
	return w.count(first, len(rows))
}

// generatedFallback is generatedTests through parquet-go, one row at a
// time, for a small file streamColumn cannot read.
func (c *Catalog) generatedFallback(pf *parquet.File, name map[int64]string, w *testWriter, u string, first int) (int, error) {
	r := parquet.NewGenericReader[genRow](pf)
	defer func() { _ = r.Close() }()
	buf := make([]genRow, 1)
	for i := int64(0); ; i++ {
		n, err := r.Read(buf)
		if n == 1 {
			if nm, ok := name[i]; ok {
				if err := w.write(nm+".in", []byte(buf[0].Input)); err != nil {
					return 0, err
				}
				if err := w.write(nm+".out", []byte(buf[0].Output)); err != nil {
					return 0, err
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("%s: %w", u, err)
		}
	}
	return w.count(first, len(name))
}
