package boot

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

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot/fixture"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

// stubFlavor is fixed text of realistic length, so the goldens pin layout
// and stay put when internal/flavor's content changes.
var stubFlavor = flavorSource{
	BiomeName: func(t string) string {
		if n, ok := map[string]string{
			"mixed": "Userland", "dp": "The Cache", "data-structures": "The Heap",
			"graphs": "The Mesh", "math": "The FPU",
		}[t]; ok {
			return n
		}
		return t
	},
	BiomeBlurb: func(t string) string {
		return "Stub blurb for " + t + ", long enough to wrap onto a second line of a card."
	},
	BossName: func(k string) string {
		return map[string]string{"heisenbug": "Heisenbug", game.FinalBoss: "The Halting Problem"}[k]
	},
	BossBlurb: func(string) string { return "It disappears the moment you add a print statement." },
	PrestigeName: func(p int) string {
		return []string{"Bronze", "Silver", "Gold", "Platinum", "Advanced"}[min(max(p, 0), 4)]
	},
	UpgradeText: func(u game.Upgrade, _ rules.Rules) (string, string) {
		switch u {
		case game.Overclock:
			return "Overclock", "+15% timer on every later door"
		case game.ECC:
			return "ECC", "+1 life"
		case game.Duck:
			return "Duck", "+1 rubber duck"
		}
		return string(u), ""
	},
	DaemonText: func(d game.Daemon, _ rules.Rules) (string, string) {
		switch d {
		case game.Strace:
			return "strace", "the failing test's input is shown"
		case game.Nice:
			return "nice -20", "+20% timer, but a WA costs 2 lives"
		case game.Fork:
			return "fork", "4 doors per room after an AC"
		case game.DevRandom:
			return "/dev/random", "mystery doors pay 2x"
		}
		return string(d), ""
	},
	QuoteFor: func(c game.Outcome, _ string, _ uint64) flavor.Quote {
		switch c {
		case game.OutOfLives:
			return flavor.Quote{
				Text:   "Program testing can be used to show the presence of bugs, but never to show their absence!",
				Author: "Edsger W. Dijkstra", Source: "Notes on Structured Programming", Year: 1970,
			}
		case game.GaveUp:
			return flavor.Quote{Text: "Talk is cheap. Show me the code.", Author: "Linus Torvalds", Source: "linux-kernel", Year: 2000}
		}
		return flavor.Quote{
			Text:   "It always takes longer than you expect, even when you take into account Hofstadter's Law.",
			Author: "Douglas Hofstadter", Source: "GEB", Year: 1979,
		}
	},
	Banner: func(k string) string {
		if k == "cleared" {
			return " _  _ ___ _  _____ ___ \n| || | _ \\ | |_   _/ __|\n| __ |   / |__| | \\__ \\\n|_||_|_|_\\____|_| |___/"
		}
		return ""
	},
	PanicReason: func(c game.Outcome, _ string) string {
		return map[game.Outcome]string{
			game.TimedOut: "timer expired", game.OutOfLives: "out of lives",
			game.GaveUp: "Attempted to kill init! exitcode=0x00000009",
		}[c]
	},
	CallTrace: func(c game.Outcome, _ string, _ uint64) []string {
		if c == game.TimedOut {
			return []string{"it_real_fn+0x1d/0x80", "hrtimer_interrupt+0x10f/0x240"}
		}
		return []string{"do_exit+0x3a/0xc0"}
	},
	// Every hook has a line, the hook named in it, so a golden shows which
	// line a screen asked for and where it went.
	Story: func(hook string, _ uint64) string { return "Stub line for " + hook + "." },
	StoryBlock: func(hook string) []string {
		return []string{"Stub block for " + hook + ", line one.", "Stub block, line two."}
	},
}

func TestMain(m *testing.M) {
	flag.Parse()
	fl = stubFlavor
	os.Exit(m.Run())
}

func env(st *game.State) Env {
	return Env{State: st, Meta: fixture.Meta(), Lookup: fixture.Lookup, Now: fixture.Now}
}

// crashEnv is the boot crashed by cause, with the core dump the crash left
// in Meta.
func crashEnv(cause game.Outcome) Env {
	e := env(fixture.Crash(cause))
	e.Meta = fixture.CrashMeta(cause)
	return e
}

type screenCase struct {
	name string
	make func() screens.Screen
}

func cases() []screenCase {
	return []screenCase{
		{"route", func() screens.Screen { return NewRoute(env(fixture.Route())) }},
		{"route3", func() screens.Screen { return NewRoute(env(fixture.Route3())) }},
		// The governor raised every aim (RT8): the route says the raised one.
		{"route_eased", func() screens.Screen {
			st := fixture.Route()
			st.Ease = []game.Ease{{Kind: game.EaseGovernor, Why: "the last 2 boots ended in Userland", Clear: []float64{0.95, 0.78, 0.65, 0.55}}}
			return NewRoute(env(st))
		}},
		{"doors", func() screens.Screen { return NewDoors(env(fixture.Doors())) }},
		{"doors_fork", func() screens.Screen { return NewDoors(env(fixture.DoorsFork())) }},
		{"doors_boss", func() screens.Screen { return NewDoors(env(fixture.Boss())) }},
		// Userland at the boot's start: its easing over the cards (RT8), D44's
		// line on every card.
		{"doors_userland", func() screens.Screen { return NewDoors(env(fixture.Userland())) }},
		{"doors_declined", func() screens.Screen {
			st := fixture.Userland()
			st.Declined, st.Lives = true, 3
			return NewDoors(env(st))
		}},
		{"doors_map", func() screens.Screen {
			d := NewDoors(env(fixture.Doors()))
			d.Update(press("m"))
			return d
		}},
		{"map", func() screens.Screen { return NewMap(env(fixture.Doors())) }},
		{"map_crashed", func() screens.Screen { return NewMap(env(fixture.Crash(game.TimedOut))) }},
		// A timeout that cost a life, and the boot went on (D44).
		{"map_timeout", func() screens.Screen { return NewMap(env(fixture.UserlandTimeout())) }},
		{"merge", func() screens.Screen { return NewMerge(env(fixture.Merge())) }},
		{"upgrade", func() screens.Screen { return NewUpgradeScreen(env(fixture.Upgrade())) }},
		{"crash_timer", func() screens.Screen { return NewCrash(crashEnv(game.TimedOut)) }},
		{"crash_lives", func() screens.Screen { return NewCrash(crashEnv(game.OutOfLives)) }},
		{"crash_quit", func() screens.Screen { return NewCrash(crashEnv(game.GaveUp)) }},
		{"cleared", func() screens.Screen {
			e := env(fixture.Cleared())
			e.Meta = fixture.ClearedMeta()
			return NewCleared(e)
		}},
	}
}

func press(k string) tea.KeyPressMsg {
	codes := map[string]rune{
		"enter": tea.KeyEnter, "left": tea.KeyLeft, "right": tea.KeyRight, "up": tea.KeyUp,
		"down": tea.KeyDown, "esc": tea.KeyEscape, "tab": tea.KeyTab, "space": tea.KeySpace,
	}
	if c, ok := codes[k]; ok {
		return tea.KeyPressMsg{Code: c}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

// emitted runs cmd and returns its message, nil for no command.
func emitted(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func golden(t *testing.T, name, got string) {
	t.Helper()
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
	if string(want) != got {
		t.Errorf("%s differs from golden (go test -update rewrites it)\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

var goldenSizes = [][2]int{{80, 24}, {140, 40}}

func TestGoldenScreens(t *testing.T) {
	for _, c := range cases() {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				golden(t, name, ansi.Strip(c.make().View(sz[0], sz[1]))+"\n")
			})
		}
	}
}

// checkFits asserts View fills exactly height rows, none wider than width.
func checkFits(t *testing.T, name, view string, w, h int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != h {
		t.Errorf("%s at %dx%d: %d lines, want %d", name, w, h, len(lines), h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got > w {
			t.Errorf("%s at %dx%d: line %d is %d wide: %q", name, w, h, i+1, got, ansi.Strip(l))
		}
	}
}

var fitSizes = [][2]int{{80, 24}, {140, 40}, {40, 16}, {60, 20}, {89, 24}, {90, 24}, {100, 30}, {120, 18}, {200, 50}}

func TestScreensFit(t *testing.T) {
	for _, src := range []struct {
		name string
		fl   flavorSource
	}{{"stub", stubFlavor}, {"real", realFlavor}} {
		fl = src.fl
		for _, c := range cases() {
			for _, sz := range fitSizes {
				s := c.make()
				// Walk the cursor across every item so each selection state
				// is measured, not just the first.
				for i := 0; i < 6; i++ {
					checkFits(t, src.name+"/"+c.name, s.View(sz[0], sz[1]), sz[0], sz[1])
					s, _ = s.Update(press("down"))
				}
			}
		}
	}
	fl = stubFlavor
}

func TestRouteKeys(t *testing.T) {
	r := NewRoute(env(fixture.Route()))
	if got := emitted(func() tea.Cmd { _, c := r.Update(press("enter")); return c }()); got != (game.ChooseRoute{Index: 0}) {
		t.Errorf("enter: got %#v", got)
	}
	r.Update(press("l"))
	r.Update(press("right")) // clamps at the last offer
	_, cmd := r.Update(press("enter"))
	if got := emitted(cmd); got != (game.ChooseRoute{Index: 1}) {
		t.Errorf("l, right, enter: got %#v", got)
	}
	r.Update(press("1"))
	r.Update(press("9")) // out of range: ignored
	if r.sel != 0 {
		t.Errorf("1, 9: selected %d, want 0", r.sel)
	}
}

func TestDoorsKeys(t *testing.T) {
	d := NewDoors(env(fixture.Doors()))
	d.Update(press("3"))
	_, cmd := d.Update(press("enter"))
	if got := emitted(cmd); got != (game.ChooseDoor{Index: 2}) {
		t.Errorf("3, enter: got %#v", got)
	}
	d.Update(press("h"))
	d.Update(press("left"))
	d.Update(press("left"))
	if d.sel != 0 {
		t.Errorf("left past the first door: selected %d", d.sel)
	}

	d.Update(press("m"))
	if v := ansi.Strip(d.View(80, 24)); !strings.Contains(v, "result") {
		t.Errorf("m should show the map with history:\n%s", v)
	}
	if _, cmd := d.Update(press("enter")); cmd != nil {
		t.Errorf("enter on the map must not open a door, got %#v", emitted(cmd))
	}
	if v := ansi.Strip(d.View(80, 24)); strings.Contains(v, "result") {
		t.Errorf("enter should leave the map:\n%s", v)
	}
}

func TestDoorsContent(t *testing.T) {
	v := ansi.Strip(NewDoors(env(fixture.Doors())).View(140, 40))
	for _, want := range []string{
		"Boot #42", "Silver", "The Heap · room 2/2", "♥♥♡", "🦆1", "96 B uncommitted", "112 B committed",
		"uptime 47:12", "daemons strace",
		"The Bovine Shuffle", "Functional Graphs · Cycle Detection", "◆1300", "⏱ 30:00", "+80 B",
		"☠ cursed", "tags hidden", "☠ no ducks · 1 WA = crash · ×2", "◌ blind · ×1.5",
		"???", "? mystery · ×2", "USACO · 2016 US Open, Silver P3",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("doors missing %q", want)
		}
	}
	// Stacked cards say what blind and mystery hide in place of the tags;
	// side by side, the tag slot says it and the line only what they pay.
	stacked := ansi.Strip(NewDoors(env(fixture.Doors())).View(80, 40))
	for _, want := range []string{"◌ blind: tags hidden · ×1.5", "? mystery: title and tags hidden · ×2"} {
		if !strings.Contains(stacked, want) {
			t.Errorf("stacked doors missing %q", want)
		}
	}
	if strings.Contains(v, "blind: tags hidden") {
		t.Errorf("side-by-side cards say tags hidden twice:\n%s", v)
	}
	// A mystery door must not leak its title or contest.
	for _, leak := range []string{"Wormhole Sort", "2020 January"} {
		if strings.Contains(v, leak) {
			t.Errorf("mystery door leaks %q", leak)
		}
	}

	v = ansi.Strip(NewDoors(env(fixture.DoorsFork())).View(80, 24))
	if !strings.Contains(v, "↺ regression: crashed you 3 days ago") {
		t.Errorf("fork doors missing the regression line:\n%s", v)
	}
	v = ansi.Strip(NewDoors(env(fixture.Boss())).View(140, 40))
	for _, want := range []string{"H E I S E N B U G", "⚔ BOSS", "It disappears the moment", "Cow Poetry"} {
		if !strings.Contains(v, want) {
			t.Errorf("boss door missing %q", want)
		}
	}
}

// A door on a mastered topic hides its tags without the blind modifier's
// pay, and the card says why; the HUD bar offers the duck for them.
func TestMasteredDoor(t *testing.T) {
	st := fixture.Doors()
	st.Doors[0].Mastered = true
	// Side by side the tag slot says the tags are hidden; stacked, the
	// reason takes the tags' place.
	for w, why := range map[int]string{140: "mastered topic", 80: "mastered topic: tags hidden"} {
		v := ansi.Strip(NewDoors(env(st)).View(w, 40))
		if !strings.Contains(v, why) || !strings.Contains(v, "tags hidden") || strings.Contains(v, "Functional Graphs") {
			t.Errorf("at %d cols the mastered door should hide its tags and say why:\n%s", w, v)
		}
	}
	p := fixture.Problem()
	p.Current.Door.Mastered = true
	prob, _ := fixture.Lookup(p.Current.Door.Problem)
	if v := ansi.Strip(HUDBar(p, prob, fixture.Now, 60)); !strings.Contains(v, "quack to reveal") {
		t.Errorf("HUD bar on a mastered door:\n%s", v)
	}
}

// s spends a held shuf on the room, and the footer says how many are held;
// a boss room and a boot without a shuf ignore it.
func TestDoorsShuf(t *testing.T) {
	press1 := func(st *game.State) tea.Msg {
		_, cmd := NewDoors(env(st)).Update(press("s"))
		return emitted(cmd)
	}
	st := fixture.Doors()
	if got := press1(st); got != nil {
		t.Errorf("s with no shuf: got %#v", got)
	}
	st.Rerolls = 2
	if got := press1(st); got != (game.Reroll{}) {
		t.Errorf("s with a shuf: got %#v", got)
	}
	if v := ansi.Strip(NewDoors(env(st)).View(140, 40)); !strings.Contains(v, "s shuf (2)") {
		t.Errorf("footer should offer the shuf:\n%s", v)
	}
	boss := fixture.Boss()
	boss.Rerolls = 1
	if got := press1(boss); got != nil {
		t.Errorf("s in the boss room: got %#v", got)
	}
}

func TestDoorsStackWhenNarrow(t *testing.T) {
	d := NewDoors(env(fixture.Doors()))
	narrow := ansi.Strip(d.View(89, 30))
	wide := ansi.Strip(d.View(90, 30))
	// A card's top edge is a corner, a rule glyph, then the door number.
	edges := func(s string) (lines, cards int) {
		for _, l := range strings.Split(s, "\n") {
			n := 0
			fs := strings.Fields(l)
			for i, f := range fs {
				if (f == "╭─" || f == "┏━") && i+1 < len(fs) && (fs[i+1] == "▸" || fs[i+1][0] >= '1' && fs[i+1][0] <= '9') {
					n++
				}
			}
			if n > 0 {
				lines++
				cards += n
			}
		}
		return lines, cards
	}
	if lines, cards := edges(narrow); lines != 3 || cards != 3 {
		t.Errorf("at 89 cols want 3 stacked cards, got %d cards on %d lines:\n%s", cards, lines, narrow)
	}
	if lines, cards := edges(wide); lines != 1 || cards != 3 {
		t.Errorf("at 90 cols want 3 cards side by side, got %d cards on %d lines:\n%s", cards, lines, wide)
	}
}

func TestMergeKeys(t *testing.T) {
	m := NewMerge(env(fixture.Merge()))
	step := func(k string) tea.Msg { _, c := m.Update(press(k)); return emitted(c) }
	if got := step("c"); got != (game.Commit{}) {
		t.Errorf("c: got %#v", got)
	}
	if got := step("enter"); got != (game.PickUpgrade{Upgrade: game.Overclock}) {
		t.Errorf("enter on first row: got %#v", got)
	}
	step("down")
	if got := step("enter"); got != (game.PickUpgrade{Upgrade: game.ECC}) {
		t.Errorf("down, enter: got %#v", got)
	}
	step("down")
	step("j")
	if got := step("enter"); got != (game.Spend{Item: game.Fsck}) {
		t.Errorf("into the shop: got %#v", got)
	}
	for range 3 {
		step("down")
	}
	if got := step("enter"); got != (game.PickDaemon{Daemon: game.Nice}) {
		t.Errorf("into daemons: got %#v", got)
	}
	for range 5 {
		step("down")
	}
	if got := step("enter"); got != (game.LeaveMerge{}) {
		t.Errorf("leave row: got %#v", got)
	}

	// Nothing to commit: c does nothing.
	st := fixture.Merge()
	st.Uncommitted = 0
	m = NewMerge(env(st))
	if got := step("c"); got != nil {
		t.Errorf("c with nothing uncommitted: got %#v", got)
	}
}

// The merge window prices every item on sale, says which the bytes don't
// cover, and sells nothing in a trial.
func TestMergeSpendRows(t *testing.T) {
	st := fixture.Merge()
	st.Uncommitted, st.Lives, st.Rerolls = 40, st.MaxLives, 1
	v := ansi.Strip(NewMerge(env(st)).View(140, 40))
	for _, want := range []string{"spend", "1 shuf held", "fsck", "64 B · lives full", "48 B · 8 B short", "shuf"} {
		if !strings.Contains(v, want) {
			t.Errorf("merge window missing %q:\n%s", want, v)
		}
	}
	st.Trial = &game.TrialState{}
	if v := ansi.Strip(NewMerge(env(st)).View(140, 40)); strings.Contains(v, "fsck") {
		t.Errorf("a trial's merge window sells items:\n%s", v)
	}
}

func TestMergeRefreshMovesPastTakenPick(t *testing.T) {
	st := fixture.Merge()
	m := NewMerge(env(st))
	m.Update(press("down")) // ECC
	st2 := fixture.Merge()
	st2.PendingUpgrade, st2.UpgradeOffers = false, nil
	m.Refresh(env(st2))
	_, cmd := m.Update(press("enter"))
	if got := emitted(cmd); got != (game.Spend{Item: game.Fsck}) {
		t.Errorf("after the upgrade was taken the cursor should sit on the first item, got %#v", got)
	}
}

// Daemon slots are a preset's max_daemons, not a fixed three: with a
// fourth slot free the offers stay pickable.
func TestMergeDaemonSlotsFromRules(t *testing.T) {
	st := fixture.Merge()
	st.PendingUpgrade = false
	st.Rules.MaxDaemons = 4
	st.Daemons = []game.Daemon{game.Strace, game.Kexec, game.Fork}
	m := NewMerge(env(st))
	for range onSale(st) {
		m.Update(press("down"))
	}
	_, cmd := m.Update(press("enter"))
	if got := emitted(cmd); got != (game.PickDaemon{Daemon: game.Nice}) {
		t.Errorf("enter on the first daemon: got %#v", got)
	}
	if v := ansi.Strip(m.View(80, 24)); !strings.Contains(v, "held 3/4") {
		t.Errorf("held count should come from the rules:\n%s", v)
	}
}

func TestUpgradePicker(t *testing.T) {
	p := picker{offers: []game.Upgrade{game.Overclock, game.ECC, game.Duck}, rules: fixture.Rules()}
	p.update(press("right"))
	if got := emitted(p.update(press("enter"))); got != (game.PickUpgrade{Upgrade: game.ECC}) {
		t.Errorf("right, enter: got %#v", got)
	}
	if v := ansi.Strip(strings.Join(p.lines(100), "\n")); !strings.Contains(v, "▸ 2") || !strings.Contains(v, "+1 life") {
		t.Errorf("cards view:\n%s", v)
	}
	if v := ansi.Strip(strings.Join(p.lines(40), "\n")); !strings.Contains(v, "▸ ECC") {
		t.Errorf("narrow view should fall back to a list:\n%s", v)
	}

	u := NewUpgradeScreen(env(fixture.Upgrade()))
	u.Update(press("3"))
	_, cmd := u.Update(press("enter"))
	if got := emitted(cmd); got != (game.PickUpgrade{Upgrade: game.Duck}) {
		t.Errorf("screen 3, enter: got %#v", got)
	}
	if v := ansi.Strip(u.View(80, 24)); !strings.Contains(v, "✓ AC  Moocast  in 14:02 of 30:00  -O3") {
		t.Errorf("upgrade screen should name the clear:\n%s", v)
	}
}

// An -O3 clear owes two picks of the same kind of cards; the count is what
// tells the second pick from the first.
func TestUpgradePicksOwed(t *testing.T) {
	for _, tc := range []struct {
		name      string
		picks     int
		o3        bool
		screen    string // the upgrade screen's ask
		mergeNote string
	}{
		{"one pick", 1, false, "Pick an upgrade.", "a clear earned one; pick it before you leave"},
		{"legacy save", 0, false, "Pick an upgrade.", "a clear earned one; pick it before you leave"},
		{"-O3, first", 2, true, "Pick an upgrade · 2 to pick (-O3 earned an extra)",
			"2 to pick (-O3 earned an extra); pick before you leave"},
		{"-O3, second", 1, true, "Pick another upgrade · 1 left (-O3 earned an extra)",
			"1 left (-O3 earned an extra); pick before you leave"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := fixture.Upgrade()
			st.UpgradePicks = tc.picks
			st.History[len(st.History)-1].O3 = tc.o3
			if v := ansi.Strip(NewUpgradeScreen(env(st)).View(100, 30)); !strings.Contains(v, tc.screen) {
				t.Errorf("upgrade screen: want %q in:\n%s", tc.screen, v)
			}

			ms := fixture.Merge()
			ms.UpgradePicks = tc.picks
			ms.History[len(ms.History)-1].O3 = tc.o3
			if v := ansi.Strip(NewMerge(env(ms)).View(140, 40)); !strings.Contains(v, tc.mergeNote) {
				t.Errorf("merge window: want %q in:\n%s", tc.mergeNote, v)
			}
		})
	}
}

// A daemon's description is cut only when the terminal is too narrow for
// it, not at a fixed column.
func TestMergeLongDescriptions(t *testing.T) {
	long := "One free door swap per biome; a swap normally costs a life, and this sentence runs on."
	fl.DaemonText = func(d game.Daemon, r rules.Rules) (string, string) {
		if d == game.Kexec {
			return "kexec", long
		}
		return stubFlavor.DaemonText(d, r)
	}
	defer func() { fl = stubFlavor }()

	st := fixture.Merge()
	st.DaemonOffers = []game.Daemon{game.Nice, game.Kexec, game.DevRandom}
	m := NewMerge(env(st))
	for _, w := range []int{120, 200} {
		if v := ansi.Strip(m.View(w, 50)); !strings.Contains(v, long) {
			t.Errorf("at %d columns the description should be whole:\n%s", w, v)
		}
	}
	v := m.View(80, 40)
	checkFits(t, "merge", v, 80, 40)
	if !strings.Contains(ansi.Strip(v), "One free door swap per biome; a swap normally") {
		t.Errorf("at 80 columns the description should use the width there is:\n%s", ansi.Strip(v))
	}
}

func TestCrashKeys(t *testing.T) {
	c := NewCrash(crashEnv(game.TimedOut))
	for k, want := range map[string]tea.Msg{
		"u": screens.Upsolve{Problem: "usaco:644"},
		"e": screens.OpenEditorial{Problem: "usaco:644"},
		"l": screens.OpenLog{},
		"g": screens.OpenGrub{}, // UX J12: q leaves for GRUB too, poweroff quits
		"q": screens.OpenGrub{},
	} {
		_, cmd := c.Update(press(k))
		if got := emitted(cmd); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v, want %#v", k, got, want)
		}
	}
}

// The editorial is a key on the panic, offered only when the problem has
// one; a problem the lookup does not know is left for the app to judge.
func TestCrashEditorialKey(t *testing.T) {
	for _, tc := range []struct {
		name   string
		lookup screens.Lookup
		want   tea.Msg // nil: e does nothing and the footer omits it
	}{
		{"has editorial", fixture.Lookup, screens.OpenEditorial{Problem: "usaco:644"}},
		{"no editorial", func(id catalog.ID) (catalog.Problem, bool) {
			p, ok := fixture.Lookup(id)
			p.HasEditorial = false
			return p, ok
		}, nil},
		{"unknown problem", func(catalog.ID) (catalog.Problem, bool) { return catalog.Problem{}, false },
			screens.OpenEditorial{Problem: "usaco:644"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := crashEnv(game.TimedOut)
			e.Lookup = tc.lookup
			c := NewCrash(e)
			_, cmd := c.Update(press("e"))
			if got := emitted(cmd); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("e: got %#v, want %#v", got, tc.want)
			}
			v := ansi.Strip(c.View(120, 40))
			if shown := strings.Contains(v, "[e] editorial"); shown != (tc.want != nil) {
				t.Errorf("footer shows [e] = %v, want %v:\n%s", shown, tc.want != nil, v)
			}
		})
	}
}

// The core dump line follows Meta, not Crash.Lost: an upsolve from the
// crash screen banks the dump, and the line must stop offering it.
func TestCrashCoreDump(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(e *Env)
		want  string
	}{
		{"dump held", func(*Env) {}, `core dumped: 96 B on "Closing the Farm"`},
		{"upsolved", func(e *Env) {
			got := game.Upsolve(&e.Meta, "usaco:644", game.Editorial, time.Minute, e.State.Crash.At.Add(time.Hour))
			if got != 96 {
				t.Fatalf("game.Upsolve recovered %d, want 96", got)
			}
		}, "core dump recovered: +96 B banked"},
		{"trial", func(e *Env) {
			e.Meta.CoreDump = nil
			e.State.Trial = &game.TrialState{}
		}, "no core dump: a trial loses its 96 B outright"},
		{"dump gone", func(e *Env) { e.Meta.CoreDump = nil }, "no core dump left to recover"},
		{"dump on another door", func(e *Env) { e.Meta.CoreDump.Problem = "usaco:764" }, "no core dump left to recover"},
		{"nothing lost", func(e *Env) {
			e.Meta.CoreDump = nil
			e.State.Crash.Lost = 0
		}, "no core dump: nothing was uncommitted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := crashEnv(game.TimedOut)
			tc.setup(&e)
			v := ansi.Strip(NewCrash(e).View(120, 40))
			if !strings.Contains(v, tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, v)
			}
			if tc.name != "dump held" && strings.Contains(v, "core dumped") {
				t.Errorf("offers a core dump Meta does not hold:\n%s", v)
			}
		})
	}
}

func TestCrashContent(t *testing.T) {
	v := ansi.Strip(NewCrash(crashEnv(game.TimedOut)).View(100, 40))
	for _, want := range []string{
		`[ 4650.337] Kernel panic - not syncing: timer expired in "Closing the Farm"`,
		"Comm: you Tainted: B (blind) C (cursed)",
		"Hardware name: katharsis Silver, preset standard, seed 8f3a",
		"Modules linked in: strace(O)",
		"[ 4650.337]  it_real_fn+0x1d/0x80\n[ 4650.337]  hrtimer_interrupt+0x10f/0x240\n[ 4650.337]  door_5+0x1e/0x1e [The Heap]",
		"[ 4650.337]  ? door_2+0x4/0xf [Userland]",
		"[ 4650.337]  boot_42+0x4d/0x69",
		"[ 4650.337] ---[ end Kernel panic ]---",
		`"It always takes longer than you expect`,
		"— Douglas Hofstadter, GEB (1979)",
		`core dumped: 96 B on "Closing the Farm"`,
		"[e] editorial  [r] reboot",
		`[ 2642.000] door_4: AC "Moocast" +96 B`,
	} {
		if !strings.Contains(v, want) {
			t.Errorf("crash missing %q", want)
		}
	}
	// Newest door first, boot frame last.
	if i, j := strings.Index(v, "door_5+"), strings.Index(v, "door_1+"); i < 0 || j < i {
		t.Errorf("trace order wrong:\n%s", v)
	}

	lives := ansi.Strip(NewCrash(env(fixture.Crash(game.OutOfLives))).View(120, 40))
	for _, want := range []string{
		`out of lives in "The Bovine Shuffle"`,
		"Killed process 1 (you) verdict:WA passed:37/52 submits:2",
		"Not tainted",
	} {
		if !strings.Contains(lives, want) {
			t.Errorf("out-of-lives crash missing %q:\n%s", want, lives)
		}
	}
	if n := strings.Count(lives, "door_5+"); n != 1 {
		t.Errorf("crashing attempt in Current should appear once in the trace, got %d", n)
	}

	quit := ansi.Strip(NewCrash(env(fixture.Crash(game.GaveUp))).View(120, 40))
	for _, want := range []string{"you[1]: :q! sent SIGKILL to init", `Attempted to kill init! exitcode=0x00000009 in "Wormhole Sort"`, "M (mystery) B (blind)"} {
		if !strings.Contains(quit, want) {
			t.Errorf(":q! crash missing %q:\n%s", want, quit)
		}
	}
}

func TestCrashElidesTraceWhenShort(t *testing.T) {
	v := ansi.Strip(NewCrash(crashEnv(game.TimedOut)).View(80, 16))
	for _, want := range []string{"door_5+", "older doors", "boot_42+", "end Kernel panic", "core dumped"} {
		if !strings.Contains(v, want) {
			t.Errorf("short crash missing %q:\n%s", want, v)
		}
	}
}

// engineCrash plays a real boot into a crash: the first door is cleared, so
// there are bytes to dump, and the second crashes by cause. With cursed
// every door is cursed, so one WA crashes with lives left.
func engineCrash(t *testing.T, cause game.Outcome, cursed bool) Env {
	t.Helper()
	r := rules.Standard()
	r.BlindChance, r.MysteryChance, r.CursedChance, r.Ascensions = 0, 0, 0, nil
	// A cold start is eased (more lives) and plays boot #1's shape, a
	// Userland timeout costs a life (D44), and Userland's easiest door is
	// never cursed: none of that is the crash this plays into.
	r.EaseAbove, r.BuybackTimers, r.GovernorAfter = 0, nil, 0
	r.FirstBiomes, r.FirstDoor, r.FirstEditorial = nil, false, false
	r.Biomes[0].TimeoutLife, r.Biomes[0].Plain = false, 0
	if cursed {
		r.CursedChance = 1
	}
	var pool []catalog.Problem
	for _, topic := range catalog.Topics {
		for rating := 800; rating <= 3000; rating += 50 {
			pool = append(pool, catalog.Problem{
				ID:    catalog.ID(fmt.Sprintf("t:%s:%d", topic, rating)),
				Title: fmt.Sprintf("%s %d", topic, rating), Topic: topic, Rating: rating, Judgeable: true,
			})
		}
	}
	lookup := func(id catalog.ID) (catalog.Problem, bool) {
		for _, p := range pool {
			if p.ID == id {
				return p, true
			}
		}
		return catalog.Problem{}, false
	}
	meta := &game.Meta{}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	st, _, err := game.NewBoot(r, meta, pool, 0, 42, now)
	if err != nil {
		t.Fatalf("NewBoot: %v", err)
	}
	step := func(a game.Action) {
		t.Helper()
		now = now.Add(time.Minute)
		if _, err := game.Step(st, meta, pool, a, now); err != nil {
			t.Fatalf("Step(%T) in %s: %v", a, st.Phase, err)
		}
	}
	open := func() {
		t.Helper()
		for st.PendingUpgrade {
			step(game.PickUpgrade{Upgrade: st.UpgradeOffers[0]})
		}
		if st.Phase == game.PhaseRoute {
			step(game.ChooseRoute{})
		}
		step(game.ChooseDoor{})
	}
	open()
	step(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
	open()
	switch cause {
	case game.OutOfLives:
		for st.Phase == game.PhaseProblem {
			step(game.SubmitResult{Verdict: "WA", Passed: 3, Total: 10})
		}
	case game.TimedOut:
		now = now.Add(st.Current.Door.Timer)
		step(game.Tick{})
	case game.GaveUp:
		step(game.GiveUp{})
	}
	if st.Phase != game.PhaseCrashed || st.Crash.Cause != cause || st.Crash.Lost == 0 {
		t.Fatalf("boot did not crash by %s with bytes dumped: phase %s, crash %+v", cause, st.Phase, st.Crash)
	}
	return Env{State: st, Meta: *meta, Lookup: lookup, Now: now}
}

// The engine logs its own panic line, the core dump and the fatal submit,
// and the screen prints each of those itself; the dmesg tail must not say
// them twice. A real engine drives it, so a change to the engine's log
// wording that defeats the filter fails here.
func TestCrashTailSkipsEngineEchoes(t *testing.T) {
	for _, tc := range []struct {
		cause  game.Outcome
		cursed bool
		fatal  string // the fatal submit's line, "" if none
	}{
		{game.OutOfLives, false, ": WA 3/10, -1 life, 0 left"},
		{game.OutOfLives, true, ": WA 3/10 on a cursed door, 3 lives left"},
		{game.TimedOut, false, ""},
		{game.GaveUp, false, ""},
	} {
		e := engineCrash(t, tc.cause, tc.cursed)
		log := ""
		for _, l := range e.State.Log {
			log += l.Text + "\n"
		}
		v := ansi.Strip(NewCrash(e).View(140, 40))
		for _, s := range []string{"Kernel panic - not syncing:", "core dumped: "} {
			if n := strings.Count(log, s); n != 1 {
				t.Errorf("%s: engine logged %q %d times, want 1 (did its wording change?)\n%s", tc.cause, s, n, log)
			}
			if n := strings.Count(v, s); n != 1 {
				t.Errorf("%s: crash screen shows %q %d times, want 1:\n%s", tc.cause, s, n, v)
			}
		}
		if tc.fatal != "" {
			if !strings.Contains(log, tc.fatal) {
				t.Errorf("%s: engine log lacks the fatal submit %q:\n%s", tc.cause, tc.fatal, log)
			}
			if strings.Contains(v, tc.fatal) {
				t.Errorf("%s: crash screen repeats the fatal submit:\n%s", tc.cause, v)
			}
		}
		// The rest of the tail stays: the regression the crash scheduled.
		if !strings.Contains(v, "regression: ") {
			t.Errorf("%s: crash screen dropped the regression line:\n%s", tc.cause, v)
		}
		if want := map[bool]string{false: "Out of lives: Killed", true: "Cursed door: Killed"}[tc.cursed]; tc.cause == game.OutOfLives && !strings.Contains(v, want) {
			t.Errorf("%s cursed=%v: want precursor %q:\n%s", tc.cause, tc.cursed, want, v)
		}
		// The curse crashes with lives left, so the panic must not blame
		// the lives.
		if want := map[bool]string{false: "not syncing: out of lives in", true: "not syncing: curse triggered in"}[tc.cursed]; tc.cause == game.OutOfLives && !strings.Contains(v, want) {
			t.Errorf("%s cursed=%v: want panic %q:\n%s", tc.cause, tc.cursed, want, v)
		}
	}
}

func TestCrashReveal(t *testing.T) {
	c := NewCrash(env(fixture.Crash(game.TimedOut)))
	full := c.View(100, 40)
	if c.revealing() {
		t.Fatal("a new crash screen shows everything until Init")
	}
	if c.Init() == nil || !c.revealing() {
		t.Fatal("Init should start the reveal")
	}
	if v := c.View(100, 40); strings.TrimSpace(ansi.Strip(v)) != "" {
		t.Errorf("reveal starts empty, got:\n%s", ansi.Strip(v))
	}
	gen := c.gen
	c.Update(revealMsg{gen: gen})
	c.Update(revealMsg{gen: gen - 1}) // stale: ignored
	if c.shown != 1 {
		t.Errorf("after one tick shown = %d, want 1", c.shown)
	}
	// A key mid-reveal skips to the end and does nothing else.
	if _, cmd := c.Update(press("u")); cmd != nil {
		t.Errorf("key during reveal should only skip, got %#v", emitted(cmd))
	}
	if c.revealing() || c.View(100, 40) != full {
		t.Error("skip should show the full panic")
	}

	// Ticking to the end stops the ticks.
	c.Init()
	var cmd tea.Cmd
	for i := 0; i < 100 && c.revealing(); i++ {
		_, cmd = c.Update(revealMsg{gen: c.gen})
	}
	if c.revealing() || cmd != nil || c.View(100, 40) != full {
		t.Errorf("reveal should end on the full panic with no further tick")
	}
}

func TestClearedKeys(t *testing.T) {
	e := env(fixture.Cleared())
	e.Meta = fixture.ClearedMeta()
	c := NewCleared(e)
	_, cmd := c.Update(press("r"))
	if got := emitted(cmd); got != (screens.StartBoot{Prestige: 2, Preset: "standard"}) {
		t.Errorf("r: got %#v", got)
	}
	_, cmd = c.Update(press("l"))
	if got := emitted(cmd); got != (screens.OpenLog{}) {
		t.Errorf("l: got %#v", got)
	}
	v := ansi.Strip(c.View(80, 24))
	for _, want := range []string{"Boot #42 cleared", "★ prestige unlocked: Gold", "uptime      2:29:52", "1.2 KiB", "7 doors · 2 hinted · 1 retreat"} {
		if !strings.Contains(v, want) {
			t.Errorf("cleared missing %q:\n%s", want, v)
		}
	}
}

func TestMapKeys(t *testing.T) {
	m := NewMap(env(fixture.Doors()))
	for _, k := range []string{"esc", "q", "m"} {
		_, cmd := m.Update(press(k))
		if got := emitted(cmd); got != (screens.Back{}) {
			t.Errorf("%s: got %#v", k, got)
		}
	}
}

func TestRenderMap(t *testing.T) {
	fixtures := []struct {
		name string
		st   *game.State
	}{
		{"route", fixture.Route()},
		{"doors", fixture.Doors()},
		{"merge", fixture.Merge()},
		{"boss", fixture.Boss()},
		{"crash", fixture.Crash(game.TimedOut)},
		{"cleared", fixture.Cleared()},
	}
	var sb strings.Builder
	for _, f := range fixtures {
		for _, w := range []int{80, 50, 30} {
			ls := renderMap(f.st, w)
			for _, l := range ls {
				if ansi.StringWidth(l) > w {
					t.Errorf("%s at %d: line %d wide: %q", f.name, w, ansi.StringWidth(l), ansi.Strip(l))
				}
			}
			fmt.Fprintf(&sb, "--- %s @%d ---\n%s\n", f.name, w, ansi.Strip(strings.Join(ls, "\n")))
		}
	}
	golden(t, "rendermap", sb.String())
	doors := ansi.Strip(strings.Join(renderMap(fixture.Doors(), 80), "\n"))
	if !strings.Contains(doors, "✓ ✓") || !strings.Contains(doors, "✓ ●") || !strings.Contains(doors, "⚔") {
		t.Errorf("doors map: want Userland done, here in the second biome, boss ahead:\n%s", doors)
	}
	if merge := ansi.Strip(strings.Join(renderMap(fixture.Merge(), 80), "\n")); !strings.Contains(merge, "─●─") {
		t.Errorf("merge map should mark the merge window:\n%s", merge)
	}
	if crash := ansi.Strip(strings.Join(renderMap(fixture.Crash(game.TimedOut), 80), "\n")); !strings.Contains(crash, "✓ ☠") {
		t.Errorf("crash map should mark the crash room:\n%s", crash)
	}
}

func TestHUDBar(t *testing.T) {
	type variant struct {
		name string
		st   func() *game.State
	}
	variants := []variant{
		{"open", fixture.Problem},
		{"blind cursed, 90 s left", func() *game.State {
			st := fixture.Problem()
			st.Current.Door.Blind, st.Current.Door.Cursed = true, true
			st.Current.Deadline = fixture.Now.Add(90 * time.Second)
			return st
		}},
		{"blind, one quack", func() *game.State {
			st := fixture.Problem()
			st.Current.Door.Blind, st.Current.Quacks, st.Current.Hint = true, 1, 1
			return st
		}},
		{"two quacks, regression boss", func() *game.State {
			st := fixture.Problem()
			st.Current.Quacks, st.Current.Hint = 2, 2
			st.Current.Door.Regression, st.Current.Door.Boss = true, "heisenbug"
			return st
		}},
		// Tags are on screen, so the engine spends the one quack on the
		// technique: Hint runs ahead of Quacks.
		{"plain, one quack", func() *game.State {
			st := fixture.Problem()
			st.Current.Quacks, st.Current.Hint = 1, 2
			return st
		}},
		{"not submitted", func() *game.State {
			st := fixture.Problem()
			st.Current.Submits, st.Current.BestPass, st.Current.Total, st.Current.LastVerdict = 0, 0, 0, ""
			return st
		}},
		{"no attempt", func() *game.State {
			st := fixture.Problem()
			st.Current = nil
			return st
		}},
		{"first submit judging", func() *game.State {
			st := fixture.Problem()
			st.Current.Submits, st.Current.BestPass, st.Current.Total, st.Current.LastVerdict = 0, 0, 0, ""
			st.Current.SubmitAt = fixture.Now
			return st
		}},
		{"judging again", func() *game.State {
			st := fixture.Problem()
			st.Current.SubmitAt = fixture.Now
			return st
		}},
		// Userland's rule (D44): a timeout here costs a life, not the boot.
		{"Userland, a spare life", func() *game.State {
			st := fixture.Problem()
			st.Biome, st.Lives = 0, 2
			st.Rules.Biomes[0].TimeoutLife = true
			return st
		}},
	}
	prob, _ := fixture.Lookup("usaco:764")
	var sb strings.Builder
	for _, w := range []int{35, 48, 60} {
		for _, v := range variants {
			s := HUDBar(v.st(), prob, fixture.Now, w)
			for _, l := range strings.Split(s, "\n") {
				if ansi.StringWidth(l) > w {
					t.Errorf("%s at %d: %q is %d wide", v.name, w, ansi.Strip(l), ansi.StringWidth(l))
				}
			}
			fmt.Fprintf(&sb, "--- %s @%d ---\n%s\n", v.name, w, ansi.Strip(s))
		}
	}
	golden(t, "hudbar", sb.String())

	open := ansi.Strip(HUDBar(fixture.Problem(), prob, fixture.Now, 48))
	for _, want := range []string{"⏱ 23:41", "♥♡♡", "🦆1", "96 B", "Functional Graphs"} {
		if !strings.Contains(open, want) {
			t.Errorf("hud missing %q:\n%s", want, open)
		}
	}
	crit := HUDBar(variants[1].st(), prob, fixture.Now, 48)
	if !strings.Contains(crit, ui.Timer(90*time.Second, variants[1].st().Current.Door.Timer)) {
		t.Errorf("under 2 min the timer should be critical (reverse): %q", crit)
	}
	if s := ansi.Strip(crit); !strings.Contains(s, "tags hidden") || strings.Contains(s, "Functional") {
		t.Errorf("blind door with no quacks must hide tags:\n%s", s)
	}
	if s := ansi.Strip(HUDBar(variants[2].st(), prob, fixture.Now, 48)); !strings.Contains(s, "Functional Graphs") || strings.Contains(s, "technique") {
		t.Errorf("one quack reveals tags, not the technique:\n%s", s)
	}
	if s := ansi.Strip(HUDBar(variants[3].st(), prob, fixture.Now, 48)); !strings.Contains(s, "technique graphs") {
		t.Errorf("two quacks reveal the technique:\n%s", s)
	}
	// The HP bar is the HUD's, or the problem screen's without one (C6):
	// the vitals never draw a second.
	for _, v := range variants {
		if s := ansi.Strip(HUDBar(v.st(), prob, fixture.Now, 48)); strings.ContainsAny(s, "▓░") {
			t.Errorf("%s: the vitals draw an HP bar:\n%s", v.name, s)
		}
	}
}

// The bar shows what the engine revealed (Attempt.Hint), whatever the duck
// count says. A real engine drives it: on a plain door one quack reveals
// the technique, on a blind door one quack reveals only the tags.
func TestHUDBarFollowsEngineReveal(t *testing.T) {
	prob, _ := fixture.Lookup("usaco:764")
	for _, tc := range []struct {
		name            string
		blind           bool
		quacks          int
		tags, technique bool
	}{
		{"plain, no quack", false, 0, true, false},
		{"plain, one quack", false, 1, true, true},
		{"blind, no quack", true, 0, false, false},
		{"blind, one quack", true, 1, true, false},
		{"blind, two quacks", true, 2, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := fixture.Problem()
			st.Current.Door.Blind = tc.blind
			st.Ducks = 2
			meta := fixture.Meta()
			for range tc.quacks {
				if _, err := game.Step(st, &meta, nil, game.Quack{}, fixture.Now); err != nil {
					t.Fatalf("quack: %v", err)
				}
			}
			s := ansi.Strip(HUDBar(st, prob, fixture.Now, 48))
			if got := strings.Contains(s, "Functional Graphs"); got != tc.tags {
				t.Errorf("tags shown = %v, want %v (Quacks %d, Hint %d):\n%s", got, tc.tags, st.Current.Quacks, st.Current.Hint, s)
			}
			if got := strings.Contains(s, "technique graphs"); got != tc.technique {
				t.Errorf("technique shown = %v, want %v (Quacks %d, Hint %d):\n%s", got, tc.technique, st.Current.Quacks, st.Current.Hint, s)
			}
		})
	}
}

func TestBiomeLabel(t *testing.T) {
	for _, tc := range []struct {
		name string
		b    game.BiomeState
		want string
	}{
		{"engine name", game.BiomeState{Name: "Userland", Topic: "mixed", Kind: rules.Fixed}, "Userland"},
		{"topic", game.BiomeState{Topic: "dp", Kind: rules.Choice}, "The Cache"},
		{"topic without flavor", game.BiomeState{Topic: "geometry", Kind: rules.Choice}, "geometry"},
		// A boss biome's topic is mixed, whose flavor name is the first
		// biome's: the boss names it instead.
		{"boss", game.BiomeState{Topic: "mixed", Kind: rules.Boss, Boss: "heisenbug"}, "Heisenbug"},
		{"final boss", game.BiomeState{Topic: "mixed", Kind: rules.Boss, Boss: game.FinalBoss}, "The Halting Problem"},
		{"boss without flavor", game.BiomeState{Topic: "mixed", Kind: rules.Boss, Boss: "oom-killer"}, "oom-killer"},
		{"boss named by preset", game.BiomeState{Name: "Ring 0", Topic: "mixed", Kind: rules.Boss, Boss: "heisenbug"}, "Ring 0"},
		{"boss without key", game.BiomeState{Kind: rules.Boss}, "boss"},
		{"choice not taken", game.BiomeState{Kind: rules.Choice}, "?"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &game.State{Biomes: []game.BiomeState{tc.b}}
			if got := biomeLabel(st, 0); got != tc.want {
				t.Errorf("biomeLabel = %q, want %q", got, tc.want)
			}
		})
	}

	// Where QA saw "Userland" for the boss: the map, the merge window's
	// leave row, and the boss room's header.
	if v := ansi.Strip(NewMerge(env(fixture.Merge())).View(140, 40)); !strings.Contains(v, "leave → biome 3: route choice") {
		t.Errorf("merge before biome 3 should lead to the route choice:\n%s", v)
	}
	st := fixture.Route3()
	st.Phase, st.Biome, st.Room = game.PhaseMerge, 3, 0
	st.Biomes[2].Topic = "graphs"
	if v := ansi.Strip(NewMerge(env(st)).View(140, 40)); !strings.Contains(v, "leave → boss: Heisenbug") {
		t.Errorf("merge before the boss should name it:\n%s", v)
	}
	v := ansi.Strip(NewDoors(env(fixture.Boss())).View(140, 40))
	for _, want := range []string{"Silver · ⚔ Heisenbug", "─ Heisenbug ─"} {
		if !strings.Contains(v, want) {
			t.Errorf("boss doors missing %q:\n%s", want, v)
		}
	}
	if strings.Count(v, "Userland") != 1 {
		t.Errorf("only the first biome is Userland:\n%s", v)
	}
}

func TestStepsPlaceAttempts(t *testing.T) {
	ss := steps(fixture.Crash(game.TimedOut))
	want := [][2]int{{0, 0}, {0, 1}, {0, 1}, {1, 0}, {1, 1}}
	if len(ss) != len(want) {
		t.Fatalf("got %d steps, want %d", len(ss), len(want))
	}
	for i, s := range ss {
		if [2]int{s.Biome, s.Room} != want[i] || s.N != i+1 {
			t.Errorf("step %d at biome %d room %d (N %d), want %v", i, s.Biome, s.Room, s.N, want[i])
		}
	}
}

// The route band previews the rating the engine aims the biome's doors at:
// raised by the prestige offset, and from the topic's own rating only once
// that topic has TopicRatingMin attempts behind it.
func TestRouteBandFollowsEngineAim(t *testing.T) {
	st := fixture.Route()
	st.Prestige = 4
	e := env(st)
	e.Meta.TopicAttempts = map[string]int{"dp": 2, "data-structures": 9}
	v := ansi.Strip(NewRoute(e).View(140, 40))
	// dp, two attempts: the overall 1180 aims, 1033 at 70%, +4·25 = 1133.
	// data-structures: its own 1240, 1093, +100 = 1193.
	for _, want := range []string{"◆1050–1250", "◆1100–1300", "your data-structures rating 1240"} {
		if !strings.Contains(v, want) {
			t.Errorf("route missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "your dp rating") {
		t.Errorf("dp doors aim by the overall rating, not dp's:\n%s", v)
	}
}

// The upgrade screen shows the -O3 the engine awarded, which counts a clear
// at exactly the fraction, and centers the cards.
func TestUpgradeScreenO3AndCentering(t *testing.T) {
	st := fixture.Upgrade()
	a := &st.History[len(st.History)-1]
	a.Closed = a.Opened.Add(time.Duration(float64(a.Door.Timer) * st.Rules.O3Frac))
	v := ansi.Strip(NewUpgradeScreen(env(st)).View(140, 40))
	if !strings.Contains(v, "in 15:00 of 30:00  -O3") {
		t.Errorf("an -O3 clear at exactly the fraction should say so:\n%s", v)
	}
	for _, l := range strings.Split(v, "\n") {
		if !strings.Contains(l, "▸ 1") {
			continue
		}
		left := len(l) - len(strings.TrimLeft(l, " "))
		right := 140 - ansi.StringWidth(l)
		if left-right > 1 || right-left > 1 {
			t.Errorf("cards sit %d from the left and %d from the right:\n%s", left, right, v)
		}
	}
}

// The reveal ends when the screen has printed everything it shows at its
// size; a key after that acts instead of only skipping.
func TestCrashRevealEndsWithScreen(t *testing.T) {
	c := NewCrash(crashEnv(game.TimedOut))
	full := c.View(80, 16)
	c.Init()
	for i := 0; i < 100 && c.View(80, 16) != full; i++ {
		c.Update(revealMsg{gen: c.gen})
	}
	if c.revealing() {
		t.Error("still revealing with the whole panic on screen")
	}
	if _, cmd := c.Update(press("u")); emitted(cmd) == nil {
		t.Error("u after the reveal should upsolve")
	}
}

// A clear that unlocks nothing says what the next prestige still needs
// (D31): rated doors first, then a clear at the highest unlocked. A boot
// played below the top names the top as the one to clear.
func TestClearedGate(t *testing.T) {
	need := fixture.Cleared().Rules.CalibAfter
	tests := []struct {
		prestige, top, rated int
		want                 string
	}{
		{1, 1, need - 26, fmt.Sprintf("Gold ▓▓▓▓░░░░ %d/%d rated doors, then a clear", need-26, need)},
		{0, 1, need, "Gold opens with a clear at Silver"},
	}
	for _, tc := range tests {
		st := fixture.Cleared()
		st.Prestige, st.Unlocked = tc.prestige, 0
		m := fixture.ClearedMeta()
		m.Prestige, m.Rated, m.TopicAttempts = tc.top, tc.rated, nil
		e := env(st)
		e.Meta = m
		v := ansi.Strip(NewCleared(e).View(100, 30))
		if strings.Contains(v, "prestige unlocked") {
			t.Errorf("%+v: claims an unlock:\n%s", tc, v)
		}
		if tc.want != "" && !strings.Contains(v, tc.want) {
			t.Errorf("%+v: missing %q:\n%s", tc, tc.want, v)
		}
	}
}

// A door names its source without saying the site twice; the fixtures are
// all USACO, so Codeforces is checked here.
func TestSourceLabel(t *testing.T) {
	for _, c := range []struct {
		id, contest string
		mystery     bool
		want        string
	}{
		{"cf:1462A", "Codeforces Round 690 (Div. 3) · A", false, "CF Round 690 (Div. 3) · A"},
		{"cf:1463A", "Educational Codeforces Round 100 (Rated for Div. 2) · A", false, "Educational CF Round 100 (Rated for Div. 2) · A"},
		{"cf:1170A", "VK Cup 2019 · A", false, "CF · VK Cup 2019 · A"},
		{"cf:2000Z", "Codeforces · Z", false, "CF · Z"},
		{"cf:1462A", "Codeforces Round 690 (Div. 3) · A", true, "CF"},
		{"usaco:764", "2017 December, Silver P1", false, "USACO · 2017 December, Silver P1"},
		{"cses:1094", "", false, "CSES"},
	} {
		id := catalog.ID(c.id)
		p := catalog.Problem{ID: id, Source: id.Source(), Contest: c.contest}
		if got := sourceLabel(p, !c.mystery); got != c.want {
			t.Errorf("sourceLabel(%s, %q) = %q, want %q", c.id, c.contest, got, c.want)
		}
	}
}

// fork says how many doors a room has without it, which is more with tmux
// installed: the engine does not stack fork on tmux's door.
func TestForkSaysTheUsualRoomSize(t *testing.T) {
	fl = realFlavor
	defer func() { fl = stubFlavor }()
	st := fixture.Merge()
	usual := st.Rules.DoorsPerRoom
	for _, c := range []struct {
		pkgs []string
		want int
	}{{nil, usual}, {[]string{"tmux"}, usual + st.Rules.Grants([]string{"tmux"}).Doors}} {
		st.Packages = c.pkgs
		v := ansi.Strip(NewMerge(env(st)).View(140, 40))
		if want := fmt.Sprintf("(normally %d)", c.want); !strings.Contains(v, want) {
			t.Errorf("packages %v: fork should say %q:\n%s", c.pkgs, want, v)
		}
	}
}

// An overtime clear pays less (State.Paid, as the strip says): the upgrade
// screen says what it paid, not the door's full reward.
func TestUpgradeScreenOvertimePay(t *testing.T) {
	st := fixture.Upgrade()
	st.Rules.OnExpire = rules.ExpireOvertime
	a := &st.History[len(st.History)-1]
	a.Overtime, a.O3 = true, false
	paid := st.Paid(*a)
	if paid == a.Door.Bytes {
		t.Fatalf("the fixture's overtime clear pays its full %d B", paid)
	}
	v := ansi.Strip(NewUpgradeScreen(env(st)).View(140, 40))
	if want := "+" + units.Bytes(paid) + " uncommitted"; !strings.Contains(v, want) {
		t.Errorf("an overtime clear should show %q, what it paid:\n%s", want, v)
	}
}

// Past a soft timer's deadline (a trial) the door goes on: the vitals say
// by how much, as the editor pane's border does, not a frozen 00:00.
func TestHUDBarOvertime(t *testing.T) {
	st := fixture.Problem()
	st.Rules.OnExpire = rules.ExpireOvertime
	prob, _ := fixture.Lookup("usaco:764")
	now := st.Current.Deadline.Add(3*time.Minute + 12*time.Second)
	if s := ansi.Strip(HUDBar(st, prob, now, 48)); !strings.Contains(s, ui.G.Timer+"+03:12 over") || strings.Contains(s, "00:00") {
		t.Errorf("3:12 past a soft deadline should read +03:12 over:\n%s", s)
	}
	st.Rules.OnExpire = rules.ExpireCrash
	if s := ansi.Strip(HUDBar(st, prob, now, 48)); !strings.Contains(s, ui.G.Timer+"00:00") {
		t.Errorf("a hard timer past its deadline reads 00:00:\n%s", s)
	}
}

// The one-line doors and routes, a short terminal's last resort, cut the
// title before the numbers: every row keeps its rating, timer and reward,
// or its rating band (§2.6).
func TestCompactRowsKeepTheNumbers(t *testing.T) {
	st := fixture.Doors()
	v := ansi.Strip(NewDoors(env(st)).View(40, 16))
	for _, d := range st.Doors {
		if s := ansi.Strip(doorStats(d)); !strings.Contains(v, s) {
			t.Errorf("door %s lost %q:\n%s", d.Problem, s, v)
		}
	}
	r := NewRoute(env(fixture.Route()))
	v = ansi.Strip(r.View(36, 10))
	for _, topic := range r.offers() {
		if s := ansi.Strip(r.describe(topic).band); !strings.Contains(v, s) {
			t.Errorf("route %s lost %q:\n%s", topic, s, v)
		}
	}
}

// Under the ASCII glyph set no screen falls back on a literal "·": every
// separator is ui.G.Sep (§2.4).
func TestASCIISeparators(t *testing.T) {
	ui.SetGlyphs(ui.ASCII)
	defer ui.SetGlyphs(ui.Unicode)
	for _, c := range append(cases(), endCases()...) {
		for _, sz := range goldenSizes {
			if v := ansi.Strip(c.make().View(sz[0], sz[1])); strings.Contains(v, "·") {
				t.Errorf("%s at %dx%d draws a literal ·:\n%s", c.name, sz[0], sz[1], v)
			}
		}
	}
	st := fixture.Problem()
	st.Current.Door.Blind, st.Current.Door.Cursed = true, true
	prob, _ := fixture.Lookup("usaco:764")
	if v := ansi.Strip(HUDBar(st, prob, fixture.Now, 48)); strings.Contains(v, "·") {
		t.Errorf("the HUD bar draws a literal ·:\n%s", v)
	}
}
