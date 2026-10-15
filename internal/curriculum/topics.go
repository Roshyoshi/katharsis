package curriculum

import (
	"maps"
	"strings"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// moduleTopics maps every module in the guide's ordering.ts (snapshot of
// 2026-10) to the catalog biome topic its trial tests. The table is explicit
// because module ids are terse ("PURS", "VT") and titles name techniques,
// not biomes. Modules that teach no algorithmic topic (the General division,
// the conclusions) are absent and get topic "": they have no capability.
// Judgment calls, so a reviewer can disagree with them in one place:
//
//   - dp-trees, all-roots           -> trees (tree problems first, DP second)
//   - convex-hull-trick, slope-trick -> dp (both are DP optimizations)
//   - line-container                 -> data-structures
//   - sliding-window, stacks         -> data-structures
//   - ternary-search                 -> binary-search
//   - hashing, string-*              -> strings; hashmaps -> data-structures
//   - meet-in-the-middle, fracturing-search -> complete-search
//   - intro-bitwise, PIE, matrix-expo, fft*, game-theory -> math
//   - toposort, mst, flows, SCC, BCC, eulerian tours -> graphs
//   - offline-del (DSU with rollback) -> dsu; link-cut-tree -> trees
//   - matroid, matroid-isect         -> sorting-greedy (greedy exchange)
//   - casework, ad-hoc, time-comp, random, interactive, vectorization,
//     bitsets                        -> mixed (no single technique)
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

// ModuleTopics returns a copy of the module id -> catalog topic table.
func ModuleTopics() map[string]string { return maps.Clone(moduleTopics) }

// topicKeywords classify modules added to the guide after the table was
// written, by words in their id or title. First match wins, so the more
// specific entries come first.
var topicKeywords = []struct{ word, topic string }{
	{"shortest", "shortest-paths"}, {"dijkstra", "shortest-paths"},
	{"union-find", "dsu"}, {"dsu", "dsu"},
	{"prefix", "prefix-sums"}, {"two pointer", "two-pointers"},
	{"binary search", "binary-search"}, {"binary-search", "binary-search"},
	{"segment tree", "data-structures"}, {"segtree", "data-structures"},
	{"dp", "dp"}, {"dynamic programming", "dp"},
	{"tree", "trees"}, {"graph", "graphs"}, {"flow", "graphs"},
	{"string", "strings"}, {"suffix", "strings"},
	{"geometry", "geometry"}, {"convex", "geometry"},
	{"queue", "data-structures"}, {"stack", "data-structures"},
	{"sort", "sorting-greedy"}, {"greedy", "sorting-greedy"},
	{"search", "complete-search"}, {"simulation", "simulation"},
	{"math", "math"}, {"number theory", "math"}, {"combinator", "math"},
}

// TopicFor returns the catalog topic for a module: the table entry when
// there is one, else a keyword guess from id and title, else Mixed.
// division "general" (and ids ending in -conclusion) have no topic ("").
func TopicFor(moduleID, title, division string) string {
	if t, ok := moduleTopics[moduleID]; ok {
		return t
	}
	if division == "general" || strings.HasSuffix(moduleID, "-conclusion") {
		return ""
	}
	hay := strings.ToLower(moduleID + " " + title)
	for _, k := range topicKeywords {
		if strings.Contains(hay, k.word) {
			return k.topic
		}
	}
	return catalog.Mixed
}

// CapabilityFor names the capability a passed trial grants for topic. The
// engine's gate table (game.TopicCapabilities) is the only source: a name
// built here would never unlock a biome the engine gates by another. So
// "dp" -> "CAP_DP" and "prefix-sums" -> "CAP_PREFIX", while topics the
// engine never gates ("mixed", the Bronze topics such as "simulation") and
// unknown ones return "": their biomes are open, so there is nothing to
// unlock.
func CapabilityFor(topic string) string { return game.CapabilityFor(topic) }

// divisionBands are CF-scale rating bands for the integration doors, the
// same division table the USACO content source uses for its estimates.
// General modules sit with Bronze; Advanced extends past Platinum.
var divisionBands = map[string][2]int{
	"general": {1000, 1300},
	"bronze":  {1000, 1300},
	"silver":  {1350, 1650},
	"gold":    {1700, 2000},
	"plat":    {2100, 2400},
	"adv":     {2400, 2800},
}

// DivisionBand returns the rating band for a guide division id; unknown
// divisions get the Bronze band.
func DivisionBand(division string) [2]int {
	if b, ok := divisionBands[division]; ok {
		return b
	}
	return divisionBands["bronze"]
}
