package rtfm

import (
	"os"
	"slices"
	"strings"
	"testing"

	"katharsis/internal/curriculum"
)

// TestLayoutMaps lays out every map the game ships: no two items share a
// cell, and no path runs through an item, either way it is routed.
func TestLayoutMaps(t *testing.T) {
	ms, err := curriculum.ParseMaps(os.DirFS("../../curriculum"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) == 0 {
		t.Fatal("no maps")
	}
	for id, m := range ms {
		l := layoutMap(m, true)
		seen := map[[2]int]int{}
		for _, i := range append([]int{readItem, trialItem}, mapIndexes(m)...) {
			c := l.cell(i)
			if j, ok := seen[c]; ok {
				t.Errorf("%s: items %d and %d both at %v", id, i, j, c)
			}
			seen[c] = i
			if c[0] < 0 || c[0] >= l.rows || c[1] < 0 || c[1] >= l.cols {
				t.Errorf("%s: item %d at %v, outside %dx%d", id, i, c, l.rows, l.cols)
			}
		}
		for _, e := range l.edges {
			if l.blocked(e, e.drop) {
				t.Errorf("%s: the path %d -> %d runs through an item", id, e.from, e.to)
			}
		}
	}
}

func mapIndexes(m curriculum.Map) []int {
	out := make([]int, len(m.Nodes))
	for i := range out {
		out[i] = i
	}
	return out
}

// TestLayoutMap: lanes stack, a side chain keeps to its row, the trial
// follows the deepest core problem, and a path whose row is taken drops.
func TestLayoutMap(t *testing.T) {
	m, err := curriculum.ParseMap("m", strings.Join([]string{
		"core usaco-1 > usaco-2 > usaco-3",
		"core usaco-4 > usaco-3",
		"side usaco-2 > usaco-5 > usaco-6",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	l := layoutMap(m, true)
	at := func(problem string) [2]int {
		return l.cell(slices.IndexFunc(m.Nodes, func(n curriculum.Node) bool { return n.Problem == problem }))
	}
	for _, tc := range []struct {
		problem string
		want    [2]int
	}{
		{"usaco-1", [2]int{0, 1}}, {"usaco-2", [2]int{0, 2}}, {"usaco-3", [2]int{0, 3}},
		{"usaco-5", [2]int{1, 3}}, {"usaco-6", [2]int{1, 4}}, // usaco-2's lane, under it
		{"usaco-4", [2]int{2, 1}}, // the second lane
	} {
		if got := at(tc.problem); got != tc.want {
			t.Errorf("%s at %v, want %v", tc.problem, got, tc.want)
		}
	}
	if l.trial != [2]int{0, 4} || l.rows != 3 || l.cols != 5 {
		t.Errorf("trial %v, %dx%d; want {0 4}, 3x5", l.trial, l.rows, l.cols)
	}
	// usaco-4 -> usaco-3 runs from (2, 1) to (0, 3): along row 2, free.
	for _, e := range l.edges {
		if e.drop {
			t.Errorf("edge %+v drops", e)
		}
	}
}
