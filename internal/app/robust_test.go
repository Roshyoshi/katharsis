package app

import (
	"strings"
	"testing"

	"katharsis/internal/game"
	"katharsis/internal/screens"
)

// A save that fails stays on screen until one succeeds. As a flash, the
// next key cleared it, and a store the game could not write played a whole
// boot and kept none of it.
func TestSaveFailureStaysUntilASaveWorks(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	// Another writer records an attempt this game never saw, so the store
	// refuses this game's snapshot as stale (store.ErrStaleBoot) until its
	// own history catches up.
	ahead := *h.m.state
	ahead.History = append(append([]game.Attempt(nil), h.m.state.History...), *cur)
	if err := h.st.Checkpoint(h.m.meta, &ahead); err != nil {
		t.Fatal(err)
	}
	banner := func() bool { return strings.Contains(h.m.View().Content, "save failed") }

	h.verdict = "WA" // costs a life: a step that saves
	h.send(submitMsg{})
	if !banner() {
		t.Fatal("no save error after a refused checkpoint")
	}
	h.send(key("j"))
	h.send(key("k"))
	if !banner() {
		t.Fatal("save error gone after a key; nothing was saved")
	}
	h.verdict = "AC" // the door closes: the history catches up, the save works
	h.send(submitMsg{})
	if banner() || h.m.saveErr != nil {
		t.Fatalf("save error %v still shown after a save that worked", h.m.saveErr)
	}
}
