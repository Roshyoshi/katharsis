// demo-boot cycles through every boot screen with fixture data, so a human
// can eyeball them without playing a boot: n and p switch screens, every
// other key goes to the screen, and whatever the screen emits is shown in
// the bottom line instead of being acted on. ctrl+c quits.
package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/boot"
	"katharsis/internal/screens/boot/fixture"
)

// page is one screen of the demo, built fresh each time it is shown.
type page struct {
	name string
	st   func() *game.State
	meta func() game.Meta
	make func(boot.Env) screens.Screen
}

func pages() []page {
	std := fixture.Meta
	crash := func(name string, cause game.Outcome) page {
		return page{name,
			func() *game.State { return fixture.Crash(cause) },
			func() game.Meta { return fixture.CrashMeta(cause) },
			func(e boot.Env) screens.Screen { return boot.NewCrash(e) }}
	}
	return []page{
		{"route", fixture.Route, std, func(e boot.Env) screens.Screen { return boot.NewRoute(e) }},
		{"route, three offers", fixture.Route3, std, func(e boot.Env) screens.Screen { return boot.NewRoute(e) }},
		{"doors", fixture.Doors, std, func(e boot.Env) screens.Screen { return boot.NewDoors(e) }},
		{"doors, fork + regression", fixture.DoorsFork, std, func(e boot.Env) screens.Screen { return boot.NewDoors(e) }},
		{"doors, boss", fixture.Boss, std, func(e boot.Env) screens.Screen { return boot.NewDoors(e) }},
		{"map", fixture.Doors, std, func(e boot.Env) screens.Screen { return boot.NewMap(e) }},
		{"merge window", fixture.Merge, std, func(e boot.Env) screens.Screen { return boot.NewMerge(e) }},
		{"upgrade after a clear", fixture.Upgrade, std, func(e boot.Env) screens.Screen { return boot.NewUpgradeScreen(e) }},
		crash("crash, timer", game.TimedOut),
		crash("crash, out of lives", game.OutOfLives),
		crash("crash, :q!", game.GaveUp),
		{"cleared", fixture.Cleared, fixture.ClearedMeta, func(e boot.Env) screens.Screen { return boot.NewCleared(e) }},
		{"HUD bar", fixture.Problem, std, func(e boot.Env) screens.Screen { return &hudPage{env: e} }},
	}
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type model struct {
	pages   []page
	i       int
	env     boot.Env
	cur     screens.Screen
	w, h    int
	started time.Time // real time the page opened; its clock runs from fixture.Now
	last    string    // last message the screen emitted
}

func (m *model) open(i int) tea.Cmd {
	n := len(m.pages)
	m.i = (i%n + n) % n
	p := m.pages[m.i]
	m.env = boot.Env{State: p.st(), Meta: p.meta(), Lookup: fixture.Lookup, Now: fixture.Now}
	m.cur = p.make(m.env)
	m.started = time.Now()
	m.last = ""
	if in, ok := m.cur.(interface{ Init() tea.Cmd }); ok {
		return in.Init()
	}
	return nil
}

func (m *model) Init() tea.Cmd { return tea.Batch(tick(), m.open(0)) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		// Let the fixture clock run so uptimes and the HUD countdown move.
		if r, ok := m.cur.(interface{ Refresh(boot.Env) }); ok {
			m.env.Now = fixture.Now.Add(time.Since(m.started))
			r.Refresh(m.env)
		}
		return m, tick()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "n":
			return m, m.open(m.i + 1)
		case "p":
			return m, m.open(m.i - 1)
		}
	}
	if emittedByScreen(msg) {
		m.last = fmt.Sprintf("%T%+v", msg, msg)
		return m, nil
	}
	var cmd tea.Cmd
	m.cur, cmd = m.cur.Update(msg)
	return m, cmd
}

// emittedByScreen tells a screen's output (an engine action or an app
// intent) from input meant for the screen.
func emittedByScreen(msg tea.Msg) bool {
	if _, ok := msg.(game.Action); ok {
		return true
	}
	t := reflect.TypeOf(msg)
	return t != nil && t.PkgPath() == "katharsis/internal/screens"
}

var bar = lipgloss.NewStyle().Reverse(true)

func (m *model) View() tea.View {
	w, h := m.w, m.h
	if w <= 0 || h <= 0 {
		w, h = 80, 24
	}
	status := fmt.Sprintf(" demo-boot %d/%d %s · n/p screens · ctrl+c quit", m.i+1, len(m.pages), m.pages[m.i].name)
	if m.last != "" {
		status += " · emitted " + m.last
	}
	status = ansi.Truncate(status, w, "…")
	status += strings.Repeat(" ", max(w-ansi.StringWidth(status), 0))
	v := tea.NewView(m.cur.View(w, h-1) + "\n" + bar.Render(status))
	v.AltScreen = true
	return v
}

// hudPage shows HUDBar at the widths the HUD pane takes, for the open door
// and two variants: a blind, cursed door in its last two minutes, and a
// regression boss after two quacks.
type hudPage struct{ env boot.Env }

func (h *hudPage) Refresh(e boot.Env) { h.env = e }

func (h *hudPage) Update(tea.Msg) (screens.Screen, tea.Cmd) { return h, nil }

var (
	paneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	paneTitle = lipgloss.NewStyle().Faint(true)
)

// pane is HUDBar for st at w columns, the HUD pane's width, in a box.
func (h *hudPage) pane(st *game.State, w int, title string) string {
	p, _ := fixture.Lookup(st.Current.Door.Problem)
	title = paneTitle.Render(fmt.Sprintf("%s · %d cols", title, w))
	return paneStyle.Width(w + 2).Render(title + "\n" + boot.HUDBar(st, p, h.env.Now, w))
}

func (h *hudPage) View(width, height int) string {
	open := h.env.State
	crit := fixture.Problem()
	crit.Current.Door.Blind, crit.Current.Door.Cursed = true, true
	crit.Current.Deadline = fixture.Now.Add(100 * time.Second)
	boss := fixture.Problem()
	boss.Current.Quacks = 2
	boss.Current.Door.Regression, boss.Current.Door.Boss = true, "heisenbug"
	boss.Current.Deadline = fixture.Now.Add(9 * time.Minute)

	panes := []string{
		h.pane(open, 48, "open door"),
		h.pane(crit, 35, "blind+cursed"),
		h.pane(boss, 60, "boss, 2 quacks"),
	}
	s := lipgloss.JoinHorizontal(lipgloss.Top, panes[0], "  ", panes[1], "  ", panes[2])
	if lipgloss.Width(s) > width {
		s = lipgloss.JoinVertical(lipgloss.Left, panes...)
	}
	ls := strings.Split(s, "\n")
	for i, l := range ls {
		ls[i] = ansi.Truncate(l, width, "…")
	}
	for len(ls) < height {
		ls = append(ls, "")
	}
	return strings.Join(ls[:height], "\n")
}

func main() {
	if _, err := tea.NewProgram(&model{pages: pages()}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "demo-boot:", err)
		os.Exit(1)
	}
}
