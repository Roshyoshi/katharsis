package store_test

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/store"
)

// One game per store: a second Acquire fails at once and names the
// holder, and the lock is free again once released.
func TestLockOneGame(t *testing.T) {
	t.Setenv("TMUX_PANE", "%7")
	path := tempPath(t)
	l, err := store.Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Acquire(path)
	var running *store.RunningError
	if !errors.As(err, &running) {
		t.Fatalf("second Acquire = %v, want a RunningError", err)
	}
	if running.Pid != os.Getpid() || running.Pane != "%7" {
		t.Errorf("holder pid %d pane %q, want %d %%7", running.Pid, running.Pane, os.Getpid())
	}
	if msg := err.Error(); !strings.Contains(msg, "already running (pid ") || !strings.Contains(msg, "pane %7") {
		t.Errorf("error %q does not say who runs", msg)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	l, err = store.Acquire(path)
	if err != nil {
		t.Fatalf("Acquire after Close: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

const lockEnv = "KATHARSIS_TEST_LOCK_CHILD"

// A game killed with -9 cannot release anything; the kernel drops its
// flock, so the next game starts.
func TestLockGoesWithTheProcess(t *testing.T) {
	if path := os.Getenv(lockEnv); path != "" {
		if _, err := store.Acquire(path); err != nil {
			os.Exit(2)
		}
		os.Stdout.WriteString("locked\n")
		// Until killed. A bare select {} has the runtime call it a
		// deadlock and exit, freeing the lock, as soon as it runs.
		time.Sleep(time.Hour)
	}
	path := tempPath(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockGoesWithTheProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), lockEnv+"="+path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 7)
	if _, err := out.Read(buf); err != nil || string(buf) != "locked\n" {
		t.Fatalf("child said %q, %v", buf, err)
	}
	var running *store.RunningError
	if _, err := store.Acquire(path); !errors.As(err, &running) || running.Pid != cmd.Process.Pid {
		t.Fatalf("Acquire while the child holds it = %v, want the child's pid %d", err, cmd.Process.Pid)
	}
	_ = cmd.Process.Signal(syscall.SIGKILL)
	_ = cmd.Wait()
	l, err := store.Acquire(path)
	if err != nil {
		t.Fatalf("Acquire after the holder was killed: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

// A second game that resumed the same boot and missed the first one's
// clear holds a snapshot with fewer attempts than the store. Saving it
// would put the cleared door back and drop the bytes; it is refused whole.
func TestCheckpointRefusesAStaleSnapshot(t *testing.T) {
	t.Parallel()
	s := openClocked(t, tempPath(t), t0)
	meta, st := sampleMeta(), midBoot(1, t0)
	stale := *st
	stale.History = append([]game.Attempt(nil), st.History...)
	cur := *st.Current
	stale.Current = &cur
	checkpoint(t, s, meta, st)

	a := *st.Current
	a.Closed, a.Outcome = a.Opened.Add(10*time.Minute), game.Cleared
	st.History, st.Current, st.Phase = append(st.History, a), nil, game.PhaseDoors
	meta.Bank += 500
	checkpoint(t, s, meta, st)
	before := see(t, s)

	staleMeta := sampleMeta()
	if err := s.Checkpoint(staleMeta, &stale); !errors.Is(err, store.ErrStaleBoot) {
		t.Fatalf("stale Checkpoint = %v, want ErrStaleBoot", err)
	}
	same(t, "store after the stale checkpoint", see(t, s), before)
}
