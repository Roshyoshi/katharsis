package boot

import (
	"fmt"
	"strings"
	"time"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/setuid"
	"katharsis/internal/ui"
)

// The setuid bits on the boot screens (docs/design/achievements.md (h)):
// the strip's line at the next safe point after a bit sets, and one line
// in the cleared screen's kept block. Never inside a door (GR6), never on
// a crash (GR3): the app says a crash's bits at the next GRUB instead. No
// door draw reads Meta.Setuid (GR7).
//
//	[ 3851.0] judge: ✓ AC "Wormhole Sort" in 24:10 of 32:00 · +192 B
//	[ 3851.0] chmod u+s /bin/two-twins /sbin/ditto-ditto
//	          Ditto, ditto: two bits where the gap was.

// Strip event kinds the bits add.
const (
	EvSetuid = "setuid" // bits set, said at a safe point
	EvCap    = "cap"    // the cycle pool at its cap for the first time ever
)

// chmodShown is how many bits one chmod line names before it counts the
// rest.
const chmodShown = 2

// Chmod is bits as the strip's verb says them: "chmod u+s /bin/two-twins
// /sbin/ditto-ditto", past two "… and 2 more · find / -perm -4000".
func Chmod(bits []game.SetuidBit) string {
	var paths []string
	for _, b := range bits[:min(len(bits), chmodShown)] {
		paths = append(paths, setuidPath(b.ID))
	}
	s := "chmod u+s " + strings.Join(paths, " ")
	if more := len(bits) - chmodShown; more > 0 {
		s += fmt.Sprintf(" and %d more%sfind / -perm -4000", more, ui.G.Sep)
	}
	return s
}

// setuidPath is bit id's path, its id for one the table lacks.
func setuidPath(id string) string {
	if d, ok := setuid.Lookup(id); ok {
		return d.Path()
	}
	return id
}

// SetuidEvent is the strip's event for bits set at now: the chmod line,
// and as its flavor line the first bit's that has one (setuid.<id>).
func SetuidEvent(bits []game.SetuidBit, seed uint64, now time.Time) Event {
	ev := Event{At: now, Kind: EvSetuid, Text: Chmod(bits), Seed: seed}
	for _, b := range bits {
		if h := flavor.SetuidHook(b.ID); fl.Story(h, seed) != "" {
			ev.Hook = h
			break
		}
	}
	return ev
}

// capEvent is the strip's line the first time any boot's pool reaches
// its cap on a map (Meta.Capped names the boot): hint step 3 of the Trial
// (achievements.md (f)), said whatever setuid_notices says, as the story's
// own line. Its flavor is the cap's bit's, so a notice of that bit in the
// same moment says it once.
func capEvent(st *game.State, now time.Time) Event {
	return Event{At: now, Kind: EvCap, Seed: st.Seed,
		Text: "cycles at the cap: " + Cycles(st),
		Hook: flavor.SetuidHook(".two-miles-high")}
}

// setuidKept is the cleared screen's kept line for the bits the boot set,
// "setuid +2 · snicker-snack, finds-none", or "" for none or with the
// notices off. A backfill's bits are git log's, not this boot's.
func setuidKept(env Env) string {
	if !env.SetuidNotices {
		return ""
	}
	var ids []string
	for _, b := range env.Meta.Setuid {
		if b.Boot == env.State.Boot && !b.Backfill {
			ids = append(ids, b.ID)
		}
	}
	if len(ids) == 0 {
		return ""
	}
	names := strings.Join(ids[:min(len(ids), chmodShown)], ", ")
	if more := len(ids) - chmodShown; more > 0 {
		names += fmt.Sprintf(" and %d more", more)
	}
	return ui.Warn.Render(ui.G.Star) + " " + ui.Strong.Render(fmt.Sprintf("setuid +%d", len(ids))) +
		ui.Muted.Render(ui.G.Sep) + names
}
