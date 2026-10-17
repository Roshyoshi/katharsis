package meta

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/screens"
	"katharsis/internal/ui"
)

// Account is a content source's login, as the settings screen manages it.
type Account interface {
	Name() string                                       // "CSES"
	Account() (nick string, ok bool)                    // the account in use
	Login(ctx context.Context, nick, pass string) error // checks it with the site, then stores it
	Logout() error
}

// Setting is one effective setting, listed read-only: its key, its value
// and what set it, such as {"session_budget", "3h", "settings.toml"}.
type Setting struct{ Key, Value, Note string }

// SettingsConfig is what passwd shows: the settings in effect, then the
// accounts the content sources log in with. No accounts hides that
// section.
type SettingsConfig struct {
	Settings []Setting
	Accounts []Account
	// Run runs a login or logout off the update loop, under the app's
	// lifetime; nil runs it in a plain command.
	Run func(work func(ctx context.Context) tea.Msg) tea.Cmd
}

// AccountDone is a login's or a logout's result, Err nil on success. The
// app takes it first, for what a source logged in or out changes, then
// hands it to the screen. It never carries the password.
type AccountDone struct {
	Name   string // Account.Name
	Nick   string // the nick logged in with; "" for a logout
	Logout bool
	Err    error
}

// loginTimeout bounds a login, which is a request to the site.
const loginTimeout = 30 * time.Second

// unlocks is what a login opens, by Account.Name: one line, with no count
// the screen cannot know.
var unlocks = map[string]string{
	"CSES": "CSES problems in RTFM practice and trials",
}

// Settings is passwd at GRUB: the game's settings in effect, read-only
// with what set each, and the content sources' accounts. enter on an
// account logs in, with a prompt for the nick and then the password, or
// logs out after a y. q or esc emits screens.Back.
type Settings struct {
	c     SettingsConfig
	accts []account
	sel   int // a row: the settings, then the accounts
	top   int // the first list line shown
	// prompt is the login being typed, nil for none; confirm asks y to log
	// out of the selected account.
	prompt  *prompt
	confirm bool
}

// account is one Account's row: its state when last read, and the last
// result to say under the list.
type account struct {
	a    Account
	nick string
	in   bool
	busy bool   // a login or logout is running
	as   string // the nick a running login logs in as
	msg  string // the last result: "logged in as alice", or the error
	err  bool
}

// prompt is a login as it is typed. pass is bytes so it can be zeroed:
// the screen keeps no password once the login is sent.
type prompt struct {
	field int // 0 the nick, 1 the password
	nick  string
	pass  []byte
}

// NewSettings builds passwd, reading each account's state.
func NewSettings(c SettingsConfig) *Settings {
	s := &Settings{c: c}
	for _, a := range c.Accounts {
		ac := account{a: a}
		ac.nick, ac.in = a.Account()
		s.accts = append(s.accts, ac)
	}
	if len(s.accts) > 0 {
		s.sel = len(c.Settings) // the first account: the rows that act
	}
	return s
}

func (s *Settings) rows() int { return len(s.c.Settings) + len(s.accts) }

// cur is the selected account, nil on a setting.
func (s *Settings) cur() *account {
	if i := s.sel - len(s.c.Settings); i >= 0 && i < len(s.accts) {
		return &s.accts[i]
	}
	return nil
}

func (s *Settings) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case AccountDone:
		s.done(msg)
		return s, nil
	case tea.PasteMsg:
		if s.prompt != nil {
			s.prompt.typed(msg.Content)
		}
		return s, nil
	case tea.KeyPressMsg:
		switch {
		case s.prompt != nil:
			return s, s.promptKey(msg)
		case s.confirm:
			s.confirm = false
			if msg.String() == "y" {
				return s, s.logout()
			}
			return s, nil
		}
		return s, s.key(msg)
	}
	return s, nil
}

func (s *Settings) key(k tea.KeyPressMsg) tea.Cmd {
	last := max(s.rows()-1, 0)
	switch k.String() {
	case "up", "k":
		s.sel = max(s.sel-1, 0)
	case "down", "j":
		s.sel = min(s.sel+1, last)
	case "home", "g":
		s.sel = 0
	case "end", "G":
		s.sel = last
	case "enter":
		a := s.cur()
		switch {
		case a == nil || a.busy:
		case a.in:
			s.confirm = true
		default:
			a.msg, a.err = "", false
			s.prompt = &prompt{}
		}
	case "q", "esc", "left", "backspace":
		return screens.Emit(screens.Back{})
	}
	return nil
}

// promptKey types into the login prompt: enter goes from the nick to the
// password and then logs in, tab and shift+tab change field, esc drops
// both fields.
func (s *Settings) promptKey(k tea.KeyPressMsg) tea.Cmd {
	p := s.prompt
	switch k.String() {
	case "esc":
		p.clear()
		s.prompt = nil
	case "enter":
		switch {
		case p.field == 0 && strings.TrimSpace(p.nick) != "":
			p.field = 1
		case p.field == 1 && strings.TrimSpace(p.nick) == "":
			p.field = 0
		case p.field == 1 && len(p.pass) > 0:
			return s.login()
		}
	case "tab", "down":
		p.field = 1
	case "shift+tab", "up":
		p.field = 0
	case "backspace", "ctrl+h":
		p.backspace()
	case "ctrl+u":
		if p.field == 0 {
			p.nick = ""
		} else {
			p.zero()
		}
	default:
		p.typed(k.Text)
	}
	return nil
}

// typed adds text to the field under the cursor, without its control
// characters; a nick takes no spaces either.
func (p *prompt) typed(text string) {
	for _, r := range text {
		switch {
		case unicode.IsControl(r):
		case p.field == 0 && !unicode.IsSpace(r):
			p.nick += string(r)
		case p.field == 1:
			p.pass = utf8.AppendRune(p.grow(utf8.RuneLen(r)), r)
		}
	}
}

// grow makes room for n more password bytes, zeroing the array it leaves
// behind so no copy of the password outlives the prompt.
func (p *prompt) grow(n int) []byte {
	if len(p.pass)+n <= cap(p.pass) {
		return p.pass
	}
	b := make([]byte, len(p.pass), 2*cap(p.pass)+max(n, 32))
	copy(b, p.pass)
	clear(p.pass[:cap(p.pass)])
	return b
}

func (p *prompt) backspace() {
	if p.field == 0 {
		_, n := utf8.DecodeLastRuneInString(p.nick)
		p.nick = p.nick[:len(p.nick)-n]
		return
	}
	_, n := utf8.DecodeLastRune(p.pass)
	clear(p.pass[len(p.pass)-n:])
	p.pass = p.pass[:len(p.pass)-n]
}

// zero empties the password field, its bytes too.
func (p *prompt) zero() {
	clear(p.pass[:cap(p.pass)])
	p.pass = p.pass[:0]
}

func (p *prompt) clear() {
	p.zero()
	p.nick = ""
}

// login sends the prompt's login in a command and drops the prompt: the
// password lives on only in the command, and leaves with it.
func (s *Settings) login() tea.Cmd {
	a := s.cur()
	nick, pass := strings.TrimSpace(s.prompt.nick), string(s.prompt.pass)
	s.prompt.clear()
	s.prompt = nil
	if a == nil {
		return nil
	}
	a.busy, a.as, a.msg, a.err = true, nick, "", false
	acct, name := a.a, a.a.Name()
	return s.run(func(ctx context.Context) tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, loginTimeout)
		defer cancel()
		err := acct.Login(ctx, nick, pass)
		return AccountDone{Name: name, Nick: nick, Err: scrub(err, pass)}
	})
}

func (s *Settings) logout() tea.Cmd {
	a := s.cur()
	if a == nil || a.busy {
		return nil
	}
	a.busy, a.msg, a.err = true, "", false
	acct, name := a.a, a.a.Name()
	return s.run(func(context.Context) tea.Msg {
		return AccountDone{Name: name, Logout: true, Err: acct.Logout()}
	})
}

func (s *Settings) run(work func(context.Context) tea.Msg) tea.Cmd {
	if s.c.Run != nil {
		return s.c.Run(work)
	}
	return func() tea.Msg { return work(context.Background()) }
}

// scrub is err with the password cut out of its text, should a source
// ever quote it back, and a timeout said plainly.
func scrub(err error, pass string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("timed out")
	case pass != "" && strings.Contains(err.Error(), pass):
		return errors.New(strings.ReplaceAll(err.Error(), pass, "***"))
	}
	return err
}

// done takes a login's or logout's result: the account read again, and
// the result said under the list.
func (s *Settings) done(d AccountDone) {
	for i := range s.accts {
		a := &s.accts[i]
		if a.a.Name() != d.Name {
			continue
		}
		a.busy = false
		a.nick, a.in = a.a.Account()
		switch {
		case d.Err != nil:
			a.msg, a.err = firstLine(d.Err.Error()), true
		case d.Logout:
			a.msg, a.err = "logged out", false
		default:
			a.msg, a.err = "logged in as "+cmp.Or(a.nick, d.Nick), false
		}
	}
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}

// ---- view ------------------------------------------------------------------------

// state is an account's state in words, the ✓ marking a login (R1).
func (a account) state() string {
	switch {
	case a.busy && a.in:
		return ui.Muted.Render("logging out" + ui.G.Ellipsis)
	case a.busy:
		return ui.Muted.Render("logging in" + ui.G.Ellipsis)
	case a.in:
		return ui.OK.Render(ui.G.Pass) + " logged in as " + ui.Strong.Render(a.nick)
	}
	return ui.Muted.Render("not logged in")
}

func (s *Settings) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	head := []string{
		ui.Clip(ui.Strong.Render("passwd")+ui.Muted.Render(ui.G.Sep+"settings and accounts"), w),
		"",
	}
	list, at := s.list(w)
	detail := s.detail(w)
	// The list takes what the header, the detail and the footer leave, and
	// scrolls to keep the selection on screen.
	room := max(h-len(head)-len(detail)-2, 1)
	if at >= 0 {
		if at < s.top {
			s.top = at
		}
		if at >= s.top+room {
			s.top = at - room + 1
		}
	}
	s.top = max(min(s.top, len(list)-room), 0)
	out := append([]string{}, head...)
	for i := s.top; i < len(list) && i < s.top+room; i++ {
		out = append(out, list[i])
	}
	out = ui.PadLines(out, max(h-len(detail)-1, len(head)))
	out = append(out, detail...)
	return ui.Layout(w, h, out, s.footer(w))
}

// list is the rows under their section titles, and the line the selected
// row is on (-1 for none).
func (s *Settings) list(w int) (lines []string, at int) {
	at = -1
	gutter := func(i int) string {
		if i == s.sel && s.prompt == nil {
			return ui.Focus.Render(ui.G.Cursor) + " "
		}
		return "  "
	}
	if len(s.c.Settings) > 0 {
		lines = append(lines, ui.Strong.Render("SETTINGS")+ui.Muted.Render(ui.G.Sep+"read-only here"))
		keyW, valW := 0, 0
		for _, st := range s.c.Settings {
			keyW, valW = max(keyW, ansi.StringWidth(st.Key)), max(valW, ansi.StringWidth(st.Value))
		}
		valW = min(valW, max(w/3, 8))
		for i, st := range s.c.Settings {
			if i == s.sel {
				at = len(lines)
			}
			lines = append(lines, gutter(i)+ui.Fit(st.Key, keyW)+"  "+ui.Fit(st.Value, valW)+"  "+ui.Muted.Render(st.Note))
		}
	}
	if len(s.accts) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, ui.Strong.Render("ACCOUNTS")+ui.Muted.Render(ui.G.Sep+"logins the content sources need"))
		nameW, stateW := 0, 0
		for _, a := range s.accts {
			nameW, stateW = max(nameW, ansi.StringWidth(a.a.Name())), max(stateW, ansi.StringWidth(a.state()))
		}
		for i, a := range s.accts {
			row := len(s.c.Settings) + i
			if row == s.sel {
				at = len(lines)
			}
			line := gutter(row) + ui.Fit(a.a.Name(), nameW) + "  " + ui.Fit(a.state(), stateW)
			if u := unlocks[a.a.Name()]; u != "" {
				line += "  " + ui.Muted.Render(u)
			}
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, ui.Muted.Render("  no settings or accounts to show"))
	}
	return lines, at
}

// detail is the lines under the list: the login prompt, the logout
// question, or what the selected row is and its last result.
func (s *Settings) detail(w int) []string {
	a := s.cur()
	field := func(i int, label, v string) string {
		l := "  " + ui.Fit(label, 9) + " " + v
		if s.prompt.field == i {
			l += ui.Select.Render(" ")
		}
		return l
	}
	switch {
	case s.prompt != nil && a != nil:
		p := s.prompt
		return []string{
			ui.Strong.Render("log in to " + a.a.Name()),
			field(0, "nick", p.nick),
			field(1, "password", strings.Repeat(ui.G.Mask, utf8.RuneCount(p.pass))),
		}
	case s.confirm && a != nil:
		return []string{ui.Danger.Render(fmt.Sprintf("log out of %s (%s)?", a.a.Name(), a.nick)) +
			" y logs out, any other key keeps it"}
	case a != nil && a.busy && a.in:
		return []string{ui.Muted.Render("logging out of " + a.a.Name() + ui.G.Ellipsis)}
	case a != nil && a.busy:
		return []string{ui.Muted.Render("logging in to " + a.a.Name() + " as " + a.as + ui.G.Ellipsis)}
	case a != nil && a.msg != "":
		mark := ui.OK.Render(ui.G.Pass)
		if a.err {
			mark = ui.Danger.Render(ui.G.Fail)
		}
		return ui.WrapN(mark+" "+a.a.Name()+": "+a.msg, max(w, 10), 2)
	case a == nil && s.sel < len(s.c.Settings):
		// What set it, in full where the row clips it, and when it is read.
		st := s.c.Settings[s.sel]
		text := st.Key + " = " + st.Value
		if st.Note != "" {
			text += " (" + st.Note + ")"
		}
		text += ui.G.Sep + "settings.toml and the flags are read at launch"
		return ui.WrapN(ui.Muted.Render(text), max(w, 10), 2)
	}
	return nil
}

func (s *Settings) footer(w int) string {
	switch {
	case s.prompt != nil:
		act := "next"
		if s.prompt.field == 1 {
			act = "log in"
		}
		return ui.Footer(w, "enter", act, "tab", "field", "esc", "cancel")
	case s.confirm:
		return ui.Footer(w, "y", "log out", "any key", "keep")
	}
	pairs := []string{"j/k", "move"}
	if a := s.cur(); a != nil && !a.busy {
		act := "log in"
		if a.in {
			act = "log out"
		}
		pairs = append(pairs, "enter", act)
	}
	return ui.Footer(w, append(pairs, "q", "back")...)
}
