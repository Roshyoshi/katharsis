// Package sim plays boots headlessly with a model player. It exists to
// balance rules presets (clear rates, crash spread, uptime, bytes) and to
// drive the engine's property tests through thousands of boots.
//
// The model player is a deliberately simple stand-in, not a prediction: it
// solves a door with the logistic chance of its true rating, takes a
// lognormal time to do it, sometimes submits wrong answers, spends ducks when
// stuck, and picks the door worth most to it (Player.Stake). The engine never
// sees the true rating; it has to learn it from outcomes like it would a
// person.
package sim

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// Player is the model.
type Player struct {
	Rating float64 // true skill, CF scale

	// Solve time is lognormal with this median at gap 0, scaled by
	// 2^(gap/GapScale) for a door gap points above the player.
	Median   time.Duration
	Sigma    float64
	GapScale float64

	WAProb      float64       // chance each submit before the AC fails
	StuckWAs    float64       // mean failed submits on a door it can't solve
	DuckBoost   float64       // chance a quack un-sticks it; 0 never quacks
	BlindCost   float64       // effective rating lost when tags are hidden
	UpsolveProb float64       // chance it upsolves the crash between boots
	Overhead    time.Duration // clock time per choice outside a door

	// FloorDrop is how far under its rating, on average, a door at the
	// pool's floor (Door.Floor: Codeforces rates nothing under 800) really
	// is: each such problem's difficulty is its rating less an exponential
	// draw of this mean, fixed per problem (a hash of its ID). 0 plays it
	// at its rating.
	FloorDrop float64

	// Stake is how much of what a crash forfeits the model weighs when it
	// picks a door: it takes the door with the best P × (bytes + Stake ×
	// at stake), where at stake is the uncommitted bytes and what the
	// rest of the boot pays on average (BaseBytes/2 a door, a boss
	// BossMult of that). 0 is the myopic bytes × P pick, which takes a
	// cursed Userland door for its double pay although losing it loses
	// the boot.
	Stake float64

	// Learn, when set, makes the player learn, space its boots and churn
	// (ml-pipeline §5.2; see Learning). ItemSD is each problem's true
	// difficulty noise around its rating (sd, fixed per problem by a hash of
	// its ID). Lambda > 0 (with Learn) picks doors by a softmax of
	// ln(bytes) + Lambda·ln(p), p its own clear chance misjudged by BeliefSD
	// in log-odds, instead of the stake pick.
	Learn            *Learning
	ItemSD           float64
	Lambda, BeliefSD float64
	// DeclineDaemons has the model turn down daemons that cost what it is
	// short of: chrt cuts the timer (most of its crashes are timeouts), and
	// nice where a preset makes it charge more than a life a wrong submit
	// (standard's nice costs bytes). Without it, it takes whatever a merge
	// window offers.
	DeclineDaemons bool

	// Spend has the model buy in merge windows before it commits, by what
	// the boot needs: an fsck for each life lost below the boot's start, a
	// dup when out of ducks, a shuf to hold while rooms are left before the
	// boss (spent on a room whose best door it expects to clear
	// RerollMargin below the biome's target or worse), and at before the
	// boss.
	Spend        bool
	RerollMargin float64
	// SpendShare, when above 0, replaces Spend with a fixed policy: each
	// merge window it spends up to this share of the bytes it found
	// uncommitted, buying whatever is on sale in shelf order, round and
	// round, and commits the rest. 1 spends while anything is affordable.
	SpendShare float64

	// Route is how the model takes a map exit among the open ones: "" at
	// random, "long" the most rooms, "short" the fewest, "gated" a scroll
	// (an upgrade pick at the exit) first. Real players pick by topic.
	Route string

	// GiveUpAt, above 0, has the model :q! a door it will not clear in time
	// at this share of the timer, where a give-up closes the door for no
	// more than a timeout would cost (State.FailCost; D47: a give-up and a
	// timeout both burn the biome's price). 0 waits for the timer.
	GiveUpAt float64

	// TwistCost is the effective rating a boss twist (Door.Twist) costs the
	// model on that boss's doors, as BlindCost does hidden tags: the twists
	// the engine does not carry out itself hide or take something the
	// model would have used (samples' detail, sanitizers, the clock, half
	// the memory). Off-by-One and Race Condition need none: their burn and
	// clock are the engine's.
	TwistCost map[string]float64

	// Trial has the model set out for the Trial (achievements.md §(f)): it
	// keeps its bytes uncommitted until the merge window before Boss II,
	// buys fsck there up to the cycle cap while it can afford one, and takes
	// the Trial's exit whenever it is open. Without it the model never takes
	// a hidden exit.
	Trial bool

	// Install lists the packages (rules.Package) the model buys at GRUB
	// between boots, in this order, each once the bank covers it; nil buys
	// none.
	Install []string
}

// DefaultPlayer is the model the reports in this package's tests use.
func DefaultPlayer(rating float64) Player {
	return Player{
		Rating:         rating,
		Median:         30 * time.Minute,
		Sigma:          0.5,
		GapScale:       200,
		WAProb:         0.2,
		StuckWAs:       1,
		DuckBoost:      0.35,
		BlindCost:      25,
		UpsolveProb:    0.6,
		Overhead:       time.Minute,
		Stake:          1,
		FloorDrop:      250,
		DeclineDaemons: true,
		GiveUpAt:       0.75,
		// As much as hidden tags (BlindCost): an assumption, not a
		// measurement (no player has met a twist yet).
		TwistCost: map[string]float64{"oom-killer": 25, "heisenbug": 25, "undefined-behavior": 25, "halting-problem": 25},

		Spend:        true,
		RerollMargin: 0.2,

		// The whole standard catalog, cheapest first, as retention RT3
		// prices it: the next package is the nearest goal.
		Install: CheapestFirst(rules.Standard()),
	}
}

// CheapestFirst names r's packages in r.Ladder's order, cheapest first.
func CheapestFirst(r rules.Rules) []string {
	ps := r.Ladder()
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return names
}

// Config is one simulation.
type Config struct {
	Rules  rules.Rules
	Pool   []catalog.Problem
	Player Player
	Boots  int
	Seed   uint64
	// Prestige < 0 climbs: every boot plays the highest unlocked level.
	// Otherwise every boot plays this level (unlocked up front).
	Prestige int
	// Warmup boots are played but left out of the report, so it shows the
	// steady state after the engine has learned the player.
	Warmup int
	// Meta is carried across boots; nil starts a player with every
	// capability (the model has done learn mode) and nothing else.
	Meta *game.Meta
	// Check, when set, sees every engine call; tests hook invariants here.
	Check func(s *game.State, m *game.Meta, a game.Action, fx []game.Effect, err error)
	// Days, when above 0, also ends the run that many days in, as a
	// learning player's year (Player.Learn).
	Days int
	// Block is how many boots each entry of Report.Blocks sums, warm-up
	// included; 0 leaves Blocks empty.
	Block int
	// ClimbFloor, above 0 with Prestige < 0, has the player choose its
	// level: the highest unlocked one whose last 10 boots there cleared at
	// least this share (fewer than 10 count as cleared), a level that fell
	// short tried again 5 boots after its last (map.md §(i)5).
	ClimbFloor float64
}

// Block sums a stretch of boots, to show the cold start, the calibration
// settling and the pool running dry over a run.
type Block struct {
	Boots, FullClears int
	Opened, OffTarget int                // doors opened, and how many off target
	Installed         int                // packages installed at the end
	Rating            float64            // the engine's rating at the end
	Offsets           map[string]float64 // calibration offsets at the end
}

// Tally counts boots and full clears at one prestige level.
type Tally struct{ Boots, FullClears int }

// Report sums a simulation. Per-biome slices are indexed like each boot's
// State.Biomes: Rules.Biomes, except that boot #1 plays Rules.FirstBiomes,
// so a run from a new player has as many entries as the longer of the two.
type Report struct {
	Boots, FullClears int
	ClearRate         float64

	CrashBiome []int // crashes per biome
	// CrashTimeouts and CrashLives split CrashBiome by cause: the timer ran
	// out, or the lives (or a curse) did; the rest gave up.
	CrashTimeouts, CrashLives []int

	Causes    map[game.Outcome]int
	Attempts  []int // doors opened per biome
	Clears    []int // doors cleared per biome
	MeanTimer []time.Duration
	// OffTarget counts opened doors more than the first rating window from
	// their target (regressions aside): where the pool had nothing near.
	OffTarget []int
	// Taught counts opened doors that teach the aim calibration (plain and
	// aimed, Door.Tier set), TaughtClears how many of them were cleared and
	// TaughtP the sum of their SolveP, so TaughtClears/Taught against
	// TaughtP/Taught is the calibration's observed against expected.
	Taught, TaughtClears []int
	TaughtP              []float64
	// Offered counts the doors on offer within the first rating window of
	// their target (regressions aside, modifiers or not) when the model
	// chose (or re-rolled), and OfferedP sums their SolveP:
	// OfferedP/Offered is where the engine aimed, before the model's choice
	// among them. Plain doors alone would lean easy, since each Userland
	// room's easiest door is plain (Biome.Plain).
	Offered  []int
	OfferedP []float64

	// Uptime is the game's (game.State.Uptime): time inside doors.
	MeanUptime, MeanClearUptime, MeanCrashUptime time.Duration
	// MedianCrashUptime is the median boot length at a crash.
	MedianCrashUptime time.Duration
	// MeanWall and MeanClearWall are boot lengths in wall time: the
	// doors and Player.Overhead for every choice between them, the
	// length of a sitting (D13's 2 h clear).
	MeanWall, MeanClearWall time.Duration

	MeanEarned, MeanCommitted, MeanSpent, MeanLost float64 // bytes per boot
	Rerolls                                        int     // shufs used
	Recovered                                      int64   // core-dump bytes upsolved back
	DumpsLost                                      int64   // core-dump bytes crashed away

	// InstalledAt is the boot (counted from 1, warm-up included) after
	// which each package of Player.Install was bought, 0 if never.
	InstalledAt []int

	Prestige   int     // highest unlocked at the end
	PrestigeAt []int   // boot that unlocked level i+1, 0 if never
	ByPrestige []Tally // index: prestige level

	FinalRating float64 // the engine's estimate of the player at the end
	Errors      int     // engine calls the model made that failed
	Meta        *game.Meta

	// Eased counts boots by the easings they ran with (State.Ease, not
	// declined: the model never declines), Saved the timeouts the floor
	// easing turned into a lost life, and Forecast sums State.Forecast,
	// the engine's own chance of a crash in the first biome, so
	// Forecast/Boots can be set against CrashBiome[0]/Boots (G1).
	Eased    map[game.EaseKind]int
	Saved    int
	Forecast float64

	// Ratings is the engine's rating after each boot, warm-up included.
	Ratings []float64
	// Banked is what each boot added to the bank, warm-up included: its
	// commits and the core dump upsolved after it.
	Banked []int64
	Blocks []Block

	// Learn sums a learning player's run (Player.Learn), nil otherwise.
	Learn *LearnStats

	// The map (map.md §(i)): each boot's time inside doors (D46's uptime),
	// the cleared boots' apart; the cycles fountains restored, the boss
	// doors failed without ending the boot, the fscks bought, how often
	// each node was taken, and each boss's boots.
	DoorTime, DoorTimeClear      []time.Duration
	Fountained, BossSaves, Fscks int
	Nodes                        map[string]int
	Bosses                       map[string]BossTally

	// The Trial (achievements.md §(f)): Wakes are the boots it ended awake,
	// cleared but no full clear (FullClears and the clear means leave them
	// out); TrialOpen counts the boots that met its exit open at Boss II's
	// tier, TrialTaken those that took it, and Capped the boots whose pool
	// reached its cap (State.Capped), the Trial's gate and hint.
	Wakes, TrialOpen, TrialTaken, Capped int
}

// BossTally is one boss's boots: how many reached it, how many it ended,
// and its doors failed for cycles (a retry).
type BossTally struct{ Reached, Crashed, Retries int }

// Simulate runs boots with DefaultPlayer(playerRating), climbing prestige.
func Simulate(r rules.Rules, pool []catalog.Problem, playerRating float64, boots int, seed uint64) Report {
	return Run(Config{Rules: r, Pool: pool, Player: DefaultPlayer(playerRating), Boots: boots, Seed: seed, Prestige: -1})
}

// Run is Simulate with every knob.
func Run(c Config) Report {
	m := c.Meta
	if m == nil {
		m = &game.Meta{}
		for _, cap := range game.TopicCapabilities {
			game.GrantCapability(m, cap)
		}
		slices.Sort(m.Capabilities)
	}
	nb := len(c.Rules.Biomes)
	if m.Boots == m.Trials { // boot #1 plays its own shape
		nb = max(nb, len(c.Rules.FirstBiomes))
	}
	if c.Prestige >= 0 {
		m.Prestige = max(m.Prestige, min(c.Prestige, game.MaxPrestige))
	}
	rng := rand.New(rand.NewPCG(c.Seed, c.Seed^0x5deece66d))
	clock := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	start := clock
	var lr *learner
	var lt tally
	var ls *LearnStats
	if c.Player.Learn != nil {
		lr = newLearner(c.Player.Learn, c.Player.Rating, c.Seed)
		ls = &LearnStats{}
	}

	warm, main := newSums(nb), newSums(nb)
	main.rep.InstalledAt = make([]int, len(c.Player.Install))
	var ratings []float64
	var banked []int64
	var blocks []Block
	recent := map[int][]bool{} // outcomes per prestige, for ClimbFloor
	last := map[int]int{}      // the boot each prestige was last played
	for i := range c.Warmup + c.Boots {
		sm := main
		if i < c.Warmup {
			sm = warm
		}
		rep := &sm.rep
		p := m.Prestige
		if c.Prestige >= 0 {
			p = min(c.Prestige, m.Prestige)
		} else if c.ClimbFloor > 0 {
			// A level that fell short is tried again 5 boots after its last.
			for p > 0 && recentClear(recent[p]) < c.ClimbFloor && i-last[p] < 5 {
				p--
			}
			last[p] = i
		}
		s, fx, err := game.NewBoot(c.Rules, m, c.Pool, p, rng.Uint64(), clock)
		if c.Check != nil {
			c.Check(s, m, nil, fx, err)
		}
		if err != nil {
			main.rep.Errors++
			break
		}
		b := &boot{c: &c, p: c.Player, s: s, m: m, rng: rng, clock: clock, sums: sm, lives0: s.Cycles, window: -1, learn: lr}
		b.play()

		rep.Boots++
		rep.ByPrestige[p].Boots++
		rep.Forecast += s.Forecast
		sm.mapped(s)
		for _, es := range s.Ease {
			rep.Eased[es.Kind]++
		}
		for j, h := range s.History {
			if h.Outcome == game.TimedOut && (s.Crash == nil || j < len(s.History)-1) {
				rep.Saved++
			}
		}
		sm.earned += s.Earned()
		sm.committed += s.Committed
		sm.spent += s.Spent
		in := s.Committed
		wall := b.clock.Sub(s.Started)
		sm.wall += wall
		sm.trial(s)
		switch {
		case s.Phase == game.PhaseCleared && s.Ending != "":
			rep.Wakes++
			sm.upWake += s.Uptime(b.clock)
		case s.Phase == game.PhaseCleared:
			rep.FullClears++
			rep.ByPrestige[p].FullClears++
			sm.upClear += s.Uptime(b.clock)
			sm.wallClear += wall
		case s.Phase == game.PhaseCrashed:
			rep.CrashBiome[s.Biome]++
			switch s.Crash.Cause {
			case game.TimedOut:
				rep.CrashTimeouts[s.Biome]++
			case game.OutOfLives:
				rep.CrashLives[s.Biome]++
			}
			sm.crashUps = append(sm.crashUps, s.Crash.Uptime)
			rep.Causes[s.Crash.Cause]++
			sm.upCrash += s.Crash.Uptime
			sm.lost += s.Crash.Lost
			rep.DumpsLost += s.Crash.DumpLost
			upsolved := ""
			if d := m.CoreDump; d != nil && rng.Float64() < c.Player.UpsolveProb {
				got := game.Upsolve(m, d.Problem, game.Editorial, 45*time.Minute, b.clock.Add(time.Hour))
				rep.Recovered += got
				in += got
				upsolved = "editorial"
			}
			if pc := b.crashed; lr != nil && pc != nil {
				lr.learn(pc.a.Door.Topic, pc.p, lr.outcomeF(pc.a, upsolved), pc.a.Closed)
			}
		}
		for j, name := range c.Player.Install {
			if main.rep.InstalledAt[j] != 0 || game.Installed(m, name) {
				continue
			}
			p, ok := c.Rules.Package(name)
			if !ok || game.Install(m, p) != nil {
				break // saving up for it
			}
			main.rep.InstalledAt[j] = i + 1
		}
		for lvl := 1; lvl <= m.Prestige; lvl++ {
			if main.rep.PrestigeAt[lvl-1] == 0 {
				main.rep.PrestigeAt[lvl-1] = i + 1
			}
		}
		cleared := s.Phase == game.PhaseCleared && s.Ending == ""
		recent[p] = append(recent[p], cleared)
		quit := false
		if lr == nil || !lr.l.Sessions {
			clock = clock.Add(24 * time.Hour) // a boot a day, so regressions come due
		} else {
			clock = b.clock.Add(lr.gap(cleared))
		}
		if lr != nil {
			if i >= c.Warmup {
				lt.observe(s, m, lr, b.clock)
				ls.Hours += b.clock.Sub(s.Started).Hours()
			}
			earlyRun, crashRun, clearRun := lt.ended(cleared, !cleared && s.Biome == 0)
			if ch := lr.l.Churn; ch != nil {
				pat := lt.pattern()
				churned := lr.rng.Float64() < ch.hazard(!cleared && s.Biome == 0, earlyRun, crashRun, clearRun, b.clock.Sub(s.Started))
				switch pat {
				case "LLL":
					ls.LLL++
					if churned {
						ls.LLLChurned++
					}
				case "LLW":
					ls.LLW++
					if churned {
						ls.LLWChurned++
					}
				}
				if churned {
					if lr.rng.Float64() < ch.QuitShare {
						quit = true
					} else {
						ls.Breaks++
						clock = clock.Add(time.Duration(lr.rng.ExpFloat64() * float64(ch.Break)))
					}
				}
			}
			ls.Days = b.clock.Sub(start).Hours() / 24
		}
		ratings = append(ratings, m.Rating)
		banked = append(banked, in)
		if c.Block > 0 {
			if i%c.Block == 0 {
				blocks = append(blocks, Block{})
			}
			bl := &blocks[len(blocks)-1]
			bl.Boots++
			if cleared {
				bl.FullClears++
			}
			bl.Opened += b.opened
			bl.OffTarget += b.off
			bl.Rating = m.Rating
			bl.Installed = len(m.Installed)
			bl.Offsets = map[string]float64{}
			for k, v := range m.Calibration {
				bl.Offsets[k] = v.Offset
			}
		}
		if quit || c.Days > 0 && clock.Sub(start) > time.Duration(c.Days)*24*time.Hour {
			if ls != nil {
				ls.Quit = quit
			}
			break
		}
	}
	main.rep.Errors += warm.rep.Errors

	rep := main.finish()
	rep.InstalledAt = main.rep.InstalledAt
	rep.Prestige = m.Prestige
	rep.FinalRating = m.Rating
	rep.Meta = m
	rep.Ratings = ratings
	rep.Banked = banked
	rep.Blocks = blocks
	if ls != nil {
		end := start.Add(time.Duration(ls.Days * 24 * float64(time.Hour)))
		ls.Gain = lr.durable(end)
		ls.Durable = lr.durable(end.Add(30 * 24 * time.Hour))
		if lt.n > 0 {
			ls.RMSE = math.Sqrt(lt.sq / float64(lt.n))
		}
		ls.ECE = lt.ece()
		rep.Learn = ls
	}
	return rep
}

// sums accumulates one report.
type sums struct {
	rep                            Report
	timerSum                       []time.Duration
	upClear, upCrash, upWake       time.Duration
	wall, wallClear                time.Duration
	crashUps                       []time.Duration
	earned, committed, spent, lost int64
}

func newSums(nb int) *sums {
	return &sums{
		rep: Report{
			CrashBiome:    make([]int, nb),
			CrashTimeouts: make([]int, nb),
			CrashLives:    make([]int, nb),
			Causes:        map[game.Outcome]int{},
			Eased:         map[game.EaseKind]int{},
			Nodes:         map[string]int{},
			Bosses:        map[string]BossTally{},
			Attempts:      make([]int, nb),
			Clears:        make([]int, nb),
			MeanTimer:     make([]time.Duration, nb),
			OffTarget:     make([]int, nb),
			Taught:        make([]int, nb),
			TaughtClears:  make([]int, nb),
			TaughtP:       make([]float64, nb),
			Offered:       make([]int, nb),
			OfferedP:      make([]float64, nb),
			PrestigeAt:    make([]int, game.MaxPrestige),
			ByPrestige:    make([]Tally, game.MaxPrestige+1),
		},
		timerSum: make([]time.Duration, nb),
	}
}

func (sm *sums) finish() Report {
	rep := sm.rep
	if n := rep.Boots; n > 0 {
		rep.ClearRate = float64(rep.FullClears) / float64(n)
		rep.MeanEarned = float64(sm.earned) / float64(n)
		rep.MeanCommitted = float64(sm.committed) / float64(n)
		rep.MeanLost = float64(sm.lost) / float64(n)
		rep.MeanSpent = float64(sm.spent) / float64(n)
		rep.MeanUptime = (sm.upClear + sm.upCrash + sm.upWake) / time.Duration(n)
		rep.MeanWall = sm.wall / time.Duration(n)
	}
	if rep.FullClears > 0 {
		rep.MeanClearUptime = sm.upClear / time.Duration(rep.FullClears)
		rep.MeanClearWall = sm.wallClear / time.Duration(rep.FullClears)
	}
	if n := rep.Boots - rep.FullClears - rep.Wakes; n > 0 {
		rep.MeanCrashUptime = sm.upCrash / time.Duration(n)
	}
	if len(sm.crashUps) > 0 {
		ups := slices.Clone(sm.crashUps)
		slices.Sort(ups)
		rep.MedianCrashUptime = ups[len(ups)/2]
	}
	for i := range rep.Attempts {
		if rep.Attempts[i] > 0 {
			rep.MeanTimer[i] = sm.timerSum[i] / time.Duration(rep.Attempts[i])
		}
	}
	return rep
}

// boot is one boot being played.
type boot struct {
	c     *Config
	p     Player
	s     *game.State
	m     *game.Meta
	rng   *rand.Rand
	clock time.Time
	sums  *sums

	opened, off int // doors opened this boot, and how many off target
	lives0      int // lives at the boot's start

	// The fixed spend policy's merge window: which biome it closes (-1
	// before the first), what is left of its budget, and where on the
	// shelf the next buy starts.
	window int
	budget int64
	shelf  int

	// learn is the learning player's skill (nil: a fixed rating), and
	// crashed the door the boot crashed on with its true chance, which
	// learns only once the upsolve after the boot is known.
	learn   *learner
	crashed *pending
}

// pending is a closed door whose lesson waits on the upsolve.
type pending struct {
	a game.Attempt
	p float64
}

func (b *boot) step(a game.Action) {
	fx, err := game.Step(b.s, b.m, b.c.Pool, a, b.clock)
	if b.c.Check != nil {
		b.c.Check(b.s, b.m, a, fx, err)
	}
	if err != nil {
		b.sums.rep.Errors++
	}
}

func (b *boot) play() {
	s := b.s
	for n := 0; s.Phase != game.PhaseCrashed && s.Phase != game.PhaseCleared; n++ {
		if n > 10000 {
			b.sums.rep.Errors++ // the model is stuck; the engine let it be
			return
		}
		b.clock = b.clock.Add(b.p.Overhead)
		if s.Phase == game.PhaseDoors && !s.PendingUpgrade {
			for _, d := range s.Doors {
				if !d.Regression && b.onTarget(d) {
					b.sums.rep.Offered[s.Biome]++
					b.sums.rep.OfferedP[s.Biome] += d.SolveP
				}
			}
		}
		switch {
		case s.PendingUpgrade:
			b.step(game.PickUpgrade{Upgrade: b.upgrade()})
		case s.Phase == game.PhaseRoute:
			b.step(game.ChooseRoute{Index: b.route()})
		case s.Phase == game.PhaseDoors && b.reroll():
			b.sums.rep.Rerolls++
			b.step(game.Reroll{})
		case s.Phase == game.PhaseDoors:
			i := b.door()
			d := s.Doors[i]
			b.sums.rep.Attempts[s.Biome]++
			b.sums.timerSum[s.Biome] += d.Timer
			b.opened++
			if !d.Regression && !b.onTarget(d) {
				b.sums.rep.OffTarget[s.Biome]++
				b.off++
			}
			biome := s.Biome
			b.step(game.ChooseDoor{Index: i})
			b.attempt(biome)
			if h := s.History; d.Tier != "" && len(h) > 0 {
				rep := &b.sums.rep
				rep.Taught[biome]++
				rep.TaughtP[biome] += d.SolveP
				if h[len(h)-1].Outcome == game.Cleared {
					rep.TaughtClears[biome]++
				}
			}
		case s.Phase == game.PhaseMerge:
			it := b.buy()
			d, pick := b.daemon()
			switch {
			case it != "":
				if it == game.Fsck {
					b.sums.rep.Fscks++
				}
				b.step(game.Spend{Item: it})
			case s.Uncommitted > 0 && !b.hoard():
				b.step(game.Commit{})
			case pick:
				b.step(game.PickDaemon{Daemon: d})
			default:
				b.step(game.LeaveMerge{})
			}
		default:
			b.sums.rep.Errors++
			return
		}
	}
}

// difficulty is d's true difficulty to the model: its rating (with the
// item noise, Player.ItemSD), or under the floor by a draw fixed per
// problem (Player.FloorDrop).
func (b *boot) difficulty(d game.Door) float64 {
	r := float64(d.Rating)
	if b.p.ItemSD > 0 {
		r += b.p.ItemSD * hashNormal("item:"+string(d.Problem))
	}
	if !d.Floor || b.p.FloorDrop <= 0 {
		return r
	}
	return r + b.p.FloorDrop*math.Log(1-hashUnit(string(d.Problem)))
}

// onTarget reports whether d is within the first rating window of the
// engine's target, as the report's Offered and OffTarget count.
func (b *boot) onTarget(d game.Door) bool {
	s := b.s
	return math.Abs(d.Difficulty(&s.Rules)-s.Target(b.m, d.Topic)) <= float64(s.Rules.RatingWindows[0])
}

// median is the model's median solve time on d.
func (b *boot) median(d game.Door) float64 {
	return float64(b.p.Median) * math.Exp2((b.difficulty(d)-b.skill(d.Topic))/b.p.GapScale)
}

// skill is the model's true rating on topic now: its fixed Rating, or what
// learning has made of it.
func (b *boot) skill(topic string) float64 {
	if b.learn == nil {
		return b.p.Rating
	}
	return b.learn.skill(topic, b.clock)
}

// solveP is the model's own chance on d, which the engine only estimates.
// A learning model recalls a regression door's problem with its recall,
// and solves it fresh otherwise.
func (b *boot) solveP(d game.Door) float64 {
	eff := b.skill(d.Topic)
	if d.TagsHidden() {
		eff -= b.p.BlindCost
	}
	if d.Mystery {
		eff -= b.p.BlindCost
	}
	eff -= b.p.TwistCost[d.Twist]
	p := game.Expected(eff, b.difficulty(d))
	if b.learn != nil && d.Regression {
		if seen, ok := b.learn.seen[string(d.Problem)]; ok {
			r, _ := b.learn.recall(string(d.Problem), b.clock.Sub(seen))
			p = r + (1-r)*p
		}
	}
	return p
}

// clearP is the model's own chance of clearing d: solving it at all (with
// the duck it holds, which a cursed door refuses), within the timer, and
// (cursed) without a single wrong submit.
func (b *boot) clearP(d game.Door) float64 {
	inTime := 0.5 * (1 + math.Erf(math.Log(float64(d.Timer)/b.median(d))/(b.p.Sigma*math.Sqrt2)))
	solve := b.solveP(d)
	if d.Cursed {
		solve *= 1 - b.p.WAProb
	} else if b.s.Ducks > 0 {
		solve += (1 - solve) * b.p.DuckBoost
	}
	return solve * inTime
}

// door is the best P × (bytes + Stake × at stake) on offer (Player.Stake);
// the first wins ties.
func (b *boot) door() int {
	if b.p.Lambda > 0 && b.learn != nil {
		return b.softDoor()
	}
	stake := b.p.Stake * b.atStake()
	best, bi := -1.0, 0
	for i, d := range b.s.Doors {
		if v := (float64(d.Bytes) + stake) * b.clearP(d); v > best {
			best, bi = v, i
		}
	}
	return bi
}

// softDoor is ml-pipeline §5.2's choice: a softmax over the doors of
// ln(bytes) + Lambda·ln(p), p the model's clear chance misjudged by
// BeliefSD in log-odds. Lambda 1 without noise is bytes × P.
func (b *boot) softDoor() int {
	w := make([]float64, len(b.s.Doors))
	top, sum := math.Inf(-1), 0.0
	for i, d := range b.s.Doors {
		p := min(max(b.clearP(d), 1e-6), 1-1e-6)
		p = 1 / (1 + math.Exp(-(math.Log(p/(1-p)) + b.p.BeliefSD*b.learn.rng.NormFloat64())))
		w[i] = math.Log(float64(d.Bytes)) + b.p.Lambda*math.Log(p)
		top = max(top, w[i])
	}
	for i := range w {
		w[i] = math.Exp(w[i] - top)
		sum += w[i]
	}
	u := b.learn.rng.Float64() * sum
	for i, x := range w {
		if u -= x; u < 0 {
			return i
		}
	}
	return len(w) - 1
}

// atStake is what crashing on the door about to open forfeits, as the
// model reckons it: the uncommitted bytes, and BaseBytes/2 for each door
// left after this one (a boss's BossMult of that), what a door pays on
// average (retention GR5).
func (b *boot) atStake() float64 {
	s := b.s
	left := 0.0
	for i := s.Biome; i < len(s.Biomes); i++ {
		rooms := s.Biomes[i].Rooms
		if i == s.Biome {
			rooms -= s.Room + 1
		}
		w := 1.0
		if s.Biomes[i].Kind == rules.Boss {
			w = s.Rules.BossMult
		}
		left += w * float64(max(rooms, 0))
	}
	return float64(s.Uncommitted) + left*float64(s.Rules.BaseBytes)/2
}

// buy is what the model buys next in a merge window, "" for nothing.
func (b *boot) buy() game.Item {
	s := b.s
	if b.p.SpendShare > 0 {
		return b.buyFixed()
	}
	if !b.p.Spend {
		return ""
	}
	bossNext := s.Biome+1 < len(s.Biomes) && s.Biomes[s.Biome+1].Kind == rules.Boss
	// Set out for the Trial: fsck to the cap in the window before Boss II.
	trial := b.p.Trial && len(s.Rules.Nodes) > 0 && s.Biome == rules.TierActII
	for _, want := range []struct {
		it game.Item
		ok bool
	}{
		{game.Fsck, trial},
		// An fsck for each life lost below the boot's start (a life is
		// LifeCycles under cycles: 10 of 30).
		{game.Fsck, s.Cycles <= b.lives0-s.Rules.Life()},
		{game.Dup, s.Ducks == 0},
		{game.Shuf, s.Rerolls == 0 && !bossNext},
		{game.At, s.AtTime == 0 && bossNext},
		{game.Nohup, s.Nohup == 0 && bossNext},
	} {
		if want.ok && s.CanSpend(want.it) == nil {
			return want.it
		}
	}
	return ""
}

// buyFixed is the SpendShare policy: the next item on the shelf, from
// where the last buy left off, that the window's budget covers.
func (b *boot) buyFixed() game.Item {
	s := b.s
	if b.window != s.Biome {
		b.window, b.shelf = s.Biome, 0
		b.budget = int64(b.p.SpendShare * float64(s.Uncommitted))
	}
	pool := s.ItemPool()
	n := len(pool)
	for k := range n {
		i := (b.shelf + k) % n
		it := pool[i]
		if s.CanSpend(it) == nil && s.Price(it) <= b.budget {
			b.budget -= s.Price(it)
			b.shelf = i + 1
			return it
		}
	}
	return ""
}

// reroll reports whether the model spends a shuf on the room on offer: it
// holds one and expects to clear the best door RerollMargin below the
// biome's target or worse.
func (b *boot) reroll() bool {
	s := b.s
	if s.Rerolls == 0 || s.PendingUpgrade || s.Biomes[s.Biome].Kind == rules.Boss {
		return false
	}
	best := 0.0
	for _, d := range s.Doors {
		best = max(best, b.clearP(d))
	}
	return best < s.Rules.Biomes[s.Biome].TargetClear-b.p.RerollMargin
}

func (b *boot) upgrade() game.Upgrade {
	offers := b.s.UpgradeOffers
	for _, want := range []struct {
		u  game.Upgrade
		ok bool
	}{
		{game.ECC, b.s.Cycles <= 2*b.s.Rules.Life()},
		{game.Duck, b.s.Ducks == 0},
		// Overclock while it leaves two fails of cushion where it costs
		// cycles (rules.OverclockCycles).
		{game.Overclock, b.s.Rules.OverclockCycles == 0 || b.s.Cycles-b.s.Rules.OverclockCycles >= 2*b.s.Rules.Life()},
	} {
		if want.ok && slices.Contains(offers, want.u) {
			return want.u
		}
	}
	// The roster's, by what the model's crashes need (timeouts at bosses):
	// clock and safety first, the cursed -Ofast and the skip last.
	for _, u := range []game.Upgrade{game.Turbo, game.Snapshot, game.Microcode, game.PrefetchPick,
		game.Heatsink, game.Smt, game.Fsanitize} {
		if slices.Contains(offers, u) {
			return u
		}
	}
	for _, u := range offers {
		if u != game.Ofast && u != game.Skip && (u != game.Overclock || b.s.Rules.OverclockCycles == 0) {
			return u
		}
	}
	if slices.Contains(offers, game.Skip) {
		return game.Skip
	}
	return offers[0]
}

// daemon is the model's pick of the window's daemons, false to take none
// (none on offer, or Player.DeclineDaemons).
func (b *boot) daemon() (game.Daemon, bool) {
	// A full window's offers are swaps (PickDaemon.Replace); the model
	// keeps what runs.
	if len(b.s.DaemonOffers) == 0 || len(b.s.Daemons) >= b.s.Rules.MaxDaemons {
		return "", false
	}
	// chrt last: most of the model's crashes are timeouts.
	// The roster's nets and engines first, its glass cannons never.
	for _, d := range []game.Daemon{game.Kdump, game.Watchdog, game.Fork, game.Kexec, game.Ksmd, game.Burst, game.Ccache,
		game.Tuned, game.DevRandom, game.Cron, game.Strace, game.Nice, game.Quiet, game.Chrt} {
		if b.p.DeclineDaemons && (d == game.Chrt || d == game.Nice && b.s.Rules.NiceLifeCost > 1) {
			continue
		}
		if slices.Contains(b.s.DaemonOffers, d) {
			return d, true
		}
	}
	if b.p.DeclineDaemons {
		return "", false
	}
	return b.s.DaemonOffers[0], true
}

type event struct {
	at time.Duration // after the door opened
	a  game.Action
}

// attempt plays the open door to its end. The model decides its whole
// timeline up front (capable or not, when it solves, which submits fail),
// then feeds it to the engine in time order until the door closes.
func (b *boot) attempt(biome int) {
	s := b.s
	if s.Phase != game.PhaseProblem {
		return
	}
	a := s.Current
	d, opened, timer := a.Door, a.Opened, a.Door.Timer
	pStar := b.solveP(d)
	defer b.taught(d, pStar)
	capable := b.rng.Float64() < pStar
	median := b.median(d)
	t := b.lognormal(median)

	// Stuck with a free swap: take it early.
	if !capable && slices.Contains(s.Daemons, game.Kexec) && !s.KexecUsed {
		at := timer / 4
		if lock := s.Rules.RetreatLock.Duration; lock > 0 {
			at = min(at, lock-30*time.Second) // before the swap locks
		}
		b.clock = opened.Add(at)
		b.step(game.Retreat{})
		return
	}
	var evs []event
	half := timer / 2
	// D64: the duck debugs a failed submit. A model stuck on the door fails
	// one first, at its price, as a player who reaches for it has; one only
	// slow has nothing to debug and no reason to fail one.
	duck := !capable || t > half && !s.Rules.DuckTest
	if duck && b.p.DuckBoost > 0 && s.Ducks > 0 && !d.Cursed {
		if s.Rules.DuckTest {
			evs = append(evs, event{half - time.Second, b.fail()})
		}
		evs = append(evs, event{half, game.Quack{}})
		if !capable {
			capable = b.rng.Float64() < b.p.DuckBoost
			t = half + b.lognormal(median/2)
		} else {
			t = half + time.Duration(float64(t-half)*0.6)
		}
	}
	if capable && t < timer {
		for b.rng.Float64() < b.p.WAProb {
			evs = append(evs, event{t/2 + time.Duration(b.rng.Float64()*float64(t/2)), b.fail()})
		}
		evs = append(evs, event{t, game.SubmitResult{Verdict: "AC", Passed: 20, Total: 20}})
	} else {
		for b.rng.Float64() < b.p.StuckWAs/(1+b.p.StuckWAs) {
			at := time.Duration((0.3 + 0.7*b.rng.Float64()) * float64(timer))
			evs = append(evs, event{min(at, timer-time.Second), b.fail()})
		}
		end := event{timer, game.Tick{}}
		if b.p.GiveUpAt > 0 {
			gc, gok := s.FailCost(game.GaveUp)
			tc, tok := s.FailCost(game.TimedOut)
			if gok && (!tok || gc <= tc) {
				end = event{time.Duration(b.p.GiveUpAt * float64(timer)), game.GiveUp{}}
			}
		}
		evs = append(evs, end)
	}
	slices.SortStableFunc(evs, func(x, y event) int { return cmp.Compare(x.at, y.at) })
	for _, ev := range evs {
		if s.Phase != game.PhaseProblem {
			break
		}
		b.clock = opened.Add(ev.at)
		b.step(ev.a)
	}
	// watchdog's overtime keeps the door open past its clock: a model that
	// solves it by the new deadline does, else the overtime runs out.
	if s.Phase == game.PhaseProblem && s.Current != nil && s.Current.Overtime && s.Rules.OnExpire != rules.ExpireOvertime {
		if at := opened.Add(t); capable && at.Before(s.Current.Deadline) && !at.Before(b.clock) {
			b.clock = at
			b.step(game.SubmitResult{Verdict: "AC", Passed: 20, Total: 20})
		} else {
			b.clock = maxTime(b.clock, s.Current.Deadline)
			b.step(game.Tick{})
		}
	}
	if s.Phase == game.PhaseProblem { // only if the engine missed the deadline
		b.sums.rep.Errors++
		b.clock = a.Deadline
		b.step(game.Tick{})
	}
	if len(s.History) > 0 && s.History[len(s.History)-1].Outcome == game.Cleared {
		b.sums.rep.Clears[biome]++
	}
}

func (b *boot) fail() game.SubmitResult {
	v := []game.Verdict{"WA", "WA", "WA", "TLE", "RE", "CE"}[b.rng.IntN(6)]
	return game.SubmitResult{Verdict: v, Passed: b.rng.IntN(20), Total: 20}
}

func (b *boot) lognormal(median float64) time.Duration {
	return time.Duration(median * math.Exp(b.p.Sigma*b.rng.NormFloat64()))
}

// SyntheticPool is n problems with ratings uniform in 600..2600 (rounded to
// 10) and clamped at 800, as Codeforces rates them (RatingSource "cf"):
// about a tenth sit at the floor, as in the real pool (687 of 6,445).
// Topics cycle through catalog.Topics, and every 20th problem is
// unjudgeable so filters get exercised.
func SyntheticPool(n int, seed uint64) []catalog.Problem {
	rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	ps := make([]catalog.Problem, n)
	for i := range ps {
		ps[i] = catalog.Problem{
			ID:           catalog.ID(fmt.Sprintf("sim:%04d", i)),
			Source:       "sim",
			Title:        fmt.Sprintf("Problem %d", i),
			Topic:        catalog.Topics[i%len(catalog.Topics)],
			Rating:       max(800, 600+10*rng.IntN(201)),
			RatingSource: "cf",
			Judgeable:    i%20 != 19,
		}
	}
	return ps
}

// String is a compact summary for test logs.
func (r Report) String() string {
	var w strings.Builder
	pct := func(a, b int) float64 {
		if b == 0 {
			return 0
		}
		return 100 * float64(a) / float64(b)
	}
	fmt.Fprintf(&w, "boots %d, full clears %d (%.1f%%), errors %d\n",
		r.Boots, r.FullClears, 100*r.ClearRate, r.Errors)
	fmt.Fprintf(&w, "crash biome:")
	for i, n := range r.CrashBiome {
		fmt.Fprintf(&w, " b%d=%d (%.1f%%)", i, n, pct(n, r.Boots))
	}
	fmt.Fprintf(&w, "\nhazard (crashed in | reached; timeouts/lives of those crashes):")
	reached := r.Boots
	for i, n := range r.CrashBiome {
		fmt.Fprintf(&w, " b%d=%.1f%% (%.0f/%.0f)", i, pct(n, reached), pct(r.CrashTimeouts[i], n), pct(r.CrashLives[i], n))
		reached -= n
	}
	fmt.Fprintf(&w, "\ncauses:")
	for _, o := range []game.Outcome{game.TimedOut, game.OutOfLives, game.GaveUp} {
		fmt.Fprintf(&w, " %s=%d", o, r.Causes[o])
	}
	if r.Boots > 0 {
		fmt.Fprintf(&w, "\neasing: floor %d, governor %d boots; %d timeouts saved; forecast %.1f%% crashes in b0, %.1f%% did",
			r.Eased[game.EaseFloor], r.Eased[game.EaseGovernor], r.Saved, 100*r.Forecast/float64(r.Boots), pct(r.CrashBiome[0], r.Boots))
	}
	fmt.Fprintf(&w, "\ndoor clear rate:")
	for i := range r.Attempts {
		fmt.Fprintf(&w, " b%d=%.1f%% (%d, timer %s, %.0f%% off target)", i, pct(r.Clears[i], r.Attempts[i]),
			r.Attempts[i], r.MeanTimer[i].Round(time.Minute), pct(r.OffTarget[i], r.Attempts[i]))
	}
	fmt.Fprintf(&w, "\nplain doors cleared vs expected:")
	for i := range r.Taught {
		exp := 0.0
		if r.Taught[i] > 0 {
			exp = 100 * r.TaughtP[i] / float64(r.Taught[i])
		}
		fmt.Fprintf(&w, " b%d=%.1f%% vs %.1f%% (%d)", i, pct(r.TaughtClears[i], r.Taught[i]), exp, r.Taught[i])
	}
	if r.Meta != nil && len(r.Meta.Calibration) > 0 {
		fmt.Fprintf(&w, "\ncalibration:")
		for _, k := range slices.Sorted(maps.Keys(r.Meta.Calibration)) {
			c := r.Meta.Calibration[k]
			fmt.Fprintf(&w, " [%s] %+.0f (%d doors, %.0f%% vs %.0f%%)", k, c.Offset, c.Doors, 100*c.Observed, 100*c.Expected)
		}
	}
	fmt.Fprintf(&w, "\nuptime: mean %s, clear %s, crash %s (median %s); wall: mean %s, clear %s\n", r.MeanUptime.Round(time.Minute),
		r.MeanClearUptime.Round(time.Minute), r.MeanCrashUptime.Round(time.Minute), r.MedianCrashUptime.Round(time.Minute),
		r.MeanWall.Round(time.Minute), r.MeanClearWall.Round(time.Minute))
	fmt.Fprintf(&w, "bytes/boot: earned %.0f, committed %.0f, spent %.0f, lost %.0f; recovered %d, dumps lost %d; %d shufs used\n",
		r.MeanEarned, r.MeanCommitted, r.MeanSpent, r.MeanLost, r.Recovered, r.DumpsLost, r.Rerolls)
	if len(r.InstalledAt) > 0 {
		fmt.Fprintf(&w, "packages installed after boots %v\n", r.InstalledAt)
	}
	fmt.Fprintf(&w, "prestige %d, unlocked at boots %v; by prestige:", r.Prestige, r.PrestigeAt)
	for i, t := range r.ByPrestige {
		if t.Boots > 0 {
			fmt.Fprintf(&w, " p%d=%d/%d (%.1f%%)", i, t.FullClears, t.Boots, pct(t.FullClears, t.Boots))
		}
	}
	fmt.Fprintf(&w, "\nengine rating estimate %.0f\n", r.FinalRating)
	return w.String()
}

// route is the model's pick of the route on offer: a topic at random, or on
// a map an open exit by Player.Route. A hidden exit (the Trial) only a
// model set out for it takes, at once. One open exit is no choice and
// draws nothing, as when the engine took it at once.
func (b *boot) route() int {
	bs := b.s.Biomes[b.s.Biome]
	if len(bs.Exits) == 0 {
		return b.rng.IntN(len(bs.Offers))
	}
	var open []int
	for i, x := range bs.Exits {
		if x.Locked != "" {
			continue
		}
		if x.Hidden && b.p.Trial {
			return i // the Trial, whenever it opens
		}
		if !x.Hidden {
			open = append(open, i)
		}
	}
	switch {
	case len(open) == 1:
		return open[0] // beside locked exits, or an open Trial the model does not take
	case b.p.Route == "":
		return open[b.rng.IntN(len(open))]
	}
	best, bi := math.Inf(-1), open[0]
	for _, i := range open {
		x := bs.Exits[i]
		v := 0.0
		switch b.p.Route {
		case "long":
			v = float64(x.Rooms)
		case "short":
			v = -float64(x.Rooms)
		case "gated":
			v = float64(x.Picks)
		}
		if v += b.rng.Float64() * 0.5; v > best {
			best, bi = v, i
		}
	}
	return bi
}

// mapped sums what the map report reads of a played boot (map.md §(i)).
func (sm *sums) mapped(s *game.State) {
	rep := &sm.rep
	var inDoors time.Duration
	for j, h := range s.History {
		inDoors += h.Closed.Sub(h.Opened)
		ended := s.Crash != nil && j == len(s.History)-1
		if h.Door.Boss != "" && h.Outcome != game.Cleared && h.Outcome != game.Retreated && !ended {
			rep.BossSaves++
			t := rep.Bosses[h.Door.Boss]
			t.Retries++
			rep.Bosses[h.Door.Boss] = t
		}
	}
	rep.DoorTime = append(rep.DoorTime, inDoors)
	if s.Phase == game.PhaseCleared && s.Ending == "" {
		rep.DoorTimeClear = append(rep.DoorTimeClear, inDoors)
	}
	rep.Fountained += s.Fountained
	for i, bs := range s.Biomes {
		if bs.Node != "" {
			rep.Nodes[bs.Node]++
		}
		if bs.Boss != "" && i <= s.Biome {
			t := rep.Bosses[bs.Boss]
			t.Reached++
			if s.Phase == game.PhaseCrashed && i == s.Biome {
				t.Crashed++
			}
			rep.Bosses[bs.Boss] = t
		}
	}
}

// recentClear is the clear share of the last 10 outcomes, padded with
// clears while there are fewer: a level is worth trying until it is known.
func recentClear(xs []bool) float64 {
	n, c := 0, 0
	for i := len(xs) - 1; i >= 0 && n < 10; i-- {
		n++
		if xs[i] {
			c++
		}
	}
	return float64(c+10-n) / 10
}

// trial sums what the Trial's check reads of a played boot: whether its
// Boss II tier met the Trial's exit open, and took it.
func (sm *sums) trial(s *game.State) {
	if s.Capped {
		sm.rep.Capped++
	}
	if rules.TierBossII >= len(s.Biomes) || len(s.Rules.Nodes) == 0 {
		return
	}
	b := s.Biomes[rules.TierBossII]
	if slices.ContainsFunc(b.Exits, func(x game.Exit) bool { return x.Ending != "" && x.Locked == "" }) {
		sm.rep.TrialOpen++
	}
	if b.Node == rules.TrialNode {
		sm.rep.TrialTaken++
	}
}

// hoard reports whether the model, set out for the Trial, keeps its bytes
// uncommitted in this merge window: every one before act II's, so the one
// before Boss II can buy fsck to the cap.
func (b *boot) hoard() bool {
	return b.p.Trial && len(b.s.Rules.Nodes) > 0 && b.s.Biome < rules.TierActII
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
