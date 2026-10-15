package boot

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Doors is the room's choice: one card per door with everything that bears
// on the risk (rating, timer, modifiers) and the reward. enter emits
// game.ChooseDoor; s spends a shuf (game.Reroll) when one is held; m flips
// to the boot map and back.
type Doors struct {
	env     Env
	sel     int
	showMap bool
	// intro is the White Queen's block (story.first-regression) over the
	// first room that offers a regression door; drawn records that the
	// last View had room for it, so the app marks it seen (Shown).
	intro, drawn bool
}

func NewDoors(env Env) *Doors {
	d := &Doors{env: env}
	d.wantIntro()
	return d
}

// Refresh swaps in new state and clock, keeping the selection.
func (d *Doors) Refresh(env Env) {
	d.env = env
	d.sel = move("", d.sel, len(d.doors()))
	d.wantIntro()
}

// regressionHook is the doors screen's one-time block, a story scene.
const regressionHook = "story.first-regression"

// wantIntro keeps the White Queen up from the room that first offers a
// regression door, unseen, until the screen goes: marking it seen once
// drawn must not take it away under the player.
func (d *Doors) wantIntro() {
	if !d.intro && !game.HasSeen(&d.env.Meta, regressionHook) {
		d.intro = slices.ContainsFunc(d.doors(), func(x game.Door) bool { return x.Regression })
	}
}

// Shown is the one-time scene this screen has drawn, for the app to mark
// seen.
func (d *Doors) Shown() []string {
	if d.drawn {
		return []string{regressionHook}
	}
	return nil
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
	case "esc":
		if d.env.Wait != nil {
			return d, screens.Emit(CancelWait{})
		}
	case "enter":
		if n > 0 {
			return d, screens.Emit(game.ChooseDoor{Index: d.sel})
		}
	case "x":
		if _, decline := easing(d.env.State); decline != nil {
			return d, screens.Emit(decline)
		}
	case "s":
		if d.canShuf() {
			return d, screens.Emit(game.Reroll{})
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
		return mapScreen(d.env, width, height, ui.Footer(width, "m", "back to doors"))
	}
	head := d.env.header(d.where(), width, height)
	n := len(st.Doors)
	var keys []string
	if n > 1 {
		keys = append(keys, "←/→", "select", fmt.Sprintf("1-%d", n), "jump")
	}
	keys = append(keys, "enter", "open door")
	if d.canShuf() {
		keys = append(keys, "s", fmt.Sprintf("shuf (%d)", st.Rerolls))
	}
	ease, decline := easing(st)
	if decline != nil {
		keys = append(keys, "x", "decline the easing")
	}
	footer := ui.Footer(width, append(keys, "m", "map")...)
	if w := d.env.waitLine(width); w != "" {
		footer = w
	}

	// The cards and the map sit centered between header and footer, the
	// way a game frames a choice, with at least one blank line under the
	// header when there is room for it.
	room := height - len(head) - 1
	var notes []string
	for _, l := range ease {
		notes = append(notes, ui.Center(ui.Muted.Render(l), width))
	}
	if len(notes) > 0 {
		notes = append(notes, "")
	}
	cards := d.cards(width, room-len(notes))
	// The White Queen goes over the cards when they keep their tier with
	// her there; else she waits for a screen with room.
	d.drawn = false
	if d.intro {
		var q []string
		for _, l := range fl.StoryBlock(regressionHook) {
			q = append(q, ui.Voice.Render(ui.Muted.Render(ui.Clip(l, width-4))))
		}
		q = centerBlock(q, width) // one block, its lines flush left
		if len(q) > 0 && len(notes)+len(q)+1+len(cards) <= room {
			notes, d.drawn = append(append(notes, q...), ""), true
		}
	}
	block := append(notes, cards...)
	if m := centerBlock(renderMap(st, width), width); len(block)+1+len(m) <= room-1 {
		block = append(append(block, ""), m...)
	}
	return ui.Layout(width, height, centered(head, block, height), footer)
}

// canShuf reports whether s re-rolls the room: a shuf is held and the room
// is not the boss's, which the engine refuses.
func (d *Doors) canShuf() bool {
	st := d.env.State
	return st.Rerolls > 0 && st.Biomes[st.Biome].Kind != rules.Boss
}

func (d *Doors) where() string {
	st := d.env.State
	b := st.Biome
	name := biomeLabel(st, b)
	if st.Biomes[b].Kind == rules.Boss {
		return ui.Modifier(ui.ModBoss, name)
	}
	return name + ui.Muted.Render(ui.G.Sep) + fmt.Sprintf("room %d/%d", st.Room+1, st.Biomes[b].Rooms)
}

// cards lays the doors out in the richest form that fits: side by side on
// wide terminals, stacked full-width cards on narrow ones, and one line per
// door when even that is too tall.
func (d *Doors) cards(width, room int) []string {
	doors := d.doors()
	n := len(doors)
	if n == 0 {
		return []string{"  " + ui.Muted.Render("no doors on offer")}
	}
	if width >= 90 {
		cw := (width - 2*(n-1)) / n
		if cw >= 28 {
			cw = min(cw, 52)
			tops, bottoms := make([][]string, n), make([][]string, n)
			for i, door := range doors {
				tops[i], bottoms[i] = d.wideBody(door, cw-4)
			}
			blocks := make([][]string, n)
			for i, body := range alignBodies(tops, bottoms) {
				blocks[i] = d.card(i, doors[i], cw, body, false)
			}
			row := ui.JoinH(2, blocks...)
			if len(row) <= room {
				return centerBlock(row, width)
			}
		}
	}
	w := min(width, 96)
	var out []string
	for i, door := range doors {
		out = append(out, indent((width-w)/2, d.card(i, door, w, d.stackedBody(door, w-4), true))...)
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
	f, num := ui.FrameIdle, ui.Muted.Render(fmt.Sprint(i+1))
	switch {
	case door.Boss != "" && sel:
		f = ui.FrameBossSelected
	case door.Boss != "":
		f = ui.FrameBoss
	case door.Cursed && sel:
		f = ui.FrameCursedSelected
	case door.Cursed:
		f = ui.FrameCursed
	case sel:
		f = ui.FrameSelected
	}
	if sel {
		num = f.Render(fmt.Sprintf("%s %d", ui.G.Cursor, i+1))
	}
	title := num
	if door.Boss != "" {
		label := "BOSS"
		if door.Boss == game.FinalBoss {
			label = "FINAL BOSS"
		}
		title += " " + ui.Modifier(ui.ModBoss, label)
	} else if src := d.env.doorText(door).Source; source && src != "" {
		title += ui.Muted.Render(ui.G.Sep + src)
	}
	return ui.Frame(f, w, title, d.tag(door), body)
}

// tag is the one modifier named in the card's top edge, the most dangerous
// first.
func (d *Doors) tag(door game.Door) string {
	switch {
	case door.Boss != "":
		return ""
	case door.Cursed:
		return ui.Modifier(ui.ModCursed, "")
	case door.Regression:
		return ui.Modifier(ui.ModRegression, "")
	case door.Mystery:
		return ui.Modifier(ui.ModMystery, "")
	case door.Blind:
		return ui.Modifier(ui.ModBlind, "")
	}
	return ""
}

// wideBody is a side-by-side card's content, cw cells wide, in two halves:
// the source, title and tags, which wrap, then a blank line, the stats and
// the modifiers. The tag slot already says the tags are hidden, so the
// blind or mystery line only says what the door pays.
func (d *Doors) wideBody(door game.Door, cw int) (top, bottom []string) {
	if door.Boss != "" {
		return d.bossBody(door, cw), nil
	}
	t := d.env.doorText(door)
	if t.Source != "" {
		top = append(top, ui.Muted.Render(ui.Clip(t.Source, cw)))
	}
	for _, l := range ui.WrapN(t.Title, cw, 2) {
		top = append(top, titleToken(door).Render(l))
	}
	top = append(top, tagLines(t, cw, 2)...)
	// The download's state shares the stats row when it fits.
	bottom = []string{"", d.doorStats(door)}
	if f := d.env.fetchNote(door); f != "" && widthOf(bottom[1])+2+widthOf(f) <= cw {
		bottom[1] = ui.Spread(bottom[1], f, cw)
	} else if f != "" {
		bottom = append(bottom, f)
	}
	return top, append(bottom, d.modifiers(door, cw, hiddenShort)...)
}

// stackedBody is a full-width card's content, cw cells wide: title and
// stats on one line, then the tags, or why they are hidden in their place,
// then the modifiers.
func (d *Doors) stackedBody(door game.Door, cw int) []string {
	if door.Boss != "" {
		return d.bossBody(door, cw)
	}
	t := d.env.doorText(door)
	stats := d.doorStats(door)
	var out []string
	title := titleToken(door).Render(t.Title)
	if widthOf(title)+2+widthOf(stats) <= cw {
		out = append(out, ui.Spread(title, stats, cw))
	} else {
		out = append(out, ui.Clip(title, cw), stats)
	}
	tags := tagLines(t, cw, 1)[0]
	if h, ok := d.hiddenMod(door, hiddenLong); ok {
		tags = h.line()
	}
	out = append(out, ui.Spread(tags, d.env.fetchNote(door), cw))
	return append(out, d.modifiers(door, cw, hiddenNone)...)
}

func titleToken(door game.Door) ui.Token {
	if door.Mystery {
		return ui.Special
	}
	return ui.Strong
}

func tagLines(t doorText, cw, n int) []string {
	switch {
	case t.Hidden:
		return []string{ui.Voice.Render(ui.Muted.Render("tags hidden"))}
	case len(t.Tags) == 0:
		return []string{ui.Muted.Render("no tags")}
	}
	return ui.WrapN(strings.Join(t.Tags, ui.G.Sep), cw, n)
}

// doorStats is the card's stats row: rating, the timer the door opens
// with (State.OpenTimer: at's time too), reward. The engine's clear
// chance (Door.SolveP) stays off the card (D36): shown before the pick, the
// pick stops measuring the player's own read of the door; the end of the
// boot shows it afterwards, as calibration. A rating on its source's floor
// is a ceiling, "◆≤800" (Door.Floor).
func (d *Doors) doorStats(door game.Door) string {
	rating := ui.Rating(door.Rating)
	if door.Floor {
		rating = ui.RatingAtMost(door.Rating)
	}
	return rating + "  " + ui.G.Timer + units.Clock(d.env.State.OpenTimer(door)) +
		"  " + ui.OK.Render("+"+units.Bytes(door.Bytes))
}

func withMult(s string, m float64) string {
	if m > 0 && m != 1 {
		return s + ui.G.Sep + fmtMult(m)
	}
	return s
}

// modLine is one modifier spelled out: a door modifier with its text, or,
// for a mastered topic, which is not one, Muted text.
type modLine struct {
	mod      ui.Mod
	text     string
	mastered bool
}

func (m modLine) lines(cw, n int) []string {
	if m.mastered {
		var out []string
		for _, l := range ui.WrapN(m.text, cw, n) {
			out = append(out, ui.Muted.Render(l))
		}
		return out
	}
	return ui.ModifierLines(m.mod, m.text, cw, n)
}

func (m modLine) line() string {
	if m.mastered {
		return ui.Muted.Render(m.text)
	}
	return ui.Modifier(m.mod, m.text)
}

// hiddenForm is how much the blind or mystery line says: nothing, what it
// pays (beside a tag slot that already says the tags are hidden), or what
// it hides and pays (in place of the tags).
type hiddenForm int

const (
	hiddenNone hiddenForm = iota
	hiddenShort
	hiddenLong
)

// hiddenMod is the blind or mystery modifier, what it hides and what it
// pays, or a mastered topic, which hides the tags without paying more. ok
// is false for a door that shows its tags.
func (d *Doors) hiddenMod(door game.Door, form hiddenForm) (modLine, bool) {
	r := d.env.State.Rules
	long := form == hiddenLong
	switch {
	case form == hiddenNone:
	case door.Mystery && long:
		return modLine{mod: ui.ModMystery, text: withMult("mystery: title and tags hidden", r.MysteryMult)}, true
	case door.Mystery:
		return modLine{mod: ui.ModMystery, text: withMult("mystery", r.MysteryMult)}, true
	case door.Blind && long:
		return modLine{mod: ui.ModBlind, text: withMult("blind: tags hidden", r.BlindMult)}, true
	case door.Blind:
		return modLine{mod: ui.ModBlind, text: withMult("blind", r.BlindMult)}, true
	case door.Mastered && long:
		return modLine{text: "mastered topic: tags hidden", mastered: true}, true
	case door.Mastered:
		return modLine{text: "mastered topic", mastered: true}, true
	}
	return modLine{}, false
}

// modifiers spell out what each door modifier costs and pays, so the
// choice is informed without reading the rules. form says whether and how
// the blind or mystery line comes first; stacked cards show it in place of
// the tags instead.
func (d *Doors) modifiers(door game.Door, cw int, form hiddenForm) []string {
	r := d.env.State.Rules
	var mods []modLine
	if h, ok := d.hiddenMod(door, form); ok {
		mods = append(mods, h)
	}
	if door.Cursed {
		mods = append(mods, modLine{mod: ui.ModCursed, text: withMult("no ducks"+ui.G.Sep+"1 WA = crash", r.CursedMult)})
	}
	if t, ok := timeoutNote(d.env.State); ok {
		mods = append(mods, modLine{text: ui.G.Timer + t, mastered: true}) // Muted, no modifier
	}
	if door.Regression {
		txt := "regression: crashed you before"
		if n, ok := d.env.regressionAge(door.Problem); ok {
			txt = "regression: crashed you " + daysAgo(n)
		}
		mods = append(mods, modLine{mod: ui.ModRegression, text: txt})
	}
	var out []string
	for _, m := range mods {
		out = append(out, m.lines(cw, 2)...)
	}
	return out
}

func daysAgo(n int) string {
	if n == 0 {
		return "today"
	}
	return units.Plural(n, "day") + " ago"
}

// bossBody is the boss door: the named bug large and centered, its blurb,
// then the problem like any other door.
func (d *Doors) bossBody(door game.Door, cw int) []string {
	name := strings.ToUpper(bossLabel(door.Boss))
	if big := spaced(name); widthOf(big) <= cw {
		name = big
	}
	out := []string{"", ui.Center(ui.Special.Render(ui.Clip(name, cw)), cw)}
	// The entrance line goes under the name (story text, boss.enter.*): a
	// pool, one entry per boot, a 2-line entry for the Jabberwock.
	if enter := fl.Story("boss.enter."+door.Boss, d.env.State.Seed); enter != "" {
		for _, e := range strings.Split(enter, "\n") {
			for _, l := range ui.WrapN(e, cw-4, 2) {
				out = append(out, ui.Center(ui.Voice.Render(ui.Muted.Render(l)), cw))
			}
		}
		out = append(out, "")
	}
	for _, l := range ui.WrapN(fl.BossBlurb(door.Boss), cw-4, 2) {
		out = append(out, ui.Center(ui.Voice.Render(l), cw))
	}
	t := d.env.doorText(door)
	out = append(out, "", ui.Center(titleToken(door).Render(ui.Clip(t.Title, cw)), cw))
	for _, l := range tagLines(t, cw, 1) {
		out = append(out, ui.Center(l, cw))
	}
	out = append(out, ui.Center(d.doorStats(door), cw))
	if f := d.env.fetchNote(door); f != "" {
		out = append(out, ui.Center(f, cw))
	}
	for _, l := range d.modifiers(door, cw, hiddenShort) {
		out = append(out, ui.Center(l, cw))
	}
	return append(out, "")
}

// spaced letter-spaces a word for the boss banner: HEISENBUG → H E I S …
func spaced(s string) string { return strings.Join(strings.Split(s, ""), " ") }

// compact is the last resort for short terminals: one line per door, its
// modifiers as glyphs, a space apart, and the title cut before the stats.
func (d *Doors) compact(width int) []string {
	var out []string
	for i, door := range d.doors() {
		t := d.env.doorText(door)
		mark := "  "
		if i == d.sel {
			mark = ui.Focus.Render(ui.G.Cursor) + " "
		}
		var flags []string
		flag := func(m ui.Mod) { flags = append(flags, m.Token().Render(m.Glyph())) }
		if door.Boss != "" {
			flag(ui.ModBoss)
		}
		if door.Cursed {
			flag(ui.ModCursed)
		}
		if door.Regression {
			flag(ui.ModRegression)
		}
		if door.Mystery {
			flag(ui.ModMystery)
		} else if door.Blind {
			flag(ui.ModBlind)
		}
		head, tail, stats := mark+fmt.Sprintf("%d ", i+1), "", d.doorStats(door)
		if len(flags) > 0 {
			tail = " " + strings.Join(flags, " ")
		}
		title := squeeze(titleToken(door).Render(t.Title), width, head, tail, stats)
		out = append(out, ui.Spread(head+title+tail, stats, width))
	}
	return out
}
