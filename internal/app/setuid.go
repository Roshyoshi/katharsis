package app

import (
	"cmp"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/meta"
	"katharsis/internal/setuid"
	"katharsis/internal/store"
	"katharsis/internal/ui"
)

// The setuid bits, app side (docs/design/achievements.md (g)-(i)): the
// checks at safe points, where each bit is said, the find / -perm -4000
// screen off GRUB, and the backfill of a save from before them.
//
//	tryAt: game.Step ─┬─ a door open ──────── nothing (GR6)
//	                  └─ else setuid.Check ── the bits land in the step's
//	                     │                    checkpoint, then are said:
//	                     ├─ crashed ───────── nowhere now; GRUB: setuid +n (GR3)
//	                     ├─ cleared ───────── the kept block (bootui.setuidKept)
//	                     └─ route, doors, merge the strip, after this look
//	outside a boot: an upsolve, an install ─ Check(nil), in the flash
//	launch: !Record.Backfilled ───────────── the backfill, saved, then GRUB
//
// setuid_notices off says none of it; the screen still lists every bit.

// setuidState is the session's side of the bits.
type setuidState struct {
	// notices is setuid_notices for this session: the find screen's n
	// turns it off or on until the game quits.
	notices bool
	// strip is the strip's line for bits a step set, added after the
	// look that step's own lines come from (syncDmesg), so it follows them.
	strip []bootui.Event
	// news counts bits set where nothing said so, a crash's and the
	// backfill's: GRUB's status line says "setuid +n" until a boot or the
	// find screen leaves it.
	news int
}

// checkSetuid sets the bits that hold for Meta and the boot (nil outside
// one) at now: a safe point, never while a door is open. Deps.SetuidCheck
// stands in for setuid.Check in tests.
func (m *Model) checkSetuid(s *game.State, now time.Time) []game.SetuidBit {
	if s != nil && s.Phase == game.PhaseProblem {
		return nil
	}
	check := m.d.SetuidCheck
	if check == nil {
		check = setuid.Check
	}
	return check(&m.meta, s, now)
}

// setuidSet records each bit set (setuid_set), noticed in phase ("" outside
// a boot).
func (m *Model) setuidSet(bits []game.SetuidBit, phase game.Phase) {
	for _, b := range bits {
		d, _ := setuid.Lookup(b.ID)
		m.event(store.EventSetuidSet, "", store.SetuidSet{ID: b.ID, Kind: string(d.Kind), Backfill: b.Backfill, Phase: string(phase)})
	}
}

// noticeSetuid says the bits a step of the boot set, where its phase
// lets it (the file comment's tree).
func (m *Model) noticeSetuid(bits []game.SetuidBit, now time.Time) {
	s := m.state
	m.setuidSet(bits, s.Phase)
	if len(bits) == 0 || !m.setuid.notices {
		return
	}
	switch s.Phase {
	case game.PhaseCrashed:
		m.setuid.news += len(bits)
	case game.PhaseCleared:
	default:
		m.setuid.strip = append(m.setuid.strip, bootui.SetuidEvent(bits, s.Seed^uint64(now.Unix()), now))
	}
}

// setuidOutside checks the bits outside a boot, after an upsolve or an
// install changed Meta, and is what the flash adds for them: " · chmod
// u+s /bin/grin-returns", "" for none. The caller saves.
func (m *Model) setuidOutside() string {
	bits := m.checkSetuid(nil, m.d.Now())
	m.setuidSet(bits, "")
	if len(bits) == 0 || !m.setuid.notices {
		return ""
	}
	return ui.G.Sep + bootui.Chmod(bits)
}

// backfillSetuid sets, once, the bits a save from before them earned
// (store.BackfillSetuid, marked "from git log"), and saves. GRUB's status
// line counts them. A backfill that fails leaves the save as it was, to
// try again at the next launch. Deps.SetuidBackfill stands in for the
// store's in tests.
func (m *Model) backfillSetuid() {
	if m.meta.Record.Backfilled {
		return
	}
	fill := m.d.SetuidBackfill
	if fill == nil {
		fill = m.d.Store.BackfillSetuid
	}
	bits, err := fill(&m.meta, m.d.Now())
	if err != nil {
		m.logErr("setuid backfill", err)
		return
	}
	if m.state != nil {
		m.saved(m.d.Store.Checkpoint(m.meta, m.state))
	} else {
		m.saved(m.d.Store.SaveMeta(m.meta))
	}
	m.setuidSet(bits, "")
	if m.setuid.notices {
		m.setuid.news += len(bits)
	}
}

// samplesRun counts a :w on the boot's open door (game.SamplesRun): the
// HUD's sample pass ended. Heisenbug's bit asks for none; an upsolve is
// no boot's door. A deadline already past is the timeout's to step.
func (m *Model) samplesRun() tea.Cmd {
	if !m.inBootDoor() {
		return nil
	}
	cmd, _ := m.try(game.SamplesRun{})
	return cmd
}

// setuidIntent handles the find screen's intents: opening it, a hint
// shown, the notices turned off or on.
func (m *Model) setuidIntent(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case screens.OpenSetuid:
		return m.openSetuid()
	case meta.SetuidHint:
		m.event(store.EventSetuidHint, "", store.SetuidHint{ID: msg.ID})
	case meta.SetuidNotices:
		m.setuid.notices = msg.On
	}
	return nil
}

// openSetuid shows find / -perm -4000 and records that it opened, with
// the spoilers it opened under and how many bits are set. GRUB's news is
// read now.
func (m *Model) openSetuid() tea.Cmd {
	spoilers := cmp.Or(m.d.Spoilers, meta.SpoilersNone)
	set := 0
	for _, d := range setuid.Table {
		if setuid.IsSet(&m.meta, d.ID) {
			set++
		}
	}
	m.event(store.EventSetuidOpen, "", store.SetuidOpen{Spoilers: spoilers, Set: set, Of: len(setuid.Table)})
	if m.setuid.news > 0 {
		m.setuid.news = 0
		m.regrub() // its status line said it
	}
	return m.push(meta.NewSetuid(meta.SetuidConfig{
		Meta: m.meta, Now: m.d.Now(), Spoilers: spoilers, Notices: m.setuid.notices,
	}))
}
