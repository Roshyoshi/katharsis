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
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
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
// door. In an upsolve it shows the problem without a timer or cycles; in
// RTFM practice, with its clock (practiceBar).
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
	// stmt is the door's statement under the pinned judgement (pane.go),
	// nil until its files are in; shown says the last View drew it. tests
	// shows the tests in full in its place (enter), help every key (h).
	stmt               *reader.Model
	shown, tests, help bool
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
	if s.help {
		s.help = false // any key closes it, and does nothing else
		return s, nil
	}
	asked := s.confirm == key
	s.confirm, s.ask = "", ""
	switch key {
	case "h":
		s.help = true
		return s, nil
	case "enter":
		if s.stmt != nil {
			s.tests = !s.tests
			return s, nil
		}
	case "s":
		if ask, ok := s.m.samplesFail("s again submits"); ok && !asked && !s.upsolve {
			s.confirm, s.ask = key, ask
			return s, nil
		}
		return s, screens.Emit(submitMsg{})
	case "t":
		return s, screens.Emit(screens.OpenStatement{Problem: s.door.Problem})
	case "w":
		if s.m.hud() != nil {
			return s, screens.Emit(screens.OpenSamples{})
		}
	case "o":
		return s, s.openPage()
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
		if s.practice() {
			// The clock holds the editorial (the app refuses it too), then
			// asks once: showing it stops the clock.
			id := s.door.Problem
			if left := game.EditorialLeft(&s.m.meta, id); left > 0 {
				return s, note("the editorial opens at %s on the clock: %s to go",
					units.Clock(game.EditorialAfter), units.Countdown(left))
			}
			if c := game.Practice(&s.m.meta, id); c.Editorial == 0 && !game.Solved(&s.m.meta)[id] && !asked {
				s.confirm, s.ask = key, "e again: the editorial stops the clock at "+units.Clock(c.Spent)+
					", red · the solve counts as editorial-assisted"
				return s, nil
			}
		}
		if s.upsolve {
			return s, screens.Emit(screens.OpenEditorial{Problem: s.door.Problem})
		}
	case "R":
		if s.upsolve {
			break
		}
		// Past the door's lock no swap is left, kexec's included: the
		// strip says so (door.retreat.locked) and nothing is stepped.
		if s.m.retreatRefused() {
			return s, nil
		}
		if ask, ok := s.m.retreatAsk(); ok && !asked {
			s.confirm, s.ask = key, ask
			return s, nil
		}
		return s, screens.Emit(game.Retreat{})
	case "Q":
		if s.upsolve && s.solved() {
			return s, screens.Emit(leaveSolvedMsg{})
		}
		if s.upsolve {
			s.m.closeDoor()
			return s, screens.Emit(screens.Back{})
		}
		if !asked {
			s.confirm, s.ask = key, "Q again: "+giveUpPrice(s.m.state)
			return s, nil
		}
		return s, screens.Emit(game.GiveUp{})
	case "q":
		return s, nil // the HUD's q would close the editor; the door decides
	}
	// The statement on screen scrolls; the rest (j/k in the tests, r to
	// rerun the samples) is the HUD's.
	if s.shown && statementKeys[key] {
		s.stmt.Update(msg)
		return s, nil
	}
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
	case s.practice():
		top = append(top, s.practiceBar(p.Title)...)
	case s.upsolve && s.solved():
		top = append(top, ui.Strong.Render("upsolve · "+p.Title)+"  "+ui.OK.Render(ui.G.Pass+" solved")+
			ui.Muted.Render(" · s submits again · Q leaves"))
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
	s.shown = false
	switch hm := s.m.hud(); {
	case s.help && room > 0:
		for _, l := range ui.PadLines(s.helpLines(w), room)[:room] {
			b.WriteString(ui.Clip(l, w) + "\n")
		}
		b.WriteString(ui.Clip(vimHelp, w) + "\n")
	case s.showsStatement(room):
		s.shown = true
		for _, l := range s.statementView(w, room) {
			b.WriteString(ui.Clip(l, w) + "\n")
		}
		b.WriteString(ui.Clip(vimHelp, w) + "\n")
	case hm != nil && room > 1:
		// The HUD sizes itself from WindowSizeMsg; give it the room left.
		if s.hudSize != [2]int{w, room} {
			s.hudSize = [2]int{w, room}
			hm.Update(tea.WindowSizeMsg{Width: w, Height: room})
		}
		// Exactly room lines, the last of them often blank padding: trimming
		// a trailing newline here would drop a row and lift the footer.
		b.WriteString(hm.View().Content + "\n")
		b.WriteString(ui.Clip(vimHelp, w) + "\n")
	default:
		b.WriteString(s.headless(w))
		for i := strings.Count(b.String(), "\n"); i < h-1; i++ {
			b.WriteString("\n")
		}
	}
	b.WriteString(ui.Clip(help, w))
	out := b.String()
	if story, rules, ok := s.m.quitShown(); ok && !s.upsolve {
		out = overlayQuit(out, quitLines(story, rules, w), len(top))
	}
	return out
}

// quitLines are the ctrl+c refusal as the pane draws it, w cells wide:
// the Rabbit's line (the story half), then the ways out with their prices
// in the confirm's tone, nearest the keys.
func quitLines(story, rules string, w int) []string {
	var out []string
	if story != "" {
		for _, l := range ui.WrapN(story, max(w-2, 10), 0) {
			out = append(out, ui.Voice.Render(ui.Muted.Render(" "+l)))
		}
	}
	for _, l := range ui.WrapN(rules, max(w-2, 10), 0) {
		out = append(out, ui.Alert.Render(ui.Fit(" "+l, w)))
	}
	return out
}

// overlayQuit puts the refusal's lines over the pane's body, just above
// its two footer rows, so nothing under it moves; the vitals (the first
// keep lines) stay in view, and what does not fit under them is cut.
func overlayQuit(view string, ls []string, keep int) string {
	lines := strings.Split(view, "\n")
	end := len(lines) - 2 // the editor's commands, then the pane's keys
	start := max(end-len(ls), keep, 0)
	for i := start; i < end; i++ {
		lines[i] = ls[len(ls)-(end-start)+i-start]
	}
	return strings.Join(lines, "\n")
}

// quitShown is the ctrl+c refusal inside a boot door (D47: no quitting
// inside one; hub.go sets Model.doorQuit), its story half (quit.warn) and
// rules half, up until the next key; ok is false when none is.
func (m *Model) quitShown() (story, rules string, ok bool) {
	q := m.doorQuit
	return q.Story, q.Rules, q != (quitRefusal{})
}

// retreatRefused reports whether a retreat of the open door is past its
// lock (rules.RetreatLock), and if so puts the strip's
// door.retreat.locked line up: no swap is left, kexec's included.
func (m *Model) retreatRefused() bool {
	if m.state == nil || !m.inBootDoor() {
		return false
	}
	ev, ok := bootui.RetreatRefused(m.state, m.d.Now())
	if ok {
		m.dmesg.Add(ev)
		m.refresh()
	}
	return ok
}

// giveUpPrice is the Q confirm's word on a :q! (D47): what it burns and
// where the boot goes, or that it ends the boot; in a trial, whether it
// loses the room or ends the trial.
func giveUpPrice(st *game.State) string {
	c, ok := st.FailCost(game.GaveUp)
	boss := st.Biome < len(st.Biomes) && st.Biomes[st.Biome].Kind == rules.Boss
	switch n := st.LostRoomsLeft(); {
	case st.Trial != nil && n > 1:
		return fmt.Sprintf(":q! loses the room · the trial goes on, %s more may go", units.Plural(n-1, "room"))
	case st.Trial != nil && n == 1:
		return ":q! loses the room · the trial goes on, the next :q! ends it"
	case st.Trial != nil:
		return ":q! ends the trial"
	case !ok && st.OneShot() && boss:
		return ":q! ends the boot (one shot)"
	case !ok:
		return ":q! ends the boot"
	case boss:
		return fmt.Sprintf(":q! burns %s, %d left · a new boss door", units.Plural(c, "cycle"), st.Cycles-c)
	case st.Rules.FailMoveOn:
		return fmt.Sprintf(":q! burns %s, %d left · room lost", units.Plural(c, "cycle"), st.Cycles-c)
	}
	return fmt.Sprintf(":q! burns %s, %d left · new doors", units.Plural(c, "cycle"), st.Cycles-c)
}

// openPage opens the problem's page online (o), for what the terminal
// cannot draw: a figure, a matrix, the original typesetting. A mystery
// door's page names it, so the door keeps it to itself until it closes;
// the upsolve after has it.
func (s *problemScreen) openPage() tea.Cmd {
	if s.door.Mystery && !s.upsolve {
		return note("mystery door: its page would name it")
	}
	p, _ := s.m.d.Catalog.Get(s.door.Problem)
	if p.URL == "" {
		return note("%s has no page online", s.door.Problem)
	}
	return screens.Emit(reader.OpenURL{URL: p.URL})
}

// practice reports whether the door on screen is RTFM practice: an
// upsolve on a clock (practice.go).
func (s *problemScreen) practice() bool { return s.upsolve && isPractice(s.door.Problem) }

// practiceBar is an RTFM practice door's vitals: its title, then its clock
// (rtfm.ClockFace) and what the clock says, ticking down to the editorial,
// then where it stopped; once solved, the keys left.
func (s *problemScreen) practiceBar(title string) []string {
	id := s.door.Problem
	c, solved := game.Practice(&s.m.meta, id), game.Solved(&s.m.meta)[id]
	clock := rtfm.ClockFace(c, solved, ui.Token.Render)
	say := s.m.practiceNote(id)
	switch {
	case solved:
		say = ui.OK.Render(ui.G.Pass+" "+say) + ui.Muted.Render(" · s submits again · Q leaves")
	case c.Editorial == 0 && game.EditorialLeft(&s.m.meta, id) == 0:
		say = ui.Muted.Render(say + " · e stops the clock")
	default:
		say = ui.Muted.Render(say)
	}
	return []string{ui.Strong.Render("practice · " + title), strings.TrimLeft(clock+"  "+say, " ")}
}

// editorialKey is e's label in an upsolve: "editorial", or on an RTFM
// practice door whose clock still holds it, the wait: "editorial 07:26".
func (s *problemScreen) editorialKey() string {
	if left := game.EditorialLeft(&s.m.meta, s.door.Problem); s.practice() && left > 0 {
		return "editorial " + units.Countdown(left)
	}
	return "editorial"
}

// solved reports whether the upsolve on screen is accepted and still open
// (Door.Solved).
func (s *problemScreen) solved() bool {
	o := s.m.open
	return s.upsolve && o != nil && o.Upsolve && o.Door.Problem == s.door.Problem && o.Solved
}

// online reports whether o opens a page here.
func (s *problemScreen) online() bool {
	p, _ := s.m.d.Catalog.Get(s.door.Problem)
	return p.URL != "" && (s.upsolve || !s.door.Mystery)
}

// retreatKey is R's label on the keys row: "retreat" until the door's
// lock shuts it (rules.RetreatLock, 2:30 in), then "locked".
func (s *problemScreen) retreatKey() string {
	if st := s.m.state; st != nil {
		if _, locked := bootui.RetreatLocked(st, s.m.d.Now()); locked {
			return "locked"
		}
	}
	return "retreat"
}

// footers are the two key rows (C8): the editor's commands, then this
// pane's keys, only the ones that matter and h for the rest (J17). Under
// 52 columns the editor's are bare names, and the pane's drop Q, whose
// :q! the editor row already has. enter names what it swaps to. o, the
// page on the web, comes after the keys J17 keeps (s, ?, h) and ahead of
// the rest, so at 48 columns it still shows. w, the samples in full,
// comes last, shown when the pane has the room; h lists it.
func (s *problemScreen) footers(w int) (vim, keys string) {
	narrow := w < 52
	var swap []string
	switch {
	case s.stmt == nil:
	case s.tests:
		swap = []string{"enter", "statement"}
	default:
		swap = []string{"enter", "tests"}
	}
	var web []string
	if s.online() {
		web = []string{"o", "web"}
	}
	row := func(ps ...[]string) string {
		out := append([]string{"s", "submit"}, swap...)
		for _, p := range ps {
			out = append(out, p...)
		}
		return ui.Footer(w, append(out, "h", "help")...)
	}
	switch {
	case s.upsolve && narrow:
		return ui.Muted.Render(":w :Submit :Debug n"),
			ui.Footer(w, append(append([]string{"s", "submit", "e", s.editorialKey()}, web...), "Q", "leave", "h", "help")...)
	case s.upsolve:
		return ui.Footer(w, ":w", "samples", ":Submit", "judge", ":Debug n", "debug"),
			row([]string{"e", s.editorialKey()}, web, []string{"Q", "leave", "w", "output"})
	case narrow:
		return ui.Muted.Render(":w :Submit :Debug n :Quack :Retreat :q!"),
			row([]string{"?", "duck"}, web, []string{"R", s.retreatKey(), "w", "output"})
	}
	return ui.Footer(w, ":w", "samples", ":Submit", "judge", ":Debug n", "debug", ":Quack", "duck",
			":Retreat", "retreat", ":q!", "give up"),
		row([]string{"?", "duck"}, web, []string{"R", s.retreatKey(), "Q", "give up", "w", "output"})
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
	var a game.Action
	switch req.Cmd {
	case session.CmdQuack:
		a = game.Quack{}
	case session.CmdGiveUp:
		if slices.Contains(req.Args, session.ArgAsk) {
			// vim's :GiveUp asks what it costs now, then confirms.
			if !m.inBootDoor() || m.attempt() == nil {
				req.Answer(session.Reply{Text: "no door open"})
			} else {
				req.Answer(session.Reply{OK: true, Text: "Give up? " + giveUpPrice(m.state) + "."})
			}
			return nil
		}
		a = game.GiveUp{}
	case session.CmdRetreat:
		if m.retreatRefused() {
			after, _ := bootui.RetreatLocked(m.state, m.d.Now())
			req.Answer(session.Reply{Text: "retreat locked " + units.Clock(after) + " in: solve it, or :q!"})
			return nil
		}
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
	if !m.inBootDoor() {
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
	s, c := m.state, m.attempt()
	switch {
	case m.open != nil && m.open.practice():
		return m.practiceText() + " on the clock"
	case c == nil:
		return "no door open"
	}
	t := units.Countdown(max(c.Deadline.Sub(m.d.Now()), 0)) + " left"
	if bootui.HiddenClock(s) {
		t = "clock hidden" // The Halting Problem's twist (D54)
	}
	return fmt.Sprintf("%s · %s · %s", t, units.Cycles(s.Cycles, s.MaxCycles), ducks(s))
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

// flashRows is the most rows a flash covers.
const flashRows = 3

// overlayFlash puts msg over the last lines of body, wrapped to w, so
// errors and notes show without resizing the screen. rows is how many it
// covers. One clipped row lost the end of the longer ones in a door, where
// the game's pane is the 36-48-column HUD: the quit warning's "ctrl+c
// again to quit".
func overlayFlash(body, msg string, w int) (_ string, rows int) {
	lines := strings.Split(body, "\n")
	ms := ui.WrapN(msg, max(w-2, 1), min(flashRows, len(lines)))
	at := len(lines) - len(ms)
	for i, m := range ms {
		lines[at+i] = ui.Select.Render(ui.Clip(" "+m+" ", w))
	}
	return strings.Join(lines, "\n"), len(ms)
}

// overlayBanner puts msg on the line above the last rows, which the flash
// and the key help keep, so the two never cover each other.
func overlayBanner(body, msg string, w, rows int) string {
	lines := strings.Split(body, "\n")
	i := max(len(lines)-1-max(rows, 1), 0)
	lines[i] = ui.Alert.Render(ui.Clip(" "+msg+" ", w))
	return strings.Join(lines, "\n")
}

// retreatHelp is R in the pane's help (h): what a retreat burns, or that
// kexec swaps free, and how long into a door it stands.
func (s *problemScreen) retreatHelp() string {
	st := s.m.state
	if st == nil {
		return "retreat (asks)"
	}
	t := "retreat for " + units.Plural(st.Rules.RetreatPrice(), "cycle")
	if _, free := st.RetreatCost(); free {
		t = "retreat, free once per " + kexecSpan(st) + " (kexec)"
	}
	if lock := st.Rules.RetreatLock.Duration; lock > 0 {
		t += ", the first " + units.Clock(lock)
	}
	return t + " (asks)"
}
