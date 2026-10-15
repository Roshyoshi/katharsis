package app

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
	"katharsis/internal/curriculum"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/libs"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

type learnLoadedMsg struct {
	cur   *curriculum.Curriculum
	err   error
	topic string // openLearn's
}

// openLearn shows the RTFM browser, with a topic at its module nearest the
// player (learnModule). The curriculum loads from the USACO Guide snapshot
// the USACO source already downloaded, once per session.
func (m *Model) openLearn(topic string) tea.Cmd {
	if m.cur != nil {
		b := rtfm.NewBrowser(m.cur, m.meta, m.judgeable, m.d.Catalog.Get)
		if id := m.learnModule(topic); id != "" {
			b.Open(id)
		}
		cmd := m.push(b)
		// The Mock Turtle's school, over the first visit (story bible).
		if m.d.Story && m.unseen("story.first-rtfm") {
			cmd = tea.Batch(cmd, m.readBeat("story.first-rtfm"))
		}
		return cmd
	}
	return m.background(func(ctx context.Context) tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		dir, err := guide.Snapshot(ctx)
		if err != nil {
			return learnLoadedMsg{err: err}
		}
		cur, err := curriculum.Load(dir)
		return learnLoadedMsg{cur: cur, err: err, topic: topic}
	})
}

// learnModule is the module RTFM opens at for topic, "" for none: of the
// modules on the topic, whose trials all earn its capability, the one
// whose doors' band (the trial's, by division) is nearest the player's
// rating on it, the first in guide order on a tie. Guide order alone sent
// every player on graphs to Bronze's module, a 1850 one too.
func (m *Model) learnModule(topic string) string {
	if topic == "" {
		return ""
	}
	r := m.aptRules()
	rating := game.PlayerRating(&r, &m.meta, topic)
	best, far := "", math.Inf(1)
	for _, mod := range m.cur.Modules() {
		if mod.Topic != topic || mod.Meta {
			continue
		}
		band := curriculum.DivisionBand(mod.Division)
		if d := max(float64(band[0])-rating, rating-float64(band[1]), 0); d < far {
			best, far = mod.ID, d
		}
	}
	return best
}

func (m *Model) onLearnLoaded(msg learnLoadedMsg) tea.Cmd {
	if msg.err != nil {
		return fail("RTFM: %v", msg.err)
	}
	m.cur = msg.cur
	return m.openLearn(msg.topic)
}

func (m *Model) judgeable(id catalog.ID) bool {
	p, ok := m.d.Catalog.Get(id)
	return ok && p.Judgeable
}

// openModule shows a module's guide page in the reader.
func (m *Model) openModule(id string) tea.Cmd {
	mod, ok := m.module(id)
	if !ok {
		return fail("no module %q", id)
	}
	md, err := mod.Markdown()
	if err != nil {
		return fail("module %s: %v", id, err)
	}
	return m.push(reader.New(reader.Doc{
		Title:     mod.Title,
		Source:    "USACO Guide · " + mod.DivisionTitle,
		URL:       mod.URL,
		Markdown:  md,
		Converted: true, // the curriculum did its math
	}))
}

func (m *Model) module(id string) (curriculum.Module, bool) {
	if m.cur == nil {
		return curriculum.Module{}, false
	}
	return m.cur.Module(id)
}

// startTrial begins a learn-mode trial for a module.
func (m *Model) startTrial(id string) tea.Cmd {
	mod, ok := m.module(id)
	if !ok {
		return fail("no module %q", id)
	}
	t := curriculum.TrialFor(mod, m.judgeable)
	if t.Capability == "" {
		return fail("%s has no trial: it teaches no gated topic", mod.Title)
	}
	return m.beginTrial(t.Spec())
}

// beginTrial starts the trial t, in the Mock Turtle's words
// (rtfm.trial.start). Like a boot, it gives up a boot still running.
func (m *Model) beginTrial(t game.TrialSpec) tea.Cmd {
	if err := m.abandon(); err != nil {
		return fail("abandon boot %d: %v", m.state.Boot, err)
	}
	m.pool = m.d.Catalog.All() // as in startBoot
	st, effs, err := game.NewTrial(rules.Trial(), &m.meta, m.pool, t, m.d.Seed(), m.d.Now())
	if err != nil {
		return fail("trial: %v", err)
	}
	return tea.Batch(m.begin(st, effs), note("%s", flavor.Story("rtfm.trial.start", 0)))
}

// sourceRunner is a Runner that can name the open door's source file, so
// accepted code can be mined for libs.
type sourceRunner interface{ Source() string }

// harvestLibs saves the libs marked in the open door's source. It runs on
// an AC only, so every lib in the bank is code that passed a full test
// set. A malformed marker is reported, never fatal.
func (m *Model) harvestLibs() tea.Cmd {
	r, ok := m.d.Runner.(sourceRunner)
	if !ok || r.Source() == "" {
		return nil
	}
	src, err := readFile(r.Source())
	if err != nil {
		return nil
	}
	found, err := libs.Extract(src)
	if err != nil {
		return note("libs: %v", err)
	}
	if len(found) == 0 {
		return nil
	}
	var names []string
	for _, l := range found {
		game.AddLib(&m.meta, l)
		names = append(names, l.Name)
	}
	return note("lib saved: %s", strings.Join(names, ", "))
}

// linkScreen picks the libs a boot links, before its first door. Space
// toggles, enter links the selection, esc links nothing.
type linkScreen struct {
	all   []game.Lib
	slots int
	on    []bool
	cur   int
}

// linkMsg leaves the link screen with the chosen names.
type linkMsg struct{ names []string }

func newLinkScreen(all []game.Lib, slots int) *linkScreen {
	return &linkScreen{all: all, slots: slots, on: make([]bool, len(all))}
}

func (s *linkScreen) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch k.String() {
	case "j", "down":
		s.cur = min(s.cur+1, len(s.all)-1)
	case "k", "up":
		s.cur = max(s.cur-1, 0)
	case "space":
		if s.on[s.cur] || s.count() < s.slots {
			s.on[s.cur] = !s.on[s.cur]
		}
	case "enter":
		var names []string
		for i, l := range s.all {
			if s.on[i] {
				names = append(names, l.Name)
			}
		}
		return s, screens.Emit(linkMsg{names})
	case "esc", "q":
		return s, screens.Emit(linkMsg{})
	}
	return s, nil
}

func (s *linkScreen) count() int {
	n := 0
	for _, on := range s.on {
		if on {
			n++
		}
	}
	return n
}

func (s *linkScreen) View(w, h int) string {
	var b strings.Builder
	b.WriteString(ui.Strong.Render("ld: link libs into this boot") + ui.Muted.Render(fmt.Sprintf("  %d/%d slots", s.count(), s.slots)) + "\n\n")
	for i, l := range s.all {
		// [x] is a checkbox, not a capability: it stays bracketed.
		mark := "[ ]"
		if s.on[i] {
			mark = "[x]"
		}
		cursor := "  "
		if i == s.cur {
			cursor = ui.Focus.Render(ui.G.Cursor) + " "
		}
		lines := strings.Count(strings.TrimRight(l.Source, "\n"), "\n") + 1
		fmt.Fprintf(&b, "%s%s %-20s %s\n", cursor, mark, l.Name, ui.Muted.Render(units.Plural(lines, "line")))
	}
	b.WriteString("\n" + ui.Muted.Render("linked libs are pasted above main in each new door's source"))
	used := strings.Count(b.String(), "\n")
	for i := used; i < h-1; i++ {
		b.WriteString("\n")
	}
	b.WriteString(ui.Footer(w, "space", "toggle", "enter", "link", "esc", "none"))
	return b.String()
}

func (m *Model) onLink(msg linkMsg) tea.Cmd {
	m.pop()
	if len(msg.names) == 0 {
		return nil
	}
	return m.step(game.Link{Libs: msg.names})
}

// offerLink opens the link screen when the bank holds libs and the boot has
// not reached a door yet.
func (m *Model) offerLink() tea.Cmd {
	if len(m.meta.Libs) == 0 || m.state == nil || m.state.Trial != nil || len(m.state.History) > 0 {
		return nil
	}
	all := slices.Clone(m.meta.Libs)
	return m.push(newLinkScreen(all, m.state.LinkSlots()))
}
