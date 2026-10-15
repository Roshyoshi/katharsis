package boot

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot/fixture"
)

// engineBoot is a real boot on the simulator's pool, with a tracker that
// has taken its baseline, for tests that step it and read the strip.
type engineBoot struct {
	t    *testing.T
	st   *game.State
	meta game.Meta
	pool []catalog.Problem
	now  time.Time
	tr   Tracker
}

func newEngineBoot(t *testing.T, r rules.Rules) *engineBoot {
	t.Helper()
	b := &engineBoot{t: t, pool: sim.SyntheticPool(400, 3), now: fixture.Now}
	byID := map[catalog.ID]catalog.Problem{}
	for _, p := range b.pool {
		byID[p.ID] = p
	}
	b.tr.Lookup = func(id catalog.ID) (catalog.Problem, bool) { p, ok := byID[id]; return p, ok }
	st, _, err := game.NewBoot(r, &b.meta, b.pool, 0, 7, b.now)
	if err != nil {
		t.Fatal(err)
	}
	b.st = st
	b.observe()
	return b
}

// observe shows the tracker the state and marks what it saw once, as the
// app does.
func (b *engineBoot) observe() []Event {
	evs := b.tr.Observe(b.st, &b.meta, b.now)
	for _, ev := range evs {
		if ev.Seen != "" {
			game.MarkSeen(&b.meta, ev.Seen)
		}
	}
	return evs
}

func (b *engineBoot) step(a game.Action) []Event {
	b.t.Helper()
	if _, err := game.Step(b.st, &b.meta, b.pool, a, b.now); err != nil {
		b.t.Fatalf("%T: %v", a, err)
	}
	return b.observe()
}

func (b *engineBoot) toDoors() {
	for i := 0; b.st.Phase == game.PhaseRoute && i < 3; i++ {
		b.step(game.ChooseRoute{})
	}
}

func kinds(evs []Event) []string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Kind)
	}
	return out
}

func plain(evs []Event) string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Kind+": "+ansi.Strip(e.Text))
	}
	return strings.Join(out, "\n")
}

// A boot played through its first room says, in order, what each step
// cost and paid: the onboarding line at the doors, the door's line, a
// failed submit with the life it took, the clear, and the pick.
func TestTrackerTellsTheBoot(t *testing.T) {
	b := newEngineBoot(t, rules.Standard())
	b.toDoors()
	if got := plain(b.tr.Events()); !strings.Contains(got, "tip: doors:") {
		t.Fatalf("first doors without the doors tip:\n%s", got)
	}
	evs := b.step(game.ChooseDoor{})
	if !slices.Contains(kinds(evs), EvTip) {
		t.Errorf("first door opened without the door tip: %v", kinds(evs))
	}
	lives := b.st.Lives
	b.tr.Judged(6, 30)
	b.now = b.now.Add(2 * time.Minute)
	evs = b.step(game.SubmitResult{Verdict: "WA", Passed: 6, Total: 30})
	want := fmt.Sprintf("verdict: judge: WA 6/30 · -1 ♥, %d left", lives-1)
	if len(evs) == 0 || plain(evs[:1]) != want || evs[0].Hook != "verdict.WA" {
		t.Errorf("failed submit: got\n%s\nwant %q with the Queen's line", plain(evs), want)
	}
	b.tr.Judged(0, 0)
	evs = b.step(game.SubmitResult{Verdict: "CE"})
	if plain(evs) != "free: judge: CE · no charge" {
		t.Errorf("free verdict: got\n%s", plain(evs))
	}
	b.now = b.now.Add(time.Minute)
	evs = b.step(game.SubmitResult{Verdict: "AC", Passed: 30, Total: 30})
	if k := kinds(evs); len(k) < 1 || k[0] != EvAC || evs[0].Hook != "o3.bonus" {
		t.Errorf("an AC in 3 minutes: got %v, want ac with the -O3 line\n%s", k, plain(evs))
	}
	if !strings.Contains(plain(evs), `AC "`) || !strings.Contains(plain(evs), "in 03:00 of") || !strings.Contains(plain(evs), "· -O3 · +") {
		t.Errorf("the AC line names the door and its time:\n%s", plain(evs))
	}
	if !slices.Contains(kinds(evs), EvTip) {
		t.Errorf("first AC without its upgrade tip: %v", kinds(evs))
	}
	u := b.st.UpgradeOffers[0]
	evs = b.step(game.PickUpgrade{Upgrade: u})
	if len(evs) != 1 || evs[0].Kind != EvUpgrade || evs[0].Hook != "upgrade."+string(u) {
		t.Errorf("upgrade pick: got\n%s", plain(evs))
	}
	// A tip shows once a save: a second boot does not repeat it.
	b2 := &engineBoot{t: t, pool: b.pool, now: b.now, meta: b.meta, tr: Tracker{Lookup: b.tr.Lookup}}
	st, _, err := game.NewBoot(rules.Standard(), &b2.meta, b2.pool, 0, 9, b2.now)
	if err != nil {
		t.Fatal(err)
	}
	b2.st = st
	b2.observe()
	b2.toDoors()
	if slices.Contains(kinds(b2.tr.Events()), EvTip) {
		t.Errorf("second boot repeats a tip:\n%s", plain(b2.tr.Events()))
	}
}

// A retreat says what it cost; kexec says it was free and used up.
func TestTrackerRetreat(t *testing.T) {
	b := newEngineBoot(t, rules.Standard())
	b.toDoors()
	b.step(game.ChooseDoor{})
	evs := b.step(game.Retreat{})
	if len(evs) == 0 || evs[0].Kind != EvRetreat || !strings.Contains(plain(evs), "-1 ♥") {
		t.Errorf("retreat: got\n%s", plain(evs))
	}
	b.st.Daemons = append(b.st.Daemons, game.Kexec)
	b.observe() // a daemon line, not under test here
	b.step(game.ChooseDoor{})
	evs = b.step(game.Retreat{})
	if len(evs) == 0 || !strings.Contains(plain(evs), "kexec swapped it free") {
		t.Errorf("kexec retreat: got\n%s", plain(evs))
	}
}

// A Userland timeout that costs a life (D44) says so, and that the room
// was drawn again; the generic lives line does not repeat it.
func TestTrackerTimeoutCostsLife(t *testing.T) {
	b := newEngineBoot(t, rules.Standard())
	b.toDoors()
	b.step(game.ChooseDoor{})
	if !b.st.TimeoutCostsLife() {
		t.Fatal("the standard Userland door's timeout ends the boot")
	}
	lives := b.st.Lives
	b.now = b.st.Current.Deadline.Add(time.Second)
	evs := b.step(game.Tick{})
	want := fmt.Sprintf("timeout: ⏱ timer ran out: -1 ♥, %d left · new doors", lives-1)
	if b.st.Phase != game.PhaseDoors || plain(evs) != want {
		t.Errorf("in %s, got\n%s\nwant %q", b.st.Phase, plain(evs), want)
	}
}

// The strip opens a boot with its easing, one line each (RT8), and says
// what a decline took back, without the generic lives line.
func TestTrackerEasing(t *testing.T) {
	b := newEngineBoot(t, rules.Standard())
	var said []string
	for _, ev := range b.tr.Events() {
		if ev.Kind == EvEase {
			said = append(said, ansi.Strip(ev.Text))
		}
	}
	lines, _ := easing(b.st)
	if len(b.st.Ease) == 0 || !slices.Equal(said, stripAll(lines)) {
		t.Fatalf("boot #1 eased %+v; the strip said %q", b.st.Ease, said)
	}
	lives := b.st.Lives
	evs := b.step(game.DeclineEase{})
	want := "ease: easing declined · new doors"
	if lost := lives - b.st.Lives; lost > 0 {
		want = fmt.Sprintf("ease: easing declined · -%d ♥, %d left · new doors", lost, b.st.Lives)
	}
	if plain(evs) != want {
		t.Errorf("decline: got\n%s\nwant %q", plain(evs), want)
	}
}

// A boot after days away opens the strip on the comeback's line, its
// reason in days (RT8).
func TestTrackerComeback(t *testing.T) {
	pool := sim.SyntheticPool(400, 3)
	meta := game.Meta{Boots: 6, Rating: 1400, LastPlayed: fixture.Now.Add(-4*24*time.Hour - time.Hour)}
	st, _, err := game.NewBoot(rules.Standard(), &meta, pool, 0, 7, fixture.Now)
	if err != nil {
		t.Fatal(err)
	}
	var tr Tracker
	if got := plain(tr.Observe(st, &meta, fixture.Now)); !strings.Contains(got, "ease: Userland aim retuned (first boot in 4 days)") {
		t.Errorf("the strip after 4 days away:\n%s", got)
	}
}

// The timer warns once at 5:00 and once at 1:00, and not on a resume that
// is already past them.
func TestTrackerTimerWarns(t *testing.T) {
	b := newEngineBoot(t, rules.Standard())
	b.toDoors()
	b.step(game.ChooseDoor{})
	dl := b.st.Current.Deadline
	var got []string
	for _, left := range []time.Duration{6 * time.Minute, 5 * time.Minute, 4 * time.Minute, time.Minute, 30 * time.Second} {
		b.now = dl.Add(-left)
		for _, ev := range b.step(game.Tick{}) {
			got = append(got, ev.Hook)
		}
	}
	if want := []string{"timer.warn.5m", "timer.warn.1m"}; !slices.Equal(got, want) {
		t.Errorf("timer warnings: got %v, want %v", got, want)
	}
	var fresh Tracker
	fresh.Observe(b.st, &b.meta, b.now)
	if evs := fresh.Observe(b.st, &b.meta, b.now.Add(time.Second)); len(evs) != 0 {
		t.Errorf("a resumed door past its marks warned: %s", plain(evs))
	}
}

// A new boot empties the strip of the last one's events; it opens on its
// own easing, if it has any.
func TestTrackerNewBootClears(t *testing.T) {
	b := newEngineBoot(t, rules.Standard())
	b.toDoors()
	if len(b.tr.Events()) == 0 {
		t.Fatal("no events to clear")
	}
	st, _, err := game.NewBoot(rules.Standard(), &b.meta, b.pool, 0, 11, b.now)
	if err != nil {
		t.Fatal(err)
	}
	b.st = st
	b.observe()
	lines, _ := easing(st)
	var said []string
	for _, ev := range b.tr.Events() {
		said = append(said, ansi.Strip(ev.Text))
	}
	if !slices.Equal(said, stripAll(lines)) {
		t.Errorf("new boot's strip, want only its easing %q:\n%s", stripAll(lines), plain(b.tr.Events()))
	}
}

// stripEvents is a merge window's worth of events: the -O3 AC that opened
// it and the ducks it refilled, then this moment's commit.
func stripEvents(st *game.State) []Event {
	ac := st.Started.Add(166200 * time.Millisecond)
	now := st.Started.Add(170 * time.Second)
	return []Event{
		{At: ac, Kind: EvAC, Text: `judge: ✓ AC "Matrix Rotation" in 00:42 of 17:00 · -O3 · +41 B`, Hook: "o3.bonus"},
		{At: ac, Kind: EvRefill, Text: "merge window: ducks refilled 🦆0 → 🦆1", Hook: "duck.refill"},
		{At: now, Kind: EvCommit, Text: "commit: 41 B banked · bank 2.1 KiB", Hook: "merge.commit"},
	}
}

func stripEnv(st *game.State) Env {
	e := env(st)
	e.Events = stripEvents(st)
	return e
}

// The strip on the screens that have it, at both golden sizes: two rows
// at 24 lines (the commit and its flavor line), three at 40 (the AC
// above them).
func TestGoldenStrip(t *testing.T) {
	cases := []screenCase{
		{"strip_doors", func() screens.Screen { return NewDoors(stripEnv(fixture.Doors())) }},
		{"strip_doors_fork", func() screens.Screen { return NewDoors(stripEnv(fixture.DoorsFork())) }},
		{"strip_merge", func() screens.Screen { return NewMerge(stripEnv(fixture.Merge())) }},
	}
	for _, c := range cases {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				golden(t, name, ansi.Strip(c.make().View(sz[0], sz[1]))+"\n")
			})
		}
	}
}

// Every screen with a strip still fits, at every size.
func TestStripScreensFit(t *testing.T) {
	for _, mk := range []func() screens.Screen{
		func() screens.Screen { return NewDoors(stripEnv(fixture.Doors())) },
		func() screens.Screen { return NewDoors(stripEnv(fixture.DoorsFork())) },
		func() screens.Screen { return NewDoors(stripEnv(fixture.Boss())) },
		func() screens.Screen { return NewMerge(stripEnv(fixture.Merge())) },
		func() screens.Screen { return NewRoute(stripEnv(fixture.Route3())) },
		func() screens.Screen { return NewUpgradeScreen(stripEnv(fixture.Upgrade())) },
		func() screens.Screen { return NewMap(stripEnv(fixture.Doors())) },
	} {
		for _, sz := range fitSizes {
			checkFits(t, "strip", mk().View(sz[0], sz[1]), sz[0], sz[1])
		}
	}
}

// The newest moment's lines win over its flavor line, which wins over
// older moments.
func TestStripPriority(t *testing.T) {
	st := fixture.Doors()
	e := stripEnv(st)
	row := func(rows int) []string {
		var out []string
		for _, l := range e.strip(80, rows) {
			out = append(out, strings.TrimSpace(ansi.Strip(l)))
		}
		return out
	}
	if got := row(1); !strings.HasSuffix(got[0], "commit: 41 B banked · bank 2.1 KiB") {
		t.Errorf("one row: %q", got)
	}
	if got := row(2); got[1] != "Stub line for merge.commit." {
		t.Errorf("two rows: %q", got)
	}
	if got := row(3); !strings.Contains(got[0], "ducks refilled") {
		t.Errorf("three rows: %q", got)
	}
	e.Events = e.Events[:2] // the AC moment, newest
	if got := row(2); !strings.Contains(got[0], "AC") || !strings.Contains(got[1], "ducks refilled") {
		t.Errorf("a moment of two keeps both lines over the flavor: %q", got)
	}
}

// An OLE is a resource blowout like an MLE: the strip folds it onto MLE as
// flavor does (flavor.NormVerdict), so it gets the MLE verdict line rather
// than none, and its own word stays on the mechanic line.
func TestTrackerFoldsOLE(t *testing.T) {
	b := newEngineBoot(t, rules.Standard())
	b.toDoors()
	b.step(game.ChooseDoor{})
	b.now = b.now.Add(time.Minute)
	evs := b.step(game.SubmitResult{Verdict: "OLE", Passed: 3, Total: 30})
	if len(evs) == 0 || evs[0].Kind != EvVerdict || evs[0].Hook != "verdict.MLE" || !strings.Contains(plain(evs[:1]), "judge: OLE") {
		t.Fatalf("an OLE: got %+v\n%s", evs, plain(evs))
	}
	if realFlavor.Story(evs[0].Hook, evs[0].Seed) == "" {
		t.Errorf("flavor has no line for %s", evs[0].Hook)
	}
}
