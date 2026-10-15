package game_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"katharsis/internal/game"
	"katharsis/internal/rules"
)

func TestCFPrior(t *testing.T) {
	for cf, want := range map[int]float64{
		1: 800, 650: 800, 800: 800, 1234: 1234, 1900: 1900, 3500: 3500, 3900: 3500,
	} {
		if got := game.CFPrior(cf); got != want {
			t.Errorf("CFPrior(%d) = %v, want %v", cf, got, want)
		}
	}
}

func TestSeedPrior(t *testing.T) {
	r := rules.Standard()
	fresh := func() game.Meta { return game.Meta{Rating: r.ColdStartRating} }

	m := fresh()
	if !game.NeedsPrior(&m) {
		t.Fatal("a fresh player is not asked")
	}
	game.SeedPrior(&m, 1450, t0)
	if m.Rating != 1450 || !reflect.DeepEqual(m.Prior, &game.Prior{CF: 1450, Seeded: 1450, At: t0}) {
		t.Errorf("after 1450: rating %v prior %+v", m.Rating, m.Prior)
	}
	if game.NeedsPrior(&m) {
		t.Error("asked again after an answer")
	}
	// The seed is what doors aim at, in every topic, until topics have
	// doors of their own; the cold K still applies.
	if got := game.PlayerRating(&r, &m, "graphs"); got != 1450 {
		t.Errorf("PlayerRating after the seed = %v, want 1450", got)
	}
	if m.Rated != 0 {
		t.Errorf("Rated %d after a seed, want 0 (cold K stays)", m.Rated)
	}

	m = fresh()
	game.SeedPrior(&m, 0, t0)
	if m.Rating != r.ColdStartRating || m.Prior == nil || m.Prior.CF != 0 || game.NeedsPrior(&m) {
		t.Errorf("after a skip: rating %v prior %+v, needs %v", m.Rating, m.Prior, game.NeedsPrior(&m))
	}

	// A player with rated doors is never asked, nor one from before
	// Meta.Rated (topic attempts).
	for _, m := range []game.Meta{{Rated: 3}, {TopicAttempts: map[string]int{"graphs": 2}}} {
		if game.NeedsPrior(&m) {
			t.Errorf("asked a player with rated doors: %+v", m)
		}
	}

	m = fresh()
	game.SeedPrior(&m, 2100, t0)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var back game.Meta
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, m) {
		t.Errorf("JSON round trip: %+v, %v; want %+v", back, err, m)
	}
	raw, _ = json.Marshal(fresh())
	var unasked game.Meta
	if json.Unmarshal(raw, &unasked) != nil || unasked.Prior != nil {
		t.Error("an unasked Meta does not round-trip to a nil Prior")
	}
}
