package meta_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/meta/fixture"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

// sizes are the terminal sizes every golden is taken at: the classic
// 80×24 and a roomy 140×40.
var sizes = [][2]int{{80, 24}, {140, 40}}

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

// key builds the KeyPressMsg a terminal sends for s ("enter", "ctrl+x",
// "G", "8", ...).
func key(s string) tea.KeyPressMsg {
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab,
		"backspace": tea.KeyBackspace, "up": tea.KeyUp, "down": tea.KeyDown,
		"left": tea.KeyLeft, "right": tea.KeyRight, "home": tea.KeyHome,
		"end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
		"f10": tea.KeyF10,
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

func grub() *meta.Menu { return meta.NewGrub(fixture.GrubConfig()) }

// ---- goldens -----------------------------------------------------------------

func TestGoldens(t *testing.T) {
	cases := []struct {
		name string
		make func() screens.Screen
		keys []string
	}{
		{"grub", func() screens.Screen { return grub() }, nil},
		{"grub_presets", func() screens.Screen { return grub() }, []string{"down", "down", "down", "down", "down", "enter"}},
		{"grub_regressions", func() screens.Screen { return grub() }, []string{"down", "down", "down", "down", "down", "down", "down", "enter"}},
		{"grub_edit", func() screens.Screen { return grub() }, []string{"e", "down", "tab", "down", "8", "f", "3", "a"}},
		{"log", func() screens.Screen { return meta.NewLog(fixture.Boots(), fixture.PrestigeName) }, nil},
		{"log_expanded", func() screens.Screen { return meta.NewLog(fixture.Boots(), fixture.PrestigeName) }, []string{"enter"}},
		{"log_crashed", func() screens.Screen { return meta.NewLog(fixture.Boots(), fixture.PrestigeName) }, []string{"f", "f", "j", "enter"}},
		{"top", func() screens.Screen { return meta.NewTop(fixture.Stats()) }, nil},
		{"top_sorted", func() screens.Screen { return meta.NewTop(fixture.Stats()) }, []string{"s", "j", "j"}},
		{"shop", func() screens.Screen { return meta.NewShop(fixture.ShopConfig()) }, []string{"j", "j"}},
	}
	for _, c := range cases {
		for _, sz := range sizes {
			t.Run(fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1]), func(t *testing.T) {
				s := c.make()
				for _, k := range c.keys {
					s, _ = s.Update(key(k))
					s.View(sz[0], sz[1])
				}
				golden(t, fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1]), s.View(sz[0], sz[1]))
			})
		}
	}
}

// ---- width -------------------------------------------------------------------

// TestFits renders every screen in every state the goldens use at sizes
// from cramped to huge: exactly height lines, none wider than width.
func TestFits(t *testing.T) {
	screensUnder := map[string]func() screens.Screen{
		"grub": func() screens.Screen { return grub() },
		"edit": func() screens.Screen { s := grub(); s.Update(key("e")); return s },
		"regressions": func() screens.Screen {
			s := grub()
			for _, k := range []string{"G", "up", "up", "up", "enter"} {
				s.Update(key(k))
			}
			return s
		},
		"log": func() screens.Screen { return meta.NewLog(fixture.Boots(), fixture.PrestigeName) },
		"log expanded": func() screens.Screen {
			s := meta.NewLog(fixture.Boots(), fixture.PrestigeName)
			s.Update(key("enter"))
			return s
		},
		"empty log": func() screens.Screen { return meta.NewLog(nil, nil) },
		"top":       func() screens.Screen { return meta.NewTop(fixture.Stats()) },
		"empty top": func() screens.Screen { return meta.NewTop(meta.Stats{}) },
	}
	for name, mk := range screensUnder {
		for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {60, 15}, {80, 24}, {100, 30}, {140, 40}, {220, 60}} {
			w, h := sz[0], sz[1]
			s := mk()
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

// ---- GRUB keys ---------------------------------------------------------------

func TestGrubEntries(t *testing.T) {
	// Entries top to bottom: prestige 2, 1, 0, nightly build, RTFM, custom
	// preset…, core dump, regressions due, git log, top, credits, poweroff.
	down := func(n int) []string {
		ks := make([]string, n)
		for i := range ks {
			ks[i] = "down"
		}
		return ks
	}
	tests := []struct {
		name string
		keys []string
		want tea.Msg
	}{
		{"default boots highest prestige", []string{"enter"}, screens.StartBoot{Prestige: 2, Preset: "standard"}},
		{"bronze", append(down(2), "enter"), screens.StartBoot{Prestige: 0, Preset: "standard"}},
		{"nightly", append(down(3), "enter"), screens.StartBoot{Prestige: 2, Preset: "standard", Seed: 0x20261014}},
		{"rtfm", append(down(4), "enter"), screens.OpenLearn{}},
		{"preset submenu", append(down(5), "enter", "down", "enter"), screens.StartBoot{Prestige: 2, Preset: "speedrun"}},
		{"core dump", append(down(6), "enter"), screens.Upsolve{Problem: "usaco:1014"}},
		// The core dump has its own entry; the submenu is regressions only,
		// oldest due first, without usaco:740, which is not due.
		{"regressions submenu lists only regressions", append(down(7), "enter", "enter"), screens.Upsolve{Problem: "usaco:891"}},
		{"regression due last", append(down(7), "enter", "G", "enter"), screens.Upsolve{Problem: "usaco:1205"}},
		{"apt", append(down(8), "enter"), screens.OpenShop{}},
		{"git log", append(down(9), "enter"), screens.OpenLog{}},
		{"top", append(down(10), "enter"), screens.OpenTop{}},
		{"credits", append(down(11), "enter"), screens.OpenCredits{}},
		{"poweroff", append(down(12), "enter"), screens.Quit{}},
		{"end is poweroff", []string{"G", "enter"}, screens.Quit{}},
		{"down stops at the end", append(down(30), "enter"), screens.Quit{}},
		{"ctrl+x boots like enter", []string{"ctrl+x"}, screens.StartBoot{Prestige: 2, Preset: "standard"}},
		{"esc leaves submenu, keeping its entry highlighted", append(down(5), "enter", "esc", "down", "enter"), screens.Upsolve{Problem: "usaco:1014"}},
		{"esc at root does nothing", []string{"esc"}, nil},
		{"e on a non-boot entry does nothing", append(down(9), "e", "enter"), screens.OpenLog{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMsg(t, press(t, grub(), tt.keys...), tt.want)
		})
	}
}

func TestGrubEdit(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want tea.Msg
	}{
		{"unchanged", []string{"e", "ctrl+x"}, screens.StartBoot{Prestige: 2, Preset: "standard"}},
		{"seed", []string{"e", "down", "down", "8", "f", "3", "a", "f10"},
			screens.StartBoot{Prestige: 2, Preset: "standard", Seed: 0x8f3a}},
		{"seed ignores non-hex", []string{"e", "down", "down", "1", "x", "z", "2", "enter"},
			screens.StartBoot{Prestige: 2, Preset: "standard", Seed: 0x12}},
		{"seed backspace", []string{"e", "down", "down", "a", "b", "backspace", "enter"},
			screens.StartBoot{Prestige: 2, Preset: "standard", Seed: 0xa}},
		{"preset and prestige", []string{"e", "down", "tab", "tab", "up", "left", "enter"},
			screens.StartBoot{Prestige: 1, Preset: "speedrun"}},
		{"prestige wraps within unlocked", []string{"e", "right", "enter"},
			screens.StartBoot{Prestige: 0, Preset: "standard"}},
		{"from a preset entry", []string{"down", "down", "down", "down", "down", "enter", "e", "ctrl+x"},
			screens.StartBoot{Prestige: 2, Preset: "hardcore"}},
		{"nightly keeps its seed", []string{"down", "down", "down", "e", "ctrl+x"},
			screens.StartBoot{Prestige: 2, Preset: "standard", Seed: 0x20261014}},
		{"esc returns to the menu", []string{"e", "down", "down", "7", "esc", "enter"},
			screens.StartBoot{Prestige: 2, Preset: "standard"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMsg(t, press(t, grub(), tt.keys...), tt.want)
		})
	}
}

func TestGrubHidesWhatIsNotThere(t *testing.T) {
	c := fixture.GrubConfig()
	c.Meta = game.Meta{Rating: 1000}
	c.Presets = nil
	got := plain(meta.NewGrub(c).View(80, 24))
	for _, absent := range []string{"custom preset", "core dump", "regressions", "prestige 1"} {
		if strings.Contains(got, absent) {
			t.Errorf("fresh player's menu shows %q:\n%s", absent, got)
		}
	}
	for _, present := range []string{"*katharsis, prestige 0 (Bronze)", "katharsis (nightly build 2026-10-14)", "git log", "top", "poweroff", "bank 0 B"} {
		if !strings.Contains(got, present) {
			t.Errorf("fresh player's menu lacks %q:\n%s", present, got)
		}
	}
	// A fresh player's nightly boots the only prestige there is.
	wantMsg(t, press(t, meta.NewGrub(c), "down", "enter"), screens.StartBoot{Prestige: 0, Preset: "standard", Seed: 0x20261014})
}

// A source still loading says so on the status line, and the line follows
// it as it moves without the menu being rebuilt.
func TestGrubSourcesNote(t *testing.T) {
	c := fixture.GrubConfig()
	note := "Codeforces indexing… 40%"
	c.Sources = func() string { return note }
	g := meta.NewGrub(c)
	if s := plain(g.View(80, 24)); !strings.Contains(s, "next boot #") || !strings.Contains(s, "· Codeforces indexing… 40%") {
		t.Errorf("status line lacks the note:\n%s", s)
	}
	note = ""
	if s := plain(g.View(80, 24)); strings.Contains(s, "Codeforces") {
		t.Errorf("a source that joined is still noted:\n%s", s)
	}
}

func TestNightlySeed(t *testing.T) {
	pdt := time.FixedZone("PDT", -7*3600)
	day := time.Date(2026, 10, 14, 0, 0, 0, 0, pdt)
	for _, tc := range []struct {
		t    time.Time
		want uint64
	}{
		{day, 0x20261014},
		{day.Add(23*time.Hour + 59*time.Minute), 0x20261014},
		{day.Add(24 * time.Hour), 0x20261015},
		// The date is the one in t's location: 23:30 PDT is already the
		// 15th in UTC, but the player's nightly is still the 14th's.
		{time.Date(2026, 10, 14, 23, 30, 0, 0, pdt), 0x20261014},
		{time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), 0x20270101},
	} {
		if got := meta.NightlySeed(tc.t); got != tc.want {
			t.Errorf("NightlySeed(%v) = %#x, want %#x", tc.t, got, tc.want)
		}
	}
}

func TestGrubBannerWhenItFits(t *testing.T) {
	c := fixture.GrubConfig()
	c.Banner = func(string) string { return "== BANNER ==" }
	g := meta.NewGrub(c)
	// SPONGE: 80x24 lost the banner when apt, credits and the prestige
	//         gate line all joined GRUB; the visual pass (C part 2, P4)
	//         decides whether credits leaves the menu to win it back.
	if s := plain(g.View(80, 25)); !strings.Contains(s, "== BANNER ==") {
		t.Errorf("banner missing at 80x25:\n%s", s)
	}
	if s := plain(g.View(80, 14)); strings.Contains(s, "== BANNER ==") {
		t.Errorf("banner should give way to entries at 80x14:\n%s", s)
	}
}

// TestGrubFlavorBanner uses flavor's real art, the default and each apt
// theme's, which carries its own version line: it must replace the
// header, not repeat it, and its lines must share one left edge.
func TestGrubFlavorBanner(t *testing.T) {
	themes := []string{""}
	for _, p := range rules.Standard().Packages {
		if p.Theme != "" {
			themes = append(themes, p.Theme)
		}
	}
	for _, theme := range themes {
		c := fixture.GrubConfig()
		c.Banner, c.Meta.Theme = nil, theme
		s := plain(meta.NewGrub(c).View(80, 30))
		if n := strings.Count(s, "GNU GRUB"); n != 1 {
			t.Errorf("theme %q: want one GNU GRUB line, got %d:\n%s", theme, n, s)
		}
		kind := "grub"
		if theme != "" {
			kind = "grub:" + theme
		}
		art := strings.Split(flavor.Banner(kind), "\n")
		if len(art) < 2 {
			t.Errorf("flavor has no multi-line %s banner", kind)
			continue
		}
		lines := strings.Split(s, "\n")
		var offs []int
		for _, a := range art {
			for _, l := range lines {
				if a != "" && strings.TrimLeft(l, " ") == strings.TrimLeft(a, " ") {
					offs = append(offs, len(l)-len(a))
					break
				}
			}
		}
		if len(offs) != len(art) {
			t.Errorf("theme %q: banner lines missing:\n%s", theme, s)
			continue
		}
		for _, o := range offs {
			if o != offs[0] {
				t.Errorf("theme %q: banner lines shifted relative to each other (%v):\n%s", theme, offs, s)
				break
			}
		}
	}
}

// ---- git log and top keys -----------------------------------------------------

func TestLogKeys(t *testing.T) {
	l := func() screens.Screen { return meta.NewLog(fixture.Boots(), fixture.PrestigeName) }
	wantMsg(t, press(t, l(), "q"), screens.Back{})
	wantMsg(t, press(t, l(), "esc"), screens.Back{})

	s := l()
	press(t, s, "enter")
	if v := plain(s.View(80, 24)); !strings.Contains(v, "Wormhole Sort") {
		t.Errorf("enter should expand boot #42's attempts:\n%s", v)
	}
	press(t, s, "enter")
	if v := plain(s.View(80, 24)); strings.Contains(v, "Wormhole Sort") {
		t.Errorf("second enter should collapse:\n%s", v)
	}

	press(t, s, "f") // cleared only
	v := plain(s.View(80, 24))
	if !strings.Contains(v, "filter: cleared") || strings.Contains(v, "crashed in") {
		t.Errorf("cleared filter shows crashes:\n%s", v)
	}
	press(t, s, "F", "F") // back past all to :q!
	if v := plain(s.View(80, 24)); !strings.Contains(v, "filter: :q!") || strings.Contains(v, "(timer)") {
		t.Errorf(":q! filter:\n%s", v)
	}
}

func TestLogScrollsToSelection(t *testing.T) {
	s := meta.NewLog(fixture.Boots(), nil)
	press(t, s, "G")
	if v := plain(s.View(80, 24)); !strings.Contains(v, "#7 ") {
		t.Errorf("G should scroll to the oldest boot:\n%s", v)
	}
	press(t, s, "g")
	if v := plain(s.View(80, 24)); !strings.Contains(v, "#42") {
		t.Errorf("g should scroll back to the newest:\n%s", v)
	}
}

func TestTopKeys(t *testing.T) {
	wantMsg(t, press(t, meta.NewTop(fixture.Stats()), "q"), screens.Back{})
	s := meta.NewTop(fixture.Stats())
	// first is the topic on the row under the table header.
	first := func() string {
		lines := strings.Split(plain(s.View(80, 24)), "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, "  TOPIC") && i+1 < len(lines) {
				return strings.Fields(lines[i+1])[0]
			}
		}
		return ""
	}
	// RATING, ATT, CLR% (simulation 23/26 beats prefix-sums 12/14), TOPIC.
	order := []string{"dp", "sorting-greedy", "simulation", "binary-search"}
	for i, want := range order {
		if got := first(); got != want {
			t.Errorf("sort %d: first topic %q, want %q", i, got, want)
		}
		press(t, s, "s")
	}
}

// ---- formatting --------------------------------------------------------------

func TestGrubConfigDefaults(t *testing.T) {
	c := fixture.GrubConfig()
	c.Now, c.Lookup = time.Time{}, nil
	c.Meta.Regressions = []game.Regression{{Problem: "usaco:1205", Due: time.Now().Add(-time.Minute)}}
	v := plain(meta.NewGrub(c).View(80, 24))
	if !strings.Contains(v, "regressions due: 1") {
		t.Errorf("zero Now should mean time.Now():\n%s", v)
	}
	if !strings.Contains(v, "upsolve core dump: usaco:1014") {
		t.Errorf("nil Lookup should show catalog IDs:\n%s", v)
	}
}

// ---- shop ----------------------------------------------------------------------

// enter installs the selected package unless it is installed; t puts an
// installed theme in use and, on the one in use, goes back to the default;
// q leaves.
func TestShopKeys(t *testing.T) {
	shop := func() screens.Screen { return meta.NewShop(fixture.ShopConfig()) }
	tests := []struct {
		name string
		keys []string
		want tea.Msg
	}{
		{"enter on installed does nothing", []string{"enter"}, nil},
		{"install", []string{"j", "j", "enter"}, screens.Install{Package: "rng-tools"}},
		{"i installs too", []string{"G", "i"}, screens.Install{Package: "binutils"}},
		{"t on the theme in use goes back to the default", []string{"t"}, screens.UseTheme{}},
		{"t on another installed theme", []string{"j", "t"}, screens.UseTheme{Theme: "tux"}},
		{"t on a package that is not a theme", []string{"j", "j", "t"}, nil},
		{"q leaves", []string{"q"}, screens.Back{}},
		{"esc leaves", []string{"esc"}, screens.Back{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMsg(t, press(t, shop(), tt.keys...), tt.want)
		})
	}
}

// The goal is the first package on the shelf not installed whose
// requirements are; GRUB's apt entry names it and what it still costs.
func TestShopGoal(t *testing.T) {
	ps := fixture.Packages()
	p, short, ok := meta.Goal(ps, 100)
	if !ok || p.Name != "rng-tools" || short != 156 {
		t.Errorf("Goal: %s short %d ok %v, want rng-tools short 156", p.Name, short, ok)
	}
	for i := range ps {
		if ps[i].Name != "procps" && ps[i].Name != "binutils" {
			ps[i].Installed = true
		}
	}
	if p, _, _ := meta.Goal(ps, 0); p.Name != "procps" {
		t.Errorf("Goal with its requirements installed: %s, want procps", p.Name)
	}
	for i := range ps {
		ps[i].Installed = true
	}
	if _, _, ok := meta.Goal(ps, 0); ok {
		t.Error("Goal with everything installed")
	}
	c := fixture.GrubConfig()
	c.Meta.Bank = 100
	if s := plain(meta.NewGrub(c).View(100, 30)); !strings.Contains(s, "apt install rng-tools (256 B, 156 B to go)") {
		t.Errorf("GRUB's apt entry:\n%s", s)
	}
	c.Packages = nil
	if s := plain(meta.NewGrub(c).View(100, 30)); strings.Contains(s, "apt") {
		t.Errorf("GRUB lists apt with no packages:\n%s", s)
	}
}
