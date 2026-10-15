package game

import (
	"fmt"
	"math"

	"katharsis/internal/rules"
)

// CalibrationTier names the tier whose Calibration aims a biome's doors.
// Biomes aiming at the same clear rate share one, so a preset's Userland
// and a custom early biome learn together; boss biomes keep their own, as
// a boss is one door with no choice and a longer timer.
func CalibrationTier(b rules.Biome) string {
	if b.Kind == rules.Boss {
		return fmt.Sprintf("boss %.2f", b.TargetClear)
	}
	return fmt.Sprintf("%.2f", b.TargetClear)
}

// CalibrationAll is the Meta.Calibration entry every tier shares: a door is
// aimed with it plus its own tier's entry.
const CalibrationAll = "all"

// aimOffset is the calibration added to rb's door targets, within
// ±CalibMax: 0 in a trial (its doors are the module's, not aimed) and with
// calibration off.
func (e *engine) aimOffset(rb rules.Biome) float64 {
	r := &e.s.Rules
	if e.s.Trial != nil || r.CalibRate <= 0 {
		return 0
	}
	c := e.meta.Calibration
	return min(max(c[CalibrationAll].Offset+c[CalibrationTier(rb)].Offset, -r.CalibMax), r.CalibMax)
}

// calibrate teaches the aim from a closed door. The expectation is
// d.SolveP: the logistic chance at the door's rating with the calibration
// taken back out, so it already counts the prestige offset. At prestige 2
// doors are aimed 2·PrestigeOffset harder, the engine expects the clear
// rate to drop by that much, and only a drop larger or smaller than that
// moves the aim: calibration corrects the curve's shape and the rating's
// bias, it does not undo prestige.
//
// Most of the correction is common to every tier: the Elo rating settles
// where hinted clears (scored HintedScore), retreats and wrong submits
// put it, not where the logistic aims right, and that is off by the same
// amount everywhere. So the error is learned twice. CalibrationAll takes
// every plain door's surprise at the same weight, (cleared − SolveP)
// scaled as if SolveP were 1/2 (for the logistic, the best estimate of a
// common shift weighs every door alike); it pools all tiers, so the boss,
// one door a boot at best, does not wait on its own doors. The tier's own
// entry takes a damped Newton step on its slope, (cleared − SolveP) /
// slope(SolveP) with slope(p) = p(1−p)·ln10/400, floored at p = 0.2 and
// 0.8, where a door says little about rating and a full step would swing
// the offset by hundreds of points. Each moves by its gain: 1/(doors +
// CalibWarm) at first, so the first doors average like a running mean,
// then CalibRate. The two entries can trade a constant between them (the
// shared one up, every tier's down) without changing a door, so a tier's
// own also leaks a tenth of CalibRate toward 0 per door: what all tiers
// share settles in CalibrationAll instead of drifting into a clamp. The
// shared entry is clamped to ±CalibMax, a tier's own to half that.
//
// A retreat teaches as a failure, and the door that replaces it as a fresh
// door, although the rating ignores retreats (a swap is no verdict on
// skill). The aim is about doors, and a player retreats from doors they
// expect to fail: skipping retreats here keeps only the doors the player
// chose to keep, which clear more often than the doors offered. In the
// simulator a logistic player with kexec then clears 32% of plain boss
// doors aimed at 50%; counted, every tier lands on target. Teaching only
// the door that ends each room would harden doors until second chances
// bought nothing.
//
// Only plain, aimed doors teach (Door.Tier): modifiers are risks the
// player chose or prestige imposed, and absorbing them would make blind
// and cursed doors easier and pay the same. Nothing teaches until the
// player has CalibAfter rated attempts (and is past the cold start): the
// rating is still climbing toward the player before that, and the offsets
// would learn the climb as if it were the curve's shape.
func calibrate(r *rules.Rules, m *Meta, d Door, o Outcome) {
	if r.CalibRate <= 0 || d.Tier == "" || Rated(m) < max(r.RatingColdAttempts, r.CalibAfter) {
		return
	}
	y := 0.0
	if o == Cleared {
		y = 1
	}
	if m.Calibration == nil {
		m.Calibration = map[string]Calibration{}
	}
	const k = math.Ln10 / 400
	p := min(max(d.SolveP, 0.2), 0.8)
	for _, s := range []struct {
		key   string
		slope float64
		max   float64
		leak  float64
	}{
		{CalibrationAll, 0.25 * k, r.CalibMax, 0},
		{d.Tier, p * (1 - p) * k, r.CalibMax / 2, r.CalibRate / 10},
	} {
		c := m.Calibration[s.key]
		gain := r.CalibRate
		if r.CalibWarm > 0 {
			gain = max(gain, 1/float64(c.Doors+r.CalibWarm))
		}
		c.Offset = c.Offset*(1-s.leak) + gain*(y-d.SolveP)/s.slope
		c.Offset = min(max(c.Offset, -s.max), s.max)
		c.Doors++
		// A running mean until there are 1/CalibRate doors, then a
		// moving one.
		w := max(r.CalibRate, 1/float64(c.Doors))
		c.Observed += w * (y - c.Observed)
		c.Expected += w * (d.SolveP - c.Expected)
		m.Calibration[s.key] = c
	}
}
