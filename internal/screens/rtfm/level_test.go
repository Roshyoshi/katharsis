package rtfm_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
	"katharsis/internal/screens/rtfm/fixture"
	"katharsis/internal/ui"
)

// atlasLevel finds module id's level in the fixture atlas and its region.
func atlasLevel(t testing.TB, c *curriculum.Curriculum, id string) (curriculum.Level, string) {
	t.Helper()
	for _, w := range c.Atlas(fixture.Judgeable).Worlds {
		for _, r := range w.Regions {
			for _, l := range r.Levels {
				if l.Module == id {
					return l, r.Title
				}
			}
		}
	}
	t.Fatalf("fixture atlas has no level %q", id)
	return curriculum.Level{}, ""
}

// The fixture judges USACO only, so Two Pointers's map is Paired Up
// (usaco:738) and Sleepy Cow Herding (usaco:918) into Diamond Collector
// (usaco:643), with Cow Checkups (usaco:1470) the stretch off Paired Up.
const (
	pairedUp  = catalog.ID("usaco:738")
	sleepy    = catalog.ID("usaco:918")
	diamond   = catalog.ID("usaco:643")
	stretchID = catalog.ID("usaco:1470")
)

// The fixture's levels have no prerequisite or shortcut inside the
// guide, so the trial level is given some: Two Pointers needs Prefix Sums
// (passed) and Casework, and opens Matrix Exponentiation and Counting
// Minimums. The player opened the page and solved what solved names, and
// passed the trials passed names besides Prefix Sums's.
func twoPointers(t testing.TB, passed []string, solved ...catalog.ID) *rtfm.Level {
	c := loadCurriculum(t)
	l, region := atlasLevel(t, c, "two-pointers")
	l.Prereqs = []string{"prefix-sums", "casework"}
	l.Warps = []string{"matrix-expo", "count-min"}
	m := fixture.Meta()
	for _, id := range solved {
		m.Solves = append(m.Solves, game.Solve{Problem: id})
	}
	m.TrialsPassed = append([]string{"prefix-sums"}, passed...)
	m.Opened = []string{"two-pointers"}
	return rtfm.NewLevel(c, l, region, m, fixture.Judgeable, fixture.Lookup)
}

// trialLevel is Two Pointers with Paired Up solved.
func trialLevel(t testing.TB) *rtfm.Level { return twoPointers(t, nil, pairedUp) }

// pageLevel is General's Fast I/O: a page, reading only.
func pageLevel(t testing.TB) *rtfm.Level {
	c := loadCurriculum(t)
	l, region := atlasLevel(t, c, "fast-io")
	return rtfm.NewLevel(c, l, region, fixture.Meta(), fixture.Judgeable, fixture.Lookup)
}

// toppedLevel is Counting Minimums: a gated topic with nothing the fixture
// catalog judges, so a page whose capability is earned in another module.
func toppedLevel(t testing.TB) *rtfm.Level {
	c := loadCurriculum(t)
	l, region := atlasLevel(t, c, "count-min")
	return rtfm.NewLevel(c, l, region, fixture.Meta(), fixture.Judgeable, fixture.Lookup)
}

// castleLevel is the Bronze conclusion, its first problem upsolved.
func castleLevel(t testing.TB) *rtfm.Level {
	c := loadCurriculum(t)
	l, region := atlasLevel(t, c, "bronze-conclusion")
	m := fixture.Meta()
	m.Solves = []game.Solve{{Problem: l.Map.Nodes[0].ID, Upsolve: true}}
	return rtfm.NewLevel(c, l, region, m, fixture.Judgeable, fixture.Lookup)
}

func TestLevelGoldens(t *testing.T) {
	cases := []struct {
		name string
		make func(t testing.TB) *rtfm.Level
		keys []string
	}{
		{"level_trial", trialLevel, nil},
		{"level_trial_node", trialLevel, []string{"n"}},
		{"level_trial_end", trialLevel, []string{"G"}},
		{"level_page", pageLevel, nil},
		{"level_topped", toppedLevel, nil},
		{"level_castle", castleLevel, []string{"n"}},
	}
	for _, c := range cases {
		for _, sz := range sizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				v := c.make(t)
				for _, k := range c.keys {
					v.Update(key(k))
					v.View(sz[0], sz[1])
				}
				golden(t, name, v.View(sz[0], sz[1]))
			})
		}
	}
	t.Run("level_trial_ascii_80x24", func(t *testing.T) {
		ui.SetGlyphs(ui.ASCII)
		defer ui.SetGlyphs(ui.Unicode)
		v := plain(trialLevel(t).View(80, 24))
		// The attribution keeps its curly quotes: they are the guide's.
		for _, g := range []string{"▸", "✓", "■", "□", "›", "·", "↑", "─", "│", "╰", "◆", "★", "☆", "→", "●", "○", "⚔"} {
			if strings.Contains(v, g) {
				t.Errorf("the ASCII level draws %q", g)
			}
		}
		golden(t, "level_trial_ascii_80x24", v)
	})
}

// TestLevelFits renders each level at sizes from cramped to huge: exactly
// height lines, none wider than width.
func TestLevelFits(t *testing.T) {
	states := map[string]func(t testing.TB) *rtfm.Level{
		"trial": trialLevel, "page": pageLevel, "topped": toppedLevel, "castle": castleLevel,
		"bare": func(testing.TB) *rtfm.Level {
			return rtfm.NewLevel(nil, curriculum.Level{Module: "x"}, "", game.Meta{}, nil, nil)
		},
	}
	for name, mk := range states {
		for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {60, 15}, {80, 24}, {100, 30}, {140, 40}, {220, 60}} {
			w, h := sz[0], sz[1]
			for _, keys := range [][]string{nil, {"n"}, {"G"}} {
				v := mk(t)
				for _, k := range keys {
					v.Update(key(k))
				}
				lines := strings.Split(v.View(w, h), "\n")
				if len(lines) != h {
					t.Errorf("%s %v at %dx%d: %d lines, want %d", name, keys, w, h, len(lines), h)
				}
				for i, l := range lines {
					if n := ansi.StringWidth(l); n > w {
						t.Errorf("%s %v at %dx%d: line %d is %d wide: %q", name, keys, w, h, i, n, ansi.Strip(l))
					}
				}
			}
		}
	}
}

// TestLevelIntents: enter acts on the item under the cursor, which the
// arrows move across the map; a locked problem does nothing, o included,
// and a page has no trial to start.
func TestLevelIntents(t *testing.T) {
	tests := []struct {
		name string
		make func(t testing.TB) *rtfm.Level
		keys []string
		want any
	}{
		{"enter on READ", trialLevel, []string{"enter"}, screens.OpenModule{Module: "two-pointers"}},
		{"r", trialLevel, []string{"l", "r"}, screens.OpenModule{Module: "two-pointers"}},
		// READ's first path goes to Paired Up, solved: again, untimed.
		{"a solved problem", trialLevel, []string{"l", "enter"}, screens.Upsolve{Problem: pairedUp}},
		{"an open root", trialLevel, []string{"l", "j", "enter"}, screens.Upsolve{Problem: sleepy}},
		// Diamond Collector waits on Sleepy Cow Herding.
		{"a locked problem", trialLevel, []string{"l", "l", "enter"}, nil},
		{"next", trialLevel, []string{"n", "enter"}, screens.Upsolve{Problem: sleepy}},
		{"next once the core is solved", func(t testing.TB) *rtfm.Level { return twoPointers(t, nil, pairedUp, sleepy, diamond) },
			[]string{"n", "enter"}, screens.StartTrial{Module: "two-pointers"}},
		{"the stretch once the trial is passed", func(t testing.TB) *rtfm.Level {
			return twoPointers(t, []string{"two-pointers"}, pairedUp, sleepy, diamond)
		}, []string{"n", "enter"}, screens.Upsolve{Problem: stretchID}},
		{"READ once nothing is left", func(t testing.TB) *rtfm.Level {
			return twoPointers(t, []string{"two-pointers"}, pairedUp, sleepy, diamond, stretchID)
		}, []string{"n", "enter"}, screens.OpenModule{Module: "two-pointers"}},
		{"t", trialLevel, []string{"t"}, screens.StartTrial{Module: "two-pointers"}},
		{"enter on TRIAL", trialLevel, []string{"G", "enter"}, screens.StartTrial{Module: "two-pointers"}},
		{"castle t", castleLevel, []string{"t"}, screens.StartTrial{Module: "bronze-conclusion"}},
		{"o on READ", trialLevel, []string{"o"}, reader.OpenURL{URL: "https://usaco.guide/silver/two-pointers"}},
		{"o on a problem", trialLevel, []string{"l", "o"}, reader.OpenURL{URL: "http://www.usaco.org/index.php?page=viewproblem2&cpid=738"}},
		{"o on a locked problem", trialLevel, []string{"l", "l", "o"}, nil},
		{"topped t", toppedLevel, []string{"t"}, nil}, // a page: no trial of its own
		{"page t", pageLevel, []string{"t"}, nil},
		{"page G enter", pageLevel, []string{"G", "enter"}, screens.OpenModule{Module: "fast-io"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMsg(t, press(t, tt.make(t), tt.keys...), tt.want)
		})
	}
	// Leaving is the host's: none of its keys ask to go back.
	for _, k := range []string{"q", "esc", "backspace", "h", "left"} {
		if msg := press(t, trialLevel(t), k); msg != nil {
			t.Errorf("%s emitted %#v, want nothing", k, msg)
		}
	}
}

// The card under the map says why a problem is locked and what it leads
// to.
func TestLevelCard(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(t testing.TB) *rtfm.Level
		keys []string
		want []string
	}{
		{"locked", trialLevel, []string{"l", "l"}, []string{"core", "locked: solve Sleepy Cow Herding first", "leads to the trial"}},
		{"open", trialLevel, []string{"l", "j"}, []string{"open: enter starts it", "leads to Diamond Collector"}},
		{"stretch", trialLevel, []string{"l", "l", "j"}, []string{"stretch", "open"}},
		{"trial", trialLevel, []string{"G"}, []string{"3 unseen doors", "pass 2 of 3", "opens once the core is solved: 1 of 3 so far"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.make(t)
			for _, k := range tc.keys {
				v.Update(key(k))
			}
			got := plain(v.View(140, 40))
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("no %q in\n%s", w, got)
				}
			}
		})
	}
}
