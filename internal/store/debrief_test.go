package store_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/store"
)

// endedBoots checkpoints boots 1..len(ends) as they ended: "crashed",
// "cleared" or "abandoned", an hour apart from t0, on preset.
func endedBoots(t *testing.T, s *store.Store, preset string, ends ...string) {
	t.Helper()
	m := sampleMeta()
	for i, end := range ends {
		st := midBoot(i+1, t0.Add(time.Duration(i)*time.Hour))
		st.Rules.Name = preset
		st.Committed = int64(100 * (i + 1))
		switch end {
		case "crashed":
			crash(st, game.TimedOut)
		case "cleared":
			clearBoot(st)
		}
		m.Boots = st.Boot
		checkpoint(t, s, m, st)
		if end == "abandoned" {
			if err := s.AbandonBoot(st.Boot); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestEndQueries(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0)
	endedBoots(t, s, "standard", "crashed", "cleared", "abandoned", "crashed")

	// Committed: 100, 320 (the clear banks its 120 B) and 400; the
	// abandoned boot's 300 does not count.
	if got, err := s.MedianCommitted(10); err != nil || got != 320 {
		t.Errorf("MedianCommitted = %d, %v; want 320", got, err)
	}
	if got, err := s.MedianCommitted(2); err != nil || got != 360 {
		t.Errorf("MedianCommitted(2) = %d, %v; want (400+320)/2", got, err)
	}
	// Uptimes: crashes at 60 min (the open door closes 20 min after it
	// opened at 40), the clear at 65 min.
	if got, err := s.MedianUptime("standard", 10); err != nil || got != time.Hour {
		t.Errorf("MedianUptime = %v, %v; want 1h", got, err)
	}
	if got, err := s.MedianUptime("short", 10); err != nil || got != 0 {
		t.Errorf("MedianUptime(short) = %v, %v; want 0, no boots", got, err)
	}
	if best, ok, err := s.BestClear("standard", 1, 9); err != nil || !ok || best != 65*time.Minute {
		t.Errorf("BestClear = %v %v %v; want 1h05", best, ok, err)
	}
	if _, ok, err := s.BestClear("standard", 1, 2); err != nil || ok {
		t.Errorf("BestClear before the clear: ok %v, %v", ok, err)
	}
	if _, ok, err := s.BestClear("standard", 2, 9); err != nil || ok {
		t.Errorf("BestClear at another prestige: ok %v, %v", ok, err)
	}
	// Boot 2's last door was no boss; mark one by hand in a new boot.
	st := midBoot(5, t0.Add(5*time.Hour))
	st.Current.Door.Boss = "heisenbug"
	clearBoot(st)
	checkpoint(t, s, sampleMeta(), st)
	if got, err := s.BossClearedBefore("heisenbug", 5); err != nil || got {
		t.Errorf("BossClearedBefore(5) = %v, %v; want false: boot 5 is the first", got, err)
	}
	if got, err := s.BossClearedBefore("heisenbug", 6); err != nil || !got {
		t.Errorf("BossClearedBefore(6) = %v, %v; want true", got, err)
	}

	// Every boot here has its own seed; boot 2 cleared, so its seed's best
	// is the clear.
	seed := midBoot(2, t0).Seed
	if r, ok, err := s.SeedBest(seed); err != nil || !ok || !r.Cleared || r.Boots != 1 || r.Doors != 3 {
		t.Errorf("SeedBest = %+v %v %v", r, ok, err)
	}
	if _, ok, err := s.SeedBest(7); err != nil || ok {
		t.Errorf("SeedBest of an unplayed seed: ok %v, %v", ok, err)
	}
	if played, err := s.Played(seed); err != nil || !played {
		t.Errorf("Played = %v, %v", played, err)
	}
}

// Boots and upsolving on the day count; a boot the day before does not.
func TestPlayToday(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0)
	m := sampleMeta()
	yesterday := crash(midBoot(1, t0.Add(-24*time.Hour)), game.TimedOut)
	today := crash(midBoot(2, t0.Add(time.Hour)), game.TimedOut)
	m.Boots = 2
	checkpoint(t, s, m, yesterday)
	checkpoint(t, s, m, today)
	if _, err := s.Event(store.Event{At: t0.Add(3 * time.Hour), Kind: store.EventUpsolveClose,
		Data: map[string]any{"elapsed_ms": (15 * time.Minute).Milliseconds()}}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.PlayToday(t0.Add(3*time.Hour + 30*time.Minute)); err != nil || got != 75*time.Minute {
		t.Errorf("PlayToday = %v, %v; want 1h of boot + 15m upsolving", got, err)
	}
}

// OpenReadOnly reads what the game wrote and never writes: no file made
// where there is none, the bytes untouched where there is one.
func TestOpenReadOnly(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "none", "katharsis.db")
	if _, err := store.OpenReadOnly(missing); !errors.Is(err, store.ErrNoStore) {
		t.Errorf("missing file: %v, want ErrNoStore", err)
	}
	if _, err := os.Stat(filepath.Dir(missing)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("OpenReadOnly made the directory: %v", err)
	}

	path := tempPath(t)
	w := open(t, path)
	m := sampleMeta()
	checkpoint(t, w, m, crash(midBoot(3, t0), game.TimedOut))
	mustClose(t, w)
	before := fileBytes(t, path)
	r, err := store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.LoadMeta()
	if err != nil || got.Bank != m.Bank {
		t.Errorf("LoadMeta = %+v, %v", got.Bank, err)
	}
	if err := r.SaveMeta(got); err == nil {
		t.Error("SaveMeta on a read-only store succeeded")
	}
	mustClose(t, r)
	if after := fileBytes(t, path); !bytes.Equal(before, after) {
		t.Error("the read-only open changed the file")
	}
}

func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
