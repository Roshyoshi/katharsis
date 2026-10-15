package app

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/reader"
	"katharsis/internal/store"
	"katharsis/internal/units"
)

// intent handles navigation messages; ok is false for anything else.
func (m *Model) intent(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case screens.Quit:
		return m.quit(), true
	case screens.Back:
		m.pop()
		return nil, true
	case screens.StartBoot:
		return m.startBoot(msg), true
	case screens.OpenLog:
		return m.openLog(), true
	case screens.OpenTop:
		return m.openTop(), true
	case screens.OpenLearn:
		return m.openLearn(), true
	case screens.OpenModule:
		return m.openModule(msg.Module), true
	case screens.StartTrial:
		return m.startTrial(msg.Module), true
	case learnLoadedMsg:
		return m.onLearnLoaded(msg), true
	case linkMsg:
		return m.onLink(msg), true
	case screens.Upsolve:
		return m.startUpsolve(msg.Problem), true
	case screens.OpenStatement:
		return m.openDoc(msg.Problem, false), true
	case screens.OpenEditorial:
		return m.openDoc(msg.Problem, true), true
	case reader.OpenURL:
		return openURL(msg.URL), true
	case docMsg:
		return m.push(reader.New(msg.doc)), true
	case submitMsg:
		return m.submit(), true
	}
	return nil, false
}

func (m *Model) grub() screens.Screen {
	var presets []string
	for name := range m.d.Presets {
		if name != "standard" {
			presets = append(presets, name)
		}
	}
	slices.Sort(presets)
	return meta.NewGrub(meta.GrubConfig{
		Meta:    m.meta,
		Presets: presets,
		Lookup:  m.d.Catalog.Get,
		Now:     m.d.Now(),
		Gate:    game.NextPrestige(m.d.Presets["standard"], &m.meta),
	})
}

func (m *Model) title(id catalog.ID) string {
	if p, ok := m.d.Catalog.Get(id); ok {
		return p.Title
	}
	return string(id)
}

func (m *Model) openLog() tea.Cmd {
	boots, err := m.d.Store.Boots(200)
	if err != nil {
		return fail("git log: %v", err)
	}
	// One query for every attempt, not one per boot; grouping keeps each
	// boot's newest first.
	atts, err := m.d.Store.Attempts(store.AttemptFilter{})
	if err != nil {
		return fail("git log: %v", err)
	}
	byBoot := map[int][]store.AttemptRecord{}
	for _, a := range atts {
		byBoot[a.Boot] = append(byBoot[a.Boot], a)
	}
	rows := make([]meta.BootRow, 0, len(boots))
	for _, b := range boots {
		rows = append(rows, m.bootRow(b, byBoot[b.Boot]))
	}
	return m.push(meta.NewLog(rows, nil))
}

func (m *Model) openTop() tea.Cmd {
	st, err := m.d.Store.Stats()
	if err != nil {
		return fail("top: %v", err)
	}
	hist, err := m.d.Store.RatingHistory(time.Time{})
	if err != nil {
		return fail("top: %v", err)
	}
	return m.push(meta.NewTop(m.stats(st, hist)))
}

type docMsg struct{ doc reader.Doc }

// openDoc shows a problem's statement or editorial in the reader, fetching
// it first if needed.
func (m *Model) openDoc(id catalog.ID, editorial bool) tea.Cmd {
	cat := m.d.Catalog
	if editorial {
		m.editorials[id] = true
		if m.open != nil && m.open.Door.Problem == id {
			m.open.ReadEditorial = true
		}
	}
	// A mystery door hides its title on the door and in the directory
	// name; the statement must not give it away either.
	mystery := !editorial && m.state != nil && m.state.Phase == game.PhaseProblem &&
		m.state.Current != nil && m.state.Current.Door.Problem == id && m.state.Current.Door.Mystery
	return m.background(func(ctx context.Context) tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		a, err := cat.Fetch(ctx, id)
		if err != nil {
			return errMsg{err}
		}
		p, _ := cat.Get(id)
		path, kind := a.Statement, "statement"
		if editorial {
			path, kind = a.Editorial, "editorial"
		}
		if path == "" {
			return errMsg{fmt.Errorf("%s has no %s", id, kind)}
		}
		md, err := readFile(path)
		if err != nil {
			return errMsg{err}
		}
		if mystery {
			// The page online names it too, so `o` is off.
			return docMsg{reader.Doc{Title: "???", Source: "mystery door · " + kind, Markdown: redact(md, p)}}
		}
		return docMsg{reader.Doc{Title: p.Title, Source: p.Contest + " · " + kind, URL: p.URL, Markdown: md}}
	})
}

// redact hides a mystery door's identity in its statement: the headings
// and emphasized lines before the first paragraph (USACO statements open
// with "# Title" and "*USACO 2021 January, Bronze, Problem 1*"), and the
// title anywhere else it appears.
func redact(md string, p catalog.Problem) string {
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		emph := len(t) > 1 && strings.ContainsRune("*_", rune(t[0])) && t[len(t)-1] == t[0]
		if !strings.HasPrefix(t, "#") && !emph {
			break // the statement proper
		}
		lines[i] = ""
	}
	md = strings.TrimLeft(strings.Join(lines, "\n"), "\n")
	if p.Title != "" {
		md = strings.ReplaceAll(md, p.Title, "???")
	}
	return md
}

// startUpsolve opens a core dump or regression problem outside a boot:
// untimed, no lives, editorial allowed.
func (m *Model) startUpsolve(id catalog.ID) tea.Cmd {
	p, ok := m.d.Catalog.Get(id)
	if !ok {
		return fail("unknown problem %s", id)
	}
	if m.state != nil && m.state.Phase == game.PhaseProblem {
		return fail("finish or give up the open door first")
	}
	door := game.Door{Problem: id, Topic: p.Topic, Rating: p.Rating}
	return tea.Batch(m.push(newUpsolveScreen(m, door)), m.openDoor(door, true))
}

// sawEditorial reports whether the player has had id's editorial in front
// of them: opened this session, or offered by a :q! on it (the crash screen
// offers it after every :q!) that no solve has come after. The second
// survives a restart, so a core dump upsolved from GRUB the next day still
// counts.
func (m *Model) sawEditorial(id catalog.ID) bool {
	if m.editorials[id] {
		return true
	}
	atts, err := m.d.Store.Attempts(store.AttemptFilter{Problem: id, Limit: 1})
	if err != nil || len(atts) == 0 || atts[0].Outcome != game.GaveUp {
		return false
	}
	return !slices.ContainsFunc(m.meta.Solves, func(s game.Solve) bool {
		return s.Problem == id && s.At.After(atts[0].Closed)
	})
}

// upsolveFiles picks where an upsolve of id is kept and what a new one
// starts from. A due regression is a recall test: it gets a directory per
// due date and a blank file, because the shared upsolve directory may hold
// last time's accepted solution. Any other upsolve shares that directory,
// and a new one starts from the attempt that crashed the boot, so a core
// dump picks up where the boot left off.
func (m *Model) upsolveFiles(id catalog.ID) (sub string, seedBoot int) {
	for _, g := range game.DueRegressions(&m.meta, m.d.Now()) {
		if g.Problem == id {
			return "regression-" + g.Due.Local().Format(time.DateOnly), 0
		}
	}
	switch {
	case m.state != nil && m.state.Crash != nil && m.state.Crash.Door.Problem == id:
		return "", m.state.Boot
	case m.meta.CoreDump != nil && m.meta.CoreDump.Problem == id:
		return "", m.meta.CoreDump.Boot
	}
	return "", 0
}

// onUpsolveJudged records an accepted upsolve; anything else just shows.
func (m *Model) onUpsolveJudged(j judgedMsg) tea.Cmd {
	if s, ok := m.top().(*problemScreen); ok {
		s.last = j
	}
	if j.verdict != "AC" {
		return nil
	}
	kind := game.Independent
	if m.open.ReadEditorial {
		kind = game.Editorial
	}
	id := m.open.Door.Problem
	got := game.Upsolve(&m.meta, id, kind, m.d.Now().Sub(m.open.Opened), m.d.Now())
	// The solve is recorded either way: unsaved, it goes with the next
	// save, and a door left open would let it count twice.
	m.closeDoor()
	m.reset()
	if !m.saved(m.d.Store.SaveMeta(m.meta)) {
		return nil // the banner says so
	}
	if got > 0 {
		return note("core dump recovered: +%s banked", units.Bytes(got))
	}
	return nil
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("open", url).Run() // no browser: the key does nothing
		return nil
	}
}
