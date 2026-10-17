package app

import (
	"cmp"
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
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

type learnLoadedMsg struct {
	cur  *curriculum.Curriculum
	err  error
	open screens.OpenLearn // openLearn's
}

// openLearn shows RTFM's map: at o's module, or with a topic at its
// module nearest the player (learnModule), else on the overworld. The curriculum loads from the
// USACO Guide snapshot the USACO source already downloaded, once per
// session.
func (m *Model) openLearn(o screens.OpenLearn) tea.Cmd {
	if m.cur != nil {
		p := rtfm.NewMap(m.cur, m.meta, m.judgeable, m.d.Catalog.Get)
		if id := cmp.Or(o.Module, m.learnModule(o.Topic)); id != "" {
			p.Open(id)
		}
		if caps := m.freshCaps(); len(caps) > 0 {
			p.Flip(caps)
		}
		cmd := m.push(p)
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
		return learnLoadedMsg{cur: cur, err: err, open: o}
	})
}

// freshCaps is the capabilities granted since RTFM's map last showed,
// for its switch palace, and notes that it now shows them all. The first
// map of a session has none: what was granted before was shown then, on
// the trial's clear screen.
func (m *Model) freshCaps() []string {
	seen := m.rtfmCaps
	m.rtfmCaps = append([]string{}, m.meta.Capabilities...)
	if seen == nil {
		return nil
	}
	var fresh []string
	for _, c := range m.meta.Capabilities {
		if !slices.Contains(seen, c) {
			fresh = append(fresh, c)
		}
	}
	return fresh
}

// openIndex shows RTFM's man index over the map, which searches, at
// module's row when it is set; q there comes back to the map.
func (m *Model) openIndex(module string) tea.Cmd {
	b := rtfm.NewBrowser(m.cur, m.meta, m.judgeable, m.d.Catalog.Get)
	if module != "" {
		b.Select(module)
	}
	return m.push(b)
}

// learnModule is the module RTFM opens at for topic, "" for none: of the
// modules on the topic, those with a trial of their own first (a page
// whose capability is earned elsewhere sent a player who came for a locked
// exit to Bronze's Introduction to Data Structures, a trial of catalog
// filler), the one whose doors' band (the trial's, by division) is nearest
// the player's rating on it, the first in guide order on a tie. Guide order
// alone sent every player on graphs to Bronze's module, a 1850 one too.
func (m *Model) learnModule(topic string) string {
	if topic == "" {
		return ""
	}
	r := m.aptRules()
	rating := game.PlayerRating(&r, &m.meta, topic)
	best, far, trial := "", math.Inf(1), false
	for _, mod := range m.cur.Modules() {
		if mod.Topic != topic || mod.Meta {
			continue
		}
		has := curriculum.TrialFor(mod, m.judgeable).Kind.HasTrial()
		band := curriculum.DivisionBand(mod.Division)
		d := max(float64(band[0])-rating, rating-float64(band[1]), 0)
		if has && !trial || has == trial && d < far {
			best, far, trial = mod.ID, d, has
		}
	}
	return best
}

// trialNext is End.Learn for a passed trial: the next module after its own
// in guide order with a trial still to pass that earns something, a
// capability the player lacks or, granting none, its ✓; or, when none is
// left or the curriculum is not loaded (a trial resumed after a restart),
// the trial's own module, back in RTFM.
func (m *Model) trialNext(st *game.State) *bootui.Learn {
	back := &bootui.Learn{Module: st.Trial.Module}
	if m.cur == nil {
		return back
	}
	mods := m.cur.Modules()
	i := slices.IndexFunc(mods, func(mod curriculum.Module) bool { return mod.ID == st.Trial.Module })
	if i < 0 {
		return back
	}
	for _, mod := range mods[i+1:] {
		c := curriculum.CapabilityFor(mod.Topic)
		if mod.Meta || slices.Contains(m.meta.TrialsPassed, mod.ID) || c != "" && slices.Contains(m.meta.Capabilities, c) {
			continue
		}
		if curriculum.TrialFor(mod, m.judgeable).Kind.HasTrial() {
			return &bootui.Learn{Module: mod.ID, Title: mod.Title}
		}
	}
	return back
}

func (m *Model) onLearnLoaded(msg learnLoadedMsg) tea.Cmd {
	if msg.err != nil {
		return fail("RTFM: %v", msg.err)
	}
	m.cur = msg.cur
	return m.openLearn(msg.open)
}

func (m *Model) judgeable(id catalog.ID) bool {
	p, ok := m.d.Catalog.Get(id)
	return ok && p.Judgeable
}

// openModule shows a module's guide page in the reader, and records it
// opened the first time (Meta.Opened: the map's "opened", never "read").
func (m *Model) openModule(id string) tea.Cmd {
	mod, ok := m.module(id)
	if !ok {
		return fail("no module %q", id)
	}
	md, err := mod.Markdown()
	if err != nil {
		return fail("module %s: %v", id, err)
	}
	if game.MarkOpened(&m.meta, mod.ID) {
		m.checkpointMeta()
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

// startTrial begins a learn-mode trial for a module: any level of the atlas
// but a page, whether or not passing grants a capability. A castle's is its
// division's gauntlet (curriculum.TrialForN).
func (m *Model) startTrial(id string) tea.Cmd {
	mod, ok := m.module(id)
	if !ok {
		return fail("no module %q", id)
	}
	t := curriculum.TrialFor(mod, m.judgeable)
	if !t.Kind.HasTrial() {
		if mod.Division == "general" {
			return fail("%s has no trial: %s is reading only", mod.Title, mod.DivisionTitle)
		}
		if t.Capability != "" {
			if l, _, ok := m.cur.Atlas(m.judgeable).EarnedIn(id); ok {
				return fail("%s has no trial of its own: %s is earned in %s", mod.Title, t.Capability, l.Title)
			}
			return fail("%s has no trial of its own: it lists no problem the game can judge", mod.Title)
		}
		return fail("%s has no trial: it grants no capability and lists no problem the game can judge", mod.Title)
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
