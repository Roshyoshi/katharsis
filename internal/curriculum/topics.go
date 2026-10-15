package curriculum

import (
	"strings"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
	"katharsis/internal/game"
)

// topicKeywords classify modules the guide added after guide.ModuleTopic's
// table was written, by words in their id or title. First match wins, so
// the more specific entries come first.
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

// TopicFor returns the catalog topic for a module: guide.ModuleTopic when
// the table has it, else a keyword guess from id and title, else Mixed.
// division "general" (and ids ending in -conclusion) have no topic ("").
func TopicFor(moduleID, title, division string) string {
	if t, ok := guide.ModuleTopic(moduleID); ok {
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
