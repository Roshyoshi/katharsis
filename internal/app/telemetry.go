package app

import (
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/store"
)

// Telemetry: the facts only the app sees (ml-pipeline.md §4.3, retention
// RT13), kept in the store's events table and nowhere else (GR12). Each
// call site is one line; this file holds the rest.
//
// An event never blocks the UI and never costs a save: emit queues it, and
// one goroutine writes the queue in order, each event in a transaction of
// its own. A full queue (the store locked by another process for seconds)
// drops events; a failed write is logged and the game goes on.

// kv is an event's payload.
type kv = map[string]any

// tele is the Model's telemetry state. Only Update's goroutine touches it.
type tele struct {
	rec    *recorder
	log    *log.Logger
	screen string // the base screen last reported

	// editorial is the reader showing an editorial, so its close can be
	// told from a statement's; edAt is when it opened.
	editorial *reader.Model
	edProblem catalog.ID
	edAt      time.Time

	upsolveLast string // the last verdict of the upsolve open, "" for none

	// end is the line under a boot's end screen (retention.go); worthDay
	// and feltBoot are the day and boot it last asked about.
	end      endLine
	worthDay string
	feltBoot int
}

// eventQueue bounds the events waiting for the store.
const eventQueue = 256

// eventFlush bounds how long quitting waits for the queue to drain.
const eventFlush = time.Second

// recorder writes queued events to the store from its own goroutine. A
// flush marker (a chan struct{}) in the queue is closed once every event
// before it is written.
type recorder struct {
	ch     chan any
	done   chan struct{}
	log    *log.Logger
	closed bool
}

func newRecorder(s *store.Store, l *log.Logger) *recorder {
	r := &recorder{ch: make(chan any, eventQueue), done: make(chan struct{}), log: l}
	go r.run(s)
	return r
}

func (r *recorder) run(s *store.Store) {
	defer close(r.done)
	var session int64
	for v := range r.ch {
		switch v := v.(type) {
		case chan struct{}:
			close(v)
		case store.Event:
			if v.Kind != store.EventAppStart {
				v.Session = session
			}
			id, err := s.Event(v)
			if err != nil {
				r.log.Printf("event %s: %v", v.Kind, err)
				continue
			}
			if v.Kind == store.EventAppStart {
				session = id
			}
		}
	}
}

// emit queues e without waiting.
func (r *recorder) emit(e store.Event) {
	if r.closed {
		return
	}
	select {
	case r.ch <- e:
	default:
		r.log.Printf("event %s dropped: %d queued", e.Kind, eventQueue)
	}
}

// flush waits up to d for the events queued so far to be written.
func (r *recorder) flush(d time.Duration) bool {
	if r.closed {
		return false
	}
	mark := make(chan struct{})
	select {
	case r.ch <- mark:
	case <-time.After(d):
		return false
	}
	select {
	case <-mark:
		return true
	case <-time.After(d):
		return false
	}
}

// close stops the queue and waits up to d for it to drain.
func (r *recorder) close(d time.Duration) {
	if r.closed {
		return
	}
	r.closed = true
	close(r.ch)
	select {
	case <-r.done:
	case <-time.After(d):
		r.log.Printf("events: %v passed with events still queued", d)
	}
}

// startTelemetry starts the session: app_start, then the first screen.
func (m *Model) startTelemetry() {
	m.tele.log = m.d.Log
	if m.tele.log == nil {
		m.tele.log = log.New(io.Discard, "", 0)
	}
	m.tele.rec = newRecorder(m.d.Store, m.tele.log)
	var presets []string
	for name := range m.d.Presets {
		presets = append(presets, name)
	}
	slices.Sort(presets)
	_, tmux := m.d.Runner.(hudRunner)
	m.event(store.EventAppStart, "", kv{"version": version(), "presets": presets, "tmux": tmux})
	m.screenShown()
}

// stopTelemetry ends the session cleanly: an upsolve still open closes,
// app_quit lands, and the queue drains (bounded).
func (m *Model) stopTelemetry() {
	if m.tele.rec == nil {
		return
	}
	m.doorClosed()
	m.endLineDone()
	m.event(store.EventAppQuit, "", nil)
	m.tele.rec.close(eventFlush)
}

// event queues one event at the app's clock, in the boot on screen (also
// one that ended) and at its open door's attempt. data is its payload: a
// kv, or a store payload type (store.SetuidSet).
func (m *Model) event(kind string, problem catalog.ID, data any) {
	if m.tele.rec == nil {
		return
	}
	e := store.Event{At: m.d.Now(), Problem: problem, Kind: kind}
	if data != nil {
		e.Data = data
	}
	if s := m.state; s != nil {
		e.Boot = s.Boot
		if m.attempt() != nil {
			i := len(s.History)
			e.Attempt = &i
		}
	}
	m.tele.rec.emit(e)
}

// inBoot reports whether a boot is still being played, for payloads that
// say "during a boot or not".
func (m *Model) inBoot() bool {
	return m.state != nil && m.state.Phase != game.PhaseCrashed && m.state.Phase != game.PhaseCleared
}

// screenName names the base screen, as baseScreen chooses it.
func (m *Model) screenName() string {
	switch {
	case m.state == nil:
		return "grub"
	case m.state.Phase == game.PhaseDoors && m.state.PendingUpgrade:
		return "upgrade"
	}
	return string(m.state.Phase)
}

// screenShown reports a new base screen; its dwell runs to the next event.
func (m *Model) screenShown() {
	if name := m.screenName(); name != m.tele.screen && m.tele.rec != nil {
		m.tele.screen = name
		m.event(store.EventScreen, "", kv{"screen": name})
		m.armEndLine(name)
	}
}

// dropped notes screens leaving the stack: the editorial reader closing.
func (m *Model) dropped(ss ...screens.Screen) {
	ed := m.tele.editorial
	if ed == nil || !slices.ContainsFunc(ss, func(s screens.Screen) bool { return s == ed }) {
		return
	}
	m.tele.editorial = nil
	m.event(store.EventEditorialClose, m.tele.edProblem,
		kv{"in_boot": m.inBoot(), "dwell_ms": m.d.Now().Sub(m.tele.edAt).Milliseconds()})
}

// docShown pushes a doc's reader, noting it when it is an editorial.
func (m *Model) docShown(msg docMsg) screens.Screen {
	r := reader.New(msg.doc)
	if msg.editorial != "" {
		m.tele.editorial, m.tele.edProblem, m.tele.edAt = r, msg.editorial, m.d.Now()
		m.event(store.EventEditorialOpen, msg.editorial, kv{"in_boot": m.inBoot()})
	}
	return r
}

// samplesJudged reports a sample pass the HUD finished (a :w).
func (m *Model) samplesJudged(s hud.SamplesMsg) {
	var problem catalog.ID
	if m.open != nil {
		problem = m.open.Door.Problem
	}
	d := kv{"compiled": s.Compiled, "passed": s.Passed, "total": s.Total, "wall_ms": s.Wall.Milliseconds(),
		"upsolve": m.open != nil && m.open.Upsolve}
	if s.Err != nil {
		d["error"] = s.Err.Error()
	}
	m.event(store.EventSamplesJudged, problem, d)
}

// upsolveOpened reports an upsolve door that opened.
func (m *Model) upsolveOpened() {
	if d := m.open; d != nil && d.Upsolve {
		m.tele.upsolveLast = ""
		m.event(store.EventUpsolveOpen, d.Door.Problem, kv{"editorial": d.ReadEditorial})
	}
}

// upsolveJudged reports an upsolve's full-test verdict.
func (m *Model) upsolveJudged(j judgedMsg) {
	d := m.open
	if d == nil || !d.Upsolve {
		return
	}
	m.tele.upsolveLast = j.verdict
	m.event(store.EventUpsolveJudged, d.Door.Problem, kv{"verdict": j.verdict, "passed": j.passed,
		"total": j.total, "editorial": d.ReadEditorial, "elapsed_ms": m.d.Now().Sub(d.Opened).Milliseconds()})
}

// doorClosed reports an upsolve door closing, solved or left.
func (m *Model) doorClosed() {
	d := m.open
	if d == nil || !d.Upsolve {
		return
	}
	m.event(store.EventUpsolveClose, d.Door.Problem, kv{"verdict": m.tele.upsolveLast,
		"editorial": d.ReadEditorial, "elapsed_ms": m.d.Now().Sub(d.Opened).Milliseconds()})
	m.tele.upsolveLast = ""
}

// version is the build's module version and VCS revision, as far as the
// binary knows them.
func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	v := bi.Main.Version
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 12 {
			v += " " + s.Value[:12]
		}
	}
	return fmt.Sprintf("%s %s", v, bi.GoVersion)
}
