package boot

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/units"
)

// Doors is the room's choice: one card per door with everything that bears
// on the risk (rating, timer, modifiers) and the reward. enter emits
// game.ChooseDoor; m flips to the boot map and back.
type Doors struct {
	env     Env
	sel     int
	showMap bool
}

func NewDoors(env Env) *Doors { return &Doors{env: env} }

// Refresh swaps in new state and clock, keeping the selection.
func (d *Doors) Refresh(env Env) {
	d.env = env
	d.sel = move("", d.sel, len(d.doors()))
}

func (d *Doors) doors() []game.Door { return d.env.State.Doors }

func (d *Doors) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	key, ok := keyName(msg)
	if !ok {
		return d, nil
	}
	if d.showMap {
		switch key {
		case "esc", "m", "q", "enter":
			d.showMap = false
		}
		return d, nil
	}
	n := len(d.doors())
	switch key {
	case "enter":
		if n > 0 {
			return d, screens.Emit(game.ChooseDoor{Index: d.sel})
		}
	case "m":
		d.showMap = true
	default:
		d.sel = move(key, d.sel, n)
	}
	return d, nil
}

func (d *Doors) View(width, height int) string {
	st := d.env.State
	if d.showMap {
		return mapScreen(d.env, width, height, hint("m", "back to doors"))
	}
	head := d.env.header(d.where(), width)
	n := len(st.Doors)
	var keys []string
	if n > 1 {
		keys = append(keys, "←/→", "select", fmt.Sprintf("1-%d", n), "jump")
	}
	footer := hint(append(keys, "enter", "open door", "m", "map")...)

	// The cards and the map sit centered between header and footer, the
	// way a game frames a choice, with at least one blank line under the
	// header when there is room for it.
	room := height - len(head) - 1
	block := d.cards(width, room)
	if m := centerBlock(renderMap(st, width), width); len(block)+1+len(m) <= room-1 {
		block = append(append(block, ""), m...)
	}
	return layout(width, height, centered(head, block, height), footer)
}

func (d *Doors) where() string {
	st := d.env.State
	b := st.Biome
	name := biomeLabel(st, b)
	if st.Biomes[b].Kind == rules.Boss {
		return sMagenta.Render("⚔ " + name)
	}
	return fmt.Sprintf("%s · room %d/%d", name, st.Room+1, st.Biomes[b].Rooms)
}

// cards lays the doors out in the richest form that fits: side by side on
// wide terminals, stacked full-width cards on narrow ones, and one line per
// door when even that is too tall.
func (d *Doors) cards(width, room int) []string {
	doors := d.doors()
	n := len(doors)
	if n == 0 {
		return []string{"  " + sFaint.Render("no doors on offer")}
	}
	if width >= 90 {
		cw := (width - 2*(n-1)) / n
		if cw >= 28 {
			cw = min(cw, 52)
			bodies := make([][]string, n)
			h := 0
			for i, door := range doors {
				bodies[i] = d.body(door, cw-4, true)
				h = max(h, len(bodies[i]))
			}
			blocks := make([][]string, n)
			for i, door := range doors {
				blocks[i] = d.card(i, door, cw, padLines(bodies[i], h), false)
			}
			row := joinH(2, blocks...)
			if len(row) <= room {
				return centerBlock(row, width)
			}
		}
	}
	w := min(width, 96)
	var out []string
	for i, door := range doors {
		out = append(out, indent((width-w)/2, d.card(i, door, w, d.body(door, w-4, false), true))...)
	}
	if len(out) <= room {
		return out
	}
	return d.compact(width)
}

// card frames a door body. Cursed doors keep their red frame even when not
// selected, so the danger is visible before the cursor gets there. The
// source rides in the top edge when the card is wide enough to carry it;
// narrow side-by-side cards put it in the body instead.
func (d *Doors) card(i int, door game.Door, w int, body []string, source bool) []string {
	sel := i == d.sel
	f, num := frameIdle, sFaint.Render(fmt.Sprint(i+1))
	switch {
	case door.Boss != "" && sel:
		f = frameBossSel
	case door.Boss != "":
		f = frameBoss
	case door.Cursed && sel:
		f = frameCursedSel
	case door.Cursed:
		f = frameCursed
	case sel:
		f = frameSelected
	}
	if sel {
		num = f.paint.Render(fmt.Sprintf("▸ %d", i+1))
	}
	title := num
	if door.Boss != "" {
		label := "⚔ BOSS"
		if door.Boss == game.FinalBoss {
			label = "⚔ FINAL BOSS"
		}
		title += " " + sMagenta.Bold(true).Render(label)
	} else if src := d.env.doorText(door).Source; source && src != "" {
		title += sFaint.Render(" · " + src)
	}
	return box(f, w, title, d.tag(door), body)
}

// tag is the one modifier named in the card's top edge, the most dangerous
// first.
func (d *Doors) tag(door game.Door) string {
	switch {
	case door.Boss != "":
		return ""
	case door.Cursed:
		return sRed.Bold(true).Render("☠ cursed")
	case door.Regression:
		return sYellow.Render("↺ regression")
	case door.Mystery:
		return sMagenta.Render("? mystery")
	case door.Blind:
		return sCyan.Render("◌ blind")
	}
	return ""
}

// body is a door card's content, cw cells wide. Side-by-side cards have
// height to spare, so titles and tags wrap and the source gets a line;
// stacked cards pack title and stats on one line and say why the tags are
// hidden in place of the tags.
func (d *Doors) body(door game.Door, cw int, wide bool) []string {
	if door.Boss != "" {
		return d.bossBody(door, cw)
	}
	t := d.env.doorText(door)
	stats, chance := doorStats(door), clearChance(door)
	var out []string
	if wide {
		if t.Source != "" {
			out = append(out, sFaint.Render(clip(t.Source, cw)))
		}
		for _, l := range wrapN(t.Title, cw, 2) {
			out = append(out, titleStyle(door).Render(l))
		}
		out = append(out, tagLines(t, cw, 2)...)
		out = append(out, "", stats)
		if chance != "" {
			out = append(out, chance)
		}
		return append(out, d.modifiers(door, cw, true)...)
	}
	title := titleStyle(door).Render(t.Title)
	if widthOf(title)+2+widthOf(stats) <= cw {
		out = append(out, spread(title, stats, cw))
	} else {
		out = append(out, clip(title, cw), stats)
	}
	tags := tagLines(t, cw, 1)[0]
	if s, txt := d.hiddenMod(door); txt != "" {
		tags = s.Render(txt)
	}
	out = append(out, spread(tags, chance, cw))
	return append(out, d.modifiers(door, cw, false)...)
}

func titleStyle(door game.Door) lipgloss.Style {
	if door.Mystery {
		return sMagenta.Bold(true)
	}
	return sBold
}

func tagLines(t doorText, cw, n int) []string {
	switch {
	case t.Hidden:
		return []string{sFaint.Italic(true).Render("tags hidden")}
	case len(t.Tags) == 0:
		return []string{sFaint.Render("no tags")}
	}
	var out []string
	for _, l := range wrapN(strings.Join(t.Tags, " · "), cw, n) {
		out = append(out, sFaint.Render(l))
	}
	return out
}

func doorStats(door game.Door) string {
	return sYellow.Render("◆") + fmt.Sprintf(" %d", door.Rating) +
		"  ⏱ " + units.Clock(door.Timer) +
		"  " + sGreen.Bold(true).Render("+"+units.Bytes(door.Bytes))
}

// clearChance is the engine's estimate that the player clears door, "" when
// it has none.
func clearChance(door game.Door) string {
	if door.SolveP <= 0 {
		return ""
	}
	return sFaint.Render(fmt.Sprintf("≈%d%% clear", int(door.SolveP*100+0.5)))
}

func withMult(s string, m float64) string {
	if m > 0 && m != 1 {
		return s + " · " + fmtMult(m)
	}
	return s
}

// hiddenMod is the blind or mystery modifier, what it hides and what it
// pays, or a mastered topic, which hides the tags without paying more.
// The text is "" for a door that shows its tags.
func (d *Doors) hiddenMod(door game.Door) (lipgloss.Style, string) {
	r := d.env.State.Rules
	switch {
	case door.Mystery:
		return sMagenta, withMult("? mystery: title and tags hidden", r.MysteryMult)
	case door.Blind:
		return sCyan, withMult("◌ blind: tags hidden", r.BlindMult)
	case door.Mastered:
		return sFaint, "mastered topic: tags hidden"
	}
	return lipgloss.Style{}, ""
}

// modifiers spell out what each door modifier costs and pays, so the
// choice is informed without reading the rules. hidden includes the blind
// or mystery line, which stacked cards show in place of the tags.
func (d *Doors) modifiers(door game.Door, cw int, hidden bool) []string {
	r := d.env.State.Rules
	type mod struct {
		style lipgloss.Style
		text  string
	}
	var mods []mod
	if s, txt := d.hiddenMod(door); hidden && txt != "" {
		mods = append(mods, mod{s, txt})
	}
	if door.Cursed {
		mods = append(mods, mod{sRed, withMult("☠ no ducks · 1 WA = crash", r.CursedMult)})
	}
	if door.Regression {
		txt := "↺ regression: crashed you before"
		if n, ok := d.env.regressionAge(door.Problem); ok {
			txt = "↺ regression: crashed you " + daysAgo(n)
		}
		mods = append(mods, mod{sYellow, txt})
	}
	var out []string
	for _, m := range mods {
		for _, l := range wrapN(m.text, cw, 2) {
			out = append(out, m.style.Render(l))
		}
	}
	return out
}

func daysAgo(n int) string {
	switch n {
	case 0:
		return "today"
	case 1:
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", n)
}

// bossBody is the boss door: the named bug large and centered, its blurb,
// then the problem like any other door.
func (d *Doors) bossBody(door game.Door, cw int) []string {
	name := strings.ToUpper(bossLabel(door.Boss))
	if big := spaced(name); widthOf(big) <= cw {
		name = big
	}
	out := []string{"", center(sMagenta.Bold(true).Render(clip(name, cw)), cw)}
	for _, l := range wrapN(fl.BossBlurb(door.Boss), cw-4, 2) {
		out = append(out, center(sItalic.Render(l), cw))
	}
	t := d.env.doorText(door)
	out = append(out, "", center(titleStyle(door).Render(clip(t.Title, cw)), cw))
	for _, l := range tagLines(t, cw, 1) {
		out = append(out, center(l, cw))
	}
	out = append(out, center(doorStats(door), cw))
	if c := clearChance(door); c != "" {
		out = append(out, center(c, cw))
	}
	for _, l := range d.modifiers(door, cw, true) {
		out = append(out, center(l, cw))
	}
	return append(out, "")
}

// spaced letter-spaces a word for the boss banner: HEISENBUG → H E I S …
func spaced(s string) string { return strings.Join(strings.Split(s, ""), " ") }

// compact is the last resort for short terminals: one line per door.
func (d *Doors) compact(width int) []string {
	var out []string
	for i, door := range d.doors() {
		t := d.env.doorText(door)
		mark := "  "
		if i == d.sel {
			mark = sCyan.Bold(true).Render("▸ ")
		}
		var flags []string
		if door.Boss != "" {
			flags = append(flags, sMagenta.Render("⚔"))
		}
		if door.Cursed {
			flags = append(flags, sRed.Render("☠"))
		}
		if door.Regression {
			flags = append(flags, sYellow.Render("↺"))
		}
		if door.Mystery {
			flags = append(flags, sMagenta.Render("?"))
		} else if door.Blind {
			flags = append(flags, sCyan.Render("◌"))
		}
		left := mark + fmt.Sprintf("%d ", i+1) + titleStyle(door).Render(t.Title)
		if len(flags) > 0 {
			left += " " + strings.Join(flags, "")
		}
		out = append(out, spread(left, doorStats(door), width))
	}
	return out
}
