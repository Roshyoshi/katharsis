package app

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/audio"
	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
)

// The app turns state into sound. Music follows a cue derived from where
// the player is (GRUB, a biome, the merge window, a boss, the kernel
// panic). Effects come from the engine's Sound effects (verdicts, crash,
// commit, boss, clear) plus what can be read off the state as it changes:
// a door opening, a submit starting, a life lost, the last minute, a
// purchase, a reroll, a pick, a quack, a prestige unlocked. The engine
// stays free of presentation, and nothing here needs it to change.

// musicKey toggles the music on any screen. It is a control key so no
// text field (the GRUB editor, the reader's search, git log's filter)
// ever takes it as a letter.
const musicKey = "ctrl+t"

// audioSnap is what the effects are diffed from.
type audioSnap struct {
	ok                     bool
	boot                   int
	phase                  game.Phase
	biome, room            int
	door                   catalog.ID // the open door (a boot.s or an upsolve)
	opened                 time.Time  // ... and when it opened
	lives, quacks          int
	uncommitted, committed int64
	picks                  int    // upgrades + daemons taken
	offered                string // the doors on offer, to see a reroll
	judging                bool
	prestige               int
}

func (m *Model) audioSnap() audioSnap {
	a := audioSnap{ok: true, judging: m.judging, prestige: m.meta.Prestige}
	if o := m.open; o != nil {
		a.door, a.opened = o.Door.Problem, o.Opened
	}
	s := m.state
	if s == nil {
		return a
	}
	a.boot, a.phase, a.biome, a.room = s.Boot, s.Phase, s.Biome, s.Room
	a.lives, a.uncommitted, a.committed = s.Cycles, s.Uncommitted, s.Committed
	a.picks = len(s.Upgrades) + len(s.Daemons)
	if c := s.Current; c != nil {
		a.quacks = c.Quacks
	}
	ids := make([]string, len(s.Doors))
	for i, d := range s.Doors {
		ids[i] = string(d.Problem)
	}
	a.offered = strings.Join(ids, ",")
	return a
}

// syncAudio runs after every Update: the cue, the duck, the effects the
// change implies, and the toast for a track that just started.
func (m *Model) syncAudio() {
	a := m.d.Audio
	a.Cue(m.audioCue())
	a.Duck(m.judging)
	now := m.audioSnap()
	prev := m.audioPrev
	m.audioPrev = now
	if prev.ok && prev.boot == now.boot && now.boot != 0 {
		for _, sfx := range audioEffects(prev, now) {
			a.Play(sfx)
		}
	}
	if prev.ok && now.door != "" && (now.door != prev.door || !now.opened.Equal(prev.opened)) {
		a.Play("door")
	}
	if prev.ok && now.prestige > prev.prestige {
		a.Play("prestige")
	}
	m.warnTimer()
	if t, serial := a.NowPlaying(); serial != m.toastSerial {
		m.toastSerial = serial
		if serial != 0 {
			m.showToast(t.Credit())
		}
	}
}

// audioEffects are the effects implied by one step within a boot.
func audioEffects(p, n audioSnap) []string {
	var fx []string
	if n.judging && !p.judging {
		fx = append(fx, "submit")
	}
	if n.lives < p.lives && n.phase != game.PhaseCrashed {
		fx = append(fx, "life") // queued after the verdict's jingle
	}
	if n.quacks > p.quacks {
		fx = append(fx, "quack")
	}
	if n.picks > p.picks {
		fx = append(fx, "pick")
	}
	// Bytes spent without being banked: a purchase at the merge window.
	if n.phase == game.PhaseMerge && p.phase == game.PhaseMerge &&
		n.uncommitted < p.uncommitted && n.committed == p.committed {
		fx = append(fx, "buy")
	}
	// The same room offering other doors: a reroll.
	if n.phase == game.PhaseDoors && p.phase == game.PhaseDoors && n.biome == p.biome &&
		n.room == p.room && n.offered != p.offered && p.offered != "" {
		fx = append(fx, "reroll")
	}
	return fx
}

// warnTimer sounds once per door when its timer passes WarnAt left. A
// trial's soft timer only goes overtime, so it gets no warning.
func (m *Model) warnTimer() {
	c, at := m.attempt(), m.d.AudioSettings.WarnAt.Duration
	if at <= 0 || c == nil || m.state.SoftTimer() {
		return
	}
	left := c.Deadline.Sub(m.d.Now())
	if left > at || left <= 0 || m.warned.Equal(c.Opened) {
		return
	}
	m.warned = c.Opened
	// A door resumed with less than a minute already left still warns:
	// the player did not hear it before.
	m.d.Audio.Play("warn")
}

// audioCue is the music the current state calls for. Inside a boot a
// biome's track carries from its doors to the screens between them, and
// the map between stages (the merge window, the route) has its own.
// Outside one, home plays (home).
func (m *Model) audioCue() audio.Cue {
	s := m.state
	if m.home() {
		return audio.Cue{Pool: audio.PoolGrub}
	}
	scene := s.Boot*100 + s.Biome
	switch {
	case s.Phase == game.PhaseCrashed:
		return audio.Cue{Pool: audio.PoolPanic, Scene: scene}
	case s.Phase == game.PhaseCleared:
		return audio.Cue{Pool: audio.PoolCleared, Scene: scene}
	case s.Trial != nil:
		return audio.Cue{Pool: audio.PoolTrial, Scene: s.Boot * 100}
	case s.Phase == game.PhaseMerge || s.Phase == game.PhaseRoute:
		return audio.Cue{Pool: audio.PoolMerge, Scene: scene}
	}
	if s.Biome < 0 || s.Biome >= len(s.Biomes) {
		return audio.Cue{Pool: audio.PoolGrub}
	}
	b := s.Biomes[s.Biome]
	act := s.ActOf(s.Biome)
	switch {
	case b.Kind == rules.Boss && b.Boss == game.FinalBoss:
		return audio.Cue{Pool: audio.PoolFinal, Key: b.Boss, Scene: scene}
	case b.Kind == rules.Boss:
		return audio.Cue{Pool: audio.BossPools[min(act, len(audio.BossPools)-1)], Key: b.Boss, Scene: scene}
	case s.Biome == 0:
		return audio.Cue{Pool: audio.PoolUserland, Key: b.Topic, Scene: scene}
	}
	return audio.Cue{Pool: audio.ActPools[min(act, len(audio.ActPools)-1)], Key: b.Topic, Scene: scene}
}

// home reports whether the player is outside a boot, where the calm home
// pool plays: no boot in flight, an upsolve open, or a boot left for GRUB,
// RTFM or the screens they open (git log, top, apt, settings, setuid).
func (m *Model) home() bool {
	if m.state == nil || m.open != nil && m.open.Upsolve {
		return true
	}
	return slices.ContainsFunc(m.stack, func(sc screens.Screen) bool {
		switch sc.(type) {
		case *meta.Menu, *meta.Log, *meta.Top, *meta.Shop, *meta.Settings, *meta.Setuid, *rtfm.Map, *rtfm.Browser:
			return true
		}
		return false
	})
}

// toggleMusic is the music key: flip it and say so.
func (m *Model) toggleMusic() tea.Cmd {
	if m.d.Audio.ToggleMusic() {
		m.showToast("♪ music on (" + musicKey + " turns it off)")
	} else {
		m.showToast("♪ music off (" + musicKey + " turns it on)")
	}
	return nil
}

func (m *Model) showToast(text string) {
	d := m.d.AudioSettings.Toast.Duration
	if d <= 0 {
		return
	}
	m.toast, m.toastUntil = text, m.d.Now().Add(d)
}

// toastLine is the toast while it lasts.
func (m *Model) toastLine() string {
	if m.toast != "" && m.d.Now().Before(m.toastUntil) {
		return m.toast
	}
	return ""
}

// openCredits shows the credits page (music, licenses, the Touhou notice)
// in the reader.
func (m *Model) openCredits() tea.Cmd {
	var md string
	if m.d.Credits != nil {
		md = m.d.Credits()
	} else {
		md = audio.Credits(nil, m.d.AudioSettings)
	}
	return m.push(reader.New(reader.Doc{Title: "Credits", Source: "katharsis", Markdown: md}))
}
