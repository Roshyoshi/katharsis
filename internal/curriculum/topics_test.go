package curriculum

import (
	"strings"
	"testing"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
	"katharsis/internal/game"
)

func TestCapabilityFor(t *testing.T) {
	for _, tc := range []struct{ topic, want string }{
		{"dp", "CAP_DP"},
		{"prefix-sums", "CAP_PREFIX"},
		{"shortest-paths", "CAP_SSSP"},
		{"complete-search", ""}, // Bronze material: never gated
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
	if len(seen) != len(game.TopicCapabilities) {
		t.Errorf("%d capabilities for the engine's %d gated topics", len(seen), len(game.TopicCapabilities))
	}
}

// A passed trial must open the biomes of the topic it taught: the bug this
// guards is the curriculum naming a capability the engine's gate never checks.
func TestTrialUnlocksItsBiome(t *testing.T) {
	for id, topic := range guide.ModuleTopics() {
		m := &game.Meta{}
		game.GrantCapability(m, CapabilityFor(topic))
		if !game.Unlocked(m, topic) {
			t.Errorf("module %s: passing grants %q, which does not unlock %s", id, CapabilityFor(topic), topic)
		}
	}
}

// guide.ModuleTopic must cover every module the pinned ordering.ts teaches;
// a guide bump that adds a module fails here so the table gets a
// deliberate entry instead of a keyword guess.
func TestModuleTopicsCoverOrdering(t *testing.T) {
	divs, err := ParseOrdering("testdata/ordering_full.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range divs {
		for _, c := range d.Chapters {
			for _, m := range c.Modules {
				_, ok := guide.ModuleTopic(m.ID)
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
