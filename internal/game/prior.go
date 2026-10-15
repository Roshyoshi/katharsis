package game

import "time"

// Prior is the player's answer to the one question asked on first launch
// (D37): their Codeforces rating, as the cold start's prior. Skipping is an
// answer too, and it is never asked again.
type Prior struct {
	CF     int     // the rating as given; 0 when skipped
	Seeded float64 // the rating it set, CFPrior(CF); 0 when skipped
	At     time.Time
}

// Problem ratings span PriorMin to PriorMax; a prior outside that would aim
// doors at ratings no problem has.
const (
	PriorMin = 800
	PriorMax = 3500
)

// NeedsPrior reports whether to ask: the question was never answered and
// no door has been rated, so Meta.Rating is still the cold start's guess.
// A player with rated doors has a better estimate than any answer.
func NeedsPrior(m *Meta) bool { return m.Prior == nil && rated(m) == 0 }

// CFPrior maps a Codeforces rating onto the engine's rating scale. They are
// one scale: Meta.Rating is CF-scale, Expected is Elo's logistic on 400
// points, which Codeforces' ratings and problem ratings follow too (a
// problem rated r is solved half the time by a contestant rated r), and
// docs/research/ml-pipeline.md §2.9 anchors a contestant's general level
// at their CF rating as is. So the map is the identity, clamped to the
// problem range. Whatever a CF contest and a timed door do not share (the
// timer, local judging) is the calibration offset's to learn, per tier,
// as it already does for the cold start.
func CFPrior(cf int) float64 { return float64(min(max(cf, PriorMin), PriorMax)) }

// SeedPrior records the answer at now; a rating (cf > 0) replaces the cold
// start. Rated stays 0, so the first rules.RatingColdAttempts doors still
// move the rating with rules.RatingKCold: a stale or rounded answer is
// corrected within a boot or two, as a wrong cold start is.
func SeedPrior(m *Meta, cf int, now time.Time) {
	p := &Prior{At: now}
	if cf > 0 {
		p.CF, p.Seeded = cf, CFPrior(cf)
		m.Rating = p.Seeded
	}
	m.Prior = p
}
