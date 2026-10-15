// Package fixture is the data the RTFM screens' golden tests and
// cmd/demo-rtfm render: internal/curriculum's fixture snapshot of the USACO
// Guide (seven real modules, at least one per division), a player holding
// one capability, and a small hand-built catalog for problem titles and
// ratings. What a human eyeballs in the demo is what the goldens pin.
package fixture

import (
	"path/filepath"
	"runtime"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
)

// GuideDir is internal/curriculum/testdata/guide, found from this file so tests
// and `go run` work from any directory.
func GuideDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "curriculum", "testdata", "guide")
}

// Curriculum loads the fixture snapshot.
func Curriculum() (*curriculum.Curriculum, error) { return curriculum.Load(GuideDir()) }

// Held is the topic whose capability the fixture player holds: Two
// Pointers passed, Prefix Sums not yet.
const Held = "two-pointers"

// Meta is the fixture player. The capability name comes from
// curriculum.CapabilityFor, so the fixture follows the name table rather than
// pinning a spelling.
func Meta() game.Meta {
	return game.Meta{Rating: 1432, Capabilities: []string{curriculum.CapabilityFor(Held)}}
}

// Judgeable accepts USACO problems only, as the catalog does until CSES
// test data is available (DESIGN.md "Content"); modules that lean on CSES
// get short units, which the screens show as topped up.
func Judgeable(id catalog.ID) bool { return id.Source() == catalog.USACO }

// problems are the fixture catalog. Titles and contests are USACO's;
// ratings are illustrative estimates for layout, not the catalog's.
var problems = map[catalog.ID]catalog.Problem{
	"usaco:572":  {Title: "Breed Counting", Contest: "2015 December, Silver P3", Rating: 1250},
	"usaco:691":  {Title: "Hoof, Paper, Scissors", Contest: "2017 January, Silver P2", Rating: 1300},
	"usaco:595":  {Title: "Subsequences Summing to Sevens", Contest: "2016 January, Silver P2", Rating: 1450},
	"usaco:1471": {Title: "Farmer John's Favorite Operation", Rating: 1600},
	"usaco:738":  {Title: "Paired Up", Contest: "2017 US Open, Silver P1", Rating: 1350},
	"usaco:643":  {Title: "Diamond Collector", Contest: "2016 US Open, Silver P2", Rating: 1500},
}

// Lookup resolves fixture problems.
func Lookup(id catalog.ID) (catalog.Problem, bool) {
	p, ok := problems[id]
	p.ID = id
	return p, ok
}
