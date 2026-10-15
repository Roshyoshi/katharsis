//go:build qa

// Chaos fuzz of the engine under odd but loadable presets: every built-in
// preset and every one in qa/chaos/presets plays random boots to the end,
// checking that nothing panics and that bytes, lives and the bank stay
// sane. Run from the repo root:
//
//	go test -tags qa ./qa/chaos -run TestPresetFuzz -v
package chaos

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

func pool() []catalog.Problem {
	var ps []catalog.Problem
	for i, t := range catalog.Topics {
		for r := 800; r <= 2400; r += 50 {
			ps = append(ps, catalog.Problem{
				ID:        catalog.ID(fmt.Sprintf("fz:%d-%d", i, r)),
				Source:    catalog.USACO,
				Title:     fmt.Sprintf("%s %d", t, r),
				Topic:     t,
				Tags:      []string{t},
				Rating:    r,
				Judgeable: true,
				TimeLimit: time.Second,
			})
		}
	}
	return ps
}

func presetFiles(t *testing.T) []string {
	fs, _ := filepath.Glob("presets/*.toml")
	if len(fs) == 0 {
		t.Fatal("no presets found")
	}
	return fs
}

func TestPresetFuzz(t *testing.T) {
	ps := pool()
	type preset struct {
		name  string
		rules rules.Rules
	}
	var presets []preset
	for _, r := range rules.BuiltIns() {
		presets = append(presets, preset{r.Name, r})
	}
	for _, f := range presetFiles(t) {
		r, err := rules.Load(f)
		if err != nil {
			t.Logf("%s: does not load (fine): %v", f, err)
			continue
		}
		presets = append(presets, preset{filepath.Base(f), r})
	}
	for _, p := range presets {
		t.Run(p.name, func(t *testing.T) {
			meta := game.Meta{} // one player across the boots, as the store keeps it
			for seed := uint64(1); seed <= 200; seed++ {
				play(t, p.rules, ps, seed, &meta)
			}
			t.Logf("after 200 boots: bank %d, lifetime %d, boots %d", meta.Bank, meta.LifetimeBytes, meta.Boots)
		})
	}
}

// play runs one boot with random actions until it ends.
func play(t *testing.T, r rules.Rules, ps []catalog.Problem, seed uint64, mp *game.Meta) {
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("seed %d: panic: %v", seed, p)
		}
	}()
	rng := rand.New(rand.NewPCG(seed, 7))
	meta := *mp
	defer func() { *mp = meta }()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	st, _, err := game.NewBoot(r, &meta, ps, 0, seed, now)
	if err != nil {
		t.Fatalf("seed %d: NewBoot: %v", seed, err)
	}
	var maxBytes int64
	defer func() {
		if seed == 200 {
			t.Logf("seed %d: phase %s, bank %d, committed %d, max door bytes %d", seed, st.Phase, meta.Bank, st.Committed, maxBytes)
		}
	}()
	for step := 0; step < 2000; step++ {
		if st.Phase == game.PhaseCrashed || st.Phase == game.PhaseCleared {
			break
		}
		now = now.Add(time.Duration(rng.IntN(15)) * time.Second)
		var a game.Action
		switch st.Phase {
		case game.PhaseRoute:
			a = game.ChooseRoute{Index: rng.IntN(4)}
		case game.PhaseDoors:
			if st.PendingUpgrade && len(st.UpgradeOffers) > 0 {
				a = game.PickUpgrade{Upgrade: st.UpgradeOffers[rng.IntN(len(st.UpgradeOffers))]}
			} else {
				a = game.ChooseDoor{Index: rng.IntN(max(len(st.Doors), 1))}
			}
		case game.PhaseProblem:
			switch rng.IntN(10) {
			case 0:
				a = game.Quack{}
			case 1:
				a = game.Retreat{}
			case 2:
				a = game.Tick{}
			default:
				v := []game.Verdict{"AC", "AC", "AC", "AC", "AC", "WA", "TLE", "CE"}[rng.IntN(8)]
				a = game.SubmitResult{Verdict: v, Passed: 1, Total: 10}
			}
		case game.PhaseMerge:
			switch {
			case st.PendingUpgrade && len(st.UpgradeOffers) > 0:
				a = game.PickUpgrade{Upgrade: st.UpgradeOffers[rng.IntN(len(st.UpgradeOffers))]}
			case rng.IntN(2) == 0:
				a = game.Commit{}
			case len(st.DaemonOffers) > 0 && rng.IntN(2) == 0:
				a = game.PickDaemon{Daemon: st.DaemonOffers[rng.IntN(len(st.DaemonOffers))]}
			default:
				a = game.LeaveMerge{}
			}
		}
		game.Step(st, &meta, ps, a, now)
		if d := st.Current; d != nil && d.Door.Bytes > maxBytes {
			maxBytes = d.Door.Bytes
		}
		switch {
		case st.Uncommitted < 0 || st.Committed < 0 || meta.Bank < 0 || meta.LifetimeBytes < 0:
			t.Fatalf("seed %d step %d: negative bytes: uncommitted %d committed %d bank %d lifetime %d",
				seed, step, st.Uncommitted, st.Committed, meta.Bank, meta.LifetimeBytes)
		case st.Lives < 0 || st.Lives > st.MaxLives:
			t.Fatalf("seed %d step %d: lives %d of %d", seed, step, st.Lives, st.MaxLives)
		case st.Ducks < 0:
			t.Fatalf("seed %d step %d: ducks %d", seed, step, st.Ducks)
		}
		for _, d := range st.Doors {
			if d.Bytes < 0 || d.Timer <= 0 {
				t.Fatalf("seed %d step %d: door %s bytes %d timer %v", seed, step, d.Problem, d.Bytes, d.Timer)
			}
		}
	}
}
