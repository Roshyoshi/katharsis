package boot

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
)

// Merge is the merge window between biomes: commit the uncommitted bytes
// (c), take a pending upgrade, pick a daemon, then leave. One cursor walks
// every pickable row and the leave row; enter acts on the row under it.
type Merge struct {
	env Env
	up  picker
	dmn int // cursor within the daemon offers
	cur int // index into rows()
}

type rowKind int

const (
	rowUpgrade rowKind = iota
	rowDaemon
	rowLeave
)

type mergeRow struct {
	kind rowKind
	i    int
}

func NewMerge(env Env) *Merge {
	m := &Merge{env: env}
	m.up.set(env.State)
	return m
}

// Refresh swaps in new state and clock. When the row under the cursor is
// gone (its pick was taken), the cursor moves to the next section.
func (m *Merge) Refresh(env Env) {
	old := m.row()
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
// daemon on offer is pickable.
func (m *Merge) rows() []mergeRow {
	st := m.env.State
	var rs []mergeRow
	if st.PendingUpgrade {
		for i := range st.UpgradeOffers {
			rs = append(rs, mergeRow{rowUpgrade, i})
		}
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
	switch r := m.row(); r.kind {
	case rowUpgrade:
		m.up.cursor = r.i
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
	case "prev", "next":
		m.cur = move(key, min(m.cur, len(m.rows())-1), len(m.rows()))
		m.sync()
	case "enter":
		switch r := m.row(); r.kind {
		case rowUpgrade:
			return m, screens.Emit(game.PickUpgrade{Upgrade: st.UpgradeOffers[r.i]})
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
	head := m.env.header("merge window", width)
	// The sections read as one form, so they share a column; on a wide
	// terminal it sits under the centered map instead of hugging the left.
	// The column grows past 76 cells when an option needs it, so a long
	// description is cut only when the terminal itself is too narrow.
	opts := daemonOptions(st.DaemonOffers, st.Rules)
	need := listWidth(opts)
	if st.PendingUpgrade {
		need = max(need, listWidth(upgradeOptions(st.UpgradeOffers, st.Rules)))
	}
	cw := min(width, max(76, 4+need))
	var secs []string

	label := func(s string) string { return "  " + sBold.Render(fmt.Sprintf("%-9s", s)) }
	if st.Uncommitted > 0 {
		secs = append(secs, label("commit")+sGreen.Bold(true).Render(formatBytes(st.Uncommitted))+
			sFaint.Render(" uncommitted, lost if you crash  ")+hint("c", "commit"))
	} else {
		secs = append(secs, label("commit")+sFaint.Render("nothing uncommitted · ")+
			formatBytes(st.Committed)+sFaint.Render(" committed this boot · bank ")+formatBytes(m.env.Meta.Bank))
	}

	if st.PendingUpgrade && len(st.UpgradeOffers) > 0 {
		note := sFaint.Render("a clear earned one; pick it before you leave")
		if n, o3 := owedPicks(st); n >= 2 || n == 1 && o3 {
			note = picksNote(n, o3) + sFaint.Render("; pick before you leave")
		}
		secs = append(secs, "", label("upgrade")+note)
		secs = append(secs, indent(4, m.up.list(cw-4, cur.kind == rowUpgrade))...)
	}

	if len(st.DaemonOffers) > 0 {
		held := make([]string, len(st.Daemons))
		for i, d := range st.Daemons {
			held[i], _ = daemonText(d, st.Rules)
		}
		note := fmt.Sprintf("pick one · held %d/%d", len(st.Daemons), st.Rules.MaxDaemons)
		if len(held) > 0 {
			note += ": " + strings.Join(held, ", ")
		}
		c := -1
		if cur.kind == rowDaemon {
			c = m.dmn
		}
		secs = append(secs, "", label("daemon")+sFaint.Render(note))
		secs = append(secs, indent(4, optionList(opts, c, cw-4))...)
	}

	leave := "  " + sBold.Render("leave") + sFaint.Render(" → "+m.next())
	if cur.kind == rowLeave {
		leave = sCyan.Bold(true).Render("▸ leave") + sFaint.Render(" → "+m.next())
	}
	secs = indent((width-cw)/2, append(secs, "", leave))

	block := secs
	if mp := centerBlock(renderMap(st, width), width); len(head)+2+len(mp)+1+len(secs) <= height-1 {
		block = append(append(mp, ""), secs...)
	}
	footer := hint("↑/↓", "move", "enter", "pick or leave", "c", "commit")
	return layout(width, height, centered(head, block, height), footer)
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
