package app

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
)

// paired is Paired Up, on Two Pointers' map: RTFM practice.
const paired = catalog.ID("usaco:738")

// practise opens paired as RTFM practice, with an editorial to show.
func practise(h *harness) {
	h.t.Helper()
	loadGuide(h)
	h.m.d.Catalog = editorialCatalog{h.m.d.Catalog.(*fakeCatalog)}
	h.send(screens.Upsolve{Problem: paired})
	if h.m.open == nil || !h.m.open.practice() {
		h.t.Fatalf("Upsolve(%s) opened %+v, want a practice door", paired, h.m.open)
	}
}

// work runs the clock d on, a tick every practiceGap.
func (h *harness) work(d time.Duration) {
	for ; d > 0; d -= practiceGap {
		h.clock.t = h.clock.t.Add(min(d, practiceGap))
		h.send(tickMsg(h.clock.t))
	}
}

func spent(h *harness) game.PracticeClock { return game.Practice(&h.m.meta, paired) }

// The clock counts up while the door is open, and no more than
// practiceGap for a tick after a long gap; the editorial waits for
// EditorialAfter on it, from the pane's e and from the app, then asks
// once, stops the clock red and joins the folder; the AC after it runs
// on in yellow and stops it. The pane says each.
func TestPracticeClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		practise(h)
		h.work(12 * time.Minute)
		if c := spent(h); c.Spent != 12*time.Minute {
			t.Fatalf("12 minutes in, the clock has %v", c.Spent)
		}
		if v := view(h); !strings.Contains(v, "practice · ") || !strings.Contains(v, "⏱ 12:00  editorial in 08:00") ||
			!strings.Contains(v, "e editorial 08:00") {
			t.Errorf("the pane at 12:00:\n%s", v)
		}
		h.send(key("e"))
		if _, ok := h.m.top().(*problemScreen); !ok || !strings.Contains(h.m.flash, "opens at 20:00 on the clock: 08:00 to go") {
			t.Fatalf("e at 12:00 shows %T, flash %q", h.m.top(), h.m.flash)
		}
		h.send(screens.OpenEditorial{Problem: paired}) // the rule is the app's too
		if _, ok := h.m.top().(*problemScreen); !ok {
			t.Fatalf("OpenEditorial at 12:00 shows %T", h.m.top())
		}

		// The machine asleep for an hour counts one gap.
		h.clock.t = h.clock.t.Add(time.Hour)
		h.send(tickMsg(h.clock.t))
		if c := spent(h); c.Spent != 12*time.Minute+practiceGap {
			t.Fatalf("after an hour asleep the clock has %v", c.Spent)
		}

		h.work(8*time.Minute - practiceGap)
		if !strings.Contains(h.m.flash, "the editorial is open") {
			t.Errorf("at 20:00 the flash is %q", h.m.flash)
		}
		h.work(time.Minute)
		h.send(key("e"))
		if p, ok := h.m.top().(*problemScreen); !ok || !strings.Contains(p.ask, "e again: the editorial stops the clock at 21:00") {
			t.Fatalf("e at 21:00 shows %T, %+v; want it to ask", h.m.top(), p)
		}
		h.send(key("e"))
		if _, ok := h.m.top().(*reader.Model); !ok {
			t.Fatalf("e again shows %T, want the editorial", h.m.top())
		}
		if c := spent(h); c.Editorial != 21*time.Minute || !h.m.open.ReadEditorial {
			t.Fatalf("the editorial marked %+v, read %v", c, h.m.open.ReadEditorial)
		}
		h.send(screens.Back{})

		h.work(5 * time.Minute)
		if v := view(h); !strings.Contains(v, "⏱ 21:00 +05:00  editorial shown") {
			t.Errorf("five minutes after the editorial:\n%s", v)
		}
		h.send(submitMsg{})
		h.work(time.Hour) // solved: stopped
		last := h.m.meta.Solves[len(h.m.meta.Solves)-1]
		if c := spent(h); c.Spent != 26*time.Minute || last.Problem != paired || last.Kind != game.Editorial {
			t.Fatalf("after the AC: clock %+v, solve %+v", c, last)
		}
		if v := view(h); !strings.Contains(v, "⏱ 21:00 +05:00  ✓ solved after the editorial") {
			t.Errorf("solved after the editorial:\n%s", v)
		}
		if saved, err := h.st.LoadMeta(); err != nil || game.Practice(&saved, paired) != spent(h) {
			t.Errorf("saved clock %+v (%v), want %+v", game.Practice(&saved, paired), err, spent(h))
		}
	})
}

// The clock is the problem's, not the door's: it goes on from where it
// was over a second upsolve, and stops green at an AC with no editorial,
// after which e shows it without asking or marking it.
func TestPracticeClockAcrossUpsolves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		practise(h)
		h.work(5 * time.Minute)
		h.send(key("Q"))
		if h.m.open != nil {
			t.Fatal("Q left the door open")
		}
		h.clock.t = h.clock.t.Add(time.Hour) // away: not counted
		h.send(screens.Upsolve{Problem: paired})
		h.work(3 * time.Minute)
		h.send(submitMsg{})
		if c := spent(h); c.Spent != 8*time.Minute || c.Editorial != 0 {
			t.Fatalf("clock %+v, want 8:00 and no editorial", c)
		}
		if v := view(h); !strings.Contains(v, "⏱ 08:00  ✓ solved") || strings.Contains(v, "editorial 1") {
			t.Errorf("solved at 8:00:\n%s", v)
		}
		h.send(key("e"))
		if _, ok := h.m.top().(*reader.Model); !ok || spent(h).Editorial != 0 {
			t.Errorf("e once solved shows %T, clock %+v; want the editorial, unmarked", h.m.top(), spent(h))
		}
	})
}

// A boot door and any other upsolve keep no clock: a core dump's upsolve
// is untimed, its editorial allowed.
func TestUpsolveOffTheMapsIsUntimed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.crashWithDump()
		id := h.m.state.Crash.Door.Problem
		h.send(screens.Upsolve{Problem: id})
		if h.m.open == nil || h.m.open.practice() || isPractice(id) {
			t.Fatalf("the core dump's upsolve is practice: %+v", h.m.open)
		}
		h.work(time.Minute)
		if len(h.m.meta.Practice) != 0 || !strings.Contains(view(h), "untimed, editorial allowed") {
			t.Errorf("clocks %+v:\n%s", h.m.meta.Practice, view(h))
		}
	})
}
