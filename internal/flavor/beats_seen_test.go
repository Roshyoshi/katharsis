package flavor

import (
	"testing"

	"katharsis/internal/game"
)

// Seen ids that are no beats (the boot screens' one-time tips) do not count
// as the newest beat: a tip shown in a dry boot must not put off the
// milestone that fills the gap.
func TestNextBeatGapIgnoresTips(t *testing.T) {
	m := seenAt(1, map[string]int{"story.prologue": 1})
	m.Seen = append(m.Seen, game.SeenScene{ID: "tip.regression", Boot: 9})
	m.Rated, m.Boots = 12, 10
	if got := next(crashed(10, 0), m, DefaultBeatGap); got != "milestone.doors.10" {
		t.Errorf("a tip in boot 9 held the milestone back: got %q", got)
	}
}
