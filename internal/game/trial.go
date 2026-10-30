package game

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
)

// NewTrial starts a learn-mode trial (DESIGN.md "Learn mode") as boot
// meta.Boots+1, so the store saves and resumes it like any boot; State.Trial
// marks it. r is normally rules.Trial(): two fixed biomes, the unit then the
// integration, whose Rooms are the minimum unit size and the number of
// integration doors. Every room has one door.
//
// The unit plays t.Unit in order, with tags shown, skipping problems the
// pool cannot judge. A unit shorter than the minimum is topped up from the
// pool (9 of the guide's 105 trials list fewer than 2 playable problems);
// top-ups are merged in by rating so the unit still ramps, and the module's
// own order is never changed. The integration draws blind doors the same
// way (see trialDraw).
//
// Clearing grants t.Capability and records t.Module in Meta.TrialsPassed;
// failing ends the trial like a crash, without a core dump or a regression
// (see crash). Doors cannot be swapped, merge windows offer no daemons, and
// trial doors are not aimed, so they teach no calibration.
func NewTrial(r rules.Rules, meta *Meta, pool []catalog.Problem, t TrialSpec, seed uint64, now time.Time) (*State, []Effect, error) {
	if meta == nil {
		return nil, nil, errors.New("game: nil meta")
	}
	r = r.WithDefaults()
	if err := r.Validate(); err != nil {
		return nil, nil, err
	}
	if len(r.Biomes) != 2 || r.Biomes[0].Kind != rules.Fixed || r.Biomes[1].Kind != rules.Fixed {
		return nil, nil, fmt.Errorf("game: trial rules %q need two fixed biomes, unit and integration (see rules.Trial)", r.Name)
	}
	if b := t.IntegBand; b[0] > b[1] {
		return nil, nil, fmt.Errorf("game: trial %q band %v is empty", t.Module, b)
	}
	// The snapshot records the trial as played: its rooms come from the
	// plan, and Biomes must not alias the caller's slice.
	r.Biomes = slices.Clone(r.Biomes)
	t.Unit = slices.Clone(t.Unit)
	s := &State{Seed: seed, RNG: seed, Rules: r, Started: now, Trial: &TrialState{TrialSpec: t}}
	e := newEngine(s, meta, pool, now)
	if err := e.planTrial(); err != nil {
		return nil, nil, err
	}

	e.begin()
	topic := t.IntegTopic
	if topic == "" {
		topic = Mixed
	}
	tr := s.Trial
	for i, ids := range tr.Plan {
		s.Rules.Biomes[i].Rooms = len(ids)
		s.Biomes = append(s.Biomes, BiomeState{Name: r.Biomes[i].Name, Topic: topic, Kind: rules.Fixed, Rooms: len(ids)})
	}
	e.logf("trial #%d: %s (%s), unit %d (%d topped up), integration %d, rules %s, seed %#x, rating %.0f",
		s.Boot, t.Module, t.Title, len(tr.Plan[0]), len(tr.TopUp), len(tr.Plan[1]), r.Name, seed, meta.Rating)
	e.emit(Prefetch{Problems: slices.Concat(tr.Plan...)})
	e.enterBiome(0)
	return s, e.fx, nil
}

// planTrial fills s.Trial's Plan and TopUp, or fails when the pool cannot
// give both biomes a door.
func (e *engine) planTrial() error {
	tr := e.s.Trial
	used := map[catalog.ID]bool{}
	var unit []*catalog.Problem
	for _, id := range tr.Unit {
		i := slices.IndexFunc(e.pool, func(p catalog.Problem) bool { return p.ID == id && p.Judgeable })
		if i < 0 || used[id] {
			continue
		}
		used[id] = true
		unit = append(unit, &e.pool[i])
	}
	var top []*catalog.Problem
	if n := e.s.Rules.Biomes[0].Rooms - len(unit); n > 0 {
		top = e.trialDraw(n, used)
		slices.SortStableFunc(top, func(a, b *catalog.Problem) int { return cmp.Compare(a.Rating, b.Rating) })
	}
	integ := e.trialDraw(e.s.Rules.Biomes[1].Rooms, used)
	if len(unit)+len(top) == 0 || len(integ) == 0 {
		return fmt.Errorf("game: the pool has too few judgeable problems for trial %q", tr.Module)
	}

	// Merge the top-ups (sorted) into the unit (module order) by rating.
	var plan []catalog.ID
	for len(unit) > 0 || len(top) > 0 {
		if len(top) > 0 && (len(unit) == 0 || top[0].Rating < unit[0].Rating) {
			tr.TopUp = append(tr.TopUp, top[0].ID)
			plan, top = append(plan, top[0].ID), top[1:]
		} else {
			plan, unit = append(plan, unit[0].ID), unit[1:]
		}
	}
	tr.Plan = [][]catalog.ID{plan, problemIDs(integ)}
	return nil
}

func problemIDs(ps []*catalog.Problem) []catalog.ID {
	out := make([]catalog.ID, len(ps))
	for i, p := range ps {
		out[i] = p.ID
	}
	return out
}

// trialDraw picks up to n judgeable problems outside used for the trial and
// adds them to used. Preference, most important first: on the integration
// topic; rating inside the band, else within the narrowest of the rules'
// rating windows around it; not solved independently. Equals are drawn
// uniformly at random.
//
// Topic comes before rating, the reverse of boot doors (see pick): a trial
// tests a topic, and a door on another topic tests nothing about it, while
// a door 100 points outside the band still does.
func (e *engine) trialDraw(n int, used map[catalog.ID]bool) []*catalog.Problem {
	t := e.s.Trial
	windows := e.s.Rules.RatingWindows
	band := t.IntegBand
	solved := map[catalog.ID]bool{}
	for _, s := range e.meta.Solves {
		if s.Kind == Independent {
			solved[s.Problem] = true
		}
	}
	type cand struct {
		p      *catalog.Problem
		off    bool // off the integration topic
		tier   int  // 0 inside the band, i+1 within windows[i] of it
		solved bool
	}
	var cs []cand
	seen := map[catalog.ID]bool{}
	for i := range e.pool {
		p := &e.pool[i]
		if !p.Judgeable || used[p.ID] || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		dist := 0
		if band != [2]int{} {
			dist = max(band[0]-p.Rating, p.Rating-band[1], 0)
		}
		tier := 0
		if dist > 0 {
			tier = len(windows) + 1
			for j, w := range windows {
				if dist <= w {
					tier = j + 1
					break
				}
			}
		}
		off := t.IntegTopic != "" && t.IntegTopic != Mixed && topicOf(p) != t.IntegTopic
		cs = append(cs, cand{p, off, tier, solved[p.ID]})
	}
	// Sorted by ID, then shuffled from the seed, then ordered by
	// preference: the draw depends on the seed, not on the catalog's order.
	slices.SortFunc(cs, func(a, b cand) int { return cmp.Compare(a.p.ID, b.p.ID) })
	e.rng.Shuffle(len(cs), func(i, j int) { cs[i], cs[j] = cs[j], cs[i] })
	bit := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(cs, func(a, b cand) int {
		return cmp.Or(cmp.Compare(bit(a.off), bit(b.off)), cmp.Compare(a.tier, b.tier), cmp.Compare(bit(a.solved), bit(b.solved)))
	})
	var out []*catalog.Problem
	for _, c := range cs[:min(n, len(cs))] {
		used[c.p.ID] = true
		out = append(out, c.p)
	}
	return out
}

// trialProblem is the current room's planned problem. If the pool lost it
// since the trial was planned (a catalog update between sessions), a
// replacement is drawn and written into the plan; with nothing left to
// draw, a stub keeps the room playable rather than empty.
func (e *engine) trialProblem() *catalog.Problem {
	s := e.s
	tr := s.Trial
	id := tr.Plan[s.Biome][s.Room]
	if i := slices.IndexFunc(e.pool, func(p catalog.Problem) bool { return p.ID == id && p.Judgeable }); i >= 0 {
		return &e.pool[i]
	}
	used := e.attempted()
	for _, ids := range tr.Plan {
		for _, id := range ids {
			used[id] = true
		}
	}
	if ps := e.trialDraw(1, used); len(ps) > 0 {
		e.logf("trial: %s left the pool, %s instead", id, ps[0].ID)
		tr.Plan[s.Biome][s.Room] = ps[0].ID
		return ps[0]
	}
	return &catalog.Problem{ID: id, Topic: s.Biomes[s.Biome].Topic, Rating: (tr.IntegBand[0] + tr.IntegBand[1]) / 2, Judgeable: true}
}

// LostRoomsLeft is how many more rooms the trial may lose to :q! and still
// pass (rules.TrialLostRooms). Only unit rooms can go, so it is 0 on a
// blind door, as outside a trial.
func (s *State) LostRoomsLeft() int {
	if s.Trial == nil || s.Biome != 0 {
		return 0
	}
	return max(s.Rules.TrialLostRooms-s.Trial.Lost, 0)
}

// loseTrialRoom is a trial's :q! while it has a unit room to lose: the
// door closes, rated as failed, and the trial goes on as after a clear,
// with no pay and no upgrade pick, to the blind doors once the unit is
// done. Like a failed trial, it schedules no regression (see crash). It
// reports false where the :q! ends the trial.
func (e *engine) loseTrialRoom(o Outcome) bool {
	s := e.s
	if o != GaveUp || s.LostRoomsLeft() == 0 {
		return false
	}
	name := e.door()
	a := e.close(o)
	s.Trial.Lost++
	e.logf("%s: :q!, room lost, %d more may go", name, s.LostRoomsLeft())
	e.emit(CloseProblem{Door: a.Door}, Sound{Name: "wa"})
	if s.Room++; s.Room < s.Biomes[s.Biome].Rooms {
		s.Phase = PhaseDoors
		e.rollRoom(true, "fresh")
	} else {
		e.enterMerge()
	}
	return true
}

// passTrial grants the trial's capability and records the module.
func (e *engine) passTrial() {
	m, tr := e.meta, e.s.Trial
	tr.Passed = true
	if c := tr.Capability; c != "" && !slices.Contains(m.Capabilities, c) {
		GrantCapability(m, c)
		tr.Granted = c
		e.logf("capability: %s granted", c)
	}
	if tr.Module != "" && !slices.Contains(m.TrialsPassed, tr.Module) {
		m.TrialsPassed = append(m.TrialsPassed, tr.Module)
	}
	e.logf("trial %s: passed", tr.Module)
}
