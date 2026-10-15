package meta_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/meta/fixture"
	"katharsis/internal/screens/story"
)

// storyGrub is GRUB for a player with two beats seen, today's nightly
// played once to a clear, and the session budget passed.
func storyGrub() *meta.Menu {
	c := fixture.GrubConfig()
	c.Meta.Seen = []game.SeenScene{{ID: "story.prologue", Boot: 0}, {ID: "story.first-crash", Boot: 3}}
	c.StoryLog = func(m *game.Meta) []flavor.LogEntry {
		var out []flavor.LogEntry
		for _, s := range m.Seen {
			out = append(out, flavor.LogEntry{ID: s.ID, Title: "stub title of " + s.ID, Boot: s.Boot})
		}
		return out
	}
	c.Nightly = &meta.NightlyBest{Played: 1, Cleared: true, Uptime: 92*time.Minute + 10*time.Second}
	c.Today, c.Budget = 3*time.Hour+12*time.Minute, 3*time.Hour
	return meta.NewGrub(c)
}

func TestGrubEndGoldens(t *testing.T) {
	for _, c := range []struct {
		name string
		keys []string
	}{
		{"grub_story", nil},
		{"grub_story_log", []string{"down", "down", "down", "down", "down", "down", "down", "down", "down", "enter"}},
	} {
		for _, sz := range sizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				var s screens.Screen = storyGrub()
				for _, k := range c.keys {
					s, _ = s.Update(key(k))
					s.View(sz[0], sz[1])
				}
				golden(t, name, s.View(sz[0], sz[1]))
			})
		}
	}
}

// The reread log lists the beats newest first, each opening its page.
func TestGrubStoryLog(t *testing.T) {
	g := storyGrub()
	log := []string{"down", "down", "down", "down", "down", "down", "down", "down", "down", "enter"}
	wantMsg(t, press(t, g, append(log, "enter")...), story.Read{ID: "story.first-crash"})
	wantMsg(t, press(t, storyGrub(), append(log, "down", "enter")...), story.Read{ID: "story.prologue"})
	// Without a beat seen there is no log to open.
	if v := plain(grub().View(140, 40)); strings.Contains(v, "/proc/story") {
		t.Errorf("an empty log has an entry:\n%s", v)
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
	if v := plain(storyGrub().View(140, 40)); !strings.Contains(v, "today 3:12 of 3:00") {
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
