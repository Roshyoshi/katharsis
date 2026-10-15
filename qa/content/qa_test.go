//go:build qa

// Package content is a QA harness for the problem data the game serves:
// statements, samples, tests, limits and the judge settings derived from
// them. It never builds into the game (build tag qa) and goes through
// each source's own Fetch, which downloads into the cache what it lacks.
// To keep the player's cache as it is, run with HOME at a scratch dir
// whose Library/Caches/katharsis is a copy (cp -cR) of the real one.
//
//	go test -tags qa ./qa/content/ -run TestAudit -v
//	go test -tags qa ./qa/content/ -run TestSolve -v
//
// QA_ONLY=usaco:84,cf:4A narrows either test to those ids; "usaco:" or
// "cf:" selects a whole source.
package content

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/codeforces"
	"katharsis/internal/content/usaco"
)

var (
	catOnce sync.Once
	cat     *catalog.Union
	catErr  error
)

// union opens both real catalogs (cached indexes; network only on a cold
// cache) once per test binary.
func union(t *testing.T) *catalog.Union {
	t.Helper()
	catOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		u, err := usaco.Open(ctx)
		if err != nil {
			catErr = err
			return
		}
		c, err := codeforces.Open(ctx)
		if err != nil {
			catErr = err
			return
		}
		cat = catalog.NewUnion(u, c)
	})
	if catErr != nil {
		t.Fatal(catErr)
	}
	return cat
}

// only reports whether id passes the QA_ONLY filter.
func only(id catalog.ID) bool {
	f := os.Getenv("QA_ONLY")
	if f == "" {
		return true
	}
	for _, s := range strings.Split(f, ",") {
		s = strings.TrimSpace(s)
		if s == string(id) || strings.HasSuffix(s, ":") && strings.HasPrefix(string(id), s) {
			return true
		}
	}
	return false
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

// norm is whitespace-insensitive content: tokens joined by one space.
func norm(s string) string { return strings.Join(strings.Fields(s), " ") }
