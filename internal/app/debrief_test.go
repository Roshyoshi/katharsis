package app

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/meta"
	"katharsis/internal/store"
)

// crashWithDump plays a boot that banks bytes on its first door and
// crashes on the next with them uncommitted: a core dump to upsolve.
func (h *harness) crashWithDump() {
	h.t.Helper()
	h.send(screens.StartBoot{Preset: "standard"})
	h.verdict, h.acFirst = "WA", 1
	if got := h.play(); got != game.PhaseCrashed || h.m.meta.CoreDump == nil {
		h.t.Fatalf("boot ended %q with dump %+v, want a crash that dumped", got, h.m.meta.CoreDump)
	}
	h.verdict = "AC"
}

// g leaves a boot that ended for GRUB (UX J12); the boot is in git log.
func TestEndToGrub(t *testing.T) {
	h := newHarness(t)
	h.crashWithDump()
	h.send(screens.OpenGrub{})
	if h.m.state != nil {
		t.Fatalf("state %+v after g, want none", h.m.state.Phase)
	}
	if _, ok := h.m.top().(*meta.Menu); !ok {
		t.Fatalf("top is %T, want GRUB", h.m.top())
	}
	if boots, err := h.st.Boots(1); err != nil || len(boots) != 1 || boots[0].Outcome != store.BootCrashed {
		t.Errorf("boots %+v, %v", boots, err)
	}
	if got := h.answers(eventBootEndAction, "action"); len(got) != 1 || got[0] != "grub" {
		t.Errorf("boot_end_action %v, want [grub]", got)
	}
}

// The beat waiting at the end of a boot opens with b and, once read, is
// seen: saved, and no longer teased. The Cat is the app's to play, so a
// boot end that owes it teases nothing.
func TestEndReadsTheBeat(t *testing.T) {
	h := newHarness(t)
	game.MarkSeen(&h.m.meta, "story.first-crash")
	h.m.meta.Boots, h.m.meta.Rated = 20, 30 // a dry spell: a milestone is due
	h.crashWithDump()
	h.send(key("x")) // ends the panic's reveal
	if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "read on") {
		t.Fatalf("no teaser:\n%s", v)
	}
	h.send(key("b"))
	if got, _ := h.watch(); !slices.Equal(got, []string{"milestone.doors.10"}) {
		t.Fatalf("b showed %v", got)
	}
	if _, ok := h.m.top().(*bootui.Crash); !ok {
		t.Fatalf("top is %T after the beat, want the crash screen back", h.m.top())
	}
	saved, err := h.st.LoadMeta()
	if err != nil || !game.HasSeen(&saved, "milestone.doors.10") {
		t.Fatalf("seen not saved: %+v, %v", saved.Seen, err)
	}
	h.send(tickMsg(h.clock.t)) // the next tick hands the screen the new Meta
	if v := ansi.Strip(h.m.View().Content); strings.Contains(v, "read on") {
		t.Errorf("a read beat still teased:\n%s", v)
	}
	if got := h.answers(eventBootEndAction, "action"); !slices.Contains(got, "story") {
		t.Errorf("boot_end_action %v, want a story read", got)
	}
}

// The end of a boot offers the presets the game has, with the player's own
// ETAs, apt's goal at their pace, and past the budget one line (RT3, RT7,
// RT12).
func TestEndInfo(t *testing.T) {
	h := newHarness(t)
	short := rules.Standard()
	short.Name = "short"
	h.m.d.Presets["short"] = short
	h.m.d.Budget = time.Minute
	h.crashWithDump()
	e := h.m.endInfo()
	if len(e.Reboots) != 1 || e.Reboots[0] != (bootui.Reboot{Key: "s", Preset: "short"}) {
		t.Errorf("reboots %+v, want short with no ETA yet", e.Reboots)
	}
	if e.ETA <= 0 {
		t.Errorf("ETA %v, want this boot's uptime as the median", e.ETA)
	}
	if e.Goal == nil || e.Goal.Name == "" || e.Goal.Per != h.m.state.Committed {
		t.Errorf("goal %+v, committed %d", e.Goal, h.m.state.Committed)
	}
	if e.Budget != time.Minute || e.Today < time.Minute {
		t.Errorf("budget %v today %v", e.Budget, e.Today)
	}
	if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "nothing waiting here expires") || !strings.Contains(v, "[s] short") {
		t.Errorf("crash screen lacks the nudge or the short key:\n%s", v)
	}
	n := 0
	for _, ev := range h.events() {
		if ev.Kind == eventNudgeShown {
			n++
		}
	}
	if n != 1 {
		t.Errorf("nudge_shown %d times, want once a boot", n)
	}
	// s reboots on short, asking twice over the live dump.
	h.send(key("s"))
	h.send(key("s"))
	if h.m.state.Phase == game.PhaseCrashed {
		t.Fatal("s s did not reboot")
	}
	if h.m.state.Rules.Name != "short" {
		t.Errorf("rebooted on %q, want short", h.m.state.Rules.Name)
	}
}

// An upsolve without the editorial recovers 1.5x and says so in the
// story's words (coredump.recovered).
func TestUpsolveRecoveredNote(t *testing.T) {
	h := newHarness(t)
	h.crashWithDump()
	dump := *h.m.meta.CoreDump
	h.send(screens.Upsolve{Problem: dump.Problem})
	h.send(submitMsg{})
	want := recoveredNote(dump.Recovers(game.Independent))
	if !strings.Contains(h.m.flash, "The grin comes back") || h.m.flash != want {
		t.Errorf("flash %q, want %q", h.m.flash, want)
	}
}

func TestQuitAndResumeWords(t *testing.T) {
	if s := quitWarn(90 * time.Second); !strings.Contains(s, "01:30 left on this door") || !strings.Contains(s, "ctrl+c again") {
		t.Errorf("quitWarn %q", s)
	}
	if s := quitWarn(-time.Minute); !strings.Contains(s, "overtime") {
		t.Errorf("quitWarn past a soft deadline %q", s)
	}
	if s := resumeNote(90 * time.Second); !strings.Contains(s, "01:30 left") {
		t.Errorf("resumeNote %q", s)
	}
}

// GRUB shows how today's nightly went, and past the budget says so
// (RT15, RT12).
func TestGrubNightlyBest(t *testing.T) {
	h := newHarness(t)
	h.m.d.Budget = time.Minute
	h.send(screens.StartBoot{Preset: "standard", Seed: meta.NightlySeed(h.clock.t)})
	h.verdict = "WA"
	if got := h.play(); got != game.PhaseCrashed {
		t.Fatalf("boot ended %q", got)
	}
	h.send(screens.OpenGrub{})
	v := ansi.Strip(h.m.top().View(140, 40))
	if !strings.Contains(v, "best: 0 doors cleared") || !strings.Contains(v, "today ") {
		t.Errorf("GRUB lacks the nightly's best or the budget:\n%s", v)
	}
}

// A due regression solved again from GRUB, without the editorial, gets
// the White Queen's line (regression.cleared).
func TestRegressionFixedNote(t *testing.T) {
	h := newHarness(t)
	h.crashWithDump()
	id := h.m.state.Crash.Door.Problem
	h.send(screens.OpenGrub{})
	game.Upsolve(&h.m.meta, id, game.Editorial, time.Minute, h.clock.t) // banks the dump; the regression stays
	h.clock.t = h.clock.t.Add(4 * 24 * time.Hour)                       // due now
	h.send(screens.Upsolve{Problem: id})
	h.send(submitMsg{})
	if !strings.Contains(h.m.flash, "White Queen forgets it") {
		t.Errorf("flash %q, want regression.cleared", h.m.flash)
	}
}
