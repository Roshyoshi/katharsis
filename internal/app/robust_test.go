package app

import (
	"bufio"
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/judge"
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

// A game that quits while a submit is judged (ctrl+c, a kill) charged no
// verdict: a failing submit was dodged, on a cursed door too (qa/chaos
// r05). The submit is noted and saved when it is made, and the next game
// judges it again, as of when it was made: a door that ran out while the
// game was closed is still saved by an AC sent in time.
func TestQuitMidJudgeJudgesOnResume(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	h.stall = make(chan struct{})
	h.clock.t = cur.Deadline.Add(-time.Minute)
	submitted := h.clock.t
	h.send(submitMsg{})
	if at := h.m.state.Current.SubmitAt; !at.Equal(submitted) {
		t.Fatalf("SubmitAt = %v, want %v", at, submitted)
	}
	saved, err := h.st.ActiveBoot()
	if err != nil || !saved.Current.SubmitAt.Equal(submitted) {
		t.Fatalf("saved SubmitAt = %v (%v), want it saved before the verdict", saved.Current.SubmitAt, err)
	}
	h.m.quit() // ctrl+c mid-judge
	close(h.stall)
	h.stall = nil

	h.clock.t = cur.Deadline.Add(time.Hour)
	h.restart()
	if h.m.state.Phase == game.PhaseCrashed {
		t.Fatalf("resumed into a crash %+v; the submit was in time", h.m.state.Crash)
	}
	last := h.m.state.History[len(h.m.state.History)-1]
	if last.Outcome != game.Cleared || !last.Closed.Equal(submitted) || !last.SubmitAt.IsZero() {
		t.Fatalf("door %q closed %v SubmitAt %v; want cleared as of %v", last.Outcome, last.Closed, last.SubmitAt, submitted)
	}
}

// A failing submit judged again on resume costs its life, and the door,
// whose time ran out while the game was closed, then times out.
func TestQuitMidJudgeChargesTheFailure(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	lives := h.m.state.Lives
	h.stall = make(chan struct{})
	h.send(submitMsg{})
	h.m.quit()
	close(h.stall)
	h.stall, h.verdict = nil, "WA"

	h.clock.t = cur.Deadline.Add(time.Hour)
	h.restart()
	h.send(tickMsg(h.clock.t))
	if h.m.state.Lives != lives-1 {
		t.Errorf("lives %d, want %d: the failure went uncharged", h.m.state.Lives, lives-1)
	}
	last := h.m.state.History[len(h.m.state.History)-1]
	if last.Submits != 1 || last.Failed != 1 || last.Outcome != game.TimedOut {
		t.Errorf("door %q with %d submits, %d failed; want 1 WA, then timed out", last.Outcome, last.Submits, last.Failed)
	}
}

// A judge error is no verdict: nothing is left to judge again on resume.
func TestJudgeErrorClearsThePendingSubmit(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	h.stall = make(chan struct{})
	close(h.stall) // the judge returns an error at once
	h.send(submitMsg{})
	if c := h.m.state.Current; h.m.judging || !c.SubmitAt.IsZero() {
		t.Fatalf("judging %v SubmitAt %v after a judge error", h.m.judging, c.SubmitAt)
	}
}

// Closing vim mid-judge quits the HUD but not its submit: the clock stays
// held for it, and its verdict, arriving after, is charged.
func TestClosedEditorKeepsTheSubmit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/sol.cpp", []byte("int main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: dir}}
	})
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	lives := h.m.state.Lives
	req, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	h.send(req)
	h.send(hudQuitMsg{}) // :q in vim
	if !h.m.judging {
		t.Fatal("closing the editor released the clock held for the submit")
	}
	h.clock.t = cur.Deadline.Add(10 * time.Second)
	h.send(tickMsg(h.clock.t))
	if h.m.state.Phase != game.PhaseProblem {
		t.Fatalf("phase %q inside the grace, want problem", h.m.state.Phase)
	}
	h.send(hud.SubmittedMsg{Summary: judge.Summary{Verdict: judge.WA, Passed: 1, Total: 4, Done: 4, First: 1}})
	if h.m.state.Lives != lives-1 {
		t.Errorf("lives %d, want %d after the verdict", h.m.state.Lives, lives-1)
	}
}

// submitDir is a door directory a HUD can judge: sol.cpp holding src, and
// one test whose input and answer are 2.
func submitDir(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"sol.cpp": src, "tests/1.in": "2\n", "tests/1.out": "2\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const (
	waSrc = "#include <cstdio>\nint main() { puts(\"1\"); }\n"
	acSrc = "#include <cstdio>\nint main() { int x; scanf(\"%d\", &x); printf(\"%d\\n\", x); }\n"
)

// stalled is a submit profile whose compiler hangs until the pass is
// cancelled: the HUD's submit runs until the door closes, and a test
// delivers the verdict itself.
func stalled(t *testing.T) judge.Profile {
	t.Helper()
	cc := filepath.Join(t.TempDir(), "cc")
	if err := os.WriteFile(cc, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return judge.Profile{Name: "stalled", Compiler: cc}
}

// hudHarness runs the app with a HUD judging dir with prof, its submits
// copied aside as the tmux runner's are. The door closes when the test
// ends, so no pass outlives it.
func hudHarness(t *testing.T, dir string, prof judge.Profile) *harness {
	t.Helper()
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: dir, SubmitProfile: prof,
			SubmitCopy: filepath.Join(dir, ".submit", "sol.cpp")}}
	})
	t.Cleanup(func() { h.m.d.Runner.Close() })
	return h
}

// answer is req's reply if it has one already.
func answer(reply <-chan session.Reply) (session.Reply, bool) {
	select {
	case r := <-reply:
		return r, true
	default:
		return session.Reply{}, false
	}
}

// A second submit while one is judged is refused at once, and the first
// one's verdict is charged: a WA, then the fix sent before its verdict,
// costs the WA's life. The second used to replace the first, which was
// never charged, so the WA cost nothing.
func TestSecondSubmitCannotDodgeTheFirst(t *testing.T) {
	if _, err := exec.LookPath(judge.Compiler); err != nil {
		t.Skipf("%s not on PATH", judge.Compiler)
	}
	dir := submitDir(t, waSrc)
	h := hudHarness(t, dir, judge.Profile{})
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	lives := h.m.state.Lives

	// Forced: the samples fail too, and :Submit alone would ask first.
	first, firstReply := session.NewRequest(session.Command{Cmd: session.CmdSubmit, Args: []string{session.ArgForce}})
	_, c1 := h.m.Update(first) // pumped below: the judge takes a while
	if err := os.WriteFile(filepath.Join(dir, "sol.cpp"), []byte(acSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	second, secondReply := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	_, c2 := h.m.Update(second)
	if rep, ok := answer(secondReply); !ok || rep.OK || rep.Text != hud.SubmitBusy {
		t.Errorf("second submit answered %v with %+v, want %q at once", ok, rep, hud.SubmitBusy)
	}
	h.pump(tea.Batch(c1, c2), func() bool { return !h.m.judging })

	if rep, _ := answer(firstReply); rep.OK || !strings.HasPrefix(rep.Text, "WA 0/1") {
		t.Errorf("first submit answered %+v, want its WA", rep)
	}
	if h.m.state.Lives != lives-1 {
		t.Errorf("lives %d, want %d: the first submit's WA went uncharged", h.m.state.Lives, lives-1)
	}
	if c := h.m.state.Current; c == nil || c.Submits != 1 || c.Failed != 1 {
		t.Errorf("attempt %+v, want the door open with 1 submit, 1 failed", c)
	}
}

// A refused submit leaves the one in flight as it was: the clock held for
// it, and its SubmitAt, saved for a resume, still when it was made. The s
// key's refusal shows over the game, as :Submit's does in vim.
func TestRefusedSubmitKeepsTheClock(t *testing.T) {
	h := hudHarness(t, submitDir(t, waSrc), stalled(t))
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	h.clock.t = cur.Deadline.Add(-time.Minute)
	made := h.clock.t
	first, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit, Args: []string{session.ArgForce}})
	h.send(first)

	h.clock.t = made.Add(20 * time.Second)
	second, reply := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	h.send(second)
	if rep, ok := answer(reply); !ok || rep.OK || rep.Text != hud.SubmitBusy {
		t.Errorf(":Submit answered %v with %+v, want %q at once", ok, rep, hud.SubmitBusy)
	}
	h.send(submitMsg{}) // the s key
	if h.m.flash != hud.SubmitBusy {
		t.Errorf("s key flashed %q, want %q", h.m.flash, hud.SubmitBusy)
	}
	saved, err := h.st.ActiveBoot()
	if err != nil {
		t.Fatal(err)
	}
	if at, sat := h.m.state.Current.SubmitAt, saved.Current.SubmitAt; !h.m.judging || !at.Equal(made) || !sat.Equal(made) {
		t.Fatalf("judging %v, SubmitAt %v (saved %v); want held, as of %v", h.m.judging, at, sat, made)
	}
	h.clock.t = cur.Deadline.Add(10 * time.Second)
	h.send(tickMsg(h.clock.t))
	if h.m.state.Phase != game.PhaseProblem {
		t.Fatalf("phase %q inside the grace, want problem: the clock is held", h.m.state.Phase)
	}
}

// A submit judged again on resume cannot be cancelled by a new one: the
// :Submit is refused, the resumed judge keeps its time, and its verdict is
// charged. Resubmitting right after a resume replaced it, which dodged
// the failure the resume was there to charge (qa/chaos r05).
func TestResumedJudgeCannotBeReplaced(t *testing.T) {
	h := hudHarness(t, submitDir(t, waSrc), stalled(t))
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	lives := h.m.state.Lives
	first, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit, Args: []string{session.ArgForce}})
	h.send(first)
	h.m.quit() // ctrl+c mid-judge

	h.clock.t = cur.Deadline.Add(-time.Minute)
	h.restart()
	resumed := h.m.resumed
	if !h.m.judging || resumed.at.IsZero() {
		t.Fatalf("judging %v resumed %+v: the resume should judge the submit again", h.m.judging, resumed)
	}
	h.clock.t = h.clock.t.Add(5 * time.Second)
	again, reply := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	h.send(again)
	if rep, ok := answer(reply); !ok || rep.OK || rep.Text != hud.SubmitBusy {
		t.Errorf(":Submit answered %v with %+v, want %q at once", ok, rep, hud.SubmitBusy)
	}
	if h.m.resumed != resumed || !h.m.state.Current.SubmitAt.Equal(resumed.at) {
		t.Fatalf("resumed %+v SubmitAt %v, want %+v kept", h.m.resumed, h.m.state.Current.SubmitAt, resumed)
	}
	h.send(hud.SubmittedMsg{Summary: judge.Summary{Verdict: judge.WA, Total: 1, Done: 1, First: 0}})
	if h.m.state.Lives != lives-1 {
		t.Errorf("lives %d, want %d: the resumed WA went uncharged", h.m.state.Lives, lives-1)
	}
}
