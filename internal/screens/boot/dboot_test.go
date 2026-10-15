package boot

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens/boot/fixture"
)

// D36: a door card never shows the engine's clear chance, at any width;
// it is shown after the door.
func TestDoorsHideClearChance(t *testing.T) {
	for _, st := range []func() *Doors{
		func() *Doors { return NewDoors(env(fixture.Doors())) },
		func() *Doors { return NewDoors(env(fixture.DoorsFork())) },
		func() *Doors { return NewDoors(env(fixture.Boss())) },
	} {
		for _, sz := range fitSizes {
			v := ansi.Strip(st().View(sz[0], sz[1]))
			if strings.Contains(v, "% clear") || strings.Contains(v, "≈") {
				t.Fatalf("door cards at %dx%d show a clear chance:\n%s", sz[0], sz[1], v)
			}
		}
	}
}

// The White Knight's block shows over the first daemon offers, is reported
// once drawn so the app marks it seen, waits when the window is too short
// for it, and stays away once seen.
func TestMergeDaemonIntro(t *testing.T) {
	m := NewMerge(env(fixture.Merge()))
	if v := ansi.Strip(m.View(80, 24)); strings.Contains(v, "daemon.intro") || m.Shown() != nil {
		t.Errorf("the intro squeezed into a full 80x24 window (shown %v):\n%s", m.Shown(), v)
	}
	if v := ansi.Strip(m.View(140, 40)); !strings.Contains(v, "Stub block for daemon.intro") {
		t.Errorf("no intro over the first daemon offers:\n%s", v)
	}
	if got := m.Shown(); len(got) != 1 || got[0] != "daemon.intro" {
		t.Errorf("Shown = %v after drawing the intro", got)
	}
	e := env(fixture.Merge())
	game.MarkSeen(&e.Meta, "daemon.intro")
	if v := ansi.Strip(NewMerge(e).View(140, 40)); strings.Contains(v, "daemon.intro") {
		t.Errorf("the intro came back once seen:\n%s", v)
	}
}

// The boss card carries the boss's entrance line under its name.
func TestBossCardEntrance(t *testing.T) {
	v := ansi.Strip(NewDoors(env(fixture.Boss())).View(80, 24))
	name := strings.Index(v, "H E I S E N B U G")
	enter := strings.Index(v, "Stub line for boss.enter.heisenbug.")
	if name < 0 || enter < name {
		t.Errorf("no entrance line under the boss's name:\n%s", v)
	}
}
