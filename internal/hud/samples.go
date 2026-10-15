package hud

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/judge"
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
}

// Samples reports the sample pass.
func (m *Model) Samples() Samples {
	s := Samples{Total: len(m.rows), Passed: m.passed()}
	s.Done = m.build != nil && !m.running && m.passErr == nil
	s.Compiled = m.build != nil && m.build.OK
	return s
}
