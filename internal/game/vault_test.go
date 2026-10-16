package game_test

import (
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// toBossI plays a map boot to Boss I's doors, Userland and act I by the
// scheduler, each door opened and, after each(i, h), accepted: each can
// wait, fail a submit, or do nothing.
func toBossI(t *testing.T, r rules.Rules, each func(int, *harness)) *harness {
	t.Helper()
	h := newHarness(t, r, &game.Meta{}, dense(catalog.Topics, 800, 2600, 20), 0, 4)
	n := 0
	tier := func() {
		b := h.s.Biome
		for h.s.Biome == b && h.s.Phase == game.PhaseDoors {
			h.open(0)
			each(n, h)
			n++
			h.do(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
		}
		h.pick()
		h.do(game.LeaveMerge{})
	}
	tier()
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")})
	tier()
	if h.s.Biome != 2 || h.s.Biomes[2].Kind != rules.Boss {
		t.Fatalf("not at Boss I: biome %d %+v", h.s.Biome, h.s.Biomes[h.s.Biome])
	}
	return h
}

// clearBoss opens Boss I's door and accepts it after d, returning the picks
// the clear owed.
func clearBoss(h *harness, d time.Duration) int {
	h.t.Helper()
	h.open(0)
	h.now = h.now.Add(d)
	h.do(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
	return h.s.UpgradePicks
}

func vaultOpen(s *game.State, kind string) (game.Vault, bool) {
	for _, v := range s.Vaults {
		if v.Kind == kind {
			return v, v.Open
		}
	}
	return game.Vault{}, false
}

// An act played fast and clean opens both vaults at its boss, each owing a
// pick on top of the clear's; the merge window after it holds them.
func TestVaultsOpen(t *testing.T) {
	t.Parallel()
	r := mapPlain()
	r.O3Frac = 0 // -O3's free picks apart, the vaults' alone count
	h := toBossI(t, r, func(int, *harness) {})
	picks := clearBoss(h, time.Minute)
	if h.s.Phase != game.PhaseMerge {
		t.Fatalf("phase %s", h.s.Phase)
	}
	timed, ok := vaultOpen(h.s, game.VaultTimed)
	if !ok || timed.Picks != 1 || timed.Took > timed.Limit || timed.Act != 0 {
		t.Errorf("timed vault %+v", timed)
	}
	if v, ok := vaultOpen(h.s, game.VaultUntouched); !ok || v.Hits != 0 || v.Picks != 1 {
		t.Errorf("untouched vault %+v", v)
	}
	if picks != 3 {
		t.Errorf("picks owed %d, want the clear's and two vaults'", picks)
	}
	if vs := h.s.ActVaults(h.s.Biome); len(vs) != 2 {
		t.Errorf("act vaults %+v", vs)
	}

	// Without vaults in the rules, the same boss owes its own pick only.
	r.VaultPicks = 0
	h = toBossI(t, r, func(int, *harness) {})
	if picks := clearBoss(h, time.Minute); picks != 1 || len(h.s.Vaults) != 0 {
		t.Errorf("no vaults: %d picks, %+v", picks, h.s.Vaults)
	}
}

// A slow act seals the timed vault, a hit the untouched one: what counts
// as a hit is a charged failure, in cycles or under ECC in clock, and not
// a free one (a sample's fail, a CE, kexec's swap).
func TestVaultsSealed(t *testing.T) {
	t.Parallel()
	r := mapPlain()
	r.O3Frac = 0
	slow := toBossI(t, r, func(_ int, h *harness) { h.now = h.now.Add(9 * time.Minute) })
	if picks := clearBoss(slow, 30*time.Minute); picks != 2 {
		t.Errorf("slow: %d picks, want the clear's and the untouched vault's", picks)
	}
	if v, open := vaultOpen(slow.s, game.VaultTimed); open || v.Took <= v.Limit {
		t.Errorf("slow: timed vault %+v", v)
	}

	for _, tc := range []struct {
		name string
		fail game.Action
		hits int
	}{
		{"WA", game.SubmitResult{Verdict: "WA", Passed: 3, Total: 10}, 1},
		{"a sample's WA", game.SubmitResult{Verdict: "WA", Passed: 0, Total: 10, SampleFailed: true}, 0},
		{"CE", game.SubmitResult{Verdict: "CE"}, 0},
	} {
		h := toBossI(t, r, func(i int, h *harness) {
			if i == 1 {
				h.do(tc.fail)
			}
		})
		clearBoss(h, time.Minute)
		v, open := vaultOpen(h.s, game.VaultUntouched)
		if v.Hits != tc.hits || open != (tc.hits == 0) {
			t.Errorf("%s: untouched vault %+v", tc.name, v)
		}
	}

	// ECC turns a failed submit's cycles into clock: still a hit.
	ecc := toBossI(t, r, func(i int, h *harness) {
		if i == 0 {
			h.s.Upgrades = append(h.s.Upgrades, game.ECC)
		}
		if i == 1 {
			cycles := h.s.Cycles
			h.do(game.SubmitResult{Verdict: "WA", Passed: 3, Total: 10})
			if h.s.Cycles != cycles {
				h.t.Fatalf("ECC charged cycles")
			}
		}
	})
	clearBoss(ecc, time.Minute)
	if v, open := vaultOpen(ecc.s, game.VaultUntouched); open || v.Hits != 1 {
		t.Errorf("ECC: untouched vault %+v", v)
	}
}

// A retreat is a hit when paid, and none when kexec swaps the door free.
func TestVaultRetreat(t *testing.T) {
	t.Parallel()
	for _, kexec := range []bool{false, true} {
		h := toBossI(t, mapPlain(), func(i int, h *harness) {
			if i != 1 {
				return
			}
			if kexec {
				h.s.Daemons = append(h.s.Daemons, game.Kexec)
				h.s.KexecUsed = false
			}
			h.do(game.Retreat{})
			h.open(0)
		})
		clearBoss(h, time.Minute)
		if v, open := vaultOpen(h.s, game.VaultUntouched); open != kexec {
			t.Errorf("kexec %v: untouched vault %+v", kexec, v)
		}
	}
}

// The timed vault measures each door against its own clock: time bought
// for it (Overclock) does not lower the bar.
func TestVaultBaseTimer(t *testing.T) {
	t.Parallel()
	h := newHarness(t, mapPlain(), &game.Meta{}, dense(catalog.Topics, 800, 2600, 20), 0, 4)
	h.s.Upgrades = append(h.s.Upgrades, game.Overclock)
	d := h.open(0)
	h.do(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
	a := h.s.History[len(h.s.History)-1]
	if a.Base <= 0 || a.Base >= d.Timer {
		t.Errorf("base %s of a %s door under Overclock", a.Base, d.Timer)
	}
}

// The act that ends the boot has no vaults: the final's clear owes none.
func TestNoVaultsAtTheEnd(t *testing.T) {
	t.Parallel()
	h := newHarness(t, mapPlain(), &game.Meta{}, dense(catalog.Topics, 800, 2600, 20), 0, 4)
	nextTier(h)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "scheduler")})
	nextTier(h)
	nextTier(h) // Off-by-One: Boss I's vaults
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "alu")})
	nextTier(h)
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "heisenbug")})
	nextTier(h) // Heisenbug: Boss II's
	h.do(game.ChooseRoute{Index: exitIndex(t, h.s, "skip")})
	before := len(h.s.Vaults)
	if _, ok := h.s.ActSplit(h.s.Biome); ok {
		t.Error("the final's act has a split")
	}
	h.pick()
	h.clearDoor(0, time.Minute)
	if h.s.Phase != game.PhaseCleared || len(h.s.Vaults) != before || before != 4 {
		t.Errorf("phase %s, vaults %d then %d, want Boss I's and Boss II's 4", h.s.Phase, before, len(h.s.Vaults))
	}
}
