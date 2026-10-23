package app

import (
	"strings"

	"katharsis/internal/catalog"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/reader"
	"katharsis/internal/ui"
)

// The problem pane reads statement first (UX J8): under the vitals the
// judgement is pinned in two lines (the HP bar, the samples with their
// first failure folded in), and the statement fills the rest and scrolls,
// so reading it never hides the judge. enter swaps the statement for the
// tests in full and back; t opens it full screen, with search. h lists
// every key, so the keys row shows only the ones that matter (J17).

// minStatement is the fewest rows the statement view needs under the
// pinned judgement: its title, a rule and a few lines of text. A shorter
// pane shows the tests, as before.
const minStatement = 8

// statementKeys scroll the statement in the pane; the rest stay the pane's.
var statementKeys = map[string]bool{
	"j": true, "k": true, "up": true, "down": true, "space": true, "b": true, "f": true,
	"pgup": true, "pgdown": true, "u": true, "d": true, "g": true, "G": true, "home": true, "end": true,
}

// Refresh is the app handing the screens the new state: the door's files
// may just have arrived, statement included.
func (s *problemScreen) Refresh(bootui.Env) { s.loadStatement() }

// loadStatement reads the open door's statement once it is on disk. A
// mystery door's is redacted, as the full-screen reader's is.
func (s *problemScreen) loadStatement() {
	o := s.m.open
	if s.stmt != nil || o == nil || o.Door.Problem != s.door.Problem || o.Upsolve != s.upsolve || o.Assets.Statement == "" {
		return
	}
	md, err := readFile(o.Assets.Statement)
	if err != nil {
		return // the tests view stays; t says why
	}
	s.stmt = reader.New(statementDoc(o.Problem, md, s.door.Mystery && !s.upsolve))
}

func statementDoc(p catalog.Problem, md string, mystery bool) reader.Doc {
	if mystery {
		return reader.Doc{Title: "???", Source: "mystery door", Markdown: redact(md, p)}
	}
	return reader.Doc{Title: p.Title, Source: p.Contest, Markdown: md}
}

// showsStatement reports whether a pane room rows tall below the vitals
// shows the statement: one has loaded, the tests or the help are not up,
// and it fits.
func (s *problemScreen) showsStatement(room int) bool {
	return s.stmt != nil && !s.tests && !s.help && room >= 2+minStatement
}

// statementView is the pinned judgement then the statement, exactly room
// lines of width w.
func (s *problemScreen) statementView(w, room int) []string {
	var pinned []string
	if h := s.m.hud(); h != nil {
		pinned = h.Pinned(w)
	} else {
		pinned = strings.Split(strings.TrimSuffix(s.headless(w), "\n"), "\n")
	}
	// The reader's own last row is its pager bar; the pane has its keys.
	v := strings.Split(s.stmt.View(w, room-len(pinned)+1), "\n")
	return append(pinned, v[:len(v)-1]...)
}

// helpLines is every key of the pane, for h.
func (s *problemScreen) helpLines(w int) []string {
	pairs := [][2]string{
		{"s", "submit (asks if samples fail)"},
		{"enter", "tests in full / statement"},
		{"j/k", "scroll · move in the tests"},
		{"space/b", "page the statement"},
		{"t", "statement full screen, /search"},
		{"r", "judge the samples again"},
		{"w", "the samples in full: output, expected, stderr"},
	}
	if s.online() {
		pairs = append(pairs, [2]string{"o", "the page on the web: figures, layout"})
	}
	if s.upsolve {
		pairs = append(pairs, [2]string{"e", "the editorial: marks the solve editorial-assisted"},
			[2]string{"Q", "leave the upsolve (solved, it stays open until then)"})
	} else {
		pairs = append(pairs,
			[2]string{"?", "duck: tags, then technique"},
			[2]string{"R", s.retreatHelp()},
			[2]string{"e", "reopen the editor"},
			[2]string{"Q", "give up, :q! (asks)"})
	}
	out := []string{ui.Strong.Render("keys") + ui.Muted.Render(ui.G.Sep+"any key closes this")}
	for _, p := range pairs {
		out = append(out, ui.Clip("  "+ui.Strong.Render(ui.Fit(p[0], 8))+ui.Muted.Render(p[1]), w))
	}
	out = append(out, "", ui.Strong.Render("vim"))
	for _, l := range ui.WrapN(":w samples · :Case adds your own · :Submit judges, :Submit! over failing samples · "+
		":Debug n · :Quack · :Retreat · :q! gives up", max(w-2, 10), 0) {
		out = append(out, "  "+ui.Muted.Render(l))
	}
	// Where the notes beside the source are, and when (editorialWhere).
	folder := "statement.md, under the problem's page · editorial.md once you solve the door"
	if s.upsolve {
		folder = "statement.md, under the problem's page · editorial.md, from the start: " +
			"read there it is not counted, e here marks the solve editorial-assisted"
	}
	out = append(out, "", ui.Strong.Render("folder"))
	for _, l := range ui.WrapN(folder, max(w-2, 10), 0) {
		out = append(out, "  "+ui.Muted.Render(l))
	}
	return out
}
