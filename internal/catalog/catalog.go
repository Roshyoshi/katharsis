// Package catalog is the shared vocabulary for problems the game can serve.
// Content sources (USACO now; CSES and Codeforces later) produce Problems;
// the engine picks doors from them; the app materializes the chosen one.
package catalog

import (
	"context"
	"time"

	"deadcases/internal/problem"
)

type Source string

const (
	USACO Source = "usaco"
	CSES  Source = "cses"
	CF    Source = "cf"
)

// ID is "<source>:<native id>", e.g. "usaco:1598", "cses:1094".
type ID string

type Problem struct {
	ID     ID
	Source Source
	Title  string
	URL    string // canonical statement page

	// Division is USACO's bronze/silver/gold/platinum, "" elsewhere.
	Division string
	// Contest is a human label, e.g. "2024 December, Silver P2".
	Contest string
	// Position is the problem's index within its contest set (1-based), 0
	// when unknown. USACO orders P1 < P2 < P3 by intended difficulty.
	Position int

	// Tags are topic labels from USACO Guide (e.g. "Prefix Sums",
	// "Binary Search"). Topic is the single biome topic derived from them
	// (see Topics), "mixed" when unknown.
	Tags  []string
	Topic string

	// Rating is an estimated Codeforces-scale difficulty, used for door
	// selection: a player rated R solves a problem rated R about half the
	// time. RatingSource says how it was estimated ("division+position",
	// "cf", "guide-label", ...).
	Rating       int
	RatingSource string

	TimeLimit time.Duration
	MemLimit  int64  // bytes
	FileIO    string // stem for freopen-style I/O, "" for stdin/stdout

	// Judgeable is false for problems v1 cannot judge locally: interactive,
	// multiple valid answers (needs a checker), or missing test data.
	Judgeable      bool
	Interactive    bool
	NeedsChecker   bool
	FloatTolerance float64 // > 0 when the statement allows abs/rel error

	HasEditorial bool
}

// Topics are the biome topics doors are grouped by. Content sources map
// their tags onto these; anything unmapped is "mixed".
var Topics = []string{
	"complete-search", "simulation", "sorting-greedy", "prefix-sums",
	"binary-search", "two-pointers", "graphs", "trees", "shortest-paths",
	"dsu", "dp", "math", "strings", "data-structures", "geometry", "mixed",
}

// Assets are local paths for a materialized problem, all inside the cache.
type Assets struct {
	Statement string         // markdown file
	Editorial string         // markdown file, "" if none
	Samples   []problem.Test // shown in the statement; judged on every save
	Tests     []problem.Test // full official tests; judged on :Submit
}

// Catalog is implemented by each content source and by a Union of them.
type Catalog interface {
	// All lists every known problem, judgeable or not, without network.
	All() []Problem
	Get(id ID) (Problem, bool)
	// Fetch downloads (or reuses from cache) everything needed to play id.
	// It is called ahead of time for the doors on offer, so it must be
	// safe to call concurrently and idempotent.
	Fetch(ctx context.Context, id ID) (Assets, error)
}
