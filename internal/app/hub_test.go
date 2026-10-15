package app

import (
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/boot/fixture"
	"katharsis/internal/screens/meta"
	"katharsis/internal/store"
)

var (
	ctrlC = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	enter = tea.KeyPressMsg{Code: tea.KeyEnter}
)

// resumeHub takes the hub a launch between doors lands on back to the
// boot.
func (h *harness) resumeHub() {
	h.t.Helper()
	if _, ok := h.m.top().(*bootui.Hub); !ok {
		h.t.Fatalf("top is %T, want the hub", h.m.top())
	}
	h.send(enter)
}

// quits reports whether cmd is tea.Quit.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// Between doors q quits with the boot saved and suspended, and says so on
// the way out; the next launch lands on the hub, where enter resumes the
// boot where it stopped (meta.md §(c)).
func TestSuspendQuitResume(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(submitMsg{}) // AC: an upgrade pick, between doors
		h.send(game.PickUpgrade{Upgrade: h.m.state.UpgradeOffers[0]})
		boot, room := h.m.state.Boot, h.m.state.Room
		if h.m.state.Phase != game.PhaseDoors || !h.m.state.Suspendable() {
			t.Fatalf("phase %s, want the doors", h.m.state.Phase)
		}
		_, cmd := h.m.Update(key("q"))
		if !quits(cmd) {
			t.Fatal("q between doors did not quit")
		}
		if s := h.m.Farewell(); !strings.Contains(s, "Suspended") {
			t.Errorf("farewell %q, want map.suspend", s)
		}

		h.restart()
		if h.m.state == nil || h.m.state.Boot != boot || h.m.state.Room != room {
			t.Fatalf("resumed %+v, want boot %d at room %d", h.m.state, boot, room)
		}
		hub, ok := h.m.top().(*bootui.Hub)
		if !ok {
			t.Fatalf("launch lands on %T, want the hub", h.m.top())
		}
		if v := ansi.Strip(hub.View(100, 30)); !strings.Contains(v, "suspended") {
			t.Errorf("the hub does not say suspended:\n%s", v)
		}
		h.resumeHub()
		if _, ok := h.m.top().(*bootui.Doors); !ok || len(h.m.stack) != 1 {
			t.Fatalf("resumed to %T (%d screens), want the doors", h.m.top(), len(h.m.stack))
		}
		if !strings.Contains(h.m.flash, "picks up where it stopped") { // map.resume
			t.Errorf("flash %q, want map.resume", h.m.flash)
		}
		if h.m.Farewell() == "" {
			t.Error("a boot between doors says nothing on the way out")
		}
	})
}

// m opens the hub from any boot screen between doors, and enter, esc or m
// go back; ctrl+c there quits like q. At GRUB there is no boot to suspend.
func TestMapKeyOpensTheHub(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		for _, back := range []tea.KeyPressMsg{enter, {Code: tea.KeyEscape}, key("m")} {
			h.send(key("m"))
			if _, ok := h.m.top().(*bootui.Hub); !ok {
				t.Fatalf("m: top %T, want the hub", h.m.top())
			}
			h.send(back)
			if _, ok := h.m.top().(*bootui.Doors); !ok {
				t.Fatalf("%s: top %T, want the doors", back, h.m.top())
			}
		}
		h.send(key("m"))
		_, cmd := h.m.Update(ctrlC)
		if !quits(cmd) {
			t.Error("ctrl+c on the hub did not quit")
		}
	})
}

// n on the hub asks first (map.abandon); y ends the suspended boot as a
// crash (Abandon): the store has it crashed and abandoned, its bytes and
// the old core dump are gone, and GRUB starts the next boot.
func TestNewBootOverSuspended(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.crashWithDump()
		dump := *h.m.meta.CoreDump
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(submitMsg{})
		boot, bytes := h.m.state.Boot, h.m.state.Uncommitted
		if bytes == 0 {
			t.Fatal("nothing at stake")
		}
		h.restart()
		h.send(key("n"))
		if h.m.state.Phase == game.PhaseCrashed {
			t.Fatal("n abandoned without asking")
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "stays behind, as a crash") || !strings.Contains(v, "destroyed") {
			t.Errorf("the question says nothing of the cost:\n%s", v)
		}
		h.send(key("y"))
		if h.m.state != nil {
			t.Fatalf("after y: boot %d in %s, want GRUB", h.m.state.Boot, h.m.state.Phase)
		}
		if _, ok := h.m.top().(*meta.Menu); !ok {
			t.Fatalf("top %T, want GRUB", h.m.top())
		}
		if h.m.meta.CoreDump != nil {
			t.Errorf("core dump %+v survived (was %+v)", h.m.meta.CoreDump, dump)
		}
		boots, err := h.st.Boots(1)
		if err != nil || len(boots) != 1 || boots[0].Boot != boot || boots[0].Outcome != store.BootCrashed || boots[0].Cause != game.Abandoned {
			t.Fatalf("boots %+v, %v: want #%d crashed, abandoned", boots, err, boot)
		}
		if active, _ := h.st.ActiveBoot(); active != nil {
			t.Errorf("boot %d still active", active.Boot)
		}
		// Another key than y keeps the boot.
		h.send(screens.StartBoot{Preset: "standard"})
		h.restart()
		h.send(key("n"))
		h.send(key("x"))
		if h.m.state == nil || h.m.state.Phase == game.PhaseCrashed {
			t.Error("x after n abandoned the boot")
		}
	})
}

// A trial started over a suspended boot asks on the hub first, then ends
// the boot as a crash and starts the trial.
func TestTrialOverSuspended(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		boot := h.m.state.Boot
		h.send(screens.StartTrial{Module: "two-pointers"})
		hub, ok := h.m.top().(*bootui.Hub)
		if !ok || h.m.state.Boot != boot {
			t.Fatalf("top %T, boot %d: want the hub asking over boot %d", h.m.top(), h.m.state.Boot, boot)
		}
		if v := ansi.Strip(hub.View(100, 30)); !strings.Contains(v, "abandon boot") {
			t.Errorf("the hub does not ask:\n%s", v)
		}
		h.send(key("y"))
		if boots, _ := h.st.Boots(1); len(boots) == 0 || boots[0].Boot != boot || boots[0].Cause != game.Abandoned {
			t.Errorf("boots %+v, want #%d abandoned", boots, boot)
		}
	})
}

// ctrl+c inside a door never quits (D47): it names the ways out, solve,
// retreat and :q!, and that the clock goes on; the next key clears it. The
// door stays open, and a launch after the terminal closed reopens it.
func TestCtrlCInDoorRefuses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		for range 2 {
			if _, cmd := h.m.Update(ctrlC); quits(cmd) {
				t.Fatal("ctrl+c in a door quit")
			}
		}
		q := h.m.doorQuit
		for _, want := range []string{"no quitting inside a door: solve", "R retreat −", ":q!", "closing the terminal does not stop the clock"} {
			if !strings.Contains(q.Rules, want) {
				t.Errorf("refusal %q lacks %q", q.Rules, want)
			}
		}
		if !strings.Contains(q.Story, "won't stop his watch") { // quit.warn
			t.Errorf("story half %q", q.Story)
		}
		if h.m.state.Phase != game.PhaseProblem || h.m.open == nil {
			t.Fatalf("the door closed: phase %s", h.m.state.Phase)
		}
		h.send(key("j"))
		if h.m.doorQuit != (quitRefusal{}) {
			t.Errorf("a key left the refusal up: %+v", h.m.doorQuit)
		}
		h.clock.t = h.clock.t.Add(time.Minute)
		h.restart() // the terminal closed
		if h.m.state.Phase != game.PhaseProblem || !strings.Contains(h.m.flash, "kept time") {
			t.Errorf("relaunch: phase %s, flash %q; want the door, its time told", h.m.state.Phase, h.m.flash)
		}
	})
}

// The refusal prices the ways out in the boot's cycles: free under kexec,
// none once the retreat locks, and a :q! that ends the boot says so.
func TestDoorQuitText(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		st := h.m.state
		if r := h.m.doorQuitText().Rules; !strings.Contains(r, ":q! ends the boot") {
			t.Errorf("before cycles a :q! ends the boot: %q", r)
		}
		st.Daemons = append(st.Daemons, game.Kexec)
		if r := h.m.doorQuitText().Rules; !strings.Contains(r, "R retreat free (kexec)") {
			t.Errorf("kexec: %q", r)
		}
		st.Daemons, st.Rules.RetreatLock.Duration = nil, time.Second
		h.clock.t = st.Current.Opened.Add(time.Minute)
		if r := h.m.doorQuitText().Rules; strings.Contains(r, "retreat") {
			t.Errorf("past the retreat lock: %q", r)
		}
	})
}

// A map boot owes its one-time scenes where it stands: act II and III at
// their first route, the skip once taken, and the first exit a capability
// locks, never a prestige lock. A fall into a node lands on its own line.
func TestMapScenes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		ids := func(st *game.State) []string {
			h.m.state = st
			var out []string
			for _, q := range h.m.mapScenes() {
				out = append(out, q.id)
			}
			return out
		}
		skipped := fixture.MapRouteFinal()
		skipped.Biomes[5].Node, skipped.Biomes[5].Rooms, skipped.Biomes[5].Name = "skip", 0, "skip"
		skipped.Biome, skipped.Phase = 6, game.PhaseDoors
		for _, c := range []struct {
			name string
			st   *game.State
			want []string
		}{
			{"act II, two exits locked", fixture.MapRoute(), []string{"story.first-act2", "story.first-locked"}},
			{"a prestige lock only", fixture.MapRoutePrestige(), []string{"story.first-act2"}},
			{"act III", fixture.MapRouteFinal(), []string{"story.first-act3", "story.first-locked"}},
			{"past the skip", skipped, []string{"story.first-act3", "story.first-skip"}},
			{"act I", fixture.MapRouteBoss(), nil},
			{"a merge window", fixture.MapMerge(), nil},
		} {
			if got := ids(c.st); !slices.Equal(got, c.want) {
				t.Errorf("%s: %v, want %v", c.name, got, c.want)
			}
		}
		game.MarkSeen(&h.m.meta, "story.first-act2")
		if got := ids(fixture.MapRoute()); !slices.Equal(got, []string{"story.first-locked"}) {
			t.Errorf("seen act II: %v", got)
		}

		sc := h.m.biomeFall(game.BiomeState{Name: "Kernel Space", Topic: game.Mixed, Node: "kernel"})
		sc.Seek(0, sc.PageLen(0))
		if v := ansi.Strip(sc.View(100, 30)); !strings.Contains(v, "Kernel croquet") || !strings.Contains(v, "K E R N E L") {
			t.Errorf("Kernel Space's fall:\n%s", v)
		}
	})
}
