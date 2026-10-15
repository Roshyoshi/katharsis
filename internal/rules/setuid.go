package rules

import "time"

// SetuidBits are the numbers of the setuid bits, the game's achievements
// (docs/design/achievements.md §(d), §(e)): the bars the visible bits
// state and completion's weights. They are no boot's rules: a bit belongs
// to the store it was set in, not to a preset, so no State snapshots them,
// and only the built-in presets' boots count toward one.
type SetuidBits struct {
	// Speedrun1 and Speedrun2 bound a three-act clear's door time (S1, S2;
	// provisional, decision 4: about half and a third of the model's
	// median clear). LastMinute is the window before The Halting Problem's
	// deadline that X4 asks its clear to land in.
	Speedrun1, Speedrun2, LastMinute time.Duration
	// SpeedCompletion and SpeedCompletion112 bound the boot time (TIME+
	// less upsolves and trials) at which 100% and 112% are reached (S15,
	// S16; placeholders until the learning simulator's 25th percentile,
	// decision 5).
	SpeedCompletion, SpeedCompletion112 time.Duration

	// Retired, Recovered, Rated and Libs are the completion counters' bars
	// (C6-C9): regressions retired, core dumps recovered, rated doors and
	// libs saved.
	Retired, Recovered, Rated, Libs int

	// Completion in points (§(e)). 100% is BossPoints a boss defeated,
	// NodePoints a map node with a cleared room, CapPoints a capability
	// held, RingPoints each prestige 1..RingTop unlocked and a three-act
	// clear at RingTop, and FirstPoints each first: a dump recovered, a
	// regression retired, a lib saved, a topic mastered, a skip clear, a
	// nightly clear. The 12% beyond is BeyondPoints each prestige past
	// RingTop to BeyondTop unlocked, a three-act clear at BeyondTop, and
	// the Trial's ending.
	BossPoints, NodePoints, CapPoints, RingPoints, FirstPoints, BeyondPoints int
	RingTop, BeyondTop                                                       int
}

// Setuid is the design's numbers: S1 1:30:00 and S2 1:00:00 of door time,
// X4's last minute, S15 60 h and S16 150 h of boot time, 5 regressions, 5
// dumps, 100 rated doors and 5 libs, and completion's 3/1/2/5/3 points to
// 100% (rings to prestige 4) and 2 each beyond, to 112% (prestige 8).
func Setuid() SetuidBits {
	return SetuidBits{
		Speedrun1: 90 * time.Minute, Speedrun2: 60 * time.Minute, LastMinute: time.Minute,
		SpeedCompletion: 60 * time.Hour, SpeedCompletion112: 150 * time.Hour,
		Retired: 5, Recovered: 5, Rated: 100, Libs: 5,
		BossPoints: 3, NodePoints: 1, CapPoints: 2, RingPoints: 5, FirstPoints: 3, BeyondPoints: 2,
		RingTop: 4, BeyondTop: 8,
	}
}
