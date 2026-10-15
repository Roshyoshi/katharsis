package hud

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/judge"
	"katharsis/internal/ui"
)

// SamplesMsg is emitted when a sample pass (a save's rejudge) finishes,
// for a host that records it (the game's telemetry). A pass a newer save
// cancelled sends none.
type SamplesMsg struct {
	Compiled      bool
	Passed, Total int
	// Wall is the judge's own time: the build plus every test's wall.
	Wall time.Duration
	Err  error // the pass could not finish (no compiler, say)
}

// samplesDone reports the sample pass once ev ends it.
func (m *Model) samplesDone(ev judge.Event) tea.Cmd {
	f, ok := ev.(judge.Finished)
	if !ok {
		return nil
	}
	s := SamplesMsg{Total: len(m.rows), Passed: m.passed(), Err: f.Err}
	if m.build != nil {
		s.Compiled, s.Wall = m.build.OK, m.build.Duration
	}
	for _, r := range m.rows {
		s.Wall += r.res.Wall
	}
	return func() tea.Msg { return s }
}

// Samples is the sample pass as it stands, for a host that asks before a
// full submit (the game's :Submit! when they fail).
type Samples struct {
	// Done is set once a pass has finished: none is running, and one ran
	// since the HUD opened. Compiled is false for a build that failed.
	Done, Compiled bool
	Passed, Total  int
	// Failed counts the samples judged and not AC so far: a pass still
	// running can already have disproved the source.
	Failed int
}

// Samples reports the sample pass.
func (m *Model) Samples() Samples {
	s := Samples{Total: len(m.rows), Passed: m.passed()}
	s.Done = m.build != nil && !m.running && m.passErr == nil
	s.Compiled = m.build != nil && m.build.OK
	for _, r := range m.rows {
		if r.done && r.res.Verdict != judge.AC {
			s.Failed++
		}
	}
	return s
}

// Pinned is the judgement in two lines w cells wide, for a host that pins
// it over the statement (the game's problem pane, UX J8): the HP bar, then
// the samples in one line, the first failure's detail folded into it.
//
//	HP ░░░░░░░░░░░░░░░░░░░░ not submitted
//	samples 1/2 · #2 WA line 3 · want NO got YES
func (m *Model) Pinned(w int) []string {
	out := m.submitLines(w)
	if len(out) == 0 {
		out = []string{ui.HP(w, 0, 0, "")}
	}
	return append(out, ui.Clip(m.samplesLine(), w))
}

// samplesLine is the sample pass in one line.
func (m *Model) samplesLine() string {
	label := "samples "
	switch {
	case m.loadErr != nil:
		return ui.Danger.Render(m.loadErr.Error())
	case m.watchErr != nil:
		return ui.Danger.Render("watcher: " + m.watchErr.Error() + " (r rejudges)")
	case m.passErr != nil:
		return ui.Danger.Render("error: " + m.passErr.Error())
	case m.waiting:
		return label + ui.Muted.Render("not run"+ui.G.Sep+":w judges")
	case m.build == nil && m.running:
		return label + ui.Muted.Render("compiling…")
	case m.build == nil:
		return label + ui.Muted.Render("not run")
	case !m.build.OK:
		return label + ui.Danger.Render("CE") + " " + ui.Muted.Render(firstError(m.compilerLog(m.build.Log)))
	}
	s := fmt.Sprintf("%s%d/%d", label, m.passed(), len(m.rows))
	i := m.firstFailure()
	switch {
	case i >= 0:
		res := m.rows[i].res
		s += ui.Muted.Render(ui.G.Sep) + ui.Verdict(string(res.Verdict)).Render(fmt.Sprintf("#%s %s", m.prob.Tests[i].Name, res.Verdict))
		if res.Reason != "" {
			s += " " + res.Reason
		}
		if d := res.Diff; d != nil {
			s += ui.Muted.Render(ui.G.Sep+"want ") + d.Want + ui.Muted.Render(" got ") + d.Got
		}
	case m.running:
		s += ui.Muted.Render(" running…")
	case len(m.rows) > 0:
		s += " " + ui.OK.Render(ui.G.Pass) + " " + ui.Strong.Render("ALL PASS")
	}
	return s
}
