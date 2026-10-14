package game

import (
	"math"
	"slices"
	"time"

	"katharsis/internal/rules"
)

// doorTimer is a door's clock: the biome default until the player has
// TimerFitMin in-boot solves, then a fit to their own solve times; either
// way scaled by the boot's multipliers. Bosses get BossTimerMult only on the
// fitted path, since the boss biome's default timer already includes the
// stretch.
func (e *engine) doorTimer(b rules.Biome, boss bool, gap float64) time.Duration {
	r := &e.s.Rules
	base := b.Timer.Duration
	if fit, ok := fitTimer(r, e.meta.Solves, gap); ok {
		if boss {
			fit = scale(fit, r.BossTimerMult)
		}
		base = min(max(fit, scale(base, r.TimerFitLo)), scale(base, r.TimerFitHi))
	}
	return roundTimer(scale(base, e.timerMult()))
}

// timerMult is the product of every timer modifier this boot runs with:
// ascension, each Overclock (compounding), and nice.
func (e *engine) timerMult() float64 {
	r := &e.s.Rules
	m := r.Ascend(e.s.Prestige).TimerMult
	for _, u := range e.s.Upgrades {
		if u == Overclock {
			m *= 1 + r.OverclockFrac
		}
	}
	if slices.Contains(e.s.Daemons, Nice) {
		m *= 1 + r.NiceTimerFrac
	}
	return m
}

// fitTimer estimates a timer for a door gap rating points above the player
// from their own in-boot solves.
//
// Method: take the TimerFitHistory most recent in-boot solves (not upsolves,
// not editorial), keep the max(TimerFitMin, half) closest to gap, move each
// onto gap with t·2^((gap−g)/TimerGapScale), take the TimerQuantile quantile
// (linear between order statistics), and multiply by TimerHeadroom.
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
		if s.Upsolve || s.Kind == Editorial || s.Elapsed <= 0 {
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
	slices.SortStableFunc(xs, func(a, b obs) int {
		switch {
		case a.dist < b.dist:
			return -1
		case a.dist > b.dist:
			return 1
		}
		return 0
	})
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
