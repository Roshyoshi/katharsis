package usaco

// tagTopic maps USACO Guide tags onto catalog.Topics. Tags that name no
// technique (Casework, Constructive, Amortized Analysis, Divide and
// Conquer) are left out so they never decide a topic.
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

	"Two Pointers": "two-pointers", "Sliding Window": "two-pointers",

	"Graphs": "graphs", "DFS": "graphs", "BFS": "graphs",
	"Flood Fill": "graphs", "Connected Components": "graphs",
	"Bipartite": "graphs", "TopoSort": "graphs", "Functional Graph": "graphs",
	"Cycle": "graphs", "SCC": "graphs", "BCC": "graphs", "MST": "graphs",
	"Spanning Tree": "graphs", "Coloring": "graphs", "Cactus": "graphs",
	"Flow": "graphs", "Min Cut": "graphs", "Euler's Formula": "graphs",

	"Tree": "trees", "LCA": "trees", "Binary Jumping": "trees",
	"Euler Tour": "trees", "HLD": "trees", "Centroid": "trees",
	"Rerooting": "trees", "Diameter": "trees", "Small to Large": "trees",

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
	"Bitset": "data-structures", "Sweep Line": "data-structures",
	"Merging": "data-structures", "Inversions": "data-structures",

	// "Convex" covers convex hulls, the convex hull trick and convexity
	// (Aliens). It only decides when nothing more specific is tagged: dp
	// outranks geometry, so "Convex, DP" is a DP problem.
	"Geometry": "geometry", "Rectangle": "geometry", "Convex": "geometry",
}

// topicRank breaks ties when a problem's tags map to several topics: the
// most specific technique wins. "Tree, DP" is a trees problem; "Sorting,
// Prefix Sums" is a prefix-sums problem; "Binary Search, Graphs" is a graph
// problem whose answer is searched.
var topicRank = []string{
	"shortest-paths", "dsu", "trees", "strings", "dp", "geometry", "graphs",
	"binary-search", "two-pointers", "prefix-sums", "data-structures", "math",
	"sorting-greedy", "complete-search", "simulation",
}

// moduleTopic is the fallback for problems whose tags map to nothing: the
// guide module that lists them names the technique.
var moduleTopic = map[string]string{
	"intro-complete": "complete-search", "complete-rec": "complete-search",
	"meet-in-the-middle": "complete-search", "fracturing-search": "complete-search",
	"simulation":   "simulation",
	"intro-greedy": "sorting-greedy", "greedy-sorting": "sorting-greedy",
	"sorting-custom": "sorting-greedy", "intro-sorting": "sorting-greedy",
	"prefix-sums": "prefix-sums", "more-prefix-sums": "prefix-sums",
	"binary-search": "binary-search", "binary-search-sorted-array": "binary-search",
	"ternary-search": "binary-search",
	"two-pointers":   "two-pointers", "sliding-window": "two-pointers",
	"flood-fill": "graphs", "graph-traversal": "graphs", "intro-graphs": "graphs",
	"toposort": "graphs", "func-graphs": "graphs", "SCC": "graphs",
	"BCC-2CC": "graphs", "min-cut": "graphs", "mst": "graphs",
	"eulers-formula": "graphs",
	"intro-tree":     "trees", "binary-jump": "trees", "tree-euler": "trees",
	"lca-euler": "trees", "hld": "trees", "centroid": "trees",
	"all-roots": "trees", "dp-trees": "trees",
	"unweighted-shortest-paths": "shortest-paths", "shortest-paths": "shortest-paths",
	"dsu":      "dsu",
	"intro-dp": "dp", "knapsack": "dp", "paths-grids": "dp", "lis": "dp",
	"dp-bitmasks": "dp", "dp-ranges": "dp", "digit-dp": "dp", "dp-sos": "dp",
	"DC-DP": "dp", "convex-hull-trick": "dp", "dp-broken-profile": "dp",
	"slope-trick": "dp", "line-container": "dp",
	"combo": "math", "modular": "math", "PIE": "math", "matrix-expo": "math",
	"intro-bitwise": "math", "lagrange": "math", "fft-ext": "math",
	"hashing": "strings", "string-search": "strings", "string-suffix": "strings",
	"suffix-array": "strings",
	"intro-sets":   "data-structures", "intro-sorted-sets": "data-structures",
	"stacks": "data-structures", "priority-queues": "data-structures",
	"PURS": "data-structures", "RURQ": "data-structures",
	"segtree-ext": "data-structures", "2DRQ": "data-structures",
	"sqrt": "data-structures", "treaps": "data-structures",
	"bitsets": "data-structures", "hashmaps": "data-structures",
	"DC-SRQ": "data-structures", "merging": "data-structures",
	"sweep-line": "data-structures",
	"rect-geo":   "geometry", "convex-hull": "geometry",
}

// topicFor picks the biome topic from tags, then from modules, else
// "mixed".
func topicFor(tags, modules []string) string {
	if t := bestTopic(tags, tagTopic); t != "" {
		return t
	}
	if t := bestTopic(modules, moduleTopic); t != "" {
		return t
	}
	return "mixed"
}

func bestTopic(keys []string, table map[string]string) string {
	have := map[string]bool{}
	for _, k := range keys {
		if t, ok := table[k]; ok {
			have[t] = true
		}
	}
	for _, t := range topicRank {
		if have[t] {
			return t
		}
	}
	return ""
}
