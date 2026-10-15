package boot

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// revealTime is how long the panic takes to print. Long enough to read as
// a console scrolling past, short enough that nobody waits for it.
const revealTime = 600 * time.Millisecond

// revealMsg advances the reveal of the crash it was started for.
type revealMsg struct{ gen int }

// Crash is the kernel panic. It reads as a real dmesg tail: the last log
// lines, the cause, taint flags for the door's modifiers, the boot's doors
// as a call trace, then a quote matched to the cause and the core dump.
// u emits screens.Upsolve, e screens.OpenEditorial (when the problem has
// one), r screens.StartBoot, l screens.OpenLog and q screens.Quit. The
// editorial is a key, not opened over the panic, so the panic is seen
// first even after a :q!.
type Crash struct {
	env   Env
	shown int // elements revealed so far; < 0 shows everything
	gen   int // bumps on Init so ticks from an older reveal are dropped
	// size is the last View's width and height. How many elements the
	// panic has depends on it (a short terminal drops old log lines and
	// doors), and the reveal ends when they are all on screen.
	size [2]int
}

// NewCrash builds the panic fully printed. Call Init to play the reveal.
func NewCrash(env Env) *Crash {
	return &Crash{env: env, shown: -1, size: [2]int{1 << 10, 1 << 10}}
}

// Refresh swaps in new state and clock.
func (c *Crash) Refresh(env Env) { c.env = env }

// Init starts the reveal: the panic prints line by line over revealTime.
// Any key during the reveal skips to the end.
func (c *Crash) Init() tea.Cmd {
	c.gen++
	c.shown = 0
	return c.tick()
}

// revealing reports whether the reveal is still printing.
func (c *Crash) revealing() bool { return c.shown >= 0 && c.shown < c.total() }

func (c *Crash) total() int { return len(c.elements(c.size[0], c.size[1])) }

func (c *Crash) tick() tea.Cmd {
	gen := c.gen
	d := revealTime / time.Duration(max(c.total(), 1))
	return tea.Tick(d, func(time.Time) tea.Msg { return revealMsg{gen: gen} })
}

func (c *Crash) crash() game.Crash { return *c.env.State.Crash }

func (c *Crash) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case revealMsg:
		if msg.gen != c.gen || !c.revealing() {
			return c, nil
		}
		c.shown++
		if c.revealing() {
			return c, c.tick()
		}
		c.shown = -1
	case tea.KeyPressMsg:
		if c.revealing() {
			c.shown = -1
			return c, nil
		}
		st := c.env.State
		switch msg.String() {
		case "u":
			return c, screens.Emit(screens.Upsolve{Problem: c.crash().Door.Problem})
		case "e":
			if id := c.crash().Door.Problem; c.env.hasEditorial(id) {
				return c, screens.Emit(screens.OpenEditorial{Problem: id})
			}
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
	c.size = [2]int{width, height}
	els := c.elements(width, height)
	n := len(els)
	if c.shown >= 0 {
		n = min(c.shown, n)
	}
	var body []string
	for _, e := range els[:n] {
		body = append(body, e...)
	}
	// A console prints at the bottom and scrolls up (§2.6, E16): the keys
	// sit on the last row, the oldest lines go first, and the reveal
	// pushes the screen up as it prints.
	if height > 0 {
		if over := len(body) - height; over > 0 {
			body = body[over:]
		}
		body = append(make([]string, height-len(body)), body...)
	}
	return ui.Layout(width, height, body, "")
}

// dline is one dmesg line before it is wrapped to the terminal: its text
// painted with style, and bold too for the panic line itself.
type dline struct {
	at    time.Duration
	text  string
	style ui.Token
	bold  bool
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
		ls = ui.WrapN(l.text, max(width-sw, 10), 0)
	}
	out := make([]string, len(ls))
	for i, s := range ls {
		pre := strings.Repeat(" ", sw)
		if i == 0 {
			pre = ui.Muted.Render(ts) + " "
		}
		if s = l.style.Render(s); l.bold {
			s = ui.Strong.Render(s)
		}
		out[i] = pre + s
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
	verdict := string(cr.Verdict)
	if verdict == "" {
		verdict = string(c.attempt(cr).LastVerdict)
	}
	// One seed per crash, so the quote and the kernel frames stay put
	// across redraws and differ between crashes.
	seed := st.Seed ^ uint64(cr.At.Unix())

	quote := c.quote(cr, verdict, seed, width)
	core := c.core(cr, title, width)
	room := height - len(quote) - len(core)

	reason := fl.PanicReason(cr.Cause, verdict)
	switch {
	case cr.Cursed:
		// Cause is OutOfLives for the curse too, but lives are left; the
		// engine's own panic line names the curse.
		reason = "curse triggered"
	case reason == "":
		reason = "unknown cause"
	}
	head := []dline{}
	if pre := c.precursor(cr, verdict); pre != "" {
		head = append(head, dline{at: up, text: pre})
	}
	head = append(head,
		dline{at: up, text: fmt.Sprintf("Kernel panic - not syncing: %s in %q", reason, title), style: ui.Danger, bold: true},
		dline{at: up, text: "CPU: 0 PID: 1 Comm: you " + taint(cr.Door)},
		dline{at: up, text: fmt.Sprintf("Hardware name: katharsis %s, preset %s, seed %x",
			prestigeLabel(st.Prestige), st.Rules.Name, st.Seed)},
	)
	if mods := modules(st); mods != "" {
		head = append(head, dline{at: up, text: "Modules linked in: " + mods})
	}
	head = append(head, dline{at: up, text: "Call Trace:"})
	tail := []dline{{at: up, text: "---[ end Kernel panic ]---", style: ui.Danger}}

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
	// The rows the panic leaves fill with log tail, newest last.
	for i := len(st.Log) - 1; i >= 0; i-- {
		l := st.Log[i]
		if echoes(l, cr, verdict) {
			continue
		}
		d := dline{at: l.At.Sub(st.Started), text: l.Text, style: ui.Muted}
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

// echoes reports whether engine log line l only repeats what the screen
// prints itself: the engine's own panic line (the header says it with the
// title), its "core dumped" line (the core dump line under the quote), and
// on running out of lives the fatal submit (the precursor has the verdict
// and the tests passed). Everything else in the tail stays, the coredump
// overwrite and the regression schedule included.
func echoes(l game.LogLine, cr game.Crash, verdict string) bool {
	if strings.HasPrefix(l.Text, "Kernel panic - not syncing:") {
		return true
	}
	// The crash's own lines are logged at its instant; older ones that
	// happen to match are history, not echoes.
	if !l.At.Equal(cr.At) {
		return false
	}
	if strings.HasPrefix(l.Text, "core dumped: ") {
		return cr.Lost > 0
	}
	if cr.Cause == game.OutOfLives && verdict != "" {
		door, rest, ok := strings.Cut(l.Text, ": ")
		return ok && strings.HasPrefix(door, "door_") && strings.HasPrefix(rest, verdict+" ")
	}
	return false
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
// (the panic reads as init being killed). A cursed door crashes on its
// first failure with lives to spare, so its report says cursed, not out of
// lives.
func (c *Crash) precursor(cr game.Crash, verdict string) string {
	switch cr.Cause {
	case game.OutOfLives:
		a := c.attempt(cr)
		if verdict == "" {
			verdict = "?"
		}
		who := "Out of lives"
		if cr.Cursed || cr.Door.Cursed && c.env.State.Lives > 0 {
			who = "Cursed door"
		}
		return fmt.Sprintf("%s: Killed process 1 (you) verdict:%s passed:%d/%d submits:%d",
			who, verdict, a.BestPass, a.Total, a.Submits)
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

// upsolved reports whether the crashed door was upsolved after the crash,
// which is what banks its core dump (game.Upsolve).
func (c *Crash) upsolved(cr game.Crash) bool {
	return slices.ContainsFunc(c.env.Meta.Solves, func(s game.Solve) bool {
		return s.Upsolve && s.Problem == cr.Door.Problem && !s.At.Before(cr.At)
	})
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
	end := cr.At
	if end.IsZero() {
		end = c.env.Now
	}
	for i := len(ss) - 1; i >= 0; i-- {
		s := ss[i]
		a := s.Attempt
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
	ls := ui.WrapN(q.Text, qw, 0)
	out := []string{""}
	for i, l := range ls {
		pre := "   "
		if i == 0 {
			pre = "  \""
		}
		if i == len(ls)-1 {
			l += "\""
		}
		out = append(out, pre+ui.Voice.Render(l))
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
		out[len(out)-1] = ui.Spread(last, ui.Muted.Render(attr), right)
	} else {
		out = append(out, strings.Repeat(" ", max(right-widthOf(attr), 4))+ui.Muted.Render(attr))
	}
	return out
}

// core is the core dump line: what the crash left behind, and the keys.
// The dump is offered only while Meta still holds it on this door.
// Crash.Lost keeps what the crash dumped, but an upsolve from this screen
// banks the dump and clears Meta.CoreDump, and the line must not keep
// offering bytes that are gone. A trial crash loses its bytes without
// leaving a dump. The keys keep their brackets: a console prompt is the
// panic's idiom (C2p).
func (c *Crash) core(cr game.Crash, title string, width int) []string {
	keys := ui.Strong.Render("[u]") + " upsolve  "
	if c.env.hasEditorial(cr.Door.Problem) {
		keys += ui.Strong.Render("[e]") + " editorial  "
	}
	keys += ui.Strong.Render("[r]") + " reboot  " + ui.Strong.Render("[l]") + " git log"
	var dump string
	switch d := c.env.Meta.CoreDump; {
	case d != nil && cr.Door.Problem != "" && d.Problem == cr.Door.Problem:
		dump = "core dumped: " + ui.OK.Render(units.Bytes(d.Bytes)) + fmt.Sprintf(" on %q", title)
	case cr.Lost > 0 && c.upsolved(cr):
		dump = "core dump recovered: " + ui.OK.Render("+"+units.Bytes(cr.Lost)) + ui.Muted.Render(" banked")
	case cr.Lost > 0 && c.env.State.Trial != nil:
		dump = ui.Muted.Render("no core dump: a trial loses its " + units.Bytes(cr.Lost) + " outright")
	case cr.Lost > 0:
		dump = ui.Muted.Render("no core dump left to recover")
	default:
		dump = ui.Muted.Render("no core dump: nothing was uncommitted")
	}
	if line := "  " + dump + ui.Muted.Render(ui.G.Sep) + keys; widthOf(line) <= width {
		return []string{"", line}
	}
	return []string{"", "  " + dump, "  " + keys}
}
