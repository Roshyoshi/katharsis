package game_test

import (
	"testing"
	"time"

	"katharsis/internal/game"
)

func TestNightlySeed(t *testing.T) {
	pdt := time.FixedZone("PDT", -7*3600)
	day := time.Date(2026, 10, 14, 0, 0, 0, 0, pdt)
	for _, tc := range []struct {
		t    time.Time
		want uint64
	}{
		{day, 0x20261014},
		{day.Add(23*time.Hour + 59*time.Minute), 0x20261014},
		{day.Add(24 * time.Hour), 0x20261015},
		// The date is the one in t's location: 23:30 PDT is already the
		// 15th in UTC, but the player's nightly is still the 14th's.
		{time.Date(2026, 10, 14, 23, 30, 0, 0, pdt), 0x20261014},
		{time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), 0x20270101},
	} {
		if got := game.NightlySeed(tc.t); got != tc.want {
			t.Errorf("NightlySeed(%v) = %#x, want %#x", tc.t, got, tc.want)
		}
	}
	// A boot is a nightly on its own start's day, and never on seed 0.
	s := &game.State{Seed: 0x20261014, Started: time.Date(2026, 10, 14, 23, 30, 0, 0, pdt)}
	if !s.Nightly() {
		t.Error("a boot on its day's seed is no nightly")
	}
	if s.Started = s.Started.Add(time.Hour); s.Nightly() {
		t.Error("a boot started the next day is a nightly of the day before")
	}
}
