package meta_test

import (
	"fmt"
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

// GRUB lists at most three due regressions and no count past them
// (GR10).
func TestGrubRegressionsCapped(t *testing.T) {
	c := fixture.GrubConfig()
	c.Meta.Regressions = nil
	for i := range 10 {
		c.Meta.Regressions = append(c.Meta.Regressions, game.Regression{Problem: "usaco:891", Due: fixture.Now.Add(-time.Duration(i+1) * time.Hour)})
	}
	g := meta.NewGrub(c)
	v := plain(g.View(140, 40))
	if !strings.Contains(v, "regressions due: 3+") || strings.Contains(v, "due: 10") {
		t.Errorf("regressions not capped:\n%s", v)
	}
	press(t, g, "down", "down", "down", "down", "down", "down", "down", "enter")
	if n := strings.Count(plain(g.View(140, 40)), "regression: "); n != 3 {
		t.Errorf("submenu lists %d regressions, want 3", n)
	}
}

// git log marks the nightlies: the first play of a day's seed as first,
// replays as practice; r plays the selected boot's seed again, a past
// nightly's included (RT15).
func TestLogNightlies(t *testing.T) {
	day := time.Date(2026, 10, 12, 21, 0, 0, 0, time.UTC)
	seed := meta.NightlySeed(day)
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

// The regressions submenu's first opening plays the White Queen's beat,
// once; a player who has seen it goes straight in.
func TestGrubFirstRegression(t *testing.T) {
	regs := []string{"down", "down", "down", "down", "down", "down", "down", "enter"}
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
	press(t, g, "down", "down", "down", "down", "down", "down", "down", "enter")
	for i, w := range want {
		v := plain(g.View(140, 40))
		if !strings.Contains(v, w) || strings.Contains(v, "{days}") {
			t.Errorf("regression %d: want %q in:\n%s", i+1, w, v)
		}
		press(t, g, "down")
	}
}
