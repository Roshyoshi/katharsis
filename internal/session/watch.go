package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch calls onChange, debounced, whenever the file at path is saved. It
// returns nil once ctx is done, or an error if the watch breaks (the
// directory goes away, or the backend fails).
//
// It watches the parent directory, not the file: vim's default save renames
// the original to path~ and writes a new file (under $TMPDIR, which
// 'backupskip' exempts, it writes in place), and other editors rename a temp
// file over it, so a watch on the original inode goes quiet after the first
// save. Other names in the directory (swap files, vim's 4913 probe, the
// backup) are ignored. On macOS (kqueue) the watch holds a descriptor per
// directory entry, so keep the solution in a small directory.
//
// onChange runs on Watch's goroutine, never concurrently and never after
// Watch returns; it should hand slow work (a judge run) to another goroutine.
func Watch(ctx context.Context, path string, debounce time.Duration, onChange func()) error {
	return watch(ctx, path, debounce, onChange, nil)
}

// watch is Watch plus ready, called once the directory is watched, so tests
// do not race their first write against setup.
func watch(ctx context.Context, path string, debounce time.Duration, onChange, ready func()) error {
	path = filepath.Clean(path)
	name := filepath.Base(path)
	// The file is moved to the backup before the new one is written; with
	// no new one yet, there is nothing to judge.
	there := func() bool { _, err := os.Stat(path); return err == nil }
	return watchDir(ctx, filepath.Dir(path), func(n string) bool { return n == name }, there,
		debounce, onChange, ready)
}

// WatchTests calls onChange, debounced, whenever a test in dir (1.in,
// 1.out, the player's c1.in) is saved, added or removed, so a case the
// player adds is judged as a save of the source would judge it. Other
// names (swap files, backups such as c1.in~) are ignored; like Watch it
// returns nil once ctx is done, and an error if the watch breaks.
func WatchTests(ctx context.Context, dir string, debounce time.Duration, onChange func()) error {
	return watchTests(ctx, dir, debounce, onChange, nil)
}

func watchTests(ctx context.Context, dir string, debounce time.Duration, onChange, ready func()) error {
	test := func(n string) bool { return strings.HasSuffix(n, ".in") || strings.HasSuffix(n, ".out") }
	// A test removed changes the tests as much as one saved.
	always := func() bool { return true }
	return watchDir(ctx, filepath.Clean(dir), test, always, debounce, onChange, ready)
}

// watchDir watches dir for saves of the names match takes, and calls
// onChange once they settle if saved still holds.
func watchDir(ctx context.Context, dir string, match func(name string) bool, saved func() bool,
	debounce time.Duration, onChange, ready func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("session: watch %s: %w", dir, err)
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		return fmt.Errorf("session: watch %s: %w", dir, err)
	}
	if ready != nil {
		ready()
	}

	// One save is several events (rename, create, writes, chmod); the timer
	// restarts on each and fires once they stop.
	settle := time.NewTimer(debounce)
	settle.Stop()
	defer settle.Stop()

	const save = fsnotify.Create | fsnotify.Write | fsnotify.Rename | fsnotify.Remove
	for {
		select {
		case <-ctx.Done():
			return nil

		case ev, ok := <-w.Events:
			if !ok {
				return fmt.Errorf("session: watch %s: closed", dir)
			}
			if ev.Name == dir && ev.Has(fsnotify.Remove|fsnotify.Rename) {
				return fmt.Errorf("session: watch %s: directory went away", dir)
			}
			// A rename or remove of the file arms the timer too, in case
			// the backend misses the replacement's Create; saved drops it
			// when nothing replaced the file. Chmod alone is not a save.
			if match(filepath.Base(ev.Name)) && ev.Has(save) {
				settle.Reset(debounce)
			}

		case err, ok := <-w.Errors:
			if !ok {
				return fmt.Errorf("session: watch %s: closed", dir)
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				settle.Reset(debounce) // a dropped event may have been a save
				continue
			}
			return fmt.Errorf("session: watch %s: %w", dir, err)

		case <-settle.C:
			if ctx.Err() != nil {
				return nil // select picks at random when both are ready
			}
			if saved() {
				onChange()
			}
		}
	}
}
