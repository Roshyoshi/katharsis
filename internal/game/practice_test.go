package game_test

import (
	"testing"
	"time"

	"katharsis/internal/game"
)

// A practice clock runs until the solve, across upsolves; the editorial
// opens at EditorialAfter, and the first showing before the solve marks
// it, once.
func TestPracticeClock(t *testing.T) {
	const id = "usaco:738"
	var m game.Meta
	if c := game.Practice(&m, id); c != (game.PracticeClock{Problem: id}) || len(m.Practice) != 0 {
		t.Fatalf("a clock never run: %+v, %d clocks", c, len(m.Practice))
	}
	if left := game.EditorialLeft(&m, id); left != game.EditorialAfter {
		t.Fatalf("editorial left %v, want %v", left, game.EditorialAfter)
	}

	game.AddPractice(&m, id, 12*time.Minute)
	game.AddPractice(&m, id, 0) // nothing
	if game.PracticeEditorial(&m, id) {
		t.Fatal("the editorial marked at 12:00, before it opens")
	}
	if left := game.EditorialLeft(&m, id); left != 8*time.Minute {
		t.Fatalf("at 12:00 the editorial is %v off, want 8m", left)
	}
	game.AddPractice(&m, id, 11*time.Minute) // a second upsolve
	if left := game.EditorialLeft(&m, id); left != 0 {
		t.Fatalf("at 23:00 the editorial is %v off, want open", left)
	}
	if !game.PracticeEditorial(&m, id) || game.PracticeEditorial(&m, id) {
		t.Fatal("the first showing should mark, and only it")
	}
	game.AddPractice(&m, id, 4*time.Minute) // the time to the AC runs on
	m.Solves = append(m.Solves, game.Solve{Problem: id, Kind: game.Editorial, Upsolve: true})
	game.AddPractice(&m, id, time.Hour) // solved: stopped
	want := game.PracticeClock{Problem: id, Spent: 27 * time.Minute, Editorial: 23 * time.Minute}
	if c := game.Practice(&m, id); c != want || len(m.Practice) != 1 {
		t.Errorf("clock %+v (%d clocks), want %+v", c, len(m.Practice), want)
	}
}

// A problem solved before its clock ran (an older save) has its editorial
// open and a clock that never starts; nor does the editorial mark it.
func TestPracticeClockSolvedBefore(t *testing.T) {
	const id = "usaco:643"
	m := game.Meta{Solves: []game.Solve{{Problem: id, Upsolve: true}}}
	game.AddPractice(&m, id, time.Minute)
	if game.EditorialLeft(&m, id) != 0 || game.PracticeEditorial(&m, id) || len(m.Practice) != 0 {
		t.Errorf("solved before: left %v, clocks %+v", game.EditorialLeft(&m, id), m.Practice)
	}
}
