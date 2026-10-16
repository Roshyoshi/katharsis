package boot

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot/fixture"
	"katharsis/internal/units"
)

// The in-door screens on the three-act map in cycles (D47-D54,
// docs/design/map.md): the boot fixture's map boot, every screen at the
// golden sizes and the fit sizes.

func cyclesCases() []screenCase {
	cs := []screenCase{
		{"cycles_room", func() screens.Screen { return NewDoors(env(fixture.CyclesRoom())) }},
		{"cycles_boss_retry", func() screens.Screen { return NewDoors(env(fixture.CyclesBossRetry())) }},
		{"cycles_merge_drink", func() screens.Screen { return NewMerge(env(fixture.CyclesMerge("drink"))) }},
		{"cycles_merge_full", func() screens.Screen { return NewMerge(env(fixture.CyclesMerge("full"))) }},
		{"cycles_merge_dry", func() screens.Screen { return NewMerge(env(fixture.CyclesMerge("dry"))) }},
		{"cycles_upgrade", func() screens.Screen { return NewUpgradeScreen(env(fixture.CyclesUpgrade())) }},
		{"cycles_crash_timeout", func() screens.Screen { return NewCrash(env(fixture.CyclesCrash("timeout"))) }},
		{"cycles_crash_submits", func() screens.Screen { return NewCrash(env(fixture.CyclesCrash("submits"))) }},
		{"cycles_crash_final", func() screens.Screen { return NewCrash(env(fixture.CyclesCrash("final"))) }},
		{"cycles_cleared", func() screens.Screen {
			e := env(fixture.CyclesCleared())
			e.Meta = fixture.ClearedMeta()
			return NewCleared(e)
		}},
	}
	for _, k := range append(slices.Clone(game.BossKeys), game.FinalBoss) {
		cs = append(cs, screenCase{"cycles_boss_" + k, func() screens.Screen { return NewDoors(env(fixture.CyclesBoss(k))) }})
	}
	return cs
}

func TestCyclesGoldens(t *testing.T) {
	for _, c := range cyclesCases() {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				golden(t, name, ansi.Strip(c.make().View(sz[0], sz[1]))+"\n")
			})
		}
	}
}

func TestCyclesScreensFit(t *testing.T) {
	for _, src := range []struct {
		name string
		fl   flavorSource
	}{{"stub", stubFlavor}, {"real", realFlavor}} {
		fl = src.fl
		for _, c := range cyclesCases() {
			for _, sz := range fitSizes {
				s := c.make()
				for range 4 {
					checkFits(t, src.name+"/"+c.name, s.View(sz[0], sz[1]), sz[0], sz[1])
					s, _ = s.Update(press("down"))
				}
			}
		}
	}
	fl = stubFlavor
}

// flat is a view with its frames and runs of space folded, for a phrase
// a card may have wrapped.
func flat(v string) string {
	for _, wall := range []string{"║", "│", "┃"} {
		v = strings.ReplaceAll(v, wall, " ")
	}
	return strings.Join(strings.Fields(v), " ")
}

// TestCyclesScreensSay: the prices and rules each screen states, in the
// boot's numbers (map.md §(d)).
func TestCyclesScreensSay(t *testing.T) {
	view := func(s screens.Screen) string { return flat(ansi.Strip(s.View(140, 40))) }
	for _, c := range []struct {
		name string
		s    screens.Screen
		want []string
	}{
		{"room", NewDoors(env(fixture.CyclesRoom())), []string{"♥ 24/50 cycles", "⏱ a timeout or :q! burns 12 cycles"}},
		{"boss I", NewDoors(env(fixture.CyclesBoss("off-by-one"))), []string{
			"Stub line for boss.rule.off-by-one.", "⚔ one test short burns ×2",
			"a fail or :q! burns 20 cycles and deals a new boss door (4 left)"}},
		{"OOM Killer", NewDoors(env(fixture.CyclesBoss("oom-killer"))), []string{"⚔ memory ×0.5, samples without ASan"}},
		{"Heisenbug", NewDoors(env(fixture.CyclesBoss("heisenbug"))), []string{"⚔ samples say only pass or fail"}},
		{"Race Condition", NewDoors(env(fixture.CyclesBoss("race-condition"))), []string{"⚔ clock ×1.25 fast, pays ×4"}},
		{"Undefined Behavior", NewDoors(env(fixture.CyclesBoss("undefined-behavior"))), []string{"⚔ samples run without sanitizers"}},
		{"final", NewDoors(env(fixture.CyclesBoss(game.FinalBoss))), []string{
			"⚔ clock hidden, warned at 05:00 and 01:00", "one shot: a fail or :q! ends the boot", "Stub line for boss.oneshot."}},
		{"retry", NewDoors(env(fixture.CyclesBossRetry())), []string{"Stub line for boss.retry.", "(2 left)"}},
		{"fountain", NewMerge(env(fixture.CyclesMerge("drink"))), []string{"fountain +5 cycles, free, up to 30 ✓", "fsck +10 cycles now · 64 B", "f fsck"}},
		{"fountain full", NewMerge(env(fixture.CyclesMerge("full"))), []string{"fountain +5 cycles up to 30: yours are at the line, none taken"}},
		{"fountain dry", NewMerge(env(fixture.CyclesMerge("dry"))), []string{"fountain dry at Advanced: no heal between acts", "fsck +5 cycles now"}},
		{"crash timeout", NewCrash(env(fixture.CyclesCrash("timeout"))), []string{
			"Out of cycles: the timeout burns 12, 8 were left", "its 12 cycles were the last", "♥ 30 cycles"}},
		{"crash submits", NewCrash(env(fixture.CyclesCrash("submits"))), []string{
			"Out of cycles: Killed process 1", "3 failed submits burned the last 6 cycles"}},
		{"crash final", NewCrash(env(fixture.CyclesCrash("final"))), []string{
			"halting_problem: one shot, no second door", "one shot, no second door"}},
		{"cleared", NewCleared(env(fixture.CyclesCleared())), []string{
			"cycles left ♥ 14/50 cycles · fountains +10", "1 lost", "1 boss retry"}},
	} {
		v := view(c.s)
		for _, w := range c.want {
			if !strings.Contains(v, w) {
				t.Errorf("%s: missing %q:\n%s", c.name, w, v)
			}
		}
		if v := strings.ReplaceAll(v, "out-of-lives", ""); strings.Contains(v, "lives") || strings.Contains(v, " life") { // hook keys keep their names
			t.Errorf("%s still says lives:\n%s", c.name, v)
		}
	}
}

// The fountain's row says what it poured, and stays true once fsck or an
// Overclock moves the cycles in the same window: it drew "before" from the
// cycles as they stand, so fsck after a 17 → 22 pour read "27 → 32".
func TestFountainRowAfterSpend(t *testing.T) {
	st := fixture.CyclesMerge("drink")
	st.Cycles += 10 // fsck
	v := flat(ansi.Strip(NewMerge(env(st)).View(140, 40)))
	if !strings.Contains(v, "fountain +5 cycles, free, up to 30 ✓") || strings.Contains(v, "27 → 32") {
		t.Errorf("fountain row after fsck:\n%s", v)
	}
}

// The reboot's pool is the one its preset deals (End.Next): a boot saved
// before cycles crashed on its old snapshot, 3 lives, and the next step
// said "♥ 2 cycles" for a map reboot at P4 that deals 25.
func TestRebootPoolFromPreset(t *testing.T) {
	st := fixture.CyclesCrash("timeout")
	st.Rules.Cycles, st.Prestige = 3, 4
	e := env(st)
	next := rules.Map()
	e.End.Next = &next
	if v := flat(ansi.Strip(NewCrash(e).View(140, 40))); !strings.Contains(v, "reboot at Advanced") || !strings.Contains(v, "♥ 25 cycles") {
		t.Errorf("crash's next step:\n%s", v)
	}
}

// TestCyclesHUDBar: the HUD bar in a map door, at the pane's widths: the
// cycles, the act's price for a timeout or :q!, each boss's twist and rule,
// and The Halting Problem's hidden clock, which no countdown gives away.
func TestCyclesHUDBar(t *testing.T) {
	var sb strings.Builder
	type variant struct {
		name string
		st   *game.State
	}
	vs := []variant{{"act II room", fixture.CyclesProblem(fixture.CyclesRoom())}}
	for _, k := range append(slices.Clone(game.BossKeys), game.FinalBoss) {
		vs = append(vs, variant{k, fixture.CyclesProblem(fixture.CyclesBoss(k))})
	}
	for _, v := range vs {
		prob, _ := fixture.Lookup(v.st.Current.Door.Problem)
		for _, w := range []int{36, 48} {
			s := HUDBar(v.st, prob, fixture.Now, w)
			for _, l := range strings.Split(s, "\n") {
				if ansi.StringWidth(l) > w {
					t.Errorf("%s at %d: %q is %d wide", v.name, w, ansi.Strip(l), ansi.StringWidth(l))
				}
			}
			fmt.Fprintf(&sb, "--- %s @%d ---\n%s\n", v.name, w, ansi.Strip(s))
		}
	}
	golden(t, "hudbar_cycles", sb.String())

	final := fixture.CyclesProblem(fixture.CyclesBoss(game.FinalBoss))
	prob, _ := fixture.Lookup(final.Current.Door.Problem)
	left := units.Countdown(final.Current.Deadline.Sub(fixture.Now))
	if s := ansi.Strip(HUDBar(final, prob, fixture.Now, 48)); strings.Contains(s, left) || !strings.Contains(s, "⏱ ??:??") {
		t.Errorf("the hidden clock shows %s:\n%s", left, s)
	}
}

// stripStep shows a tracker p, then n, and returns the events n makes.
func stripStep(t *testing.T, p, n *game.State) []Event {
	t.Helper()
	m := fixture.Meta()
	var tr Tracker
	tr.Lookup = fixture.Lookup
	tr.Observe(p, &m, fixture.Now)
	return tr.Observe(n, &m, fixture.Now)
}

// TestCyclesStrip: the strip says each cost as it happens (D47), with the
// story's line for it.
func TestCyclesStrip(t *testing.T) {
	open := func() *game.State { return fixture.CyclesProblem(fixture.CyclesRoom()) }
	type step struct {
		name       string
		change     func(n *game.State)
		kind, hook string
		text       string
	}
	for _, c := range []step{
		{"a failed submit", func(n *game.State) {
			n.Current.Submits, n.Current.LastVerdict, n.Cycles = 1, "WA", n.Cycles-2
		}, EvVerdict, "verdict.WA", "judge: WA · burned 2 cycles, 22 left"},
		{"ECC", func(n *game.State) {
			n.Upgrades = append(n.Upgrades, game.ECC)
			n.Current.Submits, n.Current.LastVerdict = 1, "WA"
			n.Current.Deadline = n.Current.Deadline.Add(-5 * time.Minute)
		}, EvVerdict, "verdict.WA.ecc", "judge: WA · ECC: -05:00 on the clock, 14:50 left"},
		{"a timeout", func(n *game.State) {
			a := *n.Current
			a.Outcome, a.Closed = game.TimedOut, a.Deadline
			n.History, n.Current, n.Phase, n.Cycles = append(n.History, a), nil, game.PhaseMerge, n.Cycles-12
		}, EvTimeout, "door.timeout", "⏱ timer ran out: burned 12 cycles · room lost · the boot goes on"},
		{"a give-up", func(n *game.State) {
			a := *n.Current
			a.Outcome, a.Closed = game.GaveUp, fixture.Now
			n.History, n.Current, n.Phase, n.Cycles = append(n.History, a), nil, game.PhaseMerge, n.Cycles-12
		}, EvGaveUp, "door.gave-up", "✗ gave up: burned 12 cycles · room lost · the boot goes on"},
		{"a retreat", func(n *game.State) {
			a := *n.Current
			a.Outcome, a.Closed = game.Retreated, fixture.Now
			n.History, n.Current, n.Phase, n.Cycles = append(n.History, a), nil, game.PhaseDoors, n.Cycles-5
		}, EvRetreat, "door.retreat", "↩ retreat: burned 5 cycles, 19 left · new doors"},
	} {
		p, n := open(), open()
		c.change(n)
		evs := stripStep(t, p, n)
		i := slices.IndexFunc(evs, func(e Event) bool { return e.Kind == c.kind })
		if i < 0 || evs[i].Hook != c.hook || ansi.Strip(evs[i].Text) != c.text {
			t.Errorf("%s: got\n%s\nwant %s %q (%s)", c.name, plain(evs), c.kind, c.text, c.hook)
		}
	}

	// A boss's failure the boot survives deals a new boss door, in the
	// story's words for it.
	p := fixture.CyclesProblem(fixture.CyclesBoss("heisenbug"))
	n := fixture.CyclesProblem(fixture.CyclesBoss("heisenbug"))
	a := *n.Current
	a.Outcome, a.Closed = game.TimedOut, a.Deadline
	n.History, n.Current, n.Phase, n.Cycles = append(n.History, a), nil, game.PhaseDoors, n.Cycles-20
	if evs := stripStep(t, p, n); len(evs) == 0 || evs[0].Hook != "boss.retry" ||
		ansi.Strip(evs[0].Text) != "⏱ timer ran out: burned 20 cycles, 4 left · a new boss door" {
		t.Errorf("boss timeout: got\n%s", plain(evs))
	}

	// kexec's free swap is once an act on the map.
	p, n = open(), open()
	p.Daemons, n.Daemons = append(p.Daemons, game.Kexec), append(n.Daemons, game.Kexec)
	a = *n.Current
	a.Outcome = game.Retreated
	n.History, n.Current, n.Phase, n.KexecUsed = append(n.History, a), nil, game.PhaseDoors, true
	if evs := stripStep(t, p, n); len(evs) == 0 || !strings.Contains(plain(evs), "kexec swapped it free · used for this act") {
		t.Errorf("kexec: got\n%s", plain(evs))
	}

	// The fountain pours on the way into the window between acts; fsck
	// is the King's men.
	for _, c := range []struct{ fountain, hook, text string }{
		{"drink", "fountain.drink", "fountain: +5 cycles, free · ♥ 17 → 22"},
		{"full", "fountain.full", "fountain: cycles at 30 or more: its +5 not taken"},
		{"dry", "fountain.dry", "fountain: dry at Advanced"},
	} {
		n := fixture.CyclesMerge(c.fountain)
		p := fixture.CyclesMerge(c.fountain)
		p.Phase, p.Cycles = game.PhaseDoors, n.Cycles-n.Fountain
		evs := stripStep(t, p, n)
		i := slices.IndexFunc(evs, func(e Event) bool { return e.Kind == EvFountain })
		if i < 0 || evs[i].Hook != c.hook || ansi.Strip(evs[i].Text) != c.text {
			t.Errorf("fountain %s: got\n%s", c.fountain, plain(evs))
		}
	}
	p, n = fixture.CyclesMerge("drink"), fixture.CyclesMerge("drink")
	n.Spent, n.Uncommitted, n.Cycles = 64, n.Uncommitted-64, n.Cycles+10
	if evs := stripStep(t, p, n); len(evs) == 0 || evs[0].Hook != "item.fsck" ||
		ansi.Strip(evs[0].Text) != "spend: fsck · +10 cycles now · 64 B · ♥ 22 → 32" {
		t.Errorf("fsck: got\n%s", plain(evs))
	}
}

// TestRetreatLock: the lock is 2:30 into a door (rules.RetreatLock): a
// retreat stands before it, and past it R and :Retreat get the strip's
// door.retreat.locked with the lock's time, kexec's swap included.
func TestRetreatLock(t *testing.T) {
	st := fixture.CyclesProblem(fixture.CyclesRoom())
	opened := st.Current.Opened
	if _, locked := RetreatLocked(st, opened.Add(149*time.Second)); locked {
		t.Error("locked at 2:29")
	}
	if _, ok := RetreatRefused(st, opened.Add(time.Minute)); ok {
		t.Error("refused a minute in")
	}
	st.Daemons = append(st.Daemons, game.Kexec)
	ev, ok := RetreatRefused(st, opened.Add(150*time.Second))
	if !ok || ev.Kind != EvRetreatLocked || ev.Hook != "door.retreat.locked" || !slices.Equal(ev.Fill, []string{"time", "02:30"}) ||
		ansi.Strip(ev.Text) != "↩ retreat: locked 02:30 in · solve it, or :q!" {
		t.Errorf("past the lock, kexec held: %+v", ev)
	}
	r := rules.Map()
	if r.RetreatLock.Duration != 150*time.Second {
		t.Fatalf("map lock %v: the fixture's times assume 2:30", r.RetreatLock.Duration)
	}
}

// TestSubmitPrice: what a failed submit will cost, said before it is
// made: the map's 2 cycles, Off-by-One's near miss, ECC's clock, the last
// cycles and the curse.
func TestSubmitPrice(t *testing.T) {
	room := fixture.CyclesProblem(fixture.CyclesRoom())
	if got := SubmitPrice(room); got != "a fail burns 3 cycles" { // 2, and 1 more at Silver (P1)
		t.Errorf("room: %q", got)
	}
	obo := fixture.CyclesProblem(fixture.CyclesBoss("off-by-one"))
	if got := SubmitPrice(obo); got != "a fail burns 3 cycles, 6 one test short" {
		t.Errorf("Off-by-One: %q", got)
	}
	ecc := fixture.CyclesProblem(fixture.CyclesRoom())
	ecc.Upgrades = append(ecc.Upgrades, game.ECC)
	if got := SubmitPrice(ecc); got != "a fail costs 05:00 of the clock (ECC)" {
		t.Errorf("ECC: %q", got)
	}
	last := fixture.CyclesProblem(fixture.CyclesRoom())
	last.Cycles = 2
	if got := SubmitPrice(last); got != "a fail ends the boot (2 cycles left)" {
		t.Errorf("last cycles: %q", got)
	}
	cursed := fixture.CyclesProblem(fixture.CyclesRoom())
	cursed.Current.Door.Cursed = true
	if got := SubmitPrice(cursed); got != "a fail ends the boot (cursed)" {
		t.Errorf("cursed: %q", got)
	}
}

// TestCyclesUpgradeCards: an Overclock card quotes its price and cap in
// the boot's numbers and how many are taken (D50); ECC is the clock
// conversion.
func TestCyclesUpgradeCards(t *testing.T) {
	st := fixture.CyclesUpgrade()
	opts := upgradeOptions(st.UpgradeOffers, st)
	want := []string{"+15% timer; 5 cycles a pick, 2 at most; 1 of 2 taken", "a failed submit costs 05:00, not cycles", "+1 rubber duck"}
	for i, o := range opts {
		if o.desc != want[i] {
			t.Errorf("%s: %q, want %q", o.name, o.desc, want[i])
		}
	}
	// With a real flavor the numbers are the same rules'.
	fl = realFlavor
	defer func() { fl = stubFlavor }()
	if _, d := upgradeText(game.Overclock, st); d != "+15% timer on later doors; 5 cycles a pick, 2 at most; 1 of 2 taken" {
		t.Errorf("real Overclock: %q", d)
	}
}

// TestCyclesStripGoldens: the strip over the next screen after a cost, as
// the player meets it: Boss II's doors after act II's room was lost to its
// timer; the window between acts after its fountain poured and fsck ran.
func TestCyclesStripGoldens(t *testing.T) {
	m := fixture.Meta()
	// A player this deep has met every tip.
	for _, id := range []string{"tip.doors", "tip.door", "tip.upgrade", "tip.bytes", "tip.fail", "tip.duck", "tip.regression"} {
		game.MarkSeen(&m, id)
	}
	lost := func() Env {
		p := fixture.CyclesProblem(fixture.CyclesRoom())
		n := fixture.CyclesBoss("heisenbug")
		a := *p.Current
		a.Outcome, a.Closed = game.TimedOut, a.Deadline
		n.History = append(n.History[:len(n.History)-1], a) // the ALU's room, lost
		n.Cycles = p.Cycles - 12
		var tr Tracker
		tr.Lookup = fixture.Lookup
		tr.Observe(p, &m, fixture.Now)
		tr.Observe(n, &m, fixture.Now)
		e := env(n)
		e.Events = tr.Events()
		return e
	}
	merge := func() Env {
		p := fixture.CyclesProblem(fixture.CyclesBoss("off-by-one"))
		n := fixture.CyclesMerge("drink")
		var tr Tracker
		tr.Lookup = fixture.Lookup
		p.Cycles = n.Cycles - n.Fountain
		tr.Observe(p, &m, fixture.Now)
		tr.Observe(n, &m, fixture.Now)
		f := fixture.CyclesMerge("drink")
		f.Spent, f.Uncommitted, f.Cycles = 64, f.Uncommitted-64, f.Cycles+10
		tr.Observe(f, &m, fixture.Now.Add(20*time.Second))
		e := env(f)
		e.Events = tr.Events()
		return e
	}
	for _, c := range []struct {
		name string
		make func() screens.Screen
	}{
		{"cycles_strip_lost", func() screens.Screen { return NewDoors(lost()) }},
		{"cycles_strip_merge", func() screens.Screen { return NewMerge(merge()) }},
	} {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			v := c.make().View(sz[0], sz[1])
			checkFits(t, name, v, sz[0], sz[1])
			golden(t, name, ansi.Strip(v)+"\n")
		}
	}
}
