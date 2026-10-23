package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
	"katharsis/internal/screens/samples"
	"katharsis/internal/session"
	"katharsis/internal/store"
	"katharsis/internal/ui"
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
	// stall, set by hold, makes the judge hang. The judge reads it on a
	// command's goroutine, which a test abandons and moves on from, so it
	// is behind mu.
	mu    sync.Mutex
	stall chan struct{}
	// path is the store file, for a test that opens it from outside.
	path string
	// real runs the commands on the real clock, for a test that waits on
	// another process (a compiler, tmux, a lock holder), which a synctest
	// bubble cannot. Every other test runs in a bubble (synctest.Test),
	// where a timer costs no time.
	real bool
	// models are every game the test started (restart), stopped when it
	// ends.
	models []*Model
}

// steady is the standard preset without its easing (game.EaseFloor), boot
// #1's rules (RT11) and Userland's timeout rule (D44): these tests crash
// on Userland timeouts, which the rule turns into lost lives, and a
// cold-start player on the fake catalog is aimed under its floor. "eased"
// plays the standard preset as shipped.
func steady() rules.Rules {
	r := rules.Legacy()
	r.EaseAbove, r.BuybackTimers, r.GovernorAfter = 0, nil, 0
	r.FirstBiomes, r.FirstDoor, r.FirstEditorial = nil, false, false
	r.Biomes = slices.Clone(r.Biomes)
	r.Biomes[0].TimeoutLife = false
	return r
}

func newHarness(t *testing.T) *harness { return newHarnessWith(t, nil) }

// newHarnessWith runs the app with the runner wrap makes around the
// harness's fakeRunner; nil wrap uses the fakeRunner itself. The test runs
// in a synctest bubble.
func newHarnessWith(t *testing.T, wrap func(*fakeRunner) Runner) *harness {
	return startHarness(t, wrap, false)
}

// newRealHarness is newHarnessWith for a test outside a bubble (real).
func newRealHarness(t *testing.T, wrap func(*fakeRunner) Runner) *harness {
	return startHarness(t, wrap, true)
}

func startHarness(t *testing.T, wrap func(*fakeRunner) Runner, real bool) *harness {
	t.Helper()
	path := filepath.Join(t.TempDir(), "k.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // some tests close it early
	h := &harness{t: t, st: st, path: path, runner: &fakeRunner{}, clock: &clock{time.Date(2026, 10, 14, 18, 0, 0, 0, time.UTC)}, verdict: "AC", real: real}
	// Before the store closes: the games' event writers and fetches stop,
	// and in a bubble the commands abandoned on a timer get their time and
	// return, as synctest.Test needs every goroutine gone.
	t.Cleanup(func() {
		for _, m := range h.models {
			m.cancel()
			m.tele.rec.close(eventFlush)
		}
		if !h.real {
			time.Sleep(time.Hour)
		}
	})
	var runner Runner = h.runner
	if wrap != nil {
		runner = wrap(h.runner)
	}
	cat := newFakeCatalog()
	cat.docs = t.TempDir()
	h.start(Deps{
		Store:   st,
		Catalog: cat,
		Runner:  runner,
		Judge: func(context.Context, Door) (string, int, int, error) {
			h.mu.Lock()
			stall := h.stall
			h.mu.Unlock()
			if stall != nil {
				<-stall
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
		Now:     h.clock.now,
		Seed:    func() uint64 { return 42 },
		Presets: map[string]rules.Rules{"standard": steady(), "eased": rules.Legacy()},
	})
	return h
}

// start runs a game on d, as launching katharsis does.
func (h *harness) start(d Deps) {
	h.t.Helper()
	m, err := New(d)
	if err != nil {
		h.t.Fatal(err)
	}
	h.m, h.models = m, append(h.models, m)
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m.Init())
}

// restart is the game started again on the same store, catalog and
// runner.
func (h *harness) restart() {
	h.t.Helper()
	d := h.m.d
	h.start(Deps{Store: d.Store, Catalog: d.Catalog, Runner: d.Runner, Judge: d.Judge, Now: d.Now, Presets: d.Presets})
}

// hold makes the judge hang, like a slow full test set, until release;
// the test delivers the verdict itself. The released judge fails, and
// later ones judge again. The test's end releases it too.
func (h *harness) hold() (release func()) {
	stall := make(chan struct{})
	h.mu.Lock()
	h.stall = stall
	h.mu.Unlock()
	var once sync.Once
	release = func() {
		once.Do(func() {
			h.mu.Lock()
			h.stall = nil
			h.mu.Unlock()
			close(stall)
		})
	}
	h.t.Cleanup(release)
	return release
}

// step is how long a command may take before run moves on without it: a
// tea.Tick that short is delivered (the crash's reveal), a longer one (the
// 1 s tick) is abandoned, the way a test clock would skip it. In a bubble
// it is fake time.
const step = 30 * time.Millisecond

// run executes a command and feeds its messages back, recursively. A
// command still blocked after step is abandoned.
func (h *harness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	if h.real {
		select {
		case msg = <-done:
		case <-time.After(step):
			return
		}
	} else {
		// Wait returns once every goroutine is blocked: the command is done,
		// or waits on a timer or a channel.
		synctest.Wait()
		select {
		case msg = <-done:
		default:
			time.Sleep(step)
			synctest.Wait()
			select {
			case msg = <-done:
			default:
				return
			}
		}
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

// pump runs cmd as a tea.Program would, for a pass that takes longer than
// run waits: its commands run concurrently and their messages go to the
// app on this goroutine, until done holds after one of them. Commands
// still blocked (ticks) are dropped on return.
func (h *harness) pump(cmd tea.Cmd, done func() bool) {
	h.t.Helper()
	msgs, stop := make(chan tea.Msg), make(chan struct{})
	defer close(stop)
	launch := func(c tea.Cmd) {
		if c != nil {
			go func() {
				select {
				case msgs <- c():
				case <-stop:
				}
			}()
		}
	}
	launch(cmd)
	deadline := time.After(60 * time.Second)
	for !done() {
		select {
		case msg := <-msgs:
			switch msg := msg.(type) {
			case nil:
			case tea.BatchMsg:
				for _, c := range msg {
					launch(c)
				}
			default:
				_, c := h.m.Update(msg)
				launch(c)
			}
		case <-deadline:
			h.t.Fatal("pump: timed out")
		}
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
	synctest.Test(t, func(t *testing.T) {
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
	})
}

func TestCrashThenUpsolveRecoversCoreDump(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		if got := dump.Recovers(game.Independent); h.m.meta.Bank != bank+got { // RT4: 1.5x, no editorial
			t.Errorf("bank %d, want %d + %d recovered", h.m.meta.Bank, bank, got)
		}
		if h.m.open == nil || !h.m.open.Solved {
			t.Fatal("the solved upsolve did not stay open")
		}
		h.send(key("Q"))
		if h.m.open != nil {
			t.Error("upsolve door still open after Q")
		}
	})
}

func TestTimerExpiryCrashes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.clock.t = h.enterDoor().Deadline.Add(time.Second)
		h.send(tickMsg(h.clock.t))
		if h.m.state.Phase != game.PhaseCrashed || h.m.state.Crash.Cause != game.TimedOut {
			t.Fatalf("phase %q crash %+v, want a timeout crash", h.m.state.Phase, h.m.state.Crash)
		}
	})
}

// A Userland timeout costs a life instead of the boot (D44): the app
// closes the door as a retreat does and offers the room again, and the
// boot is saved that way.
func TestUserlandTimeoutCostsALife(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "eased"})
		if len(h.m.state.Ease) == 0 {
			t.Fatalf("a cold start on the fake catalog is not eased: forecast %.2f", h.m.state.Forecast)
		}
		lives, closed := h.m.state.Cycles, h.runner.closed
		h.clock.t = h.enterDoor().Deadline.Add(time.Second)
		h.send(tickMsg(h.clock.t))
		s := h.m.state
		if s.Phase != game.PhaseDoors || s.Crash != nil || s.Cycles != lives-1 || len(s.Doors) == 0 || h.m.open != nil || h.runner.closed != closed+1 {
			t.Fatalf("phase %q crash %+v lives %d (was %d), %d doors, open %v, closed %d (was %d)",
				s.Phase, s.Crash, s.Cycles, lives, len(s.Doors), h.m.open != nil, h.runner.closed, closed)
		}
		h.restart()
		if h.m.state.Phase != game.PhaseDoors || h.m.state.Cycles != lives-1 {
			t.Errorf("resumed in phase %q with %d lives", h.m.state.Phase, h.m.state.Cycles)
		}
	})
}

// x at the first room declines the boot's easing (RT8, GR9): the engine
// takes back the lives it added and draws the room again, the doors say
// "declined", and the decline is saved; a second x does nothing.
func TestDeclineEaseKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "eased"})
		h.toDoors()
		s := h.m.state
		if len(s.Ease) == 0 {
			t.Fatalf("a cold start on the fake catalog is not eased: forecast %.2f", s.Forecast)
		}
		added := 0
		for _, es := range s.Ease {
			added += es.Cycles
		}
		lives := s.Cycles
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "x decline the easing") {
			t.Fatalf("no decline key on the first room:\n%s", v)
		}
		h.send(key("x"))
		s = h.m.state
		if !s.Declined || s.Cycles != max(1, lives-added) {
			t.Fatalf("declined %v, lives %d (was %d, the easing added %d)", s.Declined, s.Cycles, lives, added)
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "· declined") || strings.Contains(v, "x decline") {
			t.Errorf("the doors after a decline:\n%s", v)
		}
		h.send(key("x"))
		if h.m.state.Cycles != s.Cycles {
			t.Errorf("a second x moved lives to %d", h.m.state.Cycles)
		}
		h.restart()
		if !h.m.state.Declined {
			t.Error("the decline was not saved")
		}
	})
}

// GRUB's status line says how long the player was away when the next
// boot is a comeback (RT8), and that boot's strip opens on the same days.
func TestGrubComeback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.meta.Boots = 4
		for _, c := range []struct {
			away time.Duration
			want string
		}{{2 * 24 * time.Hour, ""}, {5*24*time.Hour + time.Hour, "first boot in 5 days"}} {
			h.m.meta.LastPlayed = h.clock.t.Add(-c.away)
			v := ansi.Strip(h.m.grub().View(100, 30))
			if got := strings.Contains(v, "first boot in"); got != (c.want != "") || c.want != "" && !strings.Contains(v, c.want) {
				t.Errorf("%v away, want %q on GRUB:\n%s", c.away, c.want, v)
			}
		}
		h.send(screens.StartBoot{Preset: "eased"})
		h.toDoors()
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "(first boot in 5 days)") {
			t.Errorf("the comeback boot does not say why it is eased:\n%s", v)
		}
	})
}

// The game ships short and patch beside standard (RT7): New adds them
// where Deps has none, and GRUB lists them first among the other presets,
// before the player's own.
func TestBuiltInPresets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		if !reflect.DeepEqual(h.m.d.Presets["short"], rules.Short()) || !reflect.DeepEqual(h.m.d.Presets["patch"], rules.Patch()) {
			t.Fatal("New did not add the built-in short and patch")
		}
		g := h.m.grub()
		for range 3 { // past prestige 0, the nightly and RTFM
			g, _ = g.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
		g, _ = g.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		v := ansi.Strip(g.View(100, 30))
		short, patch, own := strings.Index(v, "preset short"), strings.Index(v, "preset patch"), strings.Index(v, "preset eased")
		if short < 0 || patch < short || own < patch {
			t.Errorf("other presets, want short, patch, then eased:\n%s", v)
		}
	})
}

// A second app on the same store resumes the boot and reopens the door.
func TestResumeMidDoor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		want := h.enterDoor().Door
		h.restart()
		if h.m.open == nil || !reflect.DeepEqual(h.m.open.Door, want) {
			t.Fatalf("resumed door %+v, want %+v", h.m.open, want)
		}
	})
}

// A submit made before the deadline is judged after it; the engine's grace
// must cover the judge's run, so the clock may not crash the door while
// the result is on its way.
func TestSubmitJudgedPastDeadlineWithinGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		cur := h.enterDoor()
		h.hold()
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
	})
}

// A judge that never answers must not stop the clock past the grace.
func TestLostJudgeCannotStopTheClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		cur := h.enterDoor()
		h.hold()
		h.send(submitMsg{})

		h.clock.t = cur.Deadline.Add(h.m.state.Rules.SubmitGrace.Duration)
		h.send(tickMsg(h.clock.t))
		if h.m.state.Phase != game.PhaseCrashed || h.m.state.Crash.Cause != game.TimedOut {
			t.Fatalf("phase %q crash %+v, want a timeout once the grace ran out", h.m.state.Phase, h.m.state.Crash)
		}
	})
}

// :Submit from vim is how most submits arrive. Before the fix it reached
// the HUD without marking the door as judging, so a submit just before the
// deadline was crashed by the next tick instead of getting the grace.
func TestEditorSubmitGetsTheGrace(t *testing.T) {
	// The HUD needs a loadable problem, so its submit really runs (a
	// refused submit ends at once and rightly holds no clock), and a
	// compiler that holds it until the door closes, however fast a build
	// is. The samples pass, so :Submit asks nothing.
	h := hudHarness(t, submitDir(t, acSrc), stalled(t))
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
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// A door whose editor failed to open is not open, so e retries it; it used
// to count as open with no editor and could only time out.
func TestFailedOpenRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// A failed reopen leaves the editor marked closed, so e can try again; it
// used to clear the mark first, and the door had no editor and no retry.
func TestFailedReopenRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// A fetch that lands after its door is gone opens nothing: here the player
// left an upsolve while its problem downloaded.
func TestLateFetchOpensNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// A boot whose first save failed still starts, with the error shown; it
// used to sit behind the screen it was started from.
func TestUnsavedBootStillStarts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		if err := h.st.Close(); err != nil {
			t.Fatal(err)
		}
		h.send(screens.StartBoot{Preset: "standard"})
		if _, grub := h.m.top().(interface{ Editing() bool }); grub || !strings.Contains(h.m.View().Content, "save failed") {
			t.Fatalf("top %T, want the boot's screen and the save error", h.m.top())
		}
	})
}

// git log lists an upsolve once it is left: a crash's core dump upsolved,
// a WA then the AC four minutes in, then an hour more open. Its line shows
// the four minutes and the solve, its close event the bytes it recovered,
// and enter its verdicts.
func TestGitLogListsUpsolves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.crashWithDump()
		id := h.m.state.Crash.Door.Problem
		h.send(screens.Upsolve{Problem: id})
		h.verdict = "WA"
		h.send(submitMsg{})
		h.clock.t = h.clock.t.Add(4 * time.Minute)
		h.verdict = "AC"
		h.send(submitMsg{})
		got := h.m.open.Recovered
		h.clock.t = h.clock.t.Add(time.Hour)
		h.send(key("Q"))
		evs := h.events() // written out of band; git log reads the store
		if e := evs[len(evs)-1]; e.Kind != store.EventUpsolveClose || got <= 0 || payload(t, e)["recovered"] != float64(got) {
			t.Errorf("last event %s %v, want the upsolve's close recovering %d", e.Kind, payload(t, e), got)
		}
		h.send(screens.OpenLog{})
		if v := view(h); !strings.Contains(v, "04:00 solved "+h.m.title(id)) {
			t.Fatalf("git log lacks the upsolve, solved in 04:00:\n%s", v)
		}
		h.send(key("enter"))
		v := view(h)
		for _, want := range []string{"for 1:04:00", "recovered", "✗ WA", "✓ AC"} {
			if !strings.Contains(v, want) {
				t.Errorf("the upsolve's expansion lacks %q:\n%s", want, v)
			}
		}
	})
}

// An accepted upsolve stays open, solved, saved or not, and a second AC
// records nothing more. It once closed its door for that: left open, a
// second AC recorded the solve twice.
func TestUpsolveCountsOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(game.GiveUp{})
		h.send(screens.Upsolve{Problem: h.m.state.Crash.Door.Problem})
		if err := h.st.Close(); err != nil {
			t.Fatal(err)
		}
		h.send(submitMsg{})
		if h.m.open == nil || !h.m.open.Solved || !strings.Contains(h.m.View().Content, "save failed") {
			t.Fatalf("open %v, want the door open, solved, and the save error", h.m.open != nil)
		}
		solves, bank := len(h.m.meta.Solves), h.m.meta.Bank
		h.send(submitMsg{})
		if len(h.m.meta.Solves) != solves || h.m.meta.Bank != bank || !strings.Contains(view(h), "solved") {
			t.Fatalf("a second AC: %d solves (was %d), bank %d (was %d):\n%s",
				len(h.m.meta.Solves), solves, h.m.meta.Bank, bank, view(h))
		}
	})
}

// The editorial a :q! offers on the crash screen, never opened, costs the
// upsolve nothing: it pays the 1.5x the debrief promised (O2), in the same
// session and after a restart. Offered once counted as seen.
func TestUpsolveAfterGiveUpUnreadIsIndependent(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run("restart="+strconv.FormatBool(restart), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t)
				h.send(screens.StartBoot{Preset: "standard"})
				h.acFirst = 1 // bytes on door 1, so :q! on door 2 leaves a core dump
				h.enterDoor()
				h.send(submitMsg{})
				id := h.enterDoor().Door.Problem
				hasEditorial(h, id)
				h.send(game.GiveUp{})
				dump := h.m.meta.CoreDump
				if dump == nil || dump.Problem != id {
					t.Fatalf("core dump %+v, want one on %s", dump, id)
				}
				if restart {
					h.restart()
				}
				bank := h.m.meta.Bank
				h.send(screens.Upsolve{Problem: id})
				h.send(submitMsg{})
				last := h.m.meta.Solves[len(h.m.meta.Solves)-1]
				if last.Problem != id || !last.Upsolve || last.Kind != game.Independent {
					t.Fatalf("last solve %+v, want an independent upsolve of %s", last, id)
				}
				if got := dump.Recovers(game.Independent); h.m.meta.Bank != bank+got {
					t.Errorf("bank %d, want %d + %d, the independent rate", h.m.meta.Bank, bank, got)
				}
			})
		})
	}
}

// An editorial read after a crash counts against the upsolve that
// follows, even after the game restarts and the core dump is upsolved
// from GRUB: the store's editorial_open says so, whatever ended the boot.
// Once a solve has used the read, it no longer counts.
func TestUpsolveAfterReadingIsEditorial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.crashWithDump() // out of lives: no :q! involved
		id := h.m.state.Crash.Door.Problem
		hasEditorial(h, id)
		h.m.d.Catalog = editorialCatalog{h.m.d.Catalog.(*fakeCatalog)}
		h.send(screens.OpenEditorial{Problem: id})
		h.send(screens.Back{})
		h.events() // quitting flushes the queue

		h.restart()
		h.clock.t = h.clock.t.Add(time.Minute)
		h.send(screens.Upsolve{Problem: id})
		h.send(submitMsg{})
		last := h.m.meta.Solves[len(h.m.meta.Solves)-1]
		if last.Problem != id || !last.Upsolve || last.Kind != game.Editorial {
			t.Fatalf("last solve %+v, want an editorial upsolve of %s", last, id)
		}
		h.restart()
		if h.m.sawEditorial(id) {
			t.Error("the read still counts after the solve it was used for")
		}
	})
}

// hasEditorial gives problem id an editorial in the harness's catalog.
func hasEditorial(h *harness, id catalog.ID) {
	c := h.m.d.Catalog.(*fakeCatalog)
	p := c.byID[id]
	p.HasEditorial = true
	c.byID[id] = p
}

// An editorial never on screen costs the upsolve nothing. A :q! on a
// problem without one offered none, and e on the upsolve, with nothing to
// show, marked the solve editorial-assisted when it was pressed: it paid
// the editorial rate, not the independent one.
func TestUpsolveWithoutAnEditorialIsIndependent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.acFirst = 1
		h.enterDoor()
		h.send(submitMsg{})
		id := h.enterDoor().Door.Problem
		h.send(game.GiveUp{})
		h.send(screens.Upsolve{Problem: id})
		h.send(key("e")) // the upsolve's editorial key
		if !strings.Contains(h.m.flash, "no editorial") {
			t.Fatalf("e on an upsolve without an editorial flashed %q", h.m.flash)
		}
		h.send(submitMsg{})
		last := h.m.meta.Solves[len(h.m.meta.Solves)-1]
		if last.Problem != id || !last.Upsolve || last.Kind != game.Independent {
			t.Fatalf("last solve %+v, want an independent upsolve of %s", last, id)
		}
	})
}

// A practice upsolve from RTFM's level screen is of any problem, not a core
// dump or a regression: accepted, it records the solve as an upsolve and
// banks nothing (plan GR4, Q7), and the player is left with no dump.
func TestPracticeUpsolveBanksNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		h.send(screens.OpenLearn{})
		id := h.m.d.Catalog.All()[0].ID
		bank, lifetime := h.m.meta.Bank, h.m.meta.LifetimeBytes
		h.send(screens.Upsolve{Problem: id})
		if h.m.open == nil || !h.m.open.Upsolve || h.m.open.Door.Problem != id {
			t.Fatalf("open door %+v, want an upsolve of %s", h.m.open, id)
		}
		h.send(submitMsg{})
		if n := len(h.m.meta.Solves); n != 1 {
			t.Fatalf("%d solves, want 1", n)
		}
		if s := h.m.meta.Solves[0]; s.Problem != id || !s.Upsolve || s.Kind != game.Independent {
			t.Errorf("solve %+v, want an independent upsolve of %s", s, id)
		}
		if h.m.meta.Bank != bank || h.m.meta.LifetimeBytes != lifetime || h.m.meta.CoreDump != nil {
			t.Errorf("bank %d lifetime %d dump %+v, want %d and %d unchanged, no dump",
				h.m.meta.Bank, h.m.meta.LifetimeBytes, h.m.meta.CoreDump, bank, lifetime)
		}
		// Solved, it stays open until Q; practice then comes back to the
		// map it was started from, not GRUB.
		if _, ok := h.m.top().(*problemScreen); !ok || h.m.open == nil {
			t.Fatalf("after the AC %T is on top, open %v; want the solved problem", h.m.top(), h.m.open != nil)
		}
		h.send(key("Q"))
		if h.m.open != nil {
			t.Error("upsolve door still open after Q")
		}
		if _, ok := h.m.top().(*rtfm.Map); !ok {
			t.Errorf("after Q %T is on top, want RTFM's map", h.m.top())
		}
	})
}

// A mystery door's statement must not name it: not in the reader's title,
// not in the statement's heading, not through the browser key.
func TestMysteryStatementHidesTitle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// A door whose deadline passed while the game was closed crashes on
// resume without its editor opening first (it used to open, then close on
// the first tick a second later).
func TestResumeAfterDeadlineCrashesWithoutReopening(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// An upsolve starts from the attempt that crashed the boot, so a core dump
// picks up where the boot left off; a due regression starts blank in a
// directory of its own, even when the shared upsolve directory holds last
// time's accepted solution (resubmitting that is not recall).
func TestUpsolveSources(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// A door's folder holds its statement under a list of where the rest is:
// the problem's page, then its editorial and what reading it there counts
// for. A boot door's editorial joins it on the AC, as a timed door allows
// none; an upsolve has it from the start, to ask about once solved, under
// a line saying a read there is not counted.
func TestDoorFolderNotes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var r *srcRunner
		h := newHarnessWith(t, func(f *fakeRunner) Runner {
			r = &srcRunner{fakeRunner: f, dir: t.TempDir()}
			return r
		})
		h.m.d.Catalog = editorialCatalog{h.m.d.Catalog.(*fakeCatalog)}
		kept := func(dir string) bool {
			b, err := os.ReadFile(filepath.Join(dir, "editorial.md"))
			return err == nil && strings.HasPrefix(string(b), editorialHead) && strings.Contains(string(b), "Sort, then sweep.")
		}

		h.send(screens.StartBoot{Preset: "standard"})
		cur := h.enterDoor()
		if kept(r.dir) {
			t.Fatal("an open boot door's folder has its editorial")
		}
		h.send(submitMsg{}) // the fake judge accepts
		if !kept(r.dir) {
			t.Fatal("no editorial beside the source after the AC")
		}

		// The tmux runner's layout, for a boot door and an upsolve.
		id := cur.Door.Problem
		a, err := h.m.d.Catalog.Fetch(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := h.m.d.Catalog.Get(id)
		p.URL = "https://usaco.org/index.php?page=viewproblem2&cpid=736"
		tr := &TmuxRunner{Root: t.TempDir()}
		for _, up := range []bool{false, true} {
			d := Door{Door: game.Door{Problem: id}, Problem: p, Assets: a, Boot: 1, Upsolve: up}
			dir, _, err := tr.materialize(d)
			if err != nil {
				t.Fatal(err)
			}
			want := "- Problem page: <" + p.URL + ">\n- Editorial: " + editorialWhere(d) + "\n\n# "
			if b, _ := os.ReadFile(filepath.Join(dir, "statement.md")); !strings.HasPrefix(string(b), want) {
				t.Errorf("upsolve %v: statement.md starts %q, want %q", up, firstN(string(b), len(want)), want)
			}
			if kept(dir) != up {
				t.Errorf("upsolve %v: editorial.md there %v", up, kept(dir))
			}
		}
		if w := editorialWhere(Door{Upsolve: true, Assets: a}); !strings.Contains(w, "not counted") {
			t.Errorf("an upsolve's statement.md says %q of its editorial, want that a read there is not counted", w)
		}
	})
}

func firstN(s string, n int) string { return s[:min(len(s), n)] }

// git log names the biome a boot crashed in. Userland's doors keep their
// own topics, so naming it by the last door's topic called a Userland
// crash "The Scheduler" (or whatever that topic's biome is).
func TestLogNamesTheCrashBiome(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
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
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// Quitting cancels the fetches still running and waits for them, so a
// download cut short can remove its temp files before the process exits.
func TestQuitCancelsFetches(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
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
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.d.Catalog = &breakingCatalog{fakeCatalog: h.m.d.Catalog.(*fakeCatalog), broken: map[catalog.ID]bool{}}
		h.send(screens.StartBoot{Preset: "standard"}) // the room's doors prefetch
		h.toDoors()
		id := h.m.state.Doors[0].Problem
		if i := slices.IndexFunc(h.m.pool, func(p catalog.Problem) bool { return p.ID == id }); h.m.pool[i].Judgeable {
			t.Fatalf("%s still judgeable in the pool after its fetch found it not", id)
		}
	})
}

// A source that joins mid-boot (Codeforces once its index is built) stays
// out of that boot's pool, even when a recheck rereads the catalog, so
// the boot keeps drawing the doors its seed made; the next boot draws from
// it. Until it joins, GRUB says how it stands.
func TestLateSourceWaitsForTheNextBoot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		note := "Codeforces indexing… 40%"
		h.m.d.Sources = func() string { return note }
		h.m.reset()
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, note) {
			t.Errorf("GRUB lacks the sources note:\n%s", v)
		}
		bc := &breakingCatalog{fakeCatalog: h.m.d.Catalog.(*fakeCatalog), broken: map[catalog.ID]bool{}}
		u := catalog.NewUnion(bc)
		h.m.d.Catalog = u
		h.send(screens.StartBoot{Preset: "standard"})
		late := newFakeCatalog()
		for i := range late.all {
			late.all[i].ID = catalog.NewID(catalog.CF, late.all[i].ID.Native())
			late.all[i].Source = catalog.CF
			late.byID[late.all[i].ID] = late.all[i]
		}
		u.Add(late)
		// A fetch finds a problem in the pool unjudgeable after the join.
		i := slices.IndexFunc(h.m.pool, func(p catalog.Problem) bool { return p.Judgeable })
		id := h.m.pool[i].ID
		bc.mu.Lock()
		bc.broken[id] = true
		bc.mu.Unlock()
		h.send(prefetchedMsg{id})
		if slices.ContainsFunc(h.m.pool, func(p catalog.Problem) bool { return p.Source == catalog.CF }) {
			t.Fatal("a source that joined mid-boot entered its pool")
		}
		if slices.ContainsFunc(h.m.pool, func(p catalog.Problem) bool { return p.ID == id && p.Judgeable }) {
			t.Fatalf("the recheck did not happen: %s still judgeable", id)
		}
		h.send(screens.StartBoot{Preset: "standard"})
		h.send(key("y")) // the hub asks before the boot in flight ends
		if !slices.ContainsFunc(h.m.pool, func(p catalog.Problem) bool { return p.Source == catalog.CF }) {
			t.Error("the next boot does not draw from the source that joined")
		}
	})
}

// A door's timer starts when its problem is on disk, not when it is
// chosen: a cold fetch took up to 56 s in play, all of it off the timer.
// A fetch that fails leaves the door unopened instead of open and doomed.
func TestChosenDoorWaitsForItsProblem(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		if last := h.lastLine(); h.m.state.Phase != game.PhaseDoors || !strings.Contains(last, "fetching door 1") {
			t.Fatalf("phase %q, last row %q while the door's problem downloads, want doors and the wait", h.m.state.Phase, last)
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
		h.m.fetch.chosen = id
		h.send(doorReadyMsg{index: 0, id: id, err: errors.New("offline")})
		if h.m.state.Phase != game.PhaseDoors || !strings.Contains(h.m.flash, "offline") {
			t.Fatalf("phase %q flash %q after a failed fetch, want doors and the error", h.m.state.Phase, h.m.flash)
		}
	})
}

// The problem screen fills the pane exactly: boot bar, the embedded HUD,
// then the two footer rows on the last two lines.
func TestProblemScreenFillsThePane(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
			if !strings.HasPrefix(vim, ":w ") || !strings.HasPrefix(keys, "s submit") {
				t.Fatalf("footer at height %d is %q / %q, want the vim and key help", size[1], vim, keys)
			}
			// Under 52 columns the vim row is bare command names and the
			// pane's drops Q, whose :q! the vim row has (C8).
			if narrow := size[0] < 52; narrow != !strings.Contains(vim, "samples") || narrow != !strings.Contains(keys, "Q ") {
				t.Errorf("footer at width %d is %q / %q", size[0], vim, keys)
			}
			// The :q! confirm takes the keys' row: the pane keeps its height.
			h.send(key("Q"))
			lines = strings.Split(h.m.View().Content, "\n")
			if last := ansi.Strip(lines[len(lines)-1]); len(lines) != size[1] || !strings.Contains(last, "Q again") {
				t.Errorf("confirm at %dx%d: %d lines, last %q", size[0], size[1], len(lines), last)
			}
			h.send(key("x")) // any other key cancels the confirm
		}
	})
}

// A submit the HUD refuses (no problem loaded) ends the judging at once,
// so it cannot hold the door's clock until the grace runs out.
func TestRefusedSubmitReleasesTheClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// Bytes spent at a merge window buy for the boot and are saved with it; a
// held shuf re-rolls the doors from the doors screen's s key.
func TestMergeSpend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// Outside a door ctrl+c quits at once.
func TestQuitOutsideDoor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		_, cmd := h.m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		if cmd == nil {
			t.Fatal("ctrl+c at GRUB did not quit")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("ctrl+c at GRUB: not tea.Quit")
		}
	})
}

// apt at GRUB buys a package with the bank and saves it; a refused buy
// says why and changes nothing; installing a theme puts it in use, and
// UseTheme goes back to the default.
func TestShopInstall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
	})
}

// After an install the app rebuilds GRUB under the shop; its cursor stays
// on apt, not back at the top (A2 leftover).
func TestShopInstallKeepsGrubCursor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.meta.Bank = 100
		onApt := func() bool {
			for _, l := range strings.Split(ansi.Strip(h.m.stack[0].View(100, 30)), "\n") {
				if strings.Contains(l, "│*apt") {
					return true
				}
			}
			return false
		}
		for i := 0; i < 30 && !onApt(); i++ {
			h.send(tea.KeyPressMsg{Code: tea.KeyDown})
		}
		if !onApt() {
			t.Fatalf("no apt entry in GRUB:\n%s", h.m.stack[0].View(100, 30))
		}
		h.send(tea.KeyPressMsg{Code: tea.KeyEnter})
		if _, ok := h.m.top().(*meta.Shop); !ok {
			t.Fatalf("enter on apt: top is %T, want the shop", h.m.top())
		}
		h.send(screens.Install{Package: "cowsay"})
		h.send(screens.Back{})
		if !onApt() {
			t.Errorf("GRUB's cursor left apt after the install:\n%s", h.m.stack[0].View(100, 30))
		}
	})
}

// The editor border's clock is a tmux format in the HUD's tones (E15):
// reverse red in the last two minutes, the lost hearts drawn too, and the
// strip's timer warnings.
func TestBorderClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		if got := h.m.BorderClock(); got != "⏱ untimed" {
			t.Errorf("BorderClock outside a door = %q", got)
		}
		h.send(screens.StartBoot{Preset: "standard"})
		cur := h.enterDoor()
		h.clock.t = cur.Deadline.Add(-90 * time.Second)
		got := h.m.BorderClock()
		for _, want := range []string{"#[fg=colour1,reverse]⏱ 01:30#[default]", "#[fg=colour1]♥", "🦆"} {
			if !strings.Contains(got, want) {
				t.Errorf("BorderClock = %q, want %q in it", got, want)
			}
		}
		// The strip's warnings, for a moment after each mark.
		for _, c := range []struct {
			left time.Duration
			warn string
		}{
			{5*time.Minute - 3*time.Second, ui.Warn.Tmux("05:00 left")},
			{4*time.Minute + 30*time.Second, ""},
			{time.Minute - 3*time.Second, ui.Danger.Tmux("01:00 left")},
			{90 * time.Second, ""},
		} {
			h.clock.t = cur.Deadline.Add(-c.left)
			got := h.m.BorderClock()
			if warned := strings.Contains(got, " left"); warned != (c.warn != "") || !strings.Contains(got, c.warn) {
				t.Errorf("%v left: BorderClock = %q, want the warning %q", c.left, got, c.warn)
			}
		}
		if p, n := h.m.Best(); p != cur.BestPass || n != cur.Total {
			t.Errorf("Best = %d/%d, want the attempt's %d/%d", p, n, cur.BestPass, cur.Total)
		}
	})
}

// zoomRunner records the zooms the app asks for.
type zoomRunner struct {
	*fakeRunner
	zooms []bool
}

func (r *zoomRunner) Zoom(on bool) error { r.zooms = append(r.zooms, on); return nil }

// A statement opened in a pane too narrow to read in zooms the pane, and
// unzooms it when the reader goes, by q or with the door (E14).
func TestReaderZoomsANarrowPane(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var zr *zoomRunner
		h := newHarnessWith(t, func(f *fakeRunner) Runner { zr = &zoomRunner{fakeRunner: f}; return zr })
		doc := docMsg{doc: reader.Doc{Title: "Favorite Sequence", Markdown: "Given an array."}}

		h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
		h.send(doc)
		h.send(screens.Back{})
		if len(zr.zooms) != 0 {
			t.Fatalf("an 80-column pane zoomed: %v", zr.zooms)
		}

		h.send(tea.WindowSizeMsg{Width: 41, Height: 24})
		h.send(doc)
		if !slices.Equal(zr.zooms, []bool{true}) {
			t.Fatalf("a 41-column pane: zooms %v, want one zoom", zr.zooms)
		}
		h.send(tea.WindowSizeMsg{Width: 120, Height: 24}) // zoomed: the pane is the window
		h.send(screens.Back{})
		if !slices.Equal(zr.zooms, []bool{true, false}) {
			t.Errorf("after q: zooms %v, want zoom then unzoom", zr.zooms)
		}
	})
}

// emitted runs cmd and the commands it batches or sequences, in order, and
// returns their messages. tea.Sequence's message type is unexported; like
// tea.BatchMsg, it is a slice of commands.
func emitted(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Type().Elem() != reflect.TypeFor[tea.Cmd]() {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for i := range v.Len() {
		out = append(out, emitted(v.Index(i).Interface().(tea.Cmd))...)
	}
	return out
}

// A resize resets the terminal's scroll region, then repaints in full: a
// frame drawn for the old height left tmux scrolling only its rows, and a
// crash's reveal, as the editor closed and the game's pane grew, carried
// the boot-end line up into the panic until the next resize.
func TestResizeResetsScrollRegion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		_, cmd := h.m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		msgs := emitted(cmd)
		reset := slices.IndexFunc(msgs, func(m tea.Msg) bool {
			r, ok := m.(tea.RawMsg)
			return ok && r.Msg == "\x1b[r"
		})
		clear := slices.IndexFunc(msgs, func(m tea.Msg) bool {
			return reflect.TypeOf(m) == reflect.TypeOf(tea.ClearScreen())
		})
		if reset < 0 || clear < reset {
			t.Fatalf("a resize sends %#v, want the scroll region reset, then a clear screen", msgs)
		}
	})
}

// A refused action flashes the engine's reason as the player reads it,
// without its "invalid action" prefix: the merge window said "invalid
// action: dup costs 48 B, 2 B uncommitted".
func TestRefusalFlashHasNoPrefix(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.send(game.Spend{Item: game.Dup}) // no merge window open
		if h.m.flash == "" || strings.Contains(h.m.flash, game.ErrInvalid.Error()) {
			t.Fatalf("refused spend flashed %q, want the reason alone", h.m.flash)
		}
	})
}

// The resume note is about the door that resumed: it goes when the screen
// changes, here to the crash its timer ran into, and does not linger over
// the panic until a key.
func TestResumeNoteLeavesWithTheDoor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		cur := h.enterDoor()
		h.clock.t = cur.Deadline.Add(-time.Minute)
		h.restart()
		note := h.m.flash
		if !strings.Contains(note, "left") { // resume.note
			t.Fatalf("resumed door: flash %q, want the resume note", note)
		}
		h.clock.t = cur.Deadline.Add(time.Second)
		h.send(tickMsg(h.clock.t))
		if h.m.state.Phase != game.PhaseCrashed {
			t.Fatalf("phase %q, want the timeout crash", h.m.state.Phase)
		}
		if h.m.flash == note || strings.Contains(ansi.Strip(h.m.View().Content), ansi.Strip(note)) {
			t.Errorf("the resume note %q lingers on the crash screen", note)
		}
	})
}

// hudZoomRunner is a door with a HUD in a pane that zooms.
type hudZoomRunner struct {
	*hudFakeRunner
	zooms []bool
}

func (r *hudZoomRunner) Zoom(on bool) error { r.zooms = append(r.zooms, on); return nil }

// w on a door shows its samples in full (screens/samples), zoomed over
// the editor in a pane too narrow to set them side by side; w again goes
// back to the door and unzooms.
func TestSamplesView(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var zr *hudZoomRunner
		h := newHarnessWith(t, func(f *fakeRunner) Runner {
			zr = &hudZoomRunner{hudFakeRunner: &hudFakeRunner{fakeRunner: f, opt: hud.Options{Dir: t.TempDir(), Embedded: true}}}
			return zr
		})
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(tea.WindowSizeMsg{Width: 80, Height: 30})
		h.send(key("w"))
		if _, ok := h.m.top().(*samples.Model); !ok {
			t.Fatalf("w opened a %T", h.m.top())
		}
		if !slices.Equal(zr.zooms, []bool{true}) {
			t.Errorf("an 80-column pane: zooms %v, want one zoom", zr.zooms)
		}
		h.send(tea.WindowSizeMsg{Width: 160, Height: 30}) // zoomed: the pane is the window
		if v := view(h); !strings.Contains(v, "samples") || !strings.Contains(v, "w close") {
			t.Errorf("the samples view:\n%s", v)
		}
		h.send(key("w"))
		if _, ok := h.m.top().(*problemScreen); !ok {
			t.Errorf("w again left a %T on top", h.m.top())
		}
		if !slices.Equal(zr.zooms, []bool{true, false}) {
			t.Errorf("after w: zooms %v, want zoom then unzoom", zr.zooms)
		}
	})
}

// A door a boot lost, upsolved from GRUB before its regression is due,
// picks up from the code written in that boot, found by its last attempt;
// the regression stays due on its date.
func TestUpsolveLostDoor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		cur := h.enterDoor()
		id := cur.Door.Problem
		boot := h.m.state.Boot
		h.send(game.GiveUp{})
		if len(h.m.meta.Regressions) == 0 || !h.m.meta.Regressions[0].Due.After(h.m.d.Now()) {
			t.Fatalf("the give-up scheduled no regression ahead: %+v", h.m.meta.Regressions)
		}
		// Between boots, the dump gone: only the lost door is left.
		h.m.state, h.m.meta.CoreDump = nil, nil
		if sub, seed := h.m.upsolveFiles(id); sub != "" || seed != boot {
			t.Errorf("lost door upsolves in %q from boot %d, want the shared directory from boot %d", sub, seed, boot)
		}
		if due := h.m.meta.Regressions[0].Due; !due.After(h.m.d.Now()) {
			t.Errorf("the regression moved to %s", due)
		}
	})
}

// g on the map pauses into GRUB, the boot suspended under it; GRUB's
// continue goes back to where the boot stopped; a boot started from a
// paused GRUB asks first, and y ends the paused boot as a crash and boots
// the next. A rebuild of the paused GRUB (an apt install) keeps it where
// it is, still offering to continue.
func TestPauseIntoGrub(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		boot := h.m.state.Boot
		h.send(key("m"))
		h.send(key("g"))
		g, ok := h.m.top().(*meta.Menu)
		if !ok || h.m.state == nil || !h.m.suspended() {
			t.Fatalf("g: top %T, suspended %v", h.m.top(), h.m.suspended())
		}
		if v := view(h); !strings.Contains(v, fmt.Sprintf("continue boot #%d", boot)) {
			t.Fatalf("paused GRUB:\n%s", v)
		}
		h.m.regrub()
		if _, ok := h.m.top().(*meta.Menu); !ok || h.m.top() == screens.Screen(g) || !strings.Contains(view(h), "continue boot") {
			t.Errorf("regrub left %T on top", h.m.top())
		}

		h.send(tea.KeyPressMsg{Code: tea.KeyEnter})
		if len(h.m.stack) != 1 || h.m.state == nil || h.m.state.Boot != boot {
			t.Fatalf("continue: %d screens, %T on top", len(h.m.stack), h.m.top())
		}
		if _, ok := h.m.top().(*meta.Menu); ok {
			t.Fatal("continue stayed in GRUB")
		}

		h.send(key("m"))
		h.send(key("g"))
		h.send(tea.KeyPressMsg{Code: tea.KeyDown})
		h.send(tea.KeyPressMsg{Code: tea.KeyEnter}) // the highest prestige: asks
		if h.m.state.Boot != boot || !strings.Contains(view(h), fmt.Sprintf("Boot #%d is in flight", boot)) {
			t.Fatalf("a new boot did not ask:\n%s", view(h))
		}
		h.send(key("y"))
		if h.m.state == nil || h.m.state.Boot != boot+1 {
			t.Fatalf("y: boot %v, want #%d", h.m.state, boot+1)
		}
	})
}

// An upsolve solved from GRUB over a paused boot goes back to GRUB, the
// boot still paused under it, not to the boot's own screen.
func TestUpsolveBackToPausedGrub(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.acFirst = 1
		h.enterDoor()
		h.send(submitMsg{})
		var id catalog.ID // a door an earlier boot lost, say
		for _, p := range h.m.d.Catalog.All() {
			if !h.m.onOffer(p.ID) {
				id = p.ID
				break
			}
		}
		h.send(key("m"))
		h.send(key("g"))
		if _, ok := h.m.top().(*meta.Menu); !ok || !h.m.suspended() {
			t.Fatalf("g: top %T, suspended %v", h.m.top(), h.m.suspended())
		}
		h.verdict = "AC"
		h.send(screens.Upsolve{Problem: id})
		if h.m.open == nil || !h.m.open.Upsolve {
			t.Fatal("upsolve did not open the problem")
		}
		h.send(submitMsg{})
		h.send(key("Q")) // solved, it stays open until left
		if _, ok := h.m.top().(*meta.Menu); !ok || !h.m.suspended() || !strings.Contains(view(h), "continue boot") {
			t.Fatalf("after the solve and Q: top %T, suspended %v:\n%s", h.m.top(), h.m.suspended(), view(h))
		}
	})
}
