package boot

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens/boot/fixture"
)

// The doors screen's split: the timed vault's lead or lag, signed, and
// whether a door of the act was hit; gone once the boss judged the act,
// and on the act that ends the boot.
func TestVaultSplitLine(t *testing.T) {
	st := fixture.CyclesBoss("off-by-one") // Boss I's room, act I slow
	if v := ansi.Strip(vaultSplit(st)); v != "act I vaults · timed −33:00 · untouched ✓" {
		t.Errorf("split %q", v)
	}
	st.Biomes[1].Hits = 2
	if v := ansi.Strip(vaultSplit(st)); !strings.HasSuffix(v, "untouched ✗ 2 hits") {
		t.Errorf("hit: %q", v)
	}
	st.Vaults = []game.Vault{{Kind: game.VaultTimed, Act: 0}}
	if v := vaultSplit(st); v != "" {
		t.Errorf("judged: %q", v)
	}
	if v := vaultSplit(fixture.TrialDoor()); v != "" {
		t.Errorf("the Court ends the boot, yet %q", v)
	}
}

// A full window's offers are swaps: enter on one asks which running daemon
// it takes over, enter there swaps for the price, and esc backs out.
func TestDaemonSwap(t *testing.T) {
	st := fixture.CyclesMerge("drink")
	st.PendingUpgrade, st.UpgradePicks, st.UpgradeOffers = false, 0, nil
	st.Daemons = []game.Daemon{game.Strace, game.Kexec, game.Nice}
	st.DaemonOffers = []game.Daemon{game.Burst, game.Ksmd}
	m := NewMerge(env(st))
	down := func() {
		for r := m.row(); r.kind != rowDaemon; r = m.row() {
			m.Update(press("down"))
		}
	}
	down()
	if v := ansi.Strip(m.View(80, 40)); !strings.Contains(v, "every seat taken: one swaps in for 64 B") {
		t.Fatalf("full window:\n%s", v)
	}
	if _, cmd := m.Update(press("enter")); cmd != nil {
		t.Fatalf("enter on a swap emitted %#v", cmd())
	}
	if v := ansi.Strip(m.View(80, 40)); !strings.Contains(v, "take burst over which?") || !strings.Contains(v, "strace") {
		t.Fatalf("replace prompt:\n%s", v)
	}
	m.Update(press("esc"))
	if m.swap != -1 || m.row().kind != rowDaemon {
		t.Errorf("esc: swap %d, row %+v", m.swap, m.row())
	}
	m.Update(press("enter"))
	m.Update(press("down")) // the second running daemon
	_, cmd := m.Update(press("enter"))
	if got := emitted(cmd); got != (game.PickDaemon{Daemon: game.Burst, Replace: game.Kexec}) {
		t.Errorf("swap emitted %#v", got)
	}
}

// The doors screen says what the boot holds, -Ofast's curse first.
func TestHeldLine(t *testing.T) {
	st := fixture.CyclesBoss("off-by-one")
	if v := heldLine(st); v != "" {
		t.Errorf("nothing held: %q", v)
	}
	st.Snapshots, st.Nohup, st.Ofast = 2, 1, true
	if v := ansi.Strip(heldLine(st)); v != "-Ofast: the door you open is cursed · held snapshot ×2 · nohup" {
		t.Errorf("held: %q", v)
	}
}
