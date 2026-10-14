package boot

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
)

// node is one room on the map.
type node int

const (
	nodeAhead node = iota
	nodeDone
	nodeHere
	nodeCrash
	nodeBoss
)

func (n node) glyph() string {
	switch n {
	case nodeDone:
		return sGreen.Render("✓")
	case nodeHere:
		return sYellow.Bold(true).Render("●")
	case nodeCrash:
		return sRed.Bold(true).Render("☠")
	case nodeBoss:
		return sMagenta.Render("⚔")
	}
	return sFaint.Render("○")
}

// mapNodes places every room of the boot. merge is the merge window the
// player stands in, as the index of the biome it follows (-1 for none);
// here is the biome the player is in (-1 between biomes or after the end).
func mapNodes(st *game.State) (nodes [][]node, merge, here int) {
	b, r := position(st)
	merge, here = -1, -1
	switch st.Phase {
	case game.PhaseMerge:
		if r == 0 && b > 0 {
			merge = b - 1
		}
	case game.PhaseCleared:
		b, r = len(st.Biomes), 0
	default:
		here = b
	}
	nodes = make([][]node, len(st.Biomes))
	for i, bs := range st.Biomes {
		nodes[i] = make([]node, bs.Rooms)
		for j := range nodes[i] {
			switch {
			case i < b || (i == b && j < r):
				nodes[i][j] = nodeDone
			case i == b && j == r && st.Phase == game.PhaseCrashed:
				nodes[i][j] = nodeCrash
			case i == b && j == r && (st.Phase == game.PhaseDoors || st.Phase == game.PhaseProblem):
				nodes[i][j] = nodeHere
			case bs.Kind == rules.Boss:
				nodes[i][j] = nodeBoss
			}
		}
	}
	return nodes, merge, here
}

// RenderMap draws the whole boot as a strip: one box per biome with its
// rooms as nodes (✓ cleared, ● here, ○ ahead, ☠ crash, ⚔ boss) and the
// merge windows as the links between boxes. It falls back to a single line
// when the boxes don't fit width. Other screens embed it.
func RenderMap(st *game.State, width int) string {
	if st == nil || len(st.Biomes) == 0 || width <= 0 {
		return ""
	}
	nodes, merge, here := mapNodes(st)
	if s, ok := boxMap(st, nodes, merge, here, width); ok {
		return s
	}
	return lineMap(st, nodes, merge, here, width)
}

func boxMap(st *game.State, nodes [][]node, merge, here, width int) (string, bool) {
	var rows [3]strings.Builder
	total := 0
	for i := range st.Biomes {
		if i > 0 {
			link := sFaint.Render("───")
			if merge == i-1 {
				link = sFaint.Render("─") + nodeHere.glyph() + sFaint.Render("─")
			}
			rows[0].WriteString("   ")
			rows[1].WriteString(link)
			rows[2].WriteString("   ")
			total += 3
		}
		glyphs := make([]string, len(nodes[i]))
		for j, n := range nodes[i] {
			glyphs[j] = n.glyph()
		}
		inside := strings.Join(glyphs, " ")
		name := biomeLabel(st, i)
		w := max(widthOf(name)+6, widthOf(inside)+4)
		f := frameIdle
		switch {
		case i == here && st.Phase == game.PhaseCrashed:
			f = frameCursed
		case i == here:
			f = frameHere
		case allDone(nodes[i]):
			f = frameDone
		}
		label := name
		if i == here {
			label = sBold.Render(name)
		}
		b := box(f, w, label, "", []string{center(inside, w-4)})
		for k := range rows {
			rows[k].WriteString(b[k])
		}
		total += w
	}
	if total > width {
		return "", false
	}
	return rows[0].String() + "\n" + rows[1].String() + "\n" + rows[2].String(), true
}

func allDone(ns []node) bool {
	for _, n := range ns {
		if n != nodeDone {
			return false
		}
	}
	return len(ns) > 0
}

// lineMap is the one-line map: names and nodes, then nodes only except for
// the current biome, then clipped.
func lineMap(st *game.State, nodes [][]node, merge, here, width int) string {
	build := func(names bool) string {
		var sb strings.Builder
		for i := range st.Biomes {
			if i > 0 {
				if merge == i-1 {
					sb.WriteString(" " + nodeHere.glyph() + " ")
				} else {
					sb.WriteString(sFaint.Render(" › "))
				}
			}
			if names || i == here {
				name := biomeLabel(st, i)
				if i == here {
					name = sBold.Render(name)
				}
				sb.WriteString(name + " ")
			}
			for _, n := range nodes[i] {
				sb.WriteString(n.glyph())
			}
		}
		return sb.String()
	}
	if s := build(true); widthOf(s) <= width {
		return s
	}
	return clip(build(false), width)
}

// Map is the boot map as its own screen: the strip, a legend, and every
// door opened so far. Any of esc, q, m or enter emits screens.Back.
type Map struct{ env Env }

func NewMap(env Env) *Map { return &Map{env: env} }

// Refresh swaps in new state and clock.
func (m *Map) Refresh(env Env) { m.env = env }

func (m *Map) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	switch key, _ := keyName(msg); key {
	case "esc", "q", "m", "enter":
		return m, screens.Emit(screens.Back{})
	}
	return m, nil
}

func (m *Map) View(width, height int) string {
	if m.env.State == nil {
		return noBoot(width, height)
	}
	return mapScreen(m.env, width, height, hint("esc", "back"))
}

func mapScreen(env Env, width, height int, footer string) string {
	st := env.State
	body := append(env.header("map", width), "")
	body = append(body, centerBlock(RenderMap(st, width), width)...)
	legend := strings.Join([]string{
		nodeDone.glyph() + " cleared", nodeHere.glyph() + " here", nodeAhead.glyph() + " ahead",
		nodeCrash.glyph() + " crash", nodeBoss.glyph() + " boss",
	}, "   ")
	body = append(body, "", center(legend, width), "")
	rows := historyRows(env, width)
	room := height - len(body) - 1
	if height > 0 && len(rows) > room {
		// Keep the header row and the most recent doors.
		keep := max(room-1, 0)
		rows = append(rows[:1:1], rows[len(rows)-keep:]...)
	}
	body = append(body, rows...)
	return layout(width, height, body, footer)
}

// historyRows is the table of doors opened this boot, oldest first, with a
// header row. Titles are shown even for mystery doors: opening one
// reveals it.
func historyRows(env Env, width int) []string {
	ss := steps(env.State)
	if len(ss) == 0 {
		return []string{"  " + sFaint.Render("no doors opened yet")}
	}
	const (
		numW    = 3
		biomeW  = 14
		ratingW = 5
		resultW = 11
		timeW   = 13
	)
	showBiome, showTime := true, true
	titleW := width - 2 - numW - 2 - biomeW - 2 - 2 - ratingW - 2 - resultW - 2 - timeW
	if titleW < 14 {
		showBiome = false
		titleW += biomeW + 2
	}
	if titleW < 14 {
		showTime = false
		titleW += timeW + 2
	}
	// Past about 40 cells a wider title column only spreads the table thin;
	// cap it and center the table instead.
	pad := 2
	if titleW > 40 {
		pad += (titleW - 40) / 2
		titleW = 40
	}
	row := func(n, biome, title, rating, result, tm string) string {
		s := strings.Repeat(" ", pad) + fit(n, numW) + "  "
		if showBiome {
			s += fit(biome, biomeW) + "  "
		}
		s += fit(title, max(titleW, 4)) + "  " + fmt.Sprintf("%*s", ratingW, rating) + "  " + fit(result, resultW)
		if showTime {
			s += "  " + tm
		}
		return s
	}
	out := []string{sFaint.Render(row("#", "biome", "door", "◆", "result", "time"))}
	for _, s := range ss {
		a := s.Attempt
		p := env.problem(a.Door.Problem)
		tm := fmtClock(elapsed(a, env.Now)) + " / " + fmtClock(a.Door.Timer)
		if a.Opened.IsZero() {
			tm = "-"
		}
		title := p.Title
		if a.Door.Boss != "" {
			title = "⚔ " + title
		}
		out = append(out, row(fmt.Sprint(s.N), biomeLabel(env.State, s.Biome), title,
			fmt.Sprint(a.Door.Rating), outcomeLabel(a), tm))
	}
	return out
}

func outcomeLabel(a game.Attempt) string {
	switch a.Outcome {
	case game.Cleared:
		if a.Quacks > 0 {
			return sGreen.Render(fmt.Sprintf("✓ AC 🦆%d", a.Quacks))
		}
		return sGreen.Render("✓ AC")
	case game.TimedOut:
		return sRed.Render("☠ timer")
	case game.OutOfLives:
		return sRed.Render("☠ lives")
	case game.GaveUp:
		return sRed.Render("☠ :q!")
	case game.Retreated:
		return sYellow.Render("↺ retreat")
	case "":
		return sYellow.Render("● open")
	}
	return string(a.Outcome)
}

// elapsed is how long attempt a ran, up to now when it is still open.
func elapsed(a game.Attempt, now time.Time) time.Duration {
	end := a.Closed
	if end.IsZero() {
		end = now
	}
	if a.Opened.IsZero() || end.Before(a.Opened) {
		return 0
	}
	return end.Sub(a.Opened)
}
