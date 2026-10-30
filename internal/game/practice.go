package game

import (
	"slices"
	"time"

	"katharsis/internal/catalog"
)

// EditorialAfter is how long an RTFM practice problem is worked on before
// its editorial opens (DESIGN.md "Learn mode").
const EditorialAfter = 20 * time.Minute

// PracticeClock is an RTFM practice problem's stopwatch (Meta.Practice).
// Spent is the time its upsolves were open, summed over every one of them
// until its first AC, where it stops: once solved, Spent is the time to
// the solve. Editorial is the Spent at which the game first showed the
// editorial, 0 for never (it opens only at EditorialAfter, so 0 is no
// mark): the clock as the player sees it stops there, red, and the time
// from it on to the AC counts apart.
type PracticeClock struct {
	Problem   catalog.ID
	Spent     time.Duration
	Editorial time.Duration `json:",omitempty"`
}

// Practice is problem id's clock in m; the zero clock, Spent 0, for one
// that never ran.
func Practice(m *Meta, id catalog.ID) PracticeClock {
	if i := slices.IndexFunc(m.Practice, func(c PracticeClock) bool { return c.Problem == id }); i >= 0 {
		return m.Practice[i]
	}
	return PracticeClock{Problem: id}
}

// clock is id's clock in m, added when it has none.
func clock(m *Meta, id catalog.ID) *PracticeClock {
	if i := slices.IndexFunc(m.Practice, func(c PracticeClock) bool { return c.Problem == id }); i >= 0 {
		return &m.Practice[i]
	}
	m.Practice = append(m.Practice, PracticeClock{Problem: id})
	return &m.Practice[len(m.Practice)-1]
}

// AddPractice adds d, time an upsolve of id was open, to id's clock while
// the clock runs: until id is solved, anywhere (Solved).
func AddPractice(m *Meta, id catalog.ID, d time.Duration) {
	if d <= 0 || Solved(m)[id] {
		return
	}
	clock(m, id).Spent += d
}

// EditorialLeft is how long id's editorial still waits on its clock; 0
// once it is open: EditorialAfter spent on it, the editorial already
// shown, or id solved.
func EditorialLeft(m *Meta, id catalog.ID) time.Duration {
	c := Practice(m, id)
	if c.Editorial > 0 || Solved(m)[id] {
		return 0
	}
	return max(EditorialAfter-c.Spent, 0)
}

// PracticeEditorial marks id's editorial shown where its clock stands, the
// mark it stops at (PracticeClock.Editorial). Only the first showing
// before the solve marks, and only once the editorial is open; it reports
// whether this one did.
func PracticeEditorial(m *Meta, id catalog.ID) bool {
	c := Practice(m, id)
	if c.Editorial > 0 || Solved(m)[id] || c.Spent < EditorialAfter {
		return false
	}
	clock(m, id).Editorial = c.Spent
	return true
}
