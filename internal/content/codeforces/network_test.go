package codeforces

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
)

// TestNetwork opens the real catalog (downloading about 17 MB of the
// problem table on first run) and fetches three problems end to end into
// the user cache: one with complete official tests, one whose tests are
// generated, and one with an editorial (generated tests too). Opt in with
// KATHARSIS_NET=1; -v prints index statistics and fetch costs.
func TestNetwork(t *testing.T) {
	if os.Getenv("KATHARSIS_NET") != "1" {
		t.Skip("set KATHARSIS_NET=1 to use the network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	start := time.Now()
	c, err := Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("open: %v", time.Since(start).Round(time.Millisecond))
	logStats(t, c)

	for _, tc := range []struct {
		id        catalog.ID
		official  bool // all tests official, none generated
		editorial bool
		limit     time.Duration
	}{
		{"cf:4A", true, false, time.Second},
		{"cf:1899C", false, false, time.Second},
		{"cf:1004A", false, true, time.Second},
	} {
		os.RemoveAll(filepath.Join(c.root, dirName(mustEntry(t, c, tc.id)))) // time a cold fetch
		start := time.Now()
		a, err := c.Fetch(ctx, tc.id)
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		cold := time.Since(start)
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		p, _ := c.Get(tc.id)
		e := mustEntry(t, c, tc.id)
		if !p.Judgeable || p.TimeLimit != tc.limit || p.RatingSource != "cf" {
			t.Errorf("%s: %+v", tc.id, p)
		}
		if len(a.Samples) == 0 || len(a.Tests) == 0 || (a.Editorial != "") != tc.editorial {
			t.Errorf("%s: assets %+v", tc.id, a)
		}
		if tc.official && len(a.Tests) != e.TestsetSize {
			t.Errorf("%s: %d tests, Codeforces has %d", tc.id, len(a.Tests), e.TestsetSize)
		}
		if !tc.official && len(a.Tests) < e.GenTests {
			t.Errorf("%s: %d tests, want at least the %d generated", tc.id, len(a.Tests), e.GenTests)
		}
		// Codeforces' test 1 is the first sample, which checks that the
		// row found is the right problem.
		if tc.official {
			got, _ := os.ReadFile(a.Tests[0].In)
			want, _ := os.ReadFile(a.Samples[0].In)
			if strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
				t.Errorf("%s: tests/1.in differs from the sample", tc.id)
			}
		}
		again := time.Now()
		if _, err := c.Fetch(ctx, tc.id); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Dir(a.Statement)
		t.Logf("%s %q: %d samples, %d tests, %s on disk, contest file %s, cold %v, warm %v, heap sys %s",
			tc.id, p.Title, len(a.Samples), len(a.Tests), human(dirSize(dir)), human(e.GenBytes),
			cold.Round(time.Millisecond), time.Since(again).Round(time.Microsecond), human(int64(ms.HeapSys)))
	}
}

func mustEntry(t *testing.T, c *Catalog, id catalog.ID) entry {
	t.Helper()
	e, ok := c.entry(id)
	if !ok {
		t.Fatalf("%s not in the catalog", id)
	}
	return e
}

func dirSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

func human(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func logStats(t *testing.T, c *Catalog) {
	c.mu.RLock()
	var es []entry
	for _, id := range c.order {
		es = append(es, *c.entries[id])
	}
	c.mu.RUnlock()
	count := func(f func(entry) bool) int {
		n := 0
		for _, e := range es {
			if f(e) {
				n++
			}
		}
		return n
	}
	byKey := func(f func(entry) bool, key func(entry) string) string {
		m := map[string]int{}
		for _, e := range es {
			if f(e) {
				m[key(e)]++
			}
		}
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			fmt.Fprintf(&b, " %s=%d", k, m[k])
		}
		return b.String()
	}
	all := func(entry) bool { return true }
	judge := func(e entry) bool { return e.Judgeable }
	t.Logf("problems=%d rated=%d judgeable=%d editorial=%d (judgeable %d)", len(es),
		count(func(e entry) bool { return e.Rating > 0 }), count(judge),
		count(func(e entry) bool { return e.HasEditorial }),
		count(func(e entry) bool { return e.Judgeable && e.HasEditorial }))
	t.Logf("skip:%s", byKey(all, func(e entry) string { return cmpOr(e.Skip, "ok") }))
	t.Logf("tests (judgeable): official=%d generated=%d",
		count(func(e entry) bool { return e.Judgeable && e.Complete }),
		count(func(e entry) bool { return e.Judgeable && !e.Complete }))
	t.Logf("checker: has=%d needs=%d float-only=%d; float tolerance=%d (judgeable %d); any-case=%d (judgeable %d)",
		count(func(e entry) bool { return e.HasChecker }), count(func(e entry) bool { return e.NeedsChecker }),
		count(func(e entry) bool { return e.HasChecker && !e.NeedsChecker }),
		count(func(e entry) bool { return e.FloatTolerance > 0 }),
		count(func(e entry) bool { return e.Judgeable && e.FloatTolerance > 0 }),
		count(func(e entry) bool { return e.AnyCase }),
		count(func(e entry) bool { return e.Judgeable && e.AnyCase && !e.HasChecker }))
	t.Logf("input mode:%s", byKey(all, func(e entry) string { return e.InputMode }))
	t.Logf("interactive=%d", count(func(e entry) bool { return e.Interactive }))
	t.Logf("rating (judgeable, 100s):%s", byKey(judge, func(e entry) string { return fmt.Sprintf("%04d", e.Rating/100*100) }))
	t.Logf("topic (judgeable):%s", byKey(judge, func(e entry) string { return e.Topic }))
	t.Logf("non-default (judgeable) time=%d mem=%d",
		count(func(e entry) bool { return e.Judgeable && e.TimeLimit != 2*time.Second }),
		count(func(e entry) bool { return e.Judgeable && e.MemLimit != 256<<20 }))
	fi, _ := os.Stat(filepath.Join(c.root, "index.json"))
	ti, _ := os.Stat(filepath.Join(c.root, "text.bin"))
	if fi != nil && ti != nil {
		t.Logf("cache: index.json %s, text.bin %s", human(fi.Size()), human(ti.Size()))
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
