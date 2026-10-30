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
// marks it. r is normally rules.Trial(): fixed biomes whose rooms hold one
// door each, every door blind.
//
// pool is the trials' own (curriculum.Split): problems no module map shows
// and no boot draws, so every door is one the player has not met in RTFM.
// Room k's door aims at t.Target(k) and is drawn by trialDraw, which also
// passes over what the player has solved. The whole plan is drawn up
// front, so the map can show it, and anew on every try.
//
// Clearing grants t.Capability and records t.Module in Meta.TrialsPassed;
// failing ends the trial like a crash, without a core dump or a regression
// (see crash). Doors cannot be swapped, merge windows offer no daemons, and
// trial doors are not aimed at the player, so they teach no calibration.
func NewTrial(r rules.Rules, meta *Meta, pool []catalog.Problem, t TrialSpec, seed uint64, now time.Time) (*State, []Effect, error) {
	if meta == nil {
		return nil, nil, errors.New("game: nil meta")
	}
	r = r.WithDefaults()
	if err := r.Validate(); err != nil {
		return nil, nil, err
	}
	if len(r.Biomes) == 0 || slices.ContainsFunc(r.Biomes, func(b rules.Biome) bool { return b.Kind != rules.Fixed }) {
		return nil, nil, fmt.Errorf("game: trial rules %q need fixed biomes only (see rules.Trial)", r.Name)
	}
	if b := t.Band; b[0] > b[1] {
		return nil, nil, fmt.Errorf("game: trial %q band %v is empty", t.Module, b)
	}
	// The snapshot records the trial as played: Biomes must not alias the
	// caller's slice.
	r.Biomes = slices.Clone(r.Biomes)
	t.Targets = slices.Clone(t.Targets)
	s := &State{Seed: seed, RNG: seed, Rules: r, Started: now, Trial: &TrialState{TrialSpec: t}}
	e := newEngine(s, meta, pool, now)
	if err := e.planTrial(); err != nil {
		return nil, nil, err
	}

	e.begin()
	topic := cmp.Or(t.Topic, Mixed)
	for _, b := range r.Biomes {
		s.Biomes = append(s.Biomes, BiomeState{Name: b.Name, Topic: topic, Kind: rules.Fixed, Rooms: b.Rooms})
	}
	n := s.trialRooms()
	aims := make([]int, n)
	for k := range aims {
		aims[k] = t.Target(k, n)
	}
	e.logf("trial #%d: %s (%s), doors %v aimed at %v on %s, rules %s, seed %#x, rating %.0f",
		s.Boot, t.Module, t.Title, slices.Concat(s.Trial.Plan...), aims, topic, r.Name, seed, meta.Rating)
	e.emit(Prefetch{Problems: slices.Concat(s.Trial.Plan...)})
	e.enterBiome(0)
	return s, e.fx, nil
}

// planTrial draws every room's door into s.Trial.Plan, or fails when the
// pool runs out first.
func (e *engine) planTrial() error {
	tr := e.s.Trial
	n := e.s.trialRooms()
	used := map[catalog.ID]bool{}
	k := 0
	for _, b := range e.s.Rules.Biomes {
		var ids []catalog.ID
		for range b.Rooms {
			p := e.trialDraw(used, tr.Target(k, n))
			if p == nil {
				return fmt.Errorf("game: the pool has too few judgeable problems for trial %q", tr.Module)
			}
			ids = append(ids, p.ID)
			k++
		}
		tr.Plan = append(tr.Plan, ids)
	}
	return nil
}

// trialRooms is how many rooms the trial's biomes hold; trialRoom is the
// current room's index among them.
func (s *State) trialRooms() int {
	n := 0
	for _, b := range s.Rules.Biomes {
		n += b.Rooms
	}
	return n
}

func (s *State) trialRoom() int {
	k := s.Room
	for _, b := range s.Rules.Biomes[:s.Biome] {
		k += b.Rooms
	}
	return k
}

// Solved is every problem m has solved: in a boot, an upsolve or a trial,
// with help or without. A trial's doors pass over them (see trialDraw); a
// boot's let an assisted solve come back (see excluded).
func Solved(m *Meta) map[catalog.ID]bool {
	s := make(map[catalog.ID]bool, len(m.Solves))
	for _, sv := range m.Solves {
		s[sv.Problem] = true
	}
	return s
}

// trialDraw picks a judgeable problem outside used for a trial door aimed
// at target (0: any rating) and adds it to used; nil when none is left.
// Preference, most important first: within the rules' widest rating
// window of target; on the trial's topic; not solved (Solved); in the
// narrowest window. Equals are drawn uniformly at random.
//
// Topic comes before closeness, the reverse of boot doors (see pick): a
// trial tests a topic, and a door on another topic tests nothing about it
// while one 250 off the aim still does. Past every window the topic gives
// way: a prefix-sums problem 900 over a Silver trial tests the player's
// ceiling, not the module. A solved door tests nothing either, so it waits
// behind every unsolved one in the windows.
func (e *engine) trialDraw(used map[catalog.ID]bool, target int) *catalog.Problem {
	t := e.s.Trial
	windows := e.s.Rules.RatingWindows
	solved := Solved(e.meta)
	type cand struct {
		p                *catalog.Problem
		far, off, solved bool
		tier             int // the narrowest window holding it
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
		if target > 0 {
			dist = max(p.Rating-target, target-p.Rating)
		}
		tier := len(windows)
		for j, w := range windows {
			if dist <= w {
				tier = j
				break
			}
		}
		off := t.Topic != "" && t.Topic != Mixed && topicOf(p) != t.Topic
		cs = append(cs, cand{p, tier == len(windows), off, solved[p.ID], tier})
	}
	if len(cs) == 0 {
		return nil
	}
	// Sorted by ID, then shuffled from the seed, then ordered by
	// preference: the draw depends on the seed, not on the pool's order.
	slices.SortFunc(cs, func(a, b cand) int { return cmp.Compare(a.p.ID, b.p.ID) })
	e.rng.Shuffle(len(cs), func(i, j int) { cs[i], cs[j] = cs[j], cs[i] })
	bit := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	best := slices.MinFunc(cs, func(a, b cand) int {
		return cmp.Or(cmp.Compare(bit(a.far), bit(b.far)), cmp.Compare(bit(a.off), bit(b.off)),
			cmp.Compare(bit(a.solved), bit(b.solved)), cmp.Compare(a.tier, b.tier))
	})
	used[best.p.ID] = true
	return best.p
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
	target := tr.Target(s.trialRoom(), s.trialRooms())
	if p := e.trialDraw(used, target); p != nil {
		e.logf("trial: %s left the pool, %s instead", id, p.ID)
		tr.Plan[s.Biome][s.Room] = p.ID
		return p
	}
	return &catalog.Problem{ID: id, Topic: s.Biomes[s.Biome].Topic, Rating: target, Judgeable: true}
}

// LostRoomsLeft is how many more rooms the trial may lose to :q! and still
// pass (rules.TrialLostRooms); 0 outside a trial.
func (s *State) LostRoomsLeft() int {
	if s.Trial == nil {
		return 0
	}
	return max(s.Rules.TrialLostRooms-s.Trial.Lost, 0)
}

// loseTrialRoom is a trial's :q! while it has a room to lose: the door
// closes, rated as failed, and the trial goes on as after a clear, with no
// pay and no upgrade pick: to the next room, the merge window after a
// biome's last, or the pass after the trial's last. Like a failed trial,
// it schedules no regression (see crash). It reports false where the :q!
// ends the trial.
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
	s.Room++
	switch {
	case s.Room < s.Biomes[s.Biome].Rooms:
		s.Phase = PhaseDoors
		e.rollRoom(true, "fresh")
	case s.Biome == len(s.Biomes)-1:
		e.finish()
	default:
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
