// Package app is the game: it routes between screens, feeds player choices
// and judge results to the engine, saves after every step, and carries out
// the engine's effects (open a door, play a sound, show the crash). Screens
// and the engine stay pure; everything with side effects is reached through
// Deps so the whole loop runs headless in tests.
package app

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/audio"
	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/problem"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/session"
	"katharsis/internal/store"
	"katharsis/internal/ui"
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
	// Solved marks an upsolve accepted while it stays open: the solve is
	// counted once, and a submit after it only shows its verdict.
	Solved bool
	// Sub, for an upsolve, is its directory under the runner's root; ""
	// is the shared upsolve directory.
	Sub string
	// SeedBoot, for an upsolve, is the boot whose attempt at the problem
	// a new upsolve source starts from; 0 starts from the template.
	SeedBoot int
	// Libs are the boot's linked libs, pasted above main when the door's
	// source is first written.
	Libs []game.Lib
	// Failed are the tests the door's last submit failed, for the duck's
	// debugging quack (D64, duck.go).
	Failed []problem.Test
}

// Judge runs the full tests of an open door, for a Runner without a HUD
// (the headless tests). The app calls it on :Submit; it must be safe to
// call from a goroutine.
type Judge func(ctx context.Context, d Door) (verdict string, passed, total int, err error)

type Deps struct {
	Store   *store.Store
	Catalog catalog.Catalog
	// Presets are rule presets by name; the built-ins (rules.BuiltIns:
	// standard, short, patch) are added where missing.
	Presets map[string]rules.Rules
	Runner  Runner
	Judge   Judge
	Now     func() time.Time
	Seed    func() uint64 // nil uses math/rand
	// Audio plays the effects and the music the state calls for
	// (audio.go); nil is silence. AudioSettings gives the timer warning
	// and how long a track's credit shows, and Credits is the credits
	// page GRUB opens.
	Audio         audio.Sink
	AudioSettings audio.Settings
	Credits       func() string
	// Log gets what goes wrong out of the player's sight, such as an event
	// the store could not write; nil discards it.
	Log *log.Logger
	// Prompts turns on the questions that wait for the player: the
	// first-launch rating (D37) and the boot-end line. The game sets it;
	// headless tests leave it off.
	Prompts bool
	// Story plays the story's scenes at their moments (story.go). The game
	// sets it; headless tests leave it off.
	Story bool
	// Title shows the title screen at launch, before GRUB (title.go). The
	// game sets it; headless tests leave it off.
	Title bool
	// Sources, when set, is a note on content sources not serving yet
	// ("Codeforces indexing… 40%"), which GRUB shows as it changes.
	Sources func() string
	// Budget is the session budget (retention RT12): past this much play
	// today, the end of a boot says so in one line. 0 turns it off.
	Budget time.Duration
	// SetuidNotices turns on the setuid bits' notices (setuid_notices,
	// docs/design/achievements.md (h)): the strip's chmod u+s, the cleared
	// screen's kept line, GRUB's "setuid +1" after a crash. The game sets
	// it from settings.toml; headless tests leave it off. Spoilers is
	// setuid_spoilers, what find / -perm -4000 shows of a secret (meta.
	// SpoilersNone for "").
	SetuidNotices bool
	Spoilers      string
	// SetuidCheck and SetuidBackfill stand in for setuid.Check and
	// Store.BackfillSetuid, for a test that fakes the bits; nil is them.
	SetuidCheck    func(m *game.Meta, s *game.State, now time.Time) []game.SetuidBit
	SetuidBackfill func(m *game.Meta, now time.Time) ([]game.SetuidBit, error)
	// Accounts are the logins passwd offers (settings.go); nil hides its
	// accounts section. Settings lists the settings in effect there; nil
	// lists none.
	Accounts []Account
	Settings func() []Setting
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
	// rtfmCaps is the capabilities RTFM's map last showed this session, nil
	// before it first did: those granted since get its switch palace.
	rtfmCaps []string

	stack   []screens.Screen // top is shown; the bottom is GRUB or a boot screen
	open    *Door            // the door being played, nil otherwise
	judging bool
	// resumed is a submit this game judges again: the last game quit
	// while judging it (Attempt.SubmitAt). Zero otherwise.
	resumed resumedSubmit
	flash   string // one-line error shown over the screen until the next key
	// resumeFlash is the resume note (resumeNote) while it is the flash:
	// it is about the door that resumed, so it goes with the screen.
	resumeFlash string
	// saveErr is the last save that failed. It shows on every screen
	// until a save succeeds: as a flash the next key cleared, a store the
	// game could not write played a whole boot and kept none of it.
	saveErr    error
	saveFailed time.Time // when saveErr happened
	// doorQuit is a ctrl+c refused inside a door (hub.go), until the next
	// key; the problem pane draws it.
	doorQuit quitRefusal
	// rtfmLoading is set while RTFM loads for a route's locked exits.
	rtfmLoading bool

	// editorials are when each problem's editorial was last opened this
	// session, such as from the crash screen before any upsolve exists.
	editorials map[catalog.ID]time.Time
	// prefetching cancels the prefetch of each door on offer.
	prefetching map[catalog.ID]context.CancelFunc

	// ctx bounds the background fetches (background). Quit cancels it and
	// waits for them, so a download cut short removes its temp files
	// before the process exits.
	ctx     context.Context
	cancel  context.CancelFunc
	fetches sync.WaitGroup

	tele tele // telemetry.go

	width, height int

	dmesg bootui.Tracker // the boot's event strip (dmesg.go)
	// fetch is each door's download and the door waited on (fetch.go).
	fetch doorFetch

	// Sound (audio.go): the last state effects were diffed from, the door
	// whose timer warning sounded, and the credit toast.
	audioPrev   audioSnap
	warned      time.Time
	toast       string
	toastUntil  time.Time
	toastSerial uint64

	// zoomed is the reader the game's pane was zoomed for (zoom.go).
	zoomed screens.Screen

	story storyState // story.go

	end endCache // the end screens' figures (debrief.go)

	setuid setuidState // the setuid bits' notices (setuid.go)
}

func New(d Deps) (*Model, error) {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Seed == nil {
		d.Seed = func() uint64 { return rand.Uint64() }
	}
	if d.Audio == nil {
		d.Audio = audio.Null{}
	}
	if d.Presets == nil {
		d.Presets = map[string]rules.Rules{}
	}
	for _, r := range rules.BuiltIns() {
		if _, ok := d.Presets[r.Name]; !ok {
			d.Presets[r.Name] = r
		}
	}
	meta, err := d.Store.LoadMeta()
	if err != nil {
		return nil, err
	}
	st, err := d.Store.ActiveBoot()
	if err != nil {
		return nil, err
	}
	m := &Model{d: d, meta: meta, state: st, pool: drawPool(d.Catalog), editorials: map[catalog.ID]time.Time{}}
	m.setuid.notices = d.SetuidNotices
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.startTelemetry()
	// Once, for a save from before the bits (setuid.go): before the first
	// screen, so GRUB counts what it set.
	m.backfillSetuid()
	m.backfillRTFM()
	m.reset()
	m.askPrior()
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
	// goes overtime, so its door reopens. A submit the last game quit while
	// judging is judged again first, as of when it was made (resubmit): the
	// door opens for it, and the clock holds.
	if c := m.attempt(); c != nil {
		if at := c.SubmitAt; !at.IsZero() {
			m.judging, m.resumed = true, resumedSubmit{at: at, since: m.d.Now()}
		} else if !m.d.Now().Before(c.Deadline) && !m.state.SoftTimer() {
			return tea.Batch(tick(), m.step(game.Tick{}))
		}
		m.flash = resumeNote(c.Deadline.Sub(m.d.Now())) // debrief.go
		m.resumeFlash = m.flash
		return tea.Batch(tick(), m.initTop(), m.openDoor(c.Door, false))
	}
	return tea.Batch(tick(), m.initTop(), m.launchAt(), m.titleAtLaunch()) // the hub between doors (hub.go)
}

// doorLeft is the time left on the boot's open door, ok false when no
// timed door is open. A soft timer (a trial) also keeps running; past its
// deadline it is overtime, shown as negative.
func (m *Model) doorLeft() (time.Duration, bool) {
	c := m.attempt()
	if c == nil {
		return 0, false
	}
	return c.Deadline.Sub(m.d.Now()), true
}

// attempt is the boot's open door, a trial's too; nil between doors. An
// upsolve is never one: it opens only outside a boot's door.
func (m *Model) attempt() *game.Attempt {
	if s := m.state; s != nil && s.Phase == game.PhaseProblem {
		return s.Current
	}
	return nil
}

// leftText is "12:34 left", or "overtime" past a soft deadline.
func leftText(left time.Duration) string {
	if left <= 0 {
		return "overtime"
	}
	return units.Countdown(left) + " left"
}

// reset rebuilds the screen stack from scratch for the current phase. The
// resume note leaves with the screen it was about.
func (m *Model) reset() {
	m.dropped(m.stack...)
	m.stack = []screens.Screen{m.baseScreen()}
	if m.resumeFlash != "" && m.flash == m.resumeFlash {
		m.flash = ""
	}
	m.resumeFlash = ""
	m.screenShown()
}

func (m *Model) top() screens.Screen { return m.stack[len(m.stack)-1] }

func (m *Model) push(s screens.Screen) tea.Cmd {
	m.stack = append(m.stack, s)
	return m.initTop()
}

func (m *Model) pop() {
	if len(m.stack) > 1 {
		m.dropped(m.top())
		m.stack = m.stack[:len(m.stack)-1]
		// The screen shown again may show Meta that changed over it: a
		// page opened, a practice problem solved.
		if r, ok := m.top().(interface{ Refresh(game.Meta) }); ok {
			r.Refresh(m.meta)
		}
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
	defer m.syncAudio()
	defer m.syncZoom()
	defer m.syncDmesg()
	defer m.syncFetch()
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		m.flash, m.doorQuit = "", quitRefusal{}
		switch msg.String() {
		case "ctrl+c":
			return m, m.ctrlC() // never from inside a door (hub.go)
		case musicKey:
			return m, m.toggleMusic()
		}
		if m.endKey(msg) {
			return m, nil
		}
		if cmd, ok := m.hubKey(msg); ok { // m: the hub; q: suspend (hub.go)
			return m, cmd
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
	case hud.SamplesMsg:
		m.samplesJudged(msg)
		return m, m.samplesRun() // a :w, counted on a boot's door (setuid.go)
	case game.Action:
		return m, m.step(msg)
	case judgedMsg:
		return m, m.onJudged(msg)
	case hud.SubmittedMsg:
		if msg.Err != nil {
			return m, m.onJudged(judgedMsg{err: msg.Err}) // the HUD answered it
		}
		if m.open != nil {
			m.open.Failed = msg.Failed // what the duck can debug
		}
		s, lives, sample := msg.Summary, m.doorLives(), m.sampleFailed(msg.Failed)
		cmd := m.onJudged(judgedMsg{verdict: string(s.Verdict), passed: s.Passed, total: s.Total, sample: sample})
		cost := m.charged(lives)
		if cost == "" && sample {
			cost = sampleFree
		}
		msg.Answer(cost) // hud.Options.HostAnswers
		return m, cmd
	case session.Request:
		return m, m.onRequest(msg)
	case hudQuitMsg:
		// An editor that died as it opened could not start; say so, with
		// the command, rather than only that it closed.
		failed := ""
		if r, ok := m.d.Runner.(interface{ EditorFailed() string }); ok {
			failed = r.EditorFailed()
		}
		for _, s := range m.stack {
			if p, ok := s.(*problemScreen); ok {
				p.closed, p.editorErr = true, failed
			}
		}
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
	if resize {
		// The renderer ends each hardware scroll by setting the scroll
		// region to the height it last knew (ESC[1;23r), not to the whole
		// screen. A frame drawn just before tmux grew the pane left its
		// last row outside the region, and the repaint after the resize
		// moves down by line feeds, which then scroll: a crash's reveal,
		// as the editor closed and the game's pane grew to the window,
		// carried the boot-end line up into the panic. Reset the region
		// (ESC[r is the whole screen), then repaint over it.
		cmds = append(cmds, tea.Sequence(tea.Raw("\x1b[r"), tea.ClearScreen))
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) View() tea.View {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		w, h = 80, 24
	}
	body := m.body(w, h) // retention.go
	m.story.drawn = body
	rows := 0 // the flash's
	if flash := cmp.Or(m.flash, m.toastLine()); flash != "" {
		body, rows = overlayFlash(body, flash, w)
	}
	if m.saveErr != nil {
		body = overlayBanner(body, "save failed, progress is not being kept: "+m.saveErr.Error(), w, rows)
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

// quitWait bounds how long quitting waits for cancelled fetches to clean
// up; a fetch that ignores its context is left behind.
const quitWait = 2 * time.Second

func (m *Model) quit() tea.Cmd {
	m.stopTelemetry()
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

// Clock is the HUD's clock in a boot: the countdown with cycles and ducks,
// e.g. "23:41 · ♥ 24/30 cycles · 🦆1"; on The Halting Problem's doors the
// countdown is hidden (bootui.HiddenClock). Outside a boot it counts up,
// as in practice.
func (m *Model) Clock() string {
	s, c := m.state, m.attempt()
	if c == nil {
		return "untimed"
	}
	left := c.Deadline.Sub(m.d.Now())
	t := units.Countdown(left)
	switch {
	case bootui.HiddenClock(s):
		t = bootui.ClockHidden
	case left < 0 && s.SoftTimer():
		// A trial's timer is soft: past the deadline the door goes on at
		// half pay, so show how far over it is instead of a frozen 00:00.
		t = "+" + units.Clock(-left) + " over"
	}
	return fmt.Sprintf("%s · %s · %s", t, units.Cycles(s.Cycles, s.MaxCycles), ducks(s))
}

// BorderClock is Clock as a tmux format for the editor pane's border, the
// timer the player watches while typing (E15, fix 3): the countdown in its
// tone (ui.BorderTimer: bold in the last fifth, reverse under two minutes),
// or hidden on The Halting Problem's doors; a soft timer's overrun in
// Danger; the cycles as the HUD draws them, "♥ 24/30 cycles"; and for a
// moment the strip's timer warning (borderWarn), which a hidden clock
// still gives.
func (m *Model) BorderClock() string {
	s, c := m.state, m.attempt()
	if c == nil {
		return ui.TmuxEscape(ui.G.Timer + "untimed")
	}
	left := c.Deadline.Sub(m.d.Now())
	t := ui.BorderTimer(left, c.Door.Timer)
	switch {
	case bootui.HiddenClock(s):
		t = ui.Muted.Tmux(ui.G.Timer + bootui.ClockHidden)
	case left < 0 && s.SoftTimer():
		t = ui.Danger.Tmux(ui.G.Timer + "+" + units.Clock(-left) + " over")
	}
	return t + " · " + ui.BorderHearts(s.Cycles, s.MaxCycles) + ui.Muted.Tmux(" cycles") + " · " +
		ui.TmuxEscape(ducks(s)) + m.borderWarn(left)
}

// borderWarnFor is how long the border says the open door's timer passed
// 5:00 or 1:00 left (timer.warn), as the dmesg strip does: the player
// watching vim does not see the strip.
const borderWarnFor = 10 * time.Second

// warnAt is the time left at each bootui.TimerMark.
var warnAt = [...]time.Duration{1: 5 * time.Minute, 2: time.Minute}

// borderWarn is BorderClock's " · 05:00 left" for borderWarnFor after a
// mark, in Warn, or Danger at 1:00.
func (m *Model) borderWarn(left time.Duration) string {
	mark := bootui.TimerMark(m.state, m.d.Now())
	if mark == 0 || left <= warnAt[mark]-borderWarnFor {
		return ""
	}
	tone := ui.Warn
	if mark == 2 {
		tone = ui.Danger
	}
	return " · " + tone.Tmux(units.Countdown(warnAt[mark])+" left")
}

// Best is the open door's best full-test submit, passed of total, for the
// HUD's HP bar: a door resumed with a fresh HUD keeps it (hud.Options.Best).
// An upsolve, or no door, has none.
func (m *Model) Best() (passed, total int) {
	if c := m.attempt(); c != nil {
		return c.BestPass, c.Total
	}
	return 0, 0
}
