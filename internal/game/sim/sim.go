// Package sim plays boots headlessly with a model player. It exists to
// balance rules presets (clear rates, crash spread, uptime, bytes) and to
// drive the engine's property tests through thousands of boots.
//
// The model player is a deliberately simple stand-in, not a prediction: it
// solves a door with the logistic chance of its true rating, takes a
// lognormal time to do it, sometimes submits wrong answers, spends ducks when
// stuck, and picks the door with the best bytes×P. The engine never sees the
// true rating; it has to learn it from outcomes like it would a person.
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
}

// DefaultPlayer is the model the reports in this package's tests use.
func DefaultPlayer(rating float64) Player {
	return Player{
		Rating:      rating,
		Median:      30 * time.Minute,
		Sigma:       0.5,
		GapScale:    200,
		WAProb:      0.2,
		StuckWAs:    1,
		DuckBoost:   0.35,
		BlindCost:   25,
		UpsolveProb: 0.6,
		Overhead:    time.Minute,
	}
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
	// Block is how many boots each entry of Report.Blocks sums, warm-up
	// included; 0 leaves Blocks empty.
	Block int
}

// Block sums a stretch of boots, to show the cold start, the calibration
// settling and the pool running dry over a run.
type Block struct {
	Boots, FullClears int
	Opened, OffTarget int                // doors opened, and how many off target
	Rating            float64            // the engine's rating at the end
	Offsets           map[string]float64 // calibration offsets at the end
}

// Tally counts boots and full clears at one prestige level.
type Tally struct{ Boots, FullClears int }

// Report sums a simulation. Per-biome slices are indexed like Rules.Biomes.
type Report struct {
	Boots, FullClears int
	ClearRate         float64

	CrashBiome []int // crashes per biome
	Causes     map[game.Outcome]int
	Attempts   []int // doors opened per biome
	Clears     []int // doors cleared per biome
	MeanTimer  []time.Duration
	// OffTarget counts opened doors more than the first rating window from
	// their target (regressions aside): where the pool had nothing near.
	OffTarget []int
	// Taught counts opened doors that teach the aim calibration (plain and
	// aimed, Door.Tier set), TaughtClears how many of them were cleared and
	// TaughtP the sum of their SolveP, so TaughtClears/Taught against
	// TaughtP/Taught is the calibration's observed against expected.
	Taught, TaughtClears []int
	TaughtP              []float64

	MeanUptime, MeanClearUptime, MeanCrashUptime time.Duration

	MeanEarned, MeanCommitted, MeanLost float64 // bytes per boot
	Recovered                           int64   // core-dump bytes upsolved back
	DumpsLost                           int64   // core-dump bytes crashed away

	Prestige   int     // highest unlocked at the end
	PrestigeAt []int   // boot that unlocked level i+1, 0 if never
	ByPrestige []Tally // index: prestige level

	FinalRating float64 // the engine's estimate of the player at the end
	Errors      int     // engine calls the model made that failed
	Meta        *game.Meta

	// Ratings is the engine's rating after each boot, warm-up included.
	Ratings []float64
	Blocks  []Block
}

// Simulate runs boots with DefaultPlayer(playerRating), climbing prestige.
func Simulate(r rules.Rules, pool []catalog.Problem, playerRating float64, boots int, seed uint64) Report {
	return Run(Config{Rules: r, Pool: pool, Player: DefaultPlayer(playerRating), Boots: boots, Seed: seed, Prestige: -1})
}

// Run is Simulate with every knob.
func Run(c Config) Report {
	nb := len(c.Rules.Biomes)
	m := c.Meta
	if m == nil {
		m = &game.Meta{}
		for _, cap := range game.TopicCapabilities {
			game.GrantCapability(m, cap)
		}
		slices.Sort(m.Capabilities)
	}
	if c.Prestige >= 0 {
		m.Prestige = max(m.Prestige, min(c.Prestige, game.MaxPrestige))
	}
	rng := rand.New(rand.NewPCG(c.Seed, c.Seed^0x5deece66d))
	clock := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

	warm, main := newSums(nb), newSums(nb)
	var ratings []float64
	var blocks []Block
	for i := range c.Warmup + c.Boots {
		sm := main
		if i < c.Warmup {
			sm = warm
		}
		rep := &sm.rep
		p := m.Prestige
		if c.Prestige >= 0 {
			p = min(c.Prestige, m.Prestige)
		}
		s, fx, err := game.NewBoot(c.Rules, m, c.Pool, p, rng.Uint64(), clock)
		if c.Check != nil {
			c.Check(s, m, nil, fx, err)
		}
		if err != nil {
			main.rep.Errors++
			break
		}
		b := &boot{c: &c, p: c.Player, s: s, m: m, rng: rng, clock: clock, sums: sm}
		b.play()

		rep.Boots++
		rep.ByPrestige[p].Boots++
		sm.earned += s.Earned()
		sm.committed += s.Committed
		switch s.Phase {
		case game.PhaseCleared:
			rep.FullClears++
			rep.ByPrestige[p].FullClears++
			sm.upClear += b.clock.Sub(s.Started)
		case game.PhaseCrashed:
			rep.CrashBiome[s.Biome]++
			rep.Causes[s.Crash.Cause]++
			sm.upCrash += s.Crash.Uptime
			sm.lost += s.Crash.Lost
			rep.DumpsLost += s.Crash.DumpLost
			if d := m.CoreDump; d != nil && rng.Float64() < c.Player.UpsolveProb {
				rep.Recovered += game.Upsolve(m, d.Problem, game.Editorial, 45*time.Minute, b.clock.Add(time.Hour))
			}
		}
		for lvl := 1; lvl <= m.Prestige; lvl++ {
			if main.rep.PrestigeAt[lvl-1] == 0 {
				main.rep.PrestigeAt[lvl-1] = i + 1
			}
		}
		clock = clock.Add(24 * time.Hour) // a boot a day, so regressions come due
		ratings = append(ratings, m.Rating)
		if c.Block > 0 {
			if i%c.Block == 0 {
				blocks = append(blocks, Block{})
			}
			bl := &blocks[len(blocks)-1]
			bl.Boots++
			if s.Phase == game.PhaseCleared {
				bl.FullClears++
			}
			bl.Opened += b.opened
			bl.OffTarget += b.off
			bl.Rating = m.Rating
			bl.Offsets = map[string]float64{}
			for k, v := range m.Calibration {
				bl.Offsets[k] = v.Offset
			}
		}
	}
	main.rep.Errors += warm.rep.Errors

	rep := main.finish()
	rep.Prestige = m.Prestige
	rep.FinalRating = m.Rating
	rep.Meta = m
	rep.Ratings = ratings
	rep.Blocks = blocks
	return rep
}

// sums accumulates one report.
type sums struct {
	rep                     Report
	timerSum                []time.Duration
	upClear, upCrash        time.Duration
	earned, committed, lost int64
}

func newSums(nb int) *sums {
	return &sums{
		rep: Report{
			CrashBiome:   make([]int, nb),
			Causes:       map[game.Outcome]int{},
			Attempts:     make([]int, nb),
			Clears:       make([]int, nb),
			MeanTimer:    make([]time.Duration, nb),
			OffTarget:    make([]int, nb),
			Taught:       make([]int, nb),
			TaughtClears: make([]int, nb),
			TaughtP:      make([]float64, nb),
			PrestigeAt:   make([]int, game.MaxPrestige),
			ByPrestige:   make([]Tally, game.MaxPrestige+1),
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
		rep.MeanUptime = (sm.upClear + sm.upCrash) / time.Duration(n)
	}
	if rep.FullClears > 0 {
		rep.MeanClearUptime = sm.upClear / time.Duration(rep.FullClears)
	}
	if n := rep.Boots - rep.FullClears; n > 0 {
		rep.MeanCrashUptime = sm.upCrash / time.Duration(n)
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
		switch {
		case s.PendingUpgrade:
			b.step(game.PickUpgrade{Upgrade: b.upgrade()})
		case s.Phase == game.PhaseRoute:
			b.step(game.ChooseRoute{Index: b.rng.IntN(len(s.Biomes[s.Biome].Offers))})
		case s.Phase == game.PhaseDoors:
			i := b.door()
			d := s.Doors[i]
			b.sums.rep.Attempts[s.Biome]++
			b.sums.timerSum[s.Biome] += d.Timer
			b.opened++
			if !d.Regression && math.Abs(float64(d.Rating)-s.Target(b.m, d.Topic)) > float64(s.Rules.RatingWindows[0]) {
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
			switch {
			case s.Uncommitted > 0:
				b.step(game.Commit{})
			case len(s.DaemonOffers) > 0:
				b.step(game.PickDaemon{Daemon: b.daemon()})
			default:
				b.step(game.LeaveMerge{})
			}
		default:
			b.sums.rep.Errors++
			return
		}
	}
}

// solveP is the model's own chance on d, which the engine only estimates.
func (b *boot) solveP(d game.Door) float64 {
	eff := b.p.Rating
	if d.Blind {
		eff -= b.p.BlindCost
	}
	if d.Mystery {
		eff -= b.p.BlindCost
	}
	return game.Expected(eff, float64(d.Rating))
}

// clearP is the model's own chance of clearing d: solving it at all, within
// the timer, and (cursed) without a single wrong submit.
func (b *boot) clearP(d game.Door) float64 {
	median := float64(b.p.Median) * math.Exp2((float64(d.Rating)-b.p.Rating)/b.p.GapScale)
	inTime := 0.5 * (1 + math.Erf(math.Log(float64(d.Timer)/median)/(b.p.Sigma*math.Sqrt2)))
	p := b.solveP(d) * inTime
	if d.Cursed {
		p *= 1 - b.p.WAProb
	}
	return p
}

// door is the best bytes×P on offer; the first wins ties.
func (b *boot) door() int {
	best, bi := -1.0, 0
	for i, d := range b.s.Doors {
		if v := float64(d.Bytes) * b.clearP(d); v > best {
			best, bi = v, i
		}
	}
	return bi
}

func (b *boot) upgrade() game.Upgrade {
	offers := b.s.UpgradeOffers
	for _, want := range []struct {
		u  game.Upgrade
		ok bool
	}{
		{game.ECC, b.s.Lives <= 2},
		{game.Duck, b.s.Ducks == 0},
		{game.Overclock, true},
	} {
		if want.ok && slices.Contains(offers, want.u) {
			return want.u
		}
	}
	return offers[0]
}

func (b *boot) daemon() game.Daemon {
	for _, d := range []game.Daemon{game.Fork, game.Kexec, game.DevRandom, game.Strace, game.Nice} {
		if slices.Contains(b.s.DaemonOffers, d) {
			return d
		}
	}
	return b.s.DaemonOffers[0]
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
	capable := b.rng.Float64() < b.solveP(d)
	median := float64(b.p.Median) * math.Exp2((float64(d.Rating)-b.p.Rating)/b.p.GapScale)
	t := b.lognormal(median)

	// Stuck with a free swap: take it early.
	if !capable && slices.Contains(s.Daemons, game.Kexec) && !s.KexecUsed {
		b.clock = opened.Add(timer / 4)
		b.step(game.Retreat{})
		return
	}
	var evs []event
	half := timer / 2
	if (!capable || t > half) && b.p.DuckBoost > 0 && s.Ducks > 0 && !d.Cursed {
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
		evs = append(evs, event{timer, game.Tick{}})
	}
	slices.SortStableFunc(evs, func(x, y event) int { return cmp.Compare(x.at, y.at) })
	for _, ev := range evs {
		if s.Phase != game.PhaseProblem {
			break
		}
		b.clock = opened.Add(ev.at)
		b.step(ev.a)
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
	v := []string{"WA", "WA", "WA", "TLE", "RE", "CE"}[b.rng.IntN(6)]
	return game.SubmitResult{Verdict: v, Passed: b.rng.IntN(20), Total: 20}
}

func (b *boot) lognormal(median float64) time.Duration {
	return time.Duration(median * math.Exp(b.p.Sigma*b.rng.NormFloat64()))
}

// SyntheticPool is n problems with ratings uniform in 800..2600 (rounded to
// 10), topics cycling through catalog.Topics, and every 20th problem
// unjudgeable so filters get exercised.
func SyntheticPool(n int, seed uint64) []catalog.Problem {
	rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	ps := make([]catalog.Problem, n)
	for i := range ps {
		ps[i] = catalog.Problem{
			ID:        catalog.ID(fmt.Sprintf("sim:%04d", i)),
			Source:    "sim",
			Title:     fmt.Sprintf("Problem %d", i),
			Topic:     catalog.Topics[i%len(catalog.Topics)],
			Rating:    800 + 10*rng.IntN(181),
			Judgeable: i%20 != 19,
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
	fmt.Fprintf(&w, "\ncauses:")
	for _, o := range []game.Outcome{game.TimedOut, game.OutOfLives, game.GaveUp} {
		fmt.Fprintf(&w, " %s=%d", o, r.Causes[o])
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
	fmt.Fprintf(&w, "\nuptime: mean %s, clear %s, crash %s\n", r.MeanUptime.Round(time.Minute),
		r.MeanClearUptime.Round(time.Minute), r.MeanCrashUptime.Round(time.Minute))
	fmt.Fprintf(&w, "bytes/boot: earned %.0f, committed %.0f, lost %.0f; recovered %d, dumps lost %d\n",
		r.MeanEarned, r.MeanCommitted, r.MeanLost, r.Recovered, r.DumpsLost)
	fmt.Fprintf(&w, "prestige %d, unlocked at boots %v; by prestige:", r.Prestige, r.PrestigeAt)
	for i, t := range r.ByPrestige {
		if t.Boots > 0 {
			fmt.Fprintf(&w, " p%d=%d/%d (%.1f%%)", i, t.FullClears, t.Boots, pct(t.FullClears, t.Boots))
		}
	}
	fmt.Fprintf(&w, "\nengine rating estimate %.0f\n", r.FinalRating)
	return w.String()
}
