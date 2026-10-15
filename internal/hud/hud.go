// Package hud is the narrow pane beside the editor: elapsed time, the build,
// one row per test, and details for the selected test. Every save rejudges
// the samples; :Submit in the editor judges the full tests (the HP bar), and
// :Debug opens lldb on a test (command.go).
package hud

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/judge"
	"katharsis/internal/problem"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// SavedMsg tells the HUD the source was saved; the file watcher sends it.
type SavedMsg struct{}

// WatchErrMsg reports that the file watcher died; saves no longer rejudge.
type WatchErrMsg struct{ Err error }

// SubmittedMsg is emitted when a :Submit pass finishes, for a program that
// embeds the HUD and feeds the result to the game engine.
// Err is set when the submit ended without a verdict (a judge error, a
// problem that failed to load); Summary is then empty.
type SubmittedMsg struct {
	Summary judge.Summary
	Err     error
}

type Options struct {
	Dir     string
	Profile judge.Profile // judges the samples on every save
	Limits  judge.Limits
	// EditorAlive reports whether the editor pane is still open; the HUD
	// exits when it closes. Nil means there is no editor pane to watch.
	EditorAlive func() bool

	// SubmitProfile judges :Submit; when it has no Name, this machine's
	// (judge.Profiles), which is g++'s where g++ works.
	SubmitProfile judge.Profile
	// FullTests are what :Submit judges. Nil means the problem's own tests,
	// which in practice mode are the full set.
	FullTests []problem.Test
	// Debug opens a debugger on a built binary and returns how to close
	// it; nil turns :Debug off.
	Debug func(DebugTarget) (close func(), err error)
	// Border shows a line of status on the editor pane's border, so the
	// timer stays in view while focus is in the editor; nil for none. text
	// is a tmux format (session.Border.Set): #[...] styles it, and the HUD
	// has escaped every # it did not write.
	Border func(text string)
	// BorderClock is the clock for the border as a tmux format, so a host
	// can tone it (ui.BorderTimer); nil escapes the Clock text.
	BorderClock func() string
	// Best is the best full-test submit so far, passed of total, for a host
	// that keeps it across HUDs (a boot resumes a door with a fresh HUD);
	// the HP bar shows it, and how it compares with the latest submit. Nil
	// is none.
	Best func() (passed, total int)
	// CloseEditor asks the editor to quit without losing work
	// (session.CloseEditor). With it, quitting the HUD waits for the
	// editor to close first; nil quits at once.
	CloseEditor func() error
	// SubmitCopy, when set, is where :Submit copies the source to judge the
	// copy: what was submitted stays on disk, unchanged by later edits,
	// for a host that quit mid-judge to judge it again (Resubmit).
	SubmitCopy string
	// Conceal hides the full tests while it returns true: a submit reply
	// names the failing test but not its expected and actual output, and
	// :Debug takes samples only. In a boot the full tests are the judge's
	// hidden data; nil reveals everything, as in practice.
	Conceal func() bool
	// Clock replaces the elapsed-time clock in the header and border, e.g.
	// with a boot's countdown, shown after ⏱; nil counts up from when the
	// HUD started, "up 12:34".
	Clock func() string
	// Embedded is for a host that draws its own chrome around the HUD, as
	// a boot does with its HUD bar and door keys. The view then drops what
	// would repeat or contradict the host: the header (problem name and
	// clock), the key help (the host owns the keys, q included) and the
	// practice hint to add tests. It is exactly as tall as the last
	// WindowSizeMsg, so the host's footer keeps its row.
	Embedded bool
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

	pass    <-chan judge.Event // the sample pass's events; see eventMsg
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

// The HUD's own messages say which HUD and which pass they belong to. A
// host that opens a HUD per door forwards every message to the current
// one, so a late tick or judge event of the last door's HUD reaches the
// next door's; applied there, it would fill rows with another problem's
// results (or index past them) and double the tick.
type tickMsg struct {
	hud *Model
	at  time.Time
}

// closeTickMsg polls the editor quickly while it is closing.
type closeTickMsg struct{}

// eventMsg is one event of a judge pass, plus the channel to keep reading.
// The channel names the pass: one that is neither the current sample pass
// nor the current submit is stale.
type eventMsg struct {
	ch <-chan judge.Event
	ev judge.Event
}

func (m *Model) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{m, t} })
}

func closeTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return closeTickMsg{} })
}

func (m *Model) Init() tea.Cmd { return tea.Batch(m.tick(), m.rejudge(), m.updateBorder()) }

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
	m.pass, m.build, m.passErr = nil, nil, nil
	m.prob, m.loadErr = problem.Load(m.opt.Dir)
	if m.loadErr != nil {
		m.running, m.rows = false, nil
		return nil
	}
	m.running = true
	m.rows = make([]row, len(m.prob.Tests))
	m.sel = min(m.sel, max(len(m.rows)-1, 0))

	prob, prof, lim := m.prob, m.opt.Profile, m.limits()
	m.pass, m.cancel = start(func(ctx context.Context, emit func(judge.Event)) {
		judge.Judge(ctx, prob, prof, lim, emit)
	})
	return next(m.pass)
}

// start runs a judge pass in the background and streams its events into
// the returned channel, which closes when the pass ends.
func start(pass func(context.Context, func(judge.Event))) (<-chan judge.Event, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan judge.Event, 8)
	go func() {
		defer close(ch)
		pass(ctx, func(ev judge.Event) {
			select {
			case ch <- ev:
			case <-ctx.Done():
			}
		})
	}()
	return ch, cancel
}

func next(ch <-chan judge.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg{ch: ch, ev: ev}
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
		if msg.hud != m {
			return m, nil
		}
		m.now = msg.at
		if m.opt.EditorAlive != nil && !m.opt.EditorAlive() {
			return m, m.quit()
		}
		return m, tea.Batch(m.tick(), m.updateBorder())
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
	case eventMsg:
		switch msg.ch {
		case m.pass:
			m.apply(msg.ev)
			return m, tea.Batch(next(msg.ch), m.updateBorder(), m.samplesDone(msg.ev))
		case m.sub.ch:
			return m, tea.Batch(m.applySubmit(msg.ev), next(msg.ch), m.updateBorder())
		}
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

// quit stops the sample pass and leaves a submit in flight running. The
// HUD quits when the editor closes, and an embedded one lives on in its
// host until the door ends (Cleanup): cancelled here, a failing submit
// was dodged by closing vim mid-judge (qa/chaos r05). Its verdict comes
// as usual, and a HUD that replaces this one takes it over (Adopt).
func (m *Model) quit() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	m.running = false
	return tea.Quit
}

// stop cancels both passes. A cancelled pass sends nothing more, so a
// submit in flight is answered and marked stopped here.
func (m *Model) stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.running = false
	if m.sub.running {
		m.sub.cancel()
		m.sub.running = false
		m.sub.req.Answer(fail("judge stopped"))
	}
}

// Adopt takes over prev's submit, running or done, with its HP bar: prev
// is the HUD this one replaces on the same door (the editor reopened).
// prev's Cleanup then leaves it running.
func (m *Model) Adopt(prev *Model) {
	m.sub, prev.sub = prev.sub, submission{}
}

// EditorLeftOpen reports whether the player quit the HUD while it waited
// for the editor (a second q), keeping the editor open on purpose. A HUD
// that exited any other way with the editor still open (a signal) should
// have it closed.
func (m *Model) EditorLeftOpen() bool { return m.leftOpen }

// Cleanup stops the judge passes and closes the debugger, removing its
// files. Call it once the HUD is done with: its program exited, or its
// door closed.
func (m *Model) Cleanup() {
	m.stop()
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

// updateBorder hands Options.Border the status line, as a tmux format, when
// it changed.
func (m *Model) updateBorder() tea.Cmd {
	if m.opt.Border == nil {
		return nil
	}
	text := m.borderText()
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

// statusText is one line for `ctl status`, plain text:
// "⏱ 12:34 · samples 2/3 · HP 37/52 WA", or "up 12:34 · …" in practice.
func (m *Model) statusText() string {
	return m.status(m.clockText(), func(_ ui.Token, s string) string { return s })
}

// borderText is the status line as a tmux format for the editor pane's
// border: the host's toned clock (Options.BorderClock) or the plain one,
// the verdicts in their tokens, and every other part escaped, so a # in a
// clock or a name is drawn, not read as a directive.
func (m *Model) borderText() string {
	clock := ui.TmuxEscape(m.clockText())
	if m.opt.BorderClock != nil {
		clock = m.opt.BorderClock()
	}
	return " " + m.status(clock, ui.Token.Tmux) + " "
}

// status joins the clock with the samples and the last submit, each part
// painted with paint.
func (m *Model) status(clock string, paint func(ui.Token, string) string) string {
	parts := []string{clock}
	switch {
	case m.loadErr != nil:
		parts = append(parts, paint(ui.FG, "problem not loaded"))
	case m.build != nil && !m.build.OK:
		parts = append(parts, paint(ui.FG, "samples ")+paint(ui.Danger, "CE"))
	case len(m.rows) > 0:
		parts = append(parts, paint(ui.FG, fmt.Sprintf("samples %d/%d", m.passed(), len(m.rows))))
	}
	if m.sub.started {
		s := judge.Summarize(m.sub.results)
		hp := paint(ui.FG, fmt.Sprintf("HP %d/%d", s.Passed, s.Total))
		switch {
		case m.sub.running:
			hp += paint(ui.FG, " judging")
		case m.sub.err != nil:
			hp = paint(ui.Danger, "submit failed")
		case m.sub.sum != nil:
			v := string(m.sub.sum.Verdict)
			hp += " " + paint(ui.Verdict(v), v)
		}
		parts = append(parts, hp)
	}
	return strings.Join(parts, " · ")
}

func (m *Model) View() tea.View {
	w := m.width
	if w <= 0 {
		w = 40
	}
	var b strings.Builder
	line := func(s string) { b.WriteString(ui.Clip(s, w) + "\n") }
	rule := func() { line(ui.Rule(w)) }
	used := func() int { return strings.Count(b.String(), "\n") }

	// help is the row the key help takes; an embedded HUD has none, nor a
	// header.
	help := 0
	if !m.opt.Embedded {
		name := "?"
		if m.prob != nil {
			name = m.prob.Name
		}
		line(ui.Spread(ui.Strong.Render(name), m.headerClock(), w))
		help = 1
	}
	// Judgement first, the HP bar, then the samples (C8).
	for _, l := range m.submitLines(w) {
		line(l)
	}
	line(m.statusLine())
	if m.closing {
		line(ui.Warn.Render("closing vim: answer it there · q again leaves it open"))
	}
	rule()

	switch {
	case m.loadErr != nil:
		for _, l := range wrap(m.loadErr.Error(), w) {
			line(ui.Danger.Render(l))
		}
	case m.build != nil && !m.build.OK:
		line(ui.Danger.Render("CE") + ui.Muted.Render("  compile error"))
		rule()
		for _, l := range firstLines(m.compilerLog(m.build.Log), max(m.height-used()-help, 4)) {
			line(l)
		}
	default:
		// Rows get the lines the header, the detail and the key help leave,
		// and scroll to keep the selected one in view: a full test set is
		// taller than the pane.
		detail := m.detail(w)
		room := len(m.rows)
		if m.height > 0 {
			room = max(m.height-used()-len(detail)-1-help, 2)
		}
		lo, hi := 0, len(m.rows)
		if len(m.rows) > room {
			lo, hi = scroll(len(m.rows), m.sel, room-1)
		}
		for i := lo; i < hi; i++ {
			line(m.rowLine(i, m.rows[i]))
		}
		if lo > 0 || hi < len(m.rows) {
			line(ui.Muted.Render(fmt.Sprintf("  rows %d–%d of %d · j/k", lo+1, hi, len(m.rows))))
		}
		switch {
		case len(m.rows) > 0:
		case m.opt.Embedded:
			line(ui.Muted.Render("no samples · :Submit judges the full tests"))
		default:
			line(ui.Muted.Render("no tests: add tests/1.in and tests/1.out"))
		}
		rule()
		for _, l := range detail {
			line(l)
		}
	}

	// Cut or pad to exactly the pane's height, the key help pinned to the
	// bottom row. The row and log budgets above keep a few lines even in a
	// pane too short for them, and a view taller than its pane loses lines
	// it needs: standalone the terminal scrolls the header away, embedded
	// the host's lines above it go.
	ls := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if m.height > 0 {
		room := max(m.height-help, 0)
		ls = ls[:min(len(ls), room)]
		for len(ls) < room {
			ls = append(ls, "")
		}
	}
	if help > 0 {
		ls = append(ls, ui.Footer(w, ":w", "rejudge", ":Submit", "judge", ":Debug n", "debug",
			"r", "rerun", "j/k", "move", "q", "quit"))
	}
	v := tea.NewView(strings.Join(ls, "\n"))
	v.AltScreen = true
	return v
}

// passed counts the samples judged AC so far.
func (m *Model) passed() int {
	n := 0
	for _, r := range m.rows {
		if r.done && r.res.Verdict == judge.AC {
			n++
		}
	}
	return n
}

// statusLine is the samples pass: "samples 2/3 · asan+ubsan · built 1.2s"
// (C8), the count first and its state right after it, so a narrow pane
// cuts the profile and the build time first.
func (m *Model) statusLine() string {
	prof := m.opt.Profile.Name
	switch {
	case m.watchErr != nil:
		return ui.Danger.Render("watcher: " + m.watchErr.Error() + " (r rejudges)")
	case m.passErr != nil:
		return ui.Danger.Render("error: " + m.passErr.Error())
	case m.build == nil && m.running:
		return ui.Muted.Render(prof + ui.G.Sep + "compiling…")
	case m.build == nil:
		return ui.Muted.Render(prof)
	case !m.build.OK:
		return ui.Danger.Render(prof + ui.G.Sep + "compile failed")
	}
	built := fmt.Sprintf("built %.1fs", m.build.Duration.Seconds())
	if m.build.Cached {
		built = "cached build"
	}
	pass := m.passed()
	s := fmt.Sprintf("samples %d/%d", pass, len(m.rows))
	if !m.running && len(m.rows) > 0 && pass == len(m.rows) {
		// Green paints the glyph, the words are Strong (R2, R3).
		s += " " + ui.OK.Render(ui.G.Pass) + " " + ui.Strong.Render("ALL PASS")
	} else if m.running {
		s += ui.Muted.Render(" running…")
	}
	return s + ui.Muted.Render(ui.G.Sep+prof+ui.G.Sep+built)
}

func (m *Model) rowLine(i int, r row) string {
	cursor := "  "
	if i == m.sel {
		cursor = ui.Focus.Render(ui.G.Cursor) + " "
	}
	name := m.prob.Tests[i].Name
	if !r.done {
		return cursor + fmt.Sprintf("%-3s ", name) + ui.Muted.Render("·")
	}
	v := r.res.Verdict
	s := cursor + fmt.Sprintf("%-3s ", name) + ui.Verdict(string(v)).Render(fmt.Sprintf("%-4s", v))
	s += fmt.Sprintf("%6dms %6s", r.res.CPU.Milliseconds(), units.Mem(r.res.MaxRSS))
	if r.res.Reason != "" && v != judge.AC {
		s += "  " + ui.Muted.Render(r.res.Reason)
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
		return []string{ui.Muted.Render("#" + t.Name + " pending")}
	}
	res := r.res
	head := fmt.Sprintf("#%s %s", t.Name, res.Verdict)
	if res.Reason != "" {
		head += "  " + res.Reason
	}
	out := []string{ui.Verdict(string(res.Verdict)).Render(head)}
	switch {
	case res.Diff != nil:
		out = append(out,
			ui.Muted.Render("want ")+res.Diff.Want,
			ui.Muted.Render("got  ")+res.Diff.Got)
	case res.Verdict == judge.RE:
		out = append(out, firstLines(res.Stderr, max(m.height-len(m.rows)-8, 3))...)
	case res.Verdict == judge.AC:
		out = append(out, ui.Muted.Render(fmt.Sprintf("cpu %s · wall %s · peak %s",
			res.CPU.Round(time.Millisecond), res.Wall.Round(time.Millisecond), units.Mem(res.MaxRSS))))
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

// compilerLog is the compiler's log with the source's directory cut from
// every path in it (see relLog).
func (m *Model) compilerLog(log string) string {
	if m.prob == nil {
		return log
	}
	return relLog(log, m.prob.Source)
}

// relLog cuts source's directory from the paths in a compiler log. The
// compiler names the file by its absolute path, which in a boot is a deep
// temp directory; a pane 40 columns wide would show only that prefix,
// truncated, and none of the error.
func relLog(log, source string) string {
	if source == "" {
		return log
	}
	return strings.ReplaceAll(log, filepath.Dir(source)+string(filepath.Separator), "")
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

// clockText is the clock as plain text: a host's countdown under ⏱, or in
// practice the time since the HUD started, "up 12:34" (an elapsed time has
// no ⏱, §2.4).
func (m *Model) clockText() string {
	if m.opt.Clock != nil {
		return ui.G.Timer + m.opt.Clock()
	}
	return "up " + units.Clock(m.now.Sub(m.started))
}

// headerClock is clockText for the standalone header, its "up" label
// Muted.
func (m *Model) headerClock() string {
	if m.opt.Clock != nil {
		return m.clockText()
	}
	return ui.Muted.Render("up ") + units.Clock(m.now.Sub(m.started))
}
