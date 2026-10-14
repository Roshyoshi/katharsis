package boot

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"deadcases/internal/game"
	"deadcases/internal/screens"
)

// revealTime is how long the panic takes to print. Long enough to read as
// a console scrolling past, short enough that nobody waits for it.
const revealTime = 600 * time.Millisecond

// revealMsg advances the reveal of the crash it was started for.
type revealMsg struct{ gen int }

// Crash is the kernel panic. It reads as a real dmesg tail: the last log
// lines, the cause, taint flags for the door's modifiers, the boot's doors
// as a call trace, then a quote matched to the cause and the core dump.
// u emits screens.Upsolve, r screens.StartBoot, l screens.OpenLog and
// q screens.Quit.
type Crash struct {
	env   Env
	shown int // elements revealed so far; < 0 shows everything
	gen   int // bumps on Init so ticks from an older reveal are dropped
}

// NewCrash builds the panic fully printed. Call Init to play the reveal.
func NewCrash(env Env) *Crash { return &Crash{env: env, shown: -1} }

// Refresh swaps in new state and clock.
func (c *Crash) Refresh(env Env) { c.env = env }

// Init starts the reveal: the panic prints line by line over revealTime.
// Any key during the reveal skips to the end.
func (c *Crash) Init() tea.Cmd {
	c.gen++
	c.shown = 0
	return c.tick()
}

// Revealing reports whether the reveal is still printing.
func (c *Crash) Revealing() bool { return c.shown >= 0 && c.shown < c.total() }

func (c *Crash) total() int { return len(c.elements(1<<10, 1<<10)) }

func (c *Crash) tick() tea.Cmd {
	gen := c.gen
	d := revealTime / time.Duration(max(c.total(), 1))
	return tea.Tick(d, func(time.Time) tea.Msg { return revealMsg{gen: gen} })
}

func (c *Crash) crash() game.Crash {
	if c.env.State != nil && c.env.State.Crash != nil {
		return *c.env.State.Crash
	}
	return game.Crash{}
}

func (c *Crash) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case revealMsg:
		if msg.gen != c.gen || !c.Revealing() {
			return c, nil
		}
		c.shown++
		if c.Revealing() {
			return c, c.tick()
		}
		c.shown = -1
	case tea.KeyPressMsg:
		if c.Revealing() {
			c.shown = -1
			return c, nil
		}
		st := c.env.State
		if st == nil {
			return c, nil
		}
		switch msg.String() {
		case "u":
			return c, screens.Emit(screens.Upsolve{Problem: string(c.crash().Door.Problem)})
		case "r":
			return c, screens.Emit(screens.StartBoot{Prestige: st.Prestige, Preset: st.Rules.Name})
		case "l":
			return c, screens.Emit(screens.OpenLog{})
		case "q":
			return c, screens.Emit(screens.Quit{})
		}
	}
	return c, nil
}

func (c *Crash) View(width, height int) string {
	if c.env.State == nil {
		return noBoot(width, height)
	}
	els := c.elements(width, height)
	n := len(els)
	if c.shown >= 0 {
		n = min(c.shown, n)
	}
	var body []string
	for _, e := range els[:n] {
		body = append(body, e...)
	}
	return layout(width, height, body, "")
}

// dline is one dmesg line before it is wrapped to the terminal.
type dline struct {
	at    time.Duration
	text  string
	style lipgloss.Style
}

func stamp(d time.Duration) string {
	d = max(d, 0)
	return fmt.Sprintf("[%5d.%03d]", int64(d/time.Second), int64(d%time.Second/time.Millisecond))
}

// render wraps a dmesg line under its timestamp, the way a console with a
// hanging indent would.
func (l dline) render(width int) []string {
	ts := stamp(l.at)
	sw := widthOf(ts) + 1
	ls := []string{l.text}
	if sw+widthOf(l.text) > width {
		ls = wrapN(l.text, max(width-sw, 10), 0)
	}
	out := make([]string, len(ls))
	for i, s := range ls {
		pre := strings.Repeat(" ", sw)
		if i == 0 {
			pre = sFaint.Render(ts) + " "
		}
		out[i] = pre + l.style.Render(s)
	}
	return out
}

// elements is the screen as reveal steps: each dmesg line, then the quote,
// then the core dump. It fits height by dropping old log lines first, then
// eliding the middle of the call trace.
func (c *Crash) elements(width, height int) [][]string {
	st := c.env.State
	cr := c.crash()
	up := cr.Uptime
	if up == 0 && !cr.At.IsZero() && !st.Started.IsZero() {
		up = cr.At.Sub(st.Started)
	}
	title := "?"
	if cr.Door.Problem != "" {
		title = c.env.problem(cr.Door.Problem).Title
	}
	verdict := cr.Verdict
	if verdict == "" {
		verdict = c.attempt(cr).LastVerdict
	}
	// One seed per crash, so the quote and the kernel frames stay put
	// across redraws and differ between crashes.
	seed := st.Seed ^ uint64(cr.At.Unix())

	quote := c.quote(cr, verdict, seed, width)
	core := c.core(cr, title, width)
	room := height - len(quote) - len(core)

	reason := fl.PanicReason(cr.Cause, verdict)
	if reason == "" {
		reason = "unknown cause"
	}
	head := []dline{}
	if pre := c.precursor(cr, verdict); pre != "" {
		head = append(head, dline{at: up, text: pre})
	}
	head = append(head,
		dline{at: up, text: fmt.Sprintf("Kernel panic - not syncing: %s in %q", reason, title), style: sRed.Bold(true)},
		dline{at: up, text: "CPU: 0 PID: 1 Comm: you " + taint(cr.Door)},
		dline{at: up, text: fmt.Sprintf("Hardware name: dead-cases %s, preset %s, seed %x",
			prestigeLabel(st.Prestige), st.Rules.Name, st.Seed)},
	)
	if mods := modules(st); mods != "" {
		head = append(head, dline{at: up, text: "Modules linked in: " + mods})
	}
	head = append(head, dline{at: up, text: "Call Trace:"})
	tail := []dline{{at: up, text: "---[ end Kernel panic ]---", style: sRed}}

	lines := func(ds []dline) int {
		n := 0
		for _, d := range ds {
			n += len(d.render(width))
		}
		return n
	}
	var kernel []string
	for _, f := range fl.CallTrace(cr.Cause, verdict, seed) {
		kernel = append(kernel, " "+f)
	}
	doors, boot := c.frames(cr, up)
	fixed := lines(head) + lines(tail)
	rest := concat(doors, []string{boot})
	tooTall := func() bool { return fixed+lines(dlines(up, concat(kernel, rest))) > room }
	// Elide the oldest doors until the trace fits, then the outer kernel
	// frames; the innermost frame, the door that crashed and the boot
	// frame always stay.
	for k := len(doors) - 1; k >= 1 && tooTall(); k-- {
		rest = concat(doors[:k], []string{fmt.Sprintf(" ... %d older doors", len(doors)-k), boot})
	}
	for k := len(kernel) - 1; k >= 1 && tooTall(); k-- {
		kernel = kernel[:k]
	}
	body := concat(head, dlines(up, concat(kernel, rest)), tail)

	var pre []dline
	free := room - lines(body)
	for i := len(st.Log) - 1; i >= 0 && len(pre) < 4; i-- {
		l := st.Log[i]
		d := dline{at: l.At.Sub(st.Started), text: l.Text, style: sFaint}
		if st.Started.IsZero() {
			d.at = 0
		}
		n := len(d.render(width))
		if n > free {
			break
		}
		free -= n
		pre = append([]dline{d}, pre...)
	}

	var out [][]string
	for _, d := range append(pre, body...) {
		out = append(out, d.render(width))
	}
	if quote != nil {
		out = append(out, quote)
	}
	return append(out, core)
}

func dlines(at time.Duration, texts []string) []dline {
	out := make([]dline, len(texts))
	for i, t := range texts {
		out[i] = dline{at: at, text: t}
	}
	return out
}

// precursor is a log line printed just before the panic when the panic
// line alone hides what happened: running out of lives gets the OOM
// killer's report with the verdict and the HP bar, and :q! names itself
// (the panic reads as init being killed).
func (c *Crash) precursor(cr game.Crash, verdict string) string {
	switch cr.Cause {
	case game.OutOfLives:
		a := c.attempt(cr)
		if verdict == "" {
			verdict = "?"
		}
		return fmt.Sprintf("Out of lives: Killed process 1 (you) verdict:%s passed:%d/%d submits:%d",
			verdict, a.BestPass, a.Total, a.Submits)
	case game.GaveUp:
		return "you[1]: :q! sent SIGKILL to init"
	}
	return ""
}

// attempt is the attempt that crashed, or a zero Attempt if the state has
// none for the crashing door.
func (c *Crash) attempt(cr game.Crash) game.Attempt {
	ss := steps(c.env.State)
	for i := len(ss) - 1; i >= 0; i-- {
		if ss[i].Attempt.Door.Problem == cr.Door.Problem {
			return ss[i].Attempt
		}
	}
	return game.Attempt{}
}

// taint maps door modifiers onto kernel taint flags, in the kernel's own
// flag order.
func taint(d game.Door) string {
	var fs []string
	if d.Regression {
		fs = append(fs, "R (regression)")
	}
	if d.Mystery {
		fs = append(fs, "M (mystery)")
	}
	if d.Blind {
		fs = append(fs, "B (blind)")
	}
	if d.Cursed {
		fs = append(fs, "C (cursed)")
	}
	if d.Boss != "" {
		fs = append(fs, "X (boss)")
	}
	if len(fs) == 0 {
		return "Not tainted"
	}
	return "Tainted: " + strings.Join(fs, " ")
}

// modules lists daemons as out-of-tree modules, (O) as the kernel marks
// them, and linked libs as plain ones.
func modules(st *game.State) string {
	var ms []string
	for _, d := range st.Daemons {
		ms = append(ms, strings.ReplaceAll(string(d), "-", "_")+"(O)")
	}
	ms = append(ms, st.Linked...)
	return strings.Join(ms, " ")
}

// frames is the call trace: every door of the boot, newest first, as
// door_N+elapsed/timer in minutes (hex, as kernel offsets are), tagged with
// its biome the way a frame is tagged with its module. Retreated doors are
// marked "?", the kernel's sign for an unreliable frame. The boot frame
// closes the trace.
func (c *Crash) frames(cr game.Crash, up time.Duration) (doors []string, boot string) {
	st := c.env.State
	ss := steps(st)
	var sum time.Duration
	mins := func(d time.Duration) int { return int(max(d, 0) / time.Minute) }
	for i := len(ss) - 1; i >= 0; i-- {
		s := ss[i]
		a := s.Attempt
		end := cr.At
		if end.IsZero() {
			end = c.env.Now
		}
		mark := " "
		if a.Outcome == game.Retreated {
			mark = " ? "
		}
		doors = append(doors, fmt.Sprintf("%sdoor_%d+0x%x/0x%x [%s]", mark, s.N,
			mins(elapsed(a, end)), mins(a.Door.Timer), biomeLabel(st, s.Biome)))
		sum += a.Door.Timer
	}
	return doors, fmt.Sprintf(" boot_%d+0x%x/0x%x", st.Boot, mins(up), max(mins(sum), mins(up)))
}

// quote is the cause-matched quote, wrapped, with its attribution set at
// the right like a signature.
func (c *Crash) quote(cr game.Crash, verdict string, seed uint64, width int) []string {
	q := fl.QuoteFor(cr.Cause, verdict, seed)
	if q.Text == "" {
		return nil
	}
	qw := max(min(width-6, 66), 16)
	ls := wrapN(q.Text, qw, 0)
	out := []string{""}
	for i, l := range ls {
		pre := "   "
		if i == 0 {
			pre = "  \""
		}
		if i == len(ls)-1 {
			l += "\""
		}
		out = append(out, pre+sItalic.Render(l))
	}
	attr := "— " + q.Author
	if q.Source != "" {
		attr += ", " + q.Source
	}
	if q.Year != 0 {
		attr += fmt.Sprintf(" (%d)", q.Year)
	}
	right := min(width-2, 76)
	last := out[len(out)-1]
	if widthOf(last)+4+widthOf(attr) <= right {
		out[len(out)-1] = spread(last, sFaint.Render(attr), right)
	} else {
		out = append(out, strings.Repeat(" ", max(right-widthOf(attr), 4))+sFaint.Render(attr))
	}
	return out
}

// core is the core dump line: what the crash left behind, and the keys.
func (c *Crash) core(cr game.Crash, title string, width int) []string {
	keys := sBold.Render("[u]") + " upsolve  " + sBold.Render("[r]") + " reboot  " + sBold.Render("[l]") + " git log"
	dump := sFaint.Render("no core dump: nothing was uncommitted")
	if cr.Lost > 0 {
		dump = "core dumped: " + sGreen.Bold(true).Render(FormatBytes(cr.Lost)) + fmt.Sprintf(" on %q", title)
	}
	if line := "  " + dump + sFaint.Render(" · ") + keys; widthOf(line) <= width {
		return []string{"", line}
	}
	return []string{"", "  " + dump, "  " + keys}
}
