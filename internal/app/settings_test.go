package app

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/store"
)

// fakeSource is a content source's login over the fake catalog: logged
// in, its problem is judgeable, as a CSES login makes CSES's.
type fakeSource struct {
	cat     *fakeCatalog
	id      catalog.ID
	nick    string
	logins  [][2]string
	logouts int
}

func (f *fakeSource) Name() string            { return "CSES" }
func (f *fakeSource) Account() (string, bool) { return f.nick, f.nick != "" }
func (f *fakeSource) Login(_ context.Context, nick, pass string) error {
	f.logins = append(f.logins, [2]string{nick, pass})
	if pass != "hunter2" {
		return errors.New("wrong nick or password")
	}
	f.nick = nick
	f.cat.judge(f.id, true)
	return nil
}
func (f *fakeSource) Logout() error {
	f.logouts++
	f.nick = ""
	f.cat.judge(f.id, false)
	return nil
}

// judge sets id judgeable or not in a new slice, so a pool read before
// keeps the old flag, as a copy of a real catalog's would.
func (c *fakeCatalog) judge(id catalog.ID, ok bool) {
	c.all = slices.Clone(c.all)
	for i := range c.all {
		if c.all[i].ID == id {
			c.all[i].Judgeable = ok
			c.byID[id] = c.all[i]
		}
	}
}

func keyOf(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func (h *harness) typeText(s string) {
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func (h *harness) screen() string { return ansi.Strip(h.m.View().Content) }

// newSourceHarness is the app with a fake CSES login over its catalog,
// whose first problem only a login makes judgeable.
func newSourceHarness(t *testing.T) (*harness, *fakeSource) {
	h := newHarness(t)
	cat := h.m.d.Catalog.(*fakeCatalog)
	src := &fakeSource{cat: cat, id: cat.all[0].ID}
	cat.judge(src.id, false)
	h.m.pool = cat.All()
	h.m.d.Accounts = []Account{src}
	h.m.d.Settings = func() []Setting {
		return []Setting{{Key: "session_budget", Value: "3h", Note: "settings.toml"}}
	}
	return h, src
}

// GRUB's passwd, the entry above poweroff, opens the settings screen; a
// login there runs the source's Login with what was typed, shows the
// account logged in, records it without the password, and the pool
// judges the source's problems. q goes back to GRUB.
func TestSettingsLoginThroughGrub(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, src := newSourceHarness(t)
		h.send(key("G"))
		h.send(keyOf(tea.KeyUp))
		h.send(keyOf(tea.KeyEnter))
		if _, ok := h.m.top().(*meta.Settings); !ok {
			t.Fatalf("passwd opened %T", h.m.top())
		}
		if v := h.screen(); !strings.Contains(v, "session_budget") || !strings.Contains(v, "not logged in") {
			t.Fatalf("the screen:\n%s", v)
		}
		h.send(keyOf(tea.KeyEnter))
		h.typeText("alice")
		h.send(keyOf(tea.KeyEnter))
		h.typeText("hunter2")
		if v := h.screen(); strings.Contains(v, "hunter2") {
			t.Fatalf("the password shows:\n%s", v)
		}
		h.send(keyOf(tea.KeyEnter))
		if len(src.logins) != 1 || src.logins[0] != [2]string{"alice", "hunter2"} {
			t.Fatalf("logins %q", src.logins)
		}
		if v := h.screen(); !strings.Contains(v, "logged in as alice") || strings.Contains(v, "hunter2") {
			t.Errorf("after the login:\n%s", v)
		}
		i := slices.IndexFunc(h.m.pool, func(p catalog.Problem) bool { return p.ID == src.id })
		if i < 0 || !h.m.pool[i].Judgeable {
			t.Errorf("the pool does not judge %s after the login", src.id)
		}
		evs := h.eventsOf(store.EventAccount)
		if len(evs) != 1 || evs[0]["source"] != "CSES" || evs[0]["action"] != "login" {
			t.Errorf("account events: %v", evs)
		}
		for _, e := range h.events() {
			if strings.Contains(string(e.Data.(json.RawMessage)), "hunter2") {
				t.Errorf("%s event has the password", e.Kind)
			}
		}
		if strings.Contains(h.m.flash, "hunter2") {
			t.Errorf("the flash has the password: %s", h.m.flash)
		}
		// Log out again: y, after the question.
		h.send(keyOf(tea.KeyEnter))
		h.send(key("y"))
		if src.logouts != 1 || !strings.Contains(h.screen(), "not logged in") {
			t.Errorf("%d logouts:\n%s", src.logouts, h.screen())
		}
		if i := slices.IndexFunc(h.m.pool, func(p catalog.Problem) bool { return p.ID == src.id }); h.m.pool[i].Judgeable {
			t.Error("the pool judges the source after the logout")
		}
		h.send(key("q"))
		if _, ok := h.m.top().(*meta.Menu); !ok {
			t.Fatalf("q left %T on top", h.m.top())
		}
	})
}

// A failed login says the site's error and records nothing.
func TestSettingsLoginFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, src := newSourceHarness(t)
		h.send(screens.OpenSettings{})
		h.send(keyOf(tea.KeyEnter))
		h.typeText("alice")
		h.send(keyOf(tea.KeyEnter))
		h.typeText("nope")
		h.send(keyOf(tea.KeyEnter))
		if v := h.screen(); len(src.logins) != 1 || !strings.Contains(v, "wrong nick or password") || !strings.Contains(v, "not logged in") {
			t.Errorf("%d logins:\n%s", len(src.logins), v)
		}
		if evs := h.eventsOf(store.EventAccount); len(evs) != 0 {
			t.Errorf("account events: %v", evs)
		}
	})
}

// No accounts hides the section.
func TestSettingsWithoutAccounts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.OpenSettings{})
		if v := h.screen(); strings.Contains(v, "ACCOUNTS") || !strings.Contains(v, "passwd") {
			t.Errorf("the screen:\n%s", v)
		}
	})
}

// With a boot under way a logout stops the pool offering what it can no
// longer judge, and a login adds nothing until the next boot.
func TestSettingsRepoolInABoot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, src := newSourceHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		h.toDoors()
		n := len(h.m.pool)
		at := func(id catalog.ID) catalog.Problem {
			return h.m.pool[slices.IndexFunc(h.m.pool, func(p catalog.Problem) bool { return p.ID == id })]
		}
		if err := src.Login(context.Background(), "alice", "hunter2"); err != nil {
			t.Fatal(err)
		}
		h.send(meta.AccountDone{Name: "CSES", Nick: "alice"})
		if at(src.id).Judgeable {
			t.Error("a login mid-boot added to the boot's pool")
		}
		other := h.m.pool[1].ID
		h.m.d.Catalog.(*fakeCatalog).judge(other, false) // as a logout would
		h.send(meta.AccountDone{Name: "CSES", Logout: true})
		if len(h.m.pool) != n || at(other).Judgeable {
			t.Errorf("after a logout the pool has %d of %d, %s judgeable %v", len(h.m.pool), n, other, at(other).Judgeable)
		}
	})
}
