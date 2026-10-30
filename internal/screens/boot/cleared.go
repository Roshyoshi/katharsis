package boot

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/story"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Cleared is the victory screen after the last boss: how long the boot ran,
// what it banked and solved, then the debrief (retention RT1): what the
// boot kept (the rating's move, the prestige it unlocked or what the next
// still needs (D31), apt's goal bar, first-time and best lines), the
// clear's lever, and the next step. Story lines from the boss's defeat and
// the prestige just cleared (a trial's: the capability it granted) sit
// under the banner.
//
// r or enter emits screens.StartBoot at the highest unlocked prestige, an
// offered preset's key (s short, p patch) one on that preset (RT7), b
// reads the story beat waiting (story.Read), l opens git log, and
// g or q goes to GRUB (screens.OpenGrub, UX J12). After a passed trial r
// retries it, and the next step is RTFM (End.Learn): m or enter emits
// screens.OpenLearn at the next module, or back at the trial's.
type Cleared struct{ env Env }

func NewCleared(env Env) *Cleared { return &Cleared{env: env} }

// Refresh swaps in new state and clock.
func (c *Cleared) Refresh(env Env) { c.env = env }

func (c *Cleared) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return c, nil
	}
	// Raw keys: l is git log here, not the vim move keyName makes it.
	key := k.String()
	if key == "enter" {
		key = "r"
		if c.env.End.Learn != nil {
			key = learnKey // the step the screen offers
		}
	}
	if b, ok := rebootKey(c.env, key); ok {
		return c, screens.Emit(b)
	}
	switch key {
	case beatKey:
		if id := beat(c.env); id != "" {
			return c, screens.Emit(story.Read{ID: id})
		}
	case learnKey:
		if l := c.env.End.Learn; l != nil {
			return c, screens.Emit(screens.OpenLearn{Module: l.Module})
		}
	case "l":
		return c, screens.Emit(screens.OpenLog{})
	case "g", "q":
		return c, screens.Emit(screens.OpenGrub{})
	}
	return c, nil
}

func (c *Cleared) View(width, height int) string {
	env := c.env
	st := env.State
	b := game.Debrief(st, &env.Meta)
	var solves, hinted, retreats, lost, retried int
	var boss string
	end := env.Now
	for _, s := range steps(st) {
		a := s.Attempt
		switch a.Outcome {
		case game.Cleared:
			solves++
			if a.Quacks > 0 {
				hinted++
			}
			if a.Door.Boss != "" {
				boss = a.Door.Boss
			}
		case game.Retreated:
			retreats++
		case game.TimedOut, game.GaveUp:
			// The boot survived it, or it would not be cleared: a room lost
			// (D47; D44 drew it again), or a boss door dealt anew.
			if a.Door.Boss != "" {
				retried++
			} else {
				lost++
			}
		}
		if !a.Closed.IsZero() {
			end = a.Closed
		}
	}
	uptime := st.Uptime(end)

	var banner []string
	if art := fl.Banner("cleared"); art != "" {
		bl := strings.Split(art, "\n")
		for i, l := range bl {
			// systemd paints its OK green; the climax keeps it (E17).
			bl[i] = strings.Replace(l, "[  OK  ]", "[  "+ui.OK.Render("OK")+"  ]", 1)
		}
		// The climax gets the double frame (§2.5).
		if fw := maxWidth(bl) + 4; fw <= width {
			banner = centerBlock(ui.Frame(ui.FrameBoss, fw, "", "", bl), width)
		}
	}
	// Green paints the glyph, the words are Strong (R2, R3). A trial is a
	// boot in the store, but the player started a trial, not boot #N.
	what := fmt.Sprintf("Boot #%d cleared", st.Boot)
	if st.Trial != nil {
		what = st.Trial.Title + " trial cleared"
	}
	// The Trial's ending (achievements.md (f)): the boot is cleared, awake,
	// and unlocks nothing. "Awake", never "woke".
	awake := st.Ending == game.EndingWake
	if awake {
		what = fmt.Sprintf("Boot #%d", st.Boot) + ui.G.Sep + "awake"
	}
	head := []string{ui.Center(ui.OK.Render(ui.G.Pass)+" "+ui.Strong.Render(what)+
		ui.Muted.Render(ui.G.Sep)+prestigeLabel(st.Prestige), width)}
	if boss != "" {
		head = append(head, ui.Center(ui.Modifier(ui.ModBoss, bossLabel(boss)), width))
	}
	seed := st.Seed ^ uint64(end.Unix())
	var defeat, said []string
	if boss != "" {
		defeat = centerVoice(tale.Story("boss.defeat."+boss, seed), width)
	}
	switch {
	case awake: // the scene said it all
	case st.Trial == nil:
		said = centerVoice(c.clearedLine(seed), width)
	case st.Trial.Granted != "":
		// The Gryphon names what the trial granted and the biome it opens
		// (rtfm.trial.pass). A trial that granted nothing opened no route,
		// so it gets no line.
		said = centerVoice(c.trialLine(seed), width)
	}

	sol := units.Plural(solves, "door")
	if hinted > 0 {
		sol += ui.G.Sep + fmt.Sprintf("%d hinted", hinted)
	}
	if retreats > 0 {
		sol += ui.G.Sep + units.Plural(retreats, "retreat")
	}
	if lost > 0 {
		sol += ui.G.Sep + fmt.Sprintf("%d lost", lost)
	}
	if retried > 0 {
		sol += ui.G.Sep + units.Plural(retried, "boss retry")
	}
	cycles := Cycles(st)
	if st.Fountained > 0 {
		cycles += ui.Muted.Render(ui.G.Sep + fmt.Sprintf("fountains +%d", st.Fountained))
	}
	stats := [][2]string{
		{"uptime", units.Clock(uptime)},
		{"committed", ui.OK.Render(units.Bytes(st.Committed)) +
			ui.Muted.Render(ui.G.Sep+"bank ") + units.Bytes(env.Meta.Bank)},
		{"solved", sol},
		{"cycles left", cycles},
	}
	var table []string
	for _, r := range stats {
		table = append(table, ui.Muted.Render(fmt.Sprintf("%-12s", r[0]))+r[1])
	}

	// An achievement: the ★ is Warn, the words Strong (R2, R3).
	star := ui.Warn.Render(ui.G.Star) + " "
	kept := []string{ratingItem(b.Rating, "rating", false)}
	switch {
	case awake: // no prestige line: the ending unlocks none
	case b.Unlocked > 0:
		kept = append(kept, star+ui.Strong.Render("prestige unlocked: "+prestigeLabel(b.Unlocked)))
	case b.Top:
		kept = append(kept, star+ui.Strong.Render("highest prestige cleared"))
	case !b.Trial:
		kept = append(kept, gateItem(b.Gate, env.Meta.Prestige))
	}
	kept = append(kept, timePlusItem(env.End.TimePlus, uptime))
	kept = append(kept, easeItems(st)...)
	kept = append(kept, brk, goalItem(env.End.Goal, b.Bank), brk)
	for _, f := range env.End.Firsts[:min(len(env.End.Firsts), 2)] {
		kept = append(kept, star+ui.Strong.Render(f))
	}
	// The setuid bits the boot set, on a line of their own (achievements.md
	// (h)). At 80x24 the screen is full, so the line takes the blank row
	// above kept.
	above := 0
	if line := setuidKept(env); line != "" {
		kept, above = append(kept, brk, line), 9
	}
	nextStep := rebootStep(env)
	if l := env.End.Learn; l != nil {
		nextStep = trialNextStep(l)
	}
	// The debrief is one column, centered like the rest of the screen.
	cw := min(width, debriefW)
	col := func(ls []string) []string { return indent((width-cw)/2, ls) }

	pieces := []piece{
		{lines: banner, pri: 6},
		{lines: []string{""}, pri: 6},
		{lines: head},
		{lines: defeat, pri: 2},
		{lines: said, pri: 4},
		{lines: []string{""}},
		{lines: centerBlock(table, width)},
		{lines: []string{""}, pri: above},
		{lines: col(section("kept", kept, cw))},
		{lines: col(teaser(beat(env), cw)), pri: 1},
		{lines: col(section("lever", []string{leverText(env, b.Lever)}, cw))},
		{lines: col(nextSection(nextStep, cw))},
		{lines: col(budgetLine(env.End, cw)), pri: 3},
	}
	if mp := centerBlock(renderMap(st, width), width); len(mp) > 0 {
		pieces = append(pieces, piece{lines: []string{""}, pri: 5}, piece{lines: mp, pri: 5})
	}
	var body []string
	for _, p := range fit(pieces, height-1) {
		body = append(body, p...)
	}
	if pad := (height - 1 - len(body)) / 2; pad > 0 {
		body = append(make([]string, pad), body...)
	}
	// The keys sit at the left, as on every game screen (C2); GRUB is the
	// way out, kept last.
	pairs := rebootPairs(env)
	if beat(env) != "" {
		pairs = append(pairs, beatKey, "story")
	}
	pairs = append(pairs, "l", "git log", "g", "GRUB")
	return ui.Layout(width, height, body, ui.Footer(width, pairs...))
}

// clearedLine is the story's line for the prestige just cleared
// (text.md, Cleared): its own pool, or past the table the beyond one with
// its ring.
func (c *Cleared) clearedLine(seed uint64) string {
	p := c.env.State.Prestige
	if s := tale.Story("cleared."+strconv.Itoa(p), seed); s != "" {
		return s
	}
	ring, _ := flavor.Ring(p)
	return flavor.Fill(tale.Story("cleared.beyond", seed), "ring", strconv.Itoa(ring))
}

// trialLine is the story's line for a trial that granted a capability:
// the capability, and the biome of the topic it unlocks (the trial's
// integration topic is the module's, and so is the capability).
func (c *Cleared) trialLine(seed uint64) string {
	tr := c.env.State.Trial
	return flavor.Fill(tale.Story("rtfm.trial.pass", seed),
		"cap", ui.Clip(tr.Granted, 14), "biome", ui.Clip(fl.BiomeName(tr.Topic), 18))
}

// centerVoice is a story line in the flavor voice, centered, wrapped to
// at most two lines.
func centerVoice(s string, width int) []string {
	var out []string
	for _, part := range strings.Split(s, "\n") {
		for _, l := range ui.WrapN(part, max(min(width-4, 72), 10), 2) {
			out = append(out, ui.Center(ui.Voice.Render(ui.Muted.Render(l)), width))
		}
	}
	return out
}

// debriefW is the widest the clear's debrief column grows: wider, its
// lines run past where the eye takes in the centered stats.
const debriefW = 96

// trialNextStep is a passed trial's next step: the module to go on to, or
// back to RTFM when no module is left to earn a capability (End.Learn).
func trialNextStep(l *Learn) string {
	if l.Title == "" {
		return ui.Strong.Render(learnKey) + " back to RTFM"
	}
	return ui.Strong.Render(learnKey) + " next module: " + l.Title
}
