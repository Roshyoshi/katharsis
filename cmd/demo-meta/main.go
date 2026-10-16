// demo-meta shows every out-of-boot screen with fixture data so a human can
// eyeball them: the GRUB menu, git log, top, apt, the setuid bits' find /
// -perm -4000 and the reader. It routes the
// screens' intents the way the app will (git log and top push, q/esc pop)
// and prints anything it cannot act on, a StartBoot or an OpenURL, in the
// status line instead.
//
//	go run ./cmd/demo-meta                 # start at GRUB
//	go run ./cmd/demo-meta -screen top     # start somewhere else
//	go run ./cmd/demo-meta -screen setuid  # find / -perm -4000 (setuid-percent: past a clear)
//	go run ./cmd/demo-meta -status=false   # give screens the full height
//
// Keys: tab cycles the top-level screen, ctrl+c quits; everything else goes
// to the screen.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/meta/fixture"
	"katharsis/internal/screens/reader"
	"katharsis/internal/ui"
)

// stops are the top-level screens tab cycles through, in order.
var stops = []struct {
	name string
	make func() screens.Screen
}{
	{"grub", func() screens.Screen { return grub() }},
	{"log", func() screens.Screen { return meta.NewLog(fixture.Boots(), nil) }},
	{"top", func() screens.Screen { return meta.NewTop(fixture.Stats()) }},
	{"shop", func() screens.Screen { return meta.NewShop(fixture.ShopConfig()) }},
	{"setuid", func() screens.Screen { return meta.NewSetuid(fixture.SetuidConfig()) }},
	{"setuid-percent", func() screens.Screen { return meta.NewSetuid(fixture.ShowsPercent(fixture.SetuidConfig())) }},
	{"statement", func() screens.Screen { return page(fixture.Statement) }},
	{"editorial", func() screens.Screen { return page(fixture.Editorial) }},
	{"learn", func() screens.Screen { return page(fixture.Learn) }},
}

// grub uses the fixture player with flavor's real prestige names and
// banner, unlike the goldens, so the demo shows the text players will see.
func grub() *meta.Menu {
	c := fixture.GrubConfig()
	c.PrestigeName, c.Banner = nil, nil
	return meta.NewGrub(c)
}

func page(d fixture.Doc) *reader.Model {
	return reader.New(reader.Doc{Title: d.Title, Source: d.Source, URL: d.URL, Markdown: d.Markdown})
}

type model struct {
	stop          int
	stack         []screens.Screen
	status        bool
	last          string
	width, height int
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) push(s screens.Screen) { m.stack = append(m.stack, s) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			// The GRUB editor uses tab for completion; let it have it.
			if g, ok := m.top().(*meta.Menu); !ok || !g.Editing() {
				m.stop = (m.stop + 1) % len(stops)
				m.stack = []screens.Screen{stops[m.stop].make()}
				m.last = ""
				return m, nil
			}
		}
	case screens.Quit:
		return m, tea.Quit
	case screens.Back:
		m.note(msg)
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		return m, nil
	case screens.OpenLog:
		m.note(msg)
		m.push(meta.NewLog(fixture.Boots(), nil))
		return m, nil
	case screens.OpenTop:
		m.note(msg)
		m.push(meta.NewTop(fixture.Stats()))
		return m, nil
	case screens.OpenLearn:
		m.note(msg)
		m.push(page(fixture.Learn))
		return m, nil
	case screens.OpenShop:
		m.note(msg)
		m.push(meta.NewShop(fixture.ShopConfig()))
		return m, nil
	case screens.OpenSetuid:
		m.note(msg)
		m.push(meta.NewSetuid(fixture.SetuidConfig()))
		return m, nil
	case screens.Upsolve:
		m.note(msg)
		m.push(page(fixture.Statement))
		return m, nil
	case screens.OpenStatement, screens.OpenEditorial, screens.StartBoot, reader.OpenURL,
		screens.Install, screens.UseTheme, meta.SetuidHint, meta.SetuidNotices:
		m.note(msg)
		return m, nil
	}
	s, cmd := m.top().Update(msg)
	m.stack[len(m.stack)-1] = s
	return m, cmd
}

func (m *model) top() screens.Screen { return m.stack[len(m.stack)-1] }

func (m *model) note(msg tea.Msg) { m.last = fmt.Sprintf("%T%+v", msg, msg) }

func (m *model) View() tea.View {
	w, h := m.width, m.height
	if w == 0 || h == 0 {
		w, h = 80, 24
	}
	sh := h
	if m.status {
		sh = h - 1
	}
	out := m.top().View(w, max(sh, 1))
	if m.status {
		s := "demo-meta " + stops[m.stop].name
		if m.last != "" {
			s += ui.G.Sep + "emitted " + m.last
		}
		out += "\n" + ui.StatusBar(w, s, "tab", "next screen", "ctrl+c", "quit")
	}
	v := tea.NewView(out)
	v.AltScreen = true
	return v
}

func main() {
	start := flag.String("screen", "grub", "first screen: grub, log, top, shop, setuid, setuid-percent, statement, editorial, learn")
	status := flag.Bool("status", true, "show the demo's status line (screens get one row less)")
	flag.Parse()

	m := &model{status: *status}
	for i, s := range stops {
		if s.name == *start {
			m.stop = i
		}
	}
	if stops[m.stop].name != *start {
		fmt.Fprintf(os.Stderr, "demo-meta: unknown screen %q\n", *start)
		os.Exit(2)
	}
	m.stack = []screens.Screen{stops[m.stop].make()}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "demo-meta:", err)
		os.Exit(1)
	}
}
