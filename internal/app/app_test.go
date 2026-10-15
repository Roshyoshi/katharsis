package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

type fakeRunner struct{ opened, closed int }

func (r *fakeRunner) Open(context.Context, Door) error { r.opened++; return nil }
func (r *fakeRunner) Close() error                     { r.closed++; return nil }

// clock advances only when the test says so, so timers never expire by
// accident.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

// hudFakeRunner is a runner with a HUD, as the tmux runner has: the app
// then routes submits through the HUD instead of Deps.Judge. Its HUD has no
// problem to load, so it judges nothing; tests deliver hud.SubmittedMsg
// themselves.
type hudFakeRunner struct {
	*fakeRunner
	hud *hud.Model
}

func (r *hudFakeRunner) HUD() *hud.Model { return r.hud }
func (r *hudFakeRunner) Reopen() error   { return nil }

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
}

func newHarness(t *testing.T) *harness { return newHarnessWith(t, nil) }

// newHarnessWith runs the app with the runner wrap makes around the
// harness's fakeRunner; nil wrap uses the fakeRunner itself.
func newHarnessWith(t *testing.T, wrap func(*fakeRunner) Runner) *harness {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	h := &harness{t: t, st: st, runner: &fakeRunner{}, clock: &clock{time.Date(2026, 10, 14, 18, 0, 0, 0, time.UTC)}, verdict: "AC"}
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

func TestFullClear(t *testing.T) {
	h := newHarness(t)
	if _, ok := h.m.top().(interface{ Editing() bool }); !ok {
		t.Fatalf("start screen is %T, want the GRUB menu", h.m.top())
	}
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

	h.verdict = "AC"
	h.send(screens.Upsolve{Problem: string(dump.Problem)})
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
	for h.m.state.Phase != game.PhaseProblem {
		switch {
		case h.m.state.PendingUpgrade:
			h.send(game.PickUpgrade{Upgrade: h.m.state.UpgradeOffers[0]})
		case h.m.state.Phase == game.PhaseRoute:
			h.send(game.ChooseRoute{})
		default:
			h.send(game.ChooseDoor{})
		}
	}
	h.clock.t = h.m.state.Current.Deadline.Add(time.Second)
	h.send(tickMsg(h.clock.t))
	if h.m.state.Phase != game.PhaseCrashed || h.m.state.Crash.Cause != game.TimedOut {
		t.Fatalf("phase %q crash %+v, want a timeout crash", h.m.state.Phase, h.m.state.Crash)
	}
}

func TestResumeMidDoor(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	for h.m.state.Phase != game.PhaseProblem {
		switch {
		case h.m.state.Phase == game.PhaseRoute:
			h.send(game.ChooseRoute{})
		default:
			h.send(game.ChooseDoor{})
		}
	}
	want := h.m.state.Current.Door

	// A second app on the same store resumes the boot and reopens the door.
	m2, err := New(Deps{Store: h.st, Catalog: h.m.d.Catalog, Runner: h.runner, Judge: h.m.d.Judge, Now: h.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	h.m = m2
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20})
	h.run(m2.Init())
	if h.m.open == nil || !reflect.DeepEqual(h.m.open.Door, want) {
		t.Fatalf("resumed door %+v, want %+v", h.m.open, want)
	}
}

// enterDoor plays the boot up to its first open door.
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
	var hr *hudFakeRunner
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		hr = &hudFakeRunner{fakeRunner: f, hud: hud.New(hud.Options{Dir: t.TempDir()})}
		return hr
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
	h.send(hud.SubmittedMsg{Summary: judge.Summary{Verdict: judge.AC, Passed: 4, Total: 4, Done: 4, First: -1}})
	if last := h.m.state.History[len(h.m.state.History)-1]; last.Outcome != game.Cleared {
		t.Fatalf("door ended %q, want cleared", last.Outcome)
	}
}

// Editor commands answer in vim: a refused quack says why, and an accepted
// one says what the duck revealed.
func TestEditorQuackAnswers(t *testing.T) {
	h := newHarnessWith(t, func(f *fakeRunner) Runner {
		return &hudFakeRunner{fakeRunner: f, hud: hud.New(hud.Options{Dir: t.TempDir()})}
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

// The editorial a :q! opens counts against the upsolve that follows, even
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

	m2, err := New(Deps{Store: h.st, Catalog: h.m.d.Catalog, Runner: h.runner, Judge: h.m.d.Judge, Now: h.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	h.m = m2
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m2.Init())
	h.send(screens.Upsolve{Problem: string(id)})
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
	cur := h.enterDoor()
	opened := h.runner.opened

	h.clock.t = cur.Deadline.Add(time.Minute)
	m2, err := New(Deps{Store: h.st, Catalog: h.m.d.Catalog, Runner: h.runner, Judge: h.m.d.Judge, Now: h.clock.now})
	if err != nil {
		t.Fatal(err)
	}
	h.m = m2
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m2.Init())
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
	os.WriteFile(usrc, []byte("// accepted\n"), 0o644)

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

// barrierCatalog's Fetch waits until n fetches run at once (or a second
// passes), so a serial prefetch shows up as a timeout per door.
type barrierCatalog struct {
	*fakeCatalog
	n       int
	mu      sync.Mutex
	active  int
	release chan struct{}
}

func (c *barrierCatalog) Fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	c.mu.Lock()
	c.active++
	if c.active == c.n {
		close(c.release)
	}
	c.mu.Unlock()
	select {
	case <-c.release:
	case <-time.After(time.Second):
	}
	return catalog.Assets{}, nil
}

// Doors on offer are fetched side by side: one at a time, the last door of
// a cold room was ready seconds after the first.
func TestPrefetchFetchesDoorsInParallel(t *testing.T) {
	h := newHarness(t)
	fc := h.m.d.Catalog.(*fakeCatalog)
	ids := []catalog.ID{fc.all[0].ID, fc.all[1].ID, fc.all[2].ID}
	bc := &barrierCatalog{fakeCatalog: fc, n: len(ids), release: make(chan struct{})}
	h.m.d.Catalog = bc

	start := time.Now()
	h.m.prefetch(ids)()
	if took := time.Since(start); took > 500*time.Millisecond {
		t.Fatalf("prefetch of %d doors took %v, want them fetched at once", len(ids), took)
	}
}
