package app

import (
	"strings"
	"testing"
	"testing/synctest"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/screens"
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
