package game

import (
	"math"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
)

// Mixed is catalog.Mixed, the topic of untagged problems. A biome on Mixed
// draws doors from every topic, and it has no rating of its own.
const Mixed = catalog.Mixed

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
//
// Every topic rating also follows TopicFollow of the overall rating's
// change, whatever the door's topic, so a topic rating is the overall plus
// what its own doors taught. Without that, a topic first seen during the
// cold start keeps the cold rating it started from: a topic sees about a
// third of a door per boot, and at K 40 its own doors take a hundred boots
// to close a 200-point gap, all the while aiming its doors at a player who
// is no longer there.
func rate(r *rules.Rules, m *Meta, d Door, score float64) {
	overall := m.Rating // NewBoot and NewTrial set a cold start
	k := ratingK(r, m)
	m.Rated++
	m.Rating = overall + k*(score-Expected(overall, float64(d.Rating)))
	tr, seen := m.TopicRating[d.Topic]
	follow := r.TopicFollow * (m.Rating - overall)
	if follow != 0 {
		for t := range m.TopicRating {
			m.TopicRating[t] += follow
		}
	}
	if d.Topic == Mixed {
		return
	}
	if m.TopicRating == nil {
		m.TopicRating = map[string]float64{}
	}
	if m.TopicAttempts == nil {
		m.TopicAttempts = map[string]int{}
	}
	base := tr + follow
	if !seen {
		// Its first door moves it as it moved the overall, from the
		// overall: following as well would count the door twice.
		tr, base = overall, overall
	}
	m.TopicRating[d.Topic] = base + k*(score-Expected(tr, float64(d.Rating)))
	m.TopicAttempts[d.Topic]++
}

// ratingK is the Elo K for the next rated attempt. A newcomer's ratings
// move with RatingKCold until RatingColdAttempts are behind them: the cold
// start rating is a guess, and the aim makes most doors easy (a clear at
// 90% moves the rating by a tenth of K), so at the steady-state K a strong
// newcomer would grind boots of doors far below them. Topic ratings use
// the same K: a topic first seen during the cold start starts from a cold
// overall rating and has to catch up with it.
func ratingK(r *rules.Rules, m *Meta) float64 {
	if rated(m) >= r.RatingColdAttempts {
		return r.RatingK
	}
	return max(r.RatingKCold, r.RatingK)
}

// rated is how many rated attempts m has made. Saves from before Meta.Rated
// existed count their topic attempts instead, which undercounts (mixed
// doors were never counted) but keeps an old player from being put back
// into a cold start.
func rated(m *Meta) int {
	t := 0
	for _, c := range m.TopicAttempts {
		t += c
	}
	return max(m.Rated, t)
}

// PrestigeGate is what unlocking the next prestige still needs (D31). A
// boot cleared at the highest unlocked prestige unlocks the next, but only
// once Need doors are rated: before calibration starts, a strong
// newcomer's early boots are easy and would climb every prestige in a
// handful of clears.
type PrestigeGate struct {
	Next  int // the prestige the next unlock opens; 0 when all are open
	Rated int // rated doors so far
	Need  int // rated doors an unlock needs (rules.CalibAfter)
}

// NextPrestige is m's gate under r.
func NextPrestige(r rules.Rules, m *Meta) PrestigeGate {
	g := PrestigeGate{Rated: rated(m), Need: r.CalibAfter}
	if m.Prestige < MaxPrestige {
		g.Next = m.Prestige + 1
	}
	return g
}

// Open reports whether clearing a boot at the highest unlocked prestige
// unlocks Next now.
func (g PrestigeGate) Open() bool { return g.Next > 0 && g.Rated >= g.Need }

// Left is how many more doors must be rated before the gate opens.
func (g PrestigeGate) Left() int { return max(g.Need-g.Rated, 0) }
