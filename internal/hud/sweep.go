package hud

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// debugDirPrefix names the working directories :Debug makes under the
// temp dir (openDebugger); closing the debugger removes its own.
const debugDirPrefix = "katharsis-debug-"

// debugDirAge is how old a debug dir must be for SweepDebugDirs to take it
// for a leftover. The dirs carry no owner, and a debugger still open may be
// running a file-I/O program in one, so only age tells.
const debugDirAge = time.Hour

// SweepDebugDirs removes the debug dirs that a game or practice HUD killed
// before it could close its debugger left behind (kill -9, a crash), once
// they are older than debugDirAge. Best effort, like the run dir sweep:
// what it misses only takes space.
func SweepDebugDirs() {
	sweepDebugDirs(os.TempDir(), time.Now())
}

func sweepDebugDirs(dir string, now time.Time) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), debugDirPrefix) {
			continue
		}
		if fi, err := e.Info(); err == nil && now.Sub(fi.ModTime()) > debugDirAge {
			os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}
