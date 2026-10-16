package app

import (
	"strings"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/store"
	"katharsis/internal/units"
)

func (h *harness) lastLine() string {
	lines := strings.Split(ansi.Strip(h.m.View().Content), "\n")
	return lines[len(lines)-1]
}

func (h *harness) answers(kind, field string) []string {
	h.t.Helper()
	var out []string
	for _, e := range h.events() {
		if e.Kind == kind {
			out = append(out, payload(h.t, e)[field].(string))
		}
	}
	return out
}

// The end line asks under the crash screen, once a day for worth-it and
// once a boot for how it felt; its keys answer and the screen's keys pass
// through. Never inside a door (GR6).
func TestEndLine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newPromptHarness(t)
		h.send(tea.KeyPressMsg{Code: tea.KeyEscape}) // skips the rating question
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		if strings.Contains(h.lastLine(), "worth it") {
			t.Fatalf("the end line shows in a door: %q", h.lastLine())
		}
		h.send(game.GiveUp{})
		if got := h.lastLine(); !strings.Contains(got, "worth it today? y / n") || !strings.Contains(got, "1 easy") {
			t.Fatalf("crash screen's last line %q, want the end line", got)
		}
		h.send(key("y"))
		h.send(key("3"))
		if got := h.lastLine(); strings.Contains(got, "worth it") || strings.Contains(got, "easy") {
			t.Errorf("the line stayed after both answers: %q", got)
		}
		if got := h.answers(store.EventWorthIt, "answer"); len(got) != 1 || got[0] != "yes" {
			t.Errorf("worth_it answers %v, want [yes]", got)
		}

		// The next boot ends the same day: worth-it is not asked again, how
		// it felt is. Screen keys pass through: the first r ends the crash
		// screen's reveal, the second reboots.
		h.send(key("r"))
		h.send(key("r"))
		if h.m.state.Phase == game.PhaseCrashed {
			t.Fatal("r did not reboot through the line")
		}
		h.enterDoor()
		h.send(game.GiveUp{})
		if got := h.lastLine(); strings.Contains(got, "worth it") || !strings.Contains(got, "3 hard") {
			t.Errorf("second crash's line %q, want only how it felt", got)
		}
		h.send(key("y")) // not the line's key now: it ends the reveal
		h.send(key("r"))
		if got := h.answers(store.EventSelfReport, "felt"); len(got) != 2 || got[0] != "hard" || got[1] != "skip" {
			t.Errorf("self_report answers %v, want [hard skip]", got)
		}
		if got := h.answers(store.EventWorthIt, "answer"); len(got) != 1 {
			t.Errorf("worth_it answers %v, want one a day", got)
		}

		// A restart lands on GRUB, which has no line.
		h.enterDoor()
		h.send(game.GiveUp{})
		h.promptRestart()
		if strings.Contains(h.lastLine(), "worth it") {
			t.Errorf("GRUB shows the end line: %q", h.lastLine())
		}
	})
}

// With prompts off (headless tests, --no-prompts) there is no line.
func TestEndLineOff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(game.GiveUp{})
		if strings.Contains(h.lastLine(), "worth it") {
			t.Errorf("prompts off, yet: %q", h.lastLine())
		}
	})
}

// top shows a fresh alarm once and notes it; opened again within the week
// it does not repeat.
func TestTopAlarmsOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		for range 3 {
			h.m.event(store.EventWorthIt, "", kv{"answer": "no"})
		}
		h.events()
		h.send(screens.OpenTop{})
		top := h.m.top().(*meta.Top)
		if v := ansi.Strip(top.View(100, 30)); !strings.Contains(v, "! 3 of 3 worth-it answers in 14 days were no") {
			t.Fatalf("top lacks the alarm:\n%s", v)
		}
		h.send(screens.Back{})
		if got := h.answers(store.EventAlarmShown, "alarm"); len(got) != 1 || got[0] != store.AlarmWorthIt {
			t.Fatalf("alarm_shown %v", got)
		}
		h.send(screens.OpenTop{})
		if v := ansi.Strip(h.m.top().View(100, 30)); strings.Contains(v, "worth-it answers") {
			t.Errorf("the alarm showed twice in a week:\n%s", v)
		}
		if got := h.answers(store.EventAlarmShown, "alarm"); len(got) != 1 {
			t.Errorf("alarm_shown %v after the second look", got)
		}
	})
}

// git log opens under its calendar, read from the store: a cleared boot
// is one active day, and c on it shows the boot's door time, the boot and
// its clear.
func TestLogCalendar(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		if got := h.play(); got != game.PhaseCleared {
			t.Fatalf("boot ended %q, want cleared", got)
		}
		boots, err := h.st.Boots(1)
		if err != nil || len(boots) != 1 {
			t.Fatalf("stored boots %+v, %v", boots, err)
		}
		h.send(screens.OpenLog{})
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "TIME+ per day") || !strings.Contains(v, "1 active day (last 365)") {
			t.Fatalf("git log has no calendar:\n%s", v)
		}
		h.send(key("c"))
		want := units.Clock(boots[0].Uptime) + " in doors · 1 boot · 1 cleared"
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, want) {
			t.Errorf("c on the boot's day lacks %q:\n%s", want, v)
		}
	})
}
