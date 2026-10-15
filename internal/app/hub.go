package app

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
	"katharsis/internal/curriculum"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/screens/story"
	"katharsis/internal/units"
)

// The map as the hub (D47, meta.md §(c)): a boot is suspended between
// doors only, saved after every step as ever, and the hub is where it is
// resumed, ended or left.
//
//	launch ─┬─ no boot ───────────> GRUB, the title over it (D51)
//	        ├─ a door open ───────> the door, its deadline kept (D31)
//	        └─ between doors ─────> the hub, over the phase's screen
//	                                  enter resume · n abandon → GRUB ·
//	                                  u upsolve · t RTFM · q quit
//	between doors   m → the hub · q, ctrl+c → saved, suspended, quit
//	inside a door   ctrl+c → refused (doorQuit): solve, retreat or :q!
//
// The title does not show before the hub: D51 plays it only when the
// launch lands on GRUB, and the hub's own line (map.resume) greets a
// resumed boot. One boot is in flight at a time, so a boot or a trial
// started over a suspended one asks first, on the hub, and ends it as a
// crash (game.Abandon).

// quitRefusal is what a ctrl+c inside a door gets in place of a quit: the
// story's half (quit.warn, "" past a soft deadline) and the rules' half.
// The problem pane draws it (problem.go) until the next key clears it.
type quitRefusal struct{ Story, Rules string }

// suspended reports whether a boot is in flight between doors.
func (m *Model) suspended() bool { return m.state != nil && m.state.Suspendable() }

// launchAt is Init's screen for a launch between doors: the hub, over the
// screen the boot's phase shows. Any other launch gets the scenes it cut
// short (storyResume). The hub's resume plays them instead.
func (m *Model) launchAt() tea.Cmd {
	if !m.suspended() {
		return m.storyResume()
	}
	return tea.Batch(m.push(bootui.NewHub(m.env(), true)), m.preloadRTFM())
}

// hubKey takes the keys every boot screen between doors shares: m opens the
// hub, q quits, the boot saved and suspended. Only over the phase's own
// screen, so a reader or a scene keeps its keys; ok is false for any other
// key or screen.
func (m *Model) hubKey(k tea.KeyPressMsg) (tea.Cmd, bool) {
	if !m.suspended() || len(m.stack) != 1 {
		return nil, false
	}
	switch k.String() {
	case "m":
		return m.push(bootui.NewHub(m.env(), false)), true
	case "q":
		return m.quit(), true
	}
	return nil, false
}

// ctrlC quits, but never from inside a boot's door (D47): the deadline is
// absolute (D31), so a quit there would only look like a pause. The
// refusal names the ways out instead. An upsolve is untimed and quits.
func (m *Model) ctrlC() tea.Cmd {
	if m.attempt() != nil {
		m.doorQuit = m.doorQuitText()
		return nil
	}
	return m.quit()
}

// doorQuitText is the refusal for the open door, in its prices: "no
// quitting inside a door: solve · R retreat −5 · :q! give up −10 ·
// closing the terminal does not stop the clock". Retreat drops out once
// it is locked or would leave no cycles; a :q! that would end the boot
// says so.
func (m *Model) doorQuitText() quitRefusal {
	st, now := m.state, m.d.Now()
	var q quitRefusal
	if left, ok := m.doorLeft(); ok && left > 0 {
		if s := flavor.Story("quit.warn", 0); s != "" {
			t := units.Countdown(left)
			if bootui.HiddenClock(st) {
				t = bootui.ClockHidden // The Halting Problem hides it (D54)
			}
			q.Story = flavor.Fill(s, "time", t)
		}
	}
	parts := []string{"no quitting inside a door: solve"}
	if lock := st.RetreatLockAt(); lock.IsZero() || now.Before(lock) {
		switch c, free := st.RetreatCost(); {
		case free:
			parts = append(parts, "R retreat free (kexec)")
		case c < st.Cycles:
			parts = append(parts, fmt.Sprintf("R retreat −%d", c))
		}
	}
	switch c, ok := st.FailCost(game.GaveUp); {
	case st.Trial != nil:
		parts = append(parts, ":q! give up")
	case ok:
		parts = append(parts, fmt.Sprintf(":q! give up −%d", c))
	default:
		parts = append(parts, ":q! ends the boot")
	}
	q.Rules = strings.Join(append(parts, "closing the terminal does not stop the clock"), " · ")
	return q
}

// hubIntent handles the hub's intents, and a boot or trial asked to start
// over a suspended boot; ok is false for anything else.
func (m *Model) hubIntent(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case bootui.Resume:
		return m.resume(msg.Launch), true
	case bootui.Abandon:
		return m.abandonThen(msg.Then), true
	case rtfmLoadedMsg:
		return m.onRTFMLoaded(msg), true
	case screens.StartBoot, screens.StartTrial:
		if m.suspended() {
			return m.askAbandon(msg), true
		}
	}
	return nil, false
}

// resume leaves the hub for where the boot stopped. From a launch it says
// so (map.resume) and plays the scenes the last game cut short.
func (m *Model) resume(launch bool) tea.Cmd {
	if _, ok := m.top().(*bootui.Hub); ok {
		m.pop()
	}
	if !launch || m.state == nil {
		return nil
	}
	if s := flavor.Story("map.resume", 0); s != "" {
		m.flash = flavor.Fill(s, "boot", strconv.Itoa(m.state.Boot))
	}
	return m.storyResume()
}

// askAbandon shows the hub asking before next (a StartBoot or StartTrial)
// ends the boot in flight: the hub already on the stack, uncovered, or a
// new one.
func (m *Model) askAbandon(next tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	i := slices.IndexFunc(m.stack, func(s screens.Screen) bool { _, ok := s.(*bootui.Hub); return ok })
	if i < 0 {
		cmd = m.push(bootui.NewHub(m.env(), false))
		i = len(m.stack) - 1
	}
	for len(m.stack) > i+1 {
		m.pop()
	}
	m.stack[i].(*bootui.Hub).Ask(next)
	return cmd
}

// abandonThen ends the boot in flight as a crash and goes on to then: the
// boot or trial it was asked for, else GRUB, where the next boot starts.
func (m *Model) abandonThen(then tea.Msg) tea.Cmd {
	if err := m.abandon(); err != nil {
		return fail("abandon boot %d: %v", m.state.Boot, err)
	}
	if then != nil {
		if cmd, ok := m.intent(then); ok {
			return cmd
		}
	}
	return m.toGrub()
}

// onOffer reports whether problem id is a door on offer in the boot in
// flight: upsolving it would solve that door before it opens (meta.md
// §(c)).
func (m *Model) onOffer(id catalog.ID) bool {
	return m.inBoot() && slices.ContainsFunc(m.state.Doors, func(d game.Door) bool { return d.Problem == id })
}

// rtfmModule is Env.Module: the title of the RTFM module whose trial
// opens topic's capability for the player, "" until RTFM is loaded.
func (m *Model) rtfmModule(topic string) string {
	if m.cur == nil {
		return ""
	}
	mod, _ := m.module(m.learnModule(topic))
	return mod.Title
}

// Farewell is the line the shell gets after the game quit with a boot
// suspended between doors (map.suspend), "" otherwise.
func (m *Model) Farewell() string {
	if !m.suspended() {
		return ""
	}
	return flavor.Story("map.suspend", 0)
}

// mapScenes are the one-time scenes a map boot owes where it stands
// (story.first-act2, story.first-act3, story.first-skip, story.first-locked;
// map.md §(g)): acts II and III at their first route or doors, the skip
// once taken, and the first exit a capability locks, never a prestige's.
func (m *Model) mapScenes() []queued {
	s := m.state
	if len(s.Rules.Nodes) == 0 || len(s.Biomes) != len(s.Rules.Biomes) || s.Biome >= len(s.Biomes) {
		return nil
	}
	if s.Phase != game.PhaseRoute && s.Phase != game.PhaseDoors {
		return nil
	}
	var ids []string
	switch act := s.ActOf(s.Biome); {
	case act >= 2:
		ids = append(ids, "story.first-act3")
	case act == 1:
		ids = append(ids, "story.first-act2")
	}
	if slices.ContainsFunc(s.Biomes[:s.Biome], func(b game.BiomeState) bool { return b.Node != "" && b.Rooms == 0 }) {
		ids = append(ids, "story.first-skip")
	}
	if s.Phase == game.PhaseRoute && slices.ContainsFunc(s.Biomes[s.Biome].Exits, func(e game.Exit) bool { return bootui.CapLocked(s, e) }) {
		ids = append(ids, "story.first-locked")
	}
	var q []queued
	for _, id := range ids {
		if m.unseen(id) {
			q = append(q, queued{id, func() *story.Scene { return story.NewBeat(id) }})
		}
	}
	return q
}

// biomeFall is the drop into biome b (story.NewBiome); on a map, into its
// node, by the node's name and its own line where it has one.
func (m *Model) biomeFall(b game.BiomeState) *story.Scene {
	if b.Node != "" && b.Name != "" {
		return story.NewNodeBiome(b.Node, b.Topic, b.Name, m.drawnLines())
	}
	return story.NewBiome(b.Topic, m.drawnLines())
}

// rtfmLoadedMsg is RTFM loaded in the background (preloadRTFM), nil when
// it could not be.
type rtfmLoadedMsg struct{ cur *curriculum.Curriculum }

// preloadRTFM loads RTFM in the background when the boot stands at a
// route with an exit a capability locks, so the card can name the module
// whose trial opens it (route.locked) rather than the capability alone.
// RTFM otherwise loads on its first visit.
func (m *Model) preloadRTFM() tea.Cmd {
	s := m.state
	if m.cur != nil || m.rtfmLoading || s == nil || s.Phase != game.PhaseRoute || s.Biome >= len(s.Biomes) ||
		!slices.ContainsFunc(s.Biomes[s.Biome].Exits, func(e game.Exit) bool { return bootui.CapLocked(s, e) }) {
		return nil
	}
	m.rtfmLoading = true
	return m.background(func(ctx context.Context) tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		dir, err := guide.Snapshot(ctx)
		if err != nil {
			return rtfmLoadedMsg{}
		}
		cur, err := curriculum.Load(dir)
		if err != nil {
			return rtfmLoadedMsg{}
		}
		return rtfmLoadedMsg{cur}
	})
}

// onRTFMLoaded keeps what preloadRTFM loaded, unless a visit loaded it
// first, and hands the screens the module names.
func (m *Model) onRTFMLoaded(msg rtfmLoadedMsg) tea.Cmd {
	m.rtfmLoading = false
	if m.cur == nil && msg.cur != nil {
		m.cur = msg.cur
		m.refresh()
	}
	return nil
}
