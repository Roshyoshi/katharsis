// demo-rtfm shows the RTFM (learn mode) screens on the fixture curriculum
// so a human can eyeball them: the man-page index and the module pages it
// opens. It routes the screens' intents the way the app will (r opens the
// module in the reader, q/esc pop) and prints what it cannot act on, a
// StartTrial or an OpenURL, in the status line instead.
//
//	go run ./cmd/demo-rtfm                        # the fixture index
//	go run ./cmd/demo-rtfm -module prefix-sums    # straight to a page
//	go run ./cmd/demo-rtfm -guide ~/Library/Caches/katharsis/guide/<sha> -usaco
//	                                               # a real snapshot, with the
//	                                               # cached USACO catalog
//	go run ./cmd/demo-rtfm -status=false          # screens get the full height
//
// Keys: ctrl+c quits; everything else goes to the screen.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/content/usaco"
	"katharsis/internal/curriculum"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
	"katharsis/internal/screens/rtfm/fixture"
	"katharsis/internal/ui"
)

type model struct {
	cur    *curriculum.Curriculum
	stack  []screens.Screen
	status bool
	last   string
	width  int
	height int
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) top() screens.Screen { return m.stack[len(m.stack)-1] }

func (m *model) note(msg tea.Msg) { m.last = fmt.Sprintf("%T%+v", msg, msg) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case screens.Quit:
		return m, tea.Quit
	case screens.Back:
		m.note(msg)
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		return m, nil
	case screens.OpenModule:
		m.note(msg)
		mod, ok := m.cur.Module(msg.Module)
		if !ok {
			m.last = "no module " + msg.Module
			return m, nil
		}
		md, err := mod.Markdown()
		if err != nil {
			m.last = err.Error()
			return m, nil
		}
		m.stack = append(m.stack, reader.New(reader.Doc{
			Title: mod.Title, Source: "USACO Guide · " + mod.DivisionTitle, URL: mod.URL, Markdown: md,
		}))
		return m, nil
	case screens.StartTrial, reader.OpenURL:
		m.note(msg)
		return m, nil
	}
	s, cmd := m.top().Update(msg)
	m.stack[len(m.stack)-1] = s
	return m, cmd
}

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
		s := "demo-rtfm"
		if m.last != "" {
			s += ui.G.Sep + "emitted " + m.last
		}
		out += "\n" + ui.StatusBar(w, s, "ctrl+c", "quit")
	}
	v := tea.NewView(out)
	v.AltScreen = true
	return v
}

func main() {
	dir := flag.String("guide", fixture.GuideDir(), "USACO Guide snapshot (repository root or its content/)")
	useUSACO := flag.Bool("usaco", false, "judge and look problems up in the cached USACO catalog instead of the fixture's")
	module := flag.String("module", "", "start on this module's page instead of the index")
	status := flag.Bool("status", true, "show the demo's status line (screens get one row less)")
	flag.Parse()

	cur, err := curriculum.Load(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "demo-rtfm:", err)
		os.Exit(1)
	}
	var meta game.Meta
	judgeable, lookup := fixture.Judgeable, fixture.Lookup
	if *dir == fixture.GuideDir() {
		meta = fixture.Meta()
	}
	if *useUSACO {
		cat, err := usaco.Open(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, "demo-rtfm: usaco catalog:", err)
			os.Exit(1)
		}
		lookup = cat.Get
		judgeable = func(id catalog.ID) bool {
			p, ok := cat.Get(id)
			return ok && p.Judgeable
		}
	}

	b := rtfm.NewBrowser(cur, meta, judgeable, lookup)
	if *module != "" && !b.Open(*module) {
		fmt.Fprintf(os.Stderr, "demo-rtfm: no module %q\n", *module)
		os.Exit(2)
	}
	m := &model{cur: cur, stack: []screens.Screen{b}, status: *status}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "demo-rtfm:", err)
		os.Exit(1)
	}
}
