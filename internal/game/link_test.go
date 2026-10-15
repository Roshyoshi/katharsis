package game_test

import (
	"errors"
	"slices"
	"testing"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

func TestLink(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	m := &game.Meta{Libs: []game.Lib{{Name: "dsu", Source: "struct DSU{};\n"}, {Name: "bit", Source: "int b;\n"}, {Name: "seg", Source: "int s;\n"}}}
	h := newHarness(t, plain(), m, ps, 0, 1)

	for _, tc := range []struct {
		names []string
		ok    bool
	}{
		{[]string{"dsu", "bit", "seg"}, false}, // over the 2 slots
		{[]string{"nope"}, false},
		{[]string{"dsu", "dsu"}, true}, // duplicates collapse
		{[]string{"bit", "dsu"}, true},
	} {
		_, err := game.Step(h.s, m, ps, game.Link{Libs: tc.names}, h.now)
		if tc.ok != (err == nil) || (err != nil && !errors.Is(err, game.ErrInvalid)) {
			t.Errorf("Link%v: err %v", tc.names, err)
		}
	}
	if !slices.Equal(h.s.Linked, []string{"bit", "dsu"}) {
		t.Fatalf("linked %v", h.s.Linked)
	}
	if got := game.LinkedLibs(h.s, m); len(got) != 2 || got[0].Name != "bit" || got[1].Source != "struct DSU{};\n" {
		t.Fatalf("LinkedLibs %+v", got)
	}

	h.open(0)
	if _, err := game.Step(h.s, m, ps, game.Link{Libs: []string{"seg"}}, h.now); !errors.Is(err, game.ErrInvalid) {
		t.Fatalf("link after a door opened: err %v", err)
	}
}
