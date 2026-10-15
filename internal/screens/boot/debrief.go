package boot

import (
	"fmt"
	"math"
	"strings"
	"time"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// The end of a boot (retention RT1; UX J1, J6, J7): what the boot kept,
// the one lever behind how it ended, and one next step, under the crash's
// quote or the clear's stats. game.Debrief decides what to say; this file
// says it.

// End is what the end screens and the merge window show beyond the boot
// itself: figures the app reads from the store and the settings. The zero
// End shows only what the boot knows.
type End struct {
	// Goal is apt's next package to buy (D25, retention RT3); nil when there
	// is none.
	Goal *Goal
	// ETA is the player's median uptime on the boot's own preset, 0 when
	// unknown (GR3: an ETA is the player's own, never a guess).
	ETA time.Duration
	// Reboots are the other presets offered where "one more" is decided
	// (RT7), short and patch: each with its key and the player's median
	// uptime on it.
	Reboots []Reboot
	// Today is how long the player has played today; Budget is the
	// session budget (RT12), 0 for none. Past it, one line says so.
	Today, Budget time.Duration
	// Firsts are up to two first-time or personal-best lines for a clear
	// (RT1): "first Heisenbug kill", "fastest clear: 1:52:10".
	Firsts []string
}

// Goal is the next apt package and the player's own pace toward it.
type Goal struct {
	Name  string
	Price int64
	// Per is the player's median bytes committed per boot (the last ten),
	// 0 when unknown; the ETA counts boots at that rate.
	Per int64
}

// Reboot is a preset offered beside the boot's own.
type Reboot struct {
	Key, Preset string
	ETA         time.Duration // the player's median uptime on it, 0 unknown
}

// taleSource is the story the end screens tell (docs/story/text.md):
// crash epitaphs, the dump's verse, the cleared lines, the boss's defeat
// and the beat waiting at the boot's end. Tests pin it to a stub, as they
// do fl.
type taleSource struct {
	Story     func(hook string, seed uint64) string
	Block     func(hook string) []string
	NextBeat  func(st *game.State, m *game.Meta, gap int) (string, bool)
	BeatTitle func(id string) string
	BeatLines func(id string) []string
}

var realTale = taleSource{
	Story: flavor.Story, Block: flavor.StoryBlock, NextBeat: flavor.NextBeat,
	BeatTitle: flavor.BeatTitle, BeatLines: flavor.BeatLines,
}

var tale = realTale

// beatKey reads the story beat waiting at the boot's end (screens.ReadStory).
const beatKey = "b"

// beat is the story beat the end of st has waiting, "" for none. At most
// one per boot end; it waits, unread, for the next boot end otherwise
// (retention RT6).
func beat(env Env) string {
	if env.State.Trial != nil {
		return ""
	}
	id, ok := tale.NextBeat(env.State, &env.Meta, flavor.DefaultBeatGap)
	if !ok {
		return ""
	}
	return id
}

// teaser is the beat's first line, the reason to read it, then its key.
func teaser(id string, width int) []string {
	if id == "" {
		return nil
	}
	first := tale.BeatTitle(id)
	if ls := tale.BeatLines(id); len(ls) > 0 {
		first = ls[0]
	}
	key := ui.Muted.Render(ui.G.Sep) + ui.Strong.Render(beatKey) + ui.Muted.Render(" read on")
	room := max(width-widthOf(key)-labelW, 10)
	return []string{strings.Repeat(" ", labelW) + ui.Voice.Render(ui.Clip(first, room)) + key}
}

// ---- sections ------------------------------------------------------------------

// labelW is the label column of a section (C5): two cells of indent and
// the label, padded.
const labelW = 9

// brk in a section's items starts a new line: the items before and after
// it are different kinds of news.
const brk = "\n"

// section lays items out after label, joined with " · " and wrapped
// between items to width; an item too wide for a line wraps on its own.
func section(label string, items []string, width int) []string {
	return sectionAt("  "+ui.Strong.Render(fmt.Sprintf("%-*s", labelW-2, label)), items, width)
}

// nextSection is the next step's section: its label carries the cursor,
// the one highlighted step on the screen (RT1).
func nextSection(item string, width int) []string {
	return sectionAt("  "+ui.Strong.Render("next")+" "+ui.Focus.Render(ui.G.Cursor)+" ", []string{item}, width)
}

// sectionAt is section with its first line's label already drawn, labelW
// cells wide.
func sectionAt(label string, items []string, width int) []string {
	room := max(width-labelW, 12)
	var out []string
	cur := ""
	flush := func() {
		if cur != "" {
			out = append(out, cur)
			cur = ""
		}
	}
	sep := ui.Muted.Render(ui.G.Sep)
	for _, it := range items {
		if it == "" {
			continue
		}
		switch {
		case it == brk:
			flush()
		case cur == "" && widthOf(it) <= room:
			cur = it
		case cur != "" && widthOf(cur)+widthOf(sep)+widthOf(it) <= room:
			cur += sep + it
		case widthOf(it) <= room:
			flush()
			cur = it
		default:
			flush()
			out = append(out, ui.WrapN(it, room, 2)...)
		}
	}
	flush()
	for i, l := range out {
		pre := strings.Repeat(" ", labelW)
		if i == 0 {
			pre = label + strings.Repeat(" ", max(labelW-widthOf(label), 0))
		}
		out[i] = pre + l
	}
	return out
}

// piece is one block of an end screen. Required pieces (pri 0) always
// show; the rest show while there is room, the lowest pri first.
type piece struct {
	lines []string
	pri   int
}

// fit keeps the required pieces and as many optional ones as room holds,
// by priority, in their own order.
func fit(ps []piece, room int) [][]string {
	used := 0
	for _, p := range ps {
		if p.pri == 0 {
			used += len(p.lines)
		}
	}
	keep := make([]bool, len(ps))
	for pri := 1; pri <= 9; pri++ {
		for i, p := range ps {
			if p.pri == pri && len(p.lines) > 0 && used+len(p.lines) <= room {
				keep[i], used = true, used+len(p.lines)
			}
		}
	}
	var out [][]string
	for i, p := range ps {
		if (p.pri == 0 || keep[i]) && len(p.lines) > 0 {
			out = append(out, p.lines)
		}
	}
	return out
}

func rows(ps [][]string) int {
	n := 0
	for _, p := range ps {
		n += len(p)
	}
	return n
}

// ---- kept ------------------------------------------------------------------------

// ratingItem is the rating's move: "rating 1180 → 1141 ▼39". The numerals
// take their rank color (R7). A crash never paints the move as a gain
// (GR3: no win cues on a crash).
func ratingItem(c game.Change, label string, crashed bool) string {
	now := ui.Rank(c.After).Render(fmt.Sprintf("%.0f", c.After))
	if !c.Known {
		return ui.Muted.Render(label+" ") + now
	}
	d := int(math.Round(c.Delta()))
	var move string
	switch {
	case d > 0:
		move = " ▲" + fmt.Sprint(d)
		if !crashed {
			move = " " + ui.OK.Render("▲"+fmt.Sprint(d))
		}
	case d < 0:
		move = " ▼" + fmt.Sprint(-d)
	default:
		move = ui.Muted.Render(" ±0")
	}
	before := ui.Rank(c.Before).Render(fmt.Sprintf("%.0f", c.Before))
	return ui.Muted.Render(label+" ") + before + ui.Muted.Render(" "+ui.G.Arrow+" ") + now + move
}

// goalItem is the bar toward apt's next package (RT3), with the ETA at the
// player's own pace: "apt strace ▓▓▓▓░░░░ 312/512 B · ≈2 boots".
func goalItem(g *Goal, bank int64) string {
	if g == nil || g.Name == "" {
		return ""
	}
	if bank >= g.Price {
		return ui.Muted.Render("apt ") + ui.Strong.Render(g.Name) + ui.Muted.Render(" affordable at GRUB")
	}
	const cells = 8
	fill := 0
	if g.Price > 0 {
		fill = int(min(max(bank, 0)*cells/g.Price, cells-1))
	}
	bar := ui.OK.Render(strings.Repeat(ui.G.Fill, fill)) + ui.Muted.Render(strings.Repeat(ui.G.Empty, cells-fill))
	s := ui.Muted.Render("apt ") + g.Name + " " + bar + " " + units.Bytes(max(bank, 0)) + "/" + units.Bytes(g.Price)
	if g.Per > 0 {
		n := int((g.Price - bank + g.Per - 1) / g.Per)
		s += ui.Muted.Render(ui.G.Sep + ui.G.Approx + units.Plural(n, "boot"))
	}
	return s
}

// mergeGoal is the merge window's goal bar (RT3), under its commit line in
// the 9-cell label column: the bank toward apt's next package, and
// whether committing what is uncommitted would buy it.
func mergeGoal(env Env) []string {
	g := env.End.Goal
	if env.State.Trial != nil || g == nil || g.Name == "" {
		return nil
	}
	s := goalItem(g, env.Meta.Bank)
	if bank, up := env.Meta.Bank, env.State.Uncommitted; bank < g.Price && bank+up >= g.Price {
		// The ETA is moot: this window's commit buys it.
		now := *g
		now.Per = 0
		s = goalItem(&now, bank) + ui.Muted.Render(ui.G.Sep+"committing affords it")
	}
	return []string{strings.Repeat(" ", 11) + s}
}

// dumpAtRisk is the in-door status line for the core dump the boot puts
// at risk (RT4): a crash in this boot destroys it.
func dumpAtRisk(st *game.State, width int) []string {
	a := st.AtStart
	if st.Trial != nil || a == nil || a.Dump == nil || a.Dump.Bytes <= 0 {
		return nil
	}
	return []string{ui.Clip(ui.Danger.Render(ui.G.Crash)+" "+units.Bytes(a.Dump.Bytes)+ui.Muted.Render(" core dump at risk"), width)}
}

// gateItem is the next prestige's progress (D31): its name, the rated
// doors as a bar, and what else it needs.
func gateItem(g game.PrestigeGate, top int) string {
	if g.Next == 0 {
		return ""
	}
	name := prestigeLabel(g.Next)
	if g.Open() {
		return ui.Strong.Render(name) + ui.Muted.Render(" opens with a clear at "+prestigeLabel(top))
	}
	const cells = 8
	fill := 0
	if g.Need > 0 {
		fill = min(g.Rated*cells/g.Need, cells-1)
	}
	bar := ui.OK.Render(strings.Repeat(ui.G.Fill, fill)) + ui.Muted.Render(strings.Repeat(ui.G.Empty, cells-fill))
	return ui.Strong.Render(name) + " " + bar + fmt.Sprintf(" %d/%d", g.Rated, g.Need) + ui.Muted.Render(" rated doors, then a clear")
}

// dumpItem is the core dump's fate, the crash's headline.
func dumpItem(d game.Dump, title string) string {
	switch d.Kind {
	case game.DumpLive:
		return "core dumped: " + ui.Strong.Render(units.Bytes(d.Bytes)) + fmt.Sprintf(" on %q", title)
	case game.DumpRecovered:
		return "core dump recovered: " + ui.OK.Render("+"+units.Bytes(d.Recovered)) + ui.Muted.Render(" banked")
	case game.DumpTrial:
		return ui.Muted.Render("no core dump: a trial loses its " + units.Bytes(d.Bytes) + " outright")
	case game.DumpGone:
		return ui.Muted.Render("no core dump left to recover")
	}
	return ui.Muted.Render("no core dump: nothing was uncommitted")
}

// lostHeadline is the earlier dump this crash destroyed (RT4, UX J7), in
// the panic's own red.
func lostHeadline(env Env, d game.Dump) string {
	if d.Lost == nil {
		return ""
	}
	s := "core dump lost: " + units.Bytes(d.Lost.Bytes)
	if d.Lost.Problem != "" {
		s += fmt.Sprintf(" on %q", env.problem(d.Lost.Problem).Title)
	}
	return ui.Danger.Render(ui.Strong.Render(s))
}

// regressionItem is when the crashed door comes back.
func regressionItem(g *game.Regression, now time.Time) string {
	if g == nil {
		return ""
	}
	loc := now.Location()
	if now.IsZero() {
		loc = time.UTC
	}
	return ui.Warn.Render(ui.G.Regression) + " back as a door " + g.Due.In(loc).Format("Jan 2")
}

// easeItems is what easing the boot ran with (retention RT8): listed in
// kept, so a clear on an eased boot is not read as more skill than it
// shows.
//
// NOTE: the engine lane adds State.Ease (each with a Why naming the
// doors or boots behind it). Once it lands, this returns one item per
// easing, "eased: +1 ♥ · <Why>", and "declined" when State.Declined.
func easeItems(*game.State) []string { return nil }

// ---- lever -----------------------------------------------------------------------

// leverText is the one fact about the task that decided the boot (RT1): a
// test, a time, a tool or a topic, never the player. hint is what the
// verdict says to try.
func leverText(env Env, l game.Lever) string {
	tests := ""
	if l.Total > 0 {
		tests = fmt.Sprintf("%d/%d tests", l.Passed, l.Total)
	}
	v := string(l.Verdict)
	verdict := func() string {
		if v == "" {
			return ""
		}
		return ui.Verdict(v).Render(v) + " "
	}
	var s string
	switch l.Kind {
	case game.LeverTimer:
		s = ui.G.Timer + units.Clock(l.Timer) + " ran out"
		if l.Submits == 0 {
			s += " before a :Submit"
		} else {
			s += " at " + verdict() + tests
		}
	case game.LeverLives:
		s = units.Plural(l.Failed, "failed submit") + ", the best " + verdict() + tests
	case game.LeverCursed:
		s = "cursed: the first failed submit ended it, " + verdict() + tests
	case game.LeverGaveUp:
		s = ":q! at " + units.Clock(l.Elapsed) + " of " + units.Clock(l.Timer)
		if l.Submits > 0 {
			s += ", the best " + verdict() + tests
		}
	case game.LeverCleared:
		s = fmt.Sprintf("%q %s cleared in %s of %s", env.problem(l.Door.Problem).Title, ui.Rating(l.Door.Rating),
			units.Clock(l.Elapsed), units.Clock(l.Timer))
		if l.Quacks > 0 {
			s += ui.Muted.Render(ui.G.Sep) + units.Plural(l.Quacks, "duck")
		}
	default:
		return ""
	}
	if h := verdictHint(l); h != "" {
		s += " " + ui.G.Arrow + " " + h
	}
	if l.Predicted > 0 {
		p := fmt.Sprintf("%s%d%% predicted", ui.G.Approx, int(l.Predicted*100+0.5))
		if l.Door.Cursed {
			p += ", curse not counted"
		}
		s += ui.Muted.Render(ui.G.Sep + p)
	}
	return s
}

// verdictHint is where the verdict points: what was right, and what to
// look at next. Upsolves show the failing test, which a boot hides.
func verdictHint(l game.Lever) string {
	if l.Kind == game.LeverCleared || l.Total == 0 && l.Verdict == "" {
		return ""
	}
	switch l.Verdict {
	case "WA":
		return "a case the samples missed; an upsolve shows the failing test"
	case "TLE":
		if l.Passed > 0 {
			return "the answers held, the complexity did not"
		}
		return "the complexity"
	case "MLE", "OLE":
		return "the memory, not the answers"
	case "RE":
		return "a run-time error; an upsolve shows the failing test"
	case "CE":
		return "it never compiled"
	}
	return ""
}

// ---- next ------------------------------------------------------------------------

// eta is a duration as an estimate: "≈30m", "≈1h41". 0 is unknown: "".
func eta(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	d = d.Round(time.Minute)
	if d < time.Hour {
		return fmt.Sprintf(" %s%dm", ui.G.Approx, int(d.Minutes()))
	}
	return fmt.Sprintf(" %s%dh%02d", ui.G.Approx, int(d.Hours()), int(d.Minutes())%60)
}

// upsolveText is the next step when a dump is live: what upsolving pays,
// with and without the editorial (RT4), and the package it pays toward
// (RT3).
func upsolveText(env Env, n game.Next, d game.Dump) string {
	title := env.problem(n.Problem).Title
	s := ui.Strong.Render("u") + fmt.Sprintf(" upsolve %q untimed: ", title) + ui.OK.Render("+"+units.Bytes(n.Bytes))
	if n.Bytes != d.Bytes {
		s += ui.Muted.Render(fmt.Sprintf(" (%s after the editorial)", units.Bytes(d.Bytes)))
	}
	if g := env.End.Goal; g != nil && g.Name != "" && env.Meta.Bank < g.Price {
		after := env.Meta.Bank + n.Bytes
		if after >= g.Price {
			s += ui.Muted.Render(" "+ui.G.Arrow+" affords ") + g.Name
		} else {
			s += ui.Muted.Render(" "+ui.G.Arrow+" ") + g.Name + " " + units.Bytes(after) + "/" + units.Bytes(g.Price)
		}
	}
	return s
}

// budgetLine is the session budget's nudge (RT12): one line at a boot
// boundary, never a lock, past the budget only. Nothing at the end of a
// boot expires, and it says so.
func budgetLine(e End, width int) []string {
	if e.Budget <= 0 || e.Today < e.Budget {
		return nil
	}
	s := fmt.Sprintf("today %s of %s", hm(e.Today), hm(e.Budget)) + ui.G.Sep + "nothing waiting here expires"
	return []string{ui.Clip("  "+ui.Muted.Render(s), width)}
}

// hm is a long span as hours and minutes: "3:12".
func hm(d time.Duration) string {
	d = d.Truncate(time.Minute)
	return fmt.Sprintf("%d:%02d", int(d.Hours()), int(d.Minutes())%60)
}

// rebootKey reports whether key starts a boot from an end screen: r for
// the boot's own preset, or an offered preset's key. ok is false for any
// other key.
func rebootKey(env Env, key string) (screens.StartBoot, bool) {
	st := env.State
	if key == "r" {
		return screens.StartBoot{Prestige: next(env), Preset: st.Rules.Name}, true
	}
	for _, r := range env.End.Reboots {
		if r.Key == key {
			return screens.StartBoot{Prestige: next(env), Preset: r.Preset}, true
		}
	}
	return screens.StartBoot{}, false
}

// next is the prestige a reboot starts at: the highest unlocked, which the
// engine has already raised if this clear unlocked a new one.
func next(env Env) int {
	return min(max(env.Meta.Prestige, env.State.Prestige), game.MaxPrestige)
}

// rebootPairs are the footer's reboot keys with their ETAs.
func rebootPairs(env Env) []string {
	pairs := []string{"r", "reboot" + eta(env.End.ETA)}
	for _, r := range env.End.Reboots {
		pairs = append(pairs, r.Key, r.Preset+eta(r.ETA))
	}
	return pairs
}

// voiceLines is story text in the flavor voice (italic, muted), wrapped to
// width under an indent.
func voiceLines(text []string, indent, width int) []string {
	var out []string
	pad := strings.Repeat(" ", indent)
	for _, t := range text {
		for _, l := range ui.WrapN(t, max(width-indent, 10), 2) {
			out = append(out, pad+ui.Voice.Render(ui.Muted.Render(l)))
		}
	}
	return out
}
