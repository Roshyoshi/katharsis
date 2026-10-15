package catalog_test

import (
	"testing"

	"katharsis/internal/catalog"
)

func TestID(t *testing.T) {
	for _, c := range []struct {
		id     catalog.ID
		src    catalog.Source
		native string
	}{
		{catalog.NewID(catalog.USACO, "1598"), catalog.USACO, "1598"},
		{catalog.NewID(catalog.CF, "1899C"), catalog.CF, "1899C"},
		{"cses:1094", catalog.CSES, "1094"},
		{"t:dp:1300", "t", "dp:1300"}, // only the first colon splits
		{"bare", "", "bare"},
	} {
		if got := c.id.Source(); got != c.src {
			t.Errorf("%q.Source() = %q, want %q", c.id, got, c.src)
		}
		if got := c.id.Native(); got != c.native {
			t.Errorf("%q.Native() = %q, want %q", c.id, got, c.native)
		}
		if c.src != "" && catalog.NewID(c.src, c.native) != c.id {
			t.Errorf("NewID(%q, %q) does not round-trip to %q", c.src, c.native, c.id)
		}
	}
	if id := catalog.NewID(catalog.USACO, "1598"); id != "usaco:1598" {
		t.Errorf("NewID = %q", id)
	}
}

func TestSourceLabel(t *testing.T) {
	for src, want := range map[catalog.Source]string{
		catalog.USACO: "USACO", catalog.CF: "Codeforces", catalog.CSES: "CSES",
		"atcoder": "atcoder", "": "",
	} {
		if got := src.Label(); got != want {
			t.Errorf("%q.Label() = %q, want %q", src, got, want)
		}
	}
}
