package boot

import (
	"fmt"
	"math"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens"
)

// Route is the fork at the entrance of a choice biome: one card per topic
// on offer. enter emits game.ChooseRoute.
type Route struct {
	env Env
	sel int
}

func NewRoute(env Env) *Route { return &Route{env: env} }

// Refresh swaps in new state and clock, keeping the selection.
func (r *Route) Refresh(env Env) {
	r.env = env
	r.sel = move("", r.sel, len(r.offers()))
}

// Selected is the highlighted offer.
func (r *Route) Selected() int { return r.sel }

func (r *Route) offers() []string {
	st := r.env.State
	if st == nil || st.Biome < 0 || st.Biome >= len(st.Biomes) {
		return nil
	}
	return st.Biomes[st.Biome].Offers
}

func (r *Route) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	key, ok := keyName(msg)
	if !ok {
		return r, nil
	}
	n := len(r.offers())
	if key == "enter" {
		if n == 0 {
			return r, nil
		}
		return r, screens.Emit(game.ChooseRoute{Index: r.sel})
	}
	r.sel = move(key, r.sel, n)
	return r, nil
}

func (r *Route) View(width, height int) string {
	st := r.env.State
	if st == nil {
		return noBoot(width, height)
	}
	offers := r.offers()
	where := fmt.Sprintf("route · biome %d/%d", st.Biome+1, len(st.Biomes))
	head := r.env.header(where, width)
	footer := hint("←/→", "choose", "enter", "take route")
	block := []string{center(sBold.Render("Pick a route.")+
		sFaint.Render(" The topic you choose sets every door in this biome."), width), ""}
	room := height - len(head) - 1
	if len(offers) == 0 {
		block = append(block, "  "+sFaint.Render("no routes on offer"))
		return layout(width, height, append(append(head, ""), block...), footer)
	}

	n := len(offers)
	cw := (width - 2*(n-1)) / n
	var cards []string
	if cw >= 26 {
		cw = min(cw, 44)
		bodies := make([][]string, n)
		h := 0
		for i, t := range offers {
			bodies[i] = r.cardBody(t, cw-4)
			h = max(h, len(bodies[i]))
		}
		blocks := make([][]string, n)
		for i := range offers {
			blocks[i] = r.card(i, cw, padLines(bodies[i], h))
		}
		cards = centerBlock(joinLines(joinH(2, blocks...)), width)
	}
	if cards == nil || len(block)+len(cards) > room {
		cards = nil
		w := min(width, 72)
		for i, t := range offers {
			cards = append(cards, indent((width-w)/2, r.card(i, w, r.stackedBody(t, w-4)))...)
		}
	}
	if len(block)+len(cards) > room {
		cards = r.compact(width)
	}
	block = append(block, cards...)
	if m := centerBlock(RenderMap(st, width), width); len(block)+2+len(m) <= room-1 {
		block = append(block, "", center(sFaint.Render("the boot so far"), width))
		block = append(block, m...)
	}
	return layout(width, height, centered(head, block, height), footer)
}

func (r *Route) card(i, w int, body []string) []string {
	f := frameIdle
	title := sFaint.Render(fmt.Sprint(i + 1))
	if i == r.sel {
		f = frameSelected
		title = sCyan.Bold(true).Render(fmt.Sprintf("▸ %d", i+1))
	}
	return box(f, w, title, "", body)
}

// route is what a card says about one topic.
type route struct {
	name, topic, blurb string
	rooms              string // "2 rooms · ⏱ ~30:00"
	band               string // "◆ 1050–1250 · aim 70% clear", "" without rules
	mine               string // "your dp rating 1120", "" without a topic rating
}

func (r *Route) describe(topic string) route {
	st := r.env.State
	out := route{name: fl.BiomeName(topic), topic: topic, blurb: fl.BiomeBlurb(topic)}
	if out.name == "" {
		out.name = topic
	}
	rooms := st.Biomes[st.Biome].Rooms
	out.rooms = fmt.Sprintf("%d room", rooms)
	if rooms != 1 {
		out.rooms += "s"
	}
	rb, ok := ruleBiome(st, st.Biome)
	if ok && rb.Timer.Duration > 0 {
		out.rooms += sFaint.Render(" · ") + "⏱ ~" + fmtClock(rb.Timer.Duration)
	}
	if ok && rb.TargetClear > 0 && rb.TargetClear < 1 {
		base, mine := r.rating(topic)
		lo, hi := band(base, rb.TargetClear)
		out.band = sYellow.Render("◆") + fmt.Sprintf(" %d–%d", lo, hi) +
			sFaint.Render(fmt.Sprintf(" · aim %d%% clear", int(math.Round(rb.TargetClear*100))))
		if mine {
			out.mine = sFaint.Render(fmt.Sprintf("your %s rating %d", topic, int(math.Round(base))))
		}
	}
	return out
}

// cardBody is a side-by-side card: room for the blurb to wrap.
func (r *Route) cardBody(topic string, cw int) []string {
	d := r.describe(topic)
	out := []string{sBold.Render(d.name)}
	if d.name != d.topic {
		out = append(out, sFaint.Render(d.topic))
	}
	for _, l := range wrapN(d.blurb, cw, 3) {
		out = append(out, sItalic.Render(l))
	}
	out = append(out, "", d.rooms)
	if d.band != "" {
		out = append(out, d.band)
	}
	if d.mine != "" {
		out = append(out, d.mine)
	}
	return out
}

// stackedBody is a full-width card: three lines, the blurb clipped to one.
func (r *Route) stackedBody(topic string, cw int) []string {
	d := r.describe(topic)
	name := sBold.Render(d.name)
	if d.name != d.topic {
		name += sFaint.Render(" · " + d.topic)
	}
	out := []string{spread(name, d.band, cw)}
	if d.blurb != "" {
		out = append(out, sItalic.Render(clip(d.blurb, cw)))
	}
	return append(out, spread(d.rooms, d.mine, cw))
}

// compact is one line per route, for terminals too short for cards.
func (r *Route) compact(width int) []string {
	var out []string
	for i, t := range r.offers() {
		d := r.describe(t)
		mark := "  "
		if i == r.sel {
			mark = sCyan.Bold(true).Render("▸ ")
		}
		out = append(out, spread(mark+fmt.Sprintf("%d ", i+1)+sBold.Render(d.name), d.band, width))
	}
	return out
}

// rating is the player's rating for topic, and whether it is topic-specific
// rather than the overall estimate.
func (r *Route) rating(topic string) (float64, bool) {
	if v, ok := r.env.Meta.TopicRating[topic]; ok && v > 0 {
		return v, true
	}
	if r.env.Meta.Rating > 0 {
		return r.env.Meta.Rating, false
	}
	return r.env.State.Rules.ColdStartRating, false
}

// band is the door rating a player rated base clears with probability p
// under the Elo curve the catalog defines (rated R solves R half the
// time), widened ±100 and rounded to 50. It is a preview: the engine picks
// the actual doors.
func band(base, p float64) (int, int) {
	mid := base + 400*math.Log10(1/p-1)
	mid = math.Round(mid/50) * 50
	return int(mid - 100), int(mid + 100)
}

// centerBlock centers each line of a multi-line string as one block.
func centerBlock(s string, width int) []string {
	if s == "" {
		return nil
	}
	ls := splitLines(s)
	w := 0
	for _, l := range ls {
		w = max(w, widthOf(l))
	}
	return indent(max((width-w)/2, 0), ls)
}
