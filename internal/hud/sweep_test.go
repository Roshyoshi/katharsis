package hud

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A debugger dir left by a HUD that was killed goes once it is an hour
// old; a fresh one may belong to a debugger still open, and other files in
// the temp dir are not the HUD's.
func TestSweepDebugDirs(t *testing.T) {
	tmp := t.TempDir()
	now := time.Now()
	mk := func(name string, age time.Duration, dir bool) string {
		p := filepath.Join(tmp, name)
		var err error
		if dir {
			err = os.Mkdir(p, 0o700)
		} else {
			err = os.WriteFile(p, nil, 0o600)
		}
		if err == nil {
			err = os.Chtimes(p, now.Add(-age), now.Add(-age))
		}
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	old := mk(debugDirPrefix+"123", 2*time.Hour, true)
	fresh := mk(debugDirPrefix+"456", time.Minute, true)
	other := mk("katharsis-run-789", 2*time.Hour, true)
	file := mk(debugDirPrefix+"file", 2*time.Hour, false)
	sweepDebugDirs(tmp, now)
	for p, want := range map[string]bool{old: false, fresh: true, other: true, file: true} {
		if _, err := os.Stat(p); (err == nil) != want {
			t.Errorf("%s there = %v, want %v", filepath.Base(p), err == nil, want)
		}
	}
}
