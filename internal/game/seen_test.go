package game_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"katharsis/internal/game"
)

func TestMarkSeen(t *testing.T) {
	t.Parallel()
	m := game.Meta{Boots: 3}
	if game.HasSeen(&m, "story.prologue") {
		t.Fatal("a fresh Meta has seen the prologue")
	}
	if !game.MarkSeen(&m, "story.prologue") {
		t.Fatal("first MarkSeen reported a repeat")
	}
	m.Boots = 4
	if game.MarkSeen(&m, "story.prologue") {
		t.Error("second MarkSeen recorded the scene again")
	}
	if game.MarkSeen(&m, "") {
		t.Error("MarkSeen recorded an empty id")
	}
	game.MarkSeen(&m, "story.first-merge")
	want := []game.SeenScene{{ID: "story.prologue", Boot: 3}, {ID: "story.first-merge", Boot: 4}}
	if !slices.Equal(m.Seen, want) {
		t.Errorf("Seen = %+v, want %+v", m.Seen, want)
	}
	if !game.HasSeen(&m, "story.first-merge") || game.HasSeen(&m, "story.first-crash") {
		t.Errorf("HasSeen disagrees with Seen = %+v", m.Seen)
	}
}

// The store keeps Meta as JSON, so the seen set needs no schema change: it
// round-trips, and a save from before the field reads as nothing seen.
func TestSeenRoundTripsAsJSON(t *testing.T) {
	t.Parallel()
	m := game.Meta{Boots: 7}
	game.MarkSeen(&m, "reveal.prestige.1")
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var back game.Meta
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(back.Seen, m.Seen) {
		t.Errorf("round trip: %+v, want %+v", back.Seen, m.Seen)
	}
	var old game.Meta
	if err := json.Unmarshal([]byte(`{"Boots": 12, "Rating": 1100}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Seen != nil || game.HasSeen(&old, "story.prologue") {
		t.Errorf("an old save has seen %+v", old.Seen)
	}
}

// TestMarkOpened: a module is recorded once, in the order first opened,
// and a save from before Opened reads as nothing opened (Meta is JSON).
func TestMarkOpened(t *testing.T) {
	t.Parallel()
	var m game.Meta
	if !game.MarkOpened(&m, "intro-dp") || !game.MarkOpened(&m, "bfs") {
		t.Fatal("first MarkOpened reported a repeat")
	}
	if game.MarkOpened(&m, "intro-dp") || game.MarkOpened(&m, "") {
		t.Error("MarkOpened recorded a repeat or an empty id")
	}
	if want := []string{"intro-dp", "bfs"}; !slices.Equal(m.Opened, want) {
		t.Errorf("Opened = %v, want %v", m.Opened, want)
	}
	raw, err := json.Marshal(game.Meta{Boots: 2})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"Opened":`) {
		t.Errorf("an empty Opened is written: %s", raw)
	}
	var old game.Meta
	if err := json.Unmarshal([]byte(`{"Boots": 12, "TrialsPassed": ["intro-dp"]}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Opened != nil {
		t.Errorf("an old save has opened %v", old.Opened)
	}
}
