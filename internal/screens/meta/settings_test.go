package meta_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/ui"
)

// fakeAccount is a source's login that records what it was asked: a login
// with the right password succeeds, any other fails as the site would.
type fakeAccount struct {
	name, nick, want string
	in               bool
	logins           [][2]string
	logouts          int
}

func (f *fakeAccount) Name() string            { return f.name }
func (f *fakeAccount) Account() (string, bool) { return f.nick, f.in }
func (f *fakeAccount) Logout() error           { f.logouts++; f.nick, f.in = "", false; return nil }
func (f *fakeAccount) Login(_ context.Context, nick, pass string) error {
	f.logins = append(f.logins, [2]string{nick, pass})
	if pass != f.want {
		return errors.New("wrong nick or password")
	}
	f.nick, f.in = nick, true
	return nil
}

// secret is the password the tests type; no frame may show it.
const secret = "hunter2"

func settingRows() []meta.Setting {
	return []meta.Setting{
		{Key: "session_budget", Value: "3h", Note: "settings.toml (~/.config/katharsis)"},
		{Key: "setuid_notices", Value: "true", Note: "default"},
		{Key: "setuid_spoilers", Value: "none", Note: "default"},
		{Key: "sample_warnings", Value: "-Wall -Wextra -Wshadow", Note: "default"},
		{Key: "budget", Value: "1h", Note: "flag --budget"},
	}
}

func cses() *fakeAccount { return &fakeAccount{name: "CSES", want: secret} }

func settingsWith(accts ...meta.Account) *meta.Settings {
	return meta.NewSettings(meta.SettingsConfig{Settings: settingRows(), Accounts: accts})
}

// feed sends keys (and messages) without running their commands, the way a
// login is still out while the screen draws; it returns the last command.
func feed(s screens.Screen, in ...any) tea.Cmd {
	var cmd tea.Cmd
	for _, x := range in {
		msg, ok := x.(tea.Msg)
		if k, isKey := x.(string); isKey {
			msg, ok = key(k), true
		}
		if ok {
			_, cmd = s.Update(msg)
		}
		s.View(80, 24)
	}
	return cmd
}

// typing is the keys that type s.
func typing(s string) []any {
	var out []any
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}

func loginKeys(nick, pass string) []any {
	keys := append([]any{"enter"}, typing(nick)...)
	keys = append(keys, "enter")
	return append(keys, typing(pass)...)
}

func TestSettingsGoldens(t *testing.T) {
	cases := []struct {
		name string
		make func() screens.Screen
		in   []any
	}{
		{"settings_none", func() screens.Screen { return settingsWith() }, []any{"j"}},
		{"settings_out", func() screens.Screen { return settingsWith(cses()) }, nil},
		{"settings_prompt", func() screens.Screen { return settingsWith(cses()) }, loginKeys("alice", secret)},
		{"settings_logging", func() screens.Screen { return settingsWith(cses()) }, append(loginKeys("alice", secret), "enter")},
		{"settings_error", func() screens.Screen { return settingsWith(cses()) },
			append(loginKeys("alice", "nope"), "enter", meta.AccountDone{Name: "CSES", Nick: "alice", Err: errors.New("wrong nick or password")})},
		{"settings_in", func() screens.Screen {
			return settingsWith(&fakeAccount{name: "CSES", nick: "alice", in: true})
		}, nil},
	}
	for _, c := range cases {
		for _, sz := range sizes {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				s := c.make()
				feed(s, c.in...)
				golden(t, name, s.View(sz[0], sz[1]))
			})
		}
	}
	t.Run("settings_prompt_ascii_80x24", func(t *testing.T) {
		ui.SetGlyphs(ui.ASCII)
		defer ui.SetGlyphs(ui.Unicode)
		s := settingsWith(cses())
		feed(s, loginKeys("alice", secret)...)
		v := s.View(80, 24)
		if !strings.Contains(plain(v), strings.Repeat("*", len(secret))) {
			t.Errorf("the password is not masked with *:\n%s", plain(v))
		}
		golden(t, "settings_prompt_ascii_80x24", v)
	})
}

// Every state fits from cramped to huge: exactly height lines, none wider
// than width.
func TestSettingsFits(t *testing.T) {
	for name, mk := range map[string]func() screens.Screen{
		"empty":  func() screens.Screen { return meta.NewSettings(meta.SettingsConfig{}) },
		"none":   func() screens.Screen { return settingsWith() },
		"out":    func() screens.Screen { return settingsWith(cses()) },
		"prompt": func() screens.Screen { s := settingsWith(cses()); feed(s, loginKeys("alice", secret)...); return s },
		"error": func() screens.Screen {
			s := settingsWith(cses())
			feed(s, meta.AccountDone{Name: "CSES", Err: errors.New(strings.Repeat("a long error ", 20))})
			return s
		},
		"top": func() screens.Screen { s := settingsWith(cses()); feed(s, "g"); return s },
	} {
		for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {60, 15}, {80, 24}, {140, 40}, {220, 60}} {
			w, h := sz[0], sz[1]
			lines := strings.Split(mk().View(w, h), "\n")
			if len(lines) != h {
				t.Errorf("%s at %dx%d: %d lines, want %d", name, w, h, len(lines), h)
			}
			for i, l := range lines {
				if n := ansi.StringWidth(l); n > w {
					t.Errorf("%s at %dx%d: line %d is %d wide: %q", name, w, h, i, n, ansi.Strip(l))
				}
			}
		}
	}
}

// The password shows in no frame: not as it is typed, not while logging
// in, not after the result, at any size; and the error a source quotes it
// in comes back without it.
func TestSettingsPasswordNeverShows(t *testing.T) {
	a := &fakeAccount{name: "CSES", want: "other"}
	s := settingsWith(a)
	check := func(when string) {
		t.Helper()
		for _, sz := range [][2]int{{20, 6}, {80, 24}, {140, 40}} {
			if v := ansi.Strip(s.View(sz[0], sz[1])); strings.Contains(v, secret) {
				t.Fatalf("%s at %v the password shows:\n%s", when, sz, v)
			}
		}
	}
	keys := loginKeys("alice", secret)
	for i, k := range keys {
		feed(s, k)
		check(fmt.Sprintf("after key %d", i))
	}
	cmd := feed(s, "enter")
	check("logging in")
	done := cmd().(meta.AccountDone)
	feed(s, done)
	check("after the result")
	if len(a.logins) != 1 || a.logins[0] != [2]string{"alice", secret} {
		t.Errorf("logins %q", a.logins)
	}
	// A source that quotes the password back has it cut from the error.
	q := &quoting{fakeAccount: fakeAccount{name: "CSES"}}
	s = settingsWith(q)
	done = feed(s, append(loginKeys("alice", secret), "enter")...)().(meta.AccountDone)
	if strings.Contains(done.Err.Error(), secret) {
		t.Errorf("the error quotes the password: %v", done.Err)
	}
	feed(s, done)
	check("after a quoting error")
}

type quoting struct{ fakeAccount }

func (q *quoting) Login(context.Context, string, string) error {
	return fmt.Errorf("bad password %q", secret)
}

// esc drops the prompt and what was typed: enter again opens it empty.
func TestSettingsEscCancels(t *testing.T) {
	for _, at := range []int{1, 4, len(loginKeys("alice", secret))} {
		a := cses()
		s := settingsWith(a)
		cmd := feed(s, append(loginKeys("alice", secret)[:at], "esc")...)
		if cmd != nil || len(a.logins) != 0 {
			t.Errorf("esc after %d keys logged in: %v", at, a.logins)
		}
		v := plain(s.View(80, 24))
		if strings.Contains(v, "log in to CSES") || strings.Contains(v, "alice") {
			t.Errorf("esc after %d keys left the prompt:\n%s", at, v)
		}
		feed(s, "enter")
		v = plain(s.View(80, 24))
		if !strings.Contains(v, "log in to CSES") || strings.Contains(v, "alice") || strings.Contains(v, ui.G.Mask) {
			t.Errorf("the prompt after esc is not empty:\n%s", v)
		}
	}
}

// enter sends the login in a command, not in Update; its result says
// logged in, or the site's error with the account still out.
func TestSettingsLogin(t *testing.T) {
	a := cses()
	s := settingsWith(a)
	cmd := feed(s, append(loginKeys("alice", secret), "enter")...)
	if cmd == nil || len(a.logins) != 0 {
		t.Fatalf("Update logged in itself: %v", a.logins)
	}
	if v := plain(s.View(80, 24)); !strings.Contains(v, "logging in") {
		t.Errorf("no logging in:\n%s", v)
	}
	feed(s, "enter") // a second enter while busy does nothing
	done := cmd()
	wantMsg(t, done, meta.AccountDone{Name: "CSES", Nick: "alice"})
	feed(s, done)
	if v := plain(s.View(80, 24)); !strings.Contains(v, ui.G.Pass+" logged in as alice") {
		t.Errorf("no login:\n%s", v)
	}

	b := cses()
	s = settingsWith(b)
	done = feed(s, append(loginKeys("alice", "nope"), "enter")...)()
	feed(s, done)
	v := plain(s.View(80, 24))
	if !strings.Contains(v, "not logged in") || !strings.Contains(v, ui.G.Fail+" CSES: wrong nick or password") {
		t.Errorf("no error:\n%s", v)
	}
	// An empty nick or password does not log in.
	c := cses()
	s = settingsWith(c)
	if cmd := feed(s, "enter", "enter", "enter", "enter"); cmd != nil {
		t.Error("an empty nick logged in")
	}
	if cmd := feed(s, "a", "enter", "enter"); cmd != nil {
		t.Error("an empty password logged in")
	}
	// Run gets the work: the app runs it under its lifetime.
	ran := 0
	s = meta.NewSettings(meta.SettingsConfig{Accounts: []meta.Account{cses()}, Run: func(w func(context.Context) tea.Msg) tea.Cmd {
		ran++
		return func() tea.Msg { return w(context.Background()) }
	}})
	feed(s, append(loginKeys("alice", secret), "enter")...)
	if ran != 1 {
		t.Errorf("Run ran %d logins", ran)
	}
}

// enter on a logged-in account asks first: y logs out, anything else
// keeps the login.
func TestSettingsLogout(t *testing.T) {
	a := &fakeAccount{name: "CSES", nick: "alice", in: true}
	s := settingsWith(a)
	if cmd := feed(s, "enter", "n"); cmd != nil || a.logouts != 0 {
		t.Errorf("n logged out")
	}
	feed(s, "enter")
	if v := plain(s.View(80, 24)); !strings.Contains(v, "log out of CSES (alice)?") {
		t.Errorf("no question:\n%s", v)
	}
	cmd := feed(s, "y")
	if a.logouts != 0 {
		t.Error("Update logged out itself")
	}
	done := cmd()
	wantMsg(t, done, meta.AccountDone{Name: "CSES", Logout: true})
	feed(s, done)
	if v := plain(s.View(80, 24)); a.logouts != 1 || !strings.Contains(v, "not logged in") || !strings.Contains(v, "CSES: logged out") {
		t.Errorf("%d logouts:\n%s", a.logouts, v)
	}
}

// No accounts hides the section; q and esc go back, also with nothing to
// list.
func TestSettingsNoAccounts(t *testing.T) {
	for _, sz := range sizes {
		if v := plain(settingsWith().View(sz[0], sz[1])); strings.Contains(v, "ACCOUNTS") || !strings.Contains(v, "session_budget") {
			t.Errorf("at %v:\n%s", sz, v)
		}
	}
	wantMsg(t, press(t, settingsWith(), "q"), screens.Back{})
	wantMsg(t, press(t, meta.NewSettings(meta.SettingsConfig{}), "esc"), screens.Back{})
	wantMsg(t, press(t, meta.NewSettings(meta.SettingsConfig{}), "enter", "j", "k"), nil)
}

// q while typing is a letter, not back.
func TestSettingsPromptTakesLetters(t *testing.T) {
	s := settingsWith(cses())
	if cmd := feed(s, "enter", "q", "j", "k"); cmd != nil {
		t.Errorf("a letter in the prompt emitted %v", cmd())
	}
	if v := plain(s.View(80, 24)); !strings.Contains(v, "qjk") {
		t.Errorf("the nick is not qjk:\n%s", v)
	}
}
