package game

import (
	"fmt"
	"math"
	"slices"

	"katharsis/internal/catalog"
)

// Easing (retention RT0 and RT8, ml-pipeline M4 and slice ML7). The first
// biome is where a boot is lost before it starts: guardrail G1 holds the
// chance of crashing there, before the first merge window, at 0.2. Two
// things break it:
//
//   - the content floor: no local problem is rated under 800, so a player
//     the engine rates near 1000 has no door near Userland's aim (576 for
//     0.92). The pool's floor doors then get longer timers (the buy-back),
//     and when the engine still forecasts more than rules.EaseAbove crashes
//     there, the boot gets a life (D34) and a timeout in that biome costs a
//     life instead of the boot (ml-pipeline decision (e) applied to it).
//   - a run of early crashes: after rules.GovernorAfter boots in a row
//     ended in the first biome, the next aims every biome at its EaseClear
//     (the streak governor, neuro P1.3).
//
// Each easing is a State.Ease the screens announce, and DeclineEase turns
// them all off before the first door opens (GR9). None of it reads how
// often the player plays: easing answers outcomes, never engagement
// (GR7).

// planAim finds the easings that change the aim, before NewBoot lays the
// boot out (route offers are checked against the aim).
func (e *engine) planAim() {
	s, r, m := e.s, &e.s.Rules, e.meta
	first := r.Biomes[0].Name
	if first == "" {
		first = "the first biome"
	}
	if n := m.Governor.EarlyCrashes; r.GovernorAfter > 0 && n >= r.GovernorAfter {
		clear := make([]float64, len(r.Biomes))
		for i, b := range r.Biomes {
			clear[i] = b.EaseClear
		}
		if !slices.ContainsFunc(clear, func(c float64) bool { return c > 0 }) {
			clear = nil
		}
		if clear != nil || r.GovernorLives > 0 {
			s.Ease = append(s.Ease, Ease{
				Kind:  EaseGovernor,
				Why:   fmt.Sprintf("the last %d boots ended in %s", n, first),
				Clear: clear,
				Lives: r.GovernorLives,
			})
		}
	}
}

// planFloor finds the floor easing once the layout is known, before the
// first room is drawn, and deals the lives every easing adds.
func (e *engine) planFloor() {
	s, r := e.s, &e.s.Rules
	first := e.biomeName(0)
	f := e.forecast()
	s.Forecast = f.crash
	if f.floor {
		es := Ease{Kind: EaseFloor, Buyback: len(r.BuybackTimers) > 0}
		if r.EaseAbove > 0 && f.crash > r.EaseAbove {
			// D34's life, and one more for each further EaseAbove of
			// forecast crashes, up to EaseLives.
			es.Lives = min(r.EaseLives, int(math.Ceil(f.crash/r.EaseAbove))-1)
			es.Ducks = r.EaseDucks
		}
		if es.Buyback || es.Lives > 0 || es.Ducks > 0 {
			es.Why = fmt.Sprintf("%s's easiest doors in the pool are rated %d", first, f.low)
			s.Ease = append(s.Ease, es)
		}
	}
	for _, es := range s.Ease {
		s.Lives += es.Lives
		s.Ducks += es.Ducks
		s.MaxLives = max(s.MaxLives, s.Lives)
		e.logf("ease: %s (%s)%s", es.Kind, es.Why, easeTerms(es))
	}
	if f.floor || len(s.Ease) > 0 {
		e.logf("ease: forecast %.0f%% crashes in %s (doors %.0f%% each)", 100*f.crash, first, 100*f.door)
	}
}

// easeTerms lists what es changes, for dmesg.
func easeTerms(es Ease) string {
	var t string
	if es.Lives > 0 {
		t += fmt.Sprintf(", +%d lives", es.Lives)
	}
	if es.Ducks > 0 {
		t += fmt.Sprintf(", +%d ducks", es.Ducks)
	}
	if es.Buyback {
		t += ", longer timers on floor doors"
	}
	for i, c := range es.Clear {
		if c > 0 {
			t += fmt.Sprintf(", biome_%d aims %.0f%%", i, 100*c)
		}
	}
	return t
}

func (e *engine) biomeName(i int) string {
	if n := e.s.Biomes[i].Name; n != "" {
		return n
	}
	return "the first biome"
}

// active is the boot's easing of kind k, nil when it has none or the
// player declined it.
func (s *State) active(k EaseKind) *Ease {
	if s.Declined {
		return nil
	}
	for i := range s.Ease {
		if s.Ease[i].Kind == k {
			return &s.Ease[i]
		}
	}
	return nil
}

// AimClear is the clear chance biome i's doors are aimed at: its
// TargetClear, or what an easing raised it to.
func (s *State) AimClear(i int) float64 {
	c := s.Rules.Biomes[i].TargetClear
	if s.Declined {
		return c
	}
	for _, es := range s.Ease {
		if i < len(es.Clear) {
			c = max(c, es.Clear[i])
		}
	}
	return c
}

// floorCast is the first biome's forecast (forecast).
type floorCast struct {
	crash float64 // P(crash in the first biome) as aimed
	door  float64 // what the engine expects of one of its doors (SolveP)
	low   int     // the lowest rating the pool has for it
	floor bool    // door is short of the aim by more than the margin
}

// forecast is the engine's own estimate of the first biome: each of its
// rooms is cleared with the chance the engine expects of the doors it will
// offer there (offered), the worst topic of a choice biome's. The doors
// fall short of the aim when the pool's floor cuts the rating window off
// below the target: the window keeps only its harder half.
//
// It counts no lives (a timeout, the common way to fail a door, ends the
// boot whatever the lives) and none of the buy-back's time: what a longer
// timer is worth (BuybackBeta) is a prior no player has confirmed, and in
// the simulator the boots it forecast under the threshold crashed twice as
// often as it said, with the lives and the save turned off on its word.
func (e *engine) forecast() floorCast {
	s := e.s
	b := s.Biomes[0]
	topics := []string{b.Topic}
	if b.Topic == "" { // a choice biome: any topic it offers
		topics = b.Offers
	}
	f := floorCast{door: 1}
	ex := e.excluded()
	for _, t := range topics {
		p, low := e.offered(0, t, ex)
		if p < f.door {
			f.door, f.low = p, low
		}
	}
	aim := s.AimClear(0)
	f.floor = f.door < aim-s.Rules.BuybackMargin
	f.crash = 1 - math.Pow(min(aim, f.door), float64(b.Rooms))
	return f
}

// offered is what the engine expects of a door biome i offers on topic:
// the mean SolveP of what pick draws a room from (each tier's candidates
// until the room is full), and the lowest rating among the candidates; 0
// when the pool has none left.
func (e *engine) offered(i int, topic string, ex map[catalog.ID]bool) (float64, int) {
	cs := e.candidates(topic, i, ex)
	if len(cs) == 0 {
		return 0, 0
	}
	low := cs[0].p.Rating
	for _, c := range cs {
		low = min(low, c.p.Rating)
	}
	n := e.roomSize(&e.s.Biomes[i])
	need, sum := n, 0.0
	chosen := map[catalog.ID]bool{}
	for _, t := range e.tiers() {
		fit := t.fit(cs, chosen, need)
		if len(fit) == 0 {
			continue
		}
		mean := 0.0
		for _, c := range fit {
			mean += c.sp / float64(len(fit))
			chosen[c.p.ID] = true
		}
		take := min(need, len(fit))
		sum += float64(take) * mean
		if need -= take; need == 0 {
			break
		}
	}
	return sum / float64(n-need), low
}

// buyback is the timer multiplier that buys a door cleared with chance p
// back to within the margin of aim: the smallest of rules.BuybackTimers
// that gets there, or the largest; 1 when there are none.
func (e *engine) buyback(p, aim float64) float64 {
	r := &e.s.Rules
	ks := slices.Sorted(slices.Values(r.BuybackTimers))
	if len(ks) == 0 {
		return 1
	}
	for _, k := range ks {
		if lift(p, r.BuybackBeta*math.Log(k)) >= aim-r.BuybackMargin {
			return k
		}
	}
	return ks[len(ks)-1]
}

// buyTimers gives the doors of a first-biome room the floor easing's
// longer timers, when the pool has no door near the aim: each door short
// of it by more than the margin gets the multiplier that buys its clear
// chance back. Such a door teaches no calibration (Tier ""): its timer
// is a modifier, as a curse is.
func (e *engine) buyTimers(doors []Door) {
	s := e.s
	es := s.active(EaseFloor)
	if es == nil || !es.Buyback || s.Biome != 0 || s.Trial != nil {
		return
	}
	aim := s.AimClear(0)
	for i := range doors {
		d := &doors[i]
		if d.SolveP >= aim-s.Rules.BuybackMargin || d.Regression || d.Boss != "" {
			continue
		}
		k := e.buyback(d.SolveP, aim)
		if k <= 1 {
			continue
		}
		d.TimerBuy = k
		d.Timer = roundTimer(scale(d.Timer, k))
		d.Tier = ""
	}
}

// TimeoutCostsLife reports whether the open door's timer running out now
// would cost a life instead of the boot (rules.Biome.TimeoutLife, D44):
// the biome says so and a spare life is left. The door screen says it in
// these terms.
func (s *State) TimeoutCostsLife() bool {
	return s.Trial == nil && s.Biome < len(s.Rules.Biomes) && s.Rules.Biomes[s.Biome].TimeoutLife && s.Lives > 1
}

// saveTimeout answers a door's timer running out where that costs a life
// (TimeoutCostsLife): the door closes as timed out (rated, and back as a
// regression, like a crash), the life goes, and the room is drawn again,
// as after a retreat. It reports whether it did; otherwise the timeout
// crashes the boot.
func (e *engine) saveTimeout() bool {
	s := e.s
	if !s.TimeoutCostsLife() {
		return false
	}
	name := e.door()
	a := e.close(TimedOut)
	s.Lives--
	e.logf("%s: timer expired, -1 life, %d left", name, s.Lives)
	if due, ok := scheduleRegression(e.meta, a.Door.Problem, e.now, s.Rules.RegressionDays); ok {
		e.logf("regression: %s due %s", a.Door.Problem, due.Format("2006-01-02"))
	}
	e.emit(CloseProblem{Door: a.Door}, Sound{Name: "wa"})
	s.Phase = PhaseDoors
	e.rollRoom(false, "timeout")
	return true
}

// declineEase turns the boot's easing off before its first door: the
// lives and ducks it added go (one life is always left), and the room on
// offer is drawn again, aimed and timed without it.
func (e *engine) declineEase() error {
	s := e.s
	switch {
	case len(s.Ease) == 0:
		return invalid("no easing to decline")
	case s.Declined:
		return invalid("easing already declined")
	case len(s.History) > 0 || s.Current != nil || s.Phase != PhaseDoors && s.Phase != PhaseRoute:
		return invalid("easing can only be declined before the first door")
	}
	lives, ducks := 0, 0
	for _, es := range s.Ease {
		lives += es.Lives
		ducks += es.Ducks
	}
	s.Declined = true
	s.Lives = max(1, s.Lives-lives)
	s.Ducks = max(0, s.Ducks-ducks)
	s.MaxLives = max(s.Rules.MaxLives, s.Lives)
	e.logf("ease: declined, %d lives, %d ducks", s.Lives, s.Ducks)
	if s.Phase == PhaseDoors {
		e.rollRoom(true, "decline")
	}
	return nil
}

// countEnd updates Meta.Governor as a boot ends, crashed or cleared. A
// trial does not count.
func (e *engine) countEnd(crashed bool) {
	if e.s.Trial != nil {
		return
	}
	g := &e.meta.Governor
	if crashed {
		g.FullClears = 0
		if e.s.Biome == 0 {
			g.EarlyCrashes++
		}
		return
	}
	g.EarlyCrashes = 0
	g.FullClears++
}

// reachMerge starts the early-crash count over: the boot got past its
// first biome.
func (e *engine) reachMerge() {
	if e.s.Trial == nil {
		e.meta.Governor.EarlyCrashes = 0
	}
}

func logit(p float64) float64 { return math.Log(p / (1 - p)) }

// lift moves a probability by x in log-odds.
func lift(p, x float64) float64 {
	if x == 0 || p <= 0 || p >= 1 {
		return p
	}
	return 1 / (1 + math.Exp(-(logit(p) + x)))
}
