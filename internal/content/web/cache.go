package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// TempPrefix starts the name of every temp file and dir the sources make in
// a cache root, so CleanDebris can find what a killed process left.
const TempPrefix = ".tmp-"

// WriteFile replaces p with data atomically: a temp file in p's dir,
// synced to disk, then renamed over p. A reader never sees half a file,
// and a crash leaves the old file or the new one.
func WriteFile(p string, data []byte) error {
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, TempPrefix+filepath.Base(p)+"-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }() // a no-op after the rename
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}

// SyncDir puts everything under dir on disk, for a caller about to publish
// dir by renaming it into place: without it a power cut soon after the
// rename can leave a published dir of empty files, which a cache then
// trusts for good. Each entry gets a plain fsync and dir a File.Sync last.
// On darwin that last one is F_FULLFSYNC, which flushes the drive's cache
// for everything written before it; one per file would cost its ~5 ms
// each (40 test files: 6 ms in all against 150-260 ms, measured).
func SyncDir(dir string) error {
	err := filepath.WalkDir(dir, func(p string, _ fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		return syncPath(p, false)
	})
	if err != nil {
		return err
	}
	return syncPath(dir, true)
}

func syncPath(p string, full bool) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	if full {
		err = f.Sync()
	} else {
		err = syscall.Fsync(int(f.Fd()))
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// DebrisAge is how old a temp entry must be before CleanDebris removes it:
// older than any download still running, so another process's work in
// progress is left alone.
const DebrisAge = 6 * time.Hour

// CleanDebris removes the entries directly under root whose names start
// with one of prefixes and that are older than DebrisAge. A source calls it
// when it opens: the game is killed with downloads in flight (tmux kills
// its pane), and a half-fetched problem can be hundreds of MB. A missing
// root has nothing to clean; a failed removal is tried again next time.
func CleanDebris(root string, prefixes ...string) {
	des, _ := os.ReadDir(root)
	before := time.Now().Add(-DebrisAge)
	for _, d := range des {
		name := d.Name()
		if !hasAnyPrefix(name, prefixes) {
			continue
		}
		if fi, err := d.Info(); err == nil && fi.ModTime().Before(before) {
			_ = os.RemoveAll(filepath.Join(root, name))
		}
	}
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
