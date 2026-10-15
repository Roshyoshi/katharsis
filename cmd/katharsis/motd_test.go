package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/store"
)

var motdNow = time.Date(2026, 10, 15, 9, 0, 0, 0, time.Local)

// motdStore is a store with n ended boots and m as the player; the last
// boot is on today's nightly seed when nightly is set.
func motdStore(t *testing.T, n int, m game.Meta, nightly bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "katharsis.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := motdNow.Add(-time.Duration(n) * time.Hour)
	for i := 1; i <= n; i++ {
		st := &game.State{
			Boot: i, Seed: uint64(i), Rules: rules.Standard(), Started: start.Add(time.Duration(i) * time.Minute),
			Phase: game.PhaseCrashed, Crash: &game.Crash{Cause: game.TimedOut, At: start.Add(time.Duration(i)*time.Minute + 30*time.Second)},
		}
		if nightly && i == n {
			st.Seed = game.NightlySeed(motdNow)
		}
		m.Boots = i
		if err := s.Checkpoint(m, st); err != nil {
			t.Fatal(err)
		}
	}
	if n == 0 {
		if err := s.SaveMeta(m); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestMotd(t *testing.T) {
	due := game.Meta{
		CoreDump:    &game.CoreDump{Problem: "usaco:1", Bytes: 183},
		Regressions: []game.Regression{{Problem: "a", Due: motdNow.Add(-time.Hour)}, {Problem: "b", Due: motdNow.Add(-time.Minute)}, {Problem: "c", Due: motdNow.Add(time.Hour)}},
	}
	for _, tc := range []struct {
		name    string
		path    func(t *testing.T) string
		want    string
		nothing bool
	}{
		{"no store yet", func(t *testing.T) string { return filepath.Join(t.TempDir(), "none.db") }, "", true},
		{"nothing due", func(t *testing.T) string { return motdStore(t, 3, game.Meta{}, true) }, "", true},
		{"nightly not played", func(t *testing.T) string { return motdStore(t, 3, game.Meta{}, false) }, "katharsis: nightly 10-15", false},
		{"everything", func(t *testing.T) string { return motdStore(t, 3, due, false) },
			"katharsis: 2 regressions due · ☠ 183 B core dump to upsolve · nightly 10-15", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := motd(tc.path(t), motdNow)
			if err != nil || got != tc.want {
				t.Errorf("motd = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// motd never writes the store, and reads a 1,000-boot one in well under
// the 50 ms a new shell can spare (RT14).
func TestMotdReadOnlyAndFast(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 1,000-boot store")
	}
	path := motdStore(t, 1000, game.Meta{CoreDump: &game.CoreDump{Problem: "usaco:1", Bytes: 9}}, false)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stat, _ := os.Stat(path)
	best := time.Hour
	for range 5 {
		t0 := time.Now()
		line, err := motd(path, motdNow)
		best = min(best, time.Since(t0))
		if err != nil || !strings.Contains(line, "core dump") {
			t.Fatalf("motd = %q, %v", line, err)
		}
	}
	if best > 50*time.Millisecond {
		t.Errorf("motd took %v on 1,000 boots, want under 50 ms", best)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if st2, _ := os.Stat(path); !bytes.Equal(before, after) || !st2.ModTime().Equal(stat.ModTime()) {
		t.Error("motd changed the store")
	}
}

func TestRunMotdUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runMotd([]string{"extra"}, &out, &errOut); code != 2 || out.Len() != 0 {
		t.Errorf("code %d out %q", code, out.String())
	}
}

func TestSettings(t *testing.T) {
	dir := t.TempDir()
	if s, err := loadSettings(filepath.Join(dir, "none.toml")); err != nil || s != defaultSettings() || s.SessionBudget.Duration != 3*time.Hour {
		t.Errorf("missing file: %+v, %v", s, err)
	}
	for _, tc := range []struct {
		body string
		want time.Duration
		bad  bool
	}{
		{`session_budget = "90m"`, 90 * time.Minute, false},
		{`session_budget = "0s"`, 0, false},
		{`session_budget = "-1h"`, 0, true},
		{`sesion_budget = "1h"`, 0, true},
		{"setuid_spoilers = \"hints\"", 3 * time.Hour, false},
		{"setuid_spoilers = \"some\"", 0, true},
		{"setuid_notices = false", 3 * time.Hour, false},
	} {
		path := filepath.Join(dir, fmt.Sprintf("s%d.toml", len(tc.body)))
		if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
			t.Fatal(err)
		}
		s, err := loadSettings(path)
		if (err != nil) != tc.bad || !tc.bad && s.SessionBudget.Duration != tc.want {
			t.Errorf("%s: %+v, %v", tc.body, s, err)
		}
	}
	// The setuid bits' two (achievements.md (g), (h)): notices on and no
	// spoilers by default.
	if d := defaultSettings(); !d.SetuidNotices || d.SetuidSpoilers != "none" {
		t.Errorf("defaults: %+v", d)
	}
	path := filepath.Join(dir, "setuid.toml")
	if err := os.WriteFile(path, []byte("setuid_notices = false\nsetuid_spoilers = \"all\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err := loadSettings(path); err != nil || s.SetuidNotices || s.SetuidSpoilers != "all" {
		t.Errorf("setuid settings: %+v, %v", s, err)
	}
}
