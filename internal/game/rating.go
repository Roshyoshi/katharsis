package game

import (
	"math"

	"deadcases/internal/rules"
)

// Mixed is the catalog topic for problems with no mapped tag. A biome on
// Mixed draws doors from every topic, and it has no rating of its own.
const Mixed = "mixed"

// Expected is the logistic (Elo) chance that a player rated player clears a
// door rated door: 1/(1+10^((door-player)/400)).
func Expected(player, door float64) float64 {
	return 1 / (1 + math.Pow(10, (door-player)/400))
}

// Aim inverts Expected: the door rating a player rated player clears with
// probability p. p is clamped away from 0 and 1, where the logit diverges.
func Aim(player, p float64) float64 {
	p = min(max(p, 0.01), 0.99)
	return player + 400*math.Log10(1/p-1)
}

// PlayerRating is the rating doors in topic are aimed with: the topic's own
// rating once it has rules.TopicRatingMin attempts behind it, else the
// overall rating, else the cold start.
func PlayerRating(r *rules.Rules, m *Meta, topic string) float64 {
	if topic != Mixed && m.TopicAttempts[topic] >= r.TopicRatingMin {
		if tr, ok := m.TopicRating[topic]; ok {
			return tr
		}
	}
	if m.Rating > 0 {
		return m.Rating
	}
	return r.ColdStartRating
}

// rate is one Elo update of the overall and topic ratings against a closed
// door. A topic seen for the first time starts at the overall rating.
func rate(r *rules.Rules, m *Meta, d Door, score float64) {
	overall := m.Rating
	if overall <= 0 {
		overall = r.ColdStartRating
	}
	m.Rating = overall + r.RatingK*(score-Expected(overall, float64(d.Rating)))
	if d.Topic == "" || d.Topic == Mixed {
		return
	}
	if m.TopicRating == nil {
		m.TopicRating = map[string]float64{}
	}
	if m.TopicAttempts == nil {
		m.TopicAttempts = map[string]int{}
	}
	tr, ok := m.TopicRating[d.Topic]
	if !ok {
		tr = overall
	}
	m.TopicRating[d.Topic] = tr + r.RatingK*(score-Expected(tr, float64(d.Rating)))
	m.TopicAttempts[d.Topic]++
}
