package app

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
)

// submitMsg asks the app to judge the open door's full tests. The problem
// screen's key and the editor's :Submit both send it.
type submitMsg struct{}

// problemScreen is the pane beside the editor while a door is open: the
// boot HUD bar, the last full-test result, and the keys for the door. In an
// upsolve it shows the problem without a timer or lives.
//
// SPONGE: the sample-judging HUD from internal/hud (rejudge on save) is
//
//	embedded here once its boot mode lands; until then samples are
//	judged by :Submit only.
type problemScreen struct {
	m       *Model
	upsolve bool
	door    game.Door
	last    judgedMsg
	confirm bool // :q! asked once; the next Q confirms
}

func newProblemScreen(m *Model) *problemScreen {
	s := &problemScreen{m: m}
	if m.state != nil && m.state.Current != nil {
		s.door = m.state.Current.Door
	}
	return s
}

func newUpsolveScreen(m *Model, d game.Door) *problemScreen {
	return &problemScreen{m: m, upsolve: true, door: d}
}

func (s *problemScreen) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	key := k.String()
	if key != "Q" {
		s.confirm = false
	}
	switch key {
	case "s":
		return s, screens.Emit(submitMsg{})
	case "t":
		return s, screens.Emit(screens.OpenStatement{Problem: string(s.door.Problem)})
	case "?":
		if !s.upsolve {
			return s, screens.Emit(game.Quack{})
		}
	case "e":
		if s.upsolve {
			return s, screens.Emit(screens.OpenEditorial{Problem: string(s.door.Problem)})
		}
	case "R":
		if !s.upsolve {
			return s, screens.Emit(game.Retreat{})
		}
	case "Q":
		if s.upsolve {
			s.m.closeDoor()
			return s, screens.Emit(screens.Back{})
		}
		if !s.confirm {
			s.confirm = true
			return s, nil
		}
		return s, screens.Emit(game.GiveUp{})
	}
	return s, nil
}

var (
	faint = lipgloss.NewStyle().Faint(true)
	bold  = lipgloss.NewStyle().Bold(true)
	red   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	green = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

func (s *problemScreen) View(w, h int) string {
	var b strings.Builder
	line := func(t string) { b.WriteString(ansi.Truncate(t, w, "…") + "\n") }
	p, _ := s.m.d.Catalog.Get(s.door.Problem)

	switch {
	case s.upsolve:
		line(bold.Render("upsolve · "+p.Title) + faint.Render("  untimed, editorial allowed"))
	case s.m.state != nil:
		for _, l := range strings.Split(bootui.HUDBar(s.m.state, p, s.m.d.Now(), w), "\n") {
			line(l)
		}
	}
	line(faint.Render(strings.Repeat("─", w)))

	switch {
	case s.m.open == nil:
		line(faint.Render("fetching " + string(s.door.Problem) + "…"))
	case s.m.judging:
		line(faint.Render("judging full tests…"))
	case s.last.verdict != "":
		st := red
		if s.last.verdict == "AC" {
			st = green
		}
		line(st.Render(fmt.Sprintf("%s  %d/%d tests", s.last.verdict, s.last.passed, s.last.total)))
	default:
		line(faint.Render("samples rejudge on :w · :Submit for the full tests"))
	}
	if s.confirm {
		line(red.Render(":q! — this crashes the boot. Q again to confirm."))
	}

	used := strings.Count(b.String(), "\n")
	for i := used; i < h-1; i++ {
		b.WriteString("\n")
	}
	help := "s submit · t statement · ? duck · R retreat · Q :q!"
	if s.upsolve {
		help = "s submit · t statement · e editorial · Q leave"
	}
	b.WriteString(ansi.Truncate(faint.Render(help), w, "…"))
	return b.String()
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func errNo(kind string, id catalog.ID) error {
	return fmt.Errorf("%s has no %s", id, kind)
}

// overlayFlash replaces the last line of body with msg, so errors and notes
// show without resizing the screen.
func overlayFlash(body, msg string, w int) string {
	lines := strings.Split(body, "\n")
	lines[len(lines)-1] = ansi.Truncate(lipgloss.NewStyle().Reverse(true).Render(" "+msg+" "), w, "…")
	return strings.Join(lines, "\n")
}
