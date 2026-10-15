package hud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/judge"
	"katharsis/internal/problem"
	"katharsis/internal/session"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Commands from the editor arrive as session.Request messages. In practice
// mode the HUD answers submit, debug and status itself; quack, giveup and
// retreat need a door, so a program that embeds the HUD in a run handles
// those before passing messages on.

// submission is the latest :Submit pass.
type submission struct {
	ch      <-chan judge.Event // the pass's events; see eventMsg
	cancel  context.CancelFunc
	req     session.Request // answered when the pass finishes
	started bool            // a submit has happened; show the HP bar
	running bool
	tests   []problem.Test
	src     string // the file judged: the source, or Options.SubmitCopy
	build   *judge.Build
	results []judge.Result // indexed like tests; zero Verdict = pending
	sum     *judge.Summary
	err     error
}

// debugger is the open lldb pane, if any.
type debugger struct {
	close func()
	dir   string
}

// DebugTarget is what Options.Debug gets: a -O0 -g build and how to feed it
// a test.
type DebugTarget struct {
	Binary string
	Dir    string // the program's working directory, made for this session
	Stdin  string // file for standard input; "" for file I/O (Dir holds <stem>.in)
	Test   string // the test's name, for labels
}

type debugReadyMsg struct {
	hud   *Model // the HUD that asked; a closed door's result must not attach
	req   session.Request
	test  string
	dir   string
	close func()
	err   error
}

func (m *Model) updateCommand(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case session.Request:
		return m.command(msg), true
	case debugReadyMsg:
		if msg.hud != m {
			// Its door closed while lldb started: tear it down, not adopt it.
			if msg.close != nil {
				msg.close()
			}
			if msg.dir != "" {
				os.RemoveAll(msg.dir)
			}
			msg.req.Answer(fail("debug: the door closed"))
			return nil, true
		}
		m.debugReady(msg)
		return nil, true
	}
	return nil, false
}

func fail(text string) session.Reply { return session.Reply{Text: text} }

func (m *Model) command(req session.Request) tea.Cmd {
	switch req.Cmd {
	case session.CmdSubmit:
		return m.submit(req)
	case session.CmdDebug:
		return m.startDebug(req)
	case session.CmdStatus:
		req.Answer(session.Reply{OK: true, Text: m.statusText()})
	default:
		req.Answer(fail("practice mode has no door to " + req.Cmd))
	}
	return nil
}

// SubmitBusy answers a :Submit sent while the last one is still judging.
const SubmitBusy = "a submit is already judging: wait for its verdict"

// submit judges the full tests at -O2. With Options.SubmitCopy it judges a
// copy of the source made now.
//
// One submit judges at a time: one sent meanwhile is refused, and the
// running one goes on. Replacing it threw away a verdict a host charges,
// so a failing submit followed by an AC cost nothing, and it overwrote the
// copy being judged. The refusal answers req only and ends nothing, so a
// host goes on waiting for the running submit's SubmittedMsg.
func (m *Model) submit(req session.Request) tea.Cmd {
	if m.sub.running {
		req.Answer(fail(SubmitBusy))
		return nil
	}
	if m.prob == nil {
		return refuse(req, fmt.Errorf("cannot submit: %v", m.loadErr))
	}
	src := m.prob.Source
	if c := m.opt.SubmitCopy; c != "" {
		err := os.MkdirAll(filepath.Dir(c), 0o755)
		if err == nil {
			err = copyFile(src, c)
		}
		if err != nil {
			return refuse(req, fmt.Errorf("cannot submit: %w", err))
		}
		src = c
	}
	return m.judgeSubmit(req, src)
}

// Resubmit judges again the copy the last :Submit left at
// Options.SubmitCopy, as it stands: for a host whose game quit while that
// submit was judged, so its verdict was never charged (qa/chaos r05). Call
// it after Init, which loads the problem, on a HUD with no submit running.
func (m *Model) Resubmit() tea.Cmd {
	req, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
	if m.prob == nil {
		return refuse(req, fmt.Errorf("cannot judge the submit again: %v", m.loadErr))
	}
	if _, err := os.Stat(m.opt.SubmitCopy); err != nil {
		return refuse(req, fmt.Errorf("the submitted copy is gone: %w", err))
	}
	return m.judgeSubmit(req, m.opt.SubmitCopy)
}

// refuse answers a submit that cannot start, and ends it for the host.
func refuse(req session.Request, err error) tea.Cmd {
	req.Answer(fail(err.Error()))
	return func() tea.Msg { return SubmittedMsg{Err: err} }
}

// judgeSubmit starts the submit pass on src. No pass is running (submit,
// Resubmit), so the last one has answered its request.
func (m *Model) judgeSubmit(req session.Request, src string) tea.Cmd {
	if m.sub.cancel != nil {
		m.sub.cancel() // releases the finished pass's context
	}

	tests := m.opt.FullTests
	if tests == nil {
		tests = m.prob.Tests
	}
	prof := m.opt.SubmitProfile
	lim := m.limits()
	ch, cancel := start(func(ctx context.Context, emit func(judge.Event)) {
		if prof.Name == "" {
			// Off the update loop: Profiles may wait on the sample probe.
			prof = judge.Profiles(ctx).Full
		}
		judge.Submit(ctx, src, tests, prof, lim, emit)
	})
	m.sub = submission{
		ch: ch, cancel: cancel, req: req, started: true, running: true,
		tests: tests, src: src, results: make([]judge.Result, len(tests)),
	}
	return tea.Batch(next(ch), m.updateBorder())
}

func (m *Model) applySubmit(ev judge.Event) tea.Cmd {
	switch ev := ev.(type) {
	case judge.BuildDone:
		b := ev.Build
		m.sub.build = &b
	case judge.TestDone:
		m.sub.results[ev.Index] = ev.Result
	case judge.Finished:
		m.sub.running, m.sub.err, m.sub.sum = false, ev.Err, ev.Summary
		msg := SubmittedMsg{req: m.sub.req, reply: m.submitReply()}
		if ev.Summary == nil || !m.opt.HostAnswers {
			msg.Answer("")
		}
		if ev.Summary != nil {
			msg.Summary = *ev.Summary
			return func() tea.Msg { return msg }
		}
		// A pass that ended without a verdict still ends the submit, so the
		// host stops waiting on it.
		msg.Err = ev.Err
		if msg.Err == nil {
			msg.Err = errors.New("judge stopped")
		}
		return func() tea.Msg { return msg }
	}
	return nil
}

// submitReply is the one line vim shows when a submit ends, e.g.
// "WA 37/52 · #38 line 2: want 7, got 6", before a host adds the cost
// (Options.HostAnswers).
func (m *Model) submitReply() session.Reply {
	s := m.sub
	switch {
	case s.err != nil:
		return fail("judge error: " + s.err.Error())
	case s.sum == nil:
		return fail("judge stopped")
	case s.sum.Verdict == judge.CE:
		log := ""
		if s.build != nil {
			log = firstError(relLog(s.build.Log, s.src))
		}
		return fail("CE: " + log)
	case s.sum.Verdict == judge.AC:
		return session.Reply{OK: true, Text: fmt.Sprintf("AC %d/%d · max %s, %s",
			s.sum.Passed, s.sum.Total, s.sum.MaxCPU.Round(time.Millisecond), units.Mem(s.sum.MaxRSS))}
	case s.sum.First < 0:
		return fail(fmt.Sprintf("%d/%d judged", s.sum.Done, s.sum.Total))
	}
	first := s.results[s.sum.First]
	text := fmt.Sprintf("%s %d/%d · #%s", s.sum.Verdict, s.sum.Passed, s.sum.Total, s.tests[s.sum.First].Name)
	if first.Reason != "" {
		text += " " + first.Reason
	}
	if d := first.Diff; d != nil && !m.concealed() {
		text += fmt.Sprintf(": want %s, got %s", d.Want, d.Got)
	}
	return fail(text)
}

// firstError is the compiler's first error line, else its first line.
func firstError(log string) string {
	for l := range strings.Lines(log) {
		if strings.Contains(l, "error:") {
			return strings.TrimSpace(l)
		}
	}
	l, _, _ := strings.Cut(strings.TrimSpace(log), "\n")
	return l
}

// submitLines is the HP bar, the one per pane (C6): the latest submit's
// tests passed of the full set, live while it is judged, and its verdict;
// "· best b/N" when an earlier submit passed more. Before this HUD's first
// submit it shows the host's best (Options.Best), or, embedded in a boot,
// whose pane has no other bar, "not submitted":
//
//	HP ▓▓▓▓▓▓▓▓▓░░░░░░░░░░░ 14/30  judging 22/30…
//	HP ▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░ 16/30  WA #17 line 1 · best 20/30
func (m *Model) submitLines(w int) []string {
	s := m.sub
	bp, bt := 0, 0
	if m.opt.Best != nil {
		bp, bt = m.opt.Best()
	}
	if !s.started {
		switch {
		case bt > 0:
			return []string{ui.HP(w, bp, bt, ui.Muted.Render("best"))}
		case m.opt.Embedded:
			return []string{ui.HP(w, 0, 0, "")}
		}
		return nil
	}
	switch {
	case s.err != nil:
		return []string{ui.Danger.Render("submit: " + s.err.Error())}
	case s.build == nil && s.running:
		return []string{ui.Muted.Render("submit · compiling -O2…")}
	case s.build != nil && !s.build.OK:
		return []string{ui.Danger.Render("submit CE") + ui.Muted.Render("  "+firstError(relLog(s.build.Log, s.src)))}
	}
	sum := judge.Summarize(s.results)
	var note string
	switch {
	case s.running:
		note = ui.Muted.Render(fmt.Sprintf("judging %d/%d…", sum.Done, sum.Total))
	case s.sum == nil || s.sum.First < 0 && s.sum.Verdict != judge.AC:
		note = ui.Muted.Render("stopped")
	case s.sum.Verdict == judge.AC:
		note = ui.Verdict("AC").Render("AC")
	default:
		v := string(s.sum.Verdict)
		note = ui.Verdict(v).Render(v) + " #" + s.tests[s.sum.First].Name
		if r := s.results[s.sum.First].Reason; r != "" {
			note += " " + ui.Muted.Render(r)
		}
	}
	if bt == sum.Total && bp > sum.Passed {
		note += ui.Muted.Render(fmt.Sprintf("%sbest %d/%d", ui.G.Sep, bp, bt))
	}
	return []string{ui.HP(w, sum.Passed, sum.Total, note)}
}

// startDebug builds the debug profile and opens the debugger on a test; the
// answer comes once the pane is up.
func (m *Model) startDebug(req session.Request) tea.Cmd {
	if m.opt.Debug == nil {
		req.Answer(fail("no debugger here (the HUD runs without tmux)"))
		return nil
	}
	if m.prob == nil {
		req.Answer(fail(fmt.Sprintf("cannot debug: %v", m.loadErr)))
		return nil
	}
	t, err := m.debugTest(req.Args)
	if err != nil {
		req.Answer(fail(err.Error()))
		return nil
	}
	src, open, hud := m.prob.Source, m.opt.Debug, m
	return func() tea.Msg {
		dir, closePane, err := openDebugger(src, t, open)
		return debugReadyMsg{hud: hud, req: req, test: t.Name, dir: dir, close: closePane, err: err}
	}
}

// openDebugger builds src at -O0 -g with the compiler :Submit judges with,
// gives it a working directory set up for test t, and opens the debugger
// on it.
func openDebugger(src string, t problem.Test, open func(DebugTarget) (func(), error)) (dir string, closePane func(), err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	b, err := judge.Compile(ctx, src, judge.Profiles(ctx).Debug)
	if err == nil && !b.OK {
		err = fmt.Errorf("CE: %s", firstError(relLog(b.Log, src)))
	}
	if err != nil {
		return "", nil, err
	}
	if dir, err = os.MkdirTemp("", debugDirPrefix); err != nil {
		return "", nil, err
	}
	target := DebugTarget{Binary: b.Binary, Dir: dir, Stdin: t.In, Test: t.Name}
	// A detected stem gets <stem>.in as well as stdin: detection can guess
	// wrong, and the judge feeds both.
	if stem := problem.FileIO(b.Src); stem != "" {
		err = copyFile(t.In, filepath.Join(dir, stem+".in"))
	}
	if err == nil {
		closePane, err = open(target)
	}
	if err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	return dir, closePane, nil
}

// debugTest is the test :Debug means: the one named, else the first
// failure of the last submit, else the first failing sample, else the
// selected row.
func (m *Model) debugTest(args []string) (problem.Test, error) {
	all := slices.Concat(m.prob.Tests, m.sub.tests)
	if m.concealed() {
		all = m.prob.Tests
	}
	if len(args) > 0 {
		name := strings.TrimPrefix(args[0], "#")
		for _, t := range all {
			if t.Name == name {
				return t, nil
			}
		}
		if m.concealed() {
			return problem.Test{}, fmt.Errorf("no sample %q (have %s; full tests are hidden)", name, testNames(m.prob.Tests))
		}
		return problem.Test{}, fmt.Errorf("no test %q (have %s)", name, testNames(m.prob.Tests))
	}
	if s := judge.Summarize(m.sub.results); s.First >= 0 && !m.concealed() {
		return m.sub.tests[s.First], nil
	}
	if i := m.firstFailure(); i >= 0 {
		return m.prob.Tests[i], nil
	}
	if m.sel < len(m.prob.Tests) {
		return m.prob.Tests[m.sel], nil
	}
	return problem.Test{}, fmt.Errorf("no tests to debug")
}

func testNames(ts []problem.Test) string {
	switch len(ts) {
	case 0:
		return "none"
	case 1:
		return ts[0].Name
	}
	return ts[0].Name + ".." + ts[len(ts)-1].Name
}

func (m *Model) debugReady(msg debugReadyMsg) {
	if msg.err != nil {
		msg.req.Answer(fail("debug #" + msg.test + ": " + msg.err.Error()))
		return
	}
	m.closeDebugger() // one debugger at a time
	m.dbg = debugger{close: msg.close, dir: msg.dir}
	msg.req.Answer(session.Reply{OK: true, Text: "lldb on #" + msg.test + ", stopped at main: type run"})
}

func (m *Model) closeDebugger() {
	if m.dbg.close != nil {
		m.dbg.close()
	}
	if m.dbg.dir != "" {
		os.RemoveAll(m.dbg.dir)
	}
	m.dbg = debugger{}
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

func (m *Model) concealed() bool { return m.opt.Conceal != nil && m.opt.Conceal() }
