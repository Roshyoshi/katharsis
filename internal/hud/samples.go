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
