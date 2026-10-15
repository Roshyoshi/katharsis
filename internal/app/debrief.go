package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/story"
	"katharsis/internal/units"
)

// The end of a boot, app side (retention RT1, RT3, RT4, RT6, RT7, RT12;
// UX J12): the figures the end screens show from the store and the
// settings, the intents they emit, and the story's words on quit and
// resume (D31).

// Telemetry kinds this file adds (retention RT13): what the player did
// at the end of a boot, a story beat read, the budget's nudge shown.
const (
	eventBootEndAction = "boot_end_action" // json action: the intent the end screen emitted
	eventStoryRead     = "story_read"      // json beat: the beat id, "" for the reread log
	eventNudgeShown    = "nudge_shown"     // json today_ms, budget_ms
)

// goalPace and etaBoots are how many recent boots the ETAs read: apt's
// goal paces by the last ten boots' commits (RT3), a preset's ETA by its
// last twenty uptimes (RT7).
const (
	goalPace = 10
	etaBoots = 20
)

// offered are the presets the end of a boot offers beside its own, where
// "one more" is decided (RT7), each on its key, when the game has them.
var offered = []struct{ key, preset string }{{"s", "short"}, {"p", "patch"}}

// endCache is the last End and what it was computed for: the boot screens'
// Env is built on every tick, and the End reads the store.
type endCache struct {
	key    endKey
	end    bootui.End
	set    bool
	nudged int // the boot whose end showed the budget's nudge
}

type endKey struct {
	boot   int
	phase  game.Phase
	bank   int64
	solves int
}

// endInfo is the boot screens' Env.End, recomputed when the boot, its
// phase, the bank or the solves move.
func (m *Model) endInfo() bootui.End {
	st := m.state
	if st == nil {
		return bootui.End{}
	}
	k := endKey{st.Boot, st.Phase, m.meta.Bank, len(m.meta.Solves)}
	if m.end.set && m.end.key == k {
		return m.end.end
	}
	e := m.computeEnd(st)
	m.end.key, m.end.end, m.end.set = k, e, true
	return e
}

// computeEnd reads the store for End. A query that fails leaves its
// figure out: the screen then shows what it knows (GR3: no guessed ETA).
func (m *Model) computeEnd(st *game.State) bootui.End {
	var e bootui.End
	if st.Trial == nil {
		if p, _, ok := meta.Goal(m.packages(), m.meta.Bank); ok {
			per, err := m.d.Store.MedianCommitted(goalPace)
			m.logErr("goal pace", err)
			e.Goal = &bootui.Goal{Name: p.Name, Price: p.Price, Per: per}
		}
	}
	if st.Phase != game.PhaseCrashed && st.Phase != game.PhaseCleared {
		return e
	}
	var err error
	e.ETA, err = m.d.Store.MedianUptime(st.Rules.Name, etaBoots)
	m.logErr("eta", err)
	for _, o := range offered {
		if _, ok := m.d.Presets[o.preset]; !ok || o.preset == st.Rules.Name {
			continue
		}
		eta, err := m.d.Store.MedianUptime(o.preset, etaBoots)
		m.logErr("eta", err)
		e.Reboots = append(e.Reboots, bootui.Reboot{Key: o.key, Preset: o.preset, ETA: eta})
	}
	if e.Budget = m.d.Budget; e.Budget > 0 {
		e.Today, err = m.d.Store.PlayToday(m.d.Now())
		m.logErr("play today", err)
		if e.Today >= e.Budget && m.end.nudged != st.Boot {
			m.end.nudged = st.Boot
			m.event(eventNudgeShown, "", kv{"today_ms": e.Today.Milliseconds(), "budget_ms": e.Budget.Milliseconds()})
		}
	}
	if st.Phase == game.PhaseCleared && st.Trial == nil {
		e.Firsts = m.firsts(st)
	}
	return e
}

// firsts are a clear's first-time and personal-best lines (RT1), at most
// two: the boss's first kill, the first clear at this prestige on this
// preset, or its fastest.
func (m *Model) firsts(st *game.State) []string {
	var out []string
	var boss string
	var end time.Time
	for _, a := range st.History {
		if a.Outcome == game.Cleared && a.Door.Boss != "" {
			boss = a.Door.Boss
		}
		end = a.Closed
	}
	if boss != "" {
		if before, err := m.d.Store.BossClearedBefore(boss, st.Boot); err == nil && !before {
			out = append(out, "first "+flavor.BossName(boss)+" kill")
		}
	}
	up := end.Sub(st.Started)
	best, ok, err := m.d.Store.BestClear(st.Rules.Name, st.Prestige, st.Boot)
	switch {
	case err != nil:
	case !ok:
		out = append(out, "first clear at "+flavor.PrestigeName(st.Prestige))
	case up < best:
		out = append(out, fmt.Sprintf("fastest clear: %s (was %s)", units.Clock(up), units.Clock(best)))
	}
	return out[:min(len(out), 2)]
}

func (m *Model) logErr(what string, err error) {
	if err != nil {
		m.logf("%s: %v", what, err)
	}
}

// debriefIntent handles the end screens' intents and the story's; ok is
// false for anything else. An intent the end of a boot emits is logged
// on its way (RT13's boot_end_action), whoever handles it.
func (m *Model) debriefIntent(msg tea.Msg) (tea.Cmd, bool) {
	m.endAction(msg)
	switch msg := msg.(type) {
	case screens.OpenGrub:
		return m.toGrub(), true
	case screens.ReadStory:
		return m.readStory(msg.Beat), true
	case story.Done:
		return m.storyDone(msg), true
	}
	return nil, false
}

// endAction notes what the player chose at the end of a boot.
func (m *Model) endAction(msg tea.Msg) {
	if m.state == nil || m.inBoot() || len(m.stack) != 1 {
		return
	}
	var action string
	switch msg := msg.(type) {
	case screens.StartBoot:
		action = "reboot:" + msg.Preset
	case screens.Upsolve:
		action = "upsolve"
	case screens.OpenEditorial:
		action = "editorial"
	case screens.OpenLog:
		action = "git log"
	case screens.OpenGrub:
		action = "grub"
	case screens.ReadStory:
		action = "story"
	default:
		return
	}
	m.event(eventBootEndAction, "", kv{"action": action})
}

// toGrub leaves a boot that has ended for GRUB (UX J12): the boot stays in
// git log, and q at GRUB is poweroff.
func (m *Model) toGrub() tea.Cmd {
	if m.inBoot() {
		return fail("finish the boot first: GRUB is between boots")
	}
	m.state = nil
	m.end = endCache{}
	m.reset()
	return m.initTop()
}

// readStory shows a beat (story.NewBeat), or with id "" the reread log
// (RT6's /proc/story).
func (m *Model) readStory(id string) tea.Cmd {
	m.event(eventStoryRead, "", kv{"beat": id})
	if id == "" {
		return m.push(reader.New(storyLog(&m.meta)))
	}
	return m.push(story.NewBeat(id))
}

// storyDone records a scene that finished (story.Done) as seen, saves it,
// and leaves the scene. A beat read at the end of a boot is seen from
// then on: the end screen stops teasing it, and the reread log keeps it.
func (m *Model) storyDone(d story.Done) tea.Cmd {
	if game.MarkSeen(&m.meta, d.Scene) {
		if m.inBoot() {
			m.saved(m.d.Store.Checkpoint(m.meta, m.state))
		} else {
			m.saved(m.d.Store.SaveMeta(m.meta))
		}
	}
	if _, ok := m.top().(*story.Scene); ok {
		m.pop()
	}
	m.refresh()
	return nil
}

// storyLog is /proc/story: every beat the player has seen, in the order
// shown, as one page (flavor.StoryLog). Nothing the story says is
// missable.
func storyLog(m *game.Meta) reader.Doc {
	var b strings.Builder
	b.WriteString("# /proc/story\n\n")
	log := flavor.StoryLog(m)
	if len(log) == 0 {
		b.WriteString("Nothing yet. The story starts with your first boot.\n")
	}
	for _, e := range log {
		when := fmt.Sprintf("boot %d", e.Boot)
		if e.Boot == 0 {
			when = "before boot 1"
		}
		fmt.Fprintf(&b, "## %s · %s\n\n", e.Title, when)
		for i, l := range e.Lines {
			// Markdown's hard break keeps the story's own lines.
			if i < len(e.Lines)-1 {
				l += "  "
			}
			b.WriteString(l + "\n")
		}
		b.WriteString("\n")
	}
	return reader.Doc{Title: "/proc/story", Source: "the reread log", Markdown: b.String()}
}

// quitWarn is D31's warning on quitting with a door open, in the story's
// words (quit.warn) while the door has time left. A soft timer past its
// deadline keeps the plain words: the Rabbit's line counts time left.
func quitWarn(left time.Duration) string {
	if s := flavor.Story("quit.warn", 0); s != "" && left > 0 {
		return flavor.Fill(s, "time", units.Countdown(left)) + " · ctrl+c again to quit"
	}
	return "the door's timer keeps running while katharsis is closed (" + leftText(left) + ") · ctrl+c again to quit"
}

// resumeNote is D31's note on resuming a door, in the story's words
// (resume.note) while it has time left.
func resumeNote(left time.Duration) string {
	if s := flavor.Story("resume.note", 0); s != "" && left > 0 {
		return flavor.Fill(s, "time", units.Countdown(left))
	}
	return "the door's timer kept running while katharsis was closed: " + leftText(left)
}

// recoveredNote is the flash after an upsolve banks a core dump
// (coredump.recovered).
func recoveredNote(got int64) string {
	if s := flavor.Story("coredump.recovered", 0); s != "" {
		return flavor.Fill(s, "bytes", units.Bytes(got))
	}
	return "core dump recovered: +" + units.Bytes(got) + " banked"
}
