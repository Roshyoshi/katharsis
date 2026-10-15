package game_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

// checker holds what invariants compare against across one boot: Meta as
// it was when the boot started (nothing outside the boot touches it until
// the boot ends), and the previous step's phase and lives.
type checker struct {
	t         testing.TB
	bank0     int64
	lifetime0 int64
	rated0    int
	solves0   int
	dump0     string // meta.CoreDump as JSON
	dumpLost  int64  // what a crash must destroy: the dump's bytes, if any
	phase     game.Phase
	lives     int
}

// newChecker is called right after NewBoot or NewTrial, before the first
// Step: no door has closed, so Meta's bank, bytes, ratings and solves are
// still what the boot started from.
func newChecker(t testing.TB, s *game.State, m *game.Meta) *checker {
	c := &checker{
		t: t, bank0: m.Bank, lifetime0: m.LifetimeBytes, rated0: m.Rated, solves0: len(m.Solves),
		dump0: mustJSON(t, m.CoreDump), phase: s.Phase, lives: s.Lives,
	}
	if m.CoreDump != nil {
		c.dumpLost = m.CoreDump.Bytes
	}
	return c
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
	// Only an ECC pick gives a life back, one at a time.
	if gain := s.Lives - c.lives; gain > 0 && (gain != 1 || a != game.PickUpgrade{Upgrade: game.ECC}) {
		fail("lives %d -> %d", c.lives, s.Lives)
	}
	c.lives = s.Lives
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
	if s.Phase != game.PhaseMerge && len(s.DaemonOffers) != 0 {
		fail("daemons %v on offer outside a merge window", s.DaemonOffers)
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
	// A crash destroys the earlier dump first, then dumps what it lost.
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
			if got := mustJSON(t, d); got != c.dump0 || s.Crash.DumpLost != 0 {
				fail("trial crash changed the core dump %s to %s", c.dump0, got)
			}
		case s.Crash.DumpLost != c.dumpLost:
			fail("crash destroyed %d B of dump, want the %d B the boot started with", s.Crash.DumpLost, c.dumpLost)
		case (lost > 0) != (d != nil) || d != nil && (d.Problem != s.Crash.Door.Problem || d.Bytes != lost):
			fail("crash lost %d, core dump %+v", lost, d)
		}
	} else if got := mustJSON(t, m.CoreDump); got != c.dump0 {
		fail("core dump %s became %s without a crash", c.dump0, got)
	}
	if s.Crash != nil && s.Trial == nil {
		// The problem that crashed you comes back, from stage 0.
		i := slices.IndexFunc(m.Regressions, func(g game.Regression) bool { return g.Problem == s.Crash.Door.Problem })
		if i < 0 || m.Regressions[i].Stage != 0 || !m.Regressions[i].Due.Equal(s.Crash.At.AddDate(0, 0, s.Rules.RegressionDays[0])) {
			fail("crash on %s, regressions %+v", s.Crash.Door.Problem, m.Regressions)
		}
	}
	regs := map[catalog.ID]bool{}
	for _, g := range m.Regressions {
		if regs[g.Problem] || g.Stage < 0 || len(g.Days) > 0 && g.Stage >= len(g.Days) {
			fail("regression %+v in %+v", g, m.Regressions)
		}
		regs[g.Problem] = true
	}
	regDoors := 0
	for _, d := range s.Doors {
		if d.Regression {
			regDoors++
		}
	}
	if s.Current != nil && s.Current.Door.Regression {
		regDoors++
	}
	// Each attempt records the biome its door opened in, which is where
	// walking the clears through the rooms puts it (only a clear moves on
	// a room), and the open door is in the current biome.
	biome, room := 0, 0
	for _, h := range s.History {
		if h.Biome != biome {
			fail("attempt %+v in biome %d, the walk says %d", h.Door, h.Biome, biome)
		}
		if h.Outcome == game.Cleared {
			if room++; room >= s.Biomes[biome].Rooms && biome < len(s.Biomes)-1 {
				biome, room = biome+1, 0
			}
		}
	}
	if s.Current != nil && s.Current.Biome != s.Biome {
		fail("open door in biome %d, boot in %d", s.Current.Biome, s.Biome)
	}
	cleared, rated := 0, 0
	for _, h := range s.History {
		if h.Door.Regression {
			regDoors++
		}
		if h.Outcome == game.Cleared {
			cleared++
		}
		if h.Outcome != game.Retreated {
			rated++
		}
	}
	if regDoors > 1 {
		fail("%d regression doors in one boot", regDoors)
	}
	// Every closed door but a retreat is rated, and every clear is a solve.
	if m.Rated != c.rated0+rated || len(m.Solves) != c.solves0+cleared {
		fail("rated %d solves %d, want %d and %d", m.Rated, len(m.Solves), c.rated0+rated, c.solves0+cleared)
	}
	for k, cal := range m.Calibration {
		limit := s.Rules.CalibMax
		if k != game.CalibrationAll {
			limit /= 2
		}
		if math.Abs(cal.Offset) > limit || cal.Observed < 0 || cal.Observed > 1 || cal.Expected < 0 || cal.Expected > 1 {
			fail("calibration %s %+v outside ±%.0f", k, cal, limit)
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
	if m.Bank != c.bank0+s.Committed || m.LifetimeBytes != c.lifetime0+s.Earned() {
		fail("bank %d lifetime %d, want %d + committed %d and %d + earned %d",
			m.Bank, m.LifetimeBytes, c.bank0, s.Committed, c.lifetime0, s.Earned())
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
		switch r.IntN(20) {
		case 0:
			return game.GiveUp{}
		case 1, 2:
			return game.Quack{}
		case 3, 4:
			return game.Retreat{}
		case 5, 6, 7, 8:
			return game.Tick{}
		}
		return f.submit()
	}
	return game.Tick{}
}

func (f *fuzzer) submit() game.SubmitResult {
	v := []game.Verdict{"AC", "AC", "AC", "WA", "TLE", "RE", "MLE", "CE"}[f.rng.IntN(8)]
	return game.SubmitResult{Verdict: v, Passed: f.rng.IntN(11), Total: 10}
}

func (f *fuzzer) tick() {
	f.now = f.now.Add(time.Duration(f.rng.IntN(int(12 * time.Minute))))
}

// move is one fuzzed action with its time, for replay.
type move struct {
	A   game.Action
	At  time.Time
	Err bool
}

// startBoot starts a boot at a random unlocked prestige.
func startBoot(t testing.TB, r rules.Rules, m *game.Meta, ps []catalog.Problem, f *fuzzer) *game.State {
	t.Helper()
	s, _, err := game.NewBoot(r, m, ps, f.rng.IntN(m.Prestige+1), f.rng.Uint64(), f.now)
	if err != nil {
		t.Fatalf("NewBoot: %v", err)
	}
	return s
}

// startTrial starts a learn-mode trial on a spec drawn from ps.
func startTrial(t testing.TB, m *game.Meta, ps []catalog.Problem, f *fuzzer) *game.State {
	t.Helper()
	spec := fuzzSpec(f.rng, ps)
	s, _, err := game.NewTrial(rules.Trial(), m, ps, spec, f.rng.Uint64(), f.now)
	if err != nil {
		t.Fatalf("NewTrial(%+v): %v", spec, err)
	}
	return s
}

// fuzzBoot plays one boot to its end (or a step cap).
func fuzzBoot(t testing.TB, r rules.Rules, m *game.Meta, ps []catalog.Problem, f *fuzzer, verify bool) (*game.State, []move) {
	t.Helper()
	s := startBoot(t, r, m, ps, f)
	return s, newChecker(t, s, m).play(s, m, ps, f, verify)
}

// fuzzTrial is fuzzBoot for a trial.
func fuzzTrial(t testing.TB, m *game.Meta, ps []catalog.Problem, f *fuzzer, verify bool) (*game.State, []move) {
	t.Helper()
	s := startTrial(t, m, ps, f)
	return s, newChecker(t, s, m).play(s, m, ps, f, verify)
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
						c = newChecker(t, s, m)
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

// TestJSONRoundTrip saves boots and trials mid-run, loads them, and plays
// the same actions on both copies: they must end identical. Replaying the
// first half on a fresh start must also accept and refuse what the original
// run did.
func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 6)
	for _, tc := range []struct {
		name       string
		seeds      uint64
		minResumed int
		start      func(*game.Meta, *fuzzer) *game.State
	}{
		{"boot", 300, 1000, func(m *game.Meta, f *fuzzer) *game.State { return startBoot(t, rules.Standard(), m, ps, f) }},
		{"trial", 150, 300, func(m *game.Meta, f *fuzzer) *game.State { return startTrial(t, m, ps, f) }},
	} {
		resumed := 0
		for seed := range tc.seeds {
			fresh := func() (*game.State, *game.Meta, *fuzzer) {
				m := &game.Meta{Prestige: game.MaxPrestige}
				f := &fuzzer{rng: rand.New(rand.NewPCG(seed, 9)), now: t0}
				return tc.start(m, f), m, f
			}
			s, m, f := fresh()
			moves := newChecker(t, s, m).play(s, m, ps, f, false)
			if len(moves) < 4 {
				continue
			}
			cut := len(moves) / 2

			s2, m2, _ := fresh()
			for _, mv := range moves[:cut] {
				if _, err := game.Step(s2, m2, ps, mv.A, mv.At); (err != nil) != mv.Err {
					t.Fatalf("%s seed %d: replayed %T%+v: %v", tc.name, seed, mv.A, mv.A, err)
				}
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
				t.Fatalf("%s seed %d: state JSON does not round-trip", tc.name, seed)
			}
			for _, mv := range moves[cut:] {
				fx2, err2 := game.Step(s2, m2, ps, mv.A, mv.At)
				fx3, err3 := game.Step(&s3, &m3, ps, mv.A, mv.At)
				if (err2 == nil) != (err3 == nil) || (err2 != nil) != mv.Err || fmt.Sprint(fx2) != fmt.Sprint(fx3) {
					t.Fatalf("%s seed %d: %T%+v diverged after the reload: %v / %v", tc.name, seed, mv.A, mv.A, err2, err3)
				}
			}
			if mustJSON(t, s2) != mustJSON(t, &s3) || mustJSON(t, m2) != mustJSON(t, &m3) {
				t.Fatalf("%s seed %d: reloaded run ended differently", tc.name, seed)
			}
			resumed += len(moves) - cut
		}
		if resumed < tc.minResumed {
			t.Errorf("%s: only %d moves played after a reload", tc.name, resumed)
		}
	}
}
