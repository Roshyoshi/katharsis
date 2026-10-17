package rtfm_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/curriculum"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
	"katharsis/internal/screens/rtfm/fixture"
	"katharsis/internal/ui"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

// sizes are the terminal sizes every golden is taken at: the classic 80×24
// and a roomy 140×40.
var sizes = [][2]int{{80, 24}, {140, 40}}

func loadCurriculum(t testing.TB) *curriculum.Curriculum {
	t.Helper()
	c, err := fixture.Curriculum()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func browser(t testing.TB) *rtfm.Browser {
	return rtfm.NewBrowser(loadCurriculum(t), fixture.Meta(), fixture.Judgeable, fixture.Lookup)
}

func module(t testing.TB, id string) *rtfm.Module {
	t.Helper()
	c := loadCurriculum(t)
	m, ok := c.Module(id)
	if !ok {
		t.Fatalf("fixture has no module %q", id)
	}
	return rtfm.NewModule(c, m, fixture.Meta(), fixture.Judgeable, fixture.Lookup)
}

// golden compares the ANSI-stripped view with testdata/<name>.golden.
// Trailing spaces are trimmed per line so goldens survive editors.
func golden(t *testing.T, name, view string) {
	t.Helper()
	got := plain(view)
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden (go test -update to accept):\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

func plain(view string) string {
	lines := strings.Split(ansi.Strip(view), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

// key builds the KeyPressMsg a terminal sends for s ("enter", "ctrl+u",
// "G", "/", ...).
func key(s string) tea.KeyPressMsg {
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab,
		"backspace": tea.KeyBackspace, "up": tea.KeyUp, "down": tea.KeyDown,
		"left": tea.KeyLeft, "right": tea.KeyRight, "home": tea.KeyHome,
		"end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
	}
	if c, ok := named[s]; ok {
		return tea.KeyPressMsg{Code: c}
	}
	if s == "space" {
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	if c, ok := strings.CutPrefix(s, "ctrl+"); ok {
		return tea.KeyPressMsg{Code: rune(c[0]), Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// typed spells s out as single-character key presses, for the / prompt.
func typed(s string) []string {
	var ks []string
	for _, r := range s {
		ks = append(ks, string(r))
	}
	return ks
}

// press feeds keys and returns the message the last one emitted, if any.
func press(t *testing.T, s screens.Screen, keys ...string) tea.Msg {
	t.Helper()
	var msg tea.Msg
	for _, k := range keys {
		var cmd tea.Cmd
		s, cmd = s.Update(key(k))
		msg = nil
		if cmd != nil {
			msg = cmd()
		}
		s.View(80, 24) // as the program would, between keys
	}
	return msg
}

func wantMsg(t *testing.T, got, want tea.Msg) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("emitted %#v, want %#v", got, want)
	}
}

func cat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// ---- goldens ----------------------------------------------------------------

func TestGoldens(t *testing.T) {
	cases := []struct {
		name string
		make func(t *testing.T) screens.Screen
		keys []string
	}{
		{"browser", func(t *testing.T) screens.Screen { return browser(t) }, nil},
		{"browser_folded", func(t *testing.T) screens.Screen { return browser(t) }, []string{"-", "j", "j", "l"}},
		{"browser_filter", func(t *testing.T) screens.Screen { return browser(t) }, cat([]string{"/"}, typed("pointers"), []string{"enter"})},
		{"browser_typing", func(t *testing.T) screens.Screen { return browser(t) }, cat([]string{"/"}, typed("pre"))},
		{"browser_nomatch", func(t *testing.T) screens.Screen { return browser(t) }, cat([]string{"/"}, typed("zzz"), []string{"enter"})},
		{"browser_end", func(t *testing.T) screens.Screen { return browser(t) }, []string{"G"}},
		{"browser_empty", func(t *testing.T) screens.Screen { return rtfm.NewBrowser(nil, game.Meta{}, nil, nil) }, nil},
		{"module_prefix", func(t *testing.T) screens.Screen { return module(t, "prefix-sums") }, nil},
		{"module_held", func(t *testing.T) screens.Screen { return module(t, "two-pointers") }, nil},
		{"module_short", func(t *testing.T) screens.Screen { return module(t, "count-min") }, nil},
		{"module_general", func(t *testing.T) screens.Screen { return module(t, "fast-io") }, nil},
		{"browser_open", func(t *testing.T) screens.Screen { return browser(t) }, []string{"j", "j", "j", "enter"}},
	}
	for _, c := range cases {
		for _, sz := range sizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				s := c.make(t)
				for _, k := range c.keys {
					s, _ = s.Update(key(k))
					s.View(sz[0], sz[1])
				}
				golden(t, name, s.View(sz[0], sz[1]))
			})
		}
	}
}

// ---- width ------------------------------------------------------------------

// TestFits renders every screen in every state the goldens use at sizes
// from cramped to huge: exactly height lines, none wider than width.
func TestFits(t *testing.T) {
	states := map[string]func(t *testing.T) screens.Screen{
		"browser": func(t *testing.T) screens.Screen { return browser(t) },
		"browser end": func(t *testing.T) screens.Screen {
			b := browser(t)
			b.Update(key("G"))
			return b
		},
		"browser folded": func(t *testing.T) screens.Screen {
			b := browser(t)
			b.Update(key("-"))
			return b
		},
		"browser typing": func(t *testing.T) screens.Screen {
			b := browser(t)
			press(t, b, cat([]string{"/"}, typed("prefix"))...)
			return b
		},
		"browser filter": func(t *testing.T) screens.Screen {
			b := browser(t)
			press(t, b, cat([]string{"/"}, typed("o"), []string{"enter"})...)
			return b
		},
		"browser no match": func(t *testing.T) screens.Screen {
			b := browser(t)
			press(t, b, cat([]string{"/"}, typed("zzz"), []string{"enter"})...)
			return b
		},
		"empty browser": func(t *testing.T) screens.Screen { return rtfm.NewBrowser(nil, game.Meta{}, nil, nil) },
		"module":        func(t *testing.T) screens.Screen { return module(t, "prefix-sums") },
		"module scrolled": func(t *testing.T) screens.Screen {
			m := module(t, "prefix-sums")
			m.Update(key("G"))
			return m
		},
		"module general": func(t *testing.T) screens.Screen { return module(t, "fast-io") },
		"module bare": func(t *testing.T) screens.Screen {
			return rtfm.NewModule(nil, curriculum.Module{ID: "x", Frequency: -1}, game.Meta{}, nil, nil)
		},
	}
	for name, mk := range states {
		for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {60, 15}, {80, 24}, {100, 30}, {140, 40}, {220, 60}} {
			w, h := sz[0], sz[1]
			s := mk(t)
			lines := strings.Split(s.View(w, h), "\n")
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

// ---- browser keys -----------------------------------------------------------

// The fixture index, top to bottom with everything open:
//
//	0 GENERAL  1 Language-Specific  2 Fast Input & Output
//	3 BRONZE   4 Additional         5 Casework
//	           6 Conclusion         7 Additional Practice for USACO Bronze
//	8 SILVER   9 Prefix Sums        10 Introduction to Prefix Sums
//	           11 Sorting & Searching 12 Two Pointers
//	13 GOLD    14 Additional Topics 15 (Optional) Hashmaps
//	16 PLATINUM 17 Misc. Topics     18 Matrix Exponentiation
//	19 ADVANCED 20 Data Structures  21 Counting Minimums with Segment Tree
//
// The cursor starts on row 2, the first module.

func TestBrowserOpensModules(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want tea.Msg
	}{
		{"enter opens the first module; r reads it", []string{"enter", "r"}, screens.OpenModule{Module: "fast-io"}},
		{"j moves to the next module", []string{"j", "j", "j", "enter", "r"}, screens.OpenModule{Module: "casework"}},
		{"G is the last module", []string{"G", "enter", "t"}, screens.StartTrial{Module: "count-min"}},
		{"g then down", []string{"g", "j", "j", "j", "j", "j", "j", "j", "j", "j", "j", "l", "t"}, screens.StartTrial{Module: "prefix-sums"}},
		{"o opens the module online", []string{"j", "j", "j", "enter", "o"},
			reader.OpenURL{URL: "https://usaco.guide/bronze/casework"}},
		{"filter then open", cat([]string{"/"}, typed("matrix"), []string{"enter", "enter", "r"}),
			screens.OpenModule{Module: "matrix-expo"}},
		{"filter matches ids", cat([]string{"/"}, typed("count-min"), []string{"enter", "enter", "t"}),
			screens.StartTrial{Module: "count-min"}},
		{"filter is case-insensitive", cat([]string{"/"}, typed("HASH"), []string{"enter", "enter", "t"}),
			screens.StartTrial{Module: "hashmaps"}},
		{"enter on a heading folds, it does not open", []string{"k", "enter", "r"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMsg(t, press(t, browser(t), tt.keys...), tt.want)
		})
	}
}

func TestBrowserBack(t *testing.T) {
	wantMsg(t, press(t, browser(t), "q"), screens.Back{})
	wantMsg(t, press(t, browser(t), "esc"), screens.Back{})
	// esc first clears a filter, then leaves.
	wantMsg(t, press(t, browser(t), cat([]string{"/"}, typed("two"), []string{"enter", "esc"})...), nil)
	wantMsg(t, press(t, browser(t), cat([]string{"/"}, typed("two"), []string{"enter", "esc", "esc"})...), screens.Back{})
	// q, esc and h on a module page return to the index, not out of it.
	for _, k := range []string{"q", "esc", "h", "left"} {
		b := browser(t)
		wantMsg(t, press(t, b, "enter", k), nil)
		if v := plain(b.View(80, 24)); !strings.Contains(v, "RTFM(7)") {
			t.Errorf("%s on the page should show the index again:\n%s", k, v)
		}
		wantMsg(t, press(t, b, "q"), screens.Back{})
	}
}

func TestBrowserFilter(t *testing.T) {
	b := browser(t)
	press(t, b, cat([]string{"/"}, typed("pointers"))...)
	if !b.Typing() {
		t.Fatal("the / prompt should be open")
	}
	v := plain(b.View(80, 24))
	for _, want := range []string{"Two Pointers", "SILVER", "Sorting & Searching", "/pointers█", "1 match"} {
		if !strings.Contains(v, want) {
			t.Errorf("filtering for pointers: missing %q:\n%s", want, v)
		}
	}
	for _, absent := range []string{"Prefix Sums", "GOLD", "Casework"} {
		if strings.Contains(v, absent) {
			t.Errorf("filtering for pointers still shows %q:\n%s", absent, v)
		}
	}

	press(t, b, "enter")
	if b.Typing() {
		t.Error("enter should close the prompt")
	}
	if v := plain(b.View(80, 24)); !strings.Contains(v, "/pointers: 1 match") {
		t.Errorf("committed filter not shown:\n%s", v)
	}

	// esc clears the filter and keeps the found module selected, in its
	// place in the full index.
	press(t, b, "esc")
	v = plain(b.View(80, 24))
	if !strings.Contains(v, "Casework") {
		t.Errorf("esc should clear the filter:\n%s", v)
	}
	wantMsg(t, press(t, b, "enter", "t"), screens.StartTrial{Module: "two-pointers"})
}

func TestBrowserFilterCancel(t *testing.T) {
	// esc at the prompt drops the half-typed filter and the selection
	// returns to where it was before /.
	b := browser(t)
	wantMsg(t, press(t, b, cat([]string{"j", "j", "j", "/"}, typed("matrix"), []string{"esc", "enter", "r"})...),
		screens.OpenModule{Module: "casework"})
	// Backspace edits the live filter.
	b = browser(t)
	press(t, b, cat([]string{"/"}, typed("prefixx"))...)
	if v := plain(b.View(80, 24)); !strings.Contains(v, "0 matches") {
		t.Errorf("prefixx should match nothing:\n%s", v)
	}
	press(t, b, "backspace")
	if v := plain(b.View(80, 24)); !strings.Contains(v, "Introduction to Prefix Sums") {
		t.Errorf("backspace should widen the filter back to prefix:\n%s", v)
	}
}

// An empty / (opened, then closed with esc or enter) leaves the selection
// where it is, even after an earlier filter moved it.
func TestBrowserEmptyFilterKeepsSelection(t *testing.T) {
	for _, k := range []string{"esc", "enter"} {
		wantMsg(t, press(t, browser(t), "j", "j", "j", "/", k, "enter", "r"),
			screens.OpenModule{Module: "casework"})
	}
	b := browser(t)
	press(t, b, cat([]string{"/"}, typed("two"), []string{"enter", "esc"})...)
	wantMsg(t, press(t, b, "/", "esc", "enter", "t"), screens.StartTrial{Module: "two-pointers"})
}

func TestBrowserFolds(t *testing.T) {
	b := browser(t)
	// h on a module folds its chapter and moves to the chapter heading.
	press(t, b, "h")
	v := plain(b.View(80, 24))
	if strings.Contains(v, "Fast Input & Output") || !strings.Contains(v, "▹ Language-Specific  · 1 module") {
		t.Errorf("h should fold Language-Specific:\n%s", v)
	}
	// h again steps out to the division; h there folds it.
	press(t, b, "h", "h")
	v = plain(b.View(80, 24))
	if strings.Contains(v, "Language-Specific") || !strings.Contains(v, "▹ GENERAL") {
		t.Errorf("h, h should fold GENERAL:\n%s", v)
	}
	// l unfolds the division, l unfolds... the chapter is still folded.
	press(t, b, "l", "j", "l", "j", "enter")
	// fast-io is a General module: no topic, so no trial to start.
	wantMsg(t, press(t, b, "t"), nil)
	wantMsg(t, press(t, b, "r"), screens.OpenModule{Module: "fast-io"})

	// - folds every chapter; + opens them again.
	b = browser(t)
	press(t, b, "-")
	v = plain(b.View(80, 24))
	for _, hidden := range []string{"Casework", "Two Pointers", "Matrix Exponentiation"} {
		if strings.Contains(v, hidden) {
			t.Errorf("- should hide %q:\n%s", hidden, v)
		}
	}
	if !strings.Contains(v, "ADVANCED") {
		t.Errorf("- should leave divisions open:\n%s", v)
	}
	press(t, b, "+")
	if v := plain(b.View(140, 40)); !strings.Contains(v, "Matrix Exponentiation") {
		t.Errorf("+ should unfold everything:\n%s", v)
	}

	// tab toggles the heading under the cursor.
	b = browser(t)
	press(t, b, "k", "tab")
	if v := plain(b.View(80, 24)); strings.Contains(v, "Fast Input") {
		t.Errorf("tab on Language-Specific should fold it:\n%s", v)
	}
	press(t, b, "tab")
	if v := plain(b.View(80, 24)); !strings.Contains(v, "Fast Input") {
		t.Errorf("second tab should unfold it:\n%s", v)
	}

	// A filter shows matches inside folded chapters.
	b = browser(t)
	press(t, b, cat([]string{"-", "/"}, typed("casework"), []string{"enter"})...)
	if v := plain(b.View(80, 24)); !strings.Contains(v, "Casework") {
		t.Errorf("filter should reach into folded chapters:\n%s", v)
	}
}

// Open is a link into a module's page (a locked exit's t, the next step
// after a boot or a trial): q and esc go back where it came from, h to the
// index, at the module's row; a page opened from the index goes back to it.
func TestBrowserOpenByID(t *testing.T) {
	b := browser(t)
	press(t, b, "-")
	if !b.Open("matrix-expo") {
		t.Fatal("Open(matrix-expo) = false")
	}
	if v := plain(b.View(80, 24)); !strings.Contains(v, "matrix-expo - Matrix Exponentiation") {
		t.Errorf("Open should show the module page:\n%s", v)
	}
	for _, k := range []string{"q", "esc"} {
		wantMsg(t, press(t, b, k), screens.Back{})
	}
	press(t, b, "h")
	if v := plain(b.View(80, 24)); !strings.Contains(v, "Matrix Exponentiation") || strings.Contains(v, "matrix-expo - Matrix Exponentiation") {
		t.Errorf("h should land on the module's row, unfolded:\n%s", v)
	}
	wantMsg(t, press(t, b, "enter", "r"), screens.OpenModule{Module: "matrix-expo"})
	if got := press(t, b, "q"); got != nil {
		t.Errorf("q on a page opened from the index emitted %#v, want the index", got)
	}
	if b.Open("no-such-module") {
		t.Error("Open(no-such-module) = true")
	}
}

// ---- capability marks -------------------------------------------------------

func TestCapabilityMarks(t *testing.T) {
	two := curriculum.CapabilityFor("two-pointers")
	pre := curriculum.CapabilityFor("prefix-sums")
	if two == "" || pre == "" {
		t.Skip("capability table gates neither prefix sums nor two pointers")
	}
	b := browser(t)
	v := plain(b.View(140, 40))
	for _, want := range []string{"■ " + two, "□ " + pre} {
		if !strings.Contains(v, want) {
			t.Errorf("index lacks %q:\n%s", want, v)
		}
	}
	// Refresh: passing prefix sums flips its mark, here and on its page.
	press(t, b, "j", "j", "j", "j", "j", "j", "j", "j", "enter")
	b.Refresh(game.Meta{Capabilities: []string{two, pre}})
	if v := plain(b.View(140, 40)); !strings.Contains(v, "■  "+pre+" held") {
		t.Errorf("open page should show %s held after Refresh:\n%s", pre, v)
	}
	press(t, b, "q")
	if v := plain(b.View(140, 40)); strings.Contains(v, "□ "+pre) {
		t.Errorf("index should show %s held after Refresh:\n%s", pre, v)
	}
	// Modules teaching no gated topic show no capability at all.
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "Casework") && strings.ContainsAny(l, "■□") {
			t.Errorf("casework teaches no gated topic, but shows a mark: %q", l)
		}
	}
}

// ---- module page ------------------------------------------------------------

func TestModuleKeys(t *testing.T) {
	m := func() screens.Screen { return module(t, "prefix-sums") }
	wantMsg(t, press(t, m(), "r"), screens.OpenModule{Module: "prefix-sums"})
	wantMsg(t, press(t, m(), "enter"), screens.OpenModule{Module: "prefix-sums"})
	wantMsg(t, press(t, m(), "t"), screens.StartTrial{Module: "prefix-sums"})
	wantMsg(t, press(t, m(), "o"), reader.OpenURL{URL: "https://usaco.guide/silver/prefix-sums"})
	wantMsg(t, press(t, m(), "q"), screens.Back{})
	wantMsg(t, press(t, m(), "esc"), screens.Back{})
	wantMsg(t, press(t, m(), "x"), nil)
	// No URL, no o.
	bare := rtfm.NewModule(nil, curriculum.Module{ID: "x", Frequency: -1}, game.Meta{}, nil, nil)
	wantMsg(t, press(t, bare, "o"), nil)
	// Casework grants no capability, and the Bronze conclusion is a castle;
	// both have a trial that clears the level only, so t starts it and the
	// pass line says so.
	for _, id := range []string{"casework", "bronze-conclusion"} {
		p := module(t, id)
		wantMsg(t, press(t, p, "t"), screens.StartTrial{Module: id})
		v := plain(p.View(140, 40))
		if !strings.Contains(v, "start trial") || !strings.Contains(v, "pass → ✓ only: grants no capability") {
			t.Errorf("%s does not offer its trial, or misstates the pass:\n%s", id, v)
		}
	}
	// Fast I/O is General's: a page, with no trial to start.
	page := module(t, "fast-io")
	wantMsg(t, press(t, page, "t"), nil)
	if v := plain(page.View(140, 40)); strings.Contains(v, "start trial") || !strings.Contains(v, "no trial: General is reading only") {
		t.Errorf("fast-io offers a trial it cannot start:\n%s", v)
	}
}

func TestModulePlan(t *testing.T) {
	v := plain(module(t, "prefix-sums").View(140, 40))
	// The unit in trial order, with guide labels, fixture ratings and
	// contests; then integration doors in the module's biome and band.
	for _, want := range []string{
		"1  Breed Counting", "Very Easy", "◆1250", "2015 December, Silver P3",
		"4  Farmer John's Favorite Operation",
		"+  2 blind integration doors · The Accumulator · ◆1350–1650",
		"“Introduction to Prefix Sums” by Darren Yao, Dustin Miao, USACO Guide",
		"RTFM › Silver › Prefix Sums",
		"requires  nothing",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("prefix-sums page lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "topped up") {
		t.Errorf("prefix-sums has a full unit; nothing is topped up:\n%s", v)
	}

	v = plain(module(t, "hashmaps").View(140, 40))
	for _, want := range []string{"1  Farmer John Solves 3SUM", "+  3 topped up from the catalog · The Heap · ◆1700–2000"} {
		if !strings.Contains(v, want) {
			t.Errorf("hashmaps page lacks %q:\n%s", want, v)
		}
	}
}

func TestModuleScrolls(t *testing.T) {
	m := module(t, "prefix-sums")
	top := plain(m.View(80, 12))
	if !strings.Contains(top, "line 1/") || !strings.Contains(top, "NAME") {
		t.Fatalf("a 12-row page should start at the top with a ruler:\n%s", top)
	}
	m.Update(key("G")) // not press: it renders at 80x24, where all fits
	bot := plain(m.View(80, 12))
	if !strings.Contains(bot, "(END)") || !strings.Contains(bot, "CC BY-NC-SA") {
		t.Errorf("G should reach the attribution:\n%s", bot)
	}
	m.Update(key("g"))
	if v := plain(m.View(80, 12)); v != top {
		t.Errorf("g should return to the top:\n%s", v)
	}
}

// A module page has the Mock Turtle's line between its description and
// TRIAL: one of rtfm.module.intro's, the title in it cut to 24 cells, and
// the same pick on every opening.
func TestModuleIntroLine(t *testing.T) {
	c := loadCurriculum(t)
	for _, id := range []string{"prefix-sums", "count-min", "fast-io"} {
		m, ok := c.Module(id)
		if !ok {
			t.Fatalf("fixture has no module %q", id)
		}
		var lines []string
		for seed := range uint64(8) {
			l := flavor.Fill(flavor.Story("rtfm.module.intro", seed), "module", ui.Clip(m.Title, 24))
			if !slices.Contains(lines, l) {
				lines = append(lines, l)
			}
		}
		v := plain(module(t, id).View(140, 40))
		var found []string
		for _, l := range lines {
			if strings.Contains(v, "       "+l+"\n") {
				found = append(found, l)
			}
		}
		trial := strings.Index(v, "\nTRIAL")
		if len(found) != 1 || strings.Index(v, found[0]) > trial || strings.Index(v, m.Description) > strings.Index(v, found[0]) {
			t.Errorf("%s: want one intro line under the description, got %q:\n%s", id, found, v)
			continue
		}
		if again := plain(module(t, id).View(140, 40)); again != v {
			t.Errorf("%s: a second opening picks another line", id)
		}
	}
}

// Past the last module the index scrolls on through its tail to the
// colophon, as man's does, with the last module still selected; scrolled
// back until it shows, up moves the selection again. The terminal is as
// tall as the colophon (tailH), so the last module goes off screen in the
// tail.
func TestBrowserScrollsTail(t *testing.T) {
	full := strings.Split(plain(browser(t).View(80, 200)), "\n")
	from := slices.Index(full, "COLOPHON")
	to := slices.IndexFunc(full, func(l string) bool { return strings.HasPrefix(l, "katharsis ") })
	if from < 0 || to < from {
		t.Fatalf("no colophon:\n%s", strings.Join(full, "\n"))
	}
	tailH := to - from + 1 + 3 // the man header, the column header, the bar
	// tap is press on a terminal tailH tall: press views at 80x24.
	tap := func(b *rtfm.Browser, keys ...string) tea.Msg {
		var msg tea.Msg
		for _, k := range keys {
			var cmd tea.Cmd
			msg = nil
			if _, cmd = b.Update(key(k)); cmd != nil {
				msg = cmd()
			}
			b.View(80, tailH)
		}
		return msg
	}
	line := func(b *rtfm.Browser) int {
		v := plain(b.View(80, tailH))
		i := strings.LastIndex(v, " line ")
		var n int
		if _, err := fmt.Sscanf(v[i:], " line %d/", &n); err != nil {
			t.Fatalf("no pager line: %v\n%s", err, v)
		}
		return n
	}
	b := browser(t)
	tap(b, "G")
	end := line(b)
	if v := plain(b.View(80, tailH)); !strings.Contains(v, "COLOPHON") || !strings.Contains(v, "(END)") {
		t.Fatalf("G does not reach the colophon:\n%s", v)
	}
	tap(b, "k")
	if got := line(b); got != end-1 {
		t.Errorf("k in the tail: line %d, want %d", got, end-1)
	}
	tap(b, "b")
	v := plain(b.View(80, tailH))
	if rows := strings.Split(v, "\n"); !strings.Contains(rows[2], "Counting Minimums") {
		t.Fatalf("paging back stops on the last module:\n%s", v)
	}
	at := line(b)
	tap(b, "j")
	if got := line(b); got != at+1 {
		t.Errorf("j on the last module: line %d, want %d", got, at+1)
	}
	wantMsg(t, tap(b, "enter", "t"), screens.StartTrial{Module: "count-min"})

	b = browser(t)
	tap(b, "G", "b", "k", "enter") // the chapter over it, folded
	if v := plain(b.View(80, tailH)); strings.Contains(v, "Counting Minimums") {
		t.Errorf("k on screen should select the chapter, and enter fold it:\n%s", v)
	}
}

// The index leaves out the guide's pages about itself (UX J21): Using
// This Guide and the Contributing chapter, which would be a chapter of
// nothing. The cursor starts on the first module left.
func TestBrowserHidesMetaModules(t *testing.T) {
	dir := t.TempDir()
	mod := func(id, title string, prereqs ...string) string {
		s := "---\nid: " + id + "\ntitle: " + title + "\nauthor: A\ndescription: d\n"
		if len(prereqs) > 0 {
			s += "prerequisites:\n  - " + strings.Join(prereqs, "\n  - ") + "\n"
		}
		return s + "---\n\nHi.\n"
	}
	for name, body := range map[string]string{
		"content/ordering.ts": `const MODULE_ORDERING = {
  general: [
    { name: 'Getting Started', items: ['using-this-guide', 'intro-cp'] },
    { name: 'Contributing', items: ['modules', 'working-mdx'] },
  ],
  bronze: [{ name: 'Simulation', items: ['intro-sim'] }],
};
`,
		"content/1_General/Using_This_Guide.mdx": mod("using-this-guide", "Using This Guide"),
		"content/1_General/Intro_CP.mdx":         mod("intro-cp", "Introduction to Competitive Programming"),
		"content/1_General/Modules.mdx":          mod("modules", "Introducing Modules", "using-this-guide"),
		"content/1_General/Working_MDX.mdx":      mod("working-mdx", "Working With MDX", "modules"),
		"content/2_Bronze/Intro_Sim.mdx":         mod("intro-sim", "Simulation", "intro-cp"),
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := curriculum.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	b := rtfm.NewBrowser(c, game.Meta{}, nil, nil)
	v := plain(b.View(80, 24))
	for _, hidden := range []string{"Using This Guide", "Contributing", "Introducing Modules", "Working With MDX"} {
		if strings.Contains(v, hidden) {
			t.Errorf("index shows %q:\n%s", hidden, v)
		}
	}
	if !strings.Contains(v, "GENERAL") || !strings.Contains(v, "1 module") || !strings.Contains(v, "Simulation") {
		t.Errorf("index lacks the modules left:\n%s", v)
	}
	wantMsg(t, press(t, b, "enter", "r"), screens.OpenModule{Module: "intro-cp"})
}

// A passed trial that grants no capability says so on its page, where it
// asked to be passed before.
func TestModulePassedCheckOnly(t *testing.T) {
	c := loadCurriculum(t)
	mod, ok := c.Module("casework")
	if !ok {
		t.Fatal("fixture has no casework")
	}
	before := ansi.Strip(rtfm.NewModule(c, mod, game.Meta{}, nil, fixture.Lookup).View(100, 40))
	after := ansi.Strip(rtfm.NewModule(c, mod, game.Meta{TrialsPassed: []string{"casework"}}, nil, fixture.Lookup).View(100, 40))
	if !strings.Contains(before, "pass → ✓ only") || strings.Contains(before, "✓  passed") {
		t.Errorf("before passing:\n%s", before)
	}
	if !strings.Contains(after, "✓  passed: grants no capability") {
		t.Errorf("after passing:\n%s", after)
	}
}
