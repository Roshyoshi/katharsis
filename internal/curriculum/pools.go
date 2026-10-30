package curriculum

import (
	_ "embed"
	"hash/fnv"
	"slices"
	"strings"
	"sync"

	"katharsis/internal/catalog"
)

// The catalog splits three ways, so every problem has one place in the
// game (DESIGN.md "Learn mode"): RTFM's module maps (Listed), the trials'
// reserve and the boots' pool (Split). Nothing RTFM shows is a boot door,
// and nothing a trial draws was on a map.

//go:embed listed.txt
var listedText string

// Listed is every catalog problem the guide lists, in any module, at the
// commit the game is built against: RTFM's problems, judgeable or not. It
// is embedded so boots leave them out with the guide not loaded;
// TestListed checks it against a snapshot and -update rewrites it.
var Listed = sync.OnceValue(func() map[catalog.ID]bool {
	s := map[catalog.ID]bool{}
	for _, l := range strings.Split(listedText, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			s[catalog.ID(l)] = true
		}
	}
	return s
})

// listed is every catalog ID c's modules list, sorted: listed.txt.
func (c *Curriculum) listed() []catalog.ID {
	var out []catalog.ID
	for _, m := range c.Modules() {
		for _, l := range m.Problems {
			for _, p := range l.Problems {
				if p.ID != "" {
					out = append(out, p.ID)
				}
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// ReserveCap is the most problems the trials hold back from one cell of
// the pool, a topic within a division's band.
const ReserveCap = 30

// Split divides pool, the problems doors are drawn from by rating (the
// app's drawPool), between the trials and the boots once RTFM's problems
// (Listed) are out of it. The trials hold back half of each cell, a topic
// within a division's band (bandCell), rounded up and at most ReserveCap,
// so a module's trial finds unseen problems on its topic near its rating
// while boots keep most of the catalog. Which half is picked by a hash of
// the ID, not the catalog's order, so a catalog update moves few problems
// across. Problems the catalog cannot judge count toward no cell and go
// to the boots, which never offer them.
func Split(pool []catalog.Problem) (trials, boots []catalog.Problem) {
	listed := Listed()
	type cell struct {
		topic string
		band  int
	}
	cells := map[cell][]int{}
	for i, p := range pool {
		if p.Judgeable && !listed[p.ID] {
			c := cell{p.Topic, bandCell(p.Rating)}
			cells[c] = append(cells[c], i)
		}
	}
	held := make([]bool, len(pool))
	for _, is := range cells {
		slices.SortFunc(is, func(a, b int) int { return compareHash(pool[a].ID, pool[b].ID) })
		for _, i := range is[:min((len(is)+1)/2, ReserveCap)] {
			held[i] = true
		}
	}
	for i, p := range pool {
		switch {
		case listed[p.ID]:
		case held[i]:
			trials = append(trials, p)
		default:
			boots = append(boots, p)
		}
	}
	return trials, boots
}

// bandCell is the division whose band holds rating: the first whose top
// is at or over it, Advanced past Platinum's.
func bandCell(rating int) int {
	for i, d := range []string{"bronze", "silver", "gold", "plat"} {
		if rating <= DivisionBand(d)[1] {
			return i
		}
	}
	return 4
}

func compareHash(a, b catalog.ID) int {
	h := func(id catalog.ID) uint64 {
		f := fnv.New64a()
		f.Write([]byte(id))
		return f.Sum64()
	}
	ha, hb := h(a), h(b)
	switch {
	case ha < hb:
		return -1
	case ha > hb:
		return 1
	}
	return strings.Compare(string(a), string(b))
}
