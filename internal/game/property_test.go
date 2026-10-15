package game_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// next lists the phases each phase may move to in one Step.
var next = map[game.Phase][]game.Phase{
	game.PhaseRoute:   {game.PhaseRoute, game.PhaseDoors},
	game.PhaseDoors:   {game.PhaseDoors, game.PhaseProblem},
	game.PhaseProblem: {game.PhaseProblem, game.PhaseDoors, game.PhaseMerge, game.PhaseCleared, game.PhaseCrashed},
	game.PhaseMerge:   {game.PhaseMerge, game.PhaseRoute, game.PhaseDoors},
	game.PhaseCrashed: {game.PhaseCrashed},
	game.PhaseCleared: {game.PhaseCleared},
}

// checker holds what invariants compare against across one boot.
type checker struct {
	t     testing.TB
	bank0 int64 // meta.Bank when the boot started
	phase game.Phase
	dump0 string // a trial's meta.CoreDump (JSON) when it started
}

// check asserts the invariants that hold after every Step.
func (c *checker) check(s *game.State, m *game.Meta, a game.Action, err error) {
	t := c.t
	t.Helper()
	fail := func(format string, args ...any) {
		t.Helper()
		t.Fatalf("boot %d after %T%+v (phase %s): %s", s.Boot, a, a, s.Phase, fmt.Sprintf(format, args...))
	}
	if err != nil && !errors.Is(err, game.ErrInvalid) {
		fail("error %v is not ErrInvalid", err)
	}
	if !slices.Contains(next[c.phase], s.Phase) {
		fail("%s -> %s is not a transition", c.phase, s.Phase)
	}
	c.phase = s.Phase
	if s.Lives < 0 || s.Lives > s.MaxLives || s.Ducks < 0 {
		fail("lives %d/%d ducks %d", s.Lives, s.MaxLives, s.Ducks)
	}
	if s.Lives == 0 && s.Phase != game.PhaseCrashed {
		fail("no lives but not crashed")
	}
	if s.UpgradePicks < 0 || s.PendingUpgrade != (s.UpgradePicks > 0) || s.PendingUpgrade != (len(s.UpgradeOffers) > 0) {
		fail("picks %d pending %v offers %v", s.UpgradePicks, s.PendingUpgrade, s.UpgradeOffers)
	}
	if (s.Current != nil) != (s.Phase == game.PhaseProblem) {
		fail("current %v in phase %s", s.Current != nil, s.Phase)
	}
	if (s.Crash != nil) != (s.Phase == game.PhaseCrashed) {
		fail("crash %v in phase %s", s.Crash != nil, s.Phase)
	}
	if s.Phase == game.PhaseDoors {
		if len(s.Doors) == 0 {
			fail("no doors")
		}
		seen := map[catalog.ID]bool{}
		for _, d := range s.Doors {
			if seen[d.Problem] {
				fail("door %s twice in a room", d.Problem)
			}
			seen[d.Problem] = true
			if d.Bytes < 1 || d.Timer < time.Minute || d.SolveP <= 0 || d.SolveP >= 1 {
				fail("door %+v", d)
			}
			if d.Boss != "" && d.Cursed || d.Mystery && !d.Blind {
				fail("door modifiers %+v", d)
			}
		}
	} else if len(s.Doors) != 0 {
		fail("%d doors outside the doors phase", len(s.Doors))
	}
	// A merge window or a clear sits past the last room of the biome it ends.
	done := 0
	if s.Phase == game.PhaseMerge || s.Phase == game.PhaseCleared {
		done = 1
	}
	if s.Biome >= len(s.Biomes) || s.Room >= s.Biomes[s.Biome].Rooms+done || done == 1 && s.Room != s.Biomes[s.Biome].Rooms {
		fail("biome %d room %d out of the layout", s.Biome, s.Room)
	}
	if len(s.Daemons) > s.Rules.MaxDaemons || len(slices.Compact(slices.Sorted(slices.Values(s.Daemons)))) != len(s.Daemons) {
		fail("daemons %v", s.Daemons)
	}

	// Bytes are conserved: every byte a clear paid is committed, still
	// uncommitted, or lost to the crash, and only commits reach the bank.
	lost := int64(0)
	if s.Crash != nil {
		lost = s.Crash.Lost
		if s.Uncommitted != 0 {
			fail("uncommitted %d after a crash", s.Uncommitted)
		}
		d := m.CoreDump
		switch {
		case s.Trial != nil:
			// A failed trial leaves the dump a boot made, and no other.
			if got := mustJSON(t, d); got != c.dump0 {
				fail("trial crash changed the core dump %s to %s", c.dump0, got)
			}
		case (lost > 0) != (d != nil) || d != nil && (d.Problem != s.Crash.Door.Problem || d.Bytes != lost):
			fail("crash lost %d, core dump %+v", lost, d)
		}
	}
	if s.Trial != nil {
		if len(s.DaemonOffers) != 0 || len(s.Daemons) != 0 || len(s.Doors) > 1 {
			fail("trial with daemons %v offers %v doors %d", s.Daemons, s.DaemonOffers, len(s.Doors))
		}
		for _, d := range s.Doors {
			if d.Problem != s.Trial.Plan[s.Biome][s.Room] || d.Blind != (s.Biome > 0) || d.Cursed || d.Mystery || d.Tier != "" {
				fail("trial door %+v in biome %d room %d of plan %v", d, s.Biome, s.Room, s.Trial.Plan)
			}
		}
		if s.Phase == game.PhaseCleared && !s.Trial.Passed {
			fail("trial cleared but not passed")
		}
	}
	if s.Earned() != s.Committed+s.Uncommitted+lost {
		fail("earned %d != committed %d + uncommitted %d + lost %d", s.Earned(), s.Committed, s.Uncommitted, lost)
	}
	if m.Bank != c.bank0+s.Committed {
		fail("bank %d != %d + committed %d", m.Bank, c.bank0, s.Committed)
	}
	if s.Phase == game.PhaseCleared && s.Uncommitted != 0 {
		fail("cleared with %d uncommitted", s.Uncommitted)
	}
}

// fuzzer plays a boot with mostly sensible and some nonsense actions, on a
// clock that jumps around (sometimes past the deadline).
type fuzzer struct {
	rng *rand.Rand
	now time.Time
}

func (f *fuzzer) action(s *game.State) game.Action {
	r := f.rng
	if r.IntN(4) == 0 { // nonsense: any action, any index
		switch r.IntN(12) {
		case 0:
			return game.ChooseRoute{Index: r.IntN(5) - 1}
		case 1:
			return game.ChooseDoor{Index: r.IntN(6) - 1}
		case 2:
			return game.Retreat{}
		case 3:
			return game.Quack{}
		case 4:
			return game.Commit{}
		case 5:
			return game.PickUpgrade{Upgrade: game.AllUpgrades[r.IntN(3)]}
		case 6:
			return game.PickDaemon{Daemon: game.AllDaemons[r.IntN(5)]}
		case 7:
			return game.LeaveMerge{}
		case 8:
			return game.Tick{}
		case 9:
			return f.submit()
		case 10:
			if r.IntN(10) == 0 {
				return game.GiveUp{}
			}
			return game.Tick{}
		}
		return nil
	}
	switch {
	case s.PendingUpgrade:
		return game.PickUpgrade{Upgrade: s.UpgradeOffers[r.IntN(len(s.UpgradeOffers))]}
	case s.Phase == game.PhaseRoute:
		return game.ChooseRoute{Index: r.IntN(len(s.Biomes[s.Biome].Offers))}
	case s.Phase == game.PhaseDoors:
		return game.ChooseDoor{Index: r.IntN(len(s.Doors))}
	case s.Phase == game.PhaseMerge:
		switch {
		case r.IntN(2) == 0 && s.Uncommitted > 0:
			return game.Commit{}
		case len(s.DaemonOffers) > 0 && r.IntN(2) == 0:
			return game.PickDaemon{Daemon: s.DaemonOffers[r.IntN(len(s.DaemonOffers))]}
		}
		return game.LeaveMerge{}
	case s.Phase == game.PhaseProblem:
		switch r.IntN(10) {
		case 0:
			return game.Quack{}
		case 1:
			return game.Retreat{}
		case 2, 3:
			return game.Tick{}
		}
		return f.submit()
	}
	return game.Tick{}
}

func (f *fuzzer) submit() game.SubmitResult {
	v := []string{"AC", "AC", "AC", "WA", "TLE", "RE", "MLE", "CE"}[f.rng.IntN(8)]
	return game.SubmitResult{Verdict: v, Passed: f.rng.IntN(11), Total: 10}
}

func (f *fuzzer) tick() {
	f.now = f.now.Add(time.Duration(f.rng.IntN(int(12 * time.Minute))))
}

// fuzzBoot plays one boot to its end (or a step cap) and returns the
// actions with their times, for replay.
type move struct {
	A   game.Action
	At  time.Time
	Err bool
}

func fuzzBoot(t testing.TB, r rules.Rules, m *game.Meta, ps []catalog.Problem, f *fuzzer, verify bool) (*game.State, []move) {
	t.Helper()
	prestige := f.rng.IntN(m.Prestige + 1)
	s, _, err := game.NewBoot(r, m, ps, prestige, f.rng.Uint64(), f.now)
	if err != nil {
		t.Fatalf("NewBoot: %v", err)
	}
	c := &checker{t: t, bank0: m.Bank, phase: s.Phase}
	return s, c.play(s, m, ps, f, verify)
}

// fuzzTrial is fuzzBoot for a learn-mode trial on a spec drawn from ps.
func fuzzTrial(t testing.TB, m *game.Meta, ps []catalog.Problem, f *fuzzer, verify bool) (*game.State, []move) {
	t.Helper()
	spec := fuzzSpec(f.rng, ps)
	dump0 := mustJSON(t, m.CoreDump)
	s, _, err := game.NewTrial(rules.Trial(), m, ps, spec, f.rng.Uint64(), f.now)
	if err != nil {
		t.Fatalf("NewTrial(%+v): %v", spec, err)
	}
	c := &checker{t: t, bank0: m.Bank, phase: s.Phase, dump0: dump0}
	return s, c.play(s, m, ps, f, verify)
}

// fuzzSpec is a trial on a random topic and band, with a unit of 0 to 5 of
// that topic's problems and sometimes an id the pool lacks.
func fuzzSpec(rng *rand.Rand, ps []catalog.Problem) game.TrialSpec {
	topic := catalog.Topics[rng.IntN(len(catalog.Topics))]
	lo := 800 + 100*rng.IntN(16)
	spec := game.TrialSpec{Module: "m-" + topic, IntegTopic: topic, IntegBand: [2]int{lo, lo + 300}, Capability: game.CapabilityFor(topic)}
	for range rng.IntN(6) {
		p := ps[rng.IntN(len(ps))]
		spec.Unit = append(spec.Unit, p.ID)
	}
	if rng.IntN(4) == 0 {
		spec.Unit = append(spec.Unit, "gone:1")
	}
	return spec
}

// play fuzzes s to its end (or a step cap), checking c after every step,
// and returns the actions with their times, for replay.
func (c *checker) play(s *game.State, m *game.Meta, ps []catalog.Problem, f *fuzzer, verify bool) []move {
	t := c.t
	t.Helper()
	c.check(s, m, nil, nil)
	var moves []move
	for n := 0; n < 400 && s.Phase != game.PhaseCrashed && s.Phase != game.PhaseCleared; n++ {
		f.tick()
		a := f.action(s)
		var before, meta string
		if verify {
			before, meta = mustJSON(t, s), mustJSON(t, m)
		}
		_, err := game.Step(s, m, ps, a, f.now)
		if err != nil && verify && (mustJSON(t, s) != before || mustJSON(t, m) != meta) {
			t.Fatalf("%T%+v failed (%v) but changed the state", a, a, err)
		}
		c.check(s, m, a, err)
		moves = append(moves, move{a, f.now, err != nil})
	}
	return moves
}

// TestFuzzTrials plays fuzzed trials between fuzzed boots, so trials meet
// core dumps and regressions a boot left, under every invariant plus the
// trial's own (one planned door per room, no daemons, the dump untouched).
func TestFuzzTrials(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 3)
	m := &game.Meta{Prestige: game.MaxPrestige}
	f := &fuzzer{rng: rand.New(rand.NewPCG(21, 22)), now: t0}
	ends := map[game.Phase]int{}
	runs := 2000
	if testing.Short() {
		runs = 200
	}
	for i := range runs {
		if i%3 == 0 {
			fuzzBoot(t, rules.Standard(), m, ps, f, false)
		} else {
			s, _ := fuzzTrial(t, m, ps, f, i%10 == 1)
			ends[s.Phase]++
		}
		f.now = f.now.Add(time.Duration(f.rng.IntN(int(72 * time.Hour))))
	}
	t.Logf("trial ends %v, %d modules passed, capabilities %v", ends, len(m.TrialsPassed), m.Capabilities)
	if ends[game.PhaseCleared] == 0 || ends[game.PhaseCrashed] == 0 {
		t.Errorf("ends %v: the fuzzer should reach both", ends)
	}
}

// TestTrialJSONRoundTrip is TestJSONRoundTrip for trials.
func TestTrialJSONRoundTrip(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 6)
	resumed := 0
	for seed := range uint64(150) {
		f := &fuzzer{rng: rand.New(rand.NewPCG(seed, 7)), now: t0}
		_, moves := fuzzTrial(t, &game.Meta{}, ps, f, false)
		if len(moves) < 4 {
			continue
		}
		cut := len(moves) / 2
		f2 := &fuzzer{rng: rand.New(rand.NewPCG(seed, 7)), now: t0}
		m2 := &game.Meta{}
		s2, _, err := game.NewTrial(rules.Trial(), m2, ps, fuzzSpec(f2.rng, ps), f2.rng.Uint64(), t0)
		if err != nil {
			t.Fatal(err)
		}
		for _, mv := range moves[:cut] {
			game.Step(s2, m2, ps, mv.A, mv.At)
		}
		var s3 game.State
		var m3 game.Meta
		if err := json.Unmarshal([]byte(mustJSON(t, s2)), &s3); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(mustJSON(t, m2)), &m3); err != nil {
			t.Fatal(err)
		}
		for _, mv := range moves[cut:] {
			fx2, err2 := game.Step(s2, m2, ps, mv.A, mv.At)
			fx3, err3 := game.Step(&s3, &m3, ps, mv.A, mv.At)
			if (err2 == nil) != (err3 == nil) || (err2 != nil) != mv.Err || fmt.Sprint(fx2) != fmt.Sprint(fx3) {
				t.Fatalf("seed %d: %T%+v diverged after the reload: %v / %v", seed, mv.A, mv.A, err2, err3)
			}
		}
		if mustJSON(t, s2) != mustJSON(t, &s3) || mustJSON(t, m2) != mustJSON(t, &m3) {
			t.Fatalf("seed %d: reloaded trial ended differently", seed)
		}
		resumed += len(moves) - cut
	}
	if resumed < 300 {
		t.Errorf("only %d moves played after a reload", resumed)
	}
}

// TestFuzzBoots plays thousands of fuzzed boots with meta carried across,
// checking every invariant after every step.
func TestFuzzBoots(t *testing.T) {
	t.Parallel()
	pools := map[string][]catalog.Problem{
		"synthetic": sim.SyntheticPool(600, 3),
		"tiny":      dense([]string{"dp"}, 1000, 1100, 50), // exhausted at once
	}
	for name, ps := range pools {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := rules.Standard()
			m := &game.Meta{Prestige: game.MaxPrestige}
			f := &fuzzer{rng: rand.New(rand.NewPCG(11, 12)), now: t0}
			ends := map[game.Phase]int{}
			boots := 3000
			if testing.Short() {
				boots = 300
			}
			for i := range boots {
				s, _ := fuzzBoot(t, r, m, ps, f, i%10 == 0)
				ends[s.Phase]++
				if f.rng.IntN(3) == 0 && m.CoreDump != nil {
					game.Upsolve(m, m.CoreDump.Problem, game.Editorial, time.Hour, f.now)
				}
				f.now = f.now.Add(time.Duration(f.rng.IntN(int(72 * time.Hour))))
			}
			t.Logf("%s: boot ends %v, meta rating %.0f, %d solves, %d regressions",
				name, ends, m.Rating, len(m.Solves), len(m.Regressions))
			if ends[game.PhaseCleared] == 0 || ends[game.PhaseCrashed] == 0 {
				t.Errorf("ends %v: the fuzzer should reach both", ends)
			}
		})
	}
}

// TestModelInvariants runs the sim's model player through thousands of
// boots under the same invariants.
func TestModelInvariants(t *testing.T) {
	t.Parallel()
	for _, rating := range []float64{900, 1500, 2300} {
		t.Run(fmt.Sprint(rating), func(t *testing.T) {
			t.Parallel()
			var c *checker
			rep := sim.Run(sim.Config{
				Rules:    rules.Standard(),
				Pool:     sim.SyntheticPool(600, 4),
				Player:   sim.DefaultPlayer(rating),
				Boots:    1500,
				Seed:     uint64(rating),
				Prestige: -1,
				Check: func(s *game.State, m *game.Meta, a game.Action, _ []game.Effect, err error) {
					if a == nil { // NewBoot
						if err != nil {
							t.Fatalf("NewBoot: %v", err)
						}
						c = &checker{t: t, bank0: m.Bank - s.Committed, phase: s.Phase}
					}
					c.check(s, m, a, err)
				},
			})
			if rep.Errors != 0 {
				t.Errorf("%d model actions failed", rep.Errors)
			}
		})
	}
}

// TestDeterminism: the same seed and the same actions at the same times give
// byte-identical state, meta and effects.
func TestDeterminism(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 5)
	run := func() string {
		m := &game.Meta{Prestige: 2}
		f := &fuzzer{rng: rand.New(rand.NewPCG(1, 2)), now: t0}
		var out []string
		for range 50 {
			s, moves := fuzzBoot(t, rules.Standard(), m, ps, f, false)
			out = append(out, mustJSON(t, s), fmt.Sprint(len(moves)))
		}
		return mustJSON(t, out) + mustJSON(t, m)
	}
	if a, b := run(), run(); a != b {
		t.Fatal("two runs from the same seed diverged")
	}

	// Different seeds give different boots.
	m1, m2 := &game.Meta{}, &game.Meta{}
	s1, _, _ := game.NewBoot(rules.Standard(), m1, ps, 0, 1, t0)
	s2, _, _ := game.NewBoot(rules.Standard(), m2, ps, 0, 2, t0)
	if mustJSON(t, s1.Doors) == mustJSON(t, s2.Doors) {
		t.Error("seeds 1 and 2 offered the same doors")
	}
}

// TestJSONRoundTrip saves a boot mid-run, loads it, and plays the same
// actions on both copies: they must end identical.
func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 6)
	resumed := 0
	for seed := range uint64(300) {
		m := &game.Meta{Prestige: game.MaxPrestige}
		f := &fuzzer{rng: rand.New(rand.NewPCG(seed, 9)), now: t0}
		_, moves := fuzzBoot(t, rules.Standard(), m, ps, f, false)
		if len(moves) < 4 {
			continue
		}
		cut := len(moves) / 2

		// Replay the first half, snapshot, then play the rest on both.
		f2 := &fuzzer{rng: rand.New(rand.NewPCG(seed, 9)), now: t0}
		m2 := &game.Meta{Prestige: game.MaxPrestige}
		s2, _, err := game.NewBoot(rules.Standard(), m2, ps, f2.rng.IntN(m2.Prestige+1), f2.rng.Uint64(), t0)
		if err != nil {
			t.Fatal(err)
		}
		for _, mv := range moves[:cut] {
			game.Step(s2, m2, ps, mv.A, mv.At)
		}
		var s3 game.State
		var m3 game.Meta
		if err := json.Unmarshal([]byte(mustJSON(t, s2)), &s3); err != nil {
			t.Fatalf("unmarshal state: %v", err)
		}
		if err := json.Unmarshal([]byte(mustJSON(t, m2)), &m3); err != nil {
			t.Fatalf("unmarshal meta: %v", err)
		}
		if mustJSON(t, &s3) != mustJSON(t, s2) {
			t.Fatal("state JSON does not round-trip")
		}
		for _, mv := range moves[cut:] {
			fx2, err2 := game.Step(s2, m2, ps, mv.A, mv.At)
			fx3, err3 := game.Step(&s3, &m3, ps, mv.A, mv.At)
			if (err2 == nil) != (err3 == nil) || (err2 != nil) != mv.Err || fmt.Sprint(fx2) != fmt.Sprint(fx3) {
				t.Fatalf("seed %d: %T%+v diverged after the reload: %v / %v", seed, mv.A, mv.A, err2, err3)
			}
		}
		if mustJSON(t, s2) != mustJSON(t, &s3) || mustJSON(t, m2) != mustJSON(t, &m3) {
			t.Fatalf("seed %d: reloaded boot ended differently", seed)
		}
		resumed += len(moves) - cut
	}
	if resumed < 1000 {
		t.Errorf("only %d moves played after a reload", resumed)
	}
}
