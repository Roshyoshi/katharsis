package boot

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens/boot/fixture"
)

// TestMapHidesTheCourt: the map draws the Court's node only where the
// engine put its exit on the route, and a locked one without its name
// (the engine lane's guard until the UI lane draws the Court's cards).
func TestMapHidesTheCourt(t *testing.T) {
	view := func(exit *game.Exit) string {
		st := fixture.MapRoutePrestige() // the route into Boss II
		if exit != nil {
			st.Biomes[4].Exits = append(st.Biomes[4].Exits, *exit)
		}
		return ansi.Strip(NewHub(hubEnv(st), true).View(140, 40))
	}
	trial := game.Exit{Node: rules.TrialNode, Name: "The Court", Topic: game.Mixed, Rooms: 1, Boss: game.TrialBoss,
		Hidden: true, Ending: game.EndingWake, Gate: game.Gate{Full: true}, Locked: game.LockedFull}
	if v := view(nil); strings.Contains(v, "The Court") || strings.Contains(v, "???") {
		t.Errorf("an unlisted Court is on the map:\n%s", v)
	}
	if v := view(&trial); strings.Contains(v, "The Court") || !strings.Contains(v, "???") {
		t.Errorf("a hinted Court is not ??? on the map:\n%s", v)
	}
	trial.Locked = ""
	if v := view(&trial); !strings.Contains(v, "The Court") {
		t.Errorf("an open Court is not on the map:\n%s", v)
	}
}
