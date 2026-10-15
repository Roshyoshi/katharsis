package app

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/story"
	"katharsis/internal/store"
	"katharsis/internal/units"
)

// The end of a boot, app side (retention RT1, RT3, RT4, RT6, RT7, RT12;
// UX J12): the figures the end screens show from the store and the
// settings, the intents they emit, and the story's words on resuming a
// door (D31).

// Telemetry kinds this file adds (retention RT13): what the player did
// at the end of a boot and the budget's nudge shown (story.go logs the
// beats read).
const (
	eventBootEndAction = "boot_end_action" // json action: the intent the end screen emitted
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
	switch {
	case st.Trial != nil:
		if st.Phase == game.PhaseCleared {
			e.Learn = m.trialNext(st) // learn.go
		}
	case st.Phase == game.PhaseCleared:
		e.Firsts = m.firsts(st)
	default:
		e.Learn = m.learnNext(st)
	}
	return e
}

// learnAfter crashes on one topic within learnWindow make RTFM the next
// step at the end of a boot (retention RT1); learnBoots is how many recent
// boots learnNext reads for them.
const (
	learnAfter  = 2
	learnWindow = 14 * 24 * time.Hour
	learnBoots  = 50
)

// learnNext is End.Learn for a crashed boot: its door's topic has a
// capability the player lacks, and learnAfter boots crashed on it within
// learnWindow, this one too. A live core dump is the next step before it
// (the upsolve), so then there is none. A query that fails counts nothing.
func (m *Model) learnNext(st *game.State) *bootui.Learn {
	topic := st.Crash.Door.Topic
	if game.Debrief(st, &m.meta).Next.Kind != game.NextReboot || game.Unlocked(&m.meta, topic) {
		return nil
	}
	boots, err := m.d.Store.Boots(learnBoots)
	m.logErr("crashes", err)
	n, since := 1, st.Crash.At.Add(-learnWindow)
	for _, b := range boots {
		if b.Boot == st.Boot || b.Outcome != store.BootCrashed || b.Rules == rules.Trial().Name {
			continue
		}
		// The boot's last attempt is the door it crashed on.
		atts, err := m.d.Store.Attempts(store.AttemptFilter{Boot: b.Boot, Limit: 1})
		m.logErr("crashes", err)
		if len(atts) > 0 && atts[0].Door.Topic == topic && !atts[0].Closed.Before(since) {
			n++
		}
	}
	if n < learnAfter {
		return nil
	}
	return &bootui.Learn{Topic: topic, Crashes: n}
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
	up := st.Uptime(end)
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

// debriefIntent handles the end screens' own intent, OpenGrub; ok is
// false for anything else. An intent the end of a boot emits is logged
// on its way (RT13's boot_end_action), whoever handles it: a beat's page
// (story.Read) is story.go's.
func (m *Model) debriefIntent(msg tea.Msg) (tea.Cmd, bool) {
	m.endAction(msg)
	if _, ok := msg.(screens.OpenGrub); ok {
		return m.toGrub(), true
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
	case story.Read:
		action = "story"
	case screens.OpenLearn:
		action = "rtfm"
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

// resumeNote is D31's note on resuming a door, in the story's words
// (resume.note) while it has time left.
func resumeNote(left time.Duration) string {
	if s := flavor.Story("resume.note", 0); s != "" && left > 0 {
		return flavor.Fill(s, "time", units.Countdown(left))
	}
	return "the door's timer kept running while katharsis was closed: " + leftText(left)
}

// upsolveOf records the open upsolve of id as solved (game.Upsolve) and
// reports what it banked and whether it fixed a regression that was due:
// one solved again without the editorial, as the schedule counts it.
func (m *Model) upsolveOf(id catalog.ID, kind game.SolveKind) (got int64, fixed bool) {
	now := m.d.Now()
	fixed = kind != game.Editorial && slices.ContainsFunc(game.DueRegressions(&m.meta, now),
		func(g game.Regression) bool { return g.Problem == id })
	return game.Upsolve(&m.meta, id, kind, now.Sub(m.open.Opened), now), fixed
}

// upsolveNote is the flash after an accepted upsolve: the core dump it
// banked (coredump.recovered), else the regression it fixed
// (regression.cleared), else nothing.
func upsolveNote(got int64, fixed bool) tea.Cmd {
	switch {
	case got > 0:
		return note("%s", recoveredNote(got))
	case fixed:
		if s := flavor.Story("regression.cleared", 0); s != "" {
			return note("%s", s)
		}
	}
	return nil
}

// recoveredNote is the flash after an upsolve banks a core dump
// (coredump.recovered).
func recoveredNote(got int64) string {
	if s := flavor.Story("coredump.recovered", 0); s != "" {
		return flavor.Fill(s, "bytes", units.Bytes(got))
	}
	return "core dump recovered: +" + units.Bytes(got) + " banked"
}

// grubEnd adds to GRUB what the store says between boots: how today's
// nightly has been played (RT15) and today's play against the session
// budget (RT12). A query that fails leaves its line out.
func (m *Model) grubEnd(c meta.GrubConfig) meta.GrubConfig {
	now := m.d.Now()
	if r, ok, err := m.d.Store.SeedBest(meta.NightlySeed(now)); err != nil {
		m.logErr("nightly best", err)
	} else if ok {
		c.Nightly = &meta.NightlyBest{Played: r.Boots, Cleared: r.Cleared, Uptime: r.Uptime, Doors: r.Doors}
	}
	if c.Budget = m.d.Budget; c.Budget > 0 {
		var err error
		c.Today, err = m.d.Store.PlayToday(now)
		m.logErr("play today", err)
	}
	return c
}
