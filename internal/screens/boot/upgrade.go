package boot

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// option is one pickable upgrade or daemon as the player sees it.
type option struct{ name, desc string }

func upgradeOptions(us []game.Upgrade, st *game.State) []option {
	out := make([]option, len(us))
	for i, u := range us {
		out[i].name, out[i].desc = upgradeText(u, st)
		out[i].desc += rarityTag(game.RarityOf(u), st)
	}
	return out
}

// rarityTag names a perk's rarity on its card under the roster, honest
// about what is rare: " · rare". A common one says nothing.
func rarityTag(r game.Rarity, st *game.State) string {
	if r == game.Common || st.Rules.Perk.Pool == 0 {
		return ""
	}
	return ui.G.Sep + r.String()
}

func daemonOptions(ds []game.Daemon, st *game.State) []option {
	out := make([]option, len(ds))
	for i, d := range ds {
		out[i].name, out[i].desc = daemonText(d, st)
		out[i].desc += rarityTag(game.RarityOf(d), st)
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
		b := []string{ui.Strong.Render(o.name), ""}
		for _, l := range ui.WrapN(o.desc, cw-4, 3) {
			b = append(b, ui.Muted.Render(l))
		}
		bodies[i] = b
		h = max(h, len(b))
	}
	blocks := make([][]string, n)
	for i := range opts {
		f, num := ui.FrameIdle, ui.Muted.Render(fmt.Sprint(i+1))
		if i == cursor {
			f = ui.FrameSelected
			num = f.Render(fmt.Sprintf("%s %d", ui.G.Cursor, i+1))
		}
		blocks[i] = ui.Frame(f, cw, num, "", ui.PadLines(bodies[i], h))
	}
	return ui.JoinH(2, blocks...)
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

// optionList draws options one per line (C4): a Focus cursor, the names
// padded to the longest and Strong under the cursor, the descriptions
// Muted. cursor < 0 hides the cursor, for a list that does not have focus.
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
			mark = ui.Focus.Render(ui.G.Cursor) + " "
			name = ui.Strong.Render(name)
		}
		out[i] = ui.Clip(mark+ui.Fit(name, nameW)+"  "+ui.Muted.Render(o.desc), width)
	}
	return out
}

// picker is the pick-one-of-three after a clear. The upgrade screen moves
// its cursor with ←/→ or 1-9; the merge window draws it as a list under a
// cursor of its own.
type picker struct {
	offers []game.Upgrade // st's, in its pool's order (State.UpgradePool), the skip last
	cursor int
	st     *game.State // the boot, whose numbers the descriptions quote
}

// set takes the offers from st in its pool's order, however the
// engine drew them, so a redraw after a pick leaves each card where it
// stood (UX J18). The cursor stays on the upgrade it was on while that is
// offered still, so a second enter takes the card the first did, and else
// stays in range.
func (p *picker) set(st *game.State) {
	was := p.selected()
	p.offers, p.st = slices.Clone(st.UpgradeOffers), st
	// The boot's pool order, the skip last.
	order := append(st.UpgradePool(), game.Skip)
	slices.SortStableFunc(p.offers, func(a, b game.Upgrade) int {
		return slices.Index(order, a) - slices.Index(order, b)
	})
	if i := slices.Index(p.offers, was); was != "" && i >= 0 {
		p.cursor = i
		return
	}
	p.cursor = move("", p.cursor, len(p.offers))
}

// selected is the upgrade under the cursor, "" with none on offer.
func (p *picker) selected() game.Upgrade {
	if p.cursor < 0 || p.cursor >= len(p.offers) {
		return ""
	}
	return p.offers[p.cursor]
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
	opts := upgradeOptions(p.offers, p.st)
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
	return optionList(upgradeOptions(p.offers, p.st), cur, width)
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
	head := u.env.header("upgrade", width, height)
	var block []string
	if n, ok := skipped(st); ok {
		// A skip owes its picks past it: nothing was cleared to get them.
		block = append(block, ui.Center(ui.Strong.Render("skip")+ui.Muted.Render(" · no room, no bytes · ")+
			ui.OK.Render(fmt.Sprintf("+%d", n))+" upgrade picks", width), "")
	} else if a, ok := clearHere(st); ok {
		p := u.env.problem(a.Door.Problem)
		// Green paints the glyph, the words stay plain (R2); -O3 is a
		// reward, so OK, and not bold (R3).
		line := ui.OK.Render(ui.G.Pass) + " AC  " + ui.Strong.Render(p.Title) +
			ui.Muted.Render(fmt.Sprintf("  in %s of %s", units.Clock(elapsed(a, u.env.Now)), units.Clock(a.Door.Timer)))
		if a.O3 {
			line += "  " + ui.OK.Render("-O3")
		}
		block = append(block, ui.Center(line, width),
			ui.Center(ui.OK.Render("+"+units.Bytes(st.Paid(a)))+ui.Muted.Render(" uncommitted"), width), "")
	}
	ask := ui.Strong.Render("Pick an upgrade.")
	switch n, x := owedPicks(st); {
	case n >= 2:
		ask = ui.Strong.Render("Pick an upgrade") + ui.Muted.Render(ui.G.Sep) + picksNote(n, x)
	case n == 1 && x.any():
		ask = ui.Strong.Render("Pick another upgrade") + ui.Muted.Render(ui.G.Sep) + picksNote(n, x)
	}
	block = append(block, ui.Center(ask, width), "")
	if len(u.picker.offers) == 0 {
		block = append(block, ui.Center(ui.Muted.Render("nothing on offer"), width))
	} else {
		block = append(block, centerBlock(u.picker.lines(width), width)...)
	}
	return ui.Layout(width, height, centered(head, block, height), ui.Footer(width, "←/→", "choose", "enter", "take upgrade"))
}

// owedPicks is how many upgrade picks the boot owes, and what earned the
// last clear's picks past its own (extras). The picks of one clear offer
// the same kind of cards, so without a count the second reads as the first
// one again, as if the keypress was missed.
func owedPicks(st *game.State) (n int, x extras) {
	n = st.UpgradePicks
	if n == 0 && st.PendingUpgrade {
		n = 1 // a state saved before UpgradePicks existed
	}
	a, ok := clearHere(st)
	if !ok {
		return n, x
	}
	x.o3 = a.O3
	x.vaults = openVaults(st)
	// A gated biome pays at its exit (rules.Biome.Picks), so its picks are
	// owed once its last room is cleared: in the merge window after it.
	if b := st.Biome; st.Phase == game.PhaseMerge && b < len(st.Rules.Biomes) && b < len(st.Biomes) {
		if rb := st.Rules.Biomes[b]; rb.Picks > 0 && !rb.Skip {
			x.gated, x.picks = cmp.Or(st.Biomes[b].Name, "the biome"), rb.Picks
		}
	}
	return n, x
}

// extras is what earned a clear's picks past its own: -O3, a gated
// biome's exit, by the biome's name (gated, "" for none) with its picks,
// and the vaults a boss's clear opened, by name ("the timed door"), a
// pick each.
type extras struct {
	o3     bool
	gated  string
	picks  int
	vaults []string
}

func (x extras) any() bool { return x.o3 || x.gated != "" || len(x.vaults) > 0 }

// picksNote counts the picks owed after a clear that earned extras: "2 to
// pick" on the first, "1 left" on the last, and what earned them: "(-O3
// earned an extra)", "(The Accumulator earned an extra)", "(-O3 and The
// Accumulator each earned an extra)", "(-O3 earned an extra, The
// Accumulator 2 extra)".
func picksNote(n int, x extras) string {
	s := fmt.Sprintf("%d to pick", n)
	if n < 2 {
		s = fmt.Sprintf("%d left", n)
	}
	type earner struct {
		who string
		n   int
	}
	var es []earner
	if x.o3 {
		es = append(es, earner{"-O3", 1})
	}
	if x.gated != "" {
		es = append(es, earner{x.gated, x.picks})
	}
	for _, v := range x.vaults {
		es = append(es, earner{v, 1})
	}
	extra := func(n int) string {
		if n == 1 {
			return "an extra"
		}
		return fmt.Sprintf("%d extra", n)
	}
	ones := true
	names := make([]string, len(es))
	for i, e := range es {
		ones = ones && e.n == 1
		names[i] = e.who
	}
	switch {
	case len(es) == 1:
		s += " (" + es[0].who + " earned " + extra(es[0].n) + ")"
	case len(es) > 1 && ones:
		s += " (" + strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " each earned an extra)"
	case len(es) > 1:
		s += " (" + es[0].who + " earned " + extra(es[0].n)
		for _, e := range es[1:] {
			s += ", " + e.who + " " + extra(e.n)
		}
		s += ")"
	}
	return ui.Muted.Render(s)
}

// clearHere is the last clear when the boot still stands in its biome: the
// clear whose picks are owed. Past a skip the picks are the skip's.
func clearHere(st *game.State) (game.Attempt, bool) {
	ss := steps(st)
	for i := len(ss) - 1; i >= 0; i-- {
		if ss[i].Attempt.Outcome == game.Cleared {
			return ss[i].Attempt, ss[i].Biome == st.Biome
		}
	}
	return game.Attempt{}, false
}

// skipped is the picks of the skip the boot just took (map.md §(c)): it
// stands at the doors of the tier past a skipped one, none opened yet.
func skipped(st *game.State) (picks int, ok bool) {
	b := st.Biome - 1
	if b < 0 || b >= len(st.Biomes) || st.Room > 0 {
		return 0, false
	}
	n, ok := st.Rules.Node(st.Biomes[b].Node)
	if !ok || !n.Skip {
		return 0, false
	}
	for _, s := range steps(st) {
		if s.Biome == st.Biome {
			return 0, false
		}
	}
	return n.Picks, true
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
