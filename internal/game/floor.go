package game

import (
	"math"
	"slices"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
)

// Codeforces rates no problem under 800: a problem rated 800 is one whose
// difficulty is 800 or less, often far less (a Div. 2 A, "sum of two
// numbers"). Its rating is an upper bound, censored at the floor. Read as
// exactly 800, it aims a 1000-rated player's Userland doors (576 at 0.92)
// off the pool, prices them as 0.76 clears and sizes their timers for a
// problem 200 points under the player rather than 450.
//
// The engine reads such a door, in the terms of ml-pipeline M1's item
// model, as difficulty b = rating − X with X ~ Exponential(rules.FloorDrop):
// the maximum-entropy spread under a known floor with a known mean drop.
// Its clear chance is the logistic averaged over X (clearChance), and where a
// single number is needed (rating windows, timers, mastery) it is the mean,
// rating − FloorDrop (Difficulty). 687 of the 6,445 judgeable Codeforces
// problems are rated 800 against 240 to 290 at each step above, so most of
// them are clamped from below; FloorDrop 250 puts their mean difficulty
// near 550. A 1000-rated player then clears a floor door 0.89 of the
// time by the engine's reading (0.76 read as exactly 800), and its
// Userland aim of 576 meets the floor's mean (the sim's model player
// draws its own X per problem).

// floored reports whether p's rating is the floor of its rating source:
// at or under rules.FloorRating from one of rules.FloorSources.
func floored(r *rules.Rules, p *catalog.Problem) bool {
	return r.FloorDrop > 0 && p.Rating <= r.FloorRating && slices.Contains(r.FloorSources, p.RatingSource)
}

// Difficulty is the rating the engine reads d at: its Rating, or for a
// door at the pool's floor (Door.Floor) the mean difficulty under it.
func (d Door) Difficulty(r *rules.Rules) float64 {
	if d.Floor {
		return float64(d.Rating) - r.FloorDrop
	}
	return float64(d.Rating)
}

func difficulty(r *rules.Rules, p *catalog.Problem) float64 {
	if floored(r, p) {
		return float64(p.Rating) - r.FloorDrop
	}
	return float64(p.Rating)
}

// floorNodes are the quantiles at which clearChance samples the drop under the
// floor: the midpoints of 16 equal-probability slices of Exponential(1).
var floorNodes = func() [16]float64 {
	var xs [16]float64
	for i := range xs {
		xs[i] = -math.Log(1 - (float64(i)+0.5)/float64(len(xs)))
	}
	return xs
}()

// clearChance is the logistic chance that a player rated player clears a
// door rated rating, read off by off (the calibration): for a floored door
// averaged over the drop under the floor.
func clearChance(r *rules.Rules, player, rating, off float64, floor bool) float64 {
	if !floor {
		return Expected(player, rating-off)
	}
	sum := 0.0
	for _, x := range floorNodes {
		sum += Expected(player, rating-off-r.FloorDrop*x)
	}
	return sum / float64(len(floorNodes))
}
