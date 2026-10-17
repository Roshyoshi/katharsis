package boot

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Hub is the map as the boot's hub (D47, map.md §(g), meta.md §(c)). A
// boot is suspended only between doors: there m opens the hub from any
// boot screen, and a launch on a suspended boot lands on it. It draws the
// graph, what is at stake, and the keys:
//
//	enter, esc, m   Resume: back where the boot stopped
//	n               asks first (map.abandon), then Abandon: the boot ends
//	                as a crash, and GRUB starts the next one
//	t               screens.OpenLearn: RTFM
//	q               screens.Quit: the boot stays suspended, saved
//
// While it asks, y goes ahead and any other key keeps the boot.
// Upsolves are GRUB's, between boots: the map of a boot in flight is no
// place for them.
type Hub struct {
	env    Env
	launch bool // a launch landed here: enter resumes a suspended boot
	asking bool
	then   tea.Msg // what follows the abandon asked about; nil for GRUB
}

// Resume leaves the hub for where the boot stopped; Launch is set when the
// launch landed on the hub, so the boot resumes from suspension.
type Resume struct{ Launch bool }

// Abandon ends the boot in flight as a crash (game.Abandon) and goes on to
// Then, the screens.StartBoot or StartTrial the hub asked about, or to
// GRUB for nil, where the next boot is chosen.
type Abandon struct{ Then tea.Msg }

// NewHub is the hub over the boot env holds; launch says the game's launch
// landed on it.
func NewHub(env Env, launch bool) *Hub { return &Hub{env: env, launch: launch} }

// Ask has the hub ask before then (a boot or trial starting) abandons the
// boot in flight.
func (h *Hub) Ask(then tea.Msg) { h.asking, h.then = true, then }

// Refresh swaps in new state and clock.
func (h *Hub) Refresh(env Env) { h.env = env }

func (h *Hub) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	key, ok := keyName(msg)
	if !ok {
		return h, nil
	}
	if h.asking {
		h.asking = false
		if key == "y" {
			return h, screens.Emit(Abandon{Then: h.then})
		}
		h.then = nil
		return h, nil
	}
	switch key {
	case "enter", "esc", "m":
		return h, screens.Emit(Resume{Launch: h.launch})
	case "n":
		h.Ask(nil)
	case "t":
		return h, screens.Emit(screens.OpenLearn{})
	case "q":
		return h, screens.Emit(screens.Quit{})
	}
	return h, nil
}

func (h *Hub) View(width, height int) string {
	st := h.env.State
	head := []string{h.headLine(width), ui.Rule(width)}
	var foot []string
	if h.asking {
		foot = append(foot, h.askLines(width)...)
	}
	foot = append([]string{ui.Rule(width)}, foot...)
	keys := []string{"enter", "resume", "n", "new boot"}
	keys = append(keys, "t", "RTFM", "q", "quit")
	footer := ui.Footer(width, keys...)
	if h.asking {
		footer = ui.Footer(width, "y", fmt.Sprintf("abandon boot #%d", st.Boot), "any other key", "keep it")
	}

	// The graph sits in the middle of the rows between the rules.
	rows := height - len(head) - len(foot) - 1
	body := graphView(h.env, width, rows)
	free := max(rows-len(body), 0)
	out := slices.Concat(head, make([]string, free/2), body, make([]string, free-free/2), foot)
	return ui.Layout(width, height, out, footer)
}

// headLine is the hub's first row: MAP, the boot, its prestige, whether
// the launch found it suspended and where it stands, then its cycles and
// the bytes at stake on the right. Short of width, the words go before
// the figures.
func (h *Hub) headLine(width int) string {
	st := h.env.State
	dot := ui.Muted.Render(ui.G.Sep)
	where := h.where()
	if h.launch {
		where = "suspended" + dot + where
	}
	boot := ui.Strong.Render("MAP") + dot + fmt.Sprintf("boot #%d", st.Boot)
	bytes := units.Bytes(st.Uncommitted)
	for _, try := range [][2]string{
		{boot + dot + prestigeLabel(st.Prestige) + dot + where, cyclesText(st) + "  " + bytes + ui.Muted.Render(" uncommitted")},
		{boot + dot + where, cyclesText(st) + "  " + bytes},
		{boot, ui.Hearts(st.Cycles, st.MaxCycles) + "  " + bytes},
	} {
		if widthOf(try[0])+2+widthOf(try[1]) <= width {
			return ui.Spread(try[0], try[1], width)
		}
	}
	return ui.Clip(boot, width)
}

// askLines are the question before an abandon: the story's line
// (map.abandon) and what it costs.
func (h *Hub) askLines(width int) []string {
	st := h.env.State
	var out []string
	if s := fl.Story("map.abandon", 0); s != "" {
		s = flavor.Fill(s, "boot", strconv.Itoa(st.Boot))
		for _, l := range ui.WrapN(s, max(width-2, 10), 2) {
			out = append(out, " "+ui.Voice.Render(l))
		}
	}
	cost := []string{fmt.Sprintf("ends boot #%d as a crash", st.Boot)}
	if st.Uncommitted > 0 {
		cost = append(cost, units.Bytes(st.Uncommitted)+" uncommitted lost")
	}
	if d := h.env.Meta.CoreDump; d != nil && st.Trial == nil {
		cost = append(cost, fmt.Sprintf("the core dump on %q destroyed", h.env.problem(d.Problem).Title))
	}
	for _, l := range ui.WrapN(strings.Join(cost, ", "), max(width-2, 10), 2) {
		out = append(out, " "+ui.Danger.Render(l))
	}
	return out
}

// where is where the boot stands, for the header: the room it is at, or
// the last place it cleared when it stands between biomes.
func (h *Hub) where() string {
	st := h.env.State
	b, r := position(st)
	switch {
	case st.Phase == game.PhaseDoors && st.PendingUpgrade:
		return "upgrade pick"
	case st.Phase == game.PhaseDoors && b < len(st.Biomes):
		return fmt.Sprintf("%s room %d/%d", biomeLabel(st, b), r+1, st.Biomes[b].Rooms)
	case st.Phase == game.PhaseMerge:
		return "merge window"
	}
	if ss := steps(st); len(ss) > 0 {
		last := ss[len(ss)-1]
		name := biomeLabel(st, last.Biome)
		if st.Biomes[last.Biome].Boss != "" {
			name = ui.G.Boss + " " + name
		}
		return "after " + name
	}
	return "route"
}

// cyclesText is the boot's cycles out of the pool it was dealt
// (State.StartCycles), "♥ 22/30 cycles", or a row of hearts for a pool
// small enough to draw (units.Hearts decides).
func cyclesText(st *game.State) string {
	h := ui.Hearts(st.Cycles, st.MaxCycles)
	if strings.Contains(units.Hearts(st.Cycles, st.MaxCycles), "/") {
		h += ui.Muted.Render(" cycles")
	}
	return h
}

// NewMap is the hub opened between doors, not at launch.
func NewMap(env Env) *Hub { return NewHub(env, false) }
