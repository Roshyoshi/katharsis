package guide

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"katharsis/internal/content/web"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

type entry struct {
	name string
	body string
	typ  byte // tar.TypeReg when 0
	link string
}

// tarball builds a codeload-shaped archive: a pax global header, then
// entries under "usaco-guide-<sha>/".
func tarball(t *testing.T, ents []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(tw.WriteHeader(&tar.Header{
		Typeflag:   tar.TypeXGlobalHeader,
		Name:       "pax_global_header",
		PAXRecords: map[string]string{"comment": testCommit},
		Format:     tar.FormatPAX,
	}))
	for _, e := range ents {
		h := &tar.Header{Name: e.name, Mode: 0o644, Typeflag: e.typ, Linkname: e.link}
		if h.Typeflag == 0 {
			h.Typeflag = tar.TypeReg
			h.Size = int64(len(e.body))
		}
		must(tw.WriteHeader(h))
		if h.Typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			must(err)
		}
	}
	must(tw.Close())
	must(zw.Close())
	return buf.Bytes()
}

const pfx = "usaco-guide-" + testCommit + "/"

var good = []entry{
	{name: pfx, typ: tar.TypeDir},
	{name: pfx + "LICENSE", body: "CC BY-NC-SA 4.0"},
	{name: pfx + "README.md", body: "skip me"},
	{name: pfx + "content/ordering.ts", body: "export default {}"},
	{name: pfx + "content/3_Silver/Prefix_Sums.mdx", body: "# Prefix Sums"},
	{name: pfx + "content/3_Silver/Prefix_Sums.problems.json", body: "{}"},
	{name: pfx + "solutions/silver/usaco-615.mdx", body: "sol"},
	{name: pfx + "public/assets/big.png", body: "png"},
	{name: pfx + "src/components/markdown/ProblemsList/DivisionList/div_to_probs.json", body: "{}"},
	{name: pfx + "src/components/markdown/ProblemsList/DivisionList/id_to_sol.json", body: "{}"},
	{name: pfx + "src/components/markdown/ProblemsList/DivisionList/DivisionList.tsx", body: "tsx"},
	{name: pfx + "src/components/markdown/ProblemsList/DivisionList/scripts/x.json", body: "{}"},
}

// fast is the guide's client with test-sized waits.
func fast(srv *httptest.Server) *web.Client {
	return &web.Client{HTTP: srv.Client(), Retries: 2, Backoff: time.Millisecond, Idle: 100 * time.Millisecond}
}

func serve(t *testing.T, body []byte, fail int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if int(hits.Add(1)) <= fail {
			http.Error(w, "busy", http.StatusBadGateway)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestSnapshotLayout(t *testing.T) {
	root := t.TempDir()
	srv, hits := serve(t, tarball(t, good), 1) // first try is a 502
	dir, err := snapshot(context.Background(), fast(srv), root, srv.URL, testCommit)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, testCommit); dir != want {
		t.Errorf("dir = %s, want %s", dir, want)
	}
	tests := []struct {
		rel     string
		present bool
	}{
		{"LICENSE", true},
		{"content/ordering.ts", true},
		{"content/3_Silver/Prefix_Sums.mdx", true},
		{"content/3_Silver/Prefix_Sums.problems.json", true},
		{"solutions/silver/usaco-615.mdx", true},
		{DivisionList + "/div_to_probs.json", true},
		{DivisionList + "/id_to_sol.json", true},
		{"README.md", false},
		{"public/assets/big.png", false},
		{DivisionList + "/DivisionList.tsx", false},
		{DivisionList + "/scripts/x.json", false},
	}
	for _, tc := range tests {
		_, err := os.Stat(filepath.Join(dir, tc.rel))
		if got := err == nil; got != tc.present {
			t.Errorf("%s present = %v, want %v", tc.rel, got, tc.present)
		}
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("requests = %d, want 2 (one 502, one retry)", n)
	}

	// Idempotent: the published dir is reused without the network.
	srv.Close()
	again, err := snapshot(context.Background(), fast(srv), root, srv.URL, testCommit)
	if err != nil || again != dir {
		t.Fatalf("second snapshot = %q, %v; want %q", again, err, dir)
	}
	leftovers, _ := filepath.Glob(filepath.Join(root, ".tmp-*"))
	if len(leftovers) > 0 {
		t.Errorf("temp dirs left behind: %v", leftovers)
	}
}

func TestSnapshotConcurrent(t *testing.T) {
	root := t.TempDir()
	srv, hits := serve(t, tarball(t, good), 0)
	var wg sync.WaitGroup
	dirs := make([]string, 8)
	errs := make([]error, 8)
	for i := range dirs {
		wg.Go(func() {
			dirs[i], errs[i] = snapshot(context.Background(), fast(srv), root, srv.URL, testCommit)
		})
	}
	wg.Wait()
	for i := range dirs {
		if errs[i] != nil || dirs[i] != dirs[0] {
			t.Errorf("caller %d: %q, %v", i, dirs[i], errs[i])
		}
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestSnapshotRejects(t *testing.T) {
	tables := []entry{
		{name: pfx + DivisionList + "/div_to_probs.json", body: "{}"},
		{name: pfx + DivisionList + "/id_to_sol.json", body: "{}"},
	}
	tests := []struct {
		name string
		ents []entry
		want string
	}{
		{"zip slip", []entry{{name: pfx + "content/../../evil", body: "x"}}, "path"},
		{"absolute", []entry{{name: "/etc/passwd", body: "x"}}, "outside"},
		{"other commit", []entry{{name: "usaco-guide-deadbeef/content/a.mdx", body: "x"}}, "outside"},
		{"symlink", []entry{{name: pfx + "content/link", typ: tar.TypeSymlink, link: "/etc/passwd"}}, "type"},
		{"hardlink", []entry{{name: pfx + "content/link", typ: tar.TypeLink, link: pfx + "LICENSE"}}, "type"},
		{"huge file", []entry{{name: pfx + "content/big.mdx", body: strings.Repeat("x", maxFile+1)}}, "bytes"},
		{"no tables", []entry{{name: pfx + "content/ordering.ts", body: "x"}}, "missing"},
		{"nothing wanted", []entry{{name: pfx + "README.md", body: "x"}}, "no guide files"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			ents := tc.ents
			if tc.want != "missing" && tc.want != "no guide files" {
				ents = append(append([]entry{}, tables...), ents...)
			}
			srv, _ := serve(t, tarball(t, ents), 0)
			_, err := snapshot(context.Background(), fast(srv), root, srv.URL, testCommit)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
			left, _ := os.ReadDir(root)
			if len(left) > 0 {
				t.Errorf("root not clean after failure: %v", left)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(root), "evil")); err == nil {
				t.Error("zip slip wrote outside the root")
			}
		})
	}
}

// TestSnapshotNetwork downloads the real pinned snapshot into the user cache.
// Opt in with KATHARSIS_NET=1; it moves about 150 MB once.
func TestSnapshotNetwork(t *testing.T) {
	if os.Getenv("KATHARSIS_NET") != "1" {
		t.Skip("set KATHARSIS_NET=1 to download the pinned USACO Guide")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	dir, err := Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"LICENSE",
		"content/ordering.ts",
		"content/extraProblems.json",
		DivisionList + "/div_to_probs.json",
		DivisionList + "/id_to_sol.json",
		DivisionList + "/contest_to_points.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Error(err)
		}
	}
	mdx, _ := filepath.Glob(filepath.Join(dir, "content", "*", "*.mdx"))
	if len(mdx) < 100 {
		t.Errorf("only %d module .mdx files", len(mdx))
	}
}

// A download killed part way leaves its temp dir; a later Snapshot removes
// it once it is old, even when the snapshot itself is already there.
func TestSnapshotCleansDebris(t *testing.T) {
	root := t.TempDir()
	srv, hits := serve(t, tarball(t, good), 0)
	if _, err := snapshot(context.Background(), fast(srv), root, srv.URL, testCommit); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-web.DebrisAge - time.Hour)
	for _, name := range []string{".tmp-" + testCommit + "-killed", ".tmp-" + testCommit + "-running"} {
		p := filepath.Join(root, name, "content")
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(name, "killed") {
			if err := os.Chtimes(filepath.Dir(p), old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := snapshot(context.Background(), fast(srv), root, srv.URL, testCommit); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(root, ".tmp-*"))
	if len(left) != 1 || !strings.HasSuffix(left[0], "running") {
		t.Errorf("temp dirs left: %v, want only the young one", left)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

// A Commit bump leaves the old snapshot behind, up to maxExtracted of it;
// the next Snapshot removes it. Only commit-named dirs go.
func TestSnapshotSweepsOldCommits(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, strings.Repeat("9", 40))
	other := filepath.Join(root, "notes")
	for _, d := range []string{filepath.Join(old, "content"), other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	srv, _ := serve(t, tarball(t, good), 0)
	dir, err := snapshot(context.Background(), fast(srv), root, srv.URL, testCommit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("the old commit's snapshot is still there")
	}
	for _, keep := range []string{dir, other} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s: %v", keep, err)
		}
	}
}
