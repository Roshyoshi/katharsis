package main

import (
	"strings"
	"testing"

	"katharsis/internal/ui"
)

// A positional argument used to be ignored: `katharsis pratice dir` booted
// the game (and downloaded the catalogs) instead of failing.
func TestRunGameRejectsArgs(t *testing.T) {
	// Should the check regress, fail at the tmux lookup instead of
	// exec'ing tmux over the test.
	t.Setenv("TMUX", "")
	t.Setenv("PATH", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	err := runGame([]string{"pratice", "dir"})
	if err == nil || !strings.Contains(err.Error(), `"pratice"`) {
		t.Fatalf("runGame(pratice dir) = %v, want an unknown command error", err)
	}
}

// NO_COLOR takes the color out of what the game draws itself, the tmux
// border included; a non-UTF-8 locale picks the ASCII glyphs.
func TestPickUI(t *testing.T) {
	defer func() { ui.SetGlyphs(ui.Unicode); ui.SetPalette(ui.ANSI) }()
	env := func(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }
	pickUI(env(map[string]string{"LANG": "en_US.UTF-8"}))
	if got := ui.Danger.Tmux("x"); got != "#[fg=colour1]x#[default]" || ui.G.Fail != "✗" {
		t.Errorf("default: Danger.Tmux = %q, Fail %q", got, ui.G.Fail)
	}
	pickUI(env(map[string]string{"NO_COLOR": "1", "LANG": "C"}))
	if got := ui.Danger.Tmux("x"); got != "x" || ui.G.Fail != "x" {
		t.Errorf("NO_COLOR, LANG=C: Danger.Tmux = %q, Fail %q", got, ui.G.Fail)
	}
}
