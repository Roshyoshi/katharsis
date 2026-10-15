package usaco

import (
	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
)

// tagTopic maps USACO Guide tags onto catalog.Topics. A tag that names a
// guide module maps where the module does (guide.ModuleTopic), so the
// module's own problems sit in the biome its trial grants: Sliding Window
// is data-structures, Sweep Line geometry, Merging trees. Tags that name
// no technique (Casework, Constructive, Amortized Analysis, Divide and
// Conquer, and Bitset, a speed-up the guide files under no topic) are left
// out so they never decide a topic.
var tagTopic = map[string]string{
	"Complete Search": "complete-search", "Recursion": "complete-search",
	"Subsets": "complete-search", "Permutation": "complete-search",
	"Bitmasks": "complete-search", "Meet in the Middle": "complete-search",
	"Fracturing Search": "complete-search",

	"Simulation": "simulation",

	"Sorting": "sorting-greedy", "Greedy": "sorting-greedy",
	"0/1 Principle": "sorting-greedy",

	"Prefix Sums": "prefix-sums", "2D Prefix Sums": "prefix-sums",
	"Difference Array": "prefix-sums", "Max Subarray Sum": "prefix-sums",

	"Binary Search": "binary-search", "Ternary Search": "binary-search",

	"Two Pointers": "two-pointers",

	"Graphs": "graphs", "DFS": "graphs", "BFS": "graphs",
	"Flood Fill": "graphs", "Connected Components": "graphs",
	"Bipartite": "graphs", "TopoSort": "graphs", "Functional Graph": "graphs",
	"Cycle": "graphs", "SCC": "graphs", "BCC": "graphs", "MST": "graphs",
	"Spanning Tree": "graphs", "Coloring": "graphs", "Cactus": "graphs",
	"Flow": "graphs", "Min Cut": "graphs", "Euler's Formula": "graphs",

	"Tree": "trees", "LCA": "trees", "Binary Jumping": "trees",
	"Euler Tour": "trees", "HLD": "trees", "Centroid": "trees",
	"Rerooting": "trees", "Diameter": "trees", "Small to Large": "trees",
	"Merging": "trees",

	"Shortest Path": "shortest-paths", "APSP": "shortest-paths",

	"DSU": "dsu",

	"DP": "dp", "Knapsack": "dp", "Range DP": "dp", "Broken Profile": "dp",
	"Slope Trick": "dp",

	"Math": "math", "Number Theory": "math", "Combinatorics": "math",
	"Modular Arithmetic": "math", "Exponentiation": "math",
	"Prime Factorization": "math", "PIE": "math", "Probability": "math",
	"Matrix": "math", "Determinant": "math", "Generating Functions": "math",
	"Game": "math", "Pigeonhole": "math", "Bitwise": "math",

	"Strings": "strings", "Hashing": "strings", "Suffix Array": "strings",
	"Trie": "strings",

	"Stack": "data-structures", "Queue": "data-structures",
	"Deque": "data-structures", "Priority Queue": "data-structures",
	"Set": "data-structures", "Map": "data-structures",
	"Sorted Set": "data-structures", "Indexed Set": "data-structures",
	"BBST": "data-structures", "Linked List": "data-structures",
	"SegTree": "data-structures", "Lazy SegTree": "data-structures",
	"BIT": "data-structures", "PURS": "data-structures",
	"1DRQ": "data-structures", "2DRQ": "data-structures",
	"Sqrt": "data-structures", "Treap": "data-structures",
	"Sliding Window": "data-structures", "Inversions": "data-structures",

	// "Convex" covers convex hulls, the convex hull trick and convexity
	// (Aliens). It only decides when nothing more specific is tagged: dp
	// outranks geometry, so "Convex, DP" is a DP problem.
	"Geometry": "geometry", "Rectangle": "geometry", "Convex": "geometry",
	"Sweep Line": "geometry",
}

// topicFor picks the biome topic from tags, then from the guide modules
// that list the problem (guide.ModuleTopic says what each teaches), else
// catalog.Mixed. Among several, catalog.MostSpecific decides.
func topicFor(tags, modules []string) string {
	var fromTags, fromModules []string
	for _, tag := range tags {
		if t, ok := tagTopic[tag]; ok {
			fromTags = append(fromTags, t)
		}
	}
	if t := catalog.MostSpecific(fromTags); t != catalog.Mixed {
		return t
	}
	for _, m := range modules {
		if t, ok := guide.ModuleTopic(m); ok {
			fromModules = append(fromModules, t)
		}
	}
	return catalog.MostSpecific(fromModules)
}
