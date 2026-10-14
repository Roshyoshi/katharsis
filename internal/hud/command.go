package hud

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"deadcases/internal/judge"
	"deadcases/internal/problem"
	"deadcases/internal/session"
)

// Commands from the editor arrive as session.Request messages. In practice
// mode the HUD answers submit, debug and status itself; quack, giveup and
// retreat need a door, so a program that embeds the HUD in a run handles
// those before passing messages on.

// submission is the latest :Submit pass.
type submission struct {
	gen     int
	cancel  context.CancelFunc
	req     session.Request // answered when the pass finishes
	started bool            // a submit has happened; show the HP bar
	running bool
	tests   []problem.Test
	build   *judge.Build
	results []judge.Result // indexed like tests; zero Verdict = pending
	sum     *judge.Summary
	err     error
}

// submitMsg carries one event of submit pass gen.
type submitMsg struct {
	gen int
	ev  judge.Event
	ch  <-chan judge.Event
}

// debugger is the open lldb pane, if any.
type debugger struct {
	close func()
	dir   string
	test  string
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
	case submitMsg:
		if msg.gen != m.sub.gen {
			return nil, true
		}
		return tea.Batch(m.applySubmit(msg.ev), nextSubmit(msg.gen, msg.ch), m.updateBorder()), true
	case debugReadyMsg:
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

// submit judges the full tests at -O2, replacing any submit still running.
func (m *Model) submit(req session.Request) tea.Cmd {
	if m.prob == nil {
		req.Answer(fail(fmt.Sprintf("cannot submit: %v", m.loadErr)))
		return nil
	}
	if m.sub.cancel != nil {
		m.sub.cancel()
	}
	m.sub.req.Answer(fail("superseded by a newer :Submit"))

	tests := m.opt.FullTests
	if tests == nil {
		tests = m.prob.Tests
	}
	prof := m.opt.SubmitProfile
	if prof.Name == "" {
		prof = judge.Full
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.sub = submission{
		gen: m.sub.gen + 1, cancel: cancel, req: req, started: true, running: true,
		tests: tests, results: make([]judge.Result, len(tests)),
	}
	ch := make(chan judge.Event, 8)
	src, lim := m.prob.Source, m.limits()
	go func() {
		defer close(ch)
		judge.Submit(ctx, src, tests, prof, lim, func(ev judge.Event) {
			select {
			case ch <- ev:
			case <-ctx.Done():
			}
		})
	}()
	return tea.Batch(nextSubmit(m.sub.gen, ch), m.updateBorder())
}

func nextSubmit(gen int, ch <-chan judge.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return submitMsg{gen: gen, ev: ev, ch: ch}
	}
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
		m.sub.req.Answer(m.submitReply())
		if ev.Summary != nil {
			s := *ev.Summary
			return func() tea.Msg { return SubmittedMsg{Summary: s} }
		}
	}
	return nil
}

// submitReply is the one line vim shows when a submit ends, e.g.
// "WA 37/52 · #38 line 2: want 7, got 6".
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
			log = firstError(s.build.Log)
		}
		return fail("CE: " + log)
	case s.sum.Verdict == judge.AC:
		return session.Reply{OK: true, Text: fmt.Sprintf("AC %d/%d · max %s, %s",
			s.sum.Passed, s.sum.Total, s.sum.MaxCPU.Round(time.Millisecond), judge.Bytes(s.sum.MaxRSS))}
	case s.sum.First < 0:
		return fail(fmt.Sprintf("%d/%d judged", s.sum.Done, s.sum.Total))
	}
	first := s.results[s.sum.First]
	text := fmt.Sprintf("%s %d/%d · #%s", s.sum.Verdict, s.sum.Passed, s.sum.Total, s.tests[s.sum.First].Name)
	if first.Reason != "" {
		text += " " + first.Reason
	}
	if d := first.Diff; d != nil {
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

// HPBar draws passed of total as a bar of cells: ▓▓▓▓░░. It is full only
// when every test passed.
func HPBar(passed, total, cells int) string {
	if cells <= 0 {
		return ""
	}
	filled := 0
	if total > 0 {
		filled = min(passed*cells/total, cells)
	}
	return strings.Repeat("▓", filled) + strings.Repeat("░", cells-filled)
}

// submitLines is the HP bar section, shown once something was submitted:
// "submit ▓▓▓▓▓▓░░ 37/52  WA #38 line 2".
func (m *Model) submitLines(w int) []string {
	s := m.sub
	if !s.started {
		return nil
	}
	switch {
	case s.err != nil:
		return []string{red.Render("submit: " + s.err.Error())}
	case s.build == nil && s.running:
		return []string{faint.Render("submit · compiling -O2…")}
	case s.build != nil && !s.build.OK:
		return []string{red.Render("submit CE") + faint.Render("  "+firstError(s.build.Log))}
	}
	sum := judge.Summarize(s.results)
	style := faint
	switch {
	case s.sum != nil && s.sum.Verdict == judge.AC:
		style = green
	case s.sum != nil:
		style = verdictStyle(s.sum.Verdict)
	case sum.First >= 0:
		style = red
	}
	out := "submit " + style.Render(HPBar(sum.Passed, sum.Total, min(max(w-24, 6), 24))) +
		fmt.Sprintf(" %d/%d", sum.Passed, sum.Total)
	switch {
	case s.running:
		out += faint.Render(fmt.Sprintf(" judging %d/%d…", sum.Done, sum.Total))
	case s.sum == nil || s.sum.First < 0 && s.sum.Verdict != judge.AC:
		out += faint.Render(" stopped")
	case s.sum.Verdict == judge.AC:
		out += "  " + green.Bold(true).Render("AC")
	default:
		v := s.sum.Verdict
		out += "  " + verdictStyle(v).Render(string(v)) + " #" + s.tests[s.sum.First].Name
		if r := s.results[s.sum.First].Reason; r != "" {
			out += " " + faint.Render(r)
		}
	}
	return []string{out}
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
	src, open := m.prob.Source, m.opt.Debug
	return func() tea.Msg {
		ready := debugReadyMsg{req: req, test: t.Name}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		b, err := judge.Compile(ctx, src, judge.Debug)
		if err == nil && !b.OK {
			err = fmt.Errorf("CE: %s", firstError(b.Log))
		}
		if err != nil {
			ready.err = err
			return ready
		}
		dir, err := os.MkdirTemp("", "deadcases-debug-")
		if err != nil {
			ready.err = err
			return ready
		}
		target := DebugTarget{Binary: b.Binary, Dir: dir, Stdin: t.In, Test: t.Name}
		if stem := problem.FileIO(b.Src); stem != "" {
			target.Stdin = ""
			if err := copyFile(t.In, filepath.Join(dir, stem+".in")); err != nil {
				os.RemoveAll(dir)
				ready.err = err
				return ready
			}
		}
		closePane, err := open(target)
		if err != nil {
			os.RemoveAll(dir)
			ready.err = err
			return ready
		}
		ready.dir, ready.close = dir, closePane
		return ready
	}
}

// debugTest is the test :Debug means: the one named, else the first
// failure of the last submit, else the first failing sample, else the
// selected row.
func (m *Model) debugTest(args []string) (problem.Test, error) {
	all := slices.Concat(m.prob.Tests, m.sub.tests)
	if len(args) > 0 {
		name := strings.TrimPrefix(args[0], "#")
		for _, t := range all {
			if t.Name == name {
				return t, nil
			}
		}
		return problem.Test{}, fmt.Errorf("no test %q (have %s)", name, testNames(m.prob.Tests))
	}
	if s := judge.Summarize(m.sub.results); s.First >= 0 {
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
	m.dbg = debugger{close: msg.close, dir: msg.dir, test: msg.test}
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
