package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/rules"
	"katharsis/internal/screens/meta"
)

// newPromptHarness is the harness with the game's prompts on, as
// cmd/katharsis runs it.
func newPromptHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.promptRestart()
	return h
}

func (h *harness) promptRestart() {
	h.t.Helper()
	d := h.m.d
	d.Prompts = true
	m, err := New(d)
	if err != nil {
		h.t.Fatal(err)
	}
	h.m = m
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m.Init())
}

func TestPriorAskedOnceAndSeeds(t *testing.T) {
	h := newPromptHarness(t)
	if _, ok := h.m.top().(*meta.Prior); !ok {
		t.Fatalf("first launch shows %T, want the rating question", h.m.top())
	}
	for _, k := range []string{"1", "6", "2", "0"} {
		h.send(key(k))
	}
	h.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, ok := h.m.top().(*meta.Menu); !ok {
		t.Fatalf("after the answer the top is %T, want GRUB", h.m.top())
	}
	if h.m.meta.Rating != 1620 || h.m.meta.Prior == nil || h.m.meta.Prior.CF != 1620 {
		t.Fatalf("rating %v prior %+v after answering 1620", h.m.meta.Rating, h.m.meta.Prior)
	}
	saved, err := h.st.LoadMeta()
	if err != nil || saved.Prior == nil || saved.Rating != 1620 {
		t.Fatalf("saved meta %+v, %v", saved, err)
	}
	h.promptRestart()
	if _, ok := h.m.top().(*meta.Menu); !ok {
		t.Fatalf("second launch shows %T, want GRUB", h.m.top())
	}
}

func TestPriorSkip(t *testing.T) {
	h := newPromptHarness(t)
	h.send(key("9"))
	h.send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if h.m.meta.Rating != rules.Standard().ColdStartRating || h.m.meta.Prior == nil || h.m.meta.Prior.CF != 0 {
		t.Fatalf("after a skip: rating %v prior %+v", h.m.meta.Rating, h.m.meta.Prior)
	}
	h.promptRestart()
	if _, ok := h.m.top().(*meta.Menu); !ok {
		t.Fatalf("a skipped question came back: top %T", h.m.top())
	}
}

// A player who has rated doors already is not asked: the store has better
// than any answer.
func TestPriorNotAskedOfAPlayer(t *testing.T) {
	h := newHarness(t)
	h.m.meta.Rated = 4
	if err := h.st.SaveMeta(h.m.meta); err != nil {
		t.Fatal(err)
	}
	h.promptRestart()
	if _, ok := h.m.top().(*meta.Menu); !ok {
		t.Fatalf("a rated player sees %T, want GRUB", h.m.top())
	}
}
