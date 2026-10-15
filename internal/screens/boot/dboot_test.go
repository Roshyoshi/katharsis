package boot

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

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
