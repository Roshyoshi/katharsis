package meta_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/meta/fixture"
	"katharsis/internal/screens/story"
)

// nightlyGrub is GRUB for a player with today's nightly played once to a
// clear, and the session budget passed.
func nightlyGrub() *meta.Menu {
	c := fixture.GrubConfig()
	c.Nightly = &meta.NightlyBest{Played: 1, Cleared: true, Uptime: 92*time.Minute + 10*time.Second}
	c.Today, c.Budget = 3*time.Hour+12*time.Minute, 3*time.Hour
	return meta.NewGrub(c)
}

func TestGrubNightlyGolden(t *testing.T) {
	for _, sz := range sizes {
		name := fmt.Sprintf("grub_nightly_%dx%d", sz[0], sz[1])
		t.Run(name, func(t *testing.T) { golden(t, name, nightlyGrub().View(sz[0], sz[1])) })
	}
}

// The selected entry's subtitle shows under the box, in the row it
// already had; moving the cursor moves it.
func TestGrubSubtitles(t *testing.T) {
	g := grub()
	if v := plain(g.View(80, 24)); !strings.Contains(v, "stub grub.subtitle.prestige.2") {
		t.Errorf("default entry's subtitle missing:\n%s", v)
	}
	press(t, g, "down")
	if v := plain(g.View(80, 24)); !strings.Contains(v, "stub grub.subtitle.coredump") {
		t.Errorf("core dump's subtitle missing:\n%s", v)
	}
}

// Past the budget the status line says so; under it, nothing (RT12).
func TestGrubBudget(t *testing.T) {
	if v := plain(nightlyGrub().View(140, 40)); !strings.Contains(v, "today 3:12 of 3:00") {
		t.Errorf("no budget note:\n%s", v)
	}
	if v := plain(grub().View(140, 40)); strings.Contains(v, "today ") {
		t.Errorf("a budget note under budget:\n%s", v)
	}
}

// GRUB's upsolves list at most three due regressions and no count past
// them (GR10).
func TestGrubRegressionsCapped(t *testing.T) {
	c := fixture.GrubConfig()
	c.Meta.Regressions = nil
	for i := range 10 {
		c.Meta.Regressions = append(c.Meta.Regressions, game.Regression{Problem: "usaco:891", Due: fixture.Now.Add(-time.Duration(i+1) * time.Hour)})
	}
	g := meta.NewGrub(c)
	if v := plain(g.View(140, 40)); strings.Contains(v, "10 ") || strings.Contains(v, "due: ") {
		t.Errorf("GRUB counts the regressions:\n%s", v)
	}
	press(t, g, "down", "enter")
	v := plain(g.View(140, 40))
	if n := strings.Count(v, "regression: "); n != 3 || strings.Contains(v, "10 ") {
		t.Errorf("the upsolves list %d regressions, want 3 and no count:\n%s", n, v)
	}
}

// git log marks the nightlies: the first play of a day's seed as first,
// replays as practice; r plays the selected boot's seed again, a past
// nightly's included (RT15).
func TestLogNightlies(t *testing.T) {
	day := time.Date(2026, 10, 12, 21, 0, 0, 0, time.UTC)
	seed := game.NightlySeed(day)
	rows := []meta.BootRow{
		{Boot: 3, Seed: seed, Preset: "standard", Prestige: 1, Started: day.Add(2 * time.Hour), Cause: game.TimedOut},
		{Boot: 2, Seed: seed, Preset: "standard", Prestige: 1, Started: day.Add(time.Hour), Cleared: true},
		{Boot: 1, Seed: 0x51c0ffee, Preset: "standard", Started: day, Cause: game.GaveUp},
	}
	l := meta.NewLog(rows, fixture.PrestigeName)
	v := plain(l.View(120, 10))
	lines := strings.Split(v, "\n")
	if !strings.Contains(lines[1], "nightly") || strings.Contains(lines[1], "first") ||
		!strings.Contains(lines[2], "nightly · first") || strings.Contains(lines[3], "nightly") {
		t.Errorf("nightly marks wrong:\n%s", v)
	}
	wantMsg(t, press(t, l, "r"), screens.StartBoot{Prestige: 1, Preset: "standard", Seed: seed})
}

// The upsolves' first opening with a regression due plays the White
// Queen's beat, once; a player who has seen it goes straight in.
func TestGrubFirstRegression(t *testing.T) {
	regs := []string{"down", "enter"}
	g := grub()
	wantMsg(t, press(t, g, regs...), story.Read{ID: "story.first-regression"})
	wantMsg(t, press(t, g, "esc", "enter"), nil)
	c := fixture.GrubConfig()
	game.MarkSeen(&c.Meta, "story.first-regression")
	wantMsg(t, press(t, meta.NewGrub(c), regs...), nil)
}

// GRUB counts today's play in whole minutes gone, as the end screens'
// budget line does, so the two never disagree: 3:12:40 is 3:12.
func TestGrubBudgetWholeMinutes(t *testing.T) {
	c := fixture.GrubConfig()
	c.Today, c.Budget = 3*time.Hour+12*time.Minute+40*time.Second, 3*time.Hour
	if v := plain(meta.NewGrub(c).View(140, 40)); !strings.Contains(v, "today 3:12 of 3:00") {
		t.Errorf("3:12:40 should read 3:12:\n%s", v)
	}
}

// A due regression's note fills {days} from its own schedule
// (Regression.Days, as the engine advances it), and from the standard
// rules' for one scheduled before Days existed.
func TestGrubRegressionDays(t *testing.T) {
	c := fixture.GrubConfig()
	c.Story = flavor.Story
	game.MarkSeen(&c.Meta, "story.first-regression")
	due := fixture.Now.Add(-time.Hour)
	c.Meta.Regressions = []game.Regression{ // oldest due first, as GRUB lists them
		{Problem: "usaco:891", Due: due.Add(-2 * time.Hour), Stage: 0, Days: []int{5, 40}},
		{Problem: "usaco:1205", Due: due.Add(-time.Hour), Stage: 1, Days: []int{5, 40}},
		{Problem: "usaco:740", Due: due, Stage: 1},
	}
	std := rules.Legacy().RegressionDays
	want := []string{
		flavor.Fill(flavor.Story("regression.stage.1", 0), "days", "5"),
		flavor.Fill(flavor.Story("regression.stage.2", 0), "days", "40"),
		flavor.Fill(flavor.Story("regression.stage.2", 0), "days", fmt.Sprint(std[1])),
	}
	g := meta.NewGrub(c)
	press(t, g, "down", "enter", "down") // the upsolves, past the core dump
	for i, w := range want {
		v := plain(g.View(140, 40))
		if !strings.Contains(v, w) || strings.Contains(v, "{days}") {
			t.Errorf("regression %d: want %q in:\n%s", i+1, w, v)
		}
		press(t, g, "down")
	}
}

// fullGrub is GRUB with all the news the status line can carry beside
// TIME+: the days away, the load average, the budget passed and a source
// still indexing.
func fullGrub(timePlus time.Duration) *meta.Menu {
	c := fixture.GrubConfig()
	c.Away, c.Today, c.Budget = 5, 3*time.Hour+12*time.Minute, 3*time.Hour
	c.Load = &[3]float64{0.71, 0.43, 0.30}
	c.Sources = func() string { return "Codeforces indexing… 40%" }
	c.TimePlus = timePlus
	return meta.NewGrub(c)
}

// loadGrub is GRUB for a player who plays most days: a load average and
// nothing else on the status line.
func loadGrub() *meta.Menu {
	c := fixture.GrubConfig()
	c.Load = &[3]float64{0.71, 0.43, 0.30}
	return meta.NewGrub(c)
}

// statusLine is the view's status line, the one with the bank.
func statusLine(v string) string {
	for _, l := range strings.Split(plain(v), "\n") {
		if strings.Contains(l, "bank ") {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// TIME+ sits by the bank and rating where the status line has room, and
// is the first thing to go where it has none: the line is then the one
// it would be without TIME+, the news intact.
func TestGrubTimePlus(t *testing.T) {
	tp := fixture.Stats().TimePlus
	for _, sz := range sizes {
		if s := statusLine(grub().View(sz[0], sz[1])); !strings.HasPrefix(s, "bank 1.3 KiB · rating 1432 · TIME+ 61h20m · next boot #43") {
			t.Errorf("at %dx%d the status line is %q", sz[0], sz[1], s)
		}
		if got, want := statusLine(fullGrub(tp).View(sz[0], sz[1])), statusLine(fullGrub(0).View(sz[0], sz[1])); got != want {
			t.Errorf("at %dx%d a full line kept TIME+ over its news:\n%q\nwant %q", sz[0], sz[1], got, want)
		}
	}
	// One load average leaves no room at 80 columns, and plenty at 140.
	if s := statusLine(loadGrub().View(80, 24)); strings.Contains(s, "TIME+") || !strings.Contains(s, "load average") {
		t.Errorf("at 80x24 with a load average: %q", s)
	}
	if s := statusLine(loadGrub().View(140, 40)); !strings.Contains(s, "rating 1432 · TIME+ 61h20m · next boot #43 · load average") {
		t.Errorf("at 140x40 with a load average: %q", s)
	}
	c := fixture.GrubConfig()
	c.TimePlus = 0
	if s := statusLine(meta.NewGrub(c).View(140, 40)); strings.Contains(s, "TIME+") {
		t.Errorf("no TIME+ yet, but: %q", s)
	}
}

// top's TIME+ is the header's total, in the meter TIME+ took from the
// boots-only uptime, and each topic's share in its own column, which s
// sorts by after TOPIC.
func TestTopTimePlus(t *testing.T) {
	st := fixture.Stats()
	var sum time.Duration
	for _, r := range st.Topics {
		sum += r.Time
	}
	if sum != st.TimePlus {
		t.Fatalf("fixture topics add up to %v, TIME+ is %v", sum, st.TimePlus)
	}
	for _, sz := range sizes {
		v := plain(meta.NewTop(st).View(sz[0], sz[1]))
		if !strings.Contains(v, "TIME+: 61:20:44") || strings.Contains(v, "Uptime") {
			t.Errorf("at %dx%d no TIME+ meter:\n%s", sz[0], sz[1], v)
		}
		lines := strings.Split(v, "\n")
		head := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "  TOPIC") })
		if head < 0 || !strings.Contains(lines[head], "RATING     TIME+") {
			t.Fatalf("at %dx%d no TIME+ column:\n%s", sz[0], sz[1], v)
		}
		for _, want := range [][2]string{{"dp", "9:41:20"}, {"strings", "47:12"}} {
			i := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "  "+want[0]+" ") })
			if i < 0 {
				if sz[1] >= 40 {
					t.Errorf("at %dx%d no %s row", sz[0], sz[1], want[0])
				}
				continue
			}
			if f := strings.Fields(lines[i]); len(f) < 6 || f[5] != want[1] {
				t.Errorf("at %dx%d %s reads %q, want TIME+ %s", sz[0], sz[1], want[0], lines[i], want[1])
			}
		}
	}
}
