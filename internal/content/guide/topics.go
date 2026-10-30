package guide

import "maps"

// moduleTopics maps every module in the pinned ordering.ts to the catalog
// topic (catalog.Topics) it teaches. It is the one such table: the USACO
// catalog falls back on it for a problem whose tags name no topic, and
// learn mode draws a module trial's doors from its topic and grants that
// topic's capability. Modules that teach no algorithmic topic (the
// General division, the conclusions) are absent.
//
// The guide's own grouping decides first: the chapter a module sits in
// (ordering.ts), then what the module text teaches. Judgment calls, so a
// reviewer can disagree with them in one place:
//
//   - dp-trees, all-roots            -> trees (tree problems first, DP second)
//   - convex-hull-trick, slope-trick -> dp (both are DP optimizations)
//   - line-container                 -> data-structures: the module teaches
//     the container ("Convex Containers", KACTL's
//     data-structures/LineContainer.h); the DP it speeds up is
//     convex-hull-trick's
//   - sliding-window, stacks         -> data-structures (Gold chapter "Data
//     Structures"; the module keeps a multiset or a monotonic deque)
//   - merging (small to large)       -> trees (Platinum chapter "Trees";
//     prerequisites intro-tree and PURS)
//   - sweep-line                     -> geometry (Platinum chapter
//     "Geometry"; prerequisite geo-pri)
//   - ternary-search                 -> binary-search
//   - hashing, string-*              -> strings; hashmaps -> data-structures
//   - meet-in-the-middle, fracturing-search -> complete-search
//   - intro-bitwise, PIE, matrix-expo, fft*, game-theory -> math
//   - toposort, mst, flows, SCC, BCC, eulerian tours -> graphs
//   - offline-del (DSU with rollback) -> dsu; link-cut-tree -> trees
//   - matroid, matroid-isect         -> sorting-greedy (greedy exchange)
//   - casework, ad-hoc, time-comp, random, interactive, vectorization,
//     bitsets                        -> mixed (no single technique; bitsets
//     is "(Optional) Bitsets", a speed-up whose problems are knapsack,
//     PIE, geometry, DSU and toposort)
var moduleTopics = map[string]string{
	// Bronze
	"time-comp":      "mixed",
	"intro-ds":       "data-structures",
	"simulation":     "simulation",
	"intro-complete": "complete-search",
	"complete-rec":   "complete-search",
	"intro-sorting":  "sorting-greedy",
	"intro-sets":     "data-structures",
	"casework":       "mixed",
	"intro-greedy":   "sorting-greedy",
	"intro-graphs":   "graphs",
	"rect-geo":       "geometry",
	"ad-hoc":         "mixed",

	// Silver
	"prefix-sums":                "prefix-sums",
	"more-prefix-sums":           "prefix-sums",
	"two-pointers":               "two-pointers",
	"binary-search-sorted-array": "binary-search",
	"binary-search":              "binary-search",
	"sorting-custom":             "sorting-greedy",
	"greedy-sorting":             "sorting-greedy",
	"priority-queues":            "data-structures",
	"graph-traversal":            "graphs",
	"flood-fill":                 "graphs",
	"intro-tree":                 "trees",
	"func-graphs":                "graphs",
	"intro-bitwise":              "math",

	// Gold
	"divisibility":              "math",
	"modular":                   "math",
	"combo":                     "math",
	"intro-dp":                  "dp",
	"knapsack":                  "dp",
	"paths-grids":               "dp",
	"lis":                       "dp",
	"dp-bitmasks":               "dp",
	"dp-ranges":                 "dp",
	"digit-dp":                  "dp",
	"unweighted-shortest-paths": "shortest-paths",
	"dsu":                       "dsu",
	"toposort":                  "graphs",
	"shortest-paths":            "shortest-paths",
	"mst":                       "graphs",
	"intro-sorted-sets":         "data-structures",
	"custom-cpp-stl":            "data-structures",
	"stacks":                    "data-structures",
	"sliding-window":            "data-structures",
	"PURS":                      "data-structures",
	"dp-trees":                  "trees",
	"all-roots":                 "trees",
	"tree-euler":                "trees",
	"lca-euler":                 "trees",
	"ternary-search":            "binary-search",
	"hashing":                   "strings",
	"hashmaps":                  "data-structures",
	"meet-in-the-middle":        "complete-search",

	// Platinum
	"segtree-ext":       "data-structures",
	"range-sweep":       "data-structures",
	"RURQ":              "data-structures",
	"sparse-segtree":    "data-structures",
	"2DRQ":              "data-structures",
	"DC-SRQ":            "data-structures",
	"sqrt":              "data-structures",
	"binary-jump":       "trees",
	"merging":           "trees",
	"hld":               "trees",
	"centroid":          "trees",
	"VT":                "trees",
	"kruskal-tree":      "trees",
	"geo-pri":           "geometry",
	"sweep-line":        "geometry",
	"convex-hull":       "geometry",
	"convex-hull-trick": "dp",
	"PIE":               "math",
	"matrix-expo":       "math",
	"bitsets":           "mixed",
	"DC-DP":             "dp",
	"dp-sos":            "dp",

	// Advanced
	"springboards":      "data-structures",
	"wavelet":           "data-structures",
	"count-min":         "data-structures",
	"segtree-beats":     "data-structures",
	"persistent":        "data-structures",
	"treaps":            "data-structures",
	"line-container":    "data-structures",
	"lagrange":          "math",
	"slope-trick":       "dp",
	"sp-neg":            "shortest-paths",
	"eulerian-tours":    "graphs",
	"BCC-2CC":           "graphs",
	"SCC":               "graphs",
	"offline-del":       "dsu",
	"eulers-formula":    "graphs",
	"critical":          "graphs",
	"link-cut-tree":     "trees",
	"comb-sub":          "dp",
	"dp-broken-profile": "dp",
	"dp-more":           "dp",
	"max-flow":          "graphs",
	"min-cut":           "graphs",
	"flow-lb":           "graphs",
	"min-cost-flow":     "graphs",
	"fft":               "math",
	"fft-ext":           "math",
	"string-search":     "strings",
	"suffix-array":      "strings",
	"string-suffix":     "strings",
	"extend-euclid":     "math",
	"catalan":           "math",
	"xor-basis":         "math",
	"fracturing-search": "complete-search",
	"game-theory":       "math",
	"prefix-sums-nt-1":  "math",
	"prefix-sums-nt-2":  "math",
	"matroid":           "sorting-greedy",
	"matroid-isect":     "sorting-greedy",
	"random":            "mixed",
	"interactive":       "mixed",
	"vectorization":     "mixed",
}

// ModuleTopic returns the topic module id teaches; ok is false for a
// module the table does not know (one with no topic, or one the guide
// added after the table was written).
func ModuleTopic(id string) (topic string, ok bool) {
	topic, ok = moduleTopics[id]
	return topic, ok
}

// ModuleTopics returns a copy of the module id -> topic table.
func ModuleTopics() map[string]string { return maps.Clone(moduleTopics) }
