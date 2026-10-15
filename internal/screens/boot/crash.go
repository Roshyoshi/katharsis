package boot

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/story"
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
// as a call trace, then a quote matched to the cause, and under it the
// debrief (retention RT1): the core dump's fate, what the boot kept, the
// one lever and the next step.
//
// u emits screens.Upsolve, e screens.OpenEditorial (when the problem has
// one), r screens.StartBoot, and an offered preset's key (s short, p
// patch) one on that preset; with a live core dump a reboot key asks
// twice, since the next crash destroys the dump (RT4). b reads the story
// beat waiting (story.Read), m RTFM when it is the next step
// (screens.OpenLearn), l opens git log, g and q go to GRUB
// (screens.OpenGrub, UX J12). The editorial is a key, not opened over the
// panic, so the panic is seen first even after a :q!.
type Crash struct {
	env   Env
	shown int // elements revealed so far; < 0 shows everything
	gen   int // bumps on Init so ticks from an older reveal are dropped
	// size is the last View's width and height. How many elements the
	// panic has depends on it (a short terminal drops old log lines and
	// doors), and the reveal ends when they are all on screen.
	size [2]int
	// count is how many elements the panic has at size, -1 until counted. They
	// depend only on the state and the size, so View counts them as it
	// builds them, Refresh forgets the count, and the reveal's ticks read it
	// instead of building them all again.
	count int
	// armed is the reboot key pressed once over a live core dump; the
	// same key again reboots, any other disarms.
	armed string
}

// NewCrash builds the panic fully printed. Call Init to play the reveal.
func NewCrash(env Env) *Crash {
	return &Crash{env: env, shown: -1, size: [2]int{1 << 10, 1 << 10}, count: -1}
}

// Refresh swaps in new state and clock.
func (c *Crash) Refresh(env Env) { c.env, c.count = env, -1 }

// Init starts the reveal: the panic prints line by line over revealTime.
// Any key during the reveal skips to the end.
func (c *Crash) Init() tea.Cmd {
	c.gen++
	c.shown = 0
	return c.tick()
}

// revealing reports whether the reveal is still printing.
func (c *Crash) revealing() bool { return c.shown >= 0 && c.shown < c.total() }

func (c *Crash) total() int {
	if c.count < 0 {
		c.count = len(c.elements(c.size[0], c.size[1]))
	}
	return c.count
}

func (c *Crash) tick() tea.Cmd {
	gen := c.gen
	d := revealTime / time.Duration(max(c.total(), 1))
	return tea.Tick(d, func(time.Time) tea.Msg { return revealMsg{gen: gen} })
}

func (c *Crash) crash() game.Crash { return *c.env.State.Crash }

// uptime is the boot's uptime at its crash: the engine's number, the one the
// panic screen shows, or the boot's doors up to the crash in a snapshot
// saved without it.
func (c *Crash) uptime(cr game.Crash) time.Duration {
	if cr.Uptime == 0 {
		return c.env.State.Uptime(cr.At)
	}
	return cr.Uptime
}

func (c *Crash) brief() game.Brief { return game.Debrief(c.env.State, &c.env.Meta) }

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
		key := msg.String()
		armed := c.armed
		c.armed = ""
		if b, ok := rebootKey(c.env, key); ok {
			if c.brief().Dump.Kind == game.DumpLive && armed != key {
				c.armed = key
				return c, nil
			}
			return c, screens.Emit(b)
		}
		switch key {
		case "u":
			if id := c.crash().Door.Problem; id != "" { // an abandoned boot has no door
				return c, screens.Emit(screens.Upsolve{Problem: id})
			}
		case "e":
			if id := c.crash().Door.Problem; c.env.hasEditorial(id) {
				return c, screens.Emit(screens.OpenEditorial{Problem: id})
			}
		case beatKey:
			if id := beat(c.env); id != "" {
				return c, screens.Emit(story.Read{ID: id})
			}
		case learnKey:
			if l := c.env.End.Learn; l != nil {
				return c, screens.Emit(screens.OpenLearn{Topic: l.Topic})
			}
		case "l":
			return c, screens.Emit(screens.OpenLog{})
		case "g", "q":
			return c, screens.Emit(screens.OpenGrub{})
		}
	}
	return c, nil
}

func (c *Crash) View(width, height int) string {
	c.size = [2]int{width, height}
	els := c.elements(width, height)
	c.count = len(els)
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
// then the debrief's blocks. The debrief and the quote come first: the
// panic gets the rows they leave, down to its floor, and fits them by
// dropping old log lines first, then eliding the middle of the call trace.
// Story text and the beat's teaser give way before the floor does.
func (c *Crash) elements(width, height int) [][]string {
	st := c.env.State
	cr := c.crash()
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

	floor := rows(c.panic(cr, verdict, title, seed, width, 0))
	end := fit(c.end(cr, verdict, title, seed, width), height-floor)
	return append(c.panic(cr, verdict, title, seed, width, height-rows(end)), end...)
}

// panic is the kernel's own lines, one element per dmesg line, fitted
// into room rows as far as its floor allows: the panic line, the trace's
// innermost frame, the door that crashed and the boot frame always stay.
func (c *Crash) panic(cr game.Crash, verdict, title string, seed uint64, width, room int) [][]string {
	st := c.env.State
	up := c.uptime(cr)
	reason := fl.PanicReason(cr.Cause, verdict)
	switch {
	case cr.Cursed:
		// Cause is OutOfLives for the curse too, but cycles are left; the
		// engine's own panic line names the curse.
		reason = "curse triggered"
	case reason == "":
		reason = "unknown cause"
	}
	panicLine := fmt.Sprintf("Kernel panic - not syncing: %s in %q", reason, title)
	if cr.Door.Problem == "" {
		// An abandoned boot crashed between doors: no door to name.
		panicLine = "Kernel panic - not syncing: " + reason
	}
	// The head's lines in order; drop says which give way, and in what
	// order, when even the shortest trace does not fit: the modules, the
	// hardware, then the precursor (the debrief's lever says it again).
	type headLine struct {
		dline
		drop int // 0 stays
	}
	var heads []headLine
	if pre := c.precursor(cr, verdict); pre != "" {
		heads = append(heads, headLine{dline{at: up, text: pre}, 3})
	}
	heads = append(heads,
		headLine{dline{at: up, text: panicLine, style: ui.Danger, bold: true}, 0},
		headLine{dline{at: up, text: "CPU: 0 PID: 1 Comm: you " + taint(cr.Door)}, 0},
		headLine{dline{at: up, text: fmt.Sprintf("Hardware name: katharsis %s, preset %s, seed %x",
			prestigeLabel(st.Prestige), st.Rules.Name, st.Seed)}, 2},
	)
	if mods := modules(st); mods != "" {
		heads = append(heads, headLine{dline{at: up, text: "Modules linked in: " + mods}, 1})
	}
	heads = append(heads, headLine{dline{at: up, text: "Call Trace:"}, 0})
	tail := []dline{{at: up, text: "---[ end Kernel panic ]---", style: ui.Danger}}
	headOf := func(dropped int) []dline {
		var out []dline
		for _, h := range heads {
			if h.drop == 0 || h.drop > dropped {
				out = append(out, h.dline)
			}
		}
		return out
	}

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
	head := headOf(0)
	rest := slices.Concat(doors, []string{boot})
	tooTall := func() bool { return lines(head)+lines(tail)+lines(dlines(up, slices.Concat(kernel, rest))) > room }
	// Elide the oldest doors until the trace fits, then the outer kernel
	// frames, then the head's lines that can go; the panic line, the
	// innermost frame, the door that crashed and the boot frame always
	// stay.
	for k := len(doors) - 1; k >= 1 && tooTall(); k-- {
		rest = slices.Concat(doors[:k], []string{fmt.Sprintf(" ... %d older doors", len(doors)-k), boot})
	}
	for k := len(kernel) - 1; k >= 1 && tooTall(); k-- {
		kernel = kernel[:k]
	}
	for d := 1; d <= 3 && tooTall(); d++ {
		head = headOf(d)
	}
	body := slices.Concat(head, dlines(up, slices.Concat(kernel, rest)), tail)

	var pre []dline
	free := room - lines(body)
	// The rows the panic leaves fill with log tail, newest last.
	for i := len(st.Log) - 1; i >= 0; i-- {
		l := st.Log[i]
		if echoes(l, cr, verdict) {
			continue
		}
		d := dline{at: st.Uptime(l.At), text: l.Text, style: ui.Muted}
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
	return out
}

// end is everything under the panic, as pieces fit chooses from: the
// quote and its epitaph, the core dump's fate as the headline (the earlier
// dump this crash destroyed first, RT4), then kept, lever and next
// (RT1), the session budget's nudge (RT12) and the keys. The keys keep
// their brackets: a console prompt is the panic's idiom (C2p).
func (c *Crash) end(cr game.Crash, verdict, title string, seed uint64, width int) []piece {
	env := c.env
	b := c.brief()
	ps := []piece{
		{lines: c.quote(cr, verdict, seed, width)},
		{lines: voiceLines(c.epitaph(cr, verdict, seed), 5, width), pri: 2},
		{lines: []string{""}},
	}
	if h := lostHeadline(env, b.Dump); h != "" {
		ps = append(ps, piece{lines: []string{"  " + h}})
		verse := tale.Block("coredump.lost")
		for i, l := range verse {
			verse[i] = flavor.Fill(l, "bytes", units.Bytes(b.Dump.Lost.Bytes))
		}
		ps = append(ps, piece{lines: voiceLines(verse, 4, width), pri: 4})
	}
	ps = append(ps, piece{lines: []string{"  " + ui.Clip(dumpItem(b.Dump, title), width-2)}})

	kept := []string{ratingItem(b.Rating, "rating", true)}
	if b.Topic != "" {
		kept = append(kept, ratingItem(b.TopicRating, b.Topic, true))
	}
	kept = append(kept, ui.Muted.Render("bank ")+units.Bytes(b.Bank), timePlusItem(env.End.TimePlus, c.uptime(cr)),
		regressionItem(b.Regression, env.Now))
	kept = append(kept, easeItems(env.State)...)
	kept = append(kept, brk, goalItem(env.End.Goal, b.Bank), brk, gateItem(b.Gate, env.Meta.Prestige))
	ps = append(ps,
		piece{lines: section("kept", kept, width)},
		piece{lines: teaser(beat(env), width), pri: 1},
		piece{lines: section("lever", []string{leverText(env, b.Lever), forecastItem(env.State)}, width)},
		piece{lines: c.next(b, width)},
		piece{lines: budgetLine(env.End, width), pri: 3},
		piece{lines: c.keys(b, width)},
	)
	return ps
}

// learnKey opens RTFM when it is the next step.
const learnKey = "m"

// next is the one highlighted step (RT1): upsolve a live dump, else RTFM
// on a topic that keeps crashing (End.Learn), else reboot. While a reboot
// key is armed it says what the reboot risks.
func (c *Crash) next(b game.Brief, width int) []string {
	env := c.env
	var s string
	switch {
	case c.armed != "":
		s = ui.Danger.Render("reboot leaves "+units.Bytes(b.Dump.Bytes)+" at risk") +
			ui.Muted.Render(ui.G.Sep) + ui.Strong.Render(c.armed) + " again" +
			ui.Muted.Render(ui.G.Sep) + ui.Strong.Render("u") + " upsolve first"
	case b.Next.Kind == game.NextUpsolve:
		s = upsolveText(env, b.Next, b.Dump)
	case env.End.Learn != nil:
		l := env.End.Learn
		s = ui.Strong.Render(learnKey) + " RTFM " + topicName(l.Topic) +
			ui.Muted.Render(ui.G.Sep+fmt.Sprintf("%d crashes on it in 14 days", l.Crashes))
	default:
		s = rebootStep(env)
	}
	return nextSection(s, width)
}

// keys is the console prompt, one line: every key but the highlighted
// one, git log and GRUB last.
func (c *Crash) keys(b game.Brief, width int) []string {
	env := c.env
	var pairs []string
	// No upsolve key once the dump is banked (UX J31), nor twice when it
	// is the highlighted step.
	if b.Next.Kind != game.NextUpsolve && b.Dump.Kind != game.DumpRecovered && c.crash().Door.Problem != "" {
		pairs = append(pairs, "u", "upsolve")
	}
	if c.env.hasEditorial(c.crash().Door.Problem) {
		pairs = append(pairs, "e", "editorial")
	}
	reb := rebootPairs(env)
	if b.Next.Kind == game.NextReboot && env.End.Learn == nil {
		reb = reb[2:]
	}
	pairs = append(pairs, reb...)
	if beat(env) != "" {
		pairs = append(pairs, beatKey, "story")
	}
	pairs = append(pairs, "l", "git log", "g", "GRUB")
	// One line: pairs give way from the end, whole, but GRUB, the way
	// out, stays (as ui.Footer drops them). The story's key is also on its
	// teaser, and git log is a GRUB entry.
	line := func(ps []string) string {
		s := ""
		for i := 0; i+1 < len(ps); i += 2 {
			s += "  " + ui.Strong.Render("["+ps[i]+"]") + " " + ps[i+1]
		}
		return s
	}
	n := len(pairs) / 2
	for k := n; k > 1; k-- {
		pick := append(append([]string(nil), pairs[:2*(k-1)]...), pairs[2*(n-1):]...)
		if k == n {
			pick = pairs
		}
		if s := line(pick); widthOf(s) <= width {
			return []string{s}
		}
	}
	return []string{ui.Clip(line(pairs[2*(n-1):]), width)}
}

// epitaph is the story's line under the quote (text.md, Crash): a trial's
// is the Gryphon's (rtfm.trial.fail), whatever ended it; a boot's is by
// cause, the curse first, and out of cycles by its verdict where it has
// one.
func (c *Crash) epitaph(cr game.Crash, verdict string, seed uint64) []string {
	hook := "crash.epitaph." + string(cr.Cause)
	switch {
	case c.env.State.Trial != nil:
		hook = "rtfm.trial.fail"
	case cr.Cursed:
		hook = "crash.epitaph.cursed"
	case cr.Cause == game.OutOfLives && verdict != "":
		if s := tale.Story(hook+"."+flavor.NormVerdict(verdict), seed); s != "" {
			return []string{s}
		}
	}
	if s := tale.Story(hook, seed); s != "" {
		return []string{s}
	}
	return nil
}

// echoes reports whether engine log line l only repeats what the screen
// prints itself: the engine's own panic line (the header says it with the
// title), its "core dumped" line (the core dump line under the quote), and
// on running out of cycles the fatal submit (the precursor has the verdict
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
// line alone hides what happened: running out of cycles on submits gets
// the OOM killer's report with the verdict and the HP bar; a timeout or
// :q! that ended the boot says why it did, a one-shot boss's door (the
// final, D52) or the burn that took the last cycles (D47); a :q! names
// itself (the panic reads as init being killed); an abandoned boot, the
// new one that ended it. A cursed door crashes on its first failure with
// cycles to spare, so its report says cursed, not out of cycles.
func (c *Crash) precursor(cr game.Crash, verdict string) string {
	st := c.env.State
	quit := "you[1]: :q! sent SIGKILL to init"
	switch cr.Cause {
	case game.OutOfLives:
		a := c.attempt(cr)
		if verdict == "" {
			verdict = "?"
		}
		who := "Out of cycles"
		if cr.Cursed || cr.Door.Cursed && st.Cycles > 0 {
			who = "Cursed door"
		}
		return fmt.Sprintf("%s: Killed process 1 (you) verdict:%s passed:%d/%d submits:%d",
			who, verdict, a.BestPass, a.Total, a.Submits)
	case game.TimedOut, game.GaveUp:
		what := "the timeout"
		if cr.Cause == game.GaveUp {
			what = ":q!"
		}
		cost, _ := st.FailCost(cr.Cause)
		switch {
		case cr.Door.Boss != "" && st.OneShot():
			s := strings.ReplaceAll(cr.Door.Boss, "-", "_") + ": one shot, no second door"
			if cr.Cause == game.GaveUp {
				s += ui.G.Sep + quit
			}
			return s
		case cost > 0 && st.Cycles > 0 && st.Cycles <= cost:
			return fmt.Sprintf("Out of cycles: %s burns %d, %d were left", what, cost, st.Cycles)
		case cost > 0:
			return fmt.Sprintf("Out of cycles: %s burns the last %d", what, cost)
		case cr.Cause == game.GaveUp:
			return quit
		}
	case game.Abandoned:
		return "you[1]: a new boot sent SIGTERM to init"
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
// flag order: "Tainted: B (blind) C (cursed)".
func taint(d game.Door) string {
	var fs []string
	for _, f := range []struct {
		on  bool
		mod ui.Mod
	}{{d.Regression, ui.ModRegression}, {d.Mystery, ui.ModMystery}, {d.Blind, ui.ModBlind}, {d.Cursed, ui.ModCursed}, {d.Boss != "", ui.ModBoss}} {
		if f.on {
			fs = append(fs, f.mod.Taint()+" ("+f.mod.Word()+")")
		}
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

// topicName is a topic as the player meets it, by its biome's name ("dp"
// is The Cache), or its slug for a topic with no name.
func topicName(topic string) string {
	if n := fl.BiomeName(topic); n != "" {
		return n
	}
	return topic
}
