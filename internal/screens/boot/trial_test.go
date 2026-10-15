package boot

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot/fixture"
)

// The Trial on the boot screens (docs/design/achievements.md (f)): its
// exit locked behind ⚔ ??? once the hint was met, open at a full pool,
// its cursed one-shot door, and the boot it ends, awake.

func TestTrialGoldens(t *testing.T) {
	for _, c := range []struct {
		name string
		make func() screens.Screen
	}{
		{"trial_route_locked", func() screens.Screen { return NewRoute(env(fixture.MapRouteTrial(false))) }},
		{"trial_route_open", func() screens.Screen { return NewRoute(env(fixture.MapRouteTrial(true))) }},
		{"trial_door", func() screens.Screen { return NewDoors(env(fixture.TrialDoor())) }},
		{"trial_awake", func() screens.Screen {
			e := env(fixture.TrialAwake())
			e.Meta = fixture.ClearedMeta()
			return NewCleared(e)
		}},
	} {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			v := c.make().View(sz[0], sz[1])
			checkFits(t, name, v, sz[0], sz[1])
			golden(t, name, ansi.Strip(v)+"\n")
		}
	}
}

// Locked, the exit has no name anywhere, the map's graph included: ⚔ ???
// and the gate's terms. Open, its card says what the route costs: cursed,
// one shot, the boot's end, no prestige unlock (tone rule 5). Before the
// hint the engine leaves the exit out, and the map draws no such node.
func TestTrialRoute(t *testing.T) {
	for _, sz := range goldenSizes {
		locked := ansi.Strip(NewRoute(env(fixture.MapRouteTrial(false))).View(sz[0], sz[1]))
		if !strings.Contains(locked, game.LockedFull) || !strings.Contains(locked, "???") ||
			strings.Contains(locked, "Trial") || strings.Contains(locked, "Court") {
			t.Errorf("locked at %v:\n%s", sz, locked)
		}
		open := ansi.Strip(NewRoute(env(fixture.MapRouteTrial(true))).View(sz[0], sz[1]))
		for _, want := range []string{bossLabel(game.TrialBoss), courtOfHearts, "a fail ends the boot", "cursed", "retreat: a new cursed door", "ends the boot here", "no prestige unlock"} {
			if !strings.Contains(open, want) {
				t.Errorf("open at %v lacks %q:\n%s", sz, want, open)
			}
		}
		st := fixture.MapRouteTrial(false)
		ex := st.Biomes[st.Biome].Exits
		for i := range ex {
			if ex[i].Node == rules.TrialNode {
				st.Biomes[st.Biome].Exits = append(ex[:i:i], ex[i+1:]...)
				break
			}
		}
		if v := ansi.Strip(NewRoute(env(st)).View(sz[0], sz[1])); strings.Contains(v, "???") || strings.Contains(v, "Trial") {
			t.Errorf("before the hint at %v:\n%s", sz, v)
		}
	}
	// A locked secret exit takes no key: enter does nothing, nor t.
	r := NewRoute(env(fixture.MapRouteTrial(false)))
	for i, e := range r.exits() {
		if e.Node != rules.TrialNode {
			continue
		}
		r.sel = i
		for _, k := range []string{"enter", "t"} {
			if _, cmd := r.Update(press(k)); cmd != nil {
				t.Errorf("%s on the locked Trial emits %#v", k, cmd())
			}
		}
	}
}

// The boot the Trial ends is awake, not cleared, and its screen says no
// prestige: the ending unlocks none.
func TestTrialAwake(t *testing.T) {
	for _, sz := range goldenSizes {
		e := env(fixture.TrialAwake())
		e.Meta = fixture.ClearedMeta()
		v := ansi.Strip(NewCleared(e).View(sz[0], sz[1]))
		if !strings.Contains(v, "Boot #42 · awake") || strings.Contains(v, "cleared ·") ||
			strings.Contains(v, "prestige unlocked") || strings.Contains(v, "then a clear") {
			t.Errorf("awake at %v:\n%s", sz, v)
		}
	}
}
