package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Lock is a game process's claim on a store file. Readers (a `top` pane,
// sqlite3) need none, but two games on one store each resume the same boot
// and checkpoint their own copy of it over the other's: a door one clears,
// the other's next Tick un-clears. The claim is an flock(2) on <db>.lock,
// so it goes with the process however that ends, kill -9 included, and the
// file names its holder for the error the next game shows.
type Lock struct{ f *os.File }

// RunningError is the error Acquire returns while another game holds the
// lock.
type RunningError struct {
	Path string // the store file
	Pid  int    // 0 when the holder had not written itself in yet
	Pane string // the holder's $TMUX_PANE, "" when unknown
}

func (e *RunningError) Error() string {
	pid, pane := "?", e.Pane
	if e.Pid > 0 {
		pid = strconv.Itoa(e.Pid)
	}
	if pane == "" {
		pane = "?"
	}
	// main prefixes "katharsis: ", which makes it "katharsis: already
	// running (pid …)".
	return fmt.Sprintf("already running (pid %s, pane %s) on %s; quit that game first", pid, pane, e.Path)
}

// LockPath is the lock file of the store at path.
func LockPath(path string) string { return path + ".lock" }

// Acquire takes the game's lock on the store at path, without waiting: a
// lock held by another process is a *RunningError. Hold it from before Open
// until the game exits.
func Acquire(path string) (*Lock, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(LockPath(abs), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		defer f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, holder(f, abs)
		}
		return nil, fmt.Errorf("store: lock %s: %w", LockPath(abs), err)
	}
	// Who holds it, for the next game's error. The lock is the flock, not
	// this text, so a failed write only makes that error vaguer.
	if f.Truncate(0) == nil {
		_, _ = f.WriteAt([]byte(fmt.Sprintf("%d %s\n", os.Getpid(), os.Getenv("TMUX_PANE"))), 0)
	}
	return &Lock{f: f}, nil
}

// holder reads the lock file's "pid pane" line.
func holder(f *os.File, path string) *RunningError {
	e := &RunningError{Path: path}
	b := make([]byte, 128)
	n, _ := f.ReadAt(b, 0)
	fields := strings.Fields(string(b[:n]))
	if len(fields) > 0 {
		e.Pid, _ = strconv.Atoi(fields[0])
	}
	if len(fields) > 1 {
		e.Pane = fields[1]
	}
	return e
}

// Close gives the lock up. The process exiting does the same.
func (l *Lock) Close() error { return l.f.Close() }
