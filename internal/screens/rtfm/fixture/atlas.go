package fixture

import (
	_ "embed"
	"encoding/json"

	"katharsis/internal/curriculum"
)

// atlasJSON is the full USACO Guide's atlas (curriculum.Atlas) with USACO
// and CSES problems judgeable, as the guide snapshot of KATHARSIS_GUIDE
// lays it out; rtfm's TestAtlasFixture writes it with -update. The titles,
// ids and problem lists are the guide's, under CC BY-NC-SA 4.0 (see
// internal/curriculum/testdata/guide/NOTICE).
//
//go:embed atlas.json
var atlasJSON []byte

// Atlas is the full guide's map, for the world map's goldens: the fixture
// snapshot has a module or two per division, too few to fold a path.
func Atlas() curriculum.Atlas {
	var a curriculum.Atlas
	if err := json.Unmarshal(atlasJSON, &a); err != nil {
		panic("fixture atlas: " + err.Error())
	}
	return a
}
