package app

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/story"
)

// newStoryHarness is the game with its story scenes on, at GRUB.
func newStoryHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.storyRestart()
	return h
}

// storyRestart starts the game again on the same store with the story on,
// as cmd/katharsis runs it.
func (h *harness) storyRestart() {
	h.t.Helper()
	d := h.m.d
	d.Story = true
	m, err := New(d)
	if err != nil {
		h.t.Fatal(err)
	}
	h.m = m
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m.Init())
}

var esc = tea.KeyPressMsg{Code: tea.KeyEscape}

// scene is the story scene on top, nil when none is.
func (h *harness) scene() *story.Scene {
	s, _ := h.m.top().(*story.Scene)
	return s
}

// sceneName names the scene on top: its seen id, or for an every-time
// scene what it shows.
func (h *harness) sceneName() string {
	s := h.scene()
	if s == nil {
		return ""
	}
	if s.ID() != "" {
		return s.ID()
	}
	// An every-time scene is one page: look at its last frame.
	s.Seek(0, s.PageLen(0))
	v := ansi.Strip(h.m.View().Content)
	switch {
	case strings.Contains(v, "Run /sbin/init"):
		return "splash"
	case strings.Contains(v, "AC · "):
		return "accepted"
	case strings.Contains(v, "B O S S") || strings.Contains(v, " BOSS "):
		return "boss"
	}
	return "biome"
}

// watch presses keys through the scenes on top, one at a time, and names
// each. keys counts the presses it took.
func (h *harness) watch() (names []string, keys int) {
	h.t.Helper()
	var prev *story.Scene
	for i := 0; h.scene() != nil; i++ {
		if i > 40 {
			h.t.Fatalf("scenes still up after 40 keys: %v", names)
		}
		if s := h.scene(); s != prev {
			names, prev = append(names, h.sceneName()), s
		}
		h.send(key("x"))
		keys++
	}
	return names, keys
}

// noSceneOverDoor fails if a story scene sits over an open door.
func (h *harness) noSceneOverDoor() {
	h.t.Helper()
	if s := h.m.state; s != nil && s.Phase == game.PhaseProblem {
		for _, sc := range h.m.stack {
			if _, ok := sc.(*story.Scene); ok {
				h.t.Fatalf("a scene covers the open door %s", s.Current.Door.Problem)
			}
		}
	}
}

// playStory plays a boot as play does, watching every scene on the way
// and checking none covers a door. It returns the scenes in order and the
// boot's last phase.
func (h *harness) playStory() (names []string, end game.Phase) {
	h.t.Helper()
	for i := 0; i < 400; i++ {
		got, _ := h.watch()
		names = append(names, got...)
		h.noSceneOverDoor()
		s := h.m.state
		switch {
		case s.Phase == game.PhaseCleared || s.Phase == game.PhaseCrashed:
			return names, s.Phase
		case s.PendingUpgrade:
			h.send(game.PickUpgrade{Upgrade: s.UpgradeOffers[0]})
		case s.Phase == game.PhaseRoute:
			h.send(game.ChooseRoute{Index: 0})
		case s.Phase == game.PhaseDoors:
			h.send(game.ChooseDoor{Index: 0})
		case s.Phase == game.PhaseProblem:
			h.clock.t = h.clock.t.Add(5 * time.Minute)
			h.send(submitMsg{})
		case s.Phase == game.PhaseMerge:
			if s.Uncommitted > 0 {
				h.send(game.Commit{})
			} else {
				h.send(game.LeaveMerge{})
			}
		}
		h.noSceneOverDoor()
	}
	h.t.Fatal("boot did not end in 400 steps")
	return nil, ""
}

func seenIDs(m game.Meta) []string {
	var ids []string
	for _, s := range m.Seen {
		if flavor.BeatTitle(s.ID) == "" {
			continue // a one-time line that is no beat: the boot screens' tips
		}
		ids = append(ids, s.ID)
	}
	return ids
}

// TestStoryWalksABoot plays the first boot with the story on: each scene
// at its moment, each one-time scene once and saved, none over a door. The
// second boot gets only the every-time scenes.
func TestStoryWalksABoot(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		t.Run(map[bool]string{false: "full motion", true: "reduced motion"}[reduced], func(t *testing.T) {
			anim.SetMotion(anim.Motion{Reduced: reduced})
			defer anim.SetMotion(anim.Motion{})
			h := newStoryHarness(t)
			h.m.meta.Rated = 100 // calibrated, so the clear unlocks prestige 1
			h.send(screens.StartBoot{Preset: "standard"})
			got, end := h.playStory()
			want := []string{
				"splash", "story.prologue", "door.first.inscription", // the prologue's fall is Userland's drop
				"accepted",                      // room 1 of Userland
				"accepted", "story.first-merge", // room 2, then the first merge window
				"biome",            // the route's biome
				"accepted",         // its room 1
				"accepted",         // its room 2, into the merge window
				"story.first-boss", // the King's rule, then the boss card
			}
			if end != game.PhaseCleared || !slices.Equal(got, want) {
				t.Fatalf("boot ended %s with scenes\n%v\nwant\n%v", end, got, want)
			}
			seen := []string{"story.prologue", "door.first.inscription", "story.first-merge", "daemon.intro", "story.first-boss"}
			if ids := seenIDs(h.m.meta); !slices.Equal(ids, seen) {
				t.Errorf("seen %v, want %v", ids, seen)
			}
			if m, err := h.st.LoadMeta(); err != nil || !slices.Equal(seenIDs(m), seen) {
				t.Errorf("stored seen %v, %v; want %v", seenIDs(m), err, seen)
			}
			var beats []string
			for _, e := range h.events() {
				if e.Kind == storyBeat {
					beats = append(beats, payload(t, e)["id"].(string))
				}
			}
			if !slices.Equal(beats, seen) {
				t.Errorf("story_beat events %v, want %v", beats, seen)
			}

			// Boot 2: the one-time scenes are done.
			h.send(screens.StartBoot{Preset: "standard"})
			if got, _ := h.watch(); !slices.Equal(got, []string{"splash", "biome"}) {
				t.Errorf("boot 2 opens with %v, want the splash and Userland's drop", got)
			}
		})
	}
}

// TestStoryKeys: each key does one thing. A scene that leaves on its own
// leaves on one key; under reduced motion every page is drawn at once, so
// one key turns it. esc leaves the scene and the ones queued behind it.
func TestStoryKeys(t *testing.T) {
	anim.SetMotion(anim.Motion{Reduced: true})
	defer anim.SetMotion(anim.Motion{})
	h := newStoryHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	if h.sceneName() != "splash" {
		t.Fatalf("boot opens with %q", h.sceneName())
	}
	h.send(key("1")) // one key past the splash, not into the doors
	if h.sceneName() != "story.prologue" || h.m.state.Phase != game.PhaseDoors {
		t.Fatalf("after one key: %q, phase %s", h.sceneName(), h.m.state.Phase)
	}
	// Drawn at once: the whole first page is up before any key.
	if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "Nobody checks your privileges") {
		t.Errorf("reduced motion types the prologue:\n%s", v)
	}
	h.send(key("x")) // to the fall, drawn landed
	if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "Pick a door") {
		t.Errorf("reduced motion plays the fall:\n%s", v)
	}
	h.send(esc)
	if h.scene() != nil {
		t.Fatalf("esc left %q up", h.sceneName())
	}
	if ids := seenIDs(h.m.meta); !slices.Equal(ids, []string{"story.prologue"}) {
		t.Errorf("seen %v: the prologue was up, the first door never was", ids)
	}
}

// TestStoryResume: a quit mid-scene loses nothing. The one-time scenes of
// where the boot stands come back; a door resumed open gets none.
func TestStoryResume(t *testing.T) {
	h := newStoryHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	h.send(key("x")) // past the splash: the prologue is up
	if h.sceneName() != "story.prologue" {
		t.Fatalf("prologue not up: %q", h.sceneName())
	}
	h.storyRestart() // quit mid-prologue
	if got, _ := h.watch(); !slices.Equal(got, []string{"story.prologue", "door.first.inscription"}) {
		t.Errorf("resume shows %v, want the prologue and the first door", got)
	}
	h.send(game.ChooseDoor{})
	if h.m.state.Phase != game.PhaseProblem {
		t.Fatalf("phase %s", h.m.state.Phase)
	}
	h.storyRestart()
	h.noSceneOverDoor()
	if h.scene() != nil {
		t.Errorf("a resumed door is under %q", h.sceneName())
	}
}

// TestStoryFirstCrash: the Cat plays by itself once the panic has printed,
// on the first crash that left bytes, and not if the player moved on.
func TestStoryFirstCrash(t *testing.T) {
	h := newStoryHarness(t)
	game.MarkSeen(&h.m.meta, "story.prologue")
	game.MarkSeen(&h.m.meta, "door.first.inscription")

	// A crash with nothing uncommitted earns no Cat.
	h.send(screens.StartBoot{Preset: "standard"})
	h.watch()
	h.verdict = "WA"
	if _, end := h.playStory(); end != game.PhaseCrashed {
		t.Fatalf("boot ended %s", end)
	}
	if h.m.storyCrash() != nil {
		t.Error("the Cat waits on a crash that left no bytes")
	}

	// One that left bytes: the Cat, after the panic.
	h.send(screens.StartBoot{Preset: "standard"})
	h.watch()
	h.verdict, h.acFirst = "WA", 1
	if _, end := h.playStory(); end != game.PhaseCrashed || h.m.state.Crash.Lost == 0 {
		t.Fatalf("boot ended %s, lost %d", end, h.m.state.Crash.Lost)
	}
	if h.scene() != nil {
		t.Fatal("the Cat covers the panic as it prints")
	}
	if h.m.storyCrash() == nil {
		t.Fatal("no Cat scheduled")
	}
	// The player opened the upsolve first: the Cat stays away.
	h.send(screens.Upsolve{Problem: h.m.state.Crash.Door.Problem})
	h.send(crashSceneMsg{boot: h.m.state.Boot})
	if h.scene() != nil {
		t.Fatal("the Cat covers the upsolve")
	}
	h.send(key("Q"))                                // leave the upsolve
	h.send(crashSceneMsg{boot: h.m.state.Boot - 1}) // a stale one, from boot 1
	if h.scene() != nil {
		t.Fatal("a stale tick showed the Cat")
	}
	h.send(crashSceneMsg{boot: h.m.state.Boot})
	if got, _ := h.watch(); !slices.Equal(got, []string{"story.first-crash"}) {
		t.Fatalf("crash shows %v, want the Cat", got)
	}
	if id, ok := flavor.NextBeat(h.m.state, &h.m.meta, flavor.DefaultBeatGap); ok {
		t.Errorf("the boot end has %q still to show after the Cat", id)
	}
	if len(h.m.stack) != 1 || h.m.state.Phase != game.PhaseCrashed {
		t.Errorf("after the Cat: %d screens, phase %s", len(h.m.stack), h.m.state.Phase)
	}
}

// TestStoryOff: the headless game (Deps.Story unset) and a learn-mode
// trial play no story scenes; a trial still gets ACCEPTED.
func TestStoryOff(t *testing.T) {
	h := newHarness(t)
	h.send(screens.StartBoot{Preset: "standard"})
	if h.scene() != nil {
		t.Fatalf("the story is off, but %q is up", h.sceneName())
	}
	if got, _ := h.playStory(); got != nil {
		t.Errorf("the story is off, but the boot showed %v", got)
	}

	h = newStoryHarness(t)
	loadGuide(h)
	h.send(screens.StartTrial{Module: "prefix-sums"})
	if h.m.state == nil || h.m.state.Trial == nil {
		t.Fatalf("no trial running: %+v", h.m.state)
	}
	got, end := h.playStory()
	if end != game.PhaseCleared || slices.ContainsFunc(got, func(s string) bool { return s != "accepted" }) || len(got) == 0 {
		t.Errorf("the trial ended %s with scenes %v, want only ACCEPTED", end, got)
	}
	if len(h.m.meta.Seen) != 0 {
		t.Errorf("a trial marked %v seen", seenIDs(h.m.meta))
	}
}

// TestStoryRead: a screen asks for a beat's page (story.Read), as a boot
// end's teaser or the reread log will. It shows over that screen, is seen
// once read, and never shows over an open door or for an id that is no
// beat. The headless game reads too: the player asked.
func TestStoryRead(t *testing.T) {
	h := newHarness(t)
	h.send(story.Read{ID: "no.such.beat"})
	if h.scene() != nil {
		t.Fatal("an unknown beat has a page")
	}
	h.send(story.Read{ID: "reveal.prestige.1"})
	if got, _ := h.watch(); !slices.Equal(got, []string{"reveal.prestige.1"}) {
		t.Fatalf("read shows %v", got)
	}
	if _, ok := h.m.top().(*meta.Menu); !ok || !slices.Equal(seenIDs(h.m.meta), []string{"reveal.prestige.1"}) {
		t.Errorf("after reading: on %T, seen %v", h.m.top(), seenIDs(h.m.meta))
	}
	h.send(screens.StartBoot{Preset: "standard"})
	h.send(game.ChooseDoor{})
	h.send(story.Read{ID: "reveal.prestige.2"})
	h.noSceneOverDoor()
}
