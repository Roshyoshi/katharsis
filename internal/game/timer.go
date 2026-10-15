package game

import (
	"cmp"
	"math"
	"slices"
	"time"

	"katharsis/internal/rules"
)

// doorTimer is a door's clock: the biome default until the player has
// TimerFitMin in-boot solves, then a fit to their own solve times; either
// way scaled by the boot's multipliers. Bosses get BossTimerMult only on the
// fitted path, since the boss biome's default timer already includes the
// stretch. The first biome's clock is its own per prestige where
// rules.FirstClockOwn says so (Ascension.FirstTimer), so the ascensions'
// timer cuts leave it alone.
func (e *engine) doorTimer(b rules.Biome, boss bool, gap float64) time.Duration {
	r := &e.s.Rules
	base := b.Timer.Duration
	if fit, ok := fitTimer(r, e.meta.Solves, gap); ok {
		if boss {
			fit = scale(fit, r.BossTimerMult)
		}
		base = min(max(fit, scale(base, r.TimerFitLo)), scale(base, r.TimerFitHi))
	}
	own := r.FirstClockOwn && e.s.Biome == 0 && e.s.Trial == nil
	return roundTimer(scale(base, e.timerMult(!own)))
}

// RouteTimer is the timer the current biome's doors get if the player
// takes topic, for a route card before they are drawn: a door at the
// biome's target rating (State.Target) gets it, with every multiplier the
// boot runs with. Once the timer fits the player's own solves (fitTimer),
// a door above the target gets more and one below less.
func (s *State) RouteTimer(m *Meta, topic string) time.Duration {
	e := &engine{s: s, meta: m}
	rb := s.Rules.Biomes[s.Biome]
	gap := e.target(topic, s.Biome) - PlayerRating(&s.Rules, m, topic)
	return e.doorTimer(rb, rb.Kind == rules.Boss, gap)
}

// OpenTimer is the timer door d of those on offer opens with: its own,
// plus the time bought with at, which goes to whichever door opens next.
// Door cards show it, so the clock a door starts on is the one its card
// said.
func (s *State) OpenTimer(d Door) time.Duration { return d.Timer + s.AtTime }

// timerMult is the product of every timer modifier this boot runs with:
// the clock's setting (rules.TimerMult), ascension, Overclock, nice and
// chrt; the ascensions' only when asc is set.
func (e *engine) timerMult(asc bool) float64 {
	r := &e.s.Rules
	m := r.Clock()
	if asc {
		m = r.Ascend(e.s.Prestige).TimerMult * r.Clock()
	}
	if n := e.s.count(Overclock); r.OverclockMax > 0 {
		m *= e.s.overclock(n)
	} else {
		for range n { // one at a time, as before D50, to the last bit
			m *= 1 + r.OverclockFrac
		}
	}
	if slices.Contains(e.s.Daemons, Nice) {
		m *= 1 + r.NiceTimerFrac
	}
	if slices.Contains(e.s.Daemons, Chrt) {
		m *= 1 - r.ChrtTimerFrac
	}
	return m
}

// overclock is what n Overclock picks multiply a timer by: +OverclockFrac
// each, added and stopping at rules.OverclockMax picks where it is set,
// compounding without a cap where it is not (a boot saved before D50).
func (s *State) overclock(n int) float64 {
	r := &s.Rules
	if r.OverclockMax > 0 {
		return 1 + r.OverclockFrac*float64(min(n, r.OverclockMax))
	}
	return math.Pow(1+r.OverclockFrac, float64(n))
}

// count is how many of u the boot has picked.
func (s *State) count(u Upgrade) int {
	n := 0
	for _, x := range s.Upgrades {
		if x == u {
			n++
		}
	}
	return n
}

// o3Timer is the timer -O3 measures attempt a against: under
// rules.O3OnBase the door's timer before the time the player bought (at,
// Overclock, nice), so bought time never makes the free upgrade easier;
// else the timer it opened with.
func (s *State) o3Timer(a Attempt) time.Duration {
	r := &s.Rules
	if !r.O3OnBase {
		return a.Door.Timer
	}
	bought := s.overclock(s.count(Overclock))
	if slices.Contains(s.Daemons, Nice) {
		bought *= 1 + r.NiceTimerFrac
	}
	return scale(a.Door.Timer-a.At, 1/bought)
}

// fitTimer estimates a timer for a door gap rating points above the player
// from their own in-boot solves.
//
// Method: take the TimerFitHistory most recent in-boot solves (not upsolves,
// trials or editorial), keep the max(TimerFitMin, half) closest to gap,
// move each onto gap with t·2^((gap−g)/TimerGapScale), take the
// TimerQuantile quantile (linear between order statistics), and multiply
// by TimerHeadroom.
//
// Only solves that beat their timer are recorded, so the data is censored
// from above and the raw quantile undershoots; fed back, it would shrink the
// timer every boot. The headroom gives that loop a fixed point near the
// player's true quantile (for lognormal solve times with sigma about 0.5),
// and the caller's clamp bounds what is left.
func fitTimer(r *rules.Rules, solves []Solve, gap float64) (time.Duration, bool) {
	type obs struct {
		dist float64
		t    float64 // seconds, moved onto gap
	}
	var xs []obs
	for i := len(solves) - 1; i >= 0 && len(xs) < r.TimerFitHistory; i-- {
		s := solves[i]
		if s.Upsolve || s.Trial || s.Kind == Editorial || s.Elapsed <= 0 {
			continue
		}
		t := s.Elapsed.Seconds() * math.Exp2((gap-s.Gap)/r.TimerGapScale)
		xs = append(xs, obs{math.Abs(gap - s.Gap), t})
	}
	if len(xs) < r.TimerFitMin {
		return 0, false
	}
	// Stable: ties keep recency order, so the result doesn't depend on
	// sort internals.
	slices.SortStableFunc(xs, func(a, b obs) int { return cmp.Compare(a.dist, b.dist) })
	xs = xs[:max(r.TimerFitMin, len(xs)/2)]
	ts := make([]float64, len(xs))
	for i, x := range xs {
		ts[i] = x.t
	}
	q := quantile(ts, r.TimerQuantile) * r.TimerHeadroom
	return time.Duration(q * float64(time.Second)), true
}

// quantile is the type-7 sample quantile (R's default): linear interpolation
// between the order statistics around (n-1)q. It sorts xs.
func quantile(xs []float64, q float64) float64 {
	slices.Sort(xs)
	h := float64(len(xs)-1) * min(max(q, 0), 1)
	lo := int(h)
	if lo+1 >= len(xs) {
		return xs[len(xs)-1]
	}
	return xs[lo] + (h-float64(lo))*(xs[lo+1]-xs[lo])
}

func scale(d time.Duration, f float64) time.Duration {
	return time.Duration(float64(d) * f)
}

// roundTimer shows whole minutes; nothing gets less than one.
func roundTimer(d time.Duration) time.Duration {
	return max(d.Round(time.Minute), time.Minute)
}
