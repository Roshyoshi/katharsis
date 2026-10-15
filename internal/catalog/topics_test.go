package catalog_test

import (
	"testing"

	"katharsis/internal/catalog"
)

func TestMostSpecific(t *testing.T) {
	// Every topic is ranked: alone, each is its own pick.
	for _, topic := range catalog.Topics {
		if got := catalog.MostSpecific([]string{topic}); got != topic {
			t.Errorf("MostSpecific([%s]) = %s", topic, got)
		}
	}
	for _, tc := range []struct {
		topics []string
		want   string
	}{
		{[]string{"dp", "trees"}, "trees"},
		{[]string{"sorting-greedy", "prefix-sums"}, "prefix-sums"},
		{[]string{"binary-search", "graphs"}, "graphs"},
		{[]string{"math", "dp", "sorting-greedy"}, "dp"},
		{[]string{catalog.Mixed, "simulation"}, "simulation"},
		{nil, catalog.Mixed},
	} {
		if got := catalog.MostSpecific(tc.topics); got != tc.want {
			t.Errorf("MostSpecific(%v) = %s, want %s", tc.topics, got, tc.want)
		}
	}
}
