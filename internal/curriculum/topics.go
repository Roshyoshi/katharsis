package curriculum

import (
	"strings"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
	"katharsis/internal/content/usaco"
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

// DivisionBand returns the CF-scale rating band of a guide division's
// trials: the band the USACO source rates that division's problems in
// (usaco.DivisionBand), which a trial's doors spread over when its map
// aims none (Trial.Targets). General modules sit with Bronze. Advanced has
// no USACO division: it starts where Platinum ends and spans one division
// step, the gap from Gold to Platinum. Unknown divisions get the Bronze
// band.
func DivisionBand(division string) [2]int {
	switch division {
	case "general":
		division = "bronze"
	case "plat":
		division = "platinum"
	case "adv":
		gold, _ := usaco.DivisionBand("gold")
		plat, _ := usaco.DivisionBand("platinum")
		return [2]int{plat[1], plat[1] + plat[0] - gold[0]}
	}
	if b, ok := usaco.DivisionBand(division); ok {
		return b
	}
	b, _ := usaco.DivisionBand("bronze")
	return b
}
