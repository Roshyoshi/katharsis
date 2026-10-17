package game

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
)

func topicOf(p *catalog.Problem) string {
	if p.Topic == "" {
		return Mixed
	}
	return p.Topic
}

// target is the door rating at which the player clears biome i's topic
// doors with probability AimClear(i) (its TargetClear unless an easing
// raised it), raised by the prestige offset but never past where they
// would clear rb.MinClear, and corrected by the tier's calibration. The
// floor applies before the calibration, which corrects the logistic's
// shape at whatever rating the floor leaves.
func (e *engine) target(topic string, i int) float64 {
	r := &e.s.Rules
	rb := r.Biomes[i]
	R := PlayerRating(r, e.meta, topic)
	aim := Aim(R, e.s.AimClear(i)) + float64(e.s.Prestige)*r.PrestigeOffset
	if rb.MinClear > 0 {
		aim = min(aim, Aim(R, rb.MinClear))
	}
	return aim + e.aimOffset(rb)
}

// Target is the door rating the current biome aims topic doors at for m:
// what the engine is trying to offer, before the pool decides what it can.
func (s *State) Target(m *Meta, topic string) float64 {
	e := &engine{s: s, meta: m}
	return e.target(topic, s.Biome)
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
	// sp is its SolveP as a door of the biome; low is set when that is
	// under the biome's MinClear, which the rating windows pass over (D22).
	sp  float64
	low bool
}

// candidates lists eligible problems for biome i, sorted by ID so the pick
// depends on the seed and not on the order the catalog listed them in.
func (e *engine) candidates(topic string, i int, ex map[catalog.ID]bool) []cand {
	rb := e.s.Rules.Biomes[i]
	off := e.aimOffset(rb)
	// Boot #1 offers only problems with an editorial, while the pool has
	// some (RT11): its first death should teach.
	editorial := false
	if es := e.s.active(EaseFirst); es != nil && es.Editorial {
		editorial = slices.ContainsFunc(e.pool, func(p catalog.Problem) bool { return p.Judgeable && p.HasEditorial && !ex[p.ID] })
	}
	type aim struct{ target, rating float64 }
	aims := map[string]aim{}
	var cs []cand
	for j := range e.pool {
		p := &e.pool[j]
		if !p.Judgeable || ex[p.ID] || editorial && !p.HasEditorial {
			continue
		}
		t := topicOf(p)
		a, ok := aims[t]
		if !ok {
			a = aim{e.target(t, i), PlayerRating(&e.s.Rules, e.meta, t)}
			aims[t] = a
		}
		floor := floored(&e.s.Rules, p)
		sp := clearChance(&e.s.Rules, a.rating, float64(p.Rating), off, floor)
		cs = append(cs, cand{p, math.Abs(difficulty(&e.s.Rules, p) - a.target), topic == Mixed || t == topic, sp, rb.MinClear > 0 && sp < rb.MinClear})
	}
	slices.SortFunc(cs, func(a, b cand) int { return cmp.Compare(a.p.ID, b.p.ID) })
	return cs
}

// tier is one step of pick: candidates in the biome's topic only (in) or
// any, within w rating points of their target.
type tier struct {
	in bool
	w  float64
}

// tiers are pick's steps in order: the biome's topic at each rating
// window, then every topic at each window, then the nearest left.
func (e *engine) tiers() []tier {
	var ts []tier
	for _, in := range []bool{true, false} {
		for _, w := range e.s.Rules.RatingWindows {
			ts = append(ts, tier{in, float64(w)})
		}
	}
	return append(ts, tier{true, math.Inf(1)}, tier{false, math.Inf(1)})
}

// fit is what tier t draws need doors from: every candidate of cs not
// chosen that fits a window (and is not under the biome's floor), or the
// 2·need nearest past the windows.
func (t tier) fit(cs []cand, chosen map[catalog.ID]bool, need int) []cand {
	var fit []cand
	window := !math.IsInf(t.w, 1)
	for _, c := range cs {
		if !chosen[c.p.ID] && (c.in || !t.in) && c.dist <= t.w && (!window || !c.low) {
			fit = append(fit, c)
		}
	}
	if !window {
		slices.SortStableFunc(fit, func(a, b cand) int { return cmp.Compare(a.dist, b.dist) })
		fit = fit[:min(len(fit), 2*need)]
	}
	return fit
}

// drawn is a problem pick put on offer, and how: the tier (step) it was
// drawn in, by index and name, among how many, and the chance that step
// drew it (ml-pipeline §4: the propensity OfferItem logs).
type drawn struct {
	p        *catalog.Problem
	slot     string
	tier     int
	eligible int
	prop     float64
}

// slot names a tier for the log: "w100" (the biome's topic within 100),
// "any250", "near", "near-any".
func (t tier) slot() string {
	name := "near"
	if !math.IsInf(t.w, 1) {
		name = fmt.Sprintf("w%.0f", t.w)
	}
	if !t.in {
		if name == "near" {
			return "near-any"
		}
		return "any" + name[1:]
	}
	return name
}

// binomial is C(n, k) as a float.
func binomial(n, k int) float64 {
	c := 1.0
	for i := range k {
		c = c * float64(n-i) / float64(i+1)
	}
	return c
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
//
// The windows also pass over doors below the biome's MinClear (D22): the
// prestige offset stops at the floor, but a ±100 window around it would
// still put half its doors under. Only the nearest leftovers may be that
// hard, once nothing above the floor is left.
func (e *engine) pick(cs []cand, n int) []drawn {
	chosen := map[catalog.ID]bool{}
	var out []drawn
	for ti, t := range e.tiers() {
		if len(out) >= n {
			break
		}
		fit := t.fit(cs, chosen, n-len(out))
		// Each of fit is as likely as the next to be among the k drawn.
		k := min(n-len(out), len(fit))
		for _, c := range sample(e.rng, fit, n-len(out)) {
			if !chosen[c.p.ID] { // a Union catalog can list an ID twice
				chosen[c.p.ID] = true
				out = append(out, drawn{c.p, t.slot(), ti, len(fit), float64(k) / float64(len(fit))})
			}
		}
		if k > 0 {
			e.setProp /= binomial(len(fit), k)
		}
	}
	// The closest tier fills first; shuffle so door 1 isn't always nearest.
	e.rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// roomSize is how many doors a room offers: a boss room BossDoors, else
// DoorsPerRoom plus what packages add (tmux), or ForkDoors after an AC
// with fork running, whichever is more. Fork and the packages do not
// stack: each more door is one more chance to dodge a cursed or blind
// one, which prestige adds as risk (D22). So with tmux fork adds nothing,
// and is not offered (idle).
func (e *engine) roomSize(b *BiomeState) int {
	r := &e.s.Rules
	if b.Kind == rules.Boss {
		return r.BossDoors
	}
	n := r.DoorsPerRoom
	if e.s.Trial == nil { // a trial's rooms hold its plan's one door
		n += r.Grants(e.s.Packages).Doors
	}
	h := e.s.History
	if slices.Contains(e.s.Daemons, Fork) && len(h) > 0 && h[len(h)-1].Outcome == Cleared {
		n = max(n, r.ForkDoors)
	}
	return n + e.s.Smt // smt's, for this room
}

// rollRoom offers the current room's doors. fresh is false for the re-roll
// after a retreat, which never injects the regression door again; reason
// is what the offer logs (Offer.Reason).
func (e *engine) rollRoom(fresh bool, reason string) {
	rng := e.s.RNG
	doors := e.drawDoors(nil)
	// A regression door needs a judgeable problem, so the room has a door
	// for it to replace.
	if fresh && e.s.Trial == nil {
		if d, ok := e.regressionDoor(doors); ok {
			doors[len(doors)-1] = d
		}
	}
	e.offer(doors, reason, rng)
}

// drawDoors draws the current room's doors, never none, and none of skip
// while the pool has others.
//
// When everything near the player is used up, eligibility relaxes in steps
// (allow independently solved, then anything judgeable) rather than leave a
// room empty: a boot can always continue while one problem is judgeable.
func (e *engine) drawDoors(skip []Door) []Door {
	s := e.s
	b := &s.Biomes[s.Biome]
	n := e.roomSize(b)

	e.setProp = 1
	var ps []drawn
	if s.Trial != nil {
		ps = []drawn{{p: e.trialProblem(), slot: "trial", tier: -1, eligible: 1, prop: 1}}
	} else {
		for i, ex := range []map[catalog.ID]bool{e.excluded(), e.attempted(), {}} {
			for _, d := range skip {
				if i < 2 { // the last resort takes anything
					ex[d.Problem] = true
				}
			}
			e.setProp = 1
			cs := e.candidates(b.Topic, s.Biome, ex)
			if s.firstRoom() {
				cs = e.floorOf(cs)
			}
			ps = e.pick(cs, n)
			if len(ps) > 0 {
				break
			}
		}
	}
	doors := make([]Door, 0, n)
	e.draws = map[catalog.ID]drawn{}
	for _, d := range ps {
		doors = append(doors, e.makeDoor(d.p, s.Biome, b.Boss, false))
		e.draws[d.p.ID] = d
	}
	return doors
}

// offer puts doors on offer in the current room, logs them and has the app
// fetch them ahead. The room's safe doors (Biome.Plain) and the floor
// easing's timers are settled here, after any regression door joined.
func (e *engine) offer(doors []Door, reason string, rng uint64) {
	s := e.s
	// smt's door and whatis's sight are for the one room they were bought
	// for; a retreat's re-roll is plain again.
	s.Smt = 0
	if s.Whatis > 0 {
		s.Whatis--
	}
	e.plainDoors(doors)
	e.buyTimers(doors)
	s.Doors = doors
	e.logOffer(doors, reason, rng)
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
func (e *engine) regressionDoor(offered []Door) (Door, bool) {
	s := e.s
	if s.RegressionOffered || s.Biome < 1 || s.Room != 0 || s.Rules.Biomes[s.Biome].Kind == rules.Boss {
		return Door{}, false
	}
	seen := e.attempted()
	for _, d := range offered {
		seen[d.Problem] = true
	}
	for _, g := range DueRegressions(e.meta, e.now) {
		p := e.problem(g.Problem)
		if p == nil || !p.Judgeable || seen[g.Problem] {
			continue
		}
		s.RegressionOffered = true
		e.logf("regression: %s is back (stage %d)", g.Problem, g.Stage)
		return e.makeDoor(p, s.Biome, "", true), true
	}
	return Door{}, false
}

// makeDoor rolls a door's modifiers and prices it. It always draws three
// numbers, so one door's modifiers never shift the next door's rolls.
//
// SolveP takes the tier's calibration offset back out of the rating: the
// offset moved the door to where the player really clears at the target
// rate, so the logistic at the uncorrected rating is the better estimate,
// and pricing by it keeps a door's expected payout at BaseBytes/2 whatever
// the calibration has learned.
func (e *engine) makeDoor(p *catalog.Problem, i int, boss string, regression bool) Door {
	r := &e.s.Rules
	rb := r.Biomes[i]
	asc := r.Ascend(e.s.Prestige)
	topic := topicOf(p)
	R := PlayerRating(r, e.meta, topic)
	d := Door{
		Problem:    p.ID,
		Topic:      topic,
		Rating:     p.Rating,
		Floor:      floored(r, p),
		Regression: regression,
		Boss:       boss,
	}
	d.SolveP = clearChance(r, R, float64(p.Rating), e.aimOffset(rb), d.Floor)
	blind, mystery, cursed := e.rng.Float64(), e.rng.Float64(), e.rng.Float64()
	d.Mystery = mystery < r.MysteryChance+asc.MysteryChance
	d.Blind = r.BlindAll || d.Mystery || blind < r.BlindChance+asc.BlindChance
	// A cursed node (the Trial) curses every door; the roll is drawn anyway.
	d.Cursed = rb.Cursed || boss == "" && !regression && cursed < r.CursedChance+asc.CursedChance
	if e.s.perks() {
		// selinux enforcing: every topic door cursed (a regression aside, as
		// the roll spares it). whatis: the room shows what each door is.
		if e.s.has(Selinux) && boss == "" && !regression {
			d.Cursed = true
		}
		if e.s.Whatis > 0 {
			d.Blind, d.Mystery = false, false
		}
	}
	if e.s.Trial != nil {
		// The unit shows tags; the integration doors are blind.
		d.Blind, d.Mystery, d.Cursed = e.s.Biome > 0 || r.BlindAll, false, false
	}
	d.Timer = e.doorTimer(rb, boss != "", d.Difficulty(r)-R)
	e.twist(&d)
	e.dress(&d, i)
	return d
}

// dress sets what follows from a door's modifiers: whether mastery hides
// its tags, the calibration tier it teaches, and its bytes.
func (e *engine) dress(d *Door, i int) {
	r := &e.s.Rules
	// A mastered door teaches the calibration like any plain door: once most
	// doors of the known topics hide their tags, they are most of what the
	// aim can learn from.
	d.Mastered = e.s.Trial == nil && !d.Blind && mastered(r, e.meta, d.Topic, d.Difficulty(r))
	plain := !d.Blind && !d.Cursed && !d.Regression && e.s.Trial == nil && d.TimerBuy == 0
	d.Tier = ""
	if plain && math.Abs(d.Difficulty(r)-e.target(d.Topic, i)) <= float64(r.RatingWindows[0]) {
		d.Tier = CalibrationTier(r.Biomes[i])
	}
	d.Bytes = e.doorBytes(*d)
}

// floorOf is the easiest of cs, those within the first rating window of
// the lowest difficulty: boot #1's first doors (RT11).
func (e *engine) floorOf(cs []cand) []cand {
	if len(cs) == 0 {
		return cs
	}
	r := &e.s.Rules
	low := difficulty(r, cs[0].p)
	for _, c := range cs {
		low = min(low, difficulty(r, c.p))
	}
	var out []cand
	for _, c := range cs {
		if difficulty(r, c.p) <= low+float64(r.RatingWindows[0]) {
			c.in = true
			out = append(out, c)
		}
	}
	return out
}

// plainDoors takes the risk modifiers (blind, mystery, cursed) off the
// biome's Plain easiest doors on offer, regressions aside, so a safe door
// is always there (ml-pipeline M4: Userland's easy slot). Their modifiers
// were rolled already, which keeps the draws per door fixed.
func (e *engine) plainDoors(doors []Door) {
	s := e.s
	n := s.Rules.Biomes[s.Biome].Plain
	first := s.firstRoom()
	if first {
		n = len(doors) // boot #1's first room: every door plain, tags shown
	}
	if n <= 0 || s.Trial != nil {
		return
	}
	idx := make([]int, 0, len(doors))
	for i, d := range doors {
		if !d.Regression {
			idx = append(idx, i)
		}
	}
	// Easiest first; ties keep the display order.
	slices.SortStableFunc(idx, func(a, b int) int { return cmp.Compare(doors[b].SolveP, doors[a].SolveP) })
	for _, i := range idx[:min(n, len(idx))] {
		d := &doors[i]
		if d.Cursed || d.Mystery || d.Blind && !s.Rules.BlindAll || first && d.Mastered {
			d.Blind, d.Mystery, d.Cursed = s.Rules.BlindAll, false, false
			e.dress(d, s.Biome)
			if first {
				d.Mastered = false
			}
		}
	}
}

// mastered reports whether a door rated rating on topic hides its tags for
// m: the topic has r.MasteryAttempts rated attempts behind it and a rating
// at least r.MasteryMargin above the door's. Mixed has no rating of its
// own, so its doors keep their tags.
func mastered(r *rules.Rules, m *Meta, topic string, rating float64) bool {
	if r.MasteryAttempts <= 0 || topic == Mixed || m.TopicAttempts[topic] < r.MasteryAttempts {
		return false
	}
	tr, ok := m.TopicRating[topic]
	return ok && tr-rating >= r.MasteryMargin
}

// doorBytes prices a door. The difficulty factor 0.5/SolveP makes a door's
// expected payout the same at every difficulty (BaseBytes/2), so harder
// doors pay for their risk and the modifiers decide between doors. Mystery
// pays MysteryMult instead of BlindMult (it implies blind, and stacking would
// make a hidden title worth more than a curse). The daemons that pay more
// (dev-random, cron, chrt) or less (nice) multiply whole doors, so the
// payout stays flat across difficulty.
func (e *engine) doorBytes(d Door) int64 {
	r := &e.s.Rules
	mult := min(max(0.5/d.SolveP, r.MinBytesMult), r.MaxBytesMult)
	switch {
	case d.Mystery:
		mult *= r.MysteryMult
		if e.s.has(DevRandom) && !e.s.perks() { // the roster's pays a pick (rosterClear)
			mult *= r.DevRandomMult
		}
	case d.Blind:
		mult *= r.BlindMult
	}
	if d.Cursed {
		mult *= r.CursedMult
	}
	switch {
	case d.Twist == "race-condition" && r.Twists.RacePay > 0:
		mult *= r.Twists.RacePay // the race pays for its clock
	case d.Boss != "":
		mult *= r.BossMult
	}
	if d.Regression && e.s.has(Cron) && !e.s.perks() { // the roster's frees cycles
		mult *= r.CronMult
	}
	if slices.Contains(e.s.Daemons, Chrt) {
		mult *= r.ChrtBytesMult
	}
	if slices.Contains(e.s.Daemons, Nice) {
		mult *= 1 - r.NiceBytesFrac
	}
	return max(1, int64(math.Round(float64(r.BaseBytes)*mult)))
}

// routeOffers picks k topics for a biome: unlocked, and with at least
// Rooms·RouteMinPool eligible problems within the widest rating window.
// Topics not yet offered this boot go first. Mixed is the fallback, so a
// route is never empty.
func (e *engine) routeOffers(i, k int, used map[string]bool) []string {
	rng := e.s.RNG
	r := &e.s.Rules
	rb := r.Biomes[i]
	widest := float64(slices.Max(r.RatingWindows))
	ex := e.excluded()
	targets := map[string]float64{}
	count := map[string]int{}
	for j := range e.pool {
		p := &e.pool[j]
		t := topicOf(p)
		if !p.Judgeable || ex[p.ID] || t == Mixed {
			continue
		}
		tg, ok := targets[t]
		if !ok {
			tg = e.target(t, i)
			targets[t] = tg
		}
		if math.Abs(difficulty(r, p)-tg) <= widest {
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
	nf := len(offers)
	offers = append(offers, sample(e.rng, reused, k-nf)...)
	// The log: each fresh topic is as likely as the next to be among the
	// nf drawn, each reused one among the rest.
	props, set := make([]float64, len(offers)), 1.0
	for j := range offers {
		if j < nf {
			props[j] = float64(nf) / float64(len(fresh))
		} else {
			props[j] = float64(len(offers)-nf) / float64(len(reused))
		}
	}
	if nf > 0 {
		set /= binomial(len(fresh), nf)
	}
	if n := len(offers) - nf; n > 0 {
		set /= binomial(len(reused), n)
	}
	if len(offers) == 0 {
		offers, props = []string{Mixed}, []float64{1}
	}
	chosen := -1
	if rb.Kind != rules.Choice {
		chosen = 0
	}
	e.logRoute(i, offers, props, set, rng, chosen)
	return offers
}

// layout builds the boot's biomes from the rules: topics for fixed biomes,
// route offers for choice biomes, and boss keys (distinct per boot until
// they run out; The Halting Problem ends a boot at the top of the rules'
// ladder, rules.Rules.TopPrestige).
func (e *engine) layout() {
	s := e.s
	r := &s.Rules
	// The snapshot shares its slices with the caller's rules until here,
	// and a map writes its tiers' nodes into it.
	r.Biomes = slices.Clone(r.Biomes)
	defer e.firstClock()
	if s.mapped() {
		e.layoutMap()
		return
	}
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
			if s.Prestige >= r.TopPrestige(MaxPrestige) && i == len(r.Biomes)-1 {
				b.Boss = FinalBoss
			}
		case rules.Choice:
			b.Offers = e.routeOffers(i, rb.Choices+r.Grants(s.Packages).RouteOffers, used)
			for _, t := range b.Offers {
				used[t] = true
			}
		default:
			b.Topic = Mixed
			if i > 0 {
				b.Topic = e.routeOffers(i, 1, used)[0]
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
		{d.Mastered, "mastered"},
	} {
		if f.on {
			m = append(m, f.name)
		}
	}
	return strings.Join(m, " ")
}

// firstClock sets the first biome's default timer to the prestige's
// (rules.Ascension.FirstTimer: Userland 9:00 at P3, 8:00 at P6).
func (e *engine) firstClock() {
	if ft := e.s.Rules.Ascend(e.s.Prestige).FirstTimer.Duration; ft > 0 {
		e.s.Rules.Biomes[0].Timer = rules.Duration{Duration: ft}
	}
}
