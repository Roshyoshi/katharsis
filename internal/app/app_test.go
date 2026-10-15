package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/reader"
	"katharsis/internal/session"
	"katharsis/internal/store"
)

// fakeCatalog serves the simulator's synthetic pool. Fetch returns no files
// unless docs is set, where it writes a USACO-shaped statement for the
// reader tests; the headless loop never reads them.
type fakeCatalog struct {
	all  []catalog.Problem
	byID map[catalog.ID]catalog.Problem
	docs string
}

func newFakeCatalog() *fakeCatalog {
	c := &fakeCatalog{all: sim.SyntheticPool(600, 7), byID: map[catalog.ID]catalog.Problem{}}
	for _, p := range c.all {
		c.byID[p.ID] = p
	}
	return c
}

func (c *fakeCatalog) All() []catalog.Problem { return c.all }
func (c *fakeCatalog) Get(id catalog.ID) (catalog.Problem, bool) {
	p, ok := c.byID[id]
	return p, ok
}
func (c *fakeCatalog) Fetch(_ context.Context, id catalog.ID) (catalog.Assets, error) {
	if c.docs == "" {
		return catalog.Assets{}, nil
	}
	p := c.byID[id]
	md := "# " + p.Title + "\n\n*USACO 2021 January, Bronze, Problem 1*\n\n" +
		"Bessie names this problem " + p.Title + " on purpose.\n"
	path := filepath.Join(c.docs, strings.ReplaceAll(string(id), ":", "_")+".md")
	return catalog.Assets{Statement: path}, os.WriteFile(path, []byte(md), 0o644)
}

// fakeRunner counts the doors it opened and closed; while openErr is set,
// every Open fails with it.
type fakeRunner struct {
	opened, closed int
	openErr        error
}

func (r *fakeRunner) Open(context.Context, Door) error {
	if r.openErr != nil {
		return r.openErr
	}
	r.opened++
	return nil
}
func (r *fakeRunner) Close() { r.closed++ }

// clock advances only when the test says so, so timers never expire by
// accident.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

// hudFakeRunner is a runner with a HUD, as the tmux runner has: the app
// then routes submits through the HUD instead of Deps.Judge. Like the tmux
// runner it makes a HUD from opt for each door and each reopen. With no
// problem in opt.Dir the HUD judges nothing; tests deliver its results.
type hudFakeRunner struct {
	*fakeRunner
	opt       hud.Options
	hud       *hud.Model
	reopenErr error
}

func (r *hudFakeRunner) Open(ctx context.Context, d Door) error {
	if err := r.fakeRunner.Open(ctx, d); err != nil {
		return err
	}
	r.hud = hud.New(r.opt)
	return nil
}

func (r *hudFakeRunner) Close() {
	r.fakeRunner.Close()
	if r.hud != nil {
		r.hud.Cleanup()
	}
	r.hud = nil
}

func (r *hudFakeRunner) HUD() *hud.Model { return r.hud }

func (r *hudFakeRunner) Reopen() error {
	if r.reopenErr != nil {
		return r.reopenErr
	}
	// As the tmux runner does: the new HUD takes over a submit in flight.
	prev := r.hud
	r.hud = hud.New(r.opt)
	if prev != nil {
		r.hud.Adopt(prev)
		prev.Cleanup()
	}
	return nil
}

type harness struct {
	t       *testing.T
	m       *Model
	st      *store.Store
	runner  *fakeRunner
	clock   *clock
	verdict string // what the fake judge returns
	// acFirst makes the next n submits AC whatever verdict says, so a
	// test can earn bytes before it starts failing.
	acFirst int
	// stall makes the judge hang until it is closed, like a slow full test
	// set; the test delivers the result itself.
	stall chan struct{}
	// path is the store file, for a test that opens it from outside.
	path string
}

func newHarness(t *testing.T) *harness { return newHarnessWith(t, nil) }

// newHarnessWith runs the app with the runner wrap makes around the
// harness's fakeRunner; nil wrap uses the fakeRunner itself.
func newHarnessWith(t *testing.T, wrap func(*fakeRunner) Runner) *harness {
	t.Helper()
	path := filepath.Join(t.TempDir(), "k.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // some tests close it early
	h := &harness{t: t, st: st, path: path, runner: &fakeRunner{}, clock: &clock{time.Date(2026, 10, 14, 18, 0, 0, 0, time.UTC)}, verdict: "AC"}
	var runner Runner = h.runner
	if wrap != nil {
		runner = wrap(h.runner)
	}
	cat := newFakeCatalog()
	cat.docs = t.TempDir()
	m, err := New(Deps{
		Store:   st,
		Catalog: cat,
		Runner:  runner,
		Judge: func(context.Context, Door) (string, int, int, error) {
			if h.stall != nil {
				<-h.stall
				return "", 0, 0, errors.New("stalled judge released")
			}
			if h.acFirst > 0 {
				h.acFirst--
				return "AC", 10, 10, nil
			}
			if h.verdict == "AC" {
				return "AC", 10, 10, nil
			}
			return h.verdict, 3, 10, nil
		},
		Now:  h.clock.now,
		Seed: func() uint64 { return 42 },
	})
	if err != nil {
		t.Fatal(err)
	}
	h.m = m
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m.Init())
	return h
}

// restart is the game started again on the same store, catalog and
// runner.
func (h *harness) restart() {
	h.t.Helper()
	d := h.m.d
	m, err := New(Deps{Store: d.Store, Catalog: d.Catalog, Runner: d.Runner, Judge: d.Judge, Now: d.Now})
	if err != nil {
		h.t.Fatal(err)
	}
	h.m = m
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m.Init())
}

// run executes a command and feeds its messages back, recursively. Commands
// that block (the 1 s tick, reveal animations) are abandoned after a short
// wait, the way a test clock would skip them.
func (h *harness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(30 * time.Millisecond):
		return
	}
	switch msg := msg.(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			h.run(c)
		}
	default:
		h.send(msg)
	}
}

func (h *harness) send(msg tea.Msg) {
	h.t.Helper()
	_, cmd := h.m.Update(msg)
	h.run(cmd)
	// Every state must render at the current size without breaking width.
	for _, l := range strings.Split(h.m.View().Content, "\n") {
		if w := ansi.StringWidth(l); w > h.m.width {
			h.t.Fatalf("line %d wide at width %d in %T: %q", w, h.m.width, h.m.top(), ansi.Strip(l))
		}
	}
	if h.m.flash != "" {
		h.t.Logf("flash: %s", h.m.flash)
	}
}

func key(k string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: rune(k[0]), Text: k} }

// play drives the boot with the simplest choices until it ends, returning
// the final phase.
func (h *harness) play() game.Phase {
	h.t.Helper()
	for i := 0; i < 400; i++ {
		s := h.m.state
		if s == nil {
			h.t.Fatal("no boot")
		}
		switch {
		case s.Phase == game.PhaseCleared || s.Phase == game.PhaseCrashed:
			return s.Phase
		case s.PendingUpgrade:
			h.send(game.PickUpgrade{Upgrade: s.UpgradeOffers[0]})
		case s.Phase == game.PhaseRoute:
			h.send(game.ChooseRoute{Index: 0})
		case s.Phase == game.PhaseDoors:
			h.send(game.ChooseDoor{Index: 0})
		case s.Phase == game.PhaseProblem:
			if h.m.open == nil {
				h.t.Fatalf("door %s open in the engine but not in the app", s.Current.Door.Problem)
			}
			h.clock.t = h.clock.t.Add(5 * time.Minute)
			h.send(submitMsg{})
		case s.Phase == game.PhaseMerge:
			if s.Uncommitted > 0 {
				h.send(game.Commit{})
			} else {
				h.send(game.LeaveMerge{})
			}
		default:
			h.t.Fatalf("unexpected phase %q", s.Phase)
		}
	}
	h.t.Fatal("boot did not end in 400 steps")
	return ""
}

// enterDoor plays the boot up to its next open door.
func (h *harness) enterDoor() *game.Attempt {
	h.t.Helper()
	for i := 0; h.m.state.Phase != game.PhaseProblem; i++ {
		if i > 20 {
			h.t.Fatalf("no door after 20 steps, phase %q", h.m.state.Phase)
		}
		switch {
		case h.m.state.PendingUpgrade:
			h.send(game.PickUpgrade{Upgrade: h.m.state.UpgradeOffers[0]})
		case h.m.state.Phase == game.PhaseRoute:
			h.send(game.ChooseRoute{})
		case h.m.state.Phase == game.PhaseMerge:
			h.send(game.LeaveMerge{})
		default:
			h.send(game.ChooseDoor{})
		}
	}
	return h.m.state.Current
}

// toDoors plays the boot up to a room of doors. Only a route stands
// between: any other phase refuses ChooseRoute, so it fails rather than
// loop forever (a merge window left with a pick still owed stays open).
func (h *harness) toDoors() {
	h.t.Helper()
	for i := 0; h.m.state.Phase != game.PhaseDoors; i++ {
		if i > 0 && h.m.state.Phase != game.PhaseRoute || i > 5 {
			h.t.Fatalf("no doors after %d routes, phase %q (pending upgrade %v)",
				i, h.m.state.Phase, h.m.state.PendingUpgrade)
		}
		h.send(game.ChooseRoute{})
	}
}

func TestFullClear(t *testing.T) {
	h := newHarness(t)
	if _, ok := h.m.top().(interface{ Editing() bool }); !ok {
		t.Fatalf("start screen is %T, want the GRUB menu", h.m.top())
	}
	// Prestige unlocks only for a calibrated player (rules.CalibAfter
	// rated doors); this one has played enough.
	h.m.meta.Rated = 100
	h.send(screens.StartBoot{Prestige: 0, Preset: "standard"})
	if got := h.play(); got != game.PhaseCleared {
		t.Fatalf("boot ended %q, want cleared", got)
	}
	if h.m.meta.Prestige != 1 {
		t.Errorf("prestige %d after a prestige-0 clear, want 1", h.m.meta.Prestige)
	}
	if h.runner.opened == 0 || h.runner.opened != h.runner.closed {
		t.Errorf("runner opened %d, closed %d", h.runner.opened, h.runner.closed)
	}
	boots, err := h.st.Boots(10)
	if err != nil || len(boots) != 1 || boots[0].Outcome != store.BootCleared {
		t.Fatalf("stored boots %+v, %v", boots, err)
	}
	// History screens open from the cleared screen's intents.
	h.send(screens.OpenLog{})
	h.send(screens.Back{})
	h.send(screens.OpenTop{})
}

func TestCrashThenUpsolveRecoversCoreDump(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.verdict, h.acFirst = "WA", 1 // earn bytes on door 1, then fail
	if got := h.play(); got != game.PhaseCrashed {
		t.Fatalf("boot ended %q, want crashed", got)
	}
	dump := h.m.meta.CoreDump
	if dump == nil {
		t.Fatal("crash left no core dump")
	}
	bank := h.m.meta.Bank
	// git log shows what the crash dumped.
	boots, err := h.st.Boots(1)
	if err != nil || len(boots) != 1 {
		t.Fatalf("stored boots %+v, %v", boots, err)
	}
	if row := h.m.bootRow(boots[0], nil); row.Dumped != dump.Bytes {
		t.Errorf("git log row dumped %d, want the core dump's %d", row.Dumped, dump.Bytes)
	}

	h.verdict = "AC"
	h.send(screens.Upsolve{Problem: dump.Problem})
	if h.m.open == nil || !h.m.open.Upsolve {
		t.Fatal("upsolve did not open the problem")
	}
	h.send(submitMsg{})
	if h.m.meta.CoreDump != nil {
		t.Error("core dump still there after an accepted upsolve")
	}
	if h.m.meta.Bank != bank+dump.Bytes {
		t.Errorf("bank %d, want %d + %d recovered", h.m.meta.Bank, bank, dump.Bytes)
	}
	if h.m.open != nil {
		t.Error("upsolve door still open")
	}
}

func TestTimerExpiryCrashes(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.clock.t = h.enterDoor().Deadline.Add(time.Second)
	h.send(tickMsg(h.clock.t))
	if h.m.state.Phase != game.PhaseCrashed || h.m.state.Crash.Cause != game.TimedOut {
		t.Fatalf("phase %q crash %+v, want a timeout crash", h.m.state.Phase, h.m.state.Crash)
	}
}

// A second app on the same store resumes the boot and reopens the door.
func TestResumeMidDoor(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	want := h.enterDoor().Door
	h.restart()
	if h.m.open == nil || !reflect.DeepEqual(h.m.open.Door, want) {
		t.Fatalf("resumed door %+v, want %+v", h.m.open, want)
	}
}

// A submit made before the deadline is judged after it; the engine's grace
// must cover the judge's run, so the clock may not crash the door while
// the result is on its way.
func TestSubmitJudgedPastDeadlineWithinGrace(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	h.stall = make(chan struct{})
	t.Cleanup(func() { close(h.stall) })
	h.send(submitMsg{})

	h.clock.t = cur.Deadline.Add(10 * time.Second)
	h.send(tickMsg(h.clock.t))
	if h.m.state.Phase != game.PhaseProblem {
		t.Fatalf("phase %q while the submit was judged inside the grace, want problem", h.m.state.Phase)
	}
	h.send(judgedMsg{verdict: "AC", passed: 10, total: 10})
	if last := h.m.state.History[len(h.m.state.History)-1]; last.Outcome != game.Cleared {
		t.Fatalf("door ended %q, want cleared", last.Outcome)
	}
}

// A judge that never answers must not stop the clock past the grace.
func TestLostJudgeCannotStopTheClock(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	h.stall = make(chan struct{})
	t.Cleanup(func() { close(h.stall) })
	h.send(submitMsg{})

	h.clock.t = cur.Deadline.Add(h.m.state.Rules.SubmitGrace.Duration)
	h.send(tickMsg(h.clock.t))
	if h.m.state.Phase != game.PhaseCrashed || h.m.state.Crash.Cause != game.TimedOut {
		t.Fatalf("phase %q crash %+v, want a timeout once the grace ran out", h.m.state.Phase, h.m.state.Crash)
	}
}

// :Submit from vim is how most submits arrive. Before the fix it reached
// the HUD without marking the door as judging, so a submit just before the
// deadline was crashed by the next tick instead of getting the grace.
func TestEditorSubmitGetsTheGrace(t *testing.T) {
	// The HUD needs a loadable problem, so its submit really runs: a
	// refused submit ends at once and rightly holds no clock.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sol.cpp"), []byte("int main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: dir}}
	})
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()

	req, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	h.send(req)
	h.clock.t = cur.Deadline.Add(10 * time.Second)
	h.send(tickMsg(h.clock.t))
	if h.m.state.Phase != game.PhaseProblem {
		t.Fatalf("phase %q after an editor submit inside the grace, want problem", h.m.state.Phase)
	}
	sum := judge.Summary{Verdict: judge.AC, Passed: 4, Total: 4, Done: 4, First: -1}
	h.send(hud.SubmittedMsg{Summary: sum})
	if last := h.m.state.History[len(h.m.state.History)-1]; last.Outcome != game.Cleared {
		t.Fatalf("door ended %q, want cleared", last.Outcome)
	}
}

// Editor commands answer in vim: a refused quack says why, and an accepted
// one says what the duck revealed.
func TestEditorQuackAnswers(t *testing.T) {
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: t.TempDir()}}
	})
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	ask := func() session.Reply {
		req, reply := session.NewRequest(session.Command{Cmd: session.CmdQuack})
		h.send(req)
		return <-reply
	}
	ducks := h.m.state.Ducks
	got := ask()
	if !got.OK || !strings.Contains(got.Text, "technique: ") && !strings.Contains(got.Text, "tags: ") {
		t.Fatalf("quack reply %+v, want what the duck revealed", got)
	}
	if h.m.state.Ducks != ducks-1 {
		t.Fatalf("ducks %d after a quack, want %d", h.m.state.Ducks, ducks-1)
	}
	h.m.state.Ducks = 0
	if got := ask(); got.OK || !strings.Contains(got.Text, "duck") || strings.Contains(got.Text, "invalid action") {
		t.Fatalf("quack with no ducks answered %+v, want the refusal", got)
	}
}

// A door whose editor failed to open is not open, so e retries it; it used
// to count as open with no editor and could only time out.
func TestFailedOpenRetries(t *testing.T) {
	h := newHarness(t)
	h.runner.openErr = errors.New("no tmux")
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	if h.m.open != nil || !strings.Contains(h.m.flash, "no tmux") {
		t.Fatalf("open %+v flash %q after a failed open, want no door and the error", h.m.open, h.m.flash)
	}
	h.runner.openErr = nil
	h.send(key("e"))
	if h.m.open == nil {
		t.Fatal("e did not retry the door")
	}
}

// A failed reopen leaves the editor marked closed, so e can try again; it
// used to clear the mark first, and the door had no editor and no retry.
func TestFailedReopenRetries(t *testing.T) {
	var hr *hudFakeRunner
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		hr = &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: t.TempDir()}}
		return hr
	})
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	h.send(hudQuitMsg{}) // the player quit vim
	hr.reopenErr = errors.New("tmux gone")
	h.send(key("e"))
	hr.reopenErr = nil
	before := hr.hud
	h.send(key("e"))
	if s := h.m.top().(*problemScreen); s.closed || hr.hud == before {
		t.Fatalf("closed %v, new HUD %v after a retried reopen, want the editor back", s.closed, hr.hud != before)
	}
}

// A fetch that lands after its door is gone opens nothing: here the player
// left an upsolve while its problem downloaded.
func TestLateFetchOpensNothing(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	h.send(game.GiveUp{})
	id := h.m.state.Crash.Door.Problem
	c := newGateCatalog(h.m.d.Catalog.(*fakeCatalog))
	c.gates[id] = make(chan struct{})
	t.Cleanup(func() { close(c.gates[id]) })
	h.m.d.Catalog = c
	opened := h.runner.opened

	h.send(screens.Upsolve{Problem: id}) // the fetch hangs
	if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "fetching") || strings.Contains(v, "e retries") {
		t.Errorf("upsolve while fetching shows %q, want fetching without e (the editorial key)", v)
	}
	h.send(key("Q"))
	h.send(fetchedMsg{door: game.Door{Problem: id}, upsolve: true})
	if h.m.open != nil || h.runner.opened != opened {
		t.Fatalf("open %+v, runner opened %d more after the upsolve was left, want none", h.m.open, h.runner.opened-opened)
	}
}

// A boot whose first save failed still starts, with the error shown; it
// used to sit behind the screen it was started from.
func TestUnsavedBootStillStarts(t *testing.T) {
	h := newHarness(t)
	if err := h.st.Close(); err != nil {
		t.Fatal(err)
	}
	h.send(screens.StartBoot{Preset: "standard"})
	if _, grub := h.m.top().(interface{ Editing() bool }); grub || !strings.Contains(h.m.View().Content, "save failed") {
		t.Fatalf("top %T, want the boot's screen and the save error", h.m.top())
	}
}

// An accepted upsolve that could not be saved still closes its door; left
// open, a second AC recorded the solve twice.
func TestUnsavedUpsolveClosesTheDoor(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	h.send(game.GiveUp{})
	h.send(screens.Upsolve{Problem: h.m.state.Crash.Door.Problem})
	if err := h.st.Close(); err != nil {
		t.Fatal(err)
	}
	h.send(submitMsg{})
	if h.m.open != nil || !strings.Contains(h.m.View().Content, "save failed") {
		t.Fatalf("open %+v, want the door closed and the save error", h.m.open)
	}
}

// The editorial a :q! offers counts against the upsolve that follows, even
// after the game restarts and the core dump is upsolved from GRUB.
func TestUpsolveAfterGiveUpIsEditorial(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.acFirst = 1 // bytes on door 1, so :q! on door 2 leaves a core dump
	h.enterDoor()
	h.send(submitMsg{})
	id := h.enterDoor().Door.Problem
	h.send(game.GiveUp{})
	if h.m.meta.CoreDump == nil || h.m.meta.CoreDump.Problem != id {
		t.Fatalf("core dump %+v, want one on %s", h.m.meta.CoreDump, id)
	}

	h.restart()
	h.send(screens.Upsolve{Problem: id})
	h.send(submitMsg{})
	last := h.m.meta.Solves[len(h.m.meta.Solves)-1]
	if last.Problem != id || !last.Upsolve || last.Kind != game.Editorial {
		t.Fatalf("last solve %+v, want an editorial upsolve of %s", last, id)
	}
}

// A mystery door's statement must not name it: not in the reader's title,
// not in the statement's heading, not through the browser key.
func TestMysteryStatementHidesTitle(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	cur := h.enterDoor()
	cur.Door.Mystery = true
	p, _ := h.m.d.Catalog.Get(cur.Door.Problem)

	msg := h.m.openDoc(cur.Door.Problem, false)()
	dm, ok := msg.(docMsg)
	if !ok {
		t.Fatalf("openDoc gave %#v, want a doc", msg)
	}
	if d := dm.doc; strings.Contains(d.Title+d.Source+d.Markdown, p.Title) || d.URL != "" ||
		strings.Contains(d.Markdown, "USACO 2021") {
		t.Fatalf("mystery statement gives away %q: %+v", p.Title, d)
	}
	if !strings.Contains(dm.doc.Markdown, "Bessie names this problem ???") {
		t.Fatalf("statement body lost: %q", dm.doc.Markdown)
	}
	// The same door, not a mystery, shows its title as usual.
	cur.Door.Mystery = false
	if d := h.m.openDoc(cur.Door.Problem, false)().(docMsg).doc; d.Title != p.Title {
		t.Fatalf("plain door titled %q, want %q", d.Title, p.Title)
	}
	h.send(dm)
	if _, ok := h.m.top().(*reader.Model); !ok {
		t.Fatalf("top screen %T, want the reader", h.m.top())
	}
}

// A door whose deadline passed while the game was closed crashes on
// resume without its editor opening first (it used to open, then close on
// the first tick a second later).
func TestResumeAfterDeadlineCrashesWithoutReopening(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.clock.t = h.enterDoor().Deadline.Add(time.Minute)
	opened := h.runner.opened
	h.restart()
	if h.m.state.Phase != game.PhaseCrashed || h.m.state.Crash.Cause != game.TimedOut {
		t.Fatalf("phase %q crash %+v, want a timeout on resume", h.m.state.Phase, h.m.state.Crash)
	}
	if h.runner.opened != opened {
		t.Errorf("runner opened %d times on resume, want 0", h.runner.opened-opened)
	}
}

// An upsolve starts from the attempt that crashed the boot, so a core dump
// picks up where the boot left off; a due regression starts blank in a
// directory of its own, even when the shared upsolve directory holds last
// time's accepted solution (resubmitting that is not recall).
func TestUpsolveSources(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.acFirst = 1
	h.enterDoor()
	h.send(submitMsg{})
	cur := h.enterDoor()
	id := cur.Door.Problem
	p, _ := h.m.d.Catalog.Get(id)
	h.send(game.GiveUp{})

	r := &TmuxRunner{Root: t.TempDir()}
	attempt := Door{Door: game.Door{Problem: id, Mystery: true}, Problem: p, Boot: h.m.state.Boot}
	dir, src, err := r.materialize(attempt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("// my attempt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	up := Door{Door: game.Door{Problem: id}, Problem: p, Upsolve: true}
	up.Sub, up.SeedBoot = h.m.upsolveFiles(id)
	if up.SeedBoot != h.m.state.Boot {
		t.Fatalf("upsolve seeds from boot %d, want the crashed boot %d", up.SeedBoot, h.m.state.Boot)
	}
	_, usrc, err := r.materialize(up)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(usrc); string(b) != "// my attempt\n" {
		t.Fatalf("upsolve source %q, want the attempt from %s", b, dir)
	}
	if err := os.WriteFile(usrc, []byte("// accepted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Days later the regression is due: a blank file elsewhere.
	h.clock.t = h.m.meta.Regressions[0].Due.Add(time.Hour)
	h.m.meta.CoreDump = nil
	h.m.state = nil
	reg := Door{Door: game.Door{Problem: id}, Problem: p, Upsolve: true}
	reg.Sub, reg.SeedBoot = h.m.upsolveFiles(id)
	_, rsrc, err := r.materialize(reg)
	if err != nil {
		t.Fatal(err)
	}
	if rsrc == usrc || reg.SeedBoot != 0 {
		t.Fatalf("regression re-solve at %s seeded from boot %d, want a fresh directory", rsrc, reg.SeedBoot)
	}
	if b, _ := os.ReadFile(rsrc); string(b) != r.template(p) {
		t.Fatalf("regression source %q, want the template", b)
	}
}

// git log names the biome a boot crashed in. Userland's doors keep their
// own topics, so naming it by the last door's topic called a Userland
// crash "The Scheduler" (or whatever that topic's biome is).
func TestLogNamesTheCrashBiome(t *testing.T) {
	crashIn := func(clears int) (string, string) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.acFirst = clears
		for i := 0; i < clears; i++ {
			h.enterDoor()
			h.send(submitMsg{})
		}
		h.enterDoor()
		want := biomeLabelOf(h.m.state)
		h.send(game.GiveUp{})
		boots, err := h.st.Boots(1)
		if err != nil {
			t.Fatal(err)
		}
		atts, err := h.st.Attempts(store.AttemptFilter{Boot: boots[0].Boot})
		if err != nil {
			t.Fatal(err)
		}
		return h.m.bootRow(boots[0], atts).Biome, want
	}
	for _, clears := range []int{0, 2} {
		if got, want := crashIn(clears); got != want {
			t.Errorf("crash after %d clears logged in %q, want %q", clears, got, want)
		}
	}
}

// biomeLabelOf is the biome name the boot screens show for the current
// biome, rebuilt from the state the way the engine named it.
func biomeLabelOf(s *game.State) string {
	b := s.Biomes[s.Biome]
	if b.Name != "" {
		return b.Name
	}
	return flavor.BiomeName(b.Topic)
}

// gateCatalog records the order fetches start in and holds each one until
// its gate opens or its context ends; Get puts ids starting "b" in a
// second source.
type gateCatalog struct {
	*fakeCatalog
	mu        sync.Mutex
	started   []catalog.ID
	cancelled map[catalog.ID]bool
	gates     map[catalog.ID]chan struct{}
	start     chan catalog.ID
}

func newGateCatalog(fc *fakeCatalog, ids ...catalog.ID) *gateCatalog {
	c := &gateCatalog{fakeCatalog: fc, cancelled: map[catalog.ID]bool{},
		gates: map[catalog.ID]chan struct{}{}, start: make(chan catalog.ID, 16)}
	for _, id := range ids {
		c.byID[id] = catalog.Problem{ID: id, Source: catalog.Source(string(id[:1])), Title: string(id)}
		c.gates[id] = make(chan struct{})
	}
	return c
}

func (c *gateCatalog) Fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	c.mu.Lock()
	c.started = append(c.started, id)
	c.mu.Unlock()
	c.start <- id
	select {
	case <-c.gates[id]:
		return catalog.Assets{}, nil
	case <-ctx.Done():
		c.mu.Lock()
		c.cancelled[id] = true
		c.mu.Unlock()
		return catalog.Assets{}, ctx.Err()
	}
}

func (c *gateCatalog) waitStart(t *testing.T) catalog.ID {
	t.Helper()
	select {
	case id := <-c.start:
		return id
	case <-time.After(time.Second):
		t.Fatal("no fetch started")
		return ""
	}
}

func (c *gateCatalog) wasCancelled(id catalog.ID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cancelled[id]
}

// Doors on offer prefetch one at a time per source, in door order (a
// source sends one request at a time, so fetching its doors together
// only delays the first), while different sources run side by side. A
// chosen door cancels the other doors' fetches but not its own.
func TestPrefetchOrder(t *testing.T) {
	h := newHarness(t)
	c := newGateCatalog(h.m.d.Catalog.(*fakeCatalog), "a1", "a2", "a3", "b1")
	h.m.d.Catalog = c
	done := make(chan struct{})
	cmd := h.m.prefetch([]catalog.ID{"a1", "a2", "a3", "b1"})
	go func() { cmd(); close(done) }()

	first := map[catalog.ID]bool{c.waitStart(t): true, c.waitStart(t): true}
	if !first["a1"] || !first["b1"] {
		t.Fatalf("first fetches %v, want a1 and b1 side by side", first)
	}
	close(c.gates["a1"])
	if id := c.waitStart(t); id != "a2" {
		t.Fatalf("after a1, %s started, want a2", id)
	}
	h.m.stopPrefetch("a2") // the player chose door a2
	close(c.gates["a2"])
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("prefetch kept going after a door was chosen")
	}
	if !c.wasCancelled("b1") || c.wasCancelled("a2") || slices.Contains(c.started, "a3") {
		t.Errorf("fetched %v, cancelled %v; want b1 cancelled, a2 kept, a3 skipped", c.started, c.cancelled)
	}
}

// Quitting cancels the fetches still running and waits for them, so a
// download cut short can remove its temp files before the process exits.
func TestQuitCancelsFetches(t *testing.T) {
	h := newHarness(t)
	c := newGateCatalog(h.m.d.Catalog.(*fakeCatalog), "a1", "b1")
	h.m.d.Catalog = c
	go h.m.prefetch([]catalog.ID{"a1", "b1"})()
	c.waitStart(t)
	c.waitStart(t)
	h.m.quit()
	// A fetch records its cancellation before it returns, and quit waits
	// for that.
	if !c.wasCancelled("a1") || !c.wasCancelled("b1") {
		t.Errorf("cancelled %v when quit returned, want both fetches", c.cancelled)
	}
}

// breakingCatalog finds every problem it fetches unjudgeable, as a USACO
// fetch does when the problem's test zip turns out not to exist.
type breakingCatalog struct {
	*fakeCatalog
	mu     sync.Mutex
	broken map[catalog.ID]bool
}

func (c *breakingCatalog) Get(id catalog.ID) (catalog.Problem, bool) {
	p, ok := c.fakeCatalog.Get(id)
	c.mu.Lock()
	defer c.mu.Unlock()
	p.Judgeable = p.Judgeable && !c.broken[id]
	return p, ok
}

func (c *breakingCatalog) All() []catalog.Problem {
	all := slices.Clone(c.fakeCatalog.All())
	for i := range all {
		all[i], _ = c.Get(all[i].ID)
	}
	return all
}

func (c *breakingCatalog) Fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	c.mu.Lock()
	c.broken[id] = true
	c.mu.Unlock()
	return c.fakeCatalog.Fetch(ctx, id)
}

// A problem a fetch finds unjudgeable leaves the pool the engine draws
// doors from; the pool used to be read once, at start.
func TestUnjudgeableProblemLeavesThePool(t *testing.T) {
	h := newHarness(t)
	h.m.d.Catalog = &breakingCatalog{fakeCatalog: h.m.d.Catalog.(*fakeCatalog), broken: map[catalog.ID]bool{}}
	h.send(screens.StartBoot{Preset: "standard"}) // the room's doors prefetch
	h.toDoors()
	id := h.m.state.Doors[0].Problem
	if i := slices.IndexFunc(h.m.pool, func(p catalog.Problem) bool { return p.ID == id }); h.m.pool[i].Judgeable {
		t.Fatalf("%s still judgeable in the pool after its fetch found it not", id)
	}
}

// A door's timer starts when its problem is on disk, not when it is
// chosen: a cold fetch took up to 56 s in play, all of it off the timer.
// A fetch that fails leaves the door unopened instead of open and doomed.
func TestChosenDoorWaitsForItsProblem(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.toDoors()
	h.send(game.ChooseDoor{Index: 0})
	if h.m.state.Phase != game.PhaseProblem {
		t.Fatalf("phase %q after choosing a cached door, want problem", h.m.state.Phase)
	}

	h = newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.toDoors()
	id := h.m.state.Doors[0].Problem
	c := newGateCatalog(h.m.d.Catalog.(*fakeCatalog))
	c.gates[id] = make(chan struct{})
	h.m.d.Catalog = c
	h.send(game.ChooseDoor{Index: 0}) // the fetch hangs; the harness moves on
	if h.m.state.Phase != game.PhaseDoors || h.m.flash == "" {
		t.Fatalf("phase %q flash %q while the door's problem downloads, want doors and a note", h.m.state.Phase, h.m.flash)
	}
	h.clock.t = h.clock.t.Add(40 * time.Second)
	h.send(doorReadyMsg{index: 0, id: id})
	cur := h.m.state.Current
	if h.m.state.Phase != game.PhaseProblem || !cur.Opened.Equal(h.clock.t) {
		t.Fatalf("phase %q opened %v, want the door opened at %v, after its fetch", h.m.state.Phase, cur.Opened, h.clock.t)
	}
	close(c.gates[id])

	h = newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.toDoors()
	id = h.m.state.Doors[0].Problem
	h.m.choosing = id
	h.send(doorReadyMsg{index: 0, id: id, err: errors.New("offline")})
	if h.m.state.Phase != game.PhaseDoors || !strings.Contains(h.m.flash, "offline") {
		t.Fatalf("phase %q flash %q after a failed fetch, want doors and the error", h.m.state.Phase, h.m.flash)
	}
}

// The problem screen fills the pane exactly: boot bar, the embedded HUD,
// then the two footer rows on the last two lines.
func TestProblemScreenFillsThePane(t *testing.T) {
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: t.TempDir(), Embedded: true}}
	})
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	for _, size := range [][2]int{{69, 49}, {40, 20}} {
		h.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(h.m.View().Content, "\n")
		if len(lines) != size[1] {
			t.Fatalf("%d lines at height %d, want %d", len(lines), size[1], size[1])
		}
		vim, keys := ansi.Strip(lines[len(lines)-2]), ansi.Strip(lines[len(lines)-1])
		if !strings.HasPrefix(vim, "vim ") || !strings.HasPrefix(keys, "s submit") {
			t.Fatalf("footer at height %d is %q / %q, want the vim and key help", size[1], vim, keys)
		}
	}
}

// A submit the HUD refuses (no problem loaded) ends the judging at once,
// so it cannot hold the door's clock until the grace runs out.
func TestRefusedSubmitReleasesTheClock(t *testing.T) {
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: t.TempDir()}}
	})
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	req, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	h.send(req)
	if h.m.judging {
		t.Fatal("a refused submit left the clock held")
	}
}

// Bytes spent at a merge window buy for the boot and are saved with it; a
// held shuf re-rolls the doors from the doors screen's s key.
func TestMergeSpend(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	for h.m.state.Phase != game.PhaseMerge {
		h.enterDoor()
		h.send(submitMsg{})
	}
	for h.m.state.PendingUpgrade {
		h.send(game.PickUpgrade{Upgrade: h.m.state.UpgradeOffers[0]})
	}
	s := h.m.state
	s.Uncommitted = 1000 // enough for anything
	ducks := s.Ducks
	h.send(game.Spend{Item: game.Dup})
	h.send(game.Spend{Item: game.Shuf})
	if s.Ducks != ducks+1 || s.Rerolls != 1 || s.Spent != s.Rules.DupPrice+s.Rules.ShufPrice {
		t.Fatalf("after dup and shuf: ducks %d, shufs %d, spent %d", s.Ducks, s.Rerolls, s.Spent)
	}
	h.send(game.LeaveMerge{})
	h.toDoors()
	before := slices.Clone(h.m.state.Doors)
	h.send(key("s"))
	if s := h.m.state; s.Rerolls != 0 || slices.Equal(s.Doors, before) {
		t.Fatalf("s at the doors: %d shufs left, doors changed %v", s.Rerolls, !slices.Equal(s.Doors, before))
	}
	h.restart()
	if s := h.m.state; s == nil || s.Spent != s.Rules.DupPrice+s.Rules.ShufPrice || s.Ducks != ducks+1 {
		t.Fatalf("resumed boot lost the spending: %+v", s)
	}
}

// A door's deadline is absolute, so quitting with one open asks first
// (D31): ctrl+c warns, any other key takes the warning back, and a second
// ctrl+c in a row quits. The resumed door says how long it has left.
func TestQuitMidDoorConfirms(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	ctrlC := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	if _, cmd := h.m.Update(ctrlC); cmd != nil || !strings.Contains(h.m.flash, "keeps running") {
		t.Fatalf("first ctrl+c: cmd %v, flash %q; want a warning and no quit", cmd != nil, h.m.flash)
	}
	h.m.Update(key("j"))
	if _, cmd := h.m.Update(ctrlC); cmd != nil {
		t.Fatal("ctrl+c after another key quit without warning again")
	}
	_, cmd := h.m.Update(ctrlC)
	if cmd == nil {
		t.Fatal("second ctrl+c in a row did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("second ctrl+c: not tea.Quit")
	}
	h.clock.t = h.clock.t.Add(10 * time.Minute)
	h.restart()
	if !strings.Contains(h.m.flash, "kept running") || !strings.Contains(h.m.flash, "left") {
		t.Errorf("resumed door: flash %q, want how long it has left", h.m.flash)
	}
}

// Outside a door ctrl+c quits at once.
func TestQuitOutsideDoor(t *testing.T) {
	h := newHarness(t)
	_, cmd := h.m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c at GRUB did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c at GRUB: not tea.Quit")
	}
}

// apt at GRUB buys a package with the bank and saves it; a refused buy
// says why and changes nothing; installing a theme puts it in use, and
// UseTheme goes back to the default.
func TestShopInstall(t *testing.T) {
	h := newHarness(t)
	h.m.meta.Bank = 100
	h.send(screens.OpenShop{})
	if _, ok := h.m.top().(*meta.Shop); !ok {
		t.Fatalf("top is %T, want the shop", h.m.top())
	}
	h.send(screens.Install{Package: "tux"}) // 128 B
	if game.Installed(&h.m.meta, "tux") || h.m.meta.Bank != 100 || !strings.Contains(h.m.flash, "costs") {
		t.Fatalf("unaffordable install: installed %v, bank %d, flash %q", h.m.meta.Installed, h.m.meta.Bank, h.m.flash)
	}
	cowsay, _ := h.m.aptRules().Package("cowsay")
	left := 100 - cowsay.Price
	h.send(screens.Install{Package: "cowsay"})
	if !game.Installed(&h.m.meta, "cowsay") || h.m.meta.Bank != left || h.m.meta.Theme != "cowsay" {
		t.Fatalf("install: installed %v, bank %d, theme %q", h.m.meta.Installed, h.m.meta.Bank, h.m.meta.Theme)
	}
	if !strings.Contains(h.m.stack[0].View(100, 30), "bank "+strconv.FormatInt(left, 10)+" B") {
		t.Errorf("GRUB under the shop still shows the old bank:\n%s", h.m.stack[0].View(100, 30))
	}
	h.send(screens.UseTheme{})
	if h.m.meta.Theme != "" {
		t.Errorf("theme %q after going back to the default", h.m.meta.Theme)
	}
	h.restart()
	if !game.Installed(&h.m.meta, "cowsay") || h.m.meta.Bank != left {
		t.Errorf("after a restart: installed %v, bank %d", h.m.meta.Installed, h.m.meta.Bank)
	}
}
