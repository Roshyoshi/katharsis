// Package app is the game: it routes between screens, feeds player choices
// and judge results to the engine, saves after every step, and carries out
// the engine's effects (open a door, play a sound, show the crash). Screens
// and the engine stay pure; everything with side effects is reached through
// Deps so the whole loop runs headless in tests.
package app

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/session"
	"katharsis/internal/store"
	"katharsis/internal/units"
)

// Runner sets up and tears down the editing side of an open door: files on
// disk, the editor pane, the file watcher. Results come back as messages.
type Runner interface {
	Open(ctx context.Context, d Door) error
	Close()
}

// Door is everything a Runner needs to put a problem in front of the player.
type Door struct {
	Door    game.Door
	Problem catalog.Problem
	Assets  catalog.Assets
	Boot    int
	Upsolve bool // untimed, outside a boot
	Opened  time.Time
	// ReadEditorial marks an upsolve whose editorial was opened, so the
	// solve is recorded as editorial-assisted.
	ReadEditorial bool
	// Sub, for an upsolve, is its directory under the runner's root; ""
	// is the shared upsolve directory.
	Sub string
	// SeedBoot, for an upsolve, is the boot whose attempt at the problem
	// a new upsolve source starts from; 0 starts from the template.
	SeedBoot int
	// Libs are the boot's linked libs, pasted above main when the door's
	// source is first written.
	Libs []game.Lib
}

// Judge runs the full tests of an open door, for a Runner without a HUD
// (the headless tests). The app calls it on :Submit; it must be safe to
// call from a goroutine.
type Judge func(ctx context.Context, d Door) (verdict string, passed, total int, err error)

type Deps struct {
	Store   *store.Store
	Catalog catalog.Catalog
	// Presets are rule presets by name; "standard" is added if missing.
	Presets map[string]rules.Rules
	Runner  Runner
	Judge   Judge
	Now     func() time.Time
	Sound   func(name string) // nil plays nothing
	Seed    func() uint64     // nil uses math/rand
}

// Model is the root tea.Model.
type Model struct {
	d     Deps
	meta  game.Meta
	state *game.State // nil outside a boot
	// pool is the catalog as the engine draws doors from it; recheck and
	// every boot start reload it.
	pool []catalog.Problem
	cur  *curriculum.Curriculum // RTFM, loaded on first use

	stack   []screens.Screen // top is shown; the bottom is GRUB or a boot screen
	open    *Door            // the door being played, nil otherwise
	judging bool
	flash   string // one-line error shown over the screen until the next key

	// editorials are the problems whose editorial was opened this session,
	// such as from the crash screen before any upsolve exists.
	editorials map[catalog.ID]bool
	// choosing is the door chosen while its problem is still being
	// fetched; it opens when the fetch lands (chooseDoor).
	choosing catalog.ID
	// prefetching cancels the prefetch of each door on offer.
	prefetching map[catalog.ID]context.CancelFunc

	// ctx bounds the background fetches (background). Quit cancels it and
	// waits for them, so a download cut short removes its temp files
	// before the process exits.
	ctx     context.Context
	cancel  context.CancelFunc
	fetches sync.WaitGroup

	width, height int
}

func New(d Deps) (*Model, error) {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Seed == nil {
		d.Seed = func() uint64 { return rand.Uint64() }
	}
	if d.Presets == nil {
		d.Presets = map[string]rules.Rules{}
	}
	if _, ok := d.Presets["standard"]; !ok {
		d.Presets["standard"] = rules.Standard()
	}
	meta, err := d.Store.LoadMeta()
	if err != nil {
		return nil, err
	}
	st, err := d.Store.ActiveBoot()
	if err != nil {
		return nil, err
	}
	m := &Model{d: d, meta: meta, state: st, pool: d.Catalog.All(), editorials: map[catalog.ID]bool{}}
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.reset()
	return m, nil
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Init() tea.Cmd {
	// A boot resumed in the middle of a door reopens it; its deadline is
	// absolute, so time spent away still counted. A door that ran out while
	// the game was closed crashes now, rather than opening the editor only
	// for the first tick to close it again. A soft timer (a trial) only
	// goes overtime, so its door reopens.
	if s := m.state; s != nil && s.Phase == game.PhaseProblem && s.Current != nil {
		if !m.d.Now().Before(s.Current.Deadline) && !s.SoftTimer() {
			return tea.Batch(tick(), m.step(game.Tick{}))
		}
		return tea.Batch(tick(), m.initTop(), m.openDoor(s.Current.Door, false))
	}
	return tea.Batch(tick(), m.initTop())
}

// reset rebuilds the screen stack from scratch for the current phase.
func (m *Model) reset() {
	m.stack = []screens.Screen{m.baseScreen()}
}

func (m *Model) top() screens.Screen { return m.stack[len(m.stack)-1] }

func (m *Model) push(s screens.Screen) tea.Cmd {
	m.stack = append(m.stack, s)
	return m.initTop()
}

func (m *Model) pop() {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
	}
}

// initTop runs a screen's optional Init (the crash screen's reveal).
func (m *Model) initTop() tea.Cmd {
	if s, ok := m.top().(interface{ Init() tea.Cmd }); ok {
		return s.Init()
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		m.flash = ""
		if msg.String() == "ctrl+c" {
			return m, m.quit()
		}
	case tickMsg:
		return m, tea.Batch(tick(), m.onTick())
	case game.ChooseDoor:
		return m, m.chooseDoor(msg)
	case doorReadyMsg:
		return m, m.onDoorReady(msg)
	case prefetchedMsg:
		m.recheck(msg...)
		return m, nil
	case game.Action:
		return m, m.step(msg)
	case judgedMsg:
		return m, m.onJudged(msg)
	case hud.SubmittedMsg:
		if msg.Err != nil {
			return m, m.onJudged(judgedMsg{err: msg.Err})
		}
		s := msg.Summary
		return m, m.onJudged(judgedMsg{verdict: string(s.Verdict), passed: s.Passed, total: s.Total})
	case session.Request:
		return m, m.onRequest(msg)
	case hudQuitMsg:
		for _, s := range m.stack {
			if p, ok := s.(*problemScreen); ok {
				p.closed = true
			}
		}
		// The HUD cancels its submit pass when it quits; no result comes.
		m.judging = false
		return m, nil
	case fetchedMsg:
		return m, m.onFetched(msg)
	case errMsg:
		m.flash = msg.err.Error()
		return m, nil
	case noteMsg:
		m.flash = msg.text
		return m, nil
	}
	if cmd, ok := m.intent(msg); ok {
		return m, cmd
	}
	// Keys belong to the screen on top. Everything else (timers, judge
	// events, a reveal animation) goes to every screen in the stack and to
	// the open door's HUD: a screen under another one, or the HUD under the
	// statement reader, must keep its own clock and passes running.
	if _, key := msg.(tea.KeyPressMsg); key {
		s, cmd := m.top().Update(msg)
		m.stack[len(m.stack)-1] = s
		return m, cmd
	}
	var cmds []tea.Cmd
	for i := len(m.stack) - 1; i >= 0; i-- {
		s, cmd := m.stack[i].Update(msg)
		m.stack[i] = s
		cmds = append(cmds, cmd)
	}
	// The problem screen sizes the HUD to the room below its own bar, so
	// the window size is not passed on to it.
	_, resize := msg.(tea.WindowSizeMsg)
	if h := m.hud(); h != nil && !resize {
		_, cmd := h.Update(msg)
		cmds = append(cmds, guard(cmd))
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) View() tea.View {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		w, h = 80, 24
	}
	body := m.top().View(w, h)
	if m.flash != "" {
		body = overlayFlash(body, m.flash, w)
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

// quitWait bounds how long quitting waits for cancelled fetches to clean
// up; a fetch that ignores its context is left behind.
const quitWait = 2 * time.Second

func (m *Model) quit() tea.Cmd {
	if m.open != nil {
		m.d.Runner.Close()
	}
	m.cancel()
	done := make(chan struct{})
	go func() { m.fetches.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(quitWait):
	}
	return tea.Quit
}

// background runs work in a command under the app's lifetime: quit cancels
// ctx and waits for work to return. After quit it runs nothing, so no new
// work can start while quit waits.
func (m *Model) background(work func(ctx context.Context) tea.Msg) tea.Cmd {
	if m.ctx.Err() != nil {
		return nil
	}
	ctx, wg := m.ctx, &m.fetches
	wg.Add(1)
	return func() tea.Msg {
		defer wg.Done()
		return work(ctx)
	}
}

type errMsg struct{ err error }

// noteMsg flashes a message that is not an error.
type noteMsg struct{ text string }

func note(format string, args ...any) tea.Cmd {
	text := fmt.Sprintf(format, args...)
	return func() tea.Msg { return noteMsg{text} }
}

func fail(format string, args ...any) tea.Cmd {
	err := fmt.Errorf(format, args...)
	return func() tea.Msg { return errMsg{err} }
}

// Conceal reports whether the open door's full tests are hidden: in a boot
// unless the engine says its failing input shows (the strace daemon, or a
// preset that shows it). Upsolves never hide them.
func (m *Model) Conceal() bool {
	if m.state == nil || m.open == nil || m.open.Upsolve {
		return false
	}
	return !m.state.ShowsFailingInput()
}

// Clock is the HUD's clock in a boot: the countdown with lives and ducks,
// e.g. "23:41 · ♥♥ · 🦆1". Outside a boot it counts up, as in practice.
func (m *Model) Clock() string {
	s := m.state
	if s == nil || s.Phase != game.PhaseProblem || s.Current == nil || (m.open != nil && m.open.Upsolve) {
		return "untimed"
	}
	left := s.Current.Deadline.Sub(m.d.Now())
	t := units.Countdown(left)
	// A trial's timer is soft: past the deadline the door goes on at half
	// pay, so show how far over it is instead of a frozen 00:00.
	if left < 0 && s.SoftTimer() {
		t = "+" + units.Clock(-left) + " over"
	}
	return fmt.Sprintf("%s · %s · %s", t, units.Hearts(s.Lives, s.MaxLives), ducks(s))
}
