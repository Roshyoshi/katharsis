package usaco

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/catalog"
)

// fakeUSACO serves saved pages and zips the way usaco.org lays them out and
// counts requests per path.
type fakeUSACO struct {
	mu    sync.Mutex
	hits  map[string]int
	files map[string][]byte // request path (with query) -> body
}

func newFakeUSACO(t *testing.T) (*fakeUSACO, *httptest.Server) {
	t.Helper()
	read := func(p string) []byte {
		b, err := os.ReadFile(filepath.Join("testdata", p))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	f := &fakeUSACO{hits: map[string]int{}, files: map[string][]byte{
		"/index.php?page=viewproblem2&cpid=84":   read("pages/84.html"),
		"/index.php?page=viewproblem2&cpid=1059": read("pages/1059.html"),
		"/index.php?page=viewproblem2&cpid=552":  read("pages/552.html"),
		// A problem newer than facts.json; the page decides its facts.
		"/index.php?page=viewproblem2&cpid=9999":    read("pages/1211.html"),
		"/current/data/ctiming.zip":                 read("zips/ctiming.zip"),
		"/current/data/prob1_bronze_dec20.zip":      read("zips/gifts.zip"),
		"/current/data/sol_ctiming.html":            read("pages/sol_ctiming.html"),
		"/current/data/sol_prob1_bronze_dec20.html": read("pages/sol_prob1_bronze_dec20.html"),
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.RequestURI()
		f.mu.Lock()
		f.hits[key]++
		body, ok := f.files[key]
		f.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeUSACO) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[key]
}

func (f *fakeUSACO) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, v := range f.hits {
		n += v
	}
	return n
}

func testCatalog(t *testing.T, root string, srv *httptest.Server) *Catalog {
	t.Helper()
	c, _ := openTest(t, root)
	c.base = srv.URL
	c.client = &politeClient{http: srv.Client(), gap: time.Millisecond, retries: 2, backoff: time.Millisecond}
	return c
}

func TestFetch(t *testing.T) {
	t.Parallel()
	fake, srv := newFakeUSACO(t)
	root := t.TempDir()
	c := testCatalog(t, root, srv)
	ctx := context.Background()

	// Eight concurrent callers share one download per resource.
	var wg sync.WaitGroup
	results := make([]catalog.Assets, 8)
	errs := make([]error, 8)
	for i := range results {
		wg.Go(func() { results[i], errs[i] = c.Fetch(ctx, "usaco:84") })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	for _, key := range []string{"/index.php?page=viewproblem2&cpid=84", "/current/data/ctiming.zip", "/current/data/sol_ctiming.html"} {
		if n := fake.count(key); n != 1 {
			t.Errorf("%s fetched %d times, want 1", key, n)
		}
	}
	a := results[0]
	dir := filepath.Join(root, "84")
	if a.Statement != filepath.Join(dir, "statement.md") || a.Editorial != filepath.Join(dir, "editorial.md") {
		t.Errorf("assets = %+v", a)
	}
	// I.N/O.N renamed to N.in/N.out, in natural order.
	if len(a.Tests) != 10 || a.Tests[1].Name != "2" || a.Tests[9].Name != "10" {
		t.Fatalf("tests = %+v", a.Tests)
	}
	zr, _ := zip.OpenReader(filepath.Join("testdata", "zips", "ctiming.zip"))
	defer zr.Close()
	for _, zf := range zr.File {
		rc, _ := zf.Open()
		var want bytes.Buffer
		want.ReadFrom(rc)
		rc.Close()
		kind, n, _ := strings.Cut(zf.Name, ".")
		ext := map[string]string{"I": ".in", "O": ".out"}[kind]
		got, err := os.ReadFile(filepath.Join(dir, "tests", n+ext))
		if err != nil || !bytes.Equal(got, want.Bytes()) {
			t.Errorf("tests/%s%s does not match zip entry %s", n, ext, zf.Name)
		}
	}
	if len(a.Samples) != 1 {
		t.Fatalf("samples = %+v", a.Samples)
	}
	if b, _ := os.ReadFile(a.Samples[0].In); string(b) != "12 13 14\n" {
		t.Errorf("sample input = %q", b)
	}
	for _, p := range []string{a.Statement, a.Editorial} {
		if b, err := os.ReadFile(p); err != nil || len(b) < 100 {
			t.Errorf("%s: %d bytes, %v", p, len(b), err)
		}
	}

	// Idempotent: a finished dir is reused without the network, by this
	// catalog and by a fresh one over the same cache.
	before := fake.total()
	if b, err := c.Fetch(ctx, "usaco:84"); err != nil || b.Statement != a.Statement || len(b.Tests) != 10 {
		t.Errorf("second fetch = %+v, %v", b, err)
	}
	if b, err := testCatalog(t, root, srv).Fetch(ctx, "usaco:84"); err != nil || len(b.Tests) != 10 {
		t.Errorf("fetch from a new catalog = %+v, %v", b, err)
	}
	if fake.total() != before {
		t.Errorf("cached fetch made %d requests", fake.total()-before)
	}
	leftovers, _ := filepath.Glob(filepath.Join(root, ".tmp-*"))
	if len(leftovers) > 0 {
		t.Errorf("temp dirs left: %v", leftovers)
	}
}

func TestFetchParallelIDs(t *testing.T) {
	t.Parallel()
	_, srv := newFakeUSACO(t)
	c := testCatalog(t, t.TempDir(), srv)
	var wg sync.WaitGroup
	for _, id := range []catalog.ID{"usaco:84", "usaco:1059", "usaco:552"} {
		wg.Go(func() {
			if _, err := c.Fetch(context.Background(), id); err != nil {
				t.Errorf("%s: %v", id, err)
			}
		})
	}
	wg.Wait()
}

// An interactive problem has a statement and maybe an analysis, but no
// tests, and its zip is not even requested.
func TestFetchInteractive(t *testing.T) {
	t.Parallel()
	fake, srv := newFakeUSACO(t)
	c := testCatalog(t, t.TempDir(), srv)
	a, err := c.Fetch(context.Background(), "usaco:552")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Tests) != 0 || len(a.Samples) != 0 || a.Editorial != "" {
		t.Errorf("assets = %+v", a)
	}
	if n := fake.count("/current/data/googol.zip"); n != 0 {
		t.Errorf("zip requested %d times", n)
	}
	if n := fake.count("/current/data/sol_googol.html"); n != 1 {
		t.Errorf("analysis requested %d times, want 1 (a 404 is cached as none)", n)
	}
}

// A fetch refines an entry from its statement and persists it: cpid 9999
// is served Moo Network's page (4 s limit) and has no zip, so it stops
// being judgeable, also for the next Open.
func TestFetchRefinesIndex(t *testing.T) {
	t.Parallel()
	_, srv := newFakeUSACO(t)
	root := t.TempDir()
	c := testCatalog(t, root, srv)
	if p, _ := c.Get("usaco:9999"); !p.Judgeable || p.TimeLimit != 2*time.Second {
		t.Fatalf("before fetch: %+v", p)
	}
	if _, err := c.Fetch(context.Background(), "usaco:9999"); err != nil {
		t.Fatal(err)
	}
	for _, cat := range []*Catalog{c, testCatalog(t, root, srv)} {
		p, _ := cat.Get("usaco:9999")
		if p.Title != "Moo Network" || p.TimeLimit != 4*time.Second || p.Judgeable || p.HasEditorial {
			t.Errorf("after fetch: %+v", p)
		}
	}
}

func TestFetchErrors(t *testing.T) {
	t.Parallel()
	_, srv := newFakeUSACO(t)
	root := t.TempDir()
	c := testCatalog(t, root, srv)
	if _, err := c.Fetch(context.Background(), "usaco:1"); err == nil {
		t.Error("fetch of an unknown id succeeded")
	}
	// cpid 1060's statement is not served: the fetch fails and leaves no
	// dir behind, so the next call retries.
	if _, err := c.Fetch(context.Background(), "usaco:1060"); err == nil {
		t.Error("fetch with a missing statement succeeded")
	}
	if _, err := os.Stat(filepath.Join(root, "1060")); err == nil {
		t.Error("failed fetch left a problem dir")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Fetch(ctx, "usaco:1059"); err == nil {
		t.Error("fetch with a cancelled context succeeded")
	}
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUnzipTests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files map[string]string
		want  int    // tests extracted
		err   string // substring; "" for success
	}{
		{"N.in/N.out", map[string]string{"1.in": "a", "1.out": "b", "2.in": "c", "2.out": "d"}, 2, ""},
		{"I.N/O.N", map[string]string{"I.1": "a", "O.1": "b", "I.10": "c", "O.10": "d"}, 2, ""},
		{"in a folder, with junk", map[string]string{
			"paint/1.in": "a", "paint/1.out": "b", "__MACOSX/paint/._1.in": "x", "readme.txt": "x",
		}, 1, ""},
		{"zip slip names are reduced to the base name", map[string]string{
			"../../1.in": "a", "../../1.out": "b",
		}, 1, ""},
		{"absolute names too", map[string]string{"/etc/1.in": "a", `C:\x\1.out`: "b"}, 1, ""},
		{"duplicate after normalizing", map[string]string{"1.in": "a", "I.1": "b", "1.out": "c"}, 0, "two entries"},
		{"input without output", map[string]string{"1.in": "a"}, 0, "without"},
		{"output without input", map[string]string{"1.in": "a", "1.out": "b", "2.out": "c"}, 0, "outputs without inputs"},
		{"no tests", map[string]string{"readme.txt": "x"}, 0, "no tests"},
		{"test zero is not a test", map[string]string{"0.in": "a", "0.out": "b"}, 0, "no tests"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			dst := filepath.Join(parent, "tests")
			os.MkdirAll(dst, 0o755)
			zb := zipOf(t, tc.files)
			n, err := unzipTests(bytes.NewReader(zb), int64(len(zb)), dst)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil || n != tc.want {
				t.Fatalf("unzipTests = %d, %v; want %d", n, err, tc.want)
			}
			// Nothing may land outside dst.
			ents, _ := os.ReadDir(parent)
			if len(ents) != 1 {
				t.Errorf("wrote outside tests/: %v", ents)
			}
			files, _ := os.ReadDir(dst)
			if len(files) != 2*tc.want {
				t.Errorf("tests/ has %d files, want %d", len(files), 2*tc.want)
			}
		})
	}
}

func TestCopyEntryCap(t *testing.T) {
	t.Parallel()
	zb := zipOf(t, map[string]string{"1.in": strings.Repeat("x", 100)})
	zr, err := zip.NewReader(bytes.NewReader(zb), int64(len(zb)))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "1.in")
	if _, err := copyEntry(zr.File[0], dst, 99); err == nil || !strings.Contains(err.Error(), "over 99 bytes") {
		t.Errorf("err = %v, want a size error", err)
	}
	if _, err := copyEntry(zr.File[0], filepath.Join(t.TempDir(), "1.in"), 100); err != nil {
		t.Errorf("exact size: %v", err)
	}
}
