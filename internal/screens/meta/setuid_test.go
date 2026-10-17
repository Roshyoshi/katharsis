package meta_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/meta/fixture"
	"katharsis/internal/setuid"
	"katharsis/internal/ui"
)

// setuidScreen is the fixture's setuid screen with the keys pressed.
func setuidScreen(t *testing.T, c meta.SetuidConfig, keys ...string) *meta.Setuid {
	t.Helper()
	s := meta.NewSetuid(c)
	press(t, s, keys...)
	return s
}

// views is s drawn at both golden sizes, as plain text.
func views(s screens.Screen) map[string]string {
	out := map[string]string{}
	for _, sz := range sizes {
		out[fmt.Sprintf("%dx%d", sz[0], sz[1])] = plain(s.View(sz[0], sz[1]))
	}
	return out
}

// Every state the goldens use fits from cramped to huge: exactly height
// lines, none wider than width.
func TestSetuidFits(t *testing.T) {
	for name, mk := range map[string]func() screens.Screen{
		"none":    func() screens.Screen { return meta.NewSetuid(meta.SetuidConfig{Now: fixture.Now}) },
		"some":    func() screens.Screen { return meta.NewSetuid(fixture.SetuidConfig()) },
		"percent": func() screens.Screen { return meta.NewSetuid(fixture.ShowsPercent(fixture.SetuidConfig())) },
		"all": func() screens.Screen {
			return setuidScreen(t, setuidSpoil(meta.SpoilersAll), "G")
		},
		"hint": func() screens.Screen { return setuidScreen(t, fixture.SetuidConfig(), "G", "?") },
	} {
		for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {60, 15}, {80, 24}, {100, 30}, {120, 30}, {140, 40}, {220, 60}} {
			w, h := sz[0], sz[1]
			lines := strings.Split(mk().View(w, h), "\n")
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

// q and esc leave; ? shows a hidden secret's hint once and says so, and
// does nothing on any other row; n turns the notices off, then on.
func TestSetuidKeys(t *testing.T) {
	wantMsg(t, press(t, meta.NewSetuid(fixture.SetuidConfig()), "q"), screens.Back{})
	wantMsg(t, press(t, meta.NewSetuid(fixture.SetuidConfig()), "esc"), screens.Back{})
	// The secrets list the set one first: G is the last hidden secret.
	s := meta.NewSetuid(fixture.SetuidConfig())
	wantMsg(t, press(t, s, "G", "?"), meta.SetuidHint{ID: ".which-dreamed-it"})
	wantMsg(t, press(t, s, "?"), nil) // shown already
	wantMsg(t, press(t, s, "g", "?"), nil)
	wantMsg(t, press(t, meta.NewSetuid(setuidSpoil(meta.SpoilersHints)), "G", "?"), nil)
	n := meta.NewSetuid(fixture.SetuidConfig())
	wantMsg(t, press(t, n, "n"), meta.SetuidNotices{On: false})
	wantMsg(t, press(t, n, "n"), meta.SetuidNotices{On: true})
	if v := plain(n.View(80, 24)); !strings.Contains(v, "notices: on") {
		t.Errorf("header lacks the notices:\n%s", v)
	}
}

// tab goes to each directory's first bit in turn, round to /bin; j and k
// skip the headers.
func TestSetuidDirs(t *testing.T) {
	s := meta.NewSetuid(fixture.SetuidConfig())
	sel := func() string {
		for _, l := range strings.Split(plain(s.View(80, 24)), "\n") {
			if strings.HasPrefix(l, ui.G.Cursor) {
				f := strings.Fields(l)
				return f[len(f)-1]
			}
		}
		return ""
	}
	for _, want := range []string{"/sbin/speedrun-1", "/usr/bin/every-bug", "/home/you/.two-miles-high", "/bin/hello-world"} {
		press(t, s, "tab")
		if got := sel(); got != want {
			t.Errorf("tab: %q, want %q", got, want)
		}
	}
	press(t, s, "k")
	if got := sel(); got != "/bin/hello-world" {
		t.Errorf("k at the top: %q", got)
	}
	press(t, s, "tab", "k")
	if got := sel(); got != "/bin/un-birthday" {
		t.Errorf("k over a header: %q", got)
	}
}

// Under none a secret not set is -????????? with no name, its hint only
// under ?; hints shows every hint and no name; all shows names and
// barriers. A set secret is a plain row in every one. s cycles them.
func TestSetuidSpoilers(t *testing.T) {
	fm := fixture.SetuidConfig().Meta
	names := func(v string) (n int) {
		for _, d := range setuid.Table {
			if d.Secret() && !setuid.IsSet(&fm, d.ID) && strings.Contains(v, d.ID) {
				n++
			}
		}
		return n
	}
	hints := func(v string) (n int) {
		for _, d := range setuid.Table {
			if d.Secret() && d.ID != ".two-miles-high" && strings.Contains(v, "# "+d.Hint) {
				n++
			}
		}
		return n
	}
	for size, v := range views(setuidScreen(t, fixture.SetuidConfig(), "G")) {
		if names(v) != 0 || hints(v) != 0 || !strings.Contains(v, "-?????????") || !strings.Contains(v, "/home/you/.two-miles-high") {
			t.Errorf("none at %s: %d names, %d hints:\n%s", size, names(v), hints(v), v)
		}
	}
	for size, v := range views(setuidScreen(t, fixture.SetuidConfig(), "G", "?")) {
		if hints(v) != 1 || !strings.Contains(v, "Two endings. One dreamer.") {
			t.Errorf("? at %s: %d hints:\n%s", size, hints(v), v)
		}
	}
	// The hints spoilers at 140x40, where the secrets fit on screen.
	if v := views(setuidScreen(t, setuidSpoil(meta.SpoilersHints), "G"))["140x40"]; names(v) != 0 || hints(v) != 9 {
		t.Errorf("hints: %d names, %d hints:\n%s", names(v), hints(v), v)
	}
	if v := views(setuidScreen(t, setuidSpoil(meta.SpoilersAll), "G"))["140x40"]; names(v) != 9 || strings.Contains(v, "-?????????") {
		t.Errorf("all: %d names:\n%s", names(v), v)
	}
	// s goes none, hints, all, none; an unknown setting is none.
	s := meta.NewSetuid(meta.SetuidConfig{Now: fixture.Now, Spoilers: "everything"})
	for _, want := range []string{"none", "hints", "all", "none"} {
		if v := plain(s.View(80, 24)); !strings.Contains(v, "spoilers: "+want) {
			t.Errorf("want spoilers: %s:\n%s", want, v)
		}
		press(t, s, "s")
	}
}

// The percentage, and the SIZE of the bits that count it, stay hidden
// until a three-act boot is cleared (setuid.ShowsPercent, Silksong's "not
// until you go beyond Act 2"); then the header and the breakdown show it.
func TestSetuidPercent(t *testing.T) {
	for size, v := range views(setuidScreen(t, fixture.SetuidConfig(), "G")) {
		sizes := ""
		for _, l := range strings.Split(v, "\n") {
			if f := strings.Fields(l); len(f) > 2 && strings.HasPrefix(f[2], "/usr/bin/hundred") {
				sizes += f[1]
			}
		}
		if strings.Contains(v, "complete") || strings.Contains(v, "completion ·") || strings.Trim(sizes, "-") != "" {
			t.Errorf("the percentage shows before a clear at %s:\n%s", size, v)
		}
	}
	c := fixture.ShowsPercent(fixture.SetuidConfig())
	pct, _ := setuid.Completion(&c.Meta)
	for size, v := range views(setuidScreen(t, c)) {
		if !strings.Contains(v, fmt.Sprintf("%d%% complete", pct)) {
			t.Errorf("no %d%% at %s:\n%s", pct, size, v)
		}
	}
	if v := views(setuidScreen(t, c))["140x40"]; !strings.Contains(v, fmt.Sprintf("completion · %d%%", pct)) || !strings.Contains(v, "beyond") {
		t.Errorf("no breakdown:\n%s", v)
	}
}

// Rows never say "missed" or count boots (GR8): a bit not set reads "not
// set", and a speed completion past its bound "for a fresh store".
func TestSetuidNotSet(t *testing.T) {
	c := fixture.SetuidConfig()
	c.Meta.Record.BootTime = 200 * 3600e9 // past both bounds
	s := setuidScreen(t, c, "tab")
	for range 14 {
		press(t, s, "j")
	}
	for size, v := range views(s) {
		low := strings.ToLower(v)
		if strings.Contains(low, "missed") || !strings.Contains(v, "for a fresh store") || !strings.Contains(low, "not set") {
			t.Errorf("at %s:\n%s", size, v)
		}
	}
}

// A backfilled bit says it came from git log; one set in a boot names it.
func TestSetuidProvenance(t *testing.T) {
	v := views(setuidScreen(t, fixture.SetuidConfig()))["140x40"]
	if !strings.Contains(v, "from git log") || !strings.Contains(v, "boot #41") {
		t.Errorf("provenance:\n%s", v)
	}
	m := game.Meta{}
	if v := plain(meta.NewSetuid(meta.SetuidConfig{Meta: m, Now: fixture.Now}).View(80, 24)); !strings.Contains(v, "0 of 63 set") {
		t.Errorf("an empty save:\n%s", v)
	}
}

// Under the ASCII glyphs the screen draws no Unicode rule, bar or
// separator (the QA list: the parody screens still drew Unicode).
func TestSetuidASCII(t *testing.T) {
	ui.SetGlyphs(ui.ASCII)
	defer ui.SetGlyphs(ui.Unicode)
	for size, v := range views(setuidScreen(t, fixture.ShowsPercent(fixture.SetuidConfig()), "j")) {
		for _, g := range []string{"·", "▓", "░", "│", "┴", "─", "▸", "↓", "↑"} {
			if strings.Contains(v, g) {
				t.Errorf("at %s the ASCII screen draws %q:\n%s", size, g, v)
			}
		}
	}
}

// git log marks a boot the Court ended "awake (the Court)", never
// "woke", and the cleared filter keeps it: it is a clear.
func TestLogAwake(t *testing.T) {
	boots := fixture.Boots()
	boots[0].Cleared, boots[0].Cause, boots[0].Ending = true, "", game.EndingWake
	s := meta.NewLog(boots, fixture.PrestigeName)
	for _, sz := range sizes {
		if v := plain(s.View(sz[0], sz[1])); !strings.Contains(v, "awake (the Court)") || strings.Contains(v, "woke") {
			t.Errorf("at %v:\n%s", sz, v)
		}
	}
	press(t, s, "f") // cleared only
	if v := plain(s.View(80, 24)); !strings.Contains(v, "awake (the Court)") {
		t.Errorf("the cleared filter drops the awake boot:\n%s", v)
	}
}

// Every key keeps the cursor on a bit, held past either end of the table:
// j at the last bit and k at the first indexed past the rows (a panic seen
// in play), and so could tab, pgup and pgdown at the ends.
func TestSetuidKeysStayInTheTable(t *testing.T) {
	var keys []string
	for range 80 {
		keys = append(keys, "j")
	}
	keys = append(keys, "pgdown", "pgdown", "G", "j", "end", "down")
	for range 80 {
		keys = append(keys, "k")
	}
	keys = append(keys, "pgup", "pgup", "g", "k", "home", "up")
	for range 10 {
		keys = append(keys, "tab")
	}
	for range 10 {
		keys = append(keys, "shift+tab")
	}
	s := setuidScreen(t, fixture.SetuidConfig(), keys...)
	for _, k := range []string{"j", "k", "?", "G", "?", "g", "?"} {
		press(t, s, k) // ? reads the bit under the cursor
	}
	if v := plain(s.View(80, 24)); !strings.Contains(v, "find / -perm -4000") {
		t.Errorf("the table after the keys:\n%s", v)
	}
}
