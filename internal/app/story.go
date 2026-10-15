package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/story"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// The story's scenes over the game (S9; docs/story/story-bible.md, "How
// the story unfolds", and the map in .agents/work/s9-part2.md). The app
// picks the moment and internal/screens/story draws it: a scene is a
// screen pushed over the one the phase shows, and it pops when it leaves.
//
//	boot start      splash · prologue (once) · biome drop or boss card ·
//	                first door (once)
//	route chosen    biome drop · first door (once, if biome 0 is a route)
//	AC, a pick owed ACCEPTED · first merge window (once)
//	merge left      biome drop, or the boss card: the King's rule before
//	                the first boss, the Jabberwock before the first final
//	                boss
//	crash           the Cat, once the panic has printed (the first crash
//	                that left bytes: flavor.NextBeat)
//
// The rules every scene keeps:
//   - It never covers an open door. None is queued in the problem phase,
//     and a phase change rebuilds the stack under it.
//   - Each key does one thing: shows the page, turns it, or leaves, and a
//     scene that leaves on its own leaves on any key. esc leaves the scene
//     and the ones queued behind it.
//   - A one-time scene is seen once it has been on screen (game.MarkSeen
//     on story.Done, saved at once). One a quit cut short shows again on
//     resume; the every-time ones do not replay.
//   - Reduced motion (anim.SetMotion) shows each page's last frame at once.
//
// A learn-mode trial is the Mock Turtle's school and has its own beats: it
// gets ACCEPTED and nothing else.

// storyBeat is the RT6 event for a one-time scene that left: its id and
// how long it was up (docs/research/retention.md RT6).
const storyBeat = "story_beat"

// crashLead is how long the Cat waits after the crash screen appears: the
// panic prints in 600 ms (screens/boot), then holds a moment.
const crashLead = 1500 * time.Millisecond

// storyState is the scene on screen and the scenes queued behind it.
type storyState struct {
	on    *story.Scene // nil when none is up
	since time.Time    // when on was pushed
	queue []queued
	// drawn is the last screen View drew, for a biome drop to slide away
	// from.
	drawn string
}

// queued is a scene still to show. It is built when its turn comes, so it
// starts from the screen as it is then; id is its seen id, "" for an
// every-time scene.
type queued struct {
	id   string
	make func() *story.Scene
}

// crashSceneMsg asks for the Cat's beat on boot's crash, once the panic
// has printed.
type crashSceneMsg struct{ boot int }

// storyOn reports whether scenes play in this boot: the game turns them on
// (Deps.Story).
func (m *Model) storyOn() bool { return m.d.Story && m.state != nil }

// realBoot reports whether scenes play and the boot is a boot, not a
// trial.
func (m *Model) realBoot() bool { return m.storyOn() && m.state.Trial == nil }

// storyBegin queues the scenes for a boot that just started.
func (m *Model) storyBegin() tea.Cmd {
	m.syncStory()
	if !m.realBoot() {
		return nil
	}
	s := m.state
	boot, p := s.Boot, s.Prestige
	q := []queued{{"", func() *story.Scene { return story.NewBootSplash(boot, p) }}}
	prologue := m.unseen("story.prologue")
	if prologue {
		q = append(q, queued{"story.prologue", story.NewPrologue})
	}
	if s.Phase == game.PhaseDoors {
		// The prologue's own fall lands in the hall of doors: it stands in
		// for the first biome's drop.
		q = append(q, m.arrival(prologue)...)
	}
	return m.playStory(q...)
}

// storyAfter queues the scenes for a step that changed the base screen
// from before's.
func (m *Model) storyAfter(before baseKey) tea.Cmd {
	m.syncStory()
	if !m.storyOn() {
		return nil
	}
	s := m.state
	if s.Phase == game.PhaseCrashed {
		return m.storyCrash()
	}
	var q []queued
	if before.phase == game.PhaseProblem && s.PendingUpgrade && len(s.History) > 0 {
		if a := s.History[len(s.History)-1]; a.Outcome == game.Cleared {
			line := acLine(a)
			q = append(q, queued{"", func() *story.Scene { return story.NewAccepted(line) }})
		}
	}
	if m.realBoot() {
		q = append(q, m.mapScenes()...) // acts II and III, the skip, a lock (hub.go)
		switch {
		case s.Phase == game.PhaseMerge && before.phase != game.PhaseMerge:
			if m.unseen("story.first-merge") {
				q = append(q, queued{"story.first-merge", story.NewFirstMerge})
			}
		case s.Phase == game.PhaseDoors && s.Room == 0 &&
			(before.phase == game.PhaseRoute || before.phase == game.PhaseMerge):
			q = append(q, m.arrival(false)...)
		}
	}
	return m.playStory(q...)
}

// storyResume queues, for a boot resumed between doors, the one-time
// scenes of where it stands that were never on screen: a quit cut them
// short.
func (m *Model) storyResume() tea.Cmd {
	m.syncStory()
	if !m.realBoot() {
		return nil
	}
	s := m.state
	q := m.mapScenes() // acts II and III, the skip, a lock (hub.go)
	switch {
	case s.Phase == game.PhaseCrashed:
		return m.storyCrash()
	case s.Phase == game.PhaseMerge && m.unseen("story.first-merge"):
		q = append(q, queued{"story.first-merge", story.NewFirstMerge})
	case s.Phase == game.PhaseDoors && s.Room == 0:
		if s.Biome == 0 && m.unseen("story.prologue") {
			q = append(q, queued{"story.prologue", story.NewPrologue})
		}
		for _, a := range m.arrival(true) {
			if a.id != "" {
				q = append(q, a)
			}
		}
	}
	return m.playStory(q...)
}

// arrival is the scenes at the doors of a biome's first room: a boss
// biome's card, any other biome's drop unless noDrop, and in the first
// biome the first door's inscription, once.
func (m *Model) arrival(noDrop bool) []queued {
	s := m.state
	b := s.Biomes[s.Biome]
	var q []queued
	switch {
	case b.Kind == rules.Boss && b.Boss != "":
		id := "story.first-boss"
		if b.Boss == game.FinalBoss {
			id = "reveal.final-boss"
		}
		first, key, seed := m.unseen(id), b.Boss, s.Seed
		if !first {
			id = ""
		}
		q = append(q, queued{id, func() *story.Scene { return story.NewBossIntro(key, seed, first) }})
	case !noDrop && b.Topic != "":
		q = append(q, queued{"", func() *story.Scene { return m.biomeFall(b) }}) // a map node's own line (hub.go)
	}
	if s.Biome == 0 && m.unseen("door.first.inscription") {
		q = append(q, queued{"door.first.inscription", story.NewFirstDoor})
	}
	return q
}

// storyCrash schedules the Cat's beat (story.first-crash) when it is the
// beat this crash earned. It plays by itself once the panic has printed
// (S9 part 2 default 2); the other beats are the end screen's teaser.
func (m *Model) storyCrash() tea.Cmd {
	if !m.realBoot() || !m.catDue() {
		return nil
	}
	boot := m.state.Boot
	return tea.Tick(crashLead, func(time.Time) tea.Msg { return crashSceneMsg{boot} })
}

// catDue reports whether the first crash's beat is this boot end's beat.
func (m *Model) catDue() bool {
	id, ok := flavor.NextBeat(m.state, &m.meta, flavor.DefaultBeatGap)
	return ok && id == "story.first-crash"
}

// onCrashScene shows the Cat if the crash screen is still alone on top,
// not once the player moved on (an upsolve, a log, a new boot). Then the
// beat stays due, for the next crash that leaves bytes.
func (m *Model) onCrashScene(c crashSceneMsg) tea.Cmd {
	m.syncStory()
	s := m.state
	if !m.realBoot() || s.Boot != c.boot || s.Phase != game.PhaseCrashed || s.Crash == nil ||
		len(m.stack) != 1 || m.open != nil || !m.catDue() {
		return nil
	}
	bytes, title := units.Bytes(s.Crash.Lost), m.title(s.Crash.Door.Problem)
	return m.playStory(queued{"story.first-crash", func() *story.Scene { return story.NewFirstCrash(bytes, title) }})
}

// unseen reports whether the one-time scene id has never been on screen.
func (m *Model) unseen(id string) bool { return !game.HasSeen(&m.meta, id) }

// syncStory forgets the scene and its queue once a rebuilt stack dropped
// it: what was queued belonged to a moment that is over.
func (m *Model) syncStory() {
	if m.story.on != nil && !slices.Contains(m.stack, screens.Screen(m.story.on)) {
		m.story.on, m.story.queue = nil, nil
	}
}

// playStory replaces the queue with q and shows its first scene.
func (m *Model) playStory(q ...queued) tea.Cmd {
	if len(q) == 0 {
		return nil
	}
	m.story.queue = q
	return m.nextScene()
}

// nextScene pushes the queue's next scene, if any.
func (m *Model) nextScene() tea.Cmd {
	if len(m.story.queue) == 0 {
		return nil
	}
	q := m.story.queue[0]
	m.story.queue = m.story.queue[1:]
	s := q.make()
	m.story.on, m.story.since = s, m.d.Now()
	return m.push(s)
}

// sceneDone takes a scene's Done. The scene was on screen, so a one-time
// one is seen. When it is the app's scene on top it pops and the next
// queued scene shows, none after an esc; ok is false otherwise, so the
// Done goes on to the screens, for one that shows a scene itself. A Done
// from a scene a rebuilt stack dropped goes nowhere.
func (m *Model) sceneDone(d story.Done) (cmd tea.Cmd, ok bool) {
	m.markSeen(d.Scene)
	on := m.story.on
	if on == nil || !on.Left() || m.top() != screens.Screen(on) {
		m.syncStory()
		return nil, false
	}
	m.pop()
	m.story.on = nil
	if d.Skipped {
		m.story.queue = nil
	}
	return m.nextScene(), true
}

// readBeat shows a one-time beat's page (story.Read): from a boot end's
// teaser, or reread from the log. Never over an open door, and never for
// an id that is no beat.
func (m *Model) readBeat(id string) tea.Cmd {
	m.syncStory()
	if flavor.BeatTitle(id) == "" || m.open != nil || m.state != nil && m.state.Phase == game.PhaseProblem {
		return nil
	}
	return m.playStory(queued{id, func() *story.Scene { return story.NewBeat(id) }})
}

// markSeen records a one-time scene, saves it, and notes it (RT6).
func (m *Model) markSeen(id string) {
	if !game.MarkSeen(&m.meta, id) {
		return
	}
	var dwell time.Duration
	if m.story.on != nil && m.story.on.ID() == id {
		dwell = m.d.Now().Sub(m.story.since)
	}
	m.event(storyBeat, "", kv{"id": id, "dwell_ms": dwell.Milliseconds()})
	if m.state != nil {
		m.saved(m.d.Store.Checkpoint(m.meta, m.state))
	} else {
		m.saved(m.d.Store.SaveMeta(m.meta))
	}
}

// drawnLines is the screen View drew last, as rows.
func (m *Model) drawnLines() []string {
	if m.story.drawn == "" {
		return nil
	}
	return strings.Split(m.story.drawn, "\n")
}

// acLine is the verdict in figures under ACCEPTED: "AC · 10/10 tests ·
// 08:41 / 15:00", the time on the door against its timer.
func acLine(a game.Attempt) string {
	parts := []string{string(game.Accepted)}
	if a.Total > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d tests", a.BestPass, a.Total))
	}
	if a.Door.Timer > 0 && !a.Closed.IsZero() {
		parts = append(parts, units.Clock(a.Closed.Sub(a.Opened))+" / "+units.Clock(a.Door.Timer))
	}
	return strings.Join(parts, ui.G.Sep)
}
