package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	dir, name := filepath.Dir(path), filepath.Base(path)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("session: watch %s: %w", dir, err)
	}
	defer func() { _ = w.Close() }()
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
			// the backend misses the replacement's Create; the stat below
			// drops it when nothing replaced the file. Chmod alone is not a
			// save.
			if filepath.Base(ev.Name) == name && ev.Has(save) {
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
			// The file is moved to the backup before the new one is
			// written; with no new one yet, there is nothing to judge.
			if _, err := os.Stat(path); err == nil {
				onChange()
			}
		}
	}
}
