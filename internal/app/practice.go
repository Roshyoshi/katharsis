package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/units"
)

// An RTFM practice door is an upsolve of a problem on RTFM's maps
// (curriculum.Listed): no lives, but a clock that counts up
// (game.PracticeClock), summed over every upsolve of the problem until
// its first AC. Its editorial waits on the clock, game.EditorialAfter,
// and showing it stops the clock: e asks first, and the solve after it
// counts as editorial-assisted. Until the game shows it, or the AC, the
// door's folder has no editorial.md (materialize, keepEditorial).

// practiceGap is the most one Tick runs a practice clock on. Ticks come a
// second apart; a longer gap is the machine asleep, not the player at
// work, and a laptop shut overnight would open every editorial.
const practiceGap = 10 * time.Second

// practice reports whether d is an RTFM practice door.
func (d Door) practice() bool { return d.Upsolve && isPractice(d.Door.Problem) }

// isPractice reports whether id, upsolved, is RTFM practice: a problem
// the guide lists, which no boot or trial draws (curriculum.Split).
func isPractice(id catalog.ID) bool { return curriculum.Listed()[id] }

// practiceTick is a Tick on an open practice door: its clock runs on
// (foldPractice), says so once its editorial opens, and is saved each
// minute it passes.
func (m *Model) practiceTick() tea.Cmd {
	o := m.open
	if o == nil || !o.practice() {
		return nil
	}
	id := o.Door.Problem
	before, left := game.Practice(&m.meta, id).Spent, game.EditorialLeft(&m.meta, id)
	m.foldPractice()
	if game.Practice(&m.meta, id).Spent/time.Minute != before/time.Minute {
		m.checkpointMeta()
	}
	if left > 0 && game.EditorialLeft(&m.meta, id) == 0 {
		return note("%s on the clock: the editorial is open · e shows it, and stops the clock",
			units.Clock(game.EditorialAfter))
	}
	return nil
}

// foldPractice runs the open practice door's clock on by the time since it
// last did (Door.Clocked), at most practiceGap.
func (m *Model) foldPractice() {
	o := m.open
	if o == nil || !o.practice() {
		return
	}
	now := m.d.Now()
	game.AddPractice(&m.meta, o.Door.Problem, min(now.Sub(o.Clocked), practiceGap))
	o.Clocked = now
}

// openEditorial shows id's editorial, unless it is RTFM practice whose
// clock still holds it: the clock is the rule, whatever screen asked.
func (m *Model) openEditorial(id catalog.ID) tea.Cmd {
	if left := game.EditorialLeft(&m.meta, id); isPractice(id) && left > 0 {
		return fail("the editorial opens at %s on the clock: %s to go",
			units.Clock(game.EditorialAfter), units.Countdown(left))
	}
	return m.openDoc(id, true)
}

// practiceNote is what an RTFM practice door's clock says beside it: how
// long the editorial still waits, that it is open, or where the clock
// stopped.
func (m *Model) practiceNote(id catalog.ID) string {
	c, solved := game.Practice(&m.meta, id), game.Solved(&m.meta)[id]
	switch left := game.EditorialLeft(&m.meta, id); {
	case solved && c.Editorial > 0:
		return "solved after the editorial"
	case solved:
		return "solved"
	case c.Editorial > 0:
		return "editorial shown"
	case left > 0:
		return "editorial in " + units.Countdown(left)
	}
	return "editorial open"
}

// practiceText is the open practice door's clock in plain words, for the
// HUD's clock and vim's :Status, neither of which takes ⏱ (statusLine):
// "12:34 · editorial in 07:26", "22:10 +01:12 · editorial shown".
func (m *Model) practiceText() string {
	id := m.open.Door.Problem
	c := game.Practice(&m.meta, id)
	t := units.Clock(c.Spent)
	switch {
	case c.Editorial > 0:
		t = units.Clock(c.Editorial) + " +" + units.Clock(c.Spent-c.Editorial)
	case c.Spent == 0 && game.Solved(&m.meta)[id]:
		return m.practiceNote(id) // solved before its clock ran
	}
	return t + " · " + m.practiceNote(id)
}
