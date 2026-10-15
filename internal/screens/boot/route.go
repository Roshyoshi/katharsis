package boot

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Route is the fork at the entrance of a tier: one card per way on. On a
// map (map.md §(g)) a way on is an exit of the node behind (BiomeState.
// Exits): its biome or boss, the band its doors aim at, what a fail there
// burns, the picks owed at its end and the bosses it leads to. A locked
// exit stays on screen, choosable by no key but t, which opens RTFM at the
// module its capability comes from. On a linear boot a way on is a topic
// on offer (Offers). enter emits game.ChooseRoute.
type Route struct {
	env Env
	sel int
}

func NewRoute(env Env) *Route { return &Route{env: env} }

// Refresh swaps in new state and clock, keeping the selection.
func (r *Route) Refresh(env Env) {
	r.env = env
	r.sel = move("", r.sel, r.count())
}

func (r *Route) offers() []string {
	st := r.env.State
	return st.Biomes[st.Biome].Offers
}

// exits are the tier's exits on a map, nil on a linear boot.
func (r *Route) exits() []game.Exit {
	st := r.env.State
	if !onMap(st) {
		return nil
	}
	return st.Biomes[st.Biome].Exits
}

func (r *Route) count() int {
	if ex := r.exits(); ex != nil {
		return len(ex)
	}
	return len(r.offers())
}

func (r *Route) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	key, ok := keyName(msg)
	if !ok {
		return r, nil
	}
	n := r.count()
	switch key {
	case "enter":
		if n == 0 {
			return r, nil
		}
		if ex := r.exits(); ex != nil && ex[r.sel].Locked != "" {
			return r, nil // the card says what opens it
		}
		return r, screens.Emit(game.ChooseRoute{Index: r.sel})
	case "t":
		if ex := r.exits(); ex != nil && CapLocked(r.env.State, ex[r.sel]) {
			return r, screens.Emit(screens.OpenLearn{Topic: ex[r.sel].Topic})
		}
		return r, nil
	}
	r.sel = move(key, r.sel, n)
	return r, nil
}

// CapLocked reports whether exit e is locked by a capability the boot
// began without (State.Caps; its package, if any, not installed either),
// which an RTFM trial earns: not by a prestige, nor the pool.
func CapLocked(st *game.State, e game.Exit) bool {
	g := e.Gate
	return e.Locked != "" && g.Prestige <= st.Prestige && g.Capability != "" && !slices.Contains(st.Caps, g.Capability) &&
		(g.Package == "" || !slices.Contains(st.Packages, g.Package))
}

func (r *Route) View(width, height int) string {
	st := r.env.State
	exits := r.exits()
	n := r.count()
	where := fmt.Sprintf("route"+ui.G.Sep+"biome %d/%d", st.Biome+1, len(st.Biomes))
	pick := ui.Strong.Render("Pick a route.") + ui.Muted.Render(" The topic you choose sets every door in this biome.")
	if exits != nil {
		where = "route" + ui.G.Sep + actName(st.ActOf(st.Biome))
		pick = ui.Strong.Render("Pick an exit.") + ui.Muted.Render(" It sets the next door and the boss behind it.")
	}
	head := r.env.header(where, width, height)
	keys := []string{"←/→", "choose", "enter", "take route"}
	if exits != nil {
		keys[3] = "take exit"
	}
	if exits != nil && n > 0 && exits[r.sel].Locked != "" {
		keys = []string{"←/→", "choose"}
		if CapLocked(st, exits[r.sel]) {
			keys = append(keys, "t", "RTFM")
		}
	}
	footer := ui.Footer(width, append(keys, "m", "map")...)
	block := []string{ui.Center(pick, width), ""}
	room := height - len(head) - 1
	if n == 0 {
		block = append(block, "  "+ui.Muted.Render("no routes on offer"))
		return ui.Layout(width, height, append(append(head, ""), block...), footer)
	}

	cards := make([]card, n)
	for i := range cards {
		if exits != nil {
			cards[i] = r.exitCard(exits[i])
		} else {
			cards[i] = r.topicCard(r.offers()[i])
		}
	}
	var rows []string
	if cw := (width - 2*(n-1)) / n; cw >= 26 {
		cw = min(cw, 44)
		tops, bottoms := make([][]string, n), make([][]string, n)
		for i, c := range cards {
			tops[i], bottoms[i] = c.wide(cw - 4)
		}
		blocks := make([][]string, n)
		for i, body := range alignBodies(tops, bottoms) {
			blocks[i] = r.frame(i, cards[i], cw, body)
		}
		rows = centerBlock(ui.JoinH(2, blocks...), width)
	}
	if rows == nil || len(block)+len(rows) > room {
		// Only the wide cards say a fail's price: it is the tier's, the
		// same behind every exit, so the narrow layouts say it once.
		if f := tierFail(cards); f != "" {
			block = []string{block[0], ui.Center(f, width), ""}
		}
		rows = nil
		w := min(width, 72)
		for i, c := range cards {
			rows = append(rows, indent((width-w)/2, r.frame(i, c, w, c.stacked(w-4)))...)
		}
	}
	if len(block)+len(rows) > room {
		rows = r.compact(cards, width)
	}
	block = append(block, rows...)
	if m := centerBlock(renderMap(st, width), width); len(block)+2+len(m) <= room-1 {
		block = append(block, "", ui.Center(ui.Muted.Render("the boot so far"), width))
		block = append(block, m...)
	}
	return ui.Layout(width, height, centered(head, block, height), footer)
}

func (r *Route) frame(i int, c card, w int, body []string) []string {
	f, title := ui.FrameIdle, ui.Muted.Render(fmt.Sprint(i+1))
	if c.boss {
		f = ui.FrameBoss
	}
	if i == r.sel {
		f = ui.FrameSelected
		if c.boss {
			f = ui.FrameBossSelected
		}
		title = f.Render(fmt.Sprintf("%s %d", ui.G.Cursor, i+1))
	}
	return ui.Frame(f, w, title, "", body)
}

// card is what a route card says about one way on.
type card struct {
	name, sub, blurb string
	rooms            string   // "2 rooms · ⏱ ~30:00"
	band             string   // "◆1050–1250 · aim 70% clear", "" without rules or a door
	mine             string   // "your dp rating 1120", "" unless doors aim by it
	fail             string   // "a fail −12 ♥", "a fail ends the boot"
	picks            string   // "+1 upgrade pick"
	behind           []string // the bosses it leads to: "→ ⚔ Heisenbug"
	locked           []string // why it cannot be taken and what opens it; nil when open
	// rule and ends are an ending's (the Trial): its door's rules, "☠ cursed
	// · one shot" and "↩ retreat: a new cursed door", and what its clear
	// does, "ends the boot here", "no prestige unlock". The card says what
	// the route costs (tone rule 5).
	rule []string
	ends []string
	boss bool
}

// topicCard is a linear boot's card: a topic biome on offer.
func (r *Route) topicCard(topic string) card {
	st := r.env.State
	c := card{name: topicName(topic), sub: topic, blurb: fl.BiomeBlurb(topic)}
	c.rooms = units.Plural(st.Biomes[st.Biome].Rooms, "room") + r.timer(topic)
	c.band, c.mine = r.band(topic)
	return c
}

// exitCard is a map exit's card (map.md §(g)): the node, its doors' band,
// the price of a fail there, its picks, the bosses behind it, and for a
// locked exit what opens it in place of the band and the price.
func (r *Route) exitCard(e game.Exit) card {
	st := r.env.State
	// The node's own blurb (Kernel Space's), else its topic's; a mixed
	// node has no topic blurb of its own, since mixed's is Userland's.
	topic := e.Topic
	if topic == game.Mixed {
		topic = ""
	}
	c := card{name: e.Name, sub: e.Topic, blurb: fl.NodeBlurb(e.Node, topic)}
	switch {
	case e.Skip:
		c.name, c.sub, c.blurb = "skip", "no room", ""
		c.rooms = "no room, no bytes"
	case e.Hidden && e.Locked != "":
		// A secret exit shows locked only once its hint was met
		// (Meta.Capped), and never by name: ⚔ ??? (achievements.md (f)).
		c.boss = true
		c.name, c.sub, c.blurb = ui.G.Boss+" ???", "", ""
		c.locked = r.lockLines(e)
		return c
	case e.Ending != "":
		c.boss = true
		c.name, c.sub, c.blurb = ui.G.Boss+" "+bossLabel(e.Boss), courtOfHearts, fl.BossBlurb(e.Boss)
	case e.Boss != "":
		c.boss = true
		c.name, c.sub, c.blurb = ui.G.Boss+" "+bossLabel(e.Boss), actName(st.ActOf(st.Biome))+" boss", fl.BossBlurb(e.Boss)
	}
	if c.name == "" {
		c.name = e.Node
	}
	if c.sub == game.Mixed {
		c.sub = "any topic"
	}
	if !e.Skip {
		// The engine's clock for this exit, its boss's twist in it; an exit
		// saved before it had one shows the tier's.
		c.rooms = units.Plural(e.Rooms, "room") + r.timer(e.Topic)
		if e.Timer > 0 {
			c.rooms = units.Plural(e.Rooms, "room") + clockText(e.Timer)
		}
		c.band, c.mine = r.band(e.Topic)
		c.fail = failText(st)
	}
	if e.Ending != "" {
		c.fail, c.rule, c.ends = endingLines(st, e)
	}
	if e.Picks > 0 {
		word := " upgrade pick"
		if e.Picks != 1 {
			word += "s"
		}
		c.picks = ui.OK.Render(fmt.Sprintf("+%d", e.Picks)) + word
	}
	for _, b := range e.Bosses {
		c.behind = append(c.behind, ui.Muted.Render(ui.G.Arrow+" ")+ui.Special.Render(ui.G.Boss+" "+bossLabel(b)))
	}
	if e.Skip && len(e.Bosses) == 0 {
		for _, n := range st.Rules.Nodes {
			if n.Tier == st.Biome+1 && n.Boss != "" && !n.Hidden {
				c.behind = append(c.behind, ui.Muted.Render(ui.G.Arrow+" ")+ui.Special.Render(ui.G.Boss+" "+bossLabel(n.Boss)))
			}
		}
	}
	c.locked = r.lockLines(e)
	return c
}

// lockLines say why a locked exit cannot be taken and what opens it: the
// capability and the RTFM module that grants it (route.locked), or the
// ring it opens from (route.locked.prestige), or the pool. nil when open.
func (r *Route) lockLines(e game.Exit) []string {
	st := r.env.State
	switch {
	case e.Locked == "":
		return nil
	case e.Gate.Prestige > st.Prestige:
		ring, _ := flavor.Ring(e.Gate.Prestige)
		out := []string{ui.Muted.Render(ui.G.CapLocked+" locked: ") + prestigeLabel(e.Gate.Prestige)}
		if s := fl.Story("route.locked.prestige", 0); s != "" {
			out = append(out, ui.Voice.Render(flavor.Fill(s, "ring", strconv.Itoa(ring))))
		}
		return out
	case e.Gate.Full && e.Locked == game.LockedFull:
		// The Trial's gate: the pool at its cap, Rule Forty-two read
		// backwards (route.locked.full alludes to it).
		out := []string{ui.Muted.Render(ui.G.CapLocked+" locked: ") + game.LockedFull}
		if s := fl.Story("route.locked.full", 0); s != "" {
			out = append(out, ui.Voice.Render(s))
		}
		return out
	case CapLocked(st, e):
		lock := e.Gate.Capability
		if e.Gate.Package != "" {
			lock += " or apt " + e.Gate.Package
		}
		module := e.Gate.Capability
		if r.env.Module != nil {
			if m := r.env.Module(e.Topic); m != "" {
				module = "RTFM" + ui.G.Crumb + m
			}
		}
		out := []string{ui.Muted.Render(ui.G.CapLocked+" locked: ") + lock}
		if s := fl.Story("route.locked", 0); s != "" {
			out = append(out, ui.Voice.Render(flavor.Fill(s, "module", module)))
		}
		return out
	}
	return []string{ui.Muted.Render(ui.G.CapLocked + " locked: too few problems near you")}
}

// failText is what a timeout or :q! burns in the tier being entered: the
// tier's price and what happens to the room, or the boot for a one-shot
// boss.
func failText(st *game.State) string {
	c, ok := st.FailCost(game.TimedOut)
	if c == 0 {
		return ui.Danger.Render("a fail ends the boot")
	}
	then := "the room drawn again"
	switch {
	case !ok:
		return ui.Danger.Render(fmt.Sprintf("a fail −%d %s ends the boot", c, ui.G.Life))
	case st.Biomes[st.Biome].Kind == rules.Boss:
		then = "a new boss door"
	case st.Rules.FailMoveOn:
		then = "the room lost"
	}
	return fmt.Sprintf("a fail −%d %s, %s", c, ui.Life.Render(ui.G.Life), then)
}

// timer is " · ⏱ ~30:00", the doors' own timer with Overclock and the rest
// applied (UX J24): "~", since a fitted timer moves with each door's
// rating. "" when the tier has no timer.
func (r *Route) timer(topic string) string {
	st := r.env.State
	if rb, ok := ruleBiome(st, st.Biome); !ok || rb.Timer.Duration <= 0 {
		return ""
	}
	return clockText(st.RouteTimer(&r.env.Meta, topic))
}

// clockText is a card's " · ⏱ ~30:00".
func clockText(d time.Duration) string {
	return ui.Muted.Render(ui.G.Sep) + ui.G.Timer + "~" + units.Clock(d)
}

// band previews the rating the engine aims topic's doors at, widened ±100
// and rounded to 50 (the pool picks the actual doors), with the aim: the
// biome's, or what an easing raised it to (RT8). mine is the player's own
// rating on the topic when doors aim by it.
func (r *Route) band(topic string) (band, mine string) {
	st := r.env.State
	if _, ok := ruleBiome(st, st.Biome); !ok {
		return "", ""
	}
	aim := st.AimClear(st.Biome)
	if aim <= 0 || aim >= 1 {
		return "", ""
	}
	meta := &r.env.Meta
	mid := int(math.Round(st.Target(meta, topic)/50) * 50)
	band = ui.RatingBand(mid-100, mid+100) +
		ui.Muted.Render(fmt.Sprintf("%saim %d%% clear", ui.G.Sep, int(math.Round(aim*100))))
	// The engine aims with the topic's own rating only once it has enough
	// attempts behind it; a mixed door always gets the overall one.
	if own := game.PlayerRating(&st.Rules, meta, topic); own != game.PlayerRating(&st.Rules, meta, game.Mixed) {
		// A player's rating is a bare numeral; the ◆ marks a door's.
		mine = ui.Muted.Render("your "+topic+" rating ") + ui.Rank(own).Render(fmt.Sprint(int(math.Round(own))))
	}
	return band, mine
}

// wide is a side-by-side card in two halves: the name, what it is and the
// blurb, which wraps, then a blank line and the stats, or the lock.
func (c card) wide(cw int) (top, bottom []string) {
	name := ui.Strong.Render(c.name)
	if c.boss {
		name = ui.Special.Render(c.name)
	}
	top = []string{name}
	if c.sub != "" && c.sub != c.name {
		top = append(top, ui.Muted.Render(c.sub))
	}
	for _, l := range ui.WrapN(c.blurb, cw, 3) {
		top = append(top, ui.Voice.Render(l))
	}
	bottom = []string{"", c.rooms}
	if c.locked != nil {
		for _, l := range c.locked {
			bottom = append(bottom, ui.WrapN(l, cw, 3)...)
		}
	} else {
		for _, l := range append([]string{c.band, c.mine, c.fail}, c.rule...) {
			if l != "" {
				bottom = append(bottom, ui.Clip(l, cw))
			}
		}
	}
	if c.picks != "" {
		bottom = append(bottom, c.picks)
	}
	for _, b := range append(slices.Clone(c.behind), c.ends...) {
		bottom = append(bottom, ui.Clip(b, cw))
	}
	return top, bottom
}

// stacked is a full-width card: the name and band on one line, the blurb
// or the lock clipped to one, the rooms, picks and bosses behind on one.
func (c card) stacked(cw int) []string {
	name := ui.Strong.Render(c.name)
	if c.boss {
		name = ui.Special.Render(c.name)
	}
	if c.sub != "" && c.sub != c.name {
		name += ui.Muted.Render(ui.G.Sep + c.sub)
	}
	right, mid := c.band, ui.Voice.Render(c.blurb)
	if c.locked != nil {
		right, mid = c.locked[0], strings.Join(c.locked[1:], " ")
	}
	// An ending's costs take the blurb's line, and a line more, and the
	// bosses' place: the mechanic before the story.
	behind, more := strings.Join(c.behind, " "), []string(nil)
	if c.ends != nil {
		sep := ui.Muted.Render(ui.G.Sep)
		mid, more, behind = c.rule[0]+sep+c.fail, c.rule[1:], strings.Join(c.ends, sep)
	}
	out := []string{ui.Spread(name, right, cw)}
	if strings.TrimSpace(mid) != "" {
		out = append(out, ui.Clip(mid, cw))
	}
	for _, l := range more {
		out = append(out, ui.Clip(l, cw))
	}
	left := c.rooms
	if c.picks != "" {
		left += ui.Muted.Render(ui.G.Sep) + c.picks
	}
	return append(out, ui.Spread(left, cmp.Or(behind, c.mine), cw))
}

// tierFail is the price of a fail the cards carry, "" when none does (a
// linear route, a skip alone).
func tierFail(cards []card) string {
	for _, c := range cards {
		if c.fail != "" {
			return c.fail
		}
	}
	return ""
}

// compact is one line per card, for terminals too short for cards: the
// name cut before the band or the lock. Under them the selected card says
// the rest a choice turns on: its rooms and clock, the bosses behind it,
// and for a lock what opens it.
func (r *Route) compact(cards []card, width int) []string {
	out := r.compactRows(cards, width)
	c := cards[r.sel]
	out = append(out, "", ui.Spread("    "+c.rooms, cmp.Or(strings.Join(c.behind, " "), c.mine)+"    ", width))
	if len(c.locked) > 1 {
		out = append(out, indent(4, ui.WrapN(strings.Join(c.locked[1:], " "), width-8, 2))...)
	}
	return out
}

func (r *Route) compactRows(cards []card, width int) []string {
	var out []string
	for i, c := range cards {
		mark := "  "
		if i == r.sel {
			mark = ui.Focus.Render(ui.G.Cursor) + " "
		}
		head := mark + fmt.Sprintf("%d ", i+1)
		right := c.band
		if c.locked != nil {
			right = c.locked[0]
		}
		if c.ends != nil {
			right = c.fail
		}
		if c.picks != "" && right != "" {
			right = c.picks + ui.Muted.Render(ui.G.Sep) + right
		} else if c.picks != "" {
			right = c.picks
		}
		name := ui.Strong.Render(c.name)
		if c.boss {
			name = ui.Special.Render(c.name)
		}
		name = squeeze(name, width, head, right)
		out = append(out, ui.Spread(head+name, right, width))
	}
	return out
}

// courtOfHearts is the Trial's place, under its name on its card.
const courtOfHearts = "the Court of Hearts"

// endingLines are the card lines of an exit that ends the boot (the
// Trial, rules.Biome.Ending), what the route costs before it is taken
// (tone rule 5): its door cursed and one shot, a fail the end, a retreat
// a new cursed door (the engine swaps one in, as for any door), and its
// clear the end too, with no act III and no prestige unlock.
func endingLines(st *game.State, e game.Exit) (fail string, rule, ends []string) {
	fail, shot := ui.Danger.Render("a fail ends the boot"), ui.Danger.Render("one shot")
	door := "a new door"
	if n, ok := st.Rules.Node(e.Node); ok && n.Cursed {
		shot, door = ui.Danger.Render(ui.G.Cursed+" cursed"+ui.G.Sep+"one shot"), "a new cursed door"
	}
	rule = []string{shot, ui.Warn.Render(ui.G.Retreat + " retreat: " + door)}
	return fail, rule, []string{ui.Warn.Render("ends the boot here"), ui.Muted.Render("no prestige unlock")}
}
