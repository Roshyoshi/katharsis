package usaco

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
)

// TestNetwork opens the real catalog (downloading the guide snapshot on
// first run, about 150 MB) and fetches three problems end to end into the
// user cache: a Nov 2011 problem with I.N/O.N zip names, a file-I/O era
// HTML statement, and a stdin problem with a 4 s limit. Opt in with
// KATHARSIS_NET=1; -v prints index statistics.
func TestNetwork(t *testing.T) {
	if os.Getenv("KATHARSIS_NET") != "1" {
		t.Skip("set KATHARSIS_NET=1 to use the network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c, err := Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	logStats(t, c.All())

	for _, tc := range []struct {
		id     catalog.ID
		fileIO string
		limit  time.Duration
	}{
		{"usaco:84", "ctiming", 2 * time.Second},
		{"usaco:567", "paint", 2 * time.Second},
		{"usaco:1211", "", 4 * time.Second},
	} {
		start := time.Now()
		a, err := c.Fetch(ctx, tc.id)
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		p, _ := c.Get(tc.id)
		if p.FileIO != tc.fileIO || p.TimeLimit != tc.limit || !p.Judgeable {
			t.Errorf("%s: %+v", tc.id, p)
		}
		if len(a.Tests) == 0 || len(a.Samples) == 0 || a.Editorial == "" {
			t.Errorf("%s: assets %+v", tc.id, a)
		}
		// USACO's test 1 is the sample; a mismatch would mean the zip and
		// the statement disagree.
		got, _ := os.ReadFile(a.Tests[0].In)
		want, _ := os.ReadFile(a.Samples[0].In)
		if squash(string(got)) != squash(string(want)) {
			t.Errorf("%s: tests/1.in differs from the sample", tc.id)
		}
		dir := filepath.Dir(a.Statement)
		t.Logf("%s %q: %d samples, %d tests, %s on disk, %v",
			tc.id, p.Title, len(a.Samples), len(a.Tests), human(dirSize(dir)), time.Since(start).Round(time.Millisecond))
	}
}

func dirSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
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

func logStats(t *testing.T, all []catalog.Problem) {
	count := func(f func(catalog.Problem) bool) int {
		n := 0
		for _, p := range all {
			if f(p) {
				n++
			}
		}
		return n
	}
	byKey := func(key func(catalog.Problem) string) string {
		m := map[string]int{}
		for _, p := range all {
			m[key(p)]++
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
	t.Logf("problems: %d", len(all))
	t.Logf("division:%s", byKey(func(p catalog.Problem) string {
		cpid, _ := strconv.Atoi(strings.TrimPrefix(string(p.ID), "usaco:"))
		return eraDivision(cpid, p.Division)
	}))
	t.Logf("judgeable=%d interactive=%d needs-checker=%d float=%d no-zip=%d",
		count(func(p catalog.Problem) bool { return p.Judgeable }),
		count(func(p catalog.Problem) bool { return p.Interactive }),
		count(func(p catalog.Problem) bool { return p.NeedsChecker }),
		count(func(p catalog.Problem) bool { return p.FloatTolerance > 0 }),
		count(func(p catalog.Problem) bool { return !p.Judgeable && !p.Interactive && !p.NeedsChecker }))
	t.Logf("file-io=%d stdin=%d", count(func(p catalog.Problem) bool { return p.FileIO != "" }),
		count(func(p catalog.Problem) bool { return p.FileIO == "" }))
	t.Logf("tagged=%d topic!=mixed=%d", count(func(p catalog.Problem) bool { return len(p.Tags) > 0 }),
		count(func(p catalog.Problem) bool { return p.Topic != "mixed" }))
	t.Logf("topic:%s", byKey(func(p catalog.Problem) string { return p.Topic }))
	t.Logf("rating source:%s", byKey(func(p catalog.Problem) string { return p.RatingSource }))
	t.Logf("rating (100s):%s", byKey(func(p catalog.Problem) string { return fmt.Sprintf("%04d", p.Rating/100*100) }))
	t.Logf("non-default time=%d mem=%d",
		count(func(p catalog.Problem) bool { return p.TimeLimit != 2*time.Second }),
		count(func(p catalog.Problem) bool { return p.MemLimit != 256<<20 }))
}
