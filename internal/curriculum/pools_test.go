package curriculum

import (
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
)

// TestListed checks listed.txt against a real snapshot of guide.Commit. It
// runs only with KATHARSIS_GUIDE=<snapshot root>; -update rewrites it.
func TestListed(t *testing.T) {
	dir := os.Getenv("KATHARSIS_GUIDE")
	if dir == "" {
		t.Skip("set KATHARSIS_GUIDE to a usaco-guide snapshot to run")
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Every catalog problem the USACO Guide lists at %s:\n", guide.Commit)
	b.WriteString("# RTFM's, which boots never draw (pools.go). TestListed -update writes it.\n")
	for _, id := range c.listed() {
		b.WriteString(string(id) + "\n")
	}
	if *update {
		if err := os.WriteFile("listed.txt", []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if b.String() != listedText {
		t.Error("listed.txt is stale; rerun with KATHARSIS_GUIDE set and -update")
	}
}

// TestSplit holds back half of each topic-and-band cell, at most
// ReserveCap, from problems RTFM does not list, whatever the pool's order.
func TestSplit(t *testing.T) {
	var listed catalog.ID
	for id := range Listed() {
		if id.Source() == catalog.USACO {
			listed = id
			break
		}
	}
	if listed == "" {
		t.Fatal("listed.txt has no USACO problem")
	}
	var pool []catalog.Problem
	add := func(n int, topic string, rating int, judgeable bool) {
		for range n {
			id := catalog.NewID(catalog.CF, fmt.Sprintf("%d%c", 9000+len(pool), 'A'))
			pool = append(pool, catalog.Problem{ID: id, Topic: topic, Rating: rating, Judgeable: judgeable})
		}
	}
	add(5, "dp", 1500, true)              // Silver dp: 3 held
	add(2*ReserveCap+8, "dp", 1900, true) // Gold dp: capped
	add(4, "graphs", 1500, false)         // unjudgeable: boots
	pool = append(pool, catalog.Problem{ID: listed, Topic: "dp", Rating: 1500, Judgeable: true})

	trials, boots := Split(pool)
	cells := map[string]int{}
	for _, p := range trials {
		cells[fmt.Sprint(p.Topic, bandCell(p.Rating))]++
	}
	if want := map[string]int{"dp1": 3, "dp2": ReserveCap}; !sameCounts(cells, want) {
		t.Errorf("held %v, want %v", cells, want)
	}
	if got, want := len(trials)+len(boots), len(pool)-1; got != want {
		t.Errorf("trials and boots hold %d, want %d (all but the listed one)", got, want)
	}
	for _, p := range append(slices.Clone(trials), boots...) {
		if p.ID == listed {
			t.Errorf("listed %s was split", listed)
		}
	}

	shuffled := slices.Clone(pool)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	again, _ := Split(shuffled)
	ids := func(ps []catalog.Problem) []catalog.ID {
		var out []catalog.ID
		for _, p := range ps {
			out = append(out, p.ID)
		}
		slices.Sort(out)
		return out
	}
	if !slices.Equal(ids(trials), ids(again)) {
		t.Error("the reserve depends on the pool's order")
	}
}

func sameCounts(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
