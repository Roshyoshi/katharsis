package codeforces

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/web"
	"katharsis/internal/problem"
)

func readTests(t *testing.T, ts []problem.Test) [][2]string {
	t.Helper()
	var out [][2]string
	for _, tt := range ts {
		in, err := os.ReadFile(tt.In)
		if err != nil {
			t.Fatal(err)
		}
		o, err := os.ReadFile(tt.Out)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, [2]string{string(in), string(o)})
	}
	return out
}

func TestFetchOfficial(t *testing.T) {
	t.Parallel()
	c, h := openFixture(t)
	ctx := context.Background()
	vfile := "verifiable_tests/train-00000-of-00002.parquet"
	before := h.count(vfile)

	// Eight concurrent callers share one download.
	var wg sync.WaitGroup
	results := make([]catalog.Assets, 8)
	errs := make([]error, 8)
	for i := range results {
		wg.Go(func() { results[i], errs[i] = c.Fetch(ctx, "cf:4A") })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	// Footer, then id and official_tests (one request here: the file is
	// smaller than the footer guess). 4/A is row 1 of the file, not the
	// computed row 0, so this also covers the id search.
	if n := h.count(vfile) - before; n < 1 || n > 3 {
		t.Errorf("%s: %d requests, want 1-3", vfile, n)
	}
	a := results[0]
	dir := filepath.Join(c.root, "4A")
	if a.Statement != filepath.Join(dir, "statement.md") || a.Editorial != "" {
		t.Errorf("assets = %+v", a)
	}
	// CRLF from Codeforces becomes LF.
	want := [][2]string{{"8\n", "YES\n"}, {"5\n", "NO\n"}, {"4\n", "YES\n"}}
	if got := readTests(t, a.Tests); !slices.Equal(got, want) {
		t.Errorf("tests = %q, want %q", got, want)
	}
	if got := readTests(t, a.Samples); !slices.Equal(got, [][2]string{{"8\n", "YES\n"}}) {
		t.Errorf("samples = %q", got)
	}
	if b, _ := os.ReadFile(a.Statement); !strings.HasPrefix(string(b), "# Watermelon\n") {
		t.Errorf("statement.md starts %q", b[:min(40, len(b))])
	}

	// A finished dir needs no network.
	h.setFail(true)
	n := h.total()
	if _, err := c.Fetch(ctx, "cf:4A"); err != nil {
		t.Fatal(err)
	}
	if h.total() != n {
		t.Error("second Fetch touched the network")
	}
	h.setFail(false)

	// A tampered dir is fetched again.
	if err := os.Remove(a.Tests[1].Out); err != nil {
		t.Fatal(err)
	}
	if a2, err := c.Fetch(ctx, "cf:4A"); err != nil || len(a2.Tests) != 3 {
		t.Errorf("refetch after tampering: %v, %d tests", err, len(a2.Tests))
	}
	assertNoDebris(t, c.root)
}

// A complete dir written under another revision or schema used to be
// served as is, so a bump never reached problems already in the cache.
func TestFetchRefetchesOtherBuild(t *testing.T) {
	t.Parallel()
	c, h := openFixture(t)
	a, err := c.Fetch(t.Context(), "cf:4A")
	if err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(filepath.Dir(a.Statement), "meta.json")
	b, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	var m meta
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	m.Key = "rev=older schema=1"
	if err := os.WriteFile(metaPath, mustJSON(t, m), 0o644); err != nil {
		t.Fatal(err)
	}
	before := h.total()
	if _, err := c.Fetch(t.Context(), "cf:4A"); err != nil {
		t.Fatal(err)
	}
	if h.total() == before {
		t.Error("a dir from another build was served without a fetch")
	}
	if b, _ := os.ReadFile(metaPath); json.Unmarshal(b, &m) != nil || m.Key != dirKey {
		t.Errorf("meta key after refetch = %q, want %q", m.Key, dirKey)
	}
	assertNoDebris(t, c.root)
}

func TestFetchGenerated(t *testing.T) {
	t.Parallel()
	c, h := openFixture(t)
	a, err := c.Fetch(context.Background(), "cf:1899C")
	if err != nil {
		t.Fatal(err)
	}
	// The official test first, then the generated ones in test_i order,
	// normalized; 1899/B's rows are not here.
	want := [][2]string{
		{"1899/C test 1\n", "1\n"},
		{"c1 in\n", "c1 out\n"},
		{"c2 in\n", "c2 out\n"},
		{"c3 in\n", "c3 out\n"},
	}
	if got := readTests(t, a.Tests); !slices.Equal(got, want) {
		t.Errorf("tests = %q, want %q", got, want)
	}
	if a.Editorial == "" {
		t.Error("no editorial")
	}
	if n := h.count(genFile(1899)); n != 1 {
		t.Errorf("contest file fetched %d times", n)
	}
	if rs := h.rangesOf(genFile(1899)); len(rs) != 1 || rs[0] != "" {
		t.Errorf("contest file requests %q, want one whole GET", rs)
	}
	assertNoDebris(t, c.root)
}

// 2028/B's files are parquet-go's own layout, which streamColumn hands to
// the parquet-go fallbacks.
func TestFetchFallback(t *testing.T) {
	t.Parallel()
	c, _ := openFixture(t)
	a, err := c.Fetch(context.Background(), "cf:2028B")
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]string{
		{"2028/B test 1\n", "1\n"},
		{"2028/B test 2\n", "2\n"},
		{"7\n10 1 0\n", "0\n"},
	}
	if got := readTests(t, a.Tests); !slices.Equal(got, want) {
		t.Errorf("tests = %q, want %q", got, want)
	}
}

func TestFetchMissingTests(t *testing.T) {
	t.Parallel()
	c, h := openFixture(t)
	_, err := c.Fetch(context.Background(), "cf:1007A")
	if !errors.Is(err, errTestsMissing) {
		t.Fatalf("err = %v, want errTestsMissing", err)
	}
	p, _ := c.Get("cf:1007A")
	if p.Judgeable {
		t.Error("a problem without tests is still judgeable")
	}
	if _, err := os.Stat(filepath.Join(c.root, "1007A")); err == nil {
		t.Error("a failed fetch left a problem dir")
	}
	// The verdict survives a restart.
	c2, err := open(t.Context(), c.root, h.srv.URL, fixtureLayout, testClient(h))
	if err != nil {
		t.Fatal(err)
	}
	if e, _ := c2.entry("cf:1007A"); e.Judgeable || e.Skip != skipTestsMissing {
		t.Errorf("after reopen: %+v", e)
	}
	assertNoDebris(t, c.root)
}

// A problem that is not judgeable still gets its statement and samples,
// with no network for tests.
func TestFetchNotJudgeable(t *testing.T) {
	t.Parallel()
	c, h := openFixture(t)
	n := h.total()
	a, err := c.Fetch(context.Background(), "cf:1520E")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Tests) != 0 || len(a.Samples) != 1 || a.Editorial == "" {
		t.Errorf("assets = %+v", a)
	}
	if h.total() != n {
		t.Error("fetching an unjudgeable problem touched the network")
	}
}

// A dir fetched while a problem was not judgeable has no tests. If a rule
// change (a new schema) makes the problem judgeable, the next Fetch must
// get its tests rather than serve the empty dir.
func TestFetchAfterBecomingJudgeable(t *testing.T) {
	t.Parallel()
	c, _ := openFixture(t)
	c.mu.Lock()
	e := c.entries["cf:919E"]
	e.Judgeable, e.Skip = false, skipChecker
	c.mu.Unlock()
	a, err := c.Fetch(context.Background(), "cf:919E")
	if err != nil || len(a.Tests) != 0 {
		t.Fatalf("unjudgeable fetch: %v, %d tests", err, len(a.Tests))
	}
	c.mu.Lock()
	e.Judgeable, e.Skip = true, ""
	c.mu.Unlock()
	a, err = c.Fetch(context.Background(), "cf:919E")
	if err != nil || len(a.Tests) != 3 {
		t.Errorf("judgeable fetch over the old dir: %v, %d tests, want 3", err, len(a.Tests))
	}
	assertNoDebris(t, c.root)
}

func TestFetchErrors(t *testing.T) {
	t.Parallel()
	c, h := openFixture(t)
	if _, err := c.Fetch(context.Background(), "cf:12345Z"); err == nil {
		t.Error("unknown id fetched")
	}
	// A server error keeps nothing, so the next Fetch retries.
	h.setFail(true)
	if _, err := c.Fetch(context.Background(), "cf:919E"); err == nil {
		t.Fatal("fetch with the server down succeeded")
	}
	h.setFail(false)
	a, err := c.Fetch(context.Background(), "cf:919E")
	if err != nil || len(a.Tests) != 3 || a.Editorial == "" {
		t.Errorf("retry: %v, %+v", err, a)
	}
	if p, _ := c.Get("cf:919E"); !p.Judgeable {
		t.Error("a server error made the problem unjudgeable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Fetch(ctx, "cf:1791A"); !isCtxErr(err) {
		t.Errorf("cancelled fetch: %v", err)
	}
	assertNoDebris(t, c.root)
}

// assertNoDebris checks that no temp or download dir is left behind.
func assertNoDebris(t *testing.T, root string) {
	t.Helper()
	des, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range des {
		if strings.HasPrefix(d.Name(), ".") {
			t.Errorf("left behind: %s", d.Name())
		}
	}
}

// 1899/B is missing from its verifiable_tests file; its generated tests
// are the whole set, so the fetch goes on without official ones.
func TestFetchGeneratedOnly(t *testing.T) {
	t.Parallel()
	c, _ := openFixture(t)
	a, err := c.Fetch(context.Background(), "cf:1899B")
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]string{{"b1\n", "B1\n"}, {"b2\n", "B2\n"}}
	if got := readTests(t, a.Tests); !slices.Equal(got, want) {
		t.Errorf("tests = %q, want %q", got, want)
	}
	if p, _ := c.Get("cf:1899B"); !p.Judgeable {
		t.Error("missing official tests made a generated-test problem unjudgeable")
	}
}

// A fetch killed mid-download leaves its temp and download dirs behind;
// Open removes the old ones and nothing else.
func TestOpenCleansDebris(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	old := time.Now().Add(-web.DebrisAge - time.Hour)
	for _, name := range []string{".dl-1", ".tmp-1899C-2", ".tmp-text.bin-3", "4A", ".dl-new"} {
		p := filepath.Join(root, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if name != ".dl-new" {
			if err := os.Chtimes(p, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	h := newFakeHF(t, newFixture(t))
	if _, err := open(t.Context(), root, h.srv.URL, fixtureLayout, testClient(h)); err != nil {
		t.Fatal(err)
	}
	des, _ := os.ReadDir(root)
	var left []string
	for _, d := range des {
		if !d.IsDir() {
			continue // index.json, text.bin
		}
		left = append(left, d.Name())
	}
	if want := []string{".dl-new", "4A"}; !slices.Equal(left, want) {
		t.Errorf("left %v, want %v", left, want)
	}
}
