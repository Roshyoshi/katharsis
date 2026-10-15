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
// d.SolveP, the logistic chance with the calibration taken back out, so it
// already counts the prestige offset: calibration corrects the curve's
// shape and the rating's bias, it does not undo prestige.
//
// The error is learned twice. Most of it is common to every tier (the Elo
// rating settles where hinted clears, retreats and wrong submits put it,
// not where the logistic aims right), so CalibrationAll takes every plain
// door's surprise, (cleared − SolveP) scaled as if SolveP were 1/2; pooling
// the tiers means the boss, one door a boot at best, does not wait on its
// own doors. The tier's own entry takes a damped Newton step,
// (cleared − SolveP) / slope(SolveP) with slope(p) = p(1−p)·ln10/400 and p
// held to 0.2..0.8, past which a full step would swing the offset by
// hundreds of points. Each moves by 1/(doors + CalibWarm) at first, a
// running mean, then by CalibRate. The two entries could trade a constant
// (the shared one up, every tier's down) without changing a door, so a
// tier's own leaks CalibRate/10 toward 0 per door and what the tiers share
// settles in CalibrationAll instead of drifting into a clamp. The shared
// entry is clamped to ±CalibMax, a tier's own to half that.
//
// A retreat teaches as a failure, although the rating ignores it. A player
// retreats from doors they expect to fail, so skipping retreats keeps only
// the doors they chose to keep, which clear more often than the doors
// offered: in the simulator a kexec player then cleared 32% of plain boss
// doors aimed at 50%.
//
// Only plain, aimed doors teach (Door.Tier): modifiers are risks the player
// chose or prestige imposed, and absorbing them would make blind and cursed
// doors easier for the same pay. Nothing teaches before CalibAfter rated
// attempts (or during the cold start): the offsets would learn the
// rating's climb as if it were the curve's shape.
func calibrate(r *rules.Rules, m *Meta, d Door, o Outcome) {
	if r.CalibRate <= 0 || d.Tier == "" || rated(m) < max(r.RatingColdAttempts, r.CalibAfter) {
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
