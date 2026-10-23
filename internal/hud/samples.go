package hud

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/judge"
	"katharsis/internal/ui"
	"katharsis/internal/units"
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
	// running can already have disproved the source. OwnFailed are those
	// of them that are the player's own cases (problem.Test.Own).
	Failed, OwnFailed int
}

// Samples reports the sample pass.
func (m *Model) Samples() Samples {
	s := Samples{Total: len(m.rows), Passed: m.passed()}
	s.Done = m.build != nil && !m.running && m.passErr == nil
	s.Compiled = m.build != nil && m.build.OK
	for i, r := range m.rows {
		if r.done && r.res.Verdict != judge.AC {
			s.Failed++
			if m.prob.Tests[i].Own() {
				s.OwnFailed++
			}
		}
	}
	return s
}

// Pass is the sample pass in full, for the samples view (the game's w,
// screens/samples): the build with its log, then each sample's input,
// expected output and result, the program's own output among it.
type Pass struct {
	Name    string // the problem's
	Profile string // the sample build's, "asan+ubsan (libstdc++)"
	// Build is nil before the first build; its Log has the source's
	// directory cut from every path, as the HUD shows it.
	Build   *judge.Build
	Running bool
	Waiting bool  // the source is still its template, not yet saved
	Quiet   bool  // Options.Quiet: pass or fail is all it may show
	Err     error // the pass could not run: the problem, the watcher, the judge
	Tests   []PassTest
}

// PassTest is one sample: the head of its files and, once judged, its
// result.
type PassTest struct {
	Name     string
	In, Want string
	// NoWant is a case of the player's own with no answer: it is run, and
	// nothing is compared (judge.Result.Unchecked).
	NoWant bool
	Done   bool
	Result judge.Result
}

// passFileCap is the head of a sample's file a Pass carries: a sample is
// small, a test the player added need not be.
const passFileCap = 16 << 10

// Pass reports the sample pass in full. A sample's files are read once per
// problem load.
func (m *Model) Pass() Pass {
	p := Pass{Profile: m.opt.Profile.Name, Running: m.running, Waiting: m.waiting, Quiet: m.opt.Quiet,
		Err: cmp.Or(m.loadErr, m.watchErr, m.passErr)}
	if m.build != nil {
		b := *m.build
		b.Log = m.compilerLog(b.Log)
		p.Build = &b
	}
	if m.prob == nil {
		return p
	}
	p.Name = m.prob.Name
	for i, t := range m.prob.Tests {
		pt := PassTest{Name: t.Name, In: m.head(t.In), NoWant: t.Out == ""}
		if !pt.NoWant {
			pt.Want = m.head(t.Out)
		}
		if i < len(m.rows) {
			pt.Done, pt.Result = m.rows[i].done, m.rows[i].res
		}
		p.Tests = append(p.Tests, pt)
	}
	return p
}

// head is the first passFileCap of the file at path, as the HUD last read
// it, with a line saying how much more there is.
func (m *Model) head(path string) string {
	if s, ok := m.heads[path]; ok {
		return s
	}
	var s string
	if f, err := os.Open(path); err != nil {
		s = "(" + err.Error() + ")"
	} else {
		b, _ := io.ReadAll(io.LimitReader(f, passFileCap+1))
		if fi, err := f.Stat(); err == nil && len(b) > passFileCap {
			b = append(b[:passFileCap], fmt.Sprintf("\n… %s more\n", units.Mem(fi.Size()-passFileCap))...)
		}
		_ = f.Close()
		s = string(b)
	}
	if m.heads == nil {
		m.heads = map[string]string{}
	}
	m.heads[path] = s
	return s
}

// warnings is the build's compiler warnings (judge.Warnings), paths cut.
func (m *Model) warnings() []string {
	if m.build == nil || !m.build.OK {
		return nil
	}
	return judge.Warnings(m.compilerLog(m.build.Log))
}

// warningCount is "1 warning", "3 warnings", or "" for none, Warn-toned.
func (m *Model) warningCount() string {
	n := len(m.warnings())
	if n == 0 {
		return ""
	}
	return ui.Warn.Render(units.Plural(n, "warning"))
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
	case m.opt.Quiet && len(m.rows) > 0:
		word, tone := m.quiet()
		return label + tone.Render(word) // no count, no failing test (Options.Quiet)
	}
	s := fmt.Sprintf("%s%d/%d", label, m.passed(), len(m.rows))
	// The warnings come first: a narrow pane cuts the failure's detail,
	// which the samples view has in full, before them.
	if w := m.warningCount(); w != "" {
		s += ui.Muted.Render(ui.G.Sep) + w
	}
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

// ShortWarning is a warning as one line: its place and its text,
// "47:17 variable 'i' is uninitialized when used here [-Wuninitialized]",
// without the file every warning of a one-file build names.
func ShortWarning(w string) string {
	if _, rest, ok := strings.Cut(w, ": warning: "); ok {
		place := w[:len(w)-len(rest)-len(": warning: ")]
		if i := strings.Index(place, ":"); i >= 0 {
			place = place[i+1:]
		}
		return place + " " + rest
	}
	return w
}
