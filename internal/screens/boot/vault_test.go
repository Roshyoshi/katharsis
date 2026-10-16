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
