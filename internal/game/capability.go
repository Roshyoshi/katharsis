package game

import "slices"

// TopicCapabilities gates topic biomes behind capabilities earned in learn
// mode. The un-gated topics (complete-search, simulation, sorting-greedy,
// math, mixed) are USACO Bronze material and hold hundreds of problems, so a
// new player's routes always have something to offer; Silver and later
// topics unlock as the player passes their trials.
var TopicCapabilities = map[string]string{
	"prefix-sums":     "CAP_PREFIX",
	"binary-search":   "CAP_BSEARCH",
	"two-pointers":    "CAP_TWOPTR",
	"graphs":          "CAP_GRAPH",
	"trees":           "CAP_TREE",
	"shortest-paths":  "CAP_SSSP",
	"dsu":             "CAP_DSU",
	"dp":              "CAP_DP",
	"strings":         "CAP_STRING",
	"data-structures": "CAP_DS",
	"geometry":        "CAP_GEOM",
}

// CapabilityFor is the capability that unlocks topic's biomes, "" when the
// topic is open to everyone.
func CapabilityFor(topic string) string { return TopicCapabilities[topic] }

// Unlocked reports whether topic biomes can be offered to m. Doors in other
// biomes may still draw gated topics: the gate is on routes, not problems.
func Unlocked(m *Meta, topic string) bool {
	c := CapabilityFor(topic)
	return c == "" || slices.Contains(m.Capabilities, c)
}

// GrantCapability records a capability earned in learn mode. Granting one
// twice is a no-op.
func GrantCapability(m *Meta, c string) {
	if c != "" && !slices.Contains(m.Capabilities, c) {
		m.Capabilities = append(m.Capabilities, c)
	}
}

// AddLib stores a lib implemented in a trial, replacing an older one of the
// same name.
func AddLib(m *Meta, l Lib) {
	for i := range m.Libs {
		if m.Libs[i].Name == l.Name {
			m.Libs[i] = l
			return
		}
	}
	m.Libs = append(m.Libs, l)
}
