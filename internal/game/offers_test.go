package game_test

import (
	"fmt"
	"math"
	"testing"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// chiSquareP is the upper tail of the chi-square distribution with k
// degrees of freedom at x, by the Wilson–Hilferty cube-root normal
// approximation (good to about 0.01 in p for k >= 10).
func chiSquareP(x float64, k int) float64 {
	kf := float64(k)
	z := (math.Cbrt(x/kf) - (1 - 2/(9*kf))) / math.Sqrt(2/(9*kf))
	return 0.5 * math.Erfc(z/math.Sqrt2)
}

// The logged propensities are the draw's: over 10^4 seeds each problem is
// offered in the first room as often as the propensities logged for it
// add up to (chi-square, p > 0.01), whether the room fills from one rating
// window or from several.
func TestOfferPropensities(t *testing.T) {
	t.Parallel()
	at := func(topic string, rating, n int) []catalog.Problem {
		var ps []catalog.Problem
		for i := range n {
			ps = append(ps, catalog.Problem{ID: catalog.ID(fmt.Sprintf("%s:%d:%d", topic, rating, i)),
				Topic: topic, Rating: rating, Judgeable: true})
		}
		return ps
	}
	// A 1500 player's Userland aims at 1076: the first pool fills the room
	// from one window; the second has two doors within 100, so the third
	// comes from the 250 window, and four more lie past it.
	pools := map[string][]catalog.Problem{
		"one window": dense(catalog.Topics, 900, 1300, 10),
		"two windows": append(append(at("math", 1100, 2), at("math", 1250, 30)...),
			at("math", 1700, 4)...),
	}
	const seeds = 10000
	for name, ps := range pools {
		// The room is the same every seed, so each problem's propensity is
		// too: it is offered seeds x propensity times on average.
		prop := map[catalog.ID]float64{}
		seen := map[catalog.ID]float64{}
		for seed := range uint64(seeds) {
			h := newHarness(t, plain(), &game.Meta{Rating: 1500}, ps, 0, seed)
			o := h.s.Offers[len(h.s.Offers)-1]
			if o.Kind != "doors" || len(o.Items) != len(h.s.Doors) || o.Chosen != -1 {
				t.Fatalf("%s seed %d: offer %+v for doors %+v", name, seed, o, h.s.Doors)
			}
			set := 1.0
			for i, it := range o.Items {
				if it.Problem != h.s.Doors[i].Problem || it.Position != i {
					t.Fatalf("%s seed %d: item %d %+v for door %+v", name, seed, i, it, h.s.Doors[i])
				}
				if p, ok := prop[it.Problem]; ok && math.Abs(p-it.Propensity) > 1e-12 {
					t.Fatalf("%s seed %d: %s logged at %g, before at %g", name, seed, it.Problem, it.Propensity, p)
				}
				prop[it.Problem] = it.Propensity
				seen[it.Problem]++
				set *= it.Propensity
			}
			if o.SetProp <= 0 || o.SetProp > set+1e-12 {
				t.Fatalf("%s seed %d: set propensity %g, items' product %g", name, seed, o.SetProp, set)
			}
		}
		// Every problem the room could offer is offered by some seed (the
		// least likely about 90 times), so the counts cover them all.
		chi, k := 0.0, -1
		for id, p := range prop {
			e := seeds * p
			chi += (seen[id] - e) * (seen[id] - e) / e
			k++
		}
		if p := chiSquareP(chi, k); p < 0.01 {
			t.Errorf("%s: chi-square %.1f on %d df, p %.4f: inclusion does not match the logged propensities", name, chi, k, p)
		} else {
			t.Logf("%s: chi-square %.1f on %d df, p %.3f", name, chi, k, p)
		}
	}
}
