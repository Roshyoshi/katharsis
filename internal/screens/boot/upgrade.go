package boot

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/units"
)

// option is one pickable upgrade or daemon as the player sees it.
type option struct{ name, desc string }

func upgradeOptions(us []game.Upgrade, r rules.Rules) []option {
	out := make([]option, len(us))
	for i, u := range us {
		out[i].name, out[i].desc = upgradeText(u, r)
	}
	return out
}

func daemonOptions(ds []game.Daemon, st *game.State) []option {
	out := make([]option, len(ds))
	for i, d := range ds {
		out[i].name, out[i].desc = daemonText(d, st)
	}
	return out
}

// optionCards draws options as cards side by side, or nil when they don't
// fit width.
func optionCards(opts []option, cursor, width int) []string {
	n := len(opts)
	if n == 0 {
		return nil
	}
	cw := min((width-2*(n-1))/n, 30)
	if cw < 20 {
		return nil
	}
	bodies := make([][]string, n)
	h := 0
	for i, o := range opts {
		b := []string{sBold.Render(o.name), ""}
		for _, l := range wrapN(o.desc, cw-4, 3) {
			b = append(b, sFaint.Render(l))
		}
		bodies[i] = b
		h = max(h, len(b))
	}
	blocks := make([][]string, n)
	for i := range opts {
		f, num := frameIdle, sFaint.Render(fmt.Sprint(i+1))
		if i == cursor {
			f, num = frameSelected, sCyan.Bold(true).Render(fmt.Sprintf("▸ %d", i+1))
		}
		blocks[i] = box(f, cw, num, "", padLines(bodies[i], h))
	}
	return joinH(2, blocks...)
}

// listWidth is how wide optionList draws opts when nothing is cut: the
// cursor mark, the names padded to the longest, a gap, the description.
func listWidth(opts []option) int {
	if len(opts) == 0 {
		return 0
	}
	nameW, descW := 0, 0
	for _, o := range opts {
		nameW = max(nameW, widthOf(o.name))
		descW = max(descW, widthOf(o.desc))
	}
	return 2 + nameW + 2 + descW
}

// optionList draws options one per line. cursor < 0 hides the cursor, for
// a list that does not have focus.
func optionList(opts []option, cursor, width int) []string {
	nameW := 0
	for _, o := range opts {
		nameW = max(nameW, widthOf(o.name))
	}
	out := make([]string, len(opts))
	for i, o := range opts {
		mark := "  "
		name := o.name
		if i == cursor {
			mark = sCyan.Bold(true).Render("▸ ")
			name = sCyan.Bold(true).Render(name)
		}
		out[i] = clip(mark+fit(name, nameW)+"  "+sFaint.Render(o.desc), width)
	}
	return out
}

// picker is the pick-one-of-three after a clear. The upgrade screen moves
// its cursor with ←/→ or 1-9; the merge window draws it as a list under a
// cursor of its own.
type picker struct {
	offers []game.Upgrade
	cursor int
	rules  rules.Rules // the boot's rules, which the descriptions quote
}

// set takes the offers and rules from st, keeping the cursor in range.
func (p *picker) set(st *game.State) {
	p.offers, p.rules = st.UpgradeOffers, st.Rules
	p.cursor = move("", p.cursor, len(p.offers))
}

func (p *picker) update(msg tea.Msg) tea.Cmd {
	key, ok := keyName(msg)
	if !ok || len(p.offers) == 0 {
		return nil
	}
	if key == "enter" {
		return screens.Emit(game.PickUpgrade{Upgrade: p.offers[p.cursor]})
	}
	p.cursor = move(key, p.cursor, len(p.offers))
	return nil
}

// lines draws the offers as cards when they fit width, else as a list.
func (p *picker) lines(width int) []string {
	opts := upgradeOptions(p.offers, p.rules)
	if ls := optionCards(opts, p.cursor, width); ls != nil {
		return ls
	}
	return optionList(opts, p.cursor, width)
}

// list draws the offers one per line; active shows the cursor.
func (p *picker) list(width int, active bool) []string {
	cur := -1
	if active {
		cur = p.cursor
	}
	return optionList(upgradeOptions(p.offers, p.rules), cur, width)
}

// UpgradeScreen is the upgrade pick right after a cleared door, outside a
// merge window. It shows what was cleared, then the picker.
type UpgradeScreen struct {
	env    Env
	picker picker
}

func NewUpgradeScreen(env Env) *UpgradeScreen {
	u := &UpgradeScreen{env: env}
	u.picker.set(env.State)
	return u
}

// Refresh swaps in new state and clock, keeping the cursor.
func (u *UpgradeScreen) Refresh(env Env) {
	u.env = env
	u.picker.set(env.State)
}

func (u *UpgradeScreen) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	return u, u.picker.update(msg)
}

func (u *UpgradeScreen) View(width, height int) string {
	st := u.env.State
	head := u.env.header("upgrade", width)
	var block []string
	if a, ok := lastClear(st); ok {
		p := u.env.problem(a.Door.Problem)
		line := sGreen.Bold(true).Render("✓ AC") + "  " + sBold.Render(p.Title) +
			sFaint.Render(fmt.Sprintf("  in %s of %s", units.Clock(elapsed(a, u.env.Now)), units.Clock(a.Door.Timer)))
		if a.O3 {
			line += "  " + sYellow.Bold(true).Render("-O3")
		}
		block = append(block, center(line, width),
			center(sGreen.Render("+"+units.Bytes(a.Door.Bytes))+sFaint.Render(" uncommitted"), width), "")
	}
	ask := sBold.Render("Pick an upgrade.")
	switch n, o3 := owedPicks(st); {
	case n >= 2:
		ask = sBold.Render("Pick an upgrade") + sFaint.Render(" · ") + picksNote(n, o3)
	case n == 1 && o3:
		ask = sBold.Render("Pick another upgrade") + sFaint.Render(" · ") + picksNote(n, o3)
	}
	block = append(block, center(ask, width), "")
	if len(u.picker.offers) == 0 {
		block = append(block, center(sFaint.Render("nothing on offer"), width))
	} else {
		block = append(block, centerBlock(u.picker.lines(width), width)...)
	}
	return layout(width, height, centered(head, block, height), hint("←/→", "choose", "enter", "take upgrade"))
}

// owedPicks is how many upgrade picks the boot owes, and whether the last
// clear was an -O3 one, which owes two. The two picks of an -O3 clear
// offer the same kind of cards, so without a count the second reads as
// the first one again, as if the keypress was missed.
func owedPicks(st *game.State) (n int, o3 bool) {
	n = st.UpgradePicks
	if n == 0 && st.PendingUpgrade {
		n = 1 // a state saved before UpgradePicks existed
	}
	a, ok := lastClear(st)
	return n, ok && a.O3
}

// picksNote counts the picks owed after an -O3 clear: "2 to pick" on the
// first, "1 left" on the second.
func picksNote(n int, o3 bool) string {
	s := fmt.Sprintf("%d to pick", n)
	if n < 2 {
		s = fmt.Sprintf("%d left", n)
	}
	if o3 {
		s += " (-O3 earned an extra)"
	}
	return sFaint.Render(s)
}

// lastClear is the most recent cleared attempt of the boot.
func lastClear(st *game.State) (game.Attempt, bool) {
	ss := steps(st)
	for i := len(ss) - 1; i >= 0; i-- {
		if ss[i].Attempt.Outcome == game.Cleared {
			return ss[i].Attempt, true
		}
	}
	return game.Attempt{}, false
}
