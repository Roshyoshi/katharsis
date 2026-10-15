package boot

import (
	"fmt"
	"math"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
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

func (r *Route) offers() []string {
	st := r.env.State
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
	offers := r.offers()
	where := fmt.Sprintf("route"+ui.G.Sep+"biome %d/%d", st.Biome+1, len(st.Biomes))
	head := r.env.header(where, width, height)
	footer := ui.Footer(width, "←/→", "choose", "enter", "take route")
	block := []string{ui.Center(ui.Strong.Render("Pick a route.")+
		ui.Muted.Render(" The topic you choose sets every door in this biome."), width), ""}
	room := height - len(head) - 1
	if len(offers) == 0 {
		block = append(block, "  "+ui.Muted.Render("no routes on offer"))
		return ui.Layout(width, height, append(append(head, ""), block...), footer)
	}

	n := len(offers)
	cw := (width - 2*(n-1)) / n
	var cards []string
	if cw >= 26 {
		cw = min(cw, 44)
		tops, bottoms := make([][]string, n), make([][]string, n)
		for i, t := range offers {
			tops[i], bottoms[i] = r.cardBody(t, cw-4)
		}
		blocks := make([][]string, n)
		for i, body := range alignBodies(tops, bottoms) {
			blocks[i] = r.card(i, cw, body)
		}
		cards = centerBlock(ui.JoinH(2, blocks...), width)
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
	if m := centerBlock(renderMap(st, width), width); len(block)+2+len(m) <= room-1 {
		block = append(block, "", ui.Center(ui.Muted.Render("the boot so far"), width))
		block = append(block, m...)
	}
	return ui.Layout(width, height, centered(head, block, height), footer)
}

func (r *Route) card(i, w int, body []string) []string {
	f, title := ui.FrameIdle, ui.Muted.Render(fmt.Sprint(i+1))
	if i == r.sel {
		f = ui.FrameSelected
		title = f.Render(fmt.Sprintf("%s %d", ui.G.Cursor, i+1))
	}
	return ui.Frame(f, w, title, "", body)
}

// route is what a card says about one topic.
type route struct {
	name, topic, blurb string
	rooms              string // "2 rooms · ⏱ ~30:00"
	band               string // "◆1050–1250 · aim 70% clear", "" without rules
	mine               string // "your dp rating 1120", "" unless doors aim by it
}

func (r *Route) describe(topic string) route {
	st := r.env.State
	out := route{name: fl.BiomeName(topic), topic: topic, blurb: fl.BiomeBlurb(topic)}
	if out.name == "" {
		out.name = topic
	}
	rooms := st.Biomes[st.Biome].Rooms
	out.rooms = units.Plural(rooms, "room")
	rb, ok := ruleBiome(st, st.Biome)
	if ok && rb.Timer.Duration > 0 {
		out.rooms += ui.Muted.Render(ui.G.Sep) + ui.G.Timer + "~" + units.Clock(rb.Timer.Duration)
	}
	// The aim is the biome's, or what an easing raised it to (RT8).
	aim := 0.0
	if ok {
		aim = st.AimClear(st.Biome)
	}
	if aim > 0 && aim < 1 {
		// A preview of the rating the engine aims this topic's doors at,
		// widened ±100 and rounded to 50; the pool picks the actual doors.
		meta := &r.env.Meta
		mid := int(math.Round(st.Target(meta, topic)/50) * 50)
		out.band = ui.RatingBand(mid-100, mid+100) +
			ui.Muted.Render(fmt.Sprintf("%saim %d%% clear", ui.G.Sep, int(math.Round(aim*100))))
		// The engine aims with the topic's own rating only once it has
		// enough attempts behind it; a mixed door always gets the overall one.
		if own := game.PlayerRating(&st.Rules, meta, topic); own != game.PlayerRating(&st.Rules, meta, game.Mixed) {
			// A player's rating is a bare numeral; the ◆ marks a door's.
			out.mine = ui.Muted.Render("your "+topic+" rating ") + ui.Rank(own).Render(fmt.Sprint(int(math.Round(own))))
		}
	}
	return out
}

// cardBody is a side-by-side card in two halves: the name and the blurb,
// which wraps, then a blank line and the stats.
func (r *Route) cardBody(topic string, cw int) (top, bottom []string) {
	d := r.describe(topic)
	top = []string{ui.Strong.Render(d.name)}
	if d.name != d.topic {
		top = append(top, ui.Muted.Render(d.topic))
	}
	for _, l := range ui.WrapN(d.blurb, cw, 3) {
		top = append(top, ui.Voice.Render(l))
	}
	bottom = []string{"", d.rooms}
	if d.band != "" {
		bottom = append(bottom, d.band)
	}
	if d.mine != "" {
		bottom = append(bottom, d.mine)
	}
	return top, bottom
}

// stackedBody is a full-width card: three lines, the blurb clipped to one.
func (r *Route) stackedBody(topic string, cw int) []string {
	d := r.describe(topic)
	name := ui.Strong.Render(d.name)
	if d.name != d.topic {
		name += ui.Muted.Render(ui.G.Sep + d.topic)
	}
	out := []string{ui.Spread(name, d.band, cw)}
	if d.blurb != "" {
		out = append(out, ui.Voice.Render(ui.Clip(d.blurb, cw)))
	}
	return append(out, ui.Spread(d.rooms, d.mine, cw))
}

// compact is one line per route, for terminals too short for cards, the
// name cut before the rating band.
func (r *Route) compact(width int) []string {
	var out []string
	for i, t := range r.offers() {
		d := r.describe(t)
		mark := "  "
		if i == r.sel {
			mark = ui.Focus.Render(ui.G.Cursor) + " "
		}
		head := mark + fmt.Sprintf("%d ", i+1)
		name := squeeze(ui.Strong.Render(d.name), width, head, d.band)
		out = append(out, ui.Spread(head+name, d.band, width))
	}
	return out
}
