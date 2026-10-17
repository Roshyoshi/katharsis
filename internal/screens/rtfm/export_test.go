package rtfm

import (
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
)

// NewMapOf is a map over a, without the curriculum behind it, so goldens
// can draw the full guide's atlas from the fixture. A level opened on it
// has no curriculum to read.
func NewMapOf(a curriculum.Atlas, m game.Meta) *Map { return newMap(nil, a, m, nil, nil) }
