package learn

import (
	"slices"
	"strings"
	"testing"

	"katharsis/internal/catalog"
)

func TestCapabilityFor(t *testing.T) {
	for _, tc := range []struct{ topic, want string }{
		{"dp", "CAP_DP"},
		{"prefix-sums", "CAP_PREFIX_SUMS"},
		{"complete-search", "CAP_COMPLETE_SEARCH"},
		{"shortest-paths", "CAP_SHORTEST_PATHS"},
		{"mixed", ""},
		{"", ""},
		{"not-a-topic", ""},
	} {
		if got := CapabilityFor(tc.topic); got != tc.want {
			t.Errorf("CapabilityFor(%q) = %q, want %q", tc.topic, got, tc.want)
		}
	}
	// Every gated topic has a distinct capability.
	seen := map[string]string{}
	for _, topic := range catalog.Topics {
		c := CapabilityFor(topic)
		if c == "" {
			continue
		}
		if prev, dup := seen[c]; dup {
			t.Errorf("%s and %s share %s", prev, topic, c)
		}
		seen[c] = topic
	}
	if len(seen) != len(catalog.Topics)-1 {
		t.Errorf("%d capabilities for %d topics", len(seen), len(catalog.Topics))
	}
}

// The table must cover every module the pinned ordering.ts teaches and use
// only catalog topics; a guide bump that adds a module fails here so the
// table gets a deliberate entry instead of a keyword guess.
func TestModuleTopicsCoverOrdering(t *testing.T) {
	for id, topic := range ModuleTopics() {
		if !slices.Contains(catalog.Topics, topic) {
			t.Errorf("module %s maps to %q, not a catalog topic", id, topic)
		}
	}
	divs, err := ParseOrdering("testdata/ordering_full.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range divs {
		for _, c := range d.Chapters {
			for _, m := range c.Modules {
				_, ok := moduleTopics[m.ID]
				none := d.ID == "general" || strings.HasSuffix(m.ID, "-conclusion")
				if ok == none {
					t.Errorf("module %s (%s): in table = %v", m.ID, d.ID, ok)
				}
			}
		}
	}
}

func TestTopicFor(t *testing.T) {
	for _, tc := range []struct{ id, title, div, want string }{
		{"prefix-sums", "Introduction to Prefix Sums", "silver", "prefix-sums"},
		{"PURS", "Point Update Range Sum", "gold", "data-structures"},
		{"fast-io", "Fast Input and Output", "general", ""},
		{"gold-conclusion", "Conclusion", "gold", ""},
		{"new-thing", "Fancy DP on Broken Profiles", "adv", "dp"},
		{"segtree-new", "Segment Tree Tricks", "plat", "data-structures"},
		{"tree-new", "More Tree Stuff", "plat", "trees"},
		{"mystery", "Something Else", "gold", "mixed"},
	} {
		if got := TopicFor(tc.id, tc.title, tc.div); got != tc.want {
			t.Errorf("TopicFor(%q, %q, %q) = %q, want %q", tc.id, tc.title, tc.div, got, tc.want)
		}
	}
}

func TestDivisionBand(t *testing.T) {
	prev := 0
	for _, d := range []string{"bronze", "silver", "gold", "plat", "adv"} {
		b := DivisionBand(d)
		if b[0] >= b[1] || b[0] < prev {
			t.Errorf("%s band %v not increasing", d, b)
		}
		prev = b[0]
	}
	if DivisionBand("nope") != DivisionBand("bronze") {
		t.Error("unknown division should fall back to bronze")
	}
}
