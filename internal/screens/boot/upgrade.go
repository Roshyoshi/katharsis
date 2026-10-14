package boot

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
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

func daemonOptions(ds []game.Daemon, r rules.Rules) []option {
	out := make([]option, len(ds))
	for i, d := range ds {
		out[i].name, out[i].desc = daemonText(d, r)
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
	row := joinH(2, blocks...)
	return indent(max((width-widthOf(row[0]))/2, 0), row)
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

// UpgradePicker is the pick-one-of-three after a clear. It works on its
// own (see UpgradeScreen) and inside the merge window: ←/→ or 1-9 move,
// enter emits game.PickUpgrade.
type UpgradePicker struct {
	Offers []game.Upgrade
	Cursor int
	Rules  rules.Rules // the boot's rules, which the descriptions quote
}

func NewUpgradePicker(offers []game.Upgrade, r rules.Rules) UpgradePicker {
	return UpgradePicker{Offers: offers, Rules: r}
}

// SetOffers replaces the offers, keeping the cursor in range.
func (p *UpgradePicker) SetOffers(offers []game.Upgrade) {
	p.Offers = offers
	p.Cursor = move("", p.Cursor, len(offers))
}

func (p UpgradePicker) Update(msg tea.Msg) (UpgradePicker, tea.Cmd) {
	key, ok := keyName(msg)
	if !ok || len(p.Offers) == 0 {
		return p, nil
	}
	if key == "enter" {
		return p, screens.Emit(game.PickUpgrade{Upgrade: p.Offers[p.Cursor]})
	}
	p.Cursor = move(key, p.Cursor, len(p.Offers))
	return p, nil
}

// View draws the offers as cards when they fit width, else as a list.
func (p UpgradePicker) View(width int) string {
	opts := upgradeOptions(p.Offers, p.Rules)
	ls := optionCards(opts, p.Cursor, width)
	if ls == nil {
		ls = optionList(opts, p.Cursor, width)
	}
	return joinLines(ls)
}

// List draws the offers one per line; active shows the cursor.
func (p UpgradePicker) List(width int, active bool) []string {
	cur := -1
	if active {
		cur = p.Cursor
	}
	return optionList(upgradeOptions(p.Offers, p.Rules), cur, width)
}

// UpgradeScreen is the upgrade pick right after a cleared door, outside a
// merge window. It shows what was cleared, then the picker.
type UpgradeScreen struct {
	env    Env
	picker UpgradePicker
}

func NewUpgradeScreen(env Env) *UpgradeScreen {
	u := &UpgradeScreen{env: env}
	if env.State != nil {
		u.picker = NewUpgradePicker(env.State.UpgradeOffers, env.State.Rules)
	}
	return u
}

// Refresh swaps in new state and clock, keeping the cursor.
func (u *UpgradeScreen) Refresh(env Env) {
	u.env = env
	if env.State != nil {
		u.picker.Rules = env.State.Rules
		u.picker.SetOffers(env.State.UpgradeOffers)
	}
}

func (u *UpgradeScreen) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	var cmd tea.Cmd
	u.picker, cmd = u.picker.Update(msg)
	return u, cmd
}

func (u *UpgradeScreen) View(width, height int) string {
	st := u.env.State
	if st == nil {
		return noBoot(width, height)
	}
	head := u.env.header("upgrade", width)
	var block []string
	if a, ok := lastClear(st); ok {
		p := u.env.problem(a.Door.Problem)
		took := elapsed(a, u.env.Now)
		line := sGreen.Bold(true).Render("✓ AC") + "  " + sBold.Render(p.Title) +
			sFaint.Render(fmt.Sprintf("  in %s of %s", fmtClock(took), fmtClock(a.Door.Timer)))
		if f := st.Rules.O3Frac; f > 0 && a.Door.Timer > 0 && float64(took) < f*float64(a.Door.Timer) {
			line += "  " + sYellow.Bold(true).Render("-O3")
		}
		block = append(block, center(line, width),
			center(sGreen.Render("+"+FormatBytes(a.Door.Bytes))+sFaint.Render(" uncommitted"), width), "")
	}
	block = append(block, center(sBold.Render("Pick an upgrade."), width), "")
	if len(u.picker.Offers) == 0 {
		block = append(block, center(sFaint.Render("nothing on offer"), width))
	} else {
		block = append(block, centerBlock(u.picker.View(width), width)...)
	}
	return layout(width, height, centered(head, block, height), hint("←/→", "choose", "enter", "take upgrade"))
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
