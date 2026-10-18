package boot

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// The boot map, two ways. The strip (renderMap) is one box per biome with
// its rooms as nodes, which the doors, merge, route and clear screens embed.
// The graph (graphView) is the map as the hub draws it (Hub, map.md §(g)):
// on a map boot (rules.Rules.Nodes) every node of every tier by act, the
// path taken, where the boot stands, what is locked and the bosses ahead.
// A linear boot draws the strip and its doors instead.

// node is one room on the map.
type node int

const (
	nodeAhead node = iota
	nodeDone
	nodeHere
	nodeCrash
	nodeBoss
	nodeLost // a timeout or :q! the boot survived: the room is lost (FailMoveOn)
)

func (n node) glyph() string {
	switch n {
	case nodeDone:
		return ui.OK.Render(ui.G.Pass)
	case nodeHere:
		return ui.Focus.Render(ui.G.Here)
	case nodeCrash:
		return ui.Danger.Render(ui.G.Crash)
	case nodeBoss:
		return ui.Special.Render(ui.G.Boss)
	case nodeLost:
		return ui.Warn.Render(ui.G.Fail)
	}
	return ui.Muted.Render(ui.G.Ahead)
}

// mapNodes places every room of the boot. merge is the merge window the
// player stands in, as the index of the biome it follows (-1 for none);
// here is the biome the player is in (-1 between biomes or after the end).
// A room passed is cleared, or lost when its last door failed and the boot
// went on (steps).
func mapNodes(st *game.State) (nodes [][]node, merge, here int) {
	b, r := position(st)
	merge, here = -1, -1
	switch st.Phase {
	case game.PhaseMerge:
		if r == 0 && b > 0 {
			merge = b - 1
			for merge > 0 && st.Biomes[merge].Rooms == 0 {
				merge-- // the window follows the last room played, not a skip
			}
		}
	case game.PhaseCleared:
		b, r = len(st.Biomes), 0
		if st.Ending != "" {
			b = st.Biome + 1 // an ending (the Trial) leaves the tiers after it unplayed
		}
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
	for _, s := range steps(st) {
		if s.Biome < len(nodes) && s.Room < len(nodes[s.Biome]) && nodes[s.Biome][s.Room] == nodeDone &&
			lostRoom(st, s.Attempt, s.Biome) {
			nodes[s.Biome][s.Room] = nodeLost
		}
	}
	return nodes, merge, here
}

// renderMap draws the whole boot as a strip: one box per biome with its
// rooms as nodes (✓ cleared, ✗ lost, ● here, ○ ahead, ☠ crash, ⚔ boss)
// and the merge windows as the links between boxes. It falls back to a
// single line when the boxes don't fit width. Other screens embed it. A
// map's skip, a tier of no rooms, is left out.
func renderMap(st *game.State, width int) []string {
	nodes, merge, here := mapNodes(st)
	if ls := boxMap(st, nodes, merge, here, width); ls != nil {
		return ls
	}
	return []string{lineMap(st, nodes, merge, here, width)}
}

// boxMap is the map as a row of boxes, or nil when they don't fit width.
func boxMap(st *game.State, nodes [][]node, merge, here, width int) []string {
	var rows [3]strings.Builder
	total, prev := 0, -1
	for i := range st.Biomes {
		if len(nodes[i]) == 0 {
			continue
		}
		if prev >= 0 {
			link := ui.Rule(3)
			if merge == prev {
				link = ui.Rule(1) + nodeHere.glyph() + ui.Rule(1)
			}
			rows[0].WriteString("   ")
			rows[1].WriteString(link)
			rows[2].WriteString("   ")
			total += 3
		}
		prev = i
		glyphs := make([]string, len(nodes[i]))
		for j, n := range nodes[i] {
			glyphs[j] = n.glyph()
		}
		inside := strings.Join(glyphs, " ")
		name := biomeLabel(st, i)
		w := max(widthOf(name)+6, widthOf(inside)+4)
		f := ui.FrameIdle
		switch {
		case i == here && st.Phase == game.PhaseCrashed:
			f = ui.FrameCursed
		case i == here:
			f = ui.FrameHere
		case allDone(nodes[i]):
			f = ui.FrameDone
		}
		label := name
		if i == here {
			label = ui.Strong.Render(name)
		}
		b := ui.Frame(f, w, label, "", []string{ui.Center(inside, w-4)})
		for k := range rows {
			rows[k].WriteString(b[k])
		}
		total += w
	}
	if total > width {
		return nil
	}
	return []string{rows[0].String(), rows[1].String(), rows[2].String()}
}

func allDone(ns []node) bool {
	for _, n := range ns {
		if n != nodeDone && n != nodeLost {
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
		prev := -1
		for i := range st.Biomes {
			if len(nodes[i]) == 0 {
				continue
			}
			if prev >= 0 {
				if merge == prev {
					sb.WriteString(" " + nodeHere.glyph() + " ")
				} else {
					sb.WriteString(ui.Muted.Render(ui.G.Crumb))
				}
			}
			prev = i
			if names || i == here {
				name := biomeLabel(st, i)
				if i == here {
					name = ui.Strong.Render(name)
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
	return ui.Clip(build(false), width)
}

// ---- the graph ---------------------------------------------------------------

// onMap reports whether the boot plays a map: its rules carry the graph
// and its tiers are the graph's. Boot #1 plays the short linear shape
// (RT11), as does any preset without nodes.
func onMap(st *game.State) bool {
	return len(st.Rules.Nodes) > 0 && len(st.Biomes) == len(st.Rules.Biomes)
}

// spot is one node of the map as the graph draws it.
type spot struct {
	id, name   string
	boss, skip bool
	picks      int
	toll       float64 // what taking it cuts later clocks by (rules.Biome.Toll)
	taken      bool    // its tier played it
	offered    bool    // an exit on offer: at the route, or in the merge window before it
	reach      bool    // ahead, and reachable from the node the boot stands on
	gate       string  // what locks it, "" when open: "CAP_DS", "P1", "pool"
	oneShot    bool    // a boss whose fail ends the boot
	rooms      []node  // a taken node's rooms
}

// reachable is every node the boot has played or can still play: the
// nodes taken, then whatever the last one's exits lead to, open or locked.
// offeredAt is the tier whose exits are on offer, at its route or in the
// merge window before it; -1 for none.
func reachable(st *game.State) (reach map[string]bool, offeredAt int) {
	byID := map[string]rules.Biome{}
	for _, n := range st.Rules.Nodes {
		byID[n.ID] = n
	}
	reach, offeredAt = map[string]bool{}, -1
	last := ""
	for _, bs := range st.Biomes {
		if bs.Node == "" {
			break
		}
		reach[bs.Node], last = true, bs.Node
	}
	if last == "" { // Userland is the start node, whatever the engine wrote
		for _, n := range st.Rules.Nodes {
			if n.Tier == 0 {
				reach[n.ID], last = true, n.ID
			}
		}
	}
	if st.Phase == game.PhaseCrashed || st.Phase == game.PhaseCleared {
		return reach, -1
	}
	for front := []string{last}; len(front) > 0; {
		var next []string
		for _, id := range front {
			for _, n := range byID[id].Next {
				if !reach[n] {
					reach[n] = true
					next = append(next, n)
				}
			}
		}
		front = next
	}
	t := byID[last].Tier + 1
	if (st.Phase == game.PhaseRoute || st.Phase == game.PhaseMerge) && t < len(st.Biomes) && st.Biomes[t].Node == "" {
		offeredAt = t
	}
	return reach, offeredAt
}

// tierSpots is tier t's nodes in the graph's order, as the boot stands.
// The exits the engine wrote for the tier (BiomeState.Exits) decide what
// is locked there; a tier further on is read off the rules, the boot's
// capability and package snapshots and its prestige, as the engine reads
// exits, so the map shows a lock before the route meets it.
func tierSpots(st *game.State, t int, reach map[string]bool, nodes [][]node, offeredAt int) []spot {
	bs := st.Biomes[t]
	var out []spot
	for _, n := range st.Rules.Nodes {
		if n.Tier != t {
			continue
		}
		// A secret node (the Trial) is on the map only where the engine
		// wrote it into the tier's exits, or once taken; locked, it has no
		// name (achievements.md (f)).
		ex := slices.IndexFunc(bs.Exits, func(e game.Exit) bool { return e.Node == n.ID })
		if n.Hidden && ex < 0 && bs.Node != n.ID {
			continue
		}
		s := spot{id: n.ID, name: n.Name, boss: n.Boss != "", skip: n.Skip, picks: n.Picks, toll: n.Toll, reach: reach[n.ID]}
		switch {
		case n.Hidden && ex >= 0 && bs.Exits[ex].Locked != "":
			s.name = "???"
		case s.name == "" && s.boss:
			s.name = bossLabel(n.Boss)
		case s.name == "":
			s.name = n.ID
		}
		if s.boss {
			s.oneShot = oneShotAt(st, t)
		}
		s.gate = gateOf(st, n)
		if ex >= 0 {
			s.gate = exitGate(st, bs.Exits[ex])
		}
		if bs.Node == n.ID {
			s.taken, s.rooms = true, nodes[t]
		}
		s.offered = t == offeredAt && s.reach
		out = append(out, s)
	}
	return out
}

// gateOf is what locks node n for this boot, "" when nothing does: its
// prestige gate, else its topic's capability when the boot started without
// it and without the package that also opens it.
func gateOf(st *game.State, n rules.Biome) string {
	if n.MinPrestige > st.Prestige {
		return fmt.Sprintf("P%d", n.MinPrestige)
	}
	c := game.CapabilityFor(n.Topic)
	if c == "" || slices.Contains(st.Caps, c) || n.Package != "" && slices.Contains(st.Packages, n.Package) {
		return ""
	}
	return c
}

// exitGate is gateOf for an exit the engine wrote: "" when it can be
// taken. A lock with no gate unmet is the pool's, or the cycles' (the
// Trial's full pool).
func exitGate(st *game.State, e game.Exit) string {
	switch g := e.Gate; {
	case e.Locked == "":
		return ""
	case e.Locked == game.LockedFull:
		return "full"
	case g.Prestige > st.Prestige:
		return fmt.Sprintf("P%d", g.Prestige)
	case CapLocked(st, e):
		return g.Capability
	case g.Full:
		return game.LockedFull
	}
	return "pool"
}

// oneShotAt reports whether a boss of tier t ends the boot on a fail: one
// of the boot's last rules.OneShot bosses (the ascensions' raise it), or
// any boss without a failure price (a boot saved before cycles), as
// State.OneShot reads the current one.
func oneShotAt(st *game.State, t int) bool {
	r := &st.Rules
	if t >= len(r.Biomes) || r.Biomes[t].FailCycles <= 0 {
		return true
	}
	left := 0
	for i := t; i < len(st.Biomes); i++ {
		if st.Biomes[i].Kind == rules.Boss {
			left++
		}
	}
	return left <= max(r.OneShot, r.Ascend(st.Prestige).OneShot)
}

// actName is act a's name: act I, act II, act III, then act 4 on.
func actName(a int) string {
	if a < 3 {
		return "act " + []string{"I", "II", "III"}[a]
	}
	return fmt.Sprintf("act %d", a+1)
}

// label is a spot as one cell of the graph: ⚔ for a boss, its rooms when
// taken, its lock, its picks and a one-shot boss's warning. A spot out of
// reach or locked is Muted, as is one not taken in a tier played.
func (s spot) label() string {
	name := s.name
	if s.boss {
		name = ui.G.Boss + " " + name
	}
	live := s.taken || s.reach
	switch {
	case s.taken:
		name = ui.Strong.Render(name)
	case !live || s.gate != "":
		name = ui.Muted.Render(name)
	case s.boss:
		name = ui.Special.Render(name)
	}
	if s.taken && len(s.rooms) > 0 {
		var g strings.Builder
		for _, n := range s.rooms {
			g.WriteString(n.glyph())
		}
		name += " " + g.String()
	}
	if s.gate != "" && live {
		name += " " + ui.Muted.Render(ui.G.CapLocked+" "+s.gate)
	}
	if s.picks > 0 && live {
		name += " " + ui.OK.Render(fmt.Sprintf("+%d", s.picks))
	}
	if s.toll > 0 && live {
		name += " " + tollText(s.toll)
	}
	if s.oneShot && live && !s.taken {
		name += " " + ui.Danger.Render("one shot")
	}
	return name
}

// graph is the map's tiers by act, each tier its spots.
type graph struct {
	acts      [][]int
	tiers     [][]spot
	offeredAt int
	merge     int // the tier the merge window follows, -1 outside one
}

func newGraph(st *game.State) graph {
	nodes, merge, _ := mapNodes(st)
	reach, offeredAt := reachable(st)
	g := graph{offeredAt: offeredAt, merge: merge}
	for _, a := range st.Acts() {
		var ts []int
		for t := a.First; t <= a.Last; t++ {
			ts = append(ts, t)
		}
		g.acts = append(g.acts, ts)
	}
	for t := range st.Biomes {
		g.tiers = append(g.tiers, tierSpots(st, t, reach, nodes, offeredAt))
	}
	return g
}

// bands draws the graph one act a band: a rule naming the act, then the
// act's tiers side by side, each a column of its nodes, ▸ on the exits on
// offer. nil when a band does not fit width.
func (g graph) bands(width int) []string {
	var out []string
	for a, ts := range g.acts {
		var cols [][]string
		for _, t := range ts {
			var col []string
			for _, s := range g.tiers[t] {
				mark := "  "
				if s.offered {
					mark = ui.Focus.Render(ui.G.Cursor) + " "
				}
				col = append(col, mark+s.label())
			}
			// JoinH pads a short column by its first line: every line of
			// a column is one width.
			w := maxWidth(col)
			for i, l := range col {
				col[i] = ui.Fit(l, w)
			}
			cols = append(cols, col)
		}
		row := indent(1, ui.JoinH(3, cols...))
		if maxWidth(row) > width {
			return nil
		}
		head := " " + ui.Strong.Render(actName(a)) + " "
		out = append(out, ui.Rule(1)+head+ui.Rule(max(width-widthOf(head)-1, 0)))
		out = append(out, row...)
	}
	return out
}

// rows draws the graph compactly, for a terminal the bands do not fit: a
// line an act, its tiers as parts, the path taken and what lies ahead, and
// under a tier played the nodes not taken:
//
//	act I    Userland ✓✗ ─ The Scheduler ✓ ─ ⚔ Off-by-One ✓
//	           not taken: Brute Force Bunker · The Emulator · ⚔ OOM Killer
//	act II   ▸ Bisect Bridge □ CAP_BSEARCH +1 · The ALU ─ ⚔ Heisenbug | ⚔ Race Condition
//
// A tier ahead names one or two reachable biomes, counts more, and names
// every reachable boss. A line wraps between parts.
func (g graph) rows(width int) []string {
	const labelW = 9
	tierSep, exitSep, bossSep := ui.Muted.Render(" "+ui.G.Rule+" "), ui.Muted.Render(ui.G.Sep), ui.Muted.Render(" | ")
	var out []string
	for a, ts := range g.acts {
		var parts, passed []string
		add := func(first bool, ps ...string) {
			if first && len(parts) > 0 {
				ps[0] = tierSep + ps[0]
			}
			parts = append(parts, ps...)
		}
		for _, t := range ts {
			spots := g.tiers[t]
			if g.merge >= 0 && t == g.merge+1 {
				add(true, nodeHere.glyph()+" merge window")
			}
			taken := slices.IndexFunc(spots, func(s spot) bool { return s.taken })
			var reach []spot
			for _, s := range spots {
				if s.reach && !s.taken {
					reach = append(reach, s)
				}
			}
			switch {
			case taken >= 0:
				add(true, spots[taken].label())
				for _, s := range spots {
					if !s.taken && !s.skip {
						sep := exitSep
						if len(passed) == 0 {
							sep = "not taken: "
						}
						passed = append(passed, ui.Muted.Render(sep+plainName(s)))
					}
				}
			case t == g.offeredAt && len(reach) > 0:
				ex := []string{ui.Focus.Render(ui.G.Cursor) + " " + reach[0].label()}
				for _, s := range reach[1:] {
					ex = append(ex, exitSep+s.label())
				}
				add(true, ex...)
			default:
				add(true, aheadParts(reach, bossSep)...)
			}
		}
		for i, l := range wrapParts(parts, width-labelW) {
			lead := strings.Repeat(" ", labelW)
			if i == 0 {
				lead = ui.Fit(" "+ui.Strong.Render(actName(a)), labelW)
			}
			out = append(out, lead+l)
		}
		for _, l := range wrapParts(passed, width-labelW-2) {
			out = append(out, strings.Repeat(" ", labelW+2)+l)
		}
	}
	return out
}

// plainName is a spot's name for the "not taken" line, ⚔ for a boss.
func plainName(s spot) string {
	if s.boss {
		return ui.G.Boss + " " + s.name
	}
	return s.name
}

// aheadParts is a tier ahead: its reachable bosses by name, each after
// the first behind sep, else one or two biomes by name and more as a
// count, and the skip as "or skip ⏱-20%" (its toll; "+2", its picks,
// before D62).
func aheadParts(reach []spot, sep string) []string {
	var bosses, biomes []string
	var skip *spot
	for _, s := range reach {
		switch {
		case s.skip:
			skip = &s
		case s.boss && len(bosses) == 0:
			bosses = append(bosses, s.label())
		case s.boss:
			bosses = append(bosses, sep+s.label())
		default:
			biomes = append(biomes, s.label())
		}
	}
	if len(bosses) > 0 {
		return bosses
	}
	part := strings.Join(biomes, ui.Muted.Render(ui.G.Sep))
	if len(biomes) > 2 {
		part = fmt.Sprintf("1 of %d biomes", len(biomes))
	}
	if skip != nil && (skip.picks > 0 || skip.toll > 0) {
		part += ui.Muted.Render(", or skip")
		if skip.picks > 0 {
			part += " " + ui.OK.Render(fmt.Sprintf("+%d", skip.picks))
		}
		if skip.toll > 0 {
			part += " " + tollText(skip.toll)
		}
	}
	if part == "" {
		part = nodeAhead.glyph()
	}
	return []string{part}
}

// tollText is a toll on the hub: what taking the exit cuts later clocks
// by, "⏱-20%", in the warning's colour.
func tollText(toll float64) string {
	return ui.Warn.Render(fmt.Sprintf("%s-%.0f%%", ui.G.Timer, 100*toll))
}

// wrapParts runs parts into lines of at most w cells, breaking only
// between parts: each part after the first carries its own separator,
// which then leads the line it wraps to. A line still wider is clipped.
func wrapParts(parts []string, w int) []string {
	var out []string
	line := ""
	for _, p := range parts {
		if line != "" && widthOf(line)+widthOf(p) > w {
			out = append(out, ui.Clip(line, w))
			line = ""
		}
		line += p
	}
	if line != "" {
		out = append(out, ui.Clip(line, w))
	}
	return out
}

// graphView is the hub's map in rows lines at most: on a map boot, the
// bands when they fit, else the compact rows; on a linear boot, the strip,
// its legend and the doors opened, the most recent kept.
func graphView(env Env, width, rows int) []string {
	st := env.State
	if !onMap(st) {
		body := centerBlock(renderMap(st, width), width)
		body = append(body, "", ui.Center(legend(st), width), "")
		hist := historyRows(env, width)
		if room := rows - len(body); rows > 0 && len(hist) > room {
			keep := max(room-1, 0)
			hist = append(hist[:1:1], hist[len(hist)-keep:]...)
		}
		return append(body, hist...)
	}
	g := newGraph(st)
	if b := g.bands(width); b != nil && len(b) <= rows {
		return b
	}
	out := g.rows(width)
	return out[:min(len(out), max(rows, 0))]
}

// legend names the strip's glyphs; ✗ only where a room can be lost.
func legend(st *game.State) string {
	parts := []string{nodeDone.glyph() + " cleared"}
	if st.Rules.FailMoveOn {
		parts = append(parts, nodeLost.glyph()+" lost")
	}
	parts = append(parts, nodeHere.glyph()+" here", nodeAhead.glyph()+" ahead",
		nodeCrash.glyph()+" crash", nodeBoss.glyph()+" boss")
	return strings.Join(parts, "   ")
}

// mapScreen is the map under a boot screen's own header, as the doors
// screen flips to it: the graph, and the footer the caller gives.
func mapScreen(env Env, width, height int, footer string) string {
	body := append(env.header("map", width, height), "")
	body = append(body, graphView(env, width, height-len(body)-1)...)
	return ui.Layout(width, height, body, footer)
}

// historyRows is the table of doors opened this boot, oldest first, with a
// header row. Titles are shown even for mystery doors: opening one
// reveals it. A door's difficulty is a bare numeral in its rank color under
// a ◆ header, so the column stays narrow.
func historyRows(env Env, width int) []string {
	ss := steps(env.State)
	if len(ss) == 0 {
		return []string{"  " + ui.Muted.Render("no doors opened yet")}
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
		s := strings.Repeat(" ", pad) + ui.Fit(n, numW) + "  "
		if showBiome {
			s += ui.Fit(biome, biomeW) + "  "
		}
		// By cells: %*s pads bytes, which a colored numeral has more of.
		s += ui.Fit(title, max(titleW, 4)) + "  " + ui.AlignRight(rating, ratingW) + "  " + ui.Fit(result, resultW)
		if showTime {
			s += "  " + tm
		}
		return s
	}
	out := []string{ui.Muted.Render(row("#", "biome", "door", ui.G.Rating, "result", "time"))}
	for i, s := range ss {
		a := s.Attempt
		p := env.problem(a.Door.Problem)
		tm := units.Clock(elapsed(a, env.Now)) + " / " + units.Clock(a.Door.Timer)
		if a.Opened.IsZero() {
			tm = "-"
		}
		title := p.Title
		if a.Door.Boss != "" {
			title = ui.G.Boss + " " + title
		}
		// The last attempt of a crashed boot is the crash; every other one
		// left the boot running.
		last := env.State.Phase == game.PhaseCrashed && i == len(ss)-1
		rating := ""
		if catalog.ShowsRating(a.Door.Problem) {
			rating = ui.Rank(float64(a.Door.Rating)).Render(fmt.Sprint(a.Door.Rating))
		}
		out = append(out, row(fmt.Sprint(s.N), biomeLabel(env.State, s.Biome), title, rating, outcomeLabel(a, last), tm))
	}
	return out
}

// outcomeLabel is how attempt a ended; last is set for the attempt that
// ended the boot. A timeout or :q! that did not end it burned cycles, and
// lost the room or (D44, before cycles) drew it again, as a retreat does.
func outcomeLabel(a game.Attempt, last bool) string {
	switch a.Outcome {
	case game.Cleared:
		// Green paints the glyph; the word stays plain (R2).
		s := ui.OK.Render(ui.G.Pass) + " AC"
		if a.Quacks > 0 {
			s += fmt.Sprintf(" %s%d", ui.G.Duck, a.Quacks)
		}
		return s
	case game.TimedOut:
		if !last {
			return ui.Warn.Render(ui.G.Timer) + "timer -" + ui.Life.Render(ui.G.Life)
		}
		return ui.Danger.Render(ui.G.Crash + " timer")
	case game.OutOfLives:
		return ui.Danger.Render(ui.G.Crash + " cycles")
	case game.GaveUp:
		if !last {
			return ui.Warn.Render(ui.G.Fail) + " :q! -" + ui.Life.Render(ui.G.Life)
		}
		return ui.Danger.Render(ui.G.Crash + " :q!")
	case game.Retreated:
		return ui.Warn.Render(ui.G.Retreat) + " retreat"
	case "":
		return ui.Focus.Render(ui.G.Here) + " open"
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
