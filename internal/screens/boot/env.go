// Package boot holds the full-screen views of a boot in progress: route
// choice, doors, map, merge window, crash and clear, plus the HUD bar shown
// inside a problem. Every screen is a read-only view over an Env; player
// choices leave as game.Actions or screens intents through screens.Emit.
package boot

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Env is everything a boot screen reads. State is the boot in progress,
// never nil: the app builds these screens only during a boot. Screens never
// write through State. After the engine steps or the clock ticks, the app
// either builds a new screen or calls Refresh, which keeps the cursor where
// the player left it.
type Env struct {
	State  *game.State
	Meta   game.Meta
	Lookup screens.Lookup
	Now    time.Time
	End    End // the app's figures for the end screens and the goal bar (debrief.go)
	// Events is the dmesg strip, oldest first (Tracker.Events); nil shows
	// none.
	Events []Event
	// Fetch is each door's download (fetch.go), by problem; Wait the door
	// chosen while its download runs, nil for none.
	Fetch map[catalog.ID]Fetch
	Wait  *Wait
	// Module names the RTFM module whose trial opens topic's capability
	// for the player, for a route's locked exit; nil, or "" while RTFM is
	// not loaded, names the capability instead.
	Module func(topic string) string
}

// flavorSource is the slice of internal/flavor the screens use. fl points at
// the real package; tests pin it to a stub so golden files check layout,
// not whatever text the flavor package ships this week.
type flavorSource struct {
	BiomeName    func(topic string) string
	BiomeBlurb   func(topic string) string
	BossName     func(key string) string
	BossBlurb    func(key string) string
	PrestigeName func(p int) string
	// The upgrade, daemon and item texts quote the boot's own rules, so a
	// preset that changes a number shows what it actually grants.
	UpgradeText func(u game.Upgrade, r rules.Rules) (name, desc string)
	DaemonText  func(d game.Daemon, r rules.Rules) (name, desc string)
	ItemText    func(it game.Item, r rules.Rules) (name, desc string)
	QuoteFor    func(cause game.Outcome, verdict string, seed uint64) flavor.Quote
	Banner      func(kind string) string
	// PanicReason and CallTrace are the kernel's words for a crash: the
	// text after "not syncing: " and the innermost frames of the trace.
	PanicReason func(cause game.Outcome, verdict string) string
	CallTrace   func(cause game.Outcome, verdict string, seed uint64) []string
	// Story and StoryBlock are docs/story/text.md by hook: one line of a
	// pool or fixed hook, and every line of a block (flavor.Story).
	Story      func(hook string, seed uint64) string
	StoryBlock func(hook string) []string
}

var realFlavor = flavorSource{
	BiomeName:    flavor.BiomeName,
	BiomeBlurb:   flavor.BiomeBlurb,
	BossName:     flavor.BossName,
	BossBlurb:    flavor.BossBlurb,
	PrestigeName: flavor.PrestigeName,
	UpgradeText:  flavor.UpgradeTextFor,
	DaemonText:   flavor.DaemonTextFor,
	ItemText:     flavor.ItemTextFor,
	QuoteFor:     flavor.QuoteFor,
	Banner:       flavor.Banner,
	PanicReason:  flavor.PanicReason,
	CallTrace:    flavor.CallTrace,
	Story:        flavor.Story,
	StoryBlock:   flavor.StoryBlock,
}

var fl = realFlavor

func (e Env) problem(id catalog.ID) catalog.Problem {
	if e.Lookup != nil {
		if p, ok := e.Lookup(id); ok {
			return p
		}
	}
	return catalog.Problem{ID: id, Title: string(id)}
}

// hasEditorial reports whether id has an editorial to open. A problem the
// lookup cannot find gets the benefit of the doubt: the app, which fetches
// it, says so if there is none.
func (e Env) hasEditorial(id catalog.ID) bool {
	if id == "" {
		return false
	}
	if e.Lookup != nil {
		if p, ok := e.Lookup(id); ok {
			return p.HasEditorial
		}
	}
	return true
}

func prestigeLabel(p int) string {
	if n := fl.PrestigeName(p); n != "" {
		return n
	}
	return fmt.Sprintf("prestige %d", p)
}

func bossLabel(key string) string {
	if n := fl.BossName(key); n != "" {
		return n
	}
	return key
}

func upgradeText(u game.Upgrade, r rules.Rules) (string, string) {
	name, desc := fl.UpgradeText(u, r)
	if name == "" {
		name = string(u)
	}
	return name, desc
}

// itemText is an item on sale's name and effect in the boot's numbers,
// the effect without its full stop: it is one part of a " · " line.
func itemText(it game.Item, r rules.Rules) (string, string) {
	name, desc := fl.ItemText(it, r)
	if name == "" {
		name = string(it)
	}
	return name, strings.TrimSuffix(desc, ".")
}

// daemonText is a daemon's name and effect in the boot's numbers. A
// room's usual size counts the doors packages add (tmux) outside a trial,
// as the engine does (game roomSize), so fork says what it adds over the
// rooms this boot actually has: "(normally 4)" with tmux installed.
func daemonText(d game.Daemon, st *game.State) (string, string) {
	r := st.Rules
	if st.Trial == nil {
		r.DoorsPerRoom += r.Grants(st.Packages).Doors
	}
	name, desc := fl.DaemonText(d, r)
	if name == "" {
		name = string(d)
	}
	return name, desc
}

// ruleBiome is the preset entry behind biome i, for its timer and target
// clear rate, which BiomeState does not carry.
func ruleBiome(st *game.State, i int) (rules.Biome, bool) {
	if i < 0 || i >= len(st.Rules.Biomes) {
		return rules.Biome{}, false
	}
	return st.Rules.Biomes[i], true
}

// biomeLabel names biome i for maps and headers: the engine's name, else
// the boss it holds, else the flavor name of its topic, else what kind of
// biome it is. The boss goes before the topic because a boss biome's topic
// is Mixed, whose flavor name (Userland) belongs to the first biome.
func biomeLabel(st *game.State, i int) string {
	if i < 0 || i >= len(st.Biomes) {
		return "?"
	}
	b := st.Biomes[i]
	switch {
	case b.Name != "":
		return b.Name
	case b.Kind == rules.Boss && b.Boss != "":
		return bossLabel(b.Boss)
	case b.Topic != "":
		if n := fl.BiomeName(b.Topic); n != "" {
			return n
		}
		return b.Topic
	case b.Kind == rules.Boss:
		return "boss"
	}
	if rb, ok := ruleBiome(st, i); ok && rb.Name != "" {
		return rb.Name
	}
	return "?"
}

// position is where the boot stands, normalized so that "room == rooms of
// this biome" and "room 0 of the next biome" mean the same place. The
// engine may use either between biomes; screens must not care.
func position(st *game.State) (biome, room int) {
	b, r := st.Biome, st.Room
	if b >= 0 && b < len(st.Biomes) && r >= st.Biomes[b].Rooms {
		b, r = b+1, 0
	}
	return b, r
}

// step is one attempt placed in the boot: which biome and room it was in
// and its door number (1-based, counting every door opened).
type step struct {
	Attempt game.Attempt
	Biome   int
	Room    int
	N       int
}

// steps replays the boot's attempts onto its biomes. Rooms advance on a
// clear and, under cycles' rules.FailMoveOn, on a room lost to a timeout
// or a give-up the boot survived (D47); a retreat, a boss's failure (a new
// boss door) and D44's timeout draw the same room again, and any other
// outcome ends the boot, so the walk is exact. An attempt that records its
// biome (Attempt.Biome, later than the walk) moves the walk there. The
// open or crashing attempt is included once even if the engine has not
// yet moved it into History.
func steps(st *game.State) []step {
	atts := append([]game.Attempt(nil), st.History...)
	if c := st.Current; c != nil {
		dup := false
		for _, a := range atts {
			if a.Door.Problem == c.Door.Problem && a.Opened.Equal(c.Opened) {
				dup = true
			}
		}
		if !dup {
			atts = append(atts, *c)
		}
	}
	out := make([]step, 0, len(atts))
	b, r := 0, 0
	for i, a := range atts {
		if a.Biome > b && a.Biome < len(st.Biomes) {
			b, r = a.Biome, 0
		}
		out = append(out, step{Attempt: a, Biome: b, Room: r, N: i + 1})
		if a.Outcome == game.Cleared || lostRoom(st, a, b) {
			r++
			if b < len(st.Biomes) && r >= st.Biomes[b].Rooms {
				b, r = b+1, 0
			}
		}
	}
	return out
}

// lostRoom reports whether attempt a, in biome b, lost its room and moved
// the boot on (rules.FailMoveOn): a timeout or a give-up outside a boss,
// in a biome with a failure price. A boss draws a new boss door.
func lostRoom(st *game.State, a game.Attempt, b int) bool {
	if a.Outcome != game.TimedOut && a.Outcome != game.GaveUp || !st.Rules.FailMoveOn {
		return false
	}
	rb, ok := ruleBiome(st, b)
	return ok && rb.FailCycles > 0 && b < len(st.Biomes) && st.Biomes[b].Kind != rules.Boss
}

// doorText is what a door shows before it is opened. Blind hides tags, as
// does a mastered topic; mystery also hides the title, and the contest,
// which would name it.
type doorText struct {
	Title  string
	Source string
	Tags   []string
	Hidden bool // tags hidden
}

func (e Env) doorText(d game.Door) doorText {
	p := e.problem(d.Problem)
	t := doorText{Title: p.Title, Source: sourceLabel(p, !d.Mystery), Tags: p.Tags}
	if d.Mystery {
		t.Title = "???"
	}
	if d.Blind || d.Mystery || d.Mastered || e.State.Rules.BlindAll {
		t.Tags, t.Hidden = nil, true
	}
	return t
}

// sourceLabel is where a door's problem is from: the site, then the
// contest when contest is set, without naming the site twice. Codeforces
// is CF, since the round number is what a door edge has room for:
// "CF Round 690 (Div. 3) · A" (the contest names the site), "USACO ·
// 2017 December, Silver P1", "CF · VK Cup 2019 · B".
func sourceLabel(p catalog.Problem, contest bool) string {
	src := p.Source
	if src == "" {
		src = p.ID.Source()
	}
	site := src.Label()
	short := site
	if src == catalog.CF {
		short = "CF"
	}
	switch c := p.Contest; {
	case !contest || c == "":
		return short
	case site == "":
		return c
	case strings.Contains(c, site):
		return strings.Replace(c, site, short, 1)
	default:
		return short + ui.G.Sep + c
	}
}

// regressionAge is how many days ago the regression on id crashed the
// player. Regression keeps only the due date and stage, so the crash is
// inferred as Due minus that stage's delay.
func (e Env) regressionAge(id catalog.ID) (int, bool) {
	days := e.State.Rules.RegressionDays
	for _, r := range e.Meta.Regressions {
		if r.Problem != id || r.Stage < 0 || r.Stage >= len(days) {
			continue
		}
		crashed := r.Due.Add(-time.Duration(days[r.Stage]) * 24 * time.Hour)
		return max(int(e.Now.Sub(crashed).Hours()/24), 0), true
	}
	return 0, false
}

// header is the top of every boot screen: who and where on the first line,
// lives, ducks and bytes on the second, the dmesg strip once the boot has
// events (stripRows for a screen height tall), then a rule.
func (e Env) header(where string, width, height int) []string {
	st := e.State
	dot := ui.Muted.Render(ui.G.Sep)
	left := ui.Strong.Render(fmt.Sprintf("Boot #%d", st.Boot)) + dot + prestigeLabel(st.Prestige)
	if where != "" {
		left += dot + where
	}
	// Time inside doors: it stands still while the player picks.
	right := ui.Muted.Render("uptime ") + units.Clock(st.Uptime(e.Now))

	// The bytes at stake are plain, not OK: a crash loses them.
	status := ui.Hearts(st.Cycles, st.MaxCycles) + "  " + fmt.Sprintf("%s%d", ui.G.Duck, st.Ducks) + "  " +
		units.Bytes(st.Uncommitted) + ui.Muted.Render(" uncommitted") + dot +
		units.Bytes(st.Committed) + ui.Muted.Render(" committed")
	var ds []string
	for _, d := range st.Daemons {
		n, _ := daemonText(d, st)
		ds = append(ds, n)
	}
	daemons := ""
	if len(ds) > 0 {
		daemons = ui.Muted.Render("daemons ") + strings.Join(ds, " ")
	}
	out := []string{ui.Spread(left, right, width), ui.Spread(status, daemons, width)}
	out = append(out, e.strip(width, e.stripRows(height))...)
	return append(out, ui.Rule(width))
}

// keyName is the key a press stands for, folding vim and arrow synonyms
// together so every screen moves the same way.
func keyName(msg tea.Msg) (string, bool) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return "", false
	}
	switch s := k.String(); s {
	case "left", "h", "up", "k", "shift+tab":
		return "prev", true
	case "right", "l", "down", "j", "tab":
		return "next", true
	case "enter", "space":
		return "enter", true
	default:
		return s, true
	}
}

// move applies prev/next/digit keys to a cursor over n items.
func move(key string, sel, n int) int {
	switch {
	case n <= 0:
		return 0
	case key == "prev":
		return max(sel-1, 0)
	case key == "next":
		return min(sel+1, n-1)
	case len(key) == 1 && key[0] >= '1' && key[0] <= '9':
		if i := int(key[0] - '1'); i < n {
			return i
		}
	}
	return min(sel, n-1)
}
