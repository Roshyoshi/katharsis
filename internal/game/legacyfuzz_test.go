package game_test

import (
	"math/rand/v2"
	"time"

	"katharsis/internal/game"
)

// legacyFuzz is property_test.go's fuzzer as it was when the cycles meta
// landed (D47), frozen: the old-save fixtures (testdata/legacy) were played
// with it on the engine before the change, and TestLegacySaves plays it
// again on the engine now. A change to the live fuzzer must not reach it,
// or the replays stop meaning anything.
type legacyFuzz struct {
	rng *rand.Rand
	now time.Time
}

func (f *legacyFuzz) action(s *game.State) game.Action {
	r := f.rng
	if r.IntN(4) == 0 {
		switch r.IntN(15) {
		case 0:
			return game.ChooseRoute{Index: r.IntN(5) - 1}
		case 1:
			return game.ChooseDoor{Index: r.IntN(6) - 1}
		case 2:
			return game.Retreat{}
		case 3:
			return game.Quack{}
		case 4:
			return game.Commit{}
		case 5:
			return game.PickUpgrade{Upgrade: []game.Upgrade{game.Overclock, game.ECC, game.Duck}[r.IntN(3)]}
		case 6:
			return game.PickDaemon{Daemon: []game.Daemon{game.Strace, game.Nice, game.Fork, game.DevRandom, game.Kexec}[r.IntN(5)]}
		case 7:
			return game.LeaveMerge{}
		case 8:
			return game.Tick{}
		case 9:
			return f.submit()
		case 10:
			if r.IntN(10) == 0 {
				return game.GiveUp{}
			}
			return game.Tick{}
		case 11:
			return game.Spend{Item: []game.Item{game.Fsck, game.Dup, game.Shuf, "bogus"}[r.IntN(4)]}
		case 12:
			return game.Reroll{}
		case 13:
			return game.DeclineEase{}
		}
		return nil
	}
	switch {
	case s.PendingUpgrade:
		return game.PickUpgrade{Upgrade: s.UpgradeOffers[r.IntN(len(s.UpgradeOffers))]}
	case s.Phase == game.PhaseRoute:
		return game.ChooseRoute{Index: r.IntN(len(s.Biomes[s.Biome].Offers))}
	case s.Phase == game.PhaseDoors:
		if len(s.Ease) > 0 && !s.Declined && len(s.History) == 0 && r.IntN(4) == 0 {
			return game.DeclineEase{}
		}
		if s.Rerolls > 0 && r.IntN(4) == 0 {
			return game.Reroll{}
		}
		return game.ChooseDoor{Index: r.IntN(len(s.Doors))}
	case s.Phase == game.PhaseMerge:
		items := []game.Item{game.Fsck, game.Dup, game.Shuf, game.At}
		switch {
		case r.IntN(3) == 0:
			return game.Spend{Item: items[r.IntN(len(items))]}
		case r.IntN(2) == 0 && s.Uncommitted > 0:
			return game.Commit{}
		case len(s.DaemonOffers) > 0 && r.IntN(2) == 0:
			return game.PickDaemon{Daemon: s.DaemonOffers[r.IntN(len(s.DaemonOffers))]}
		}
		return game.LeaveMerge{}
	case s.Phase == game.PhaseProblem:
		switch r.IntN(20) {
		case 0:
			return game.GiveUp{}
		case 1, 2:
			return game.Quack{}
		case 3, 4:
			return game.Retreat{}
		case 5, 6, 7, 8:
			return game.Tick{}
		}
		return f.submit()
	}
	return game.Tick{}
}

func (f *legacyFuzz) submit() game.SubmitResult {
	v := []game.Verdict{"AC", "AC", "AC", "WA", "TLE", "RE", "MLE", "CE"}[f.rng.IntN(8)]
	return game.SubmitResult{Verdict: v, Passed: f.rng.IntN(11), Total: 10}
}

func (f *legacyFuzz) tick() {
	f.now = f.now.Add(time.Duration(f.rng.IntN(int(12 * time.Minute))))
}
