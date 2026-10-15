package sim_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// With its learning, sessions and churn off and no item noise, the
// learning player is today's model player draw for draw: the same report
// and the same Meta from the same seeds (ml-pipeline §5.2's identity).
func TestLearnOffIsToday(t *testing.T) {
	t.Parallel()
	pool := sim.SyntheticPool(3000, 4)
	for _, rating := range []float64{1000, 1600} {
		for seed := range uint64(3) {
			today := sim.DefaultPlayer(rating)
			off := today
			off.Learn = &sim.Learning{}
			a := sim.Run(sim.Config{Rules: rules.Standard(), Pool: pool, Player: today, Boots: 120, Warmup: 20, Seed: seed})
			b := sim.Run(sim.Config{Rules: rules.Standard(), Pool: pool, Player: off, Boots: 120, Warmup: 20, Seed: seed})
			ma, _ := json.Marshal(a.Meta)
			mb, _ := json.Marshal(b.Meta)
			if a.String() != b.String() || string(ma) != string(mb) {
				t.Fatalf("rating %.0f seed %d: the learner with everything off changed the run:\n%s\n%s", rating, seed, a, b)
			}
			if b.Learn == nil || b.Learn.Gain != 0 || b.Learn.Quit {
				t.Errorf("rating %.0f seed %d: learn stats %+v", rating, seed, b.Learn)
			}
		}
	}
}

// The churn hazard after three crashes in a row is EOMM's: about twice
// the hazard after crash, crash, clear (5.1% against 2.6%).
func TestChurnRatio(t *testing.T) {
	t.Parallel()
	c := sim.DefaultLearning().Churn
	sigmoid := func(x float64) float64 { return 1 / (1 + math.Exp(-x)) }
	lll, llw := sigmoid(c.A0+c.A3), sigmoid(c.A0)
	if r := lll / llw; math.Abs(llw-0.026) > 1e-9 || math.Abs(r-1.96) > 0.05 {
		t.Errorf("LLL %.4f, LLW %.4f: ratio %.2f, want EOMM's 0.051/0.026", lll, llw, r)
	}
}

// The ideal player in the learning harness (the logistic model, no noise,
// learning off) is calibrated: each biome's plain doors clear at the clear
// chance the engine gave them, within 3 standard errors and 2 points, and
// Elo sits within its own noise of the true level. (The binned ECE is
// logged, not held: Elo's noise alone spreads SolveP past the truth.) Learning on (3 points a door), the same player gains
// and keeps most of it; how far Elo then lags is logged, for the shadow
// model (slice ML4) to beat.
func TestIdealLearnerCalibrated(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow")
	}
	r := rules.Standard()
	r.Packages = nil
	for _, learning := range []bool{false, true} {
		p := ideal(1300)
		p.Install = nil
		p.Learn = &sim.Learning{}
		if learning {
			p.Learn = sim.DefaultLearning()
			p.Learn.Sessions, p.Learn.Churn = false, nil
		}
		rep := sim.Run(sim.Config{Rules: r, Pool: sim.SyntheticPool(20000, 1), Player: p, Boots: 400, Warmup: 50, Seed: 3})
		st := rep.Learn
		t.Logf("ideal 1300, learning %v: gain %+.0f (durable %+.0f), rating %.0f, RMSE %.0f, ECE %.3f",
			learning, st.Gain, st.Durable, rep.FinalRating, st.RMSE, st.ECE)
		if !learning && (st.RMSE > 120 || st.Gain != 0) {
			t.Errorf("ideal player: RMSE %.0f, gain %.1f", st.RMSE, st.Gain)
		}
		for i := range rep.Taught {
			if learning {
				break
			}
			got, exp := taught(rep, i)
			if tol := 3*math.Sqrt(exp*(1-exp)/float64(max(rep.Taught[i], 1))) + 0.02; math.Abs(got-exp) > tol {
				t.Errorf("ideal player biome %d: plain doors clear %.3f, engine expected %.3f (±%.3f)", i, got, exp, tol)
			}
		}
		if learning && (st.Gain <= 0 || st.Durable >= st.Gain || st.Durable <= 0.8*st.Gain) {
			t.Errorf("gain %+.1f, durable %+.1f: the learner should gain and keep most of it a month on", st.Gain, st.Durable)
		}
	}
}

// TestLearningReport plays a year of learning players per learning shape
// and gain (ml-pipeline §5.2): durable skill, hours, how many quit, G1,
// the engine's calibration and error, and the churn after three crashes
// against after crash, crash, clear. A report, not a check: set
// KATHARSIS_LEARN=1 and run with -v; KATHARSIS_LEARN_PLAYERS sets players
// per cell (default 40).
func TestLearningReport(t *testing.T) {
	if os.Getenv("KATHARSIS_LEARN") == "" {
		t.Skip("set KATHARSIS_LEARN=1 for the learning report")
	}
	players := 40
	if n, err := strconv.Atoi(os.Getenv("KATHARSIS_LEARN_PLAYERS")); err == nil && n > 0 {
		players = n
	}
	pool := sim.SyntheticPool(20000, 1)
	type cell struct {
		gain, durable, hours, rmse, ece float64
		quit, boots, g1                 int
		lll, lllc, llw, llwc            int
	}
	shapes := []string{"flat", "inverted-u", "harder"}
	gains := []float64{1, 3, 6}
	ratings := []float64{900, 1300, 1600}
	var w strings.Builder
	var mu sync.Mutex
	var all cell
	for _, rating := range ratings {
		fmt.Fprintf(&w, "\nplayer %.0f, %d players a cell, one year:\n  %-11s %3s  %14s %7s %6s %5s %5s %5s\n",
			rating, players, "shape", "γ", "gain/durable", "hours", "quit", "G1", "RMSE", "ECE")
		for _, shape := range shapes {
			for _, g := range gains {
				var c cell
				var wg sync.WaitGroup
				for i := range players {
					wg.Go(func() {
						p := sim.DefaultPlayer(rating)
						l := sim.DefaultLearning()
						l.Shape, l.Gain = shape, g
						p.Learn = l
						rep := sim.Run(sim.Config{Rules: rules.Standard(), Pool: pool, Player: p, Boots: 2000, Days: 365, Seed: uint64(1000*i + 7)})
						st := rep.Learn
						mu.Lock()
						defer mu.Unlock()
						c.gain += st.Gain
						c.durable += st.Durable
						c.hours += st.Hours
						c.rmse += st.RMSE
						c.ece += st.ECE
						c.boots += rep.Boots
						c.g1 += rep.CrashBiome[0]
						c.lll += st.LLL
						c.lllc += st.LLLChurned
						c.llw += st.LLW
						c.llwc += st.LLWChurned
						if st.Quit {
							c.quit++
						}
					})
				}
				wg.Wait()
				n := float64(players)
				fmt.Fprintf(&w, "  %-11s %3.0f  %+6.0f / %+5.0f %7.0f %5.0f%% %4.1f%% %5.0f %5.3f\n", shape, g,
					c.gain/n, c.durable/n, c.hours/n, 100*float64(c.quit)/n, 100*float64(c.g1)/float64(max(c.boots, 1)),
					c.rmse/n, c.ece/n)
				all.lll += c.lll
				all.lllc += c.lllc
				all.llw += c.llw
				all.llwc += c.llwc
			}
		}
	}
	lll := float64(all.lllc) / float64(max(all.lll, 1))
	llw := float64(all.llwc) / float64(max(all.llw, 1))
	fmt.Fprintf(&w, "\nchurn after LLL %.1f%% (%d), after LLW %.1f%% (%d): ratio %.2f (EOMM 1.96)\n",
		100*lll, all.lll, 100*llw, all.llw, lll/max(llw, 1e-9))
	t.Log(w.String())
	_ = time.Hour
}
