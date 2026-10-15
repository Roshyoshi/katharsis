package game

import (
	"math"
	"slices"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
)

// The engine logs every set of doors and every route it puts on offer
// (State.Offers, ml-pipeline §4 and slice ML1). Boots played before the
// log cannot be weighed against another door policy, so it comes first:
// each item with its propensity, the chance the draw offered it given the
// state it was drawn in. Logging never draws from the RNG and never
// changes a door: a seed offers the doors it offered before the log
// existed (TestDoorsGolden).

// policyElo names the draw the offers log: Elo's aim, uniform within
// rating windows (pick).
const policyElo = "elo-window/1"

// logOffer appends the doors on offer to State.Offers. rng is State.RNG
// before they were drawn. A door pick did not draw (a regression, the
// regression a re-roll keeps) is logged with propensity 1.
func (e *engine) logOffer(doors []Door, reason string, rng uint64) {
	s, r := e.s, &e.s.Rules
	i := s.Biome
	o := Offer{
		Seq: len(s.Offers), Kind: "doors", Biome: i, Room: s.Room, At: e.now,
		Policy: policyElo, Reason: reason, RNG: rng,
		Target: s.AimClear(i), Floor: r.Biomes[i].MinClear,
		Mu: PlayerRating(r, e.meta, s.Biomes[i].Topic), SetProp: e.setProp, Chosen: -1,
	}
	for pos, d := range doors {
		it := OfferItem{
			Problem: d.Problem, Source: d.Problem.Source(), Topic: d.Topic, Rating: d.Rating,
			P: d.SolveP, PBase: d.SolveP, Propensity: 1, Eligible: 1, Tier: -1,
			Blind: d.Blind, Mystery: d.Mystery, Cursed: d.Cursed, Review: d.Regression,
			BlindWhy: blindWhy(r, s, d), Timer: d.Timer, Bytes: d.Bytes, Position: pos,
		}
		if d.TimerBuy > 1 {
			it.P = lift(d.SolveP, r.BuybackBeta*math.Log(d.TimerBuy))
		}
		if dr, ok := e.draws[d.Problem]; ok && !d.Regression {
			it.Slot, it.Tier, it.Eligible, it.Propensity = dr.slot, dr.tier, dr.eligible, dr.prop
			it.RatingSrc = dr.p.RatingSource
		} else {
			it.Slot = "review"
			if p := e.problem(d.Problem); p != nil {
				it.RatingSrc = p.RatingSource
			}
		}
		o.Items = append(o.Items, it)
	}
	s.Offers = append(s.Offers, o)
}

// logRoute appends a route offer to State.Offers: the topics a biome's
// entrance offers (or the one a fixed biome got), each with the chance
// the draw offered it. A fixed biome's is chosen at once.
func (e *engine) logRoute(i int, topics []string, props []float64, setProp float64, rng uint64, chosen int) {
	s := e.s
	o := Offer{
		Seq: len(s.Offers), Kind: "route", Biome: i, Room: -1, At: e.now,
		Policy: policyElo, Reason: "fresh", RNG: rng, Target: s.AimClear(i),
		Floor: s.Rules.Biomes[i].MinClear, SetProp: setProp, Chosen: chosen,
	}
	if chosen >= 0 {
		o.ChosenAt = e.now
	}
	for pos, t := range topics {
		o.Items = append(o.Items, OfferItem{Topic: t, Slot: "route", Propensity: props[pos], Eligible: 1, Tier: -1, Position: pos})
	}
	s.Offers = append(s.Offers, o)
}

// choose records that the player took item i of the latest offer of kind,
// and returns that offer's index.
func (e *engine) choose(kind string, i int) int {
	s := e.s
	for j := len(s.Offers) - 1; j >= 0; j-- {
		if o := &s.Offers[j]; o.Kind == kind {
			o.Chosen, o.ChosenAt = i, e.now
			return j
		}
	}
	return -1
}

// event appends what just happened on the open door to its log.
func (e *engine) event(ev AttemptEvent) {
	ev.At = e.now
	e.s.Current.Events = append(e.s.Current.Events, ev)
}

// blindWhy is why d's tags are hidden, for the log.
func blindWhy(r *rules.Rules, s *State, d Door) string {
	switch {
	case d.Mastered:
		return "mastery"
	case !d.Blind:
		return ""
	case s.Trial != nil:
		return "trial"
	case r.BlindAll:
		return "run"
	}
	return "roll"
}

// problem is the pool's problem id, nil when the pool has none.
func (e *engine) problem(id catalog.ID) *catalog.Problem {
	if i := slices.IndexFunc(e.pool, func(p catalog.Problem) bool { return p.ID == id }); i >= 0 {
		return &e.pool[i]
	}
	return nil
}
