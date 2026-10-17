package rtfm_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

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

// The fixture's levels have no prerequisite or shortcut inside the
// guide, so the trial level is given some: Two Pointers needs Prefix Sums
// (passed) and Casework, and opens Matrix Exponentiation and Counting
// Minimums. The player solved Paired Up and opened the page.
func trialLevel(t testing.TB) *rtfm.Level {
	c := loadCurriculum(t)
	l, region := atlasLevel(t, c, "two-pointers")
	l.Prereqs = []string{"prefix-sums", "casework"}
	l.Warps = []string{"matrix-expo", "count-min"}
	m := fixture.Meta()
	m.Solves = []game.Solve{{Problem: "usaco:738"}}
	m.TrialsPassed = []string{"prefix-sums"}
	m.Opened = []string{"two-pointers"}
	return rtfm.NewLevel(c, l, region, m, fixture.Judgeable, fixture.Lookup)
}

// pageLevel is General's Fast I/O: a page, reading only, its problem one
// to practise with no trial.
func pageLevel(t testing.TB) *rtfm.Level {
	c := loadCurriculum(t)
	l, region := atlasLevel(t, c, "fast-io")
	return rtfm.NewLevel(c, l, region, fixture.Meta(), fixture.Judgeable, fixture.Lookup)
}

// toppedLevel is Counting Minimums: a gated topic with nothing the fixture
// catalog judges, so a page whose capability is earned in another module
// and whose problems are links.
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
	m.Solves = []game.Solve{{Problem: l.Playable[0], Upsolve: true}}
	return rtfm.NewLevel(c, l, region, m, fixture.Judgeable, fixture.Lookup)
}

func TestLevelGoldens(t *testing.T) {
	cases := []struct {
		name string
		make func(t testing.TB) *rtfm.Level
		keys []string
	}{
		{"level_trial", trialLevel, nil},
		{"level_page", pageLevel, []string{"p"}},
		{"level_topped", toppedLevel, []string{"p"}},
		{"level_castle", castleLevel, nil},
		{"level_trial_scrolled", trialLevel, []string{"p", "j", "j", "j", "j", "j", "j", "j", "j"}},
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
		for _, g := range []string{"▸", "✓", "■", "□", "›", "·", "↑", "─", "◆", "★", "☆", "→"} {
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
			v := mk(t)
			v.Practice()
			lines := strings.Split(v.View(w, h), "\n")
			if len(lines) != h {
				t.Errorf("%s at %dx%d: %d lines, want %d", name, w, h, len(lines), h)
			}
			for i, l := range lines {
				if n := ansi.StringWidth(l); n > w {
					t.Errorf("%s at %dx%d: line %d is %d wide: %q", name, w, h, i, n, ansi.Strip(l))
				}
			}
		}
	}
}

// TestLevelIntents: each panel's key asks the app for its act, and a page
// has no trial to start.
func TestLevelIntents(t *testing.T) {
	c := loadCurriculum(t)
	cm, _ := c.Module("count-min")
	tests := []struct {
		name string
		make func(t testing.TB) *rtfm.Level
		keys []string
		want any
	}{
		{"enter on READ", trialLevel, []string{"enter"}, screens.OpenModule{Module: "two-pointers"}},
		{"r", trialLevel, []string{"j", "r"}, screens.OpenModule{Module: "two-pointers"}},
		{"enter on a problem", trialLevel, []string{"j", "enter"}, screens.Upsolve{Problem: "usaco:643"}},
		{"p then enter", trialLevel, []string{"p", "j", "enter"}, screens.Upsolve{Problem: "usaco:918"}},
		{"t", trialLevel, []string{"t"}, screens.StartTrial{Module: "two-pointers"}},
		{"enter on TRIAL", trialLevel, []string{"G", "enter"}, screens.StartTrial{Module: "two-pointers"}},
		{"castle t", castleLevel, []string{"t"}, screens.StartTrial{Module: "bronze-conclusion"}},
		{"o on READ", trialLevel, []string{"o"}, reader.OpenURL{URL: "https://usaco.guide/silver/two-pointers"}},
		{"o on an external problem", toppedLevel, []string{"p", "o"}, reader.OpenURL{URL: cm.Problems[0].Problems[0].URL}},
		{"topped t", toppedLevel, []string{"t"}, nil}, // a page: no trial of its own
		{"page t", pageLevel, []string{"t"}, nil},
		{"page t on its last row", pageLevel, []string{"G", "j", "t"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMsg(t, press(t, tt.make(t), tt.keys...), tt.want)
		})
	}
	// Leaving is the host's: none of its keys ask to go back.
	for _, k := range []string{"q", "esc", "h", "left", "backspace"} {
		if msg := press(t, trialLevel(t), k); msg != nil {
			t.Errorf("%s emitted %#v, want nothing", k, msg)
		}
	}
}

// PRACTICE runs in the order to try it: unsolved before solved, the
// guide's starred (★) first among the unsolved, the stretch (Very Hard,
// Insane) after the rest, links last.
func TestPracticeOrder(t *testing.T) {
	for name, mk := range map[string]func(testing.TB) *rtfm.Level{"trial": trialLevel, "castle": castleLevel, "page": pageLevel} {
		v := mk(t)
		v.Practice()
		lines := strings.Split(plain(v.View(140, 60)), "\n")
		var rows []string
		in := false
		for _, l := range lines {
			switch {
			case strings.Contains(l, "PRACTICE") || strings.Contains(l, "EXAMPLES"):
				in = true
				continue
			case strings.Contains(l, "TRIAL") || strings.Contains(l, "SEE ALSO"):
				in = false
			}
			if in && strings.TrimSpace(l) != "" {
				rows = append(rows, l)
			}
		}
		if len(rows) < 2 {
			t.Fatalf("%s: %d practice rows, too few to order", name, len(rows))
		}
		rank := func(l string) int {
			switch {
			case strings.Contains(l, "link"):
				return 4
			case strings.Contains(l, "Very Hard") || strings.Contains(l, "Insane"):
				return 3
			case strings.Contains(l, ui.G.Pass):
				return 2
			case strings.Contains(l, ui.G.Star):
				return 0
			}
			return 1
		}
		for i := 1; i < len(rows); i++ {
			if rank(rows[i]) < rank(rows[i-1]) {
				t.Errorf("%s: %q comes after %q", name, strings.TrimSpace(rows[i]), strings.TrimSpace(rows[i-1]))
			}
		}
	}
}
