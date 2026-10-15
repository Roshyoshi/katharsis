package app

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/session"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// submitMsg asks the app to judge the open door's full tests. The problem
// screen's key and the editor's :Submit both lead here.
type submitMsg struct{}

// hudRunner is a Runner with a HUD to embed (the tmux runner). The
// headless tests' runner has none, and the app judges through Deps.Judge.
type hudRunner interface {
	HUD() *hud.Model
	Reopen() error
}

// hud is the open door's HUD, nil between doors and without a hudRunner.
func (m *Model) hud() *hud.Model {
	if r, ok := m.d.Runner.(hudRunner); ok {
		return r.HUD()
	}
	return nil
}

// hudQuitMsg replaces the HUD's tea.Quit: embedded in the game, the HUD
// quitting means the player closed the editor, not that the game ends.
type hudQuitMsg struct{}

// guard rewrites a HUD command so its quit cannot stop the program.
func guard(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		switch msg := cmd().(type) {
		case tea.QuitMsg:
			return hudQuitMsg{}
		case tea.BatchMsg:
			out := make(tea.BatchMsg, len(msg))
			for i, c := range msg {
				out[i] = guard(c)
			}
			return out
		default:
			return msg
		}
	}
}

// problemScreen is the pane beside the editor while a door is open: the
// boot HUD bar on top, the sample/submit HUD below, and the keys for the
// door. In an upsolve it shows the problem without a timer or lives.
type problemScreen struct {
	m       *Model
	upsolve bool
	door    game.Door
	last    judgedMsg // the headless judge's last result
	// confirm is the key that asked once, its question on the keys' row:
	// the same key again goes ahead, any other lets it go. Q gives up, R
	// retreats, s submits over failing samples (J3, J4).
	confirm, ask string
	closed       bool // the player closed the editor
	// editorErr, with closed, is why the editor could not start.
	editorErr string
	hudSize   [2]int
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
	asked := s.confirm == key
	s.confirm, s.ask = "", ""
	switch key {
	case "s":
		if ask, ok := s.m.samplesFail("s again submits"); ok && !asked && !s.upsolve {
			s.confirm, s.ask = key, ask
			return s, nil
		}
		return s, screens.Emit(submitMsg{})
	case "t":
		return s, screens.Emit(screens.OpenStatement{Problem: s.door.Problem})
	case "?":
		if !s.upsolve {
			return s, screens.Emit(game.Quack{})
		}
	case "e":
		if r, ok := s.m.d.Runner.(hudRunner); ok && s.closed {
			if err := r.Reopen(); err != nil {
				return s, fail("reopen editor: %v", err)
			}
			s.closed = false
			return s, guard(r.HUD().Init())
		}
		if s.m.open == nil && !s.upsolve {
			// A resumed door whose fetch failed (offline, say) has no
			// editor; without a retry it could only time out.
			return s, s.m.openDoor(s.door, false)
		}
		if s.upsolve {
			return s, screens.Emit(screens.OpenEditorial{Problem: s.door.Problem})
		}
	case "R":
		if s.upsolve {
			break
		}
		if ask, ok := s.m.retreatAsk(); ok && !asked {
			s.confirm, s.ask = key, ask
			return s, nil
		}
		return s, screens.Emit(game.Retreat{})
	case "Q":
		if s.upsolve {
			s.m.closeDoor()
			return s, screens.Emit(screens.Back{})
		}
		if !asked {
			s.confirm, s.ask = key, "Q again: :q! crashes the boot"
			return s, nil
		}
		return s, screens.Emit(game.GiveUp{})
	case "q":
		return s, nil // the HUD's q would close the editor; the door decides
	}
	// The rest (j/k, r to rerun samples) is the HUD's.
	if h := s.m.hud(); h != nil {
		_, cmd := h.Update(msg)
		return s, guard(cmd)
	}
	return s, nil
}

func (s *problemScreen) View(w, h int) string {
	var top []string
	p, _ := s.m.d.Catalog.Get(s.door.Problem)
	switch {
	case s.upsolve:
		top = append(top, ui.Strong.Render("upsolve · "+p.Title)+ui.Muted.Render("  untimed, editorial allowed"))
	case s.m.state != nil:
		top = append(top, strings.Split(bootui.HUDBar(s.m.state, p, s.m.d.Now(), w), "\n")...)
		// The strip's newest line on this door: its cost, its flavor (J5).
		top = append(top, s.m.env().DoorLines(w, 3)...)
	}
	switch {
	case s.closed && s.editorErr != "":
		// Wrapped, not cut: the command is the point.
		for _, l := range strings.Split(ansi.Wrap(s.editorErr+" · e retries", max(w, 10), ""), "\n") {
			top = append(top, ui.Danger.Render(l))
		}
	case s.closed:
		top = append(top, ui.Danger.Render("editor closed · e reopens it"))
	}
	// The vitals end in a rule; the judgement starts under it (C8).
	top = append(top, ui.Rule(w))
	vimHelp, help := s.footers(w)
	if s.confirm != "" {
		// The confirm takes the keys' row, so the pane does not move.
		help = ui.Alert.Render(ui.Fit(" "+s.ask, w))
	}

	var b strings.Builder
	for _, l := range top {
		b.WriteString(ui.Clip(l, w) + "\n")
	}
	// Two footer rows: the editor's commands, then this pane's keys. The
	// HUD runs embedded (no header or key help of its own) and is exactly
	// as tall as the room it is given.
	room := h - len(top) - 2
	if hm := s.m.hud(); hm != nil && room > 1 {
		// The HUD sizes itself from WindowSizeMsg; give it the room left.
		if s.hudSize != [2]int{w, room} {
			s.hudSize = [2]int{w, room}
			hm.Update(tea.WindowSizeMsg{Width: w, Height: room})
		}
		// Exactly room lines, the last of them often blank padding: trimming
		// a trailing newline here would drop a row and lift the footer.
		b.WriteString(hm.View().Content + "\n")
		b.WriteString(ui.Clip(vimHelp, w) + "\n")
	} else {
		b.WriteString(s.headless(w))
		for i := strings.Count(b.String(), "\n"); i < h-1; i++ {
			b.WriteString("\n")
		}
	}
	b.WriteString(ui.Clip(help, w))
	return b.String()
}

// footers are the two key rows (C8): the editor's commands, then this
// pane's keys. Under 52 columns the editor's are bare names, and the
// pane's drop Q, whose :q! the editor row already has.
func (s *problemScreen) footers(w int) (vim, keys string) {
	narrow := w < 52
	switch {
	case s.upsolve && narrow:
		return ui.Muted.Render(":w :Submit :Debug n"),
			ui.Footer(w, "s", "submit", "t", "stmt", "e", "editorial", "Q", "leave")
	case s.upsolve:
		return ui.Footer(w, ":w", "samples", ":Submit", "judge", ":Debug n", "debug"),
			ui.Footer(w, "s", "submit", "t", "statement", "e", "editorial", "r", "rerun", "j/k", "move", "Q", "leave")
	case narrow:
		return ui.Muted.Render(":w :Submit :Debug n :Quack :Retreat :q!"),
			ui.Footer(w, "s", "submit", "t", "stmt", "?", "duck", "R", "retreat")
	}
	return ui.Footer(w, ":w", "samples", ":Submit", "judge", ":Debug n", "debug", ":Quack", "duck",
			":Retreat", "retreat", ":q!", "give up"),
		ui.Footer(w, "s", "submit", "t", "statement", "?", "duck", "R", "retreat", "r", "rerun",
			"j/k", "move", "Q", "give up")
}

// headless is the body without a HUD: the pane's one HP bar (C6), the
// door's best full-test submit, or in an upsolve the last one judged here,
// over what the pane is waiting for.
func (s *problemScreen) headless(w int) string {
	passed, total, verdict := s.last.passed, s.last.total, s.last.verdict
	if c := s.m.state; !s.upsolve && c != nil && c.Current != nil {
		passed, total = c.Current.BestPass, c.Current.Total
		if verdict == "" {
			verdict = string(c.Current.LastVerdict)
		}
	}
	note := ""
	switch {
	case s.m.judging:
		note = ui.Muted.Render("judging full tests…")
	case verdict != "":
		note = ui.Verdict(verdict).Render(verdict)
	}
	line := "samples rejudge on :w · :Submit for the full tests"
	if s.m.open == nil {
		line = "fetching " + string(s.door.Problem) + "…"
		if !s.upsolve {
			line += " · e retries" // in an upsolve e is the editorial
		}
	}
	return ui.HP(w, passed, total, note) + "\n" + ui.Clip(ui.Muted.Render(line), w) + "\n"
}

// onRequest handles a command from the editor. The game's own commands
// become engine actions; the HUD judges a submit the game starts
// (startSubmit), and debug belongs to it.
func (m *Model) onRequest(req session.Request) tea.Cmd {
	inBoot := m.state != nil && m.state.Phase == game.PhaseProblem && m.open != nil && !m.open.Upsolve
	var a game.Action
	switch req.Cmd {
	case session.CmdQuack:
		a = game.Quack{}
	case session.CmdGiveUp:
		a = game.GiveUp{}
	case session.CmdRetreat:
		a = game.Retreat{}
	case session.CmdStatus:
		req.Answer(session.Reply{OK: true, Text: m.statusLine()})
		return nil
	default:
		h := m.hud()
		switch {
		case h == nil || m.open == nil:
			req.Answer(session.Reply{Text: "no door open"})
			return nil
		case req.Cmd == session.CmdSubmit && !slices.Contains(req.Args, session.ArgForce):
			// Failing samples disprove the submit before it is judged: a
			// misinput, not a gamble, so it asks for :Submit! (J3).
			if ask, ok := m.samplesFail(":Submit! submits anyway"); ok {
				req.Answer(session.Reply{Text: ask})
				return nil
			}
			return m.startSubmit(req)
		case req.Cmd == session.CmdSubmit:
			// The submit most players make, so it must hold the clock
			// for the grace, and be noted, the way the s key's is.
			return m.startSubmit(req)
		}
		_, cmd := h.Update(req)
		return guard(cmd)
	}
	if !inBoot {
		req.Answer(session.Reply{Text: req.Cmd + " needs an open door in a boot"})
		return nil
	}
	hint := m.state.Current.Hint
	cmd, err := m.try(a)
	if err != nil {
		m.refused(a)
		req.Answer(session.Reply{Text: refusal(err)})
		return nil
	}
	text := m.statusLine()
	if c := m.state.Current; c != nil && c.Hint > hint {
		text = m.reveal(c.Hint) + " · " + text
	}
	req.Answer(session.Reply{OK: true, Text: text})
	return cmd
}

// reveal is what a quack to level showed, for the editor's message line:
// the player asked from vim, so the answer belongs there and not only in
// the HUD.
func (m *Model) reveal(level int) string {
	p, _ := m.d.Catalog.Get(m.state.Current.Door.Problem)
	if level >= 2 {
		return "technique: " + cmp.Or(p.Topic, "unknown")
	}
	if len(p.Tags) == 0 {
		return "tags: none listed"
	}
	return "tags: " + strings.Join(p.Tags, " · ")
}

// statusLine is the one-line boot state for the editor's message line:
// countdown, lives, ducks. It has no ⏱: vim counts that glyph two columns
// wide and tmux draws it in one, so vim leaves a stale character after it.
func (m *Model) statusLine() string {
	s := m.state
	if s == nil || s.Phase != game.PhaseProblem || s.Current == nil {
		return "no door open"
	}
	left := max(s.Current.Deadline.Sub(m.d.Now()), 0)
	return fmt.Sprintf("%s left · %s · %s", units.Countdown(left), units.Hearts(s.Lives, s.MaxLives), ducks(s))
}

// ducks is the duck count, or ✗ on a cursed door, which takes none.
func ducks(s *game.State) string {
	if s.Current != nil && s.Current.Door.Cursed {
		return ui.G.Duck + ui.G.Fail
	}
	return fmt.Sprintf("%s%d", ui.G.Duck, s.Ducks)
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// overlayFlash replaces the last line of body with msg, so errors and notes
// show without resizing the screen.
func overlayFlash(body, msg string, w int) string {
	lines := strings.Split(body, "\n")
	lines[len(lines)-1] = ui.Select.Render(ui.Clip(" "+msg+" ", w))
	return strings.Join(lines, "\n")
}

// overlayBanner puts msg on the line above the last, which the flash and
// the key help keep, so the two never cover each other.
func overlayBanner(body, msg string, w int) string {
	lines := strings.Split(body, "\n")
	i := max(len(lines)-2, 0)
	lines[i] = ui.Alert.Render(ui.Clip(" "+msg+" ", w))
	return strings.Join(lines, "\n")
}
