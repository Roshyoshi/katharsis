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
}

func TestMain(m *testing.M) {
	flag.Parse()
	fl = stubFlavor
	os.Exit(m.Run())
}

func env(st *game.State) Env {
	return Env{State: st, Meta: fixture.Meta(), Lookup: fixture.Lookup, Now: fixture.Now}
}

type screenCase struct {
	name string
	make func() screens.Screen
}

func cases() []screenCase {
	return []screenCase{
		{"route", func() screens.Screen { return NewRoute(env(fixture.Route())) }},
		{"route3", func() screens.Screen { return NewRoute(env(fixture.Route3())) }},
		{"doors", func() screens.Screen { return NewDoors(env(fixture.Doors())) }},
		{"doors_fork", func() screens.Screen { return NewDoors(env(fixture.DoorsFork())) }},
		{"doors_boss", func() screens.Screen { return NewDoors(env(fixture.Boss())) }},
		{"doors_map", func() screens.Screen {
			d := NewDoors(env(fixture.Doors()))
			d.Update(press("m"))
			return d
		}},
		{"map", func() screens.Screen { return NewMap(env(fixture.Doors())) }},
		{"map_crashed", func() screens.Screen { return NewMap(env(fixture.Crash(game.TimedOut))) }},
		{"merge", func() screens.Screen { return NewMerge(env(fixture.Merge())) }},
		{"upgrade", func() screens.Screen { return NewUpgradeScreen(env(fixture.Upgrade())) }},
		{"crash_timer", func() screens.Screen { return NewCrash(env(fixture.Crash(game.TimedOut))) }},
		{"crash_lives", func() screens.Screen { return NewCrash(env(fixture.Crash(game.OutOfLives))) }},
		{"crash_quit", func() screens.Screen { return NewCrash(env(fixture.Crash(game.GaveUp))) }},
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
	if r.Selected() != 0 {
		t.Errorf("1, 9: selected %d, want 0", r.Selected())
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
	if d.Selected() != 0 {
		t.Errorf("left past the first door: selected %d", d.Selected())
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
		"The Bovine Shuffle", "Functional Graphs · Cycle Detection", "◆ 1300", "⏱ 30:00", "+80 B",
		"☠ cursed", "tags hidden", "☠ no ducks · 1 WA = crash · ×2", "◌ blind: tags hidden · ×1.5",
		"???", "? mystery: title and tags hidden · ×2", "USACO · 2016 US Open, Silver P3",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("doors missing %q", want)
		}
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

func TestMergeRefreshMovesPastTakenPick(t *testing.T) {
	st := fixture.Merge()
	m := NewMerge(env(st))
	m.Update(press("down")) // ECC
	st2 := fixture.Merge()
	st2.PendingUpgrade, st2.UpgradeOffers = false, nil
	m.Refresh(env(st2))
	_, cmd := m.Update(press("enter"))
	if got := emitted(cmd); got != (game.PickDaemon{Daemon: game.Nice}) {
		t.Errorf("after the upgrade was taken the cursor should sit on the first daemon, got %#v", got)
	}
}

func TestMergeDaemonSlotsFull(t *testing.T) {
	st := fixture.Merge()
	st.PendingUpgrade = false
	st.Daemons = []game.Daemon{game.Strace, game.Kexec, game.Fork}
	m := NewMerge(env(st))
	_, cmd := m.Update(press("enter"))
	if got := emitted(cmd); got != (game.LeaveMerge{}) {
		t.Errorf("with full slots the only row is leave, got %#v", got)
	}
	if v := ansi.Strip(m.View(80, 24)); !strings.Contains(v, "all daemon slots in use") {
		t.Errorf("full slots not explained:\n%s", v)
	}
}

func TestUpgradePicker(t *testing.T) {
	p := NewUpgradePicker([]game.Upgrade{game.Overclock, game.ECC, game.Duck}, fixture.Rules())
	p, _ = p.Update(press("right"))
	p, cmd := p.Update(press("enter"))
	if got := emitted(cmd); got != (game.PickUpgrade{Upgrade: game.ECC}) {
		t.Errorf("right, enter: got %#v", got)
	}
	if v := ansi.Strip(p.View(100)); !strings.Contains(v, "▸ 2") || !strings.Contains(v, "+1 life") {
		t.Errorf("cards view:\n%s", v)
	}
	if v := ansi.Strip(p.View(40)); !strings.Contains(v, "▸ ECC") {
		t.Errorf("narrow view should fall back to a list:\n%s", v)
	}

	u := NewUpgradeScreen(env(fixture.Upgrade()))
	u.Update(press("3"))
	_, cmd = u.Update(press("enter"))
	if got := emitted(cmd); got != (game.PickUpgrade{Upgrade: game.Duck}) {
		t.Errorf("screen 3, enter: got %#v", got)
	}
	if v := ansi.Strip(u.View(80, 24)); !strings.Contains(v, "✓ AC  Moocast  in 14:02 of 30:00  -O3") {
		t.Errorf("upgrade screen should name the clear:\n%s", v)
	}
}

func TestCrashKeys(t *testing.T) {
	c := NewCrash(env(fixture.Crash(game.TimedOut)))
	for k, want := range map[string]tea.Msg{
		"u": screens.Upsolve{Problem: "usaco:644"},
		"r": screens.StartBoot{Prestige: 1, Preset: "standard"},
		"l": screens.OpenLog{},
		"q": screens.Quit{},
	} {
		_, cmd := c.Update(press(k))
		if got := emitted(cmd); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v, want %#v", k, got, want)
		}
	}
}

func TestCrashContent(t *testing.T) {
	v := ansi.Strip(NewCrash(env(fixture.Crash(game.TimedOut))).View(100, 40))
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
		`core dumped: 96 B on "Closing the Farm" · [u] upsolve  [r] reboot  [l] git log`,
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
	v := ansi.Strip(NewCrash(env(fixture.Crash(game.TimedOut))).View(80, 16))
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
		{game.OutOfLives, true, ": WA 3/10, -1 life, 2 left"},
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
	}
}

func TestCrashReveal(t *testing.T) {
	c := NewCrash(env(fixture.Crash(game.TimedOut)))
	full := c.View(100, 40)
	if c.Revealing() {
		t.Fatal("a new crash screen shows everything until Init")
	}
	if c.Init() == nil || !c.Revealing() {
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
	if c.Revealing() || c.View(100, 40) != full {
		t.Error("skip should show the full panic")
	}

	// Ticking to the end stops the ticks.
	c.Init()
	var cmd tea.Cmd
	for i := 0; i < 100 && c.Revealing(); i++ {
		_, cmd = c.Update(revealMsg{gen: c.gen})
	}
	if c.Revealing() || cmd != nil || c.View(100, 40) != full {
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
			s := RenderMap(f.st, w)
			for _, l := range strings.Split(s, "\n") {
				if ansi.StringWidth(l) > w {
					t.Errorf("%s at %d: line %d wide: %q", f.name, w, ansi.StringWidth(l), ansi.Strip(l))
				}
			}
			fmt.Fprintf(&sb, "--- %s @%d ---\n%s\n", f.name, w, ansi.Strip(s))
		}
	}
	golden(t, "rendermap", sb.String())
	doors := ansi.Strip(RenderMap(fixture.Doors(), 80))
	if !strings.Contains(doors, "✓ ✓") || !strings.Contains(doors, "✓ ●") || !strings.Contains(doors, "⚔") {
		t.Errorf("doors map: want Userland done, here in the second biome, boss ahead:\n%s", doors)
	}
	if merge := ansi.Strip(RenderMap(fixture.Merge(), 80)); !strings.Contains(merge, "─●─") {
		t.Errorf("merge map should mark the merge window:\n%s", merge)
	}
	if crash := ansi.Strip(RenderMap(fixture.Crash(game.TimedOut), 80)); !strings.Contains(crash, "✓ ☠") {
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
			st.Current.Door.Blind, st.Current.Quacks = true, 1
			return st
		}},
		{"two quacks, regression boss", func() *game.State {
			st := fixture.Problem()
			st.Current.Quacks = 2
			st.Current.Door.Regression, st.Current.Door.Boss = true, "heisenbug"
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
	for _, want := range []string{"⏱ 23:41", "♥♡♡", "🦆1", "96 B", "▓", "37/52", "Functional Graphs"} {
		if !strings.Contains(open, want) {
			t.Errorf("hud missing %q:\n%s", want, open)
		}
	}
	crit := HUDBar(variants[1].st(), prob, fixture.Now, 48)
	if !strings.Contains(crit, toneStyle[toneCrit].Render("⏱ 01:30")) {
		t.Errorf("under 2 min the timer should blink: %q", crit)
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
}

func TestTimerTone(t *testing.T) {
	m := time.Minute
	for _, c := range []struct {
		left, total time.Duration
		want        tone
	}{
		{20 * m, 30 * m, toneCalm},
		{15 * m, 30 * m, toneWarn}, // exactly half is no longer calm
		{10 * m, 30 * m, toneWarn},
		{6 * m, 30 * m, toneLow}, // exactly a fifth
		{3 * m, 30 * m, toneLow},
		{119 * time.Second, 30 * m, toneCrit},
		{-time.Second, 30 * m, toneCrit},
		{5 * m, 0, toneCalm},
	} {
		if got := timerTone(c.left, c.total); got != c.want {
			t.Errorf("timerTone(%v, %v) = %v, want %v", c.left, c.total, got, c.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0 B", 312: "312 B", 1023: "1023 B", 1024: "1 KiB", 1536: "1.5 KiB",
		10 << 10: "10 KiB", 150 << 10: "150 KiB", 1 << 20: "1 MiB", 3 << 19: "1.5 MiB", -312: "-312 B",
	} {
		if got := FormatBytes(n); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", n, got, want)
		}
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

func TestNilState(t *testing.T) {
	e := Env{}
	for _, s := range []screens.Screen{NewRoute(e), NewDoors(e), NewMap(e), NewMerge(e), NewUpgradeScreen(e), NewCrash(e), NewCleared(e)} {
		checkFits(t, fmt.Sprintf("%T", s), s.View(80, 24), 80, 24)
		s.Update(press("enter"))
	}
	if RenderMap(nil, 80) != "" {
		t.Error("RenderMap(nil) should be empty")
	}
	if HUDBar(nil, catalog.Problem{}, fixture.Now, 40) == "" {
		t.Error("HUDBar(nil) should still say something")
	}
}
