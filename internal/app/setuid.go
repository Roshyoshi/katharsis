package app

import (
	"cmp"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/curriculum"
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
//	a trial passed ──────────────────────── CheckRTFM too, with RTFM's map
//	launch: !Record.Backfilled ───────────── the backfill, saved, then GRUB
//	launch, RTFM loaded ──────────────────── backfillRTFM, saved
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
	// passed is len(Meta.TrialsPassed) when the RTFM bits were last checked
	// (checkSetuid, backfillRTFM): a trial passed since checks them again.
	passed int
}

// checkSetuid sets the bits that hold for Meta and the boot (nil outside
// one) at now: a safe point, never while a door is open. A trial passed
// since the last check also checks the bits that read RTFM's map
// (setuid.CheckRTFM). Deps.SetuidCheck stands in for both in tests.
func (m *Model) checkSetuid(s *game.State, now time.Time) []game.SetuidBit {
	if s != nil && s.Phase == game.PhaseProblem {
		return nil
	}
	if m.d.SetuidCheck != nil {
		return m.d.SetuidCheck(&m.meta, s, now)
	}
	bits := setuid.Check(&m.meta, s, now)
	if n := len(m.meta.TrialsPassed); n != m.setuid.passed && (s == nil || s.Current == nil) {
		m.setuid.passed = n
		bits = append(bits, setuid.CheckRTFM(&m.meta, m.rtfmMap(), now, false)...)
	}
	return bits
}

// rtfmMap is RTFM's map, nil until RTFM is loaded.
func (m *Model) rtfmMap() *curriculum.Atlas {
	if m.cur == nil {
		return nil
	}
	a := m.cur.Atlas(m.judgeable)
	return &a
}

// checkpointMeta saves Meta, with the boot in play when there is one.
func (m *Model) checkpointMeta() {
	if m.state != nil {
		m.saved(m.d.Store.Checkpoint(m.meta, m.state))
	} else {
		m.saved(m.d.Store.SaveMeta(m.meta))
	}
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
	m.checkpointMeta()
	m.setuidSet(bits, "")
	if m.setuid.notices {
		m.setuid.news += len(bits)
	}
}

// backfillRTFM sets the RTFM bits that trials passed before them earned
// (setuid.CheckRTFM, marked "from git log"), and saves when it set any.
// It runs at launch, after backfillSetuid, which a save backfilled before
// these bits skips, and again once RTFM loads, for the bits that read its
// map. GRUB's status line counts them. A test that fakes the bits
// (Deps.SetuidCheck) gets none.
func (m *Model) backfillRTFM() {
	if m.d.SetuidCheck != nil {
		return
	}
	m.setuid.passed = len(m.meta.TrialsPassed)
	bits := setuid.CheckRTFM(&m.meta, m.rtfmMap(), m.d.Now(), true)
	if len(bits) == 0 {
		return
	}
	m.checkpointMeta()
	m.setuidSet(bits, "")
	if m.setuid.notices {
		m.setuid.news += len(bits)
		m.regrub() // its status line says it
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
