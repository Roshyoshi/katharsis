package flavor

import (
	"strings"
	"testing"

	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// TestNodeBlurb: Kernel Space, act III's mixed node, has its own route
// line; every other node reads its topic's.
func TestNodeBlurb(t *testing.T) {
	want := "Any topic, every privilege, and no MMU to catch you."
	if got := NodeBlurb("kernel", ""); got != want {
		t.Errorf("NodeBlurb(kernel) = %q, want %q", got, want)
	}
	checkLine(t, "NodeBlurb(kernel)", NodeBlurb("kernel", ""))
	for _, n := range rules.MapNodes() {
		if n.Skip || n.Boss != "" {
			continue
		}
		topic := n.Topic
		if topic == "" {
			topic = game.Mixed
		}
		if got := NodeBlurb(n.ID, topic); got == "" {
			t.Errorf("node %s has no route line", n.ID)
		} else if n.ID != "kernel" && got != BiomeBlurb(topic) {
			t.Errorf("node %s: %q, want its topic's", n.ID, got)
		}
	}
}

// TestCyclesTexts: the upgrade, daemon and item texts say cycles (D48)
// and quote the map's prices (map.md §(d)): Overclock 5 cycles a pick, two
// at most; ECC 5:00 of the clock; fsck +10; kexec once an act for a swap
// that burns 5. Under the rules before cycles they read one cycle apiece.
func TestCyclesTexts(t *testing.T) {
	m := rules.Map()
	for _, c := range []struct{ got, want string }{
		{text(UpgradeTextFor(game.Overclock, m)), "+15% timer on later doors; 5 cycles a pick, 2 at most."},
		{text(UpgradeTextFor(game.ECC, m)), "A failed submit costs 05:00 of the door's clock, not cycles, while more is left."},
		{text(ItemTextFor(game.Fsck, m)), "+10 cycles now."},
		{text(DaemonTextFor(game.Kexec, m)), "One free door swap per act; a swap normally burns 5 cycles."},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
	l := rules.Legacy()
	if got := text(ItemTextFor(game.Fsck, l)); got != "+1 cycle now." {
		t.Errorf("fsck before cycles: %q", got)
	}
	for _, r := range []rules.Rules{m, l, rules.Standard()} {
		var all []string
		for _, u := range []game.Upgrade{game.Overclock, game.ECC, game.Duck, game.ShufPick} {
			all = append(all, text(UpgradeTextFor(u, r)))
		}
		for _, d := range game.AllDaemons {
			all = append(all, text(DaemonTextFor(d, r)))
		}
		for _, it := range game.AllItems {
			all = append(all, text(ItemTextFor(it, r)))
		}
		for _, s := range all {
			if strings.Contains(s, "life") || strings.Contains(s, "lives") {
				t.Errorf("%s: %q still says lives", r.Name, s)
			}
		}
	}
}

func text(_, desc string) string { return desc }
