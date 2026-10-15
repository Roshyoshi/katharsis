package sim

import (
	"hash/fnv"
	"math"
	"math/rand/v2"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
)

// Learning makes the model player learn, take breaks and quit
// (ml-pipeline §5.2): the simulator a door policy is ranked on, since one
// real player cannot measure learning differences in useful time. Every
// number is assumed and tunable; a policy ranking is only trusted when it
// holds across Shape and Gain.
//
// With Gain 0, Sessions false and Churn nil the learner changes nothing:
// the run is today's model player's, draw for draw (TestLearnOffIsToday).
// Its own draws come from a stream of their own, so turning one part on
// never shifts the others' or the engine's.
type Learning struct {
	// Gain is γ, rating points a door teaches at G = F = 1 (1, 3 or 6).
	Gain float64
	// Shape is G, how a door's true clear chance p shapes its gain:
	// "flat" (1), "inverted-u" (a bump at 0.75, sd 0.15) or "harder"
	// (1.6 − p); each has mean 1 over p in [0, 1].
	Shape string
	// Transfer is ρ, the share of a topic door's gain every topic gets.
	Transfer float64
	// FastHalf and SlowHalf are the half-lives of the two parts of a
	// gain; a gain consolidates (goes to the slow part) by
	// ζ(gap) = 0.3 + 0.7·(1 − e^(−gap/Consolidate)), gap the time since the
	// topic's last door: spaced practice keeps more.
	FastHalf, SlowHalf, Consolidate time.Duration
	// BlindBonus multiplies a clean clear's gain when the tags were hidden.
	BlindBonus float64

	// Recall is a reviewed problem's true stability: a regression door is
	// recalled with R = (1 + 19/81·t/S)^(−1/2) (FSRS-4.5), S starting at
	// RecallS0 and growing by RecallW8 on each successful review.
	RecallS0 time.Duration
	RecallW8 float64

	// Sessions spaces boots like a person: another boot the same day with
	// SameDayClear (after a clear) or SameDayCrash, else a gap of 1.2 days
	// (0.7), 4 days (0.25) or 21 days (0.05), exponential. False is a boot
	// a day.
	Sessions                   bool
	SameDayClear, SameDayCrash float64

	// Churn, when set, may end a boot with a break or for good.
	Churn *Churn
}

// Churn is the hazard of not coming back after a boot, a logistic of how
// the recent boots ended (ml-pipeline §5.2): A0 the base, A1 a crash in the
// first biome, A2 two of those in a row, A3 three crashes in a row (EOMM:
// LLL churned 5.1% against LLW's 2.6%, +0.70), A4 three full clears in a
// row, A5 per unit of ln(uptime in hours). Who churns takes a break of
// Break on average (exponential), or quits for good with QuitShare.
type Churn struct {
	A0, A1, A2, A3, A4, A5 float64
	Break                  time.Duration
	QuitShare              float64
}

// DefaultLearning is §5.2's player with γ = 3 on the flat shape, sessions
// and churn on.
func DefaultLearning() *Learning {
	return &Learning{
		Gain: 3, Shape: "flat", Transfer: 0.3,
		FastHalf: 21 * 24 * time.Hour, SlowHalf: 365 * 24 * time.Hour, Consolidate: 3 * 24 * time.Hour,
		BlindBonus: 1.2,
		RecallS0:   3 * 24 * time.Hour, RecallW8: 2.3,
		Sessions: true, SameDayClear: 0.3, SameDayCrash: 0.15,
		Churn: &Churn{A0: math.Log(0.026 / 0.974), A3: 0.7, Break: 30 * 24 * time.Hour, QuitShare: 0.3},
	}
}

// part is one decaying part of a skill: its value at at.
type part struct {
	v  float64
	at time.Time
}

func (p part) value(now time.Time, half time.Duration) float64 {
	if p.v == 0 || half <= 0 {
		return p.v
	}
	return p.v * math.Exp2(-float64(now.Sub(p.at))/float64(half))
}

// learner is one run's true skill: a fast and a slow part per topic, and
// for the general level that untagged (Mixed) doors are played at, which
// Transfer of every topic gain feeds (θ*_g in §5.2).
type learner struct {
	l          *Learning
	base       float64
	fast, slow map[string]part
	last       map[string]time.Time // the topic's last door
	seen       map[string]time.Time // a problem's last door
	stability  map[string]time.Duration
	rng        *rand.Rand
}

func newLearner(l *Learning, base float64, seed uint64) *learner {
	return &learner{
		l: l, base: base, fast: map[string]part{}, slow: map[string]part{},
		last: map[string]time.Time{}, seen: map[string]time.Time{}, stability: map[string]time.Duration{},
		rng: rand.New(rand.NewPCG(seed, seed^0x2545f4914f6cdd1d)),
	}
}

// general is the key of the part every topic shares.
const general = ""

// gained is what topic's doors have taught by now; Mixed is the general
// level.
func (lr *learner) gained(topic string, now time.Time) float64 {
	if topic == game.Mixed {
		topic = general
	}
	return lr.fast[topic].value(now, lr.l.FastHalf) + lr.slow[topic].value(now, lr.l.SlowHalf)
}

func (lr *learner) skill(topic string, now time.Time) float64 { return lr.base + lr.gained(topic, now) }

// shape is G(p).
func (lr *learner) shape(p float64) float64 {
	switch lr.l.Shape {
	case "inverted-u":
		// exp(−(p − 0.75)²/(2·0.15²)) has mean 0.358 over [0, 1].
		return math.Exp(-(p-0.75)*(p-0.75)/(2*0.15*0.15)) / 0.358
	case "harder":
		return (1.6 - p) / 1.1
	}
	return 1
}

// learn adds what a closed door taught: γ·G(p)·f, where p is its true
// clear chance and f what the outcome makes of it (F in §5.2).
func (lr *learner) learn(topic string, p, f float64, now time.Time) {
	if lr.l.Gain == 0 || f == 0 {
		return
	}
	g := lr.l.Gain * lr.shape(p) * f
	add := func(key string, g float64) {
		gap := 1e9 // never practised: fully spaced
		if t, ok := lr.last[key]; ok {
			gap = float64(now.Sub(t)) / float64(lr.l.Consolidate)
		}
		zeta := 0.3 + 0.7*(1-math.Exp(-gap))
		fp, sp := lr.fast[key], lr.slow[key]
		lr.fast[key] = part{fp.value(now, lr.l.FastHalf) + (1-zeta)*g, now}
		lr.slow[key] = part{sp.value(now, lr.l.SlowHalf) + zeta*g, now}
		lr.last[key] = now
	}
	if topic == game.Mixed {
		add(general, g)
		return
	}
	add(topic, g)
	add(general, lr.l.Transfer*g)
}

// recall is the chance the model remembers how it solved a regression
// door's problem: FSRS's forgetting curve at its stability, from the
// time it was last seen.
func (lr *learner) recall(id string, since time.Duration) (r float64, s time.Duration) {
	s, ok := lr.stability[id]
	if !ok {
		s = lr.l.RecallS0
	}
	if s <= 0 {
		return 0, s
	}
	return math.Pow(1+19.0/81*float64(since)/float64(s), -0.5), s
}

// reviewed grows a problem's stability after a successful review at
// recall r (FSRS-4.5 with w9 0.137, w10 1.05).
func (lr *learner) reviewed(id string, r float64, s time.Duration) {
	sd := float64(s) / float64(24*time.Hour)
	sd *= 1 + math.Exp(lr.l.RecallW8)*math.Pow(sd, -0.137)*(math.Exp(1.05*(1-r))-1)
	lr.stability[id] = time.Duration(sd * float64(24*time.Hour))
}

// gap is the time to the next boot after one that cleared or not.
func (lr *learner) gap(cleared bool) time.Duration {
	day := 24 * time.Hour
	if !lr.l.Sessions {
		return day
	}
	same := lr.l.SameDayCrash
	if cleared {
		same = lr.l.SameDayClear
	}
	if lr.rng.Float64() < same {
		return time.Hour
	}
	mean := 1.2
	switch u := lr.rng.Float64(); {
	case u >= 0.95:
		mean = 21
	case u >= 0.7:
		mean = 4
	}
	return time.Duration(lr.rng.ExpFloat64() * mean * float64(day))
}

// outcomeF is F in §5.2: what a closed door's outcome makes of its gain.
// upsolved is how the crash door was upsolved after the boot: "" none,
// "own" or "editorial".
func (lr *learner) outcomeF(a game.Attempt, upsolved string) float64 {
	switch {
	case a.Outcome == game.Cleared && a.Quacks == 0:
		if a.Door.TagsHidden() {
			return lr.l.BlindBonus
		}
		return 1
	case a.Outcome == game.Cleared:
		return 0.6
	case upsolved == "own":
		return 0.8
	case upsolved == "editorial":
		return 0.5
	}
	return 0.1 // no feedback, no gain (Rowland 2014)
}

// hashNormal is a standard normal draw fixed by key: an item's noise is
// the same every time its problem comes up, in every run.
func hashNormal(key string) float64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	x := h.Sum64()
	u1 := (float64(x>>11) + 0.5) / (1 << 53)
	h.Reset()
	_, _ = h.Write([]byte(key + "#2"))
	u2 := (float64(h.Sum64()>>11) + 0.5) / (1 << 53)
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}

// hazard is the chance of churning after a boot, given the endings.
func (c *Churn) hazard(earlyCrash bool, earlyRun, crashRun, clearRun int, uptime time.Duration) float64 {
	x := c.A0
	if earlyCrash {
		x += c.A1
	}
	if earlyRun >= 2 {
		x += c.A2
	}
	if crashRun >= 3 {
		x += c.A3
	}
	if clearRun >= 3 {
		x += c.A4
	}
	if h := uptime.Hours(); h > 0 {
		x += c.A5 * math.Log(h)
	}
	return 1 / (1 + math.Exp(-x))
}

// taught learns from the door just closed, whose true chance was p: a
// clear or a failure the boot went on after at once, the boot's crash
// once its upsolve is known (Run). Without a learner it does nothing.
func (b *boot) taught(d game.Door, p float64) {
	lr := b.learn
	h := b.s.History
	if lr == nil || len(h) == 0 || h[len(h)-1].Door.Problem != d.Problem {
		return
	}
	a := h[len(h)-1]
	id := string(d.Problem)
	if d.Regression && a.Outcome == game.Cleared {
		if seen, ok := lr.seen[id]; ok {
			r, s := lr.recall(id, a.Closed.Sub(seen))
			lr.reviewed(id, r, s)
		}
	}
	lr.seen[id] = a.Closed
	if b.s.Phase == game.PhaseCrashed {
		b.crashed = &pending{a, p}
		return
	}
	lr.learn(d.Topic, p, lr.outcomeF(a, ""), a.Closed)
}

// LearnStats sums a learning player's run (Report.Learn).
type LearnStats struct {
	Days  float64 // simulated, to the last boot's end
	Hours float64 // of boots played
	Quit  bool    // churned for good
	// Breaks is how many times it churned and came back.
	Breaks int
	// Gain is the mean true skill gained over the topics at the end, and
	// Durable the same after 30 more days without practice: durable skill,
	// what the policy is ranked on.
	Gain, Durable float64
	// RMSE is the engine's rating against the model's true general level,
	// over the boots after the warm-up; ECE is the expected calibration
	// error of the plain doors' SolveP over the same boots (10 bins).
	RMSE, ECE float64
	// LLL counts boots ending a run of three crashes and LLW boots ending
	// crash, crash, clear, with how many of each churned after: EOMM saw
	// 5.1% against 2.6%.
	LLL, LLLChurned, LLW, LLWChurned int
}

// tally is a learning run's sums as it goes.
type tally struct {
	sq   float64
	n    int
	bins [10]struct {
		n, y int
		p    float64
	}
	endings []bool // true: cleared
	early   int
	crashes int
	clears  int
}

func (t *tally) ece() float64 {
	total, sum := 0, 0.0
	for _, b := range t.bins {
		total += b.n
	}
	for _, b := range t.bins {
		if b.n > 0 {
			sum += float64(b.n) / float64(total) * math.Abs(float64(b.y)/float64(b.n)-b.p/float64(b.n))
		}
	}
	return sum
}

// observe adds a finished boot's plain doors to the calibration bins and
// its rating error.
func (t *tally) observe(s *game.State, m *game.Meta, lr *learner, end time.Time) {
	d := m.Rating - lr.skill(game.Mixed, end)
	t.sq += d * d
	t.n++
	for _, h := range s.History {
		if h.Door.Tier == "" || h.Outcome == game.Retreated {
			continue
		}
		i := min(int(h.Door.SolveP*10), 9)
		t.bins[i].n++
		t.bins[i].p += h.Door.SolveP
		if h.Outcome == game.Cleared {
			t.bins[i].y++
		}
	}
}

// ended records how a boot ended and returns the churn hazard's inputs.
func (t *tally) ended(cleared, early bool) (earlyRun, crashRun, clearRun int) {
	t.endings = append(t.endings, cleared)
	switch {
	case cleared:
		t.clears++
		t.crashes, t.early = 0, 0
	default:
		t.crashes++
		t.clears = 0
		if early {
			t.early++
		} else {
			t.early = 0
		}
	}
	return t.early, t.crashes, t.clears
}

// pattern is "LLL" when the last three boots crashed, "LLW" for crash,
// crash, clear, "" otherwise.
func (t *tally) pattern() string {
	n := len(t.endings)
	if n < 3 {
		return ""
	}
	a, b, c := t.endings[n-3], t.endings[n-2], t.endings[n-1]
	switch {
	case !a && !b && !c:
		return "LLL"
	case !a && !b && c:
		return "LLW"
	}
	return ""
}

// durable is the mean skill gained over topics at t.
func (lr *learner) durable(t time.Time) float64 {
	sum, n := 0.0, 0
	for _, k := range catalogTopics() {
		sum += lr.gained(k, t)
		n++
	}
	return sum / float64(n)
}

// catalogTopics are the topics skill is kept for (Mixed aside).
func catalogTopics() []string {
	var ts []string
	for _, t := range catalog.Topics {
		if t != catalog.Mixed {
			ts = append(ts, t)
		}
	}
	return ts
}
