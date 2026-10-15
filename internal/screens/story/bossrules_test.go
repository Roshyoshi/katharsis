package story

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
)

// TestBossCardRules: a boss card names the rule its twist bends in the
// boss's voice (boss.rule.<key>), its price under cycles, and a one-shot
// boss's warning (boss.oneshot), whole, at 80x24 and in a 60x18 pane.
// Without rules it is NewBossIntro's card.
func TestBossCardRules(t *testing.T) {
	retry := "a fail or :q! burns 20 cycles and deals a new boss door (22 left)"
	for _, k := range append(slices.Clone(game.BossKeys), game.FinalBoss) {
		br := BossRules{Twist: true, Price: retry}
		if k == game.FinalBoss {
			br = BossRules{Twist: true, Price: "one shot: a fail or :q! ends the boot", OneShot: true}
		}
		for _, size := range [][2]int{{80, 24}, {60, 18}} {
			fs := frames(NewBossCard(k, 3, false, br), size[0], size[1])
			last := ansi.Strip(fs[len(fs)-1])
			flat := strings.Join(strings.Fields(strings.ReplaceAll(last, "║", " ")), " ")
			want := []string{flavor.Story("boss.rule."+k, 3), br.Price}
			if br.OneShot && size[1] >= 24 {
				want = append(want, flavor.Story("boss.oneshot", 3))
			}
			for _, w := range want {
				if !strings.Contains(flat, strings.Join(strings.Fields(w), " ")) {
					t.Errorf("%s at %v: card misses %q:\n%s", k, size, w, last)
				}
			}
			for i, l := range strings.Split(last, "\n") {
				if ansi.StringWidth(l) > size[0] {
					t.Errorf("%s at %v: row %d is %d wide", k, size, i, ansi.StringWidth(l))
				}
			}
		}
	}
	plain := ansi.Strip(strings.Join(frames(NewBossIntro("heisenbug", 3, false), 80, 24), "\n"))
	if strings.Contains(plain, flavor.Story("boss.rule.heisenbug", 3)[:20]) || strings.Contains(plain, "cycles") {
		t.Errorf("NewBossIntro says rules it was not given:\n%s", plain)
	}
}
