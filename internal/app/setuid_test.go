package app

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/meta"
	"katharsis/internal/setuid"
	"katharsis/internal/store"
)

// eventsOf is the payloads of the store's events of kind.
func (h *harness) eventsOf(kind string) []map[string]any {
	h.t.Helper()
	var out []map[string]any
	for _, e := range h.events() {
		if e.Kind == kind {
			out = append(out, payload(h.t, e))
		}
	}
	return out
}

// GRUB's find / -perm -4000 opens the setuid screen and records
// setuid_open; ? on a hidden secret records setuid_hint; q goes back.
func TestSetuidScreenOpens(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.OpenSetuid{})
		if _, ok := h.m.top().(*meta.Setuid); !ok {
			t.Fatalf("top is %T", h.m.top())
		}
		opens := h.eventsOf(store.EventSetuidOpen)
		if len(opens) != 1 || opens[0]["spoilers"] != "none" || opens[0]["set"] != 0.0 || opens[0]["of"] != float64(len(setuid.Table)) {
			t.Errorf("setuid_open: %v", opens)
		}
		h.send(key("G"))
		h.send(key("?"))
		hints := h.eventsOf(store.EventSetuidHint)
		if len(hints) != 1 || hints[0]["id"] != ".which-dreamed-it" {
			t.Errorf("setuid_hint: %v", hints)
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "Two endings. One dreamer.") {
			t.Errorf("the hint is not on screen:\n%s", v)
		}
		h.send(key("q"))
		if _, ok := h.m.top().(*meta.Menu); !ok {
			t.Fatalf("q left %T on top", h.m.top())
		}
	})
}

// setuid_spoilers sets what the screen opens with: hints shows every
// secret's hint at once, and setuid_open says so.
func TestSetuidSpoilerSetting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.d.Spoilers = meta.SpoilersHints
		h.send(screens.OpenSetuid{})
		h.send(key("G"))
		v := ansi.Strip(h.m.View().Content)
		if !strings.Contains(v, "spoilers: hints") || !strings.Contains(v, "# Count them. Count again.") {
			t.Errorf("hints spoilers:\n%s", v)
		}
		if opens := h.eventsOf(store.EventSetuidOpen); len(opens) != 1 || opens[0]["spoilers"] != "hints" {
			t.Errorf("setuid_open: %v", opens)
		}
		h.send(key("?")) // every hint shows already: nothing to record
		if hints := h.eventsOf(store.EventSetuidHint); len(hints) != 0 {
			t.Errorf("setuid_hint under hints: %v", hints)
		}
	})
}

// n on the screen turns the session's notices off, then on.
func TestSetuidNoticesToggle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.setuid.notices = true
		h.send(screens.OpenSetuid{})
		h.send(key("n"))
		if h.m.setuid.notices {
			t.Error("n left the notices on")
		}
		h.send(key("n"))
		if !h.m.setuid.notices {
			t.Error("n twice left the notices off")
		}
	})
}

// fakeBits is a SetuidCheck that sets its ids the first time a check runs
// where pick allows (nil: anywhere), and logs the phase of every check, ""
// outside a boot. The conditions are setuid's; the app's part is when it
// asks and where it says what was set.
type fakeBits struct {
	ids    []string
	pick   func(s *game.State) bool
	phases []game.Phase
}

func (f *fakeBits) check(m *game.Meta, s *game.State, now time.Time) []game.SetuidBit {
	var ph game.Phase
	if s != nil {
		ph = s.Phase
	}
	f.phases = append(f.phases, ph)
	if len(f.ids) == 0 || f.pick != nil && !f.pick(s) {
		return nil
	}
	var set []game.SetuidBit
	for _, id := range f.ids {
		b := game.SetuidBit{ID: id, At: now}
		if s != nil {
			b.Boot = s.Boot
		}
		m.Setuid = append(m.Setuid, b)
		set = append(set, b)
	}
	f.ids = nil
	return set
}

// fakeSetuid installs f on h's game with the notices on.
func (h *harness) fakeSetuid(f *fakeBits) {
	h.m.d.SetuidCheck = f.check
	h.m.setuid.notices = true
}

// stripSays reports whether the strip has an event of kind saying text.
func (h *harness) stripSays(kind, text string) bool {
	return slices.ContainsFunc(h.m.dmesg.Events(), func(e bootui.Event) bool {
		return e.Kind == kind && strings.Contains(ansi.Strip(e.Text), text)
	})
}

// Every :w inside a boot's door is a game.SamplesRun: the door counts it
// and the checkpoint keeps it. A sample pass ending after the door closed,
// or in an upsolve, counts nothing and refuses nothing.
func TestSamplesRunOnW(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		pass := hud.SamplesMsg{Compiled: true, Passed: 1, Total: 2}
		h.send(pass)
		h.send(pass)
		if n := h.m.state.Current.Samples; n != 2 {
			t.Fatalf("Samples %d after two :w, want 2", n)
		}
		if st, err := h.st.ActiveBoot(); err != nil || st.Current == nil || st.Current.Samples != 2 {
			t.Fatalf("saved %+v, %v", st, err)
		}
		h.send(submitMsg{}) // AC: the door closes
		h.send(pass)
		if a := h.m.state.History[len(h.m.state.History)-1]; a.Samples != 2 || h.m.flash != "" {
			t.Errorf("after the door: Samples %d, flash %q", a.Samples, h.m.flash)
		}
		h.crashAndGrub()
		h.send(screens.Upsolve{Problem: h.m.d.Catalog.All()[0].ID})
		h.send(pass)
		if h.m.flash != "" {
			t.Errorf("an upsolve's :w: flash %q", h.m.flash)
		}
	})
}

// crashAndGrub ends the boot as a :q! crash and leaves for GRUB.
func (h *harness) crashAndGrub() {
	h.t.Helper()
	h.verdict = "WA"
	if got := h.play(); got != game.PhaseCrashed {
		h.t.Fatalf("boot ended %q", got)
	}
	h.verdict = "AC"
	h.send(screens.OpenGrub{})
}

// No check runs while a door is open (GR6): a bit that holds then is set
// and said at the next safe point, after the door's own lines, and
// recorded (setuid_set) and saved with the step that set it.
func TestSetuidNotInADoor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		f := &fakeBits{ids: []string{"hello-world"}, pick: func(s *game.State) bool { return s != nil && s.Phase != game.PhaseRoute }}
		h.fakeSetuid(f)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(hud.SamplesMsg{Compiled: true, Passed: 1, Total: 2})
		h.send(game.Quack{})
		if slices.Contains(f.phases, game.PhaseProblem) {
			t.Fatalf("a check ran inside the door: %v", f.phases)
		}
		if h.stripSays(bootui.EvSetuid, "chmod") || strings.Contains(ansi.Strip(h.m.View().Content), "chmod") {
			t.Fatal("a bit said inside the door")
		}
		h.send(submitMsg{})
		evs := h.m.dmesg.Events()
		last := evs[len(evs)-1]
		ac := slices.IndexFunc(evs, func(e bootui.Event) bool { return e.Kind == bootui.EvAC })
		if last.Kind != bootui.EvSetuid || ansi.Strip(last.Text) != "chmod u+s /bin/hello-world" || ac < 0 || !evs[ac].At.Equal(last.At) {
			t.Errorf("strip after the AC: %+v", evs)
		}
		if !strings.Contains(ansi.Strip(h.m.View().Content), "chmod u+s /bin/hello-world") {
			t.Errorf("not on screen:\n%s", ansi.Strip(h.m.View().Content))
		}
		set := h.eventsOf(store.EventSetuidSet)
		if len(set) != 1 || set[0]["id"] != "hello-world" || set[0]["kind"] != "progression" ||
			set[0]["backfill"] != false || set[0]["phase"] != string(h.m.state.Phase) {
			t.Errorf("setuid_set: %v", set)
		}
		if m, err := h.st.LoadMeta(); err != nil || !setuid.IsSet(&m, "hello-world") {
			t.Errorf("the bit is not saved: %v", err)
		}
	})
}

// A bit a crash sets is said nowhere on the crash screen (GR3); the next
// GRUB's status line says "setuid +1" until the find screen reads it.
func TestSetuidCrashSaysGrub(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.fakeSetuid(&fakeBits{ids: []string{"exit-0"}, pick: func(s *game.State) bool { return s != nil && s.Phase == game.PhaseCrashed }})
		h.send(screens.StartBoot{Preset: "standard"})
		h.verdict = "WA"
		if got := h.play(); got != game.PhaseCrashed || !setuid.IsSet(&h.m.meta, "exit-0") {
			t.Fatalf("boot ended %q, bits %v", got, h.m.meta.Setuid)
		}
		for _, sz := range [][2]int{{80, 24}, {140, 40}} {
			h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			if v := ansi.Strip(h.m.View().Content); strings.Contains(v, "setuid") || strings.Contains(v, "chmod") {
				t.Errorf("the crash screen says the bit at %v:\n%s", sz, v)
			}
		}
		h.send(screens.OpenGrub{})
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "setuid +1") {
			t.Errorf("GRUB lacks the news:\n%s", v)
		}
		h.send(screens.OpenSetuid{})
		h.send(screens.Back{})
		if v := ansi.Strip(h.m.View().Content); strings.Contains(v, "setuid +1") {
			t.Errorf("GRUB still says it after the find screen:\n%s", v)
		}
	})
}

// A clear's bits are one line of the cleared screen's kept block; with
// setuid_notices off, none, and none on the strip either.
func TestSetuidClearedKept(t *testing.T) {
	for _, on := range []bool{true, false} {
		synctest.Test(t, func(t *testing.T) {
			h := newHarness(t)
			h.fakeSetuid(&fakeBits{ids: []string{"snicker-snack", "finds-none"}, pick: func(s *game.State) bool { return s != nil && s.Phase == game.PhaseCleared }})
			h.m.setuid.notices = on
			h.send(screens.StartBoot{Preset: "standard"})
			if got := h.play(); got != game.PhaseCleared {
				t.Fatalf("boot ended %q", got)
			}
			for _, sz := range [][2]int{{80, 24}, {140, 40}} {
				h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
				v := ansi.Strip(h.m.View().Content)
				if says := strings.Contains(v, "setuid +2 · snicker-snack, finds-none"); says != on {
					t.Errorf("notices %v at %v: the kept line shows %v:\n%s", on, sz, says, v)
				}
			}
		})
	}
}

// Outside a boot an upsolve's bits ride on its flash, and an install's on
// apt's, recorded with no phase.
func TestSetuidOutsideABoot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.fakeSetuid(&fakeBits{ids: []string{"grin-returns"}, pick: func(s *game.State) bool { return s == nil }})
		h.send(screens.Upsolve{Problem: h.m.d.Catalog.All()[0].ID})
		h.send(submitMsg{})
		if !strings.Contains(h.m.flash, "chmod u+s /bin/grin-returns") {
			t.Errorf("upsolve flash %q", h.m.flash)
		}
		h.fakeSetuid(&fakeBits{ids: []string{"apt-full"}, pick: func(s *game.State) bool { return s == nil }})
		h.m.meta.Bank = 1 << 20
		h.send(screens.Install{Package: h.m.packages()[0].Name})
		if !strings.Contains(h.m.flash, "installed ") || !strings.Contains(h.m.flash, "chmod u+s /usr/bin/apt-full") {
			t.Errorf("install flash %q", h.m.flash)
		}
		set := h.eventsOf(store.EventSetuidSet)
		if len(set) != 2 || set[0]["phase"] != "" || set[1]["id"] != "apt-full" {
			t.Errorf("setuid_set: %v", set)
		}
	})
}

// The backfill runs once, at the launch that finds a save not backfilled,
// before GRUB, which counts its bits; it is saved, so the next launch
// runs none. One that fails changes nothing and runs again next launch.
func TestSetuidBackfillOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		// The store's own backfill marked the fresh save at the first launch.
		if m, err := h.st.LoadMeta(); err != nil || !m.Record.Backfilled {
			t.Fatalf("first launch: backfilled %v, %v", m.Record.Backfilled, err)
		}
		m := h.m.meta
		m.Record.Backfilled = false
		if err := h.st.SaveMeta(m); err != nil {
			t.Fatal(err)
		}
		calls, fail := 0, true
		d := h.m.d
		d.SetuidNotices = true
		d.SetuidBackfill = func(m *game.Meta, now time.Time) ([]game.SetuidBit, error) {
			calls++
			if fail {
				return nil, errors.New("disk on fire")
			}
			b := game.SetuidBit{ID: "exit-0", At: now, Backfill: true}
			m.Setuid, m.Record.Backfilled = append(m.Setuid, b), true
			return []game.SetuidBit{b}, nil
		}
		h.start(d)
		if m, _ := h.st.LoadMeta(); calls != 1 || m.Record.Backfilled {
			t.Fatalf("a failed backfill: %d calls, backfilled %v", calls, m.Record.Backfilled)
		}
		fail = false
		h.start(d)
		m, _ = h.st.LoadMeta()
		if calls != 2 || !m.Record.Backfilled || !setuid.IsSet(&m, "exit-0") {
			t.Fatalf("backfill: %d calls, backfilled %v, bits %v", calls, m.Record.Backfilled, m.Setuid)
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "setuid +1") {
			t.Errorf("GRUB lacks the backfill's news:\n%s", v)
		}
		if set := h.eventsOf(store.EventSetuidSet); len(set) != 1 || set[0]["backfill"] != true {
			t.Errorf("setuid_set: %v", set)
		}
		h.start(d)
		if calls != 2 {
			t.Errorf("a backfilled save ran it again: %d calls", calls)
		}
	})
}

// heisenbugClear clears the boot's next door as Heisenbug's twisted door
// (the engine deals it so on a map), after w :w runs on it, with the real
// conditions (setuid.Check).
func heisenbugClear(t *testing.T, w int) *harness {
	t.Helper()
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.enterDoor()
	d := &h.m.state.Current.Door
	d.Boss, d.Twist = "heisenbug", "heisenbug"
	for range w {
		h.send(hud.SamplesMsg{Compiled: true, Passed: 2, Total: 2})
	}
	h.send(submitMsg{})
	if !setuid.IsSet(&h.m.meta, "shelf-stays-full") {
		t.Fatalf("Heisenbug's clear set no shelf-stays-full: %v", h.m.meta.Setuid)
	}
	return h
}

// SamplesRun and Check are a pair: a :w on Heisenbug's door keeps its
// skill bit (S10, stopped-looking) unset, and only a door with none sets
// it. Were the app to skip SamplesRun, every Heisenbug clear would set it.
func TestSetuidSamplesPair(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if h := heisenbugClear(t, 1); setuid.IsSet(&h.m.meta, "stopped-looking") {
			t.Error("a :w on Heisenbug's door set stopped-looking")
		}
	})
	synctest.Test(t, func(t *testing.T) {
		if h := heisenbugClear(t, 0); !setuid.IsSet(&h.m.meta, "stopped-looking") {
			t.Errorf("no :w, and stopped-looking is not set: %v", h.m.meta.Setuid)
		}
	})
}

// trialsOf is the modules of the world with division id that hold a
// trial on h's RTFM map, once h's catalog judges every problem the
// guide serves (the fake one has none of them).
func trialsOf(h *harness, id string) []string {
	h.t.Helper()
	cat := h.m.d.Catalog.(*fakeCatalog)
	for _, mod := range h.m.cur.Modules() {
		for _, list := range mod.Problems {
			for _, p := range list.Problems {
				if _, ok := cat.byID[p.ID]; p.ID != "" && !ok {
					cat.byID[p.ID] = catalog.Problem{ID: p.ID, Rating: 1400, Judgeable: true}
				}
			}
		}
	}
	var out []string
	for _, w := range h.m.rtfmMap().Worlds {
		if w.ID != id {
			continue
		}
		for _, l := range w.Levels() {
			if l.Kind != curriculum.PageLevel {
				out = append(out, l.Module)
			}
		}
	}
	return out
}

// Passing a world's last trial sets best-of-educations at the trial's
// end, live, with RTFM's map loaded.
func TestSetuidWorldOnTrialPass(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		silver := trialsOf(h, "silver")
		if len(silver) < 2 || !slices.Contains(silver, "prefix-sums") {
			t.Fatalf("the fixture's Silver trials %v: want prefix-sums and another", silver)
		}
		h.m.meta.TrialsPassed = slices.DeleteFunc(slices.Clone(silver), func(id string) bool { return id == "prefix-sums" })
		readyTrial(h, "prefix-sums")
		h.send(screens.StartTrial{Module: "prefix-sums"})
		if setuid.IsSet(&h.m.meta, "best-of-educations") {
			t.Fatal("set before the world's last trial passed")
		}
		if got := h.play(); got != game.PhaseCleared {
			t.Fatalf("trial ended %q, want cleared", got)
		}
		b, ok := setuid.Bit(&h.m.meta, "best-of-educations")
		if !ok || b.Backfill {
			t.Fatalf("best-of-educations %+v (set %v), want it live", b, ok)
		}
		if m, _ := h.st.LoadMeta(); !setuid.IsSet(&m, "best-of-educations") {
			t.Error("the bit was not saved")
		}
	})
}

// A save backfilled before the RTFM bits gets them on launch, marked
// "from git log": the conclusion's at once, the world's once RTFM loads,
// since it reads the map. GRUB counts them.
func TestSetuidRTFMBackfill(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		silver := trialsOf(h, "silver")
		cur := h.m.cur
		m := h.m.meta
		m.TrialsPassed = append(slices.Clone(silver), "bronze-conclusion")
		if err := h.st.SaveMeta(m); err != nil {
			t.Fatal(err)
		}
		d := h.m.d
		d.SetuidNotices = true
		h.start(d)
		saved, _ := h.st.LoadMeta()
		if b, ok := setuid.Bit(&saved, "lobster-quadrille"); !ok || !b.Backfill {
			t.Fatalf("lobster-quadrille %+v (set %v) at launch, want it from git log", b, ok)
		}
		if setuid.IsSet(&saved, "best-of-educations") {
			t.Fatal("best-of-educations set before RTFM loaded")
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "setuid +1") {
			t.Errorf("GRUB lacks the backfill's news:\n%s", v)
		}
		h.send(rtfmLoadedMsg{cur})
		saved, _ = h.st.LoadMeta()
		if b, ok := setuid.Bit(&saved, "best-of-educations"); !ok || !b.Backfill {
			t.Fatalf("best-of-educations %+v (set %v) once RTFM loaded, want it from git log", b, ok)
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "setuid +2") {
			t.Errorf("GRUB lacks the second backfill's news:\n%s", v)
		}
		h.start(d)
		if set := h.eventsOf(store.EventSetuidSet); len(set) != 2 {
			t.Errorf("setuid_set %v: want each RTFM bit once", set)
		}
	})
}
