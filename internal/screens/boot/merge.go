package boot

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Merge is the merge window between biomes: commit the uncommitted bytes
// (c) or spend them on this boot, take a pending upgrade, pick a daemon,
// then leave. One cursor walks every pickable row and the leave row; enter
// acts on the row under it.
type Merge struct {
	env Env
	up  picker
	buy int // cursor within the items on sale
	dmn int // cursor within the daemon offers
	cur int // index into rows()
	// at is the row under the cursor when it last moved. The engine changes
	// the State this screen shares in place, so by Refresh the rows have
	// changed under cur already, and only at still names the row it was.
	at mergeRow

	// intro is the White Knight's block (daemon.intro) over the daemon
	// offers, for the first window that offers any; drawn records that the
	// last View had room to draw it, so the app marks it seen (Shown).
	intro, drawn bool
}

type rowKind int

const (
	rowUpgrade rowKind = iota
	rowSpend
	rowDaemon
	rowLeave
)

type mergeRow struct {
	kind rowKind
	i    int
}

func NewMerge(env Env) *Merge {
	m := &Merge{env: env}
	m.intro = len(env.State.DaemonOffers) > 0 && !game.HasSeen(&env.Meta, introHook)
	m.up.set(env.State)
	m.at = m.row()
	return m
}

// Refresh swaps in new state and clock. When the row under the cursor is
// gone (its pick was taken), the cursor moves to the next section.
func (m *Merge) Refresh(env Env) {
	old := m.at
	m.env = env
	m.up.set(env.State)
	rs := m.rows()
	m.cur = len(rs) - 1
	for i, r := range rs {
		if r == old || r.kind > old.kind {
			m.cur = i
			break
		}
	}
	m.sync()
}

// rows are the pickable rows, then leave. The engine offers daemons only
// while a slot is free and clears the offers once one is picked, so every
// daemon on offer is pickable. Every item on sale has a row, bought or
// not: one the bytes don't cover says so, and the engine refuses it.
func (m *Merge) rows() []mergeRow {
	st := m.env.State
	var rs []mergeRow
	if st.PendingUpgrade {
		for i := range st.UpgradeOffers {
			rs = append(rs, mergeRow{rowUpgrade, i})
		}
	}
	for i := range onSale(st) {
		rs = append(rs, mergeRow{rowSpend, i})
	}
	for i := range st.DaemonOffers {
		rs = append(rs, mergeRow{rowDaemon, i})
	}
	return append(rs, mergeRow{kind: rowLeave})
}

func (m *Merge) row() mergeRow {
	rs := m.rows()
	return rs[min(max(m.cur, 0), len(rs)-1)]
}

// sync points the section cursors at the selected row so each list draws
// its own cursor.
func (m *Merge) sync() {
	r := m.row()
	m.at = r
	switch r.kind {
	case rowUpgrade:
		m.up.cursor = r.i
	case rowSpend:
		m.buy = r.i
	case rowDaemon:
		m.dmn = r.i
	}
}

func (m *Merge) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	key, ok := keyName(msg)
	if !ok {
		return m, nil
	}
	st := m.env.State
	switch key {
	case "c":
		if st.Uncommitted > 0 {
			return m, screens.Emit(game.Commit{})
		}
	case "f":
		// fsck at a key, as the sketch has it (map.md §(g)): the heal the
		// boot pays for, whatever row the cursor is on.
		if slices.Contains(onSale(st), game.Fsck) {
			return m, screens.Emit(game.Spend{Item: game.Fsck})
		}
	case "prev", "next":
		m.cur = move(key, min(m.cur, len(m.rows())-1), len(m.rows()))
		m.sync()
	case "enter":
		switch r := m.row(); r.kind {
		case rowUpgrade:
			return m, screens.Emit(game.PickUpgrade{Upgrade: st.UpgradeOffers[r.i]})
		case rowSpend:
			return m, screens.Emit(game.Spend{Item: onSale(st)[r.i]})
		case rowDaemon:
			return m, screens.Emit(game.PickDaemon{Daemon: st.DaemonOffers[r.i]})
		default:
			return m, screens.Emit(game.LeaveMerge{})
		}
	}
	return m, nil
}

func (m *Merge) View(width, height int) string {
	st := m.env.State
	m.sync()
	cur := m.row()
	head := m.env.header("merge window", width, height)
	// The sections read as one form, so they share a column; on a wide
	// terminal it sits under the centered map instead of hugging the left.
	// The column grows past 76 cells when an option needs it, so a long
	// description is cut only when the terminal itself is too narrow.
	opts := daemonOptions(st.DaemonOffers, st)
	need := listWidth(opts)
	need = max(need, listWidth(itemOptions(st)))
	if st.PendingUpgrade {
		need = max(need, listWidth(upgradeOptions(st.UpgradeOffers, st)))
	}
	cw := min(width, max(76, 4+need))
	// The story lines over the sections go when the window is too short
	// for them: the daemon intro first, since it waits for a window with
	// room, while a boss's defeat line has only this one. The intro sits
	// over the offers, so a pick that took them takes it too.
	intro := m.intro && len(st.DaemonOffers) > 0
	defeat := bossDefeat(st)
	var secs []string
	m.drawn = false
	for _, c := range []struct {
		intro  bool
		defeat string
	}{{intro, defeat}, {false, defeat}, {intro, ""}, {false, ""}} {
		secs = m.sections(width, cw, cur, c.intro, c.defeat)
		if len(head)+1+len(secs) <= height-1 {
			m.drawn = c.intro
			break
		}
	}
	// Still too tall, the blank rows between the sections go, last first,
	// before the leave row would: at 80x24 the strip and apt's goal line
	// over a window that offers everything pushed it off the screen.
	for i := len(secs) - 1; i >= 0 && len(head)+len(secs) > height-1; i-- {
		if strings.TrimSpace(ansi.Strip(secs[i])) == "" {
			secs = slices.Delete(secs, i, i+1)
		}
	}
	// The Hatter's line over the table when there is room: the table's
	// word on the bytes it holds (merge.line), or on none (merge.nothing).
	hook := "merge.line"
	if st.Uncommitted == 0 {
		hook = "merge.nothing"
	}
	if v := fl.Story(hook, st.Seed^uint64(st.Biome)); v != "" && len(head)+3+len(secs) <= height-1 {
		secs = append([]string{ui.Center(ui.Voice.Render(ui.Muted.Render(ui.Clip(v, width))), width), ""}, secs...)
	}

	block := secs
	if mp := centerBlock(renderMap(st, width), width); len(head)+2+len(mp)+1+len(secs) <= height-1 {
		block = append(append(mp, ""), secs...)
	}
	keys := []string{"↑/↓", "move", "enter", "pick, buy or leave", "c", "commit"}
	if slices.Contains(onSale(st), game.Fsck) {
		keys = append(keys, "f", "fsck")
	}
	return ui.Layout(width, height, centered(head, block, height), ui.Footer(width, keys...))
}

// Fountain is a merge window's fountain (map.md §(e)): what it heals here
// (rules.FountainCycles with the ascensions' cut, free, up to the boot's
// starting cycles), and here, whether this window has one by
// rules.FountainWhere. A fountain here that heals nothing is dry: gone at
// the prestige that takes it (P5).
func Fountain(st *game.State) (heal int, here bool) {
	r := &st.Rules
	if r.FountainCycles <= 0 || st.Trial != nil || st.Phase != game.PhaseMerge {
		return 0, false
	}
	b := st.Biome
	boss := func(i int) bool { return i >= 0 && i < len(st.Biomes) && st.Biomes[i].Kind == rules.Boss }
	switch r.FountainWhere {
	case rules.FountainAct:
		here = boss(b)
	case rules.FountainBoss:
		here = boss(b + 1)
	default:
		here = true
	}
	return max(r.FountainCycles+r.Ascend(st.Prestige).FountainCycles, 0), here
}

// fountainRow is the window's fountain line, in the label column: what it
// gave (State.Fountain, "+5 cycles, free, up to 30 ✓"; the strip said
// "♥ 17 → 22" as it poured, and fsck or an Overclock since may have moved
// the cycles), that it gave none (the cycles at the line, fountain.full),
// or that it is dry (fountain.dry). nil in a window without one.
func fountainRow(st *game.State, label func(string) string) []string {
	heal, here := Fountain(st)
	if !here {
		return nil
	}
	start := st.StartCycles()
	var s string
	switch {
	case heal == 0:
		s = ui.Muted.Render("dry at " + prestigeLabel(st.Prestige) + ": no heal between acts")
	case st.Fountain > 0:
		s = fmt.Sprintf("+%d cycles", st.Fountain) + ui.Muted.Render(", free, up to "+fmt.Sprint(start)) + "  " +
			ui.OK.Render(ui.G.Pass)
	default:
		s = ui.Muted.Render(fmt.Sprintf("+%d cycles up to %d: yours are at the line, none taken", heal, start))
	}
	return []string{label("fountain") + s}
}

// fountainHook is the story's line for the window's fountain
// (fountain.drink, .full, .dry), "" for none.
func fountainHook(st *game.State) string {
	heal, here := Fountain(st)
	switch {
	case !here:
		return ""
	case heal == 0:
		return "fountain.dry"
	case st.Fountain > 0:
		return "fountain.drink"
	}
	return "fountain.full"
}

// introHook is the merge window's one-time block, a story scene.
const introHook = "daemon.intro"

// fullHook is the line where the daemon offers would be when every daemon
// seat is taken.
const fullHook = "merge.full"

// daemonsFull reports whether a window offers no daemons because the boot
// holds MaxDaemons (game enterMerge). There is no swap, so nothing on
// screen may promise one. A trial offers none at any count, so it is never
// full.
func daemonsFull(st *game.State) bool {
	return st.Trial == nil && len(st.DaemonOffers) == 0 && len(st.Daemons) > 0 && len(st.Daemons) >= st.Rules.MaxDaemons
}

// Shown is the one-time scene this screen has drawn, for the app to mark
// seen.
func (m *Merge) Shown() []string {
	if m.drawn {
		return []string{introHook}
	}
	return nil
}

// bossDefeat is the defeat line of the boss whose clear owes the pending
// upgrade (boss.defeat.<key>), for the pick it earned; "" when the pick is
// owed to another door, or none is. Only a boss mid-boot leads to a pick:
// the boot's last one ends it, and the cleared screen has its line.
func bossDefeat(st *game.State) string {
	if !st.PendingUpgrade {
		return ""
	}
	a, ok := lastClear(st)
	if !ok || a.Door.Boss == "" {
		return ""
	}
	return fl.Story("boss.defeat."+a.Door.Boss, st.Seed^uint64(a.Closed.Unix()))
}

// sections are the window's rows in a column cw wide, centered in width:
// commit, upgrade (under defeat, a boss's line, when it is set), spend,
// daemon (under the intro block when intro is set; what is held and
// merge.full when every seat is taken), leave.
func (m *Merge) sections(width, cw int, cur mergeRow, intro bool, defeat string) []string {
	st := m.env.State
	opts := daemonOptions(st.DaemonOffers, st)
	var secs []string

	label := func(s string) string { return "  " + ui.Strong.Render(fmt.Sprintf("%-9s", s)) }
	// Between acts the fountain comes first: it has already poured.
	secs = append(secs, fountainRow(st, label)...)
	// The bytes at stake are plain: a crash loses them. The footer has the
	// c key, so the line does not repeat it (C2).
	if st.Uncommitted > 0 {
		secs = append(secs, label("commit")+units.Bytes(st.Uncommitted)+
			ui.Muted.Render(" uncommitted: a crash loses it, the bank keeps it"))
	} else {
		secs = append(secs, label("commit")+ui.Muted.Render("nothing uncommitted"+ui.G.Sep)+
			units.Bytes(st.Committed)+ui.Muted.Render(" committed this boot"+ui.G.Sep+"bank ")+units.Bytes(m.env.Meta.Bank))
	}
	secs = append(secs, mergeGoal(m.env)...) // apt's goal bar (debrief.go, RT3)

	if st.PendingUpgrade && len(st.UpgradeOffers) > 0 {
		note := ui.Muted.Render("a clear earned one; pick it before you leave")
		if n, o3 := owedPicks(st); n >= 2 || n == 1 && o3 {
			note = picksNote(n, o3) + ui.Muted.Render("; pick before you leave")
		}
		secs = append(secs, "")
		if defeat != "" {
			for _, l := range strings.Split(defeat, "\n") {
				secs = append(secs, "  "+ui.Voice.Render(ui.Muted.Render(ui.Clip(l, cw-2))))
			}
		}
		secs = append(secs, label("upgrade")+note)
		secs = append(secs, indent(4, m.up.list(cw-4, cur.kind == rowUpgrade))...)
	}

	if items := itemOptions(st); len(items) > 0 {
		c := -1
		if cur.kind == rowSpend {
			c = m.buy
		}
		note := "uncommitted bytes, for this boot only"
		if st.Rerolls > 0 {
			note += ui.G.Sep + fmt.Sprintf("%d shuf held", st.Rerolls)
		}
		if st.AtTime > 0 {
			note += ui.G.Sep + "next door +" + units.Clock(st.AtTime)
		}
		secs = append(secs, "", label("spend")+ui.Muted.Render(note))
		secs = append(secs, indent(4, optionList(items, c, cw-4))...)
	}

	if full := daemonsFull(st); len(st.DaemonOffers) > 0 || full {
		held := make([]string, len(st.Daemons))
		for i, d := range st.Daemons {
			held[i], _ = daemonText(d, st)
		}
		// What a daemon is, every time: the first one is picked before
		// anyone has said. A full window has nothing to pick, so it only
		// says what runs.
		note := fmt.Sprintf("pick one, it runs for the rest of the boot%sheld %d/%d", ui.G.Sep, len(st.Daemons), st.Rules.MaxDaemons)
		if full {
			note = fmt.Sprintf("held %d/%d", len(st.Daemons), st.Rules.MaxDaemons)
		}
		if len(held) > 0 {
			note += ": " + strings.Join(held, ", ")
		}
		c := -1
		if cur.kind == rowDaemon {
			c = m.dmn
		}
		secs = append(secs, "")
		if intro {
			for _, l := range fl.StoryBlock(introHook) {
				secs = append(secs, "  "+ui.Voice.Render(ui.Muted.Render(ui.Clip(l, cw-2))))
			}
		}
		secs = append(secs, label("daemon")+ui.Muted.Render(note))
		if !full {
			secs = append(secs, indent(4, optionList(opts, c, cw-4))...)
		} else if s := fl.Story(fullHook, st.Seed); s != "" {
			// The Hatter's "No room!", true this once (merge.full), where the
			// offers would be.
			secs = append(secs, "    "+ui.Voice.Render(ui.Muted.Render(ui.Clip(s, cw-4))))
		}
	}

	mark := "  "
	if cur.kind == rowLeave {
		mark = ui.Focus.Render(ui.G.Cursor) + " "
	}
	leave := mark + ui.Strong.Render("leave") + ui.Muted.Render(" "+ui.G.Arrow+" "+m.next())
	return indent((width-cw)/2, append(secs, "", leave))
}

// onSale lists the items this boot's merge windows sell.
func onSale(st *game.State) []game.Item {
	var out []game.Item
	for _, it := range game.AllItems {
		if st.Price(it) > 0 {
			out = append(out, it)
		}
	}
	return out
}

// itemOptions describes the items on sale with their prices, and why one
// cannot be bought right now.
func itemOptions(st *game.State) []option {
	var out []option
	for _, it := range onSale(st) {
		name, desc := itemText(it, st)
		desc += ui.G.Sep + units.Bytes(st.Price(it))
		switch {
		case it == game.Fsck && st.Cycles >= st.MaxCycles:
			desc += ui.G.Sep + fmt.Sprintf("cycles full at %d", st.MaxCycles)
		case st.Uncommitted < st.Price(it):
			desc += ui.G.Sep + units.Bytes(st.Price(it)-st.Uncommitted) + " short"
		}
		out = append(out, option{name: name, desc: desc})
	}
	return out
}

// next names what leaving the merge window leads to.
func (m *Merge) next() string {
	st := m.env.State
	b, _ := position(st)
	if b >= len(st.Biomes) {
		return "end of boot"
	}
	bs := st.Biomes[b]
	switch {
	case bs.Kind == rules.Boss:
		if n := biomeLabel(st, b); n != "boss" {
			return "boss: " + n
		}
		return "boss"
	case bs.Kind == rules.Choice && bs.Topic == "":
		return fmt.Sprintf("biome %d: route choice", b+1)
	}
	return fmt.Sprintf("biome %d: %s", b+1, biomeLabel(st, b))
}
