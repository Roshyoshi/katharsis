package game

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"deadcases/internal/catalog"
	"deadcases/internal/rules"
)

func topicOf(p *catalog.Problem) string {
	if p.Topic == "" {
		return Mixed
	}
	return p.Topic
}

// target is the door rating at which the player clears topic doors with
// probability clear, raised by the prestige offset.
func (e *engine) target(topic string, clear float64) float64 {
	r := &e.s.Rules
	return Aim(PlayerRating(r, e.meta, topic), clear) + float64(e.s.Prestige)*r.PrestigeOffset
}

// attempted is every problem opened this boot.
func (e *engine) attempted() map[catalog.ID]bool {
	ex := map[catalog.ID]bool{}
	for _, a := range e.s.History {
		ex[a.Door.Problem] = true
	}
	if e.s.Current != nil {
		ex[e.s.Current.Door.Problem] = true
	}
	return ex
}

// excluded is what doors must not offer: problems opened this boot, and
// problems the player solved without help (hinted or editorial solves can
// come back).
func (e *engine) excluded() map[catalog.ID]bool {
	ex := e.attempted()
	for _, s := range e.meta.Solves {
		if s.Kind == Independent {
			ex[s.Problem] = true
		}
	}
	return ex
}

type cand struct {
	p    *catalog.Problem
	dist float64 // from the target of the problem's own topic
	in   bool    // in the biome's topic
}

// candidates lists eligible problems, sorted by ID so the pick depends on the
// seed and not on the order the catalog listed them in.
func (e *engine) candidates(topic string, clear float64, ex map[catalog.ID]bool) []cand {
	targets := map[string]float64{}
	var cs []cand
	for i := range e.pool {
		p := &e.pool[i]
		if !p.Judgeable || ex[p.ID] {
			continue
		}
		t := topicOf(p)
		tg, ok := targets[t]
		if !ok {
			tg = e.target(t, clear)
			targets[t] = tg
		}
		cs = append(cs, cand{p, math.Abs(float64(p.Rating) - tg), topic == Mixed || t == topic})
	}
	slices.SortFunc(cs, func(a, b cand) int { return cmp.Compare(a.p.ID, b.p.ID) })
	return cs
}

// pick fills n doors from cs, tier by tier: the biome's topic at each rating
// window, then every topic at each window, then the nearest left at any
// rating. Going off-topic before off-rating is deliberate: a dp door 1000
// points too hard has a solve chance near zero, while a graph door at the
// right rating in a dp biome is merely off-theme.
//
// Within a window the pick is uniform. Past the last window it is not: once
// a player has solved everything near their level (a few hundred boots on
// USACO), a uniform pick over the whole rating range would offer 2600s to a
// 1600 player; the nearest 2n leftovers keep doors as close as the pool
// allows while still varying.
func (e *engine) pick(cs []cand, n int) []*catalog.Problem {
	type tier struct {
		in bool
		w  float64
	}
	var tiers []tier
	for _, in := range []bool{true, false} {
		for _, w := range e.s.Rules.RatingWindows {
			tiers = append(tiers, tier{in, float64(w)})
		}
	}
	tiers = append(tiers, tier{true, math.Inf(1)}, tier{false, math.Inf(1)})

	chosen := map[catalog.ID]bool{}
	var out []*catalog.Problem
	for _, t := range tiers {
		if len(out) >= n {
			break
		}
		var fit []cand
		for _, c := range cs {
			if !chosen[c.p.ID] && (c.in || !t.in) && c.dist <= t.w {
				fit = append(fit, c)
			}
		}
		if math.IsInf(t.w, 1) {
			slices.SortStableFunc(fit, func(a, b cand) int { return cmp.Compare(a.dist, b.dist) })
			fit = fit[:min(len(fit), 2*(n-len(out)))]
		}
		for _, c := range sample(e.rng, fit, n-len(out)) {
			if !chosen[c.p.ID] { // a Union catalog can list an ID twice
				chosen[c.p.ID] = true
				out = append(out, c.p)
			}
		}
	}
	// The closest tier fills first; shuffle so door 1 isn't always nearest.
	e.rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func (e *engine) roomSize(b *BiomeState) int {
	r := &e.s.Rules
	if b.Kind == rules.Boss {
		return r.BossDoors
	}
	h := e.s.History
	if slices.Contains(e.s.Daemons, Fork) && len(h) > 0 && h[len(h)-1].Outcome == Cleared {
		return max(r.ForkDoors, r.DoorsPerRoom)
	}
	return r.DoorsPerRoom
}

// rollRoom offers the current room's doors. fresh is false for the re-roll
// after a retreat, which never injects the regression door again.
//
// When everything near the player is used up, eligibility relaxes in steps
// (allow independently solved, then anything judgeable) rather than leave a
// room empty: a boot can always continue while one problem is judgeable.
func (e *engine) rollRoom(fresh bool) {
	s := e.s
	b := &s.Biomes[s.Biome]
	rb := s.Rules.Biomes[s.Biome]
	n := e.roomSize(b)

	var ps []*catalog.Problem
	for _, ex := range []map[catalog.ID]bool{e.excluded(), e.attempted(), {}} {
		ps = e.pick(e.candidates(b.Topic, rb.TargetClear, ex), n)
		if len(ps) > 0 {
			break
		}
	}
	doors := make([]Door, 0, n)
	for _, p := range ps {
		doors = append(doors, e.makeDoor(p, rb, b.Boss, false))
	}
	if fresh {
		if d, ok := e.regressionDoor(rb, doors); ok {
			if len(doors) > 0 {
				doors[len(doors)-1] = d
			} else {
				doors = append(doors, d)
			}
		}
	}
	s.Doors = doors

	ids := make([]catalog.ID, len(doors))
	desc := make([]string, len(doors))
	for i, d := range doors {
		ids[i] = d.Problem
		desc[i] = fmt.Sprint(d.Rating)
		if m := doorMods(d); m != "" {
			desc[i] += "(" + m + ")"
		}
	}
	e.logf("room_%d.%d: %d doors at %s", s.Biome, s.Room, len(doors), strings.Join(desc, " "))
	e.emit(Prefetch{Problems: ids})
}

// regressionDoor is the boot's one due regression, offered in the first room
// of a non-boss biome after the first. Its problem must still be in the pool
// (for rating and topic) and not opened this boot.
func (e *engine) regressionDoor(rb rules.Biome, offered []Door) (Door, bool) {
	s := e.s
	if s.RegressionOffered || s.Biome < 1 || s.Room != 0 || rb.Kind == rules.Boss {
		return Door{}, false
	}
	seen := e.attempted()
	for _, d := range offered {
		seen[d.Problem] = true
	}
	for _, g := range DueRegressions(e.meta, e.now) {
		i := slices.IndexFunc(e.pool, func(p catalog.Problem) bool { return p.ID == g.Problem })
		if i < 0 || !e.pool[i].Judgeable || seen[g.Problem] {
			continue
		}
		s.RegressionOffered = true
		e.logf("regression: %s is back (stage %d)", g.Problem, g.Stage)
		return e.makeDoor(&e.pool[i], rb, "", true), true
	}
	return Door{}, false
}

// makeDoor rolls a door's modifiers and prices it. It always draws three
// numbers, so one door's modifiers never shift the next door's rolls.
func (e *engine) makeDoor(p *catalog.Problem, rb rules.Biome, boss string, regression bool) Door {
	r := &e.s.Rules
	asc := r.Ascend(e.s.Prestige)
	topic := topicOf(p)
	R := PlayerRating(r, e.meta, topic)
	d := Door{
		Problem:    p.ID,
		Topic:      topic,
		Rating:     p.Rating,
		SolveP:     Expected(R, float64(p.Rating)),
		Regression: regression,
		Boss:       boss,
	}
	blind, mystery, cursed := e.rng.Float64(), e.rng.Float64(), e.rng.Float64()
	d.Mystery = mystery < r.MysteryChance+asc.MysteryChance
	d.Blind = r.BlindAll || d.Mystery || blind < r.BlindChance+asc.BlindChance
	d.Cursed = boss == "" && !regression && cursed < r.CursedChance+asc.CursedChance
	d.Timer = e.doorTimer(rb, boss != "", float64(p.Rating)-R)
	d.Bytes = e.doorBytes(d)
	return d
}

// doorBytes prices a door. The difficulty factor 0.5/SolveP makes a door's
// expected payout the same at every difficulty (BaseBytes/2), so harder
// doors pay for their risk and the modifiers decide between doors. Mystery
// pays MysteryMult instead of BlindMult (it implies blind, and stacking would
// make a hidden title worth more than a curse).
func (e *engine) doorBytes(d Door) int64 {
	r := &e.s.Rules
	mult := min(max(0.5/d.SolveP, r.MinBytesMult), r.MaxBytesMult)
	switch {
	case d.Mystery:
		mult *= r.MysteryMult
		if slices.Contains(e.s.Daemons, DevRandom) {
			mult *= r.DevRandomMult
		}
	case d.Blind:
		mult *= r.BlindMult
	}
	if d.Cursed {
		mult *= r.CursedMult
	}
	if d.Boss != "" {
		mult *= r.BossMult
	}
	return max(1, int64(math.Round(float64(r.BaseBytes)*mult)))
}

// routeOffers picks k topics for a biome: unlocked, and with at least
// Rooms·RouteMinPool eligible problems within the widest rating window.
// Topics not yet offered this boot go first. Mixed is the fallback, so a
// route is never empty.
func (e *engine) routeOffers(rb rules.Biome, k int, used map[string]bool) []string {
	r := &e.s.Rules
	widest := 0.0
	for _, w := range r.RatingWindows {
		widest = max(widest, float64(w))
	}
	ex := e.excluded()
	targets := map[string]float64{}
	count := map[string]int{}
	for i := range e.pool {
		p := &e.pool[i]
		t := topicOf(p)
		if !p.Judgeable || ex[p.ID] || t == Mixed {
			continue
		}
		tg, ok := targets[t]
		if !ok {
			tg = e.target(t, rb.TargetClear)
			targets[t] = tg
		}
		if math.Abs(float64(p.Rating)-tg) <= widest {
			count[t]++
		}
	}
	var fresh, reused []string
	for _, t := range catalog.Topics {
		if t == Mixed || !Unlocked(e.meta, t) || count[t] < rb.Rooms*r.RouteMinPool {
			continue
		}
		if used[t] {
			reused = append(reused, t)
		} else {
			fresh = append(fresh, t)
		}
	}
	offers := sample(e.rng, fresh, k)
	offers = append(offers, sample(e.rng, reused, k-len(offers))...)
	if len(offers) == 0 {
		offers = []string{Mixed}
	}
	return offers
}

// layout builds the boot's biomes from the rules: topics for fixed biomes,
// route offers for choice biomes, and boss keys (distinct per boot until
// they run out; The Halting Problem ends a max-prestige boot).
func (e *engine) layout() {
	s := e.s
	r := &s.Rules
	keys := sample(e.rng, BossKeys, len(BossKeys))
	used := map[string]bool{}
	bosses := 0
	for i, rb := range r.Biomes {
		b := BiomeState{Name: rb.Name, Kind: rb.Kind, Rooms: rb.Rooms}
		switch rb.Kind {
		case rules.Boss:
			b.Rooms, b.Topic = 1, Mixed
			b.Boss = keys[bosses%len(keys)]
			bosses++
			if s.Prestige == MaxPrestige && i == len(r.Biomes)-1 {
				b.Boss = FinalBoss
			}
		case rules.Choice:
			b.Offers = e.routeOffers(rb, rb.Choices, used)
			for _, t := range b.Offers {
				used[t] = true
			}
		default:
			b.Topic = Mixed
			if i > 0 {
				b.Topic = e.routeOffers(rb, 1, used)[0]
				used[b.Topic] = true
			}
		}
		s.Biomes = append(s.Biomes, b)
	}
}

func doorMods(d Door) string {
	var m []string
	for _, f := range []struct {
		on   bool
		name string
	}{
		{d.Boss != "", "boss " + d.Boss}, {d.Regression, "regression"},
		{d.Mystery, "mystery"}, {d.Blind && !d.Mystery, "blind"}, {d.Cursed, "cursed"},
	} {
		if f.on {
			m = append(m, f.name)
		}
	}
	return strings.Join(m, " ")
}
