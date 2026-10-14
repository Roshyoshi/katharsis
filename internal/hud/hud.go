// Package hud is the narrow pane beside the editor: elapsed time, the build,
// one row per test, and details for the selected test. Every save rejudges
// the samples; :Submit in the editor judges the full tests (the HP bar), and
// :Debug opens lldb on a test (command.go).
package hud

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/judge"
	"katharsis/internal/problem"
)

// SavedMsg tells the HUD the source was saved; the file watcher sends it.
type SavedMsg struct{}

// WatchErrMsg reports that the file watcher died; saves no longer rejudge.
type WatchErrMsg struct{ Err error }

// SubmittedMsg is emitted when a :Submit pass finishes, for a program that
// embeds the HUD and feeds the result to the game engine.
type SubmittedMsg struct{ Summary judge.Summary }

type Options struct {
	Dir     string
	Profile judge.Profile // judges the samples on every save
	Limits  judge.Limits
	// EditorAlive reports whether the editor pane is still open; the HUD
	// exits when it closes. Nil means there is no editor pane to watch.
	EditorAlive func() bool

	// SubmitProfile judges :Submit; judge.Full when it has no Name.
	SubmitProfile judge.Profile
	// FullTests are what :Submit judges. Nil means the problem's own tests,
	// which in practice mode are the full set.
	FullTests []problem.Test
	// Debug opens a debugger on a built binary and returns how to close
	// it; nil turns :Debug off.
	Debug func(DebugTarget) (close func(), err error)
	// Border shows a line of status on the editor pane's border, so the
	// timer stays in view while focus is in the editor; nil for none.
	Border func(text string)
	// CloseEditor asks the editor to quit without losing work
	// (session.CloseEditor). With it, quitting the HUD waits for the
	// editor to close first; nil quits at once.
	CloseEditor func() error
	// Conceal hides the full tests while it returns true: a submit reply
	// names the failing test but not its expected and actual output, and
	// :Debug takes samples only. In a boot the full tests are the judge's
	// hidden data; nil reveals everything, as in practice.
	Conceal func() bool
	// Clock replaces the elapsed-time clock in the header and border, e.g.
	// with a boot's countdown; nil counts up from when the HUD started.
	Clock func() string
}

type row struct {
	done bool
	res  judge.Result
}

type Model struct {
	opt     Options
	prob    *problem.Problem
	loadErr error
	started time.Time
	now     time.Time

	gen     int // judge pass; events from older passes are dropped
	cancel  context.CancelFunc
	running bool
	build   *judge.Build
	passErr error
	rows    []row
	sel     int

	sub      submission
	dbg      debugger
	closing  bool   // asked the editor to quit; waiting for it
	leftOpen bool   // quit anyway, leaving the editor open on purpose
	border   string // last text given to Options.Border

	watchErr error

	width, height int
}

func New(opt Options) *Model {
	now := time.Now()
	return &Model{opt: opt, started: now, now: now}
}

type tickMsg time.Time

// closeTickMsg polls the editor quickly while it is closing.
type closeTickMsg struct{}

// judgeMsg carries one event of pass gen, plus the channel to keep reading.
type judgeMsg struct {
	gen int
	ev  judge.Event
	ch  <-chan judge.Event
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func closeTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return closeTickMsg{} })
}

func (m *Model) Init() tea.Cmd { return tea.Batch(tick(), m.rejudge(), m.updateBorder()) }

// limits fills in the problem's own time limit when the options leave it
// unset.
func (m *Model) limits() judge.Limits {
	lim := m.opt.Limits
	if lim.Time == 0 && m.prob != nil {
		lim.Time = m.prob.TimeLim
	}
	return lim
}

// rejudge cancels any pass in flight, reloads the problem (tests may have
// been added since), and starts a new pass that streams events back.
func (m *Model) rejudge() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	m.gen++
	m.build, m.passErr = nil, nil
	m.prob, m.loadErr = problem.Load(m.opt.Dir)
	if m.loadErr != nil {
		m.running, m.rows = false, nil
		return nil
	}
	m.running = true
	m.rows = make([]row, len(m.prob.Tests))
	m.sel = min(m.sel, max(len(m.rows)-1, 0))

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch := make(chan judge.Event, 8)
	prob, prof, lim := m.prob, m.opt.Profile, m.limits()
	go func() {
		defer close(ch)
		judge.Judge(ctx, prob, prof, lim, func(ev judge.Event) {
			select {
			case ch <- ev:
			case <-ctx.Done():
			}
		})
	}()
	return next(m.gen, ch)
}

func next(gen int, ch <-chan judge.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return judgeMsg{gen: gen, ev: ev, ch: ch}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd, ok := m.updateCommand(msg); ok {
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		m.now = time.Time(msg)
		if m.opt.EditorAlive != nil && !m.opt.EditorAlive() {
			return m, m.quit()
		}
		return m, tea.Batch(tick(), m.updateBorder())
	case closeTickMsg:
		if !m.closing {
			return m, nil
		}
		if !m.opt.EditorAlive() {
			return m, m.quit()
		}
		return m, closeTick()
	case WatchErrMsg:
		m.watchErr = msg.Err
	case SavedMsg:
		return m, m.rejudge()
	case judgeMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.apply(msg.ev)
		return m, tea.Batch(next(msg.gen, msg.ch), m.updateBorder())
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, m.requestQuit()
		case "r":
			return m, m.rejudge()
		case "j", "down":
			m.sel = min(m.sel+1, max(len(m.rows)-1, 0))
		case "k", "up":
			m.sel = max(m.sel-1, 0)
		}
	}
	return m, nil
}

// requestQuit asks the editor to close first, so quitting the HUD never
// throws away unsaved work: vim's own prompt decides. The HUD exits once
// the editor pane is gone; a second q exits now and leaves the editor open.
func (m *Model) requestQuit() tea.Cmd {
	ed := m.opt.EditorAlive
	if m.closing {
		m.leftOpen = true
		return m.quit()
	}
	if m.opt.CloseEditor == nil || ed == nil || !ed() {
		return m.quit()
	}
	m.closing = true
	closeEditor := m.opt.CloseEditor
	return tea.Batch(func() tea.Msg {
		if err := closeEditor(); err != nil {
			return tea.QuitMsg{} // cannot ask it; leave it open
		}
		return nil
	}, closeTick())
}

func (m *Model) quit() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	if m.sub.cancel != nil {
		m.sub.cancel()
	}
	return tea.Quit
}

// EditorLeftOpen reports whether the player quit the HUD while it waited
// for the editor (a second q), keeping the editor open on purpose. A HUD
// that exited any other way with the editor still open (a signal) should
// have it closed.
func (m *Model) EditorLeftOpen() bool { return m.leftOpen }

// Cleanup closes the debugger and removes its files. Call it once the
// program has exited.
func (m *Model) Cleanup() {
	if m.sub.cancel != nil {
		m.sub.cancel()
	}
	m.closeDebugger()
}

func (m *Model) apply(ev judge.Event) {
	switch ev := ev.(type) {
	case judge.BuildDone:
		b := ev.Build
		m.build = &b
		if !b.OK {
			m.running = false
		}
	case judge.TestDone:
		m.rows[ev.Index] = row{done: true, res: ev.Result}
		// Follow the first failure so its details show without a keypress.
		if ev.Result.Verdict != judge.AC && m.firstFailure() == ev.Index {
			m.sel = ev.Index
		}
	case judge.Finished:
		m.running, m.passErr = false, ev.Err
	}
}

func (m *Model) firstFailure() int {
	for i, r := range m.rows {
		if r.done && r.res.Verdict != judge.AC {
			return i
		}
	}
	return -1
}

// updateBorder hands Options.Border the status line when it changed.
func (m *Model) updateBorder() tea.Cmd {
	if m.opt.Border == nil {
		return nil
	}
	text := " " + m.statusText() + " "
	if text == m.border {
		return nil
	}
	m.border = text
	show := m.opt.Border
	return func() tea.Msg {
		show(text)
		return nil
	}
}

// statusText is one line for the pane border and `ctl status`:
// "⏱ 12:34 · samples 2/3 · HP 37/52 WA".
func (m *Model) statusText() string {
	parts := []string{"⏱ " + m.clockText()}
	switch {
	case m.loadErr != nil:
		parts = append(parts, "problem not loaded")
	case m.build != nil && !m.build.OK:
		parts = append(parts, "samples CE")
	case len(m.rows) > 0:
		pass := 0
		for _, r := range m.rows {
			if r.done && r.res.Verdict == judge.AC {
				pass++
			}
		}
		parts = append(parts, fmt.Sprintf("samples %d/%d", pass, len(m.rows)))
	}
	if m.sub.started {
		s := judge.Summarize(m.sub.results)
		hp := fmt.Sprintf("HP %d/%d", s.Passed, s.Total)
		switch {
		case m.sub.running:
			hp += " judging"
		case m.sub.err != nil:
			hp = "submit failed"
		case m.sub.sum != nil:
			hp += " " + string(m.sub.sum.Verdict)
		}
		parts = append(parts, hp)
	}
	return strings.Join(parts, " · ")
}

var (
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	faint  = lipgloss.NewStyle().Faint(true)
	bold   = lipgloss.NewStyle().Bold(true)
)

func verdictStyle(v judge.Verdict) lipgloss.Style {
	switch v {
	case judge.AC:
		return green
	case judge.WA, judge.RE, judge.CE:
		return red
	}
	return yellow
}

func (m *Model) View() tea.View {
	w := m.width
	if w <= 0 {
		w = 40
	}
	var b strings.Builder
	line := func(s string) { b.WriteString(ansi.Truncate(s, w, "…") + "\n") }
	rule := func() { line(faint.Render(strings.Repeat("─", w))) }

	name := "?"
	if m.prob != nil {
		name = m.prob.Name
	}
	clock := m.clockText()
	line(bold.Render(name) + strings.Repeat(" ", max(w-len(name)-len(clock)-2, 1)) + "⏱ " + clock)
	line(m.statusLine())
	if m.closing {
		line(yellow.Render("closing vim: answer it there · q again leaves it open"))
	}
	for _, l := range m.submitLines(w) {
		line(l)
	}
	rule()

	if m.loadErr != nil {
		for _, l := range wrap(m.loadErr.Error(), w) {
			line(red.Render(l))
		}
		v := tea.NewView(b.String())
		v.AltScreen = true
		return v
	}

	if m.build != nil && !m.build.OK {
		line(red.Render("CE") + faint.Render("  compile error"))
		rule()
		for _, l := range firstLines(m.build.Log, max(m.height-6, 4)) {
			line(l)
		}
	} else {
		// Rows get the lines the header, the detail and the key help leave,
		// and scroll to keep the selected one in view: a full test set is
		// taller than the pane.
		detail := m.detail(w)
		room := len(m.rows)
		if m.height > 0 {
			room = max(m.height-strings.Count(b.String(), "\n")-len(detail)-2, 2)
		}
		lo, hi := 0, len(m.rows)
		if len(m.rows) > room {
			lo, hi = scroll(len(m.rows), m.sel, room-1)
		}
		for i := lo; i < hi; i++ {
			line(m.rowLine(i, m.rows[i]))
		}
		if lo > 0 || hi < len(m.rows) {
			line(faint.Render(fmt.Sprintf("  rows %d–%d of %d · j/k", lo+1, hi, len(m.rows))))
		}
		if len(m.rows) == 0 {
			line(faint.Render("no tests: add tests/1.in and tests/1.out"))
		}
		rule()
		for _, l := range detail {
			line(l)
		}
	}

	// Pin the key help to the bottom row.
	used := strings.Count(b.String(), "\n")
	for i := used; i < m.height-1; i++ {
		b.WriteString("\n")
	}
	b.WriteString(ansi.Truncate(faint.Render(":w rejudges · :Submit · :Debug n · r rerun · j/k · q quit"), w, "…"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *Model) statusLine() string {
	prof := m.opt.Profile.Name
	switch {
	case m.watchErr != nil:
		return red.Render("watcher: " + m.watchErr.Error() + " (r rejudges)")
	case m.passErr != nil:
		return red.Render("error: " + m.passErr.Error())
	case m.build == nil && m.running:
		return faint.Render(prof + " · compiling…")
	case m.build == nil:
		return faint.Render(prof)
	case !m.build.OK:
		return red.Render(prof + " · compile failed")
	}
	built := fmt.Sprintf("built %.1fs", m.build.Duration.Seconds())
	if m.build.Cached {
		built = "cached build"
	}
	pass, done := 0, 0
	for _, r := range m.rows {
		if r.done {
			done++
			if r.res.Verdict == judge.AC {
				pass++
			}
		}
	}
	s := faint.Render(prof+" · "+built+" · ") + fmt.Sprintf("%d/%d", pass, len(m.rows))
	if !m.running && len(m.rows) > 0 && pass == len(m.rows) {
		s += "  " + green.Bold(true).Render("ALL PASS")
	} else if m.running {
		s += faint.Render(" running…")
	}
	return s
}

func (m *Model) rowLine(i int, r row) string {
	cursor := "  "
	if i == m.sel {
		cursor = "▸ "
	}
	name := m.prob.Tests[i].Name
	if !r.done {
		return cursor + fmt.Sprintf("%-3s ", name) + faint.Render("·")
	}
	v := r.res.Verdict
	s := cursor + fmt.Sprintf("%-3s ", name) + verdictStyle(v).Render(fmt.Sprintf("%-4s", v))
	s += fmt.Sprintf("%6dms %6s", r.res.CPU.Milliseconds(), judge.Bytes(r.res.MaxRSS))
	if r.res.Reason != "" && v != judge.AC {
		s += "  " + faint.Render(r.res.Reason)
	}
	return s
}

// detail explains the selected test: the diff for WA, the crash report for
// RE, the limit for TLE/MLE/OLE.
func (m *Model) detail(w int) []string {
	if len(m.rows) == 0 {
		return nil
	}
	r, t := m.rows[m.sel], m.prob.Tests[m.sel]
	if !r.done {
		return []string{faint.Render("#" + t.Name + " pending")}
	}
	res := r.res
	head := fmt.Sprintf("#%s %s", t.Name, res.Verdict)
	if res.Reason != "" {
		head += "  " + res.Reason
	}
	out := []string{verdictStyle(res.Verdict).Render(head)}
	switch {
	case res.Diff != nil:
		out = append(out,
			faint.Render("want ")+res.Diff.Want,
			faint.Render("got  ")+res.Diff.Got)
	case res.Verdict == judge.RE:
		out = append(out, firstLines(res.Stderr, max(m.height-len(m.rows)-8, 3))...)
	case res.Verdict == judge.AC:
		out = append(out, faint.Render(fmt.Sprintf("cpu %s · wall %s · peak %s",
			res.CPU.Round(time.Millisecond), res.Wall.Round(time.Millisecond), judge.Bytes(res.MaxRSS))))
	}
	return out
}

// scroll picks rows [lo, hi) of n, room of them, with sel inside and as
// close to the middle as the ends allow.
func scroll(n, sel, room int) (lo, hi int) {
	room = max(min(room, n), 1)
	lo = min(max(sel-room/2, 0), n-room)
	return lo, lo + room
}

func fmtClock(d time.Duration) string {
	d = d.Round(time.Second)
	h, mnt, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, mnt, s)
	}
	return fmt.Sprintf("%02d:%02d", mnt, s)
}

func firstLines(s string, n int) []string {
	ls := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(ls) > n {
		ls = append(ls[:n-1], fmt.Sprintf("… %d more lines", len(ls)-n+1))
	}
	return ls
}

func wrap(s string, w int) []string {
	return strings.Split(ansi.Wrap(s, max(w, 10), ""), "\n")
}

func (m *Model) clockText() string {
	if m.opt.Clock != nil {
		return m.opt.Clock()
	}
	return fmtClock(m.now.Sub(m.started))
}
