package app

import (
	"bufio"
	"context"
	"database/sql"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/screens"
	"katharsis/internal/session"
	"katharsis/internal/store"
)

// A save that fails stays on screen until one succeeds. As a flash, the
// next key cleared it, and a store the game could not write played a whole
// boot and kept none of it.
func TestSaveFailureStaysUntilASaveWorks(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	// Another writer records an attempt this game never saw, so the store
	// refuses this game's snapshot as stale (store.ErrStaleBoot) until its
	// own history catches up.
	ahead := *h.m.state
	ahead.History = append(append([]game.Attempt(nil), h.m.state.History...), *cur)
	if err := h.st.Checkpoint(h.m.meta, &ahead); err != nil {
		t.Fatal(err)
	}
	banner := func() bool { return strings.Contains(h.m.View().Content, "save failed") }

	h.verdict = "WA" // costs a life: a step that saves
	h.send(submitMsg{})
	if !banner() {
		t.Fatal("no save error after a refused checkpoint")
	}
	h.send(key("j"))
	h.send(key("k"))
	if !banner() {
		t.Fatal("save error gone after a key; nothing was saved")
	}
	h.verdict = "AC" // the door closes: the history catches up, the save works
	h.send(submitMsg{})
	if banner() || h.m.saveErr != nil {
		t.Fatalf("save error %v still shown after a save that worked", h.m.saveErr)
	}
}

const holdLockEnv = "KATHARSIS_TEST_HOLD_LOCK"

// TestHoldLockChild is not a test: holdWriteLock runs the test binary
// with holdLockEnv set to a store path, and this holds that store's write
// lock until killed.
func TestHoldLockChild(t *testing.T) {
	path := os.Getenv(holdLockEnv)
	if path == "" {
		t.Skip("helper process for holdWriteLock")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		os.Exit(2)
	}
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		os.Exit(2)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		os.Exit(2)
	}
	os.Stdout.WriteString("locked\n")
	// Until killed. A bare select {} has the runtime call it a deadlock
	// and exit, freeing the lock, as soon as it runs.
	time.Sleep(time.Hour)
}

// holdWriteLock has another process take the store's write lock, as an
// sqlite3 shell in BEGIN IMMEDIATE or a backup would, until the test ends
// or release is called. A connection in this process would not do: a
// write waiting on it ran past busy_timeout when tried.
func holdWriteLock(t *testing.T, path string) (release func()) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHoldLockChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), holdLockEnv+"="+path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "locked\n" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("lock holder said %q, %v", line, err)
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
	}
	t.Cleanup(release)
	return release
}

// A Tick that changes nothing saves nothing. Each one saved, so while
// another process held the store's write lock the UI froze for the busy
// timeout (5 s) on every tick (qa/chaos r08). A Tick that does change
// something still saves.
func TestIdleTickDoesNotWrite(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	release := holdWriteLock(t, h.path)
	// onTick, not a tickMsg: Update re-arms the 1 s tick before it saves,
	// so with 5 s saves the next tick is due before the last returns, and
	// the harness would chase them forever.
	start := time.Now()
	for range 3 {
		h.clock.t = h.clock.t.Add(time.Second)
		h.run(h.m.onTick())
	}
	if d := time.Since(start); d > 2*time.Second || h.m.saveErr != nil {
		t.Fatalf("3 idle ticks under a held lock took %v, save error %v; want no writes", d, h.m.saveErr)
	}
	release()

	h.clock.t = cur.Deadline
	h.send(tickMsg(h.clock.t))
	boots, err := h.st.Boots(1)
	if err != nil {
		t.Fatal(err)
	}
	if h.m.state.Phase != game.PhaseCrashed || boots[0].Outcome != store.BootCrashed {
		t.Fatalf("phase %q, stored outcome %q after the deadline tick, want crashed in both", h.m.state.Phase, boots[0].Outcome)
	}
}

// A save that failed is retried by a later Tick even when the Tick changed
// nothing, so a store that comes back catches up without waiting for the
// player's next move.
func TestTickRetriesAFailedSave(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	h.m.saved(store.ErrStaleBoot) // as if the last step's save failed
	h.clock.t = h.clock.t.Add(time.Second)
	h.send(tickMsg(h.clock.t))
	if h.m.saveErr == nil {
		t.Fatal("a tick 1 s after a failed save retried it; want it to wait saveRetry")
	}
	h.clock.t = h.clock.t.Add(saveRetry)
	h.send(tickMsg(h.clock.t))
	if h.m.saveErr != nil {
		t.Fatalf("save error %v after a tick past saveRetry, want the retry to have saved", h.m.saveErr)
	}
}

// An editor pane that dies as it opens is a command that could not start.
// The problem screen used to say only "editor closed · e reopens it"
// (qa/chaos r13); it now names the command.
func TestEditorThatCannotStart(t *testing.T) {
	tmux := privateTmux(t)
	dead := tmux("new-window", "-P", "-F", "#{pane_id}", "--", "true")
	for deadline := time.Now().Add(5 * time.Second); session.PaneAlive(dead); {
		if time.Now().After(deadline) {
			t.Fatal("pane running true never exited")
		}
		time.Sleep(20 * time.Millisecond)
	}
	r := &TmuxRunner{Editor: "nosuchvim -p", pane: dead, opened: time.Now()}
	if got := r.EditorFailed(); !strings.Contains(got, "nosuchvim -p") {
		t.Errorf("EditorFailed = %q, want it to name the command", got)
	}
	r.opened = time.Now().Add(-time.Minute)
	if got := r.EditorFailed(); got != "" {
		t.Errorf("EditorFailed = %q for an editor that ran a minute, want none", got)
	}

	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &failedEditorRunner{hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: t.TempDir()}}}
	})
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	h.send(hudQuitMsg{})
	if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "exited at once: nosuchvim") {
		t.Errorf("problem screen after the editor died at once:\n%s", v)
	}
}

type failedEditorRunner struct{ hudFakeRunner }

func (r *failedEditorRunner) EditorFailed() string {
	return "the editor exited at once: nosuchvim (see --editor)"
}
