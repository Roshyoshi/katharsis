package meta_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/meta/fixture"
	"katharsis/internal/ui"
)

// calendarLog is git log over boots, headed by their calendar.
func calendarLog(boots []meta.BootRow) *meta.Log {
	return meta.NewLog(boots, fixture.PrestigeName).WithCalendar(fixture.CalendarOf(boots))
}

// playerLog is the fixture player's git log and calendar.
func playerLog() *meta.Log {
	return meta.NewLog(fixture.Boots(), fixture.PrestigeName).WithCalendar(fixture.Calendar())
}

// The calendar heads git log: the whole year where it fits two cells a
// day, the last weeks beside the summary at 80 columns, and the whole year
// one cell a day once c gives it the keys. A year without play, a sparse
// one and a dense one; the cursor on the selected boot's day; the dense
// year in ASCII and under Mono, whose shades carry the levels without hue.
func TestCalendarGoldens(t *testing.T) {
	cases := []struct {
		name  string
		make  func() screens.Screen
		keys  []string
		glyph ui.Glyphs
		pal   ui.Palette
	}{
		{name: "empty", make: func() screens.Screen {
			return meta.NewLog(nil, nil).WithCalendar(meta.Calendar{Today: fixture.Now})
		}},
		{name: "sparse", make: func() screens.Screen { return calendarLog(fixture.SparseBoots()) }},
		{name: "dense", make: func() screens.Screen { return calendarLog(fixture.DenseBoots()) }},
		// Boot #41 is selected, so c opens on its day, Oct 13.
		{name: "cursor", make: func() screens.Screen { return playerLog() }, keys: []string{"j", "c"}},
		{name: "ascii", make: func() screens.Screen { return calendarLog(fixture.DenseBoots()) }, glyph: ui.ASCII},
		{name: "mono", make: func() screens.Screen { return calendarLog(fixture.DenseBoots()) }, pal: ui.Mono},
	}
	for _, c := range cases {
		for _, sz := range sizes {
			name := fmt.Sprintf("log_calendar_%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				if c.glyph.Fill != "" {
					ui.SetGlyphs(c.glyph)
					defer ui.SetGlyphs(ui.Unicode)
				}
				ui.SetPalette(c.pal)
				defer ui.SetPalette(ui.ANSI)
				s := c.make()
				for _, k := range c.keys {
					s, _ = s.Update(key(k))
					s.View(sz[0], sz[1])
				}
				view := s.View(sz[0], sz[1])
				golden(t, name, view)
				if c.pal == ui.Mono && hued(view) {
					t.Errorf("Mono paints a hue:\n%q", view)
				}
			})
		}
	}
}

var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// hued reports whether s paints any foreground or background color.
func hued(s string) bool {
	for _, m := range sgr.FindAllStringSubmatch(s, -1) {
		for _, p := range strings.Split(m[1], ";") {
			var n int
			if _, err := fmt.Sscan(p, &n); err == nil && (n >= 30 && n <= 49 || n >= 90 && n <= 107) {
				return true
			}
		}
	}
	return false
}

// calendarOf is a calendar with door time on the days given, counted back
// from fixture.Now's day.
func calendarOf(door map[int]time.Duration) meta.Calendar {
	c := meta.Calendar{Today: fixture.Now}
	today := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	for ago, d := range door {
		c.Days = append(c.Days, meta.Day{Date: today.AddDate(0, 0, -ago), Door: d, Boots: 1})
	}
	return c
}

// row is the calendar's row for a weekday at 140 columns, its cells only.
func row(t *testing.T, s screens.Screen, weekday string) string {
	t.Helper()
	for _, l := range strings.Split(plain(s.View(140, 40)), "\n") {
		if strings.HasPrefix(l, weekday+" ") {
			return strings.TrimSpace(strings.TrimSuffix(l[len(weekday)+1:], "less ░▒▓█ more"))
		}
	}
	t.Fatalf("no %s row:\n%s", weekday, plain(s.View(140, 40)))
	return ""
}

// A day's shade is the quarter its door time ranks in among the year's
// days with any, the way GitHub ranks contributions: four Wednesdays of
// 10 to 40 minutes take the four shades, and a day without play none.
func TestCalendarShades(t *testing.T) {
	min := func(n int) time.Duration { return time.Duration(n) * time.Minute }
	s := meta.NewLog(nil, nil).WithCalendar(calendarOf(map[int]time.Duration{0: min(40), 7: min(10), 14: min(30), 21: min(20)}))
	if got, want := row(t, s, "Wed"), strings.Repeat("· ", 49)+"▒ ▓ ░ █"; got != want {
		t.Errorf("Wednesdays:\n got %q\nwant %q", got, want)
	}
	// One day of play is the year's most, however short.
	s = meta.NewLog(nil, nil).WithCalendar(calendarOf(map[int]time.Duration{2: min(3)}))
	if got := row(t, s, "Mon"); !strings.HasSuffix(got, "█") || strings.Count(got, "█") != 1 {
		t.Errorf("Mondays with one 3-minute day: %q", got)
	}
	// Days after today are blank, not empty: Thursday to Sunday end a
	// cell short.
	for _, wd := range []string{"Thu", "Sun"} {
		if got := row(t, s, wd); strings.Count(got, "·") != 52 {
			t.Errorf("%s has %d cells, want 52: %q", wd, strings.Count(got, "·"), got)
		}
	}
}

// c gives the calendar the keys, on the selected boot's day: hjkl and the
// arrows move it a week across or a day down within the year drawn, the
// detail line follows, and c or esc gives the keys back. enter goes to the
// newest boot of the day. git log's own q, r and f still work.
func TestCalendarKeys(t *testing.T) {
	detail := func(s screens.Screen) string {
		t.Helper()
		for _, l := range strings.Split(plain(s.View(140, 40)), "\n") {
			if strings.HasPrefix(l, "Mon 20") || strings.HasPrefix(l, "Tue 20") || strings.HasPrefix(l, "Wed 20") ||
				strings.HasPrefix(l, "Thu 20") || strings.HasPrefix(l, "Fri 20") || strings.HasPrefix(l, "Sat 20") ||
				strings.HasPrefix(l, "Sun 20") {
				return l
			}
		}
		return ""
	}
	s := playerLog()
	if d := detail(s); d != "" {
		t.Errorf("a detail line before c: %q", d)
	}
	press(t, s, "j", "j", "c") // boot #40, Oct 12
	if got, want := detail(s), "Mon 2026-10-12 · 55:35 in doors · 1 boot · 1 crashed · 1 upsolve"; got != want {
		t.Errorf("c on boot #40:\n got %q\nwant %q", got, want)
	}
	for _, step := range []struct {
		keys []string
		want string
	}{
		{[]string{"k"}, "Sun 2026-10-11"},
		{[]string{"down", "down"}, "Tue 2026-10-13"},
		{[]string{"h"}, "Tue 2026-10-06"},
		{[]string{"l", "right"}, "Tue 2026-10-13"}, // no week after this one
		{[]string{"j", "j"}, "Wed 2026-10-14"},     // nor a day after today
		{[]string{"G", "left", "up"}, "Tue 2026-10-06"},
		{[]string{"g", "up", "h"}, "Mon 2025-10-13"}, // the first day drawn
		{[]string{"end"}, "Wed 2026-10-14"},
	} {
		press(t, s, step.keys...)
		if got := detail(s); !strings.HasPrefix(got, step.want) {
			t.Errorf("after %v the cursor is on %q, want %s", step.keys, got, step.want)
		}
	}
	// enter shows the day's boot in the list, and the keys go back to it.
	press(t, s, "h", "enter")
	if v := plain(s.View(140, 40)); detail(s) != "" || !strings.Contains(v, "#35 2026-10-07") {
		t.Errorf("enter on Oct 7:\n%s", v)
	} else if sel := selectedLine(t, s); !strings.Contains(sel, "#35") {
		t.Errorf("enter on Oct 7 selected %q, want boot #35", sel)
	}
	press(t, s, "c", "esc")
	if d := detail(s); d != "" {
		t.Errorf("esc kept the calendar's keys: %q", d)
	}
	press(t, s, "c", "f") // the filter still cycles
	if v := plain(s.View(140, 40)); !strings.Contains(v, "filter: cleared") || detail(s) == "" {
		t.Errorf("f under the calendar:\n%s", v)
	}
	wantMsg(t, press(t, s, "q"), screens.Back{})

	// A boot older than the calendar opens it on today.
	old := []meta.BootRow{{Boot: 1, Started: fixture.Now.AddDate(-2, 0, 0)}}
	s = meta.NewLog(old, nil).WithCalendar(meta.Calendar{Today: fixture.Now})
	press(t, s, "c")
	if got := detail(s); got != "Wed 2026-10-14" {
		t.Errorf("c past the year: %q", got)
	}
}

// selectedLine is the list's reversed row, as text.
func selectedLine(t *testing.T, s screens.Screen) string {
	t.Helper()
	for _, l := range strings.Split(s.View(140, 40), "\n") {
		if strings.Contains(l, "\x1b[7m") {
			return ansi.Strip(l)
		}
	}
	t.Fatal("no row selected")
	return ""
}

// At 80x24 the calendar is the last weeks beside its summary, with the
// list keeping its rows; c shows the whole year, one cell a day. Short
// screens drop the calendar until c, and every view fits.
func TestCalendarSizes(t *testing.T) {
	s := playerLog()
	v := plain(s.View(80, 24))
	if !strings.Contains(v, "TIME+ per day") || !strings.Contains(v, "less ░▒▓█ more") || strings.Count(v, "* ") < 10 {
		t.Errorf("80x24:\n%s", v)
	}
	press(t, s, "c")
	lines := strings.Split(plain(s.View(80, 24)), "\n")
	if got := ansi.StringWidth(strings.TrimPrefix(lines[4], "Tue ")); got != 53 {
		t.Errorf("80x24 under c: Tuesday has %d cells, want 53:\n%s", got, strings.Join(lines, "\n"))
	}
	if v := plain(playerLog().View(80, 14)); strings.Contains(v, "TIME+") {
		t.Errorf("80x14 keeps the calendar over the list:\n%s", v)
	}

	for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {60, 15}, {80, 24}, {100, 30}, {124, 30}, {125, 30}, {140, 40}, {220, 60}} {
		for _, keys := range [][]string{nil, {"c"}, {"c", "g"}} {
			s := playerLog()
			press(t, s, keys...)
			lines := strings.Split(s.View(sz[0], sz[1]), "\n")
			if len(lines) != sz[1] {
				t.Errorf("%dx%d %v: %d lines", sz[0], sz[1], keys, len(lines))
			}
			for i, l := range lines {
				if n := ansi.StringWidth(l); n > sz[0] {
					t.Errorf("%dx%d %v: line %d is %d wide: %q", sz[0], sz[1], keys, i, n, ansi.Strip(l))
				}
			}
		}
	}
}
