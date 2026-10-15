package codeforces

// tagTopic maps Codeforces tags onto catalog.Topics. Tags that name no
// technique are left out so they never decide a topic: constructive
// algorithms, divide and conquer, schedules, interactive and *special.
// Codeforces has no prefix-sums tag, so that biome gets no CF problems.
//
// "bitmasks" maps to math: on Codeforces it mostly marks XOR and bit
// tricks, and bitmask DP is tagged dp as well, which outranks it.
var tagTopic = map[string]string{
	"brute force":        "complete-search",
	"meet-in-the-middle": "complete-search",

	"implementation": "simulation",

	"greedy":   "sorting-greedy",
	"sortings": "sorting-greedy",

	"binary search":  "binary-search",
	"ternary search": "binary-search",

	"two pointers": "two-pointers",

	"graphs":          "graphs",
	"dfs and similar": "graphs",
	"graph matchings": "graphs",
	"flows":           "graphs",
	"2-sat":           "graphs",

	"trees": "trees",

	"shortest paths": "shortest-paths",

	"dsu": "dsu",

	"dp": "dp",

	"math":                      "math",
	"number theory":             "math",
	"combinatorics":             "math",
	"probabilities":             "math",
	"matrices":                  "math",
	"chinese remainder theorem": "math",
	"fft":                       "math",
	"games":                     "math",
	"bitmasks":                  "math",

	"strings":                  "strings",
	"hashing":                  "strings",
	"string suffix structures": "strings",
	"expression parsing":       "strings",

	"data structures": "data-structures",

	"geometry": "geometry",
}

// topicRank picks among the topics a problem's tags map to: the most
// specific technique wins. It is the USACO source's order, so a topic
// means the same thing whichever source a door comes from. Codeforces
// tags are generous (a typical Div. 2 C carries "greedy", "math" and
// "implementation" on top of its real technique), and the dataset lists
// them alphabetically, so tag order carries no signal and rank decides:
//
//	dp, greedy, two pointers                    → dp
//	binary search, data structures, greedy      → binary-search
//	combinatorics, dfs and similar, dp, trees   → trees
//	constructive algorithms, greedy, math       → math
var topicRank = []string{
	"shortest-paths", "dsu", "trees", "strings", "dp", "geometry", "graphs",
	"binary-search", "two-pointers", "prefix-sums", "data-structures", "math",
	"sorting-greedy", "complete-search", "simulation",
}

// topicFor picks the biome topic for a tag list, "mixed" if no tag maps.
func topicFor(tags []string) string {
	have := map[string]bool{}
	for _, t := range tags {
		if topic, ok := tagTopic[t]; ok {
			have[topic] = true
		}
	}
	for _, t := range topicRank {
		if have[t] {
			return t
		}
	}
	return "mixed"
}
