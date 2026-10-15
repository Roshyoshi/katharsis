package sim_test

import (
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// TestFirstBootLoop is retention RT11 against the model players: from an
// empty Meta, boot #1 (the short shape, the floor's doors first) closes in
// about an hour (mean wall time at most 75 minutes) and leaves the
// cheapest package affordable in 80% of runs. The bound was 70 until
// Userland's doors went from 10 to 15 min after the alpha: the 1000 model
// survives Userland more often and plays on, 66 -> 73 min. The engine's
// own estimate of the first door is logged: a cold start reads a
// Codeforces 800 at 0.89, short of RT11's 0.95 until the player gives a
// rating (D37) or the pool has easier problems.
func TestFirstBootLoop(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	r := rules.Standard()
	cheapest := r.Ladder()[0].Price
	pool := sim.SyntheticPool(20000, 1)
	for _, rating := range []float64{1000, 1600} {
		const runs = 200
		var up, wall time.Duration
		var est float64
		afford, cleared := 0, 0
		for i := range runs {
			var first *game.Attempt
			rep := sim.Run(sim.Config{Rules: r, Pool: pool, Player: sim.DefaultPlayer(rating), Boots: 1, Seed: uint64(5000 + i),
				Check: func(s *game.State, m *game.Meta, a game.Action, fx []game.Effect, err error) {
					if first == nil && len(s.History) > 0 {
						h := s.History[0]
						first = &h
					}
				}})
			up += rep.MeanUptime
			wall += rep.MeanWall
			if rep.Banked[0] >= cheapest {
				afford++
			}
			if first != nil {
				est += first.Door.SolveP
				if first.Outcome == game.Cleared {
					cleared++
				}
			}
		}
		mean := wall / runs
		t.Logf("model %.0f, boot #1: mean wall time %s (uptime %s), first door estimated %.2f and cleared %d%%, package after boot 1 in %d%%",
			rating, mean.Round(time.Minute), (up / runs).Round(time.Minute), est/runs, 100*cleared/runs, 100*afford/runs)
		if mean > 75*time.Minute {
			t.Errorf("model %.0f: boot #1 lasts %s on average, want at most 75 min", rating, mean.Round(time.Minute))
		}
		if 5*afford < 4*runs {
			t.Errorf("model %.0f: the cheapest package after boot 1 in %d of %d runs, want 80%%", rating, afford, runs)
		}
	}
}
