package app

import (
	"context"
	"os/exec"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/reader"
	"katharsis/internal/store"
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
	case screens.Upsolve:
		return m.startUpsolve(catalog.ID(msg.Problem)), true
	case screens.OpenStatement:
		return m.openDoc(catalog.ID(msg.Problem), false), true
	case screens.OpenEditorial:
		return m.openDoc(catalog.ID(msg.Problem), true), true
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
		Title:   m.title,
		Now:     m.d.Now(),
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
	rows := make([]meta.BootRow, 0, len(boots))
	for _, b := range boots {
		atts, err := m.d.Store.Attempts(store.AttemptFilter{Boot: b.Boot})
		if err != nil {
			return fail("git log: %v", err)
		}
		rows = append(rows, m.bootRow(b, atts))
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

// openLearn is the RTFM entry point.
//
// SPONGE: learn-mode screens and trial boots land in a later slice; until
//
//	then the entry explains itself instead of doing nothing.
func (m *Model) openLearn() tea.Cmd {
	return fail("RTFM (learn mode) is not wired up yet")
}

type docMsg struct{ doc reader.Doc }

// openDoc shows a problem's statement or editorial in the reader, fetching
// it first if needed.
func (m *Model) openDoc(id catalog.ID, editorial bool) tea.Cmd {
	cat := m.d.Catalog
	if m.open != nil && m.open.Door.Problem == id && editorial {
		m.open.ReadEditorial = true
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
			return errMsg{errNo(kind, id)}
		}
		md, err := readFile(path)
		if err != nil {
			return errMsg{err}
		}
		return docMsg{reader.Doc{Title: p.Title, Source: p.Contest + " · " + kind, URL: p.URL, Markdown: md}}
	}
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
	if err := m.d.Store.SaveMeta(m.meta); err != nil {
		return fail("save: %v", err)
	}
	m.closeDoor()
	m.pop()
	m.reset()
	if got > 0 {
		return note("core dump recovered: +%s banked", meta.Bytes(got))
	}
	return nil
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		exec.Command("open", url).Run()
		return nil
	}
}
