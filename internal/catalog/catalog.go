// Package catalog is the shared vocabulary for problems the game can serve.
// Content sources (USACO and Codeforces now, CSES later) produce Problems;
// the engine picks doors from them; the app materializes the chosen one.
package catalog

import (
	"context"
	"strings"
	"time"

	"katharsis/internal/problem"
)

type Source string

const (
	USACO Source = "usaco"
	CSES  Source = "cses"
	CF    Source = "cf"
)

// Label is the source's name as screens show it: "USACO", "Codeforces",
// "CSES". An unknown source shows as itself.
func (s Source) Label() string {
	switch s {
	case USACO:
		return "USACO"
	case CF:
		return "Codeforces"
	case CSES:
		return "CSES"
	}
	return string(s)
}

// ID is "<source>:<native id>", e.g. "usaco:1598", "cf:1899C",
// "cses:1094". Build one with NewID and take it apart with Source and
// Native rather than by hand.
type ID string

// NewID is the catalog ID of a source's own problem id: NewID(USACO,
// "1598") is "usaco:1598".
func NewID(src Source, native string) ID { return ID(string(src) + ":" + native) }

// Source is the source part of the ID, "" when it has none.
func (id ID) Source() Source {
	src, _, ok := strings.Cut(string(id), ":")
	if !ok {
		return ""
	}
	return Source(src)
}

// Native is the source's own id for the problem: "1598" for "usaco:1598".
// An ID with no source is all native.
func (id ID) Native() string {
	if _, native, ok := strings.Cut(string(id), ":"); ok {
		return native
	}
	return string(id)
}

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
	// (see Topics), Mixed when unknown.
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
	// AnyCase: the statement accepts YES/NO-style answers in any letter
	// case, so the judge folds those words before comparing.
	AnyCase bool

	HasEditorial bool
}

// Mixed is the topic of problems no tag maps to a biome topic. A biome on
// Mixed draws doors from every topic.
const Mixed = "mixed"

// Topics are the biome topics doors are grouped by. Content sources map
// their tags onto these; anything unmapped is Mixed.
var Topics = []string{
	"complete-search", "simulation", "sorting-greedy", "prefix-sums",
	"binary-search", "two-pointers", "graphs", "trees", "shortest-paths",
	"dsu", "dp", "math", "strings", "data-structures", "geometry", Mixed,
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
