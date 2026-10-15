package meta

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Log is git log: one line per boot, newest first, in the shape of
// `git log --graph --oneline`. Enter expands the highlighted boot into its
// attempts; f cycles an outcome filter.
type Log struct {
	all    []BootRow // newest first
	filter int       // index into logFilters
	shown  []int     // indexes into all that pass the filter
	sel    int       // index into shown
	open   int       // Boot number expanded, 0 for none
	top    int       // first visible line
	nameOf func(int) string
	h      int // list rows at the last View, for paging
}

type logFilter struct {
	name string
	keep func(BootRow) bool
}

var logFilters = []logFilter{
	{"all", func(BootRow) bool { return true }},
	{"cleared", func(b BootRow) bool { return b.Cleared }},
	{"crashed", func(b BootRow) bool { return !b.Cleared && b.Cause != "" }},
	{"timer", func(b BootRow) bool { return !b.Cleared && b.Cause == game.TimedOut }},
	{"out of lives", func(b BootRow) bool { return !b.Cleared && b.Cause == game.OutOfLives }},
	{":q!", func(b BootRow) bool { return !b.Cleared && b.Cause == game.GaveUp }},
}

// NewLog builds git log over boots in any order. prestigeName labels
// prestige levels in the expanded view; nil means flavor.PrestigeName.
func NewLog(boots []BootRow, prestigeName func(int) string) *Log {
	if prestigeName == nil {
		prestigeName = flavor.PrestigeName
	}
	all := slices.Clone(boots)
	slices.SortStableFunc(all, func(a, b BootRow) int { return b.Boot - a.Boot })
	markNightlies(all)
	l := &Log{all: all, nameOf: prestigeName}
	l.apply()
	return l
}

func (l *Log) apply() {
	l.shown = l.shown[:0]
	for i, b := range l.all {
		if logFilters[l.filter].keep(b) {
			l.shown = append(l.shown, i)
		}
	}
	l.sel, l.top = 0, 0
}

func (l *Log) selected() *BootRow {
	if len(l.shown) == 0 {
		return nil
	}
	return &l.all[l.shown[l.sel]]
}

func (l *Log) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return l, nil
	}
	last := max(len(l.shown)-1, 0)
	page := max(l.h-1, 1)
	switch k.String() {
	case "q", "esc":
		return l, screens.Emit(screens.Back{})
	case "up", "k":
		l.sel = max(l.sel-1, 0)
	case "down", "j":
		l.sel = min(l.sel+1, last)
	case "pgup", "b", "ctrl+u":
		l.sel = max(l.sel-page, 0)
	case "pgdown", "space", "ctrl+d":
		l.sel = min(l.sel+page, last)
	case "home", "g":
		l.sel = 0
	case "end", "G":
		l.sel = last
	case "enter", "tab", "o":
		if b := l.selected(); b != nil {
			if l.open == b.Boot {
				l.open = 0
			} else {
				l.open = b.Boot
			}
		}
	case "r":
		// Any boot's seed plays again, a past nightly's too: nothing
		// here expires (retention RT15).
		if b := l.selected(); b != nil {
			return l, screens.Emit(screens.StartBoot{Prestige: b.Prestige, Preset: cmp.Or(b.Preset, standardPreset), Seed: b.Seed})
		}
	case "f":
		l.filter = (l.filter + 1) % len(logFilters)
		l.apply()
	case "F":
		l.filter = (l.filter + len(logFilters) - 1) % len(logFilters)
		l.apply()
	}
	return l, nil
}

func (l *Log) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	cleared, crashed := 0, 0
	for _, b := range l.all {
		switch {
		case b.Cleared:
			cleared++
		case b.Cause != "":
			crashed++
		}
	}
	head := ui.Spread(
		ui.Strong.Render("git log --graph --oneline")+
			ui.Muted.Render(fmt.Sprintf("  %s%s%d cleared%s%d crashed", units.Plural(len(l.all), "boot"), ui.G.Sep, cleared, ui.G.Sep, crashed)),
		"filter: "+ui.Focus.Render(logFilters[l.filter].name),
		w)
	keys := ui.Footer(w, "j/k", "move", "enter", "expand", "r", "replay seed", "f", "filter", "g/G", "ends", "q", "back")

	rows := max(h-2, 0)
	l.h = rows
	var body []string
	switch {
	case len(l.all) == 0:
		body = []string{"fatal: your current branch 'main' does not have any commits yet"}
	case len(l.shown) == 0:
		body = []string{ui.Muted.Render("no boots match filter: " + logFilters[l.filter].name)}
	default:
		lines, selAt, selEnd := l.lines(w)
		// Keep the selected boot on screen, and its expansion too when
		// it fits.
		if selAt < l.top {
			l.top = selAt
		}
		if selEnd >= l.top+rows {
			l.top = min(selAt, selEnd-rows+1)
		}
		l.top = max(min(l.top, len(lines)-rows), 0)
		body = lines[l.top:min(l.top+rows, len(lines))]
	}

	out := append([]string{head}, body...)
	for len(out) < h-1 {
		out = append(out, "")
	}
	out = append(out[:h-1], keys)
	return ui.Layout(w, h, out, "")
}

// lines renders every shown boot, with the expansion under the open one.
// selAt and selEnd are the first and last line of the selected boot.
func (l *Log) lines(w int) (out []string, selAt, selEnd int) {
	numW := len(fmt.Sprint(l.all[0].Boot)) + 1
	wide := w >= 100
	for i, idx := range l.shown {
		b := l.all[idx]
		if i == l.sel {
			selAt = len(out)
		}
		line := l.bootLine(b, w, numW, wide)
		if i == l.sel {
			line = ui.Select.Render(ui.Fit(ansi.Strip(line), w))
		}
		out = append(out, line)
		if b.Boot == l.open {
			out = append(out, l.details(b, w)...)
		}
		if i == l.sel {
			selEnd = len(out) - 1
		}
	}
	return out, selAt, selEnd
}

// bootLine is one boot in at most w cells. Below 100 columns the date drops
// its time so "crashed in <biome> (<cause>)" keeps room to be read.
func (l *Log) bootLine(b BootRow, w, numW int, wide bool) string {
	// git's graph star carries the outcome; the words stay plain except a
	// crash, which red may paint (R2).
	star := ui.Warn.Render("*")
	switch {
	case b.Cleared:
		star = ui.OK.Render("*")
	case b.Cause != "":
		star = ui.Danger.Render("*")
	}
	hash := ui.Warn.Render(shortHash(b)) // git's yellow hash: the parody keeps it (R8)
	date := b.Started.Format("Jan 02")
	if wide {
		date = b.Started.Format("2006-01-02 15:04")
	}
	left := fmt.Sprintf("%s %s %s %s P%d %7s ", star, hash,
		ui.Fit(fmt.Sprintf("#%d", b.Boot), numW), ui.Muted.Render(date), b.Prestige, units.Clock(b.Uptime))
	const bytesW = 8 // "+1.2 KiB", "+312 KiB"; a boot banking 10-99 KiB runs one over
	bytes := ui.Muted.Render(ui.AlignRight("0 B", bytesW))
	if b.Banked > 0 {
		bytes = ui.OK.Render(ui.AlignRight("+"+units.Bytes(b.Banked), bytesW))
	}
	room := w - ansi.StringWidth(left) - bytesW - 1
	if tag := seedTag(b); tag != "" && room > 30 {
		room -= ansi.StringWidth(tag)
		return left + ui.Fit(outcomeText(b, room), room) + ui.Muted.Render(tag) + " " + bytes
	}
	return left + ui.Fit(outcomeText(b, room), room) + " " + bytes
}

// shortHash stands in for a commit hash: 7 hex digits mixed from the seed
// and boot number (splitmix64's finalizer), so boots with small or similar
// seeds still get distinct, stable hashes. The full seed is in the
// expansion.
func shortHash(b BootRow) string {
	z := b.Seed ^ uint64(b.Boot)*0x9e3779b97f4a7c15
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	z ^= z >> 31
	return fmt.Sprintf("%07x", z>>36)
}

// outcomeText says how a boot ended in at most room cells. The biome name
// gives way first: the cause is the part worth reading.
func outcomeText(b BootRow, room int) string {
	switch {
	case b.Cleared:
		return "cleared"
	case b.Cause == "":
		return "abandoned"
	}
	cause := " (" + causeText(b.Cause) + ")"
	if b.Biome == "" {
		return ui.Danger.Render("crashed" + cause)
	}
	biome := b.Biome
	if n := len("crashed in ") + ansi.StringWidth(biome) + ansi.StringWidth(cause); n > room {
		biome = ui.Clip(biome, max(ansi.StringWidth(biome)-(n-room), 1))
	}
	return ui.Danger.Render("crashed in " + biome + cause)
}

// details is the expansion: a summary line, then one line per attempt.
func (l *Log) details(b BootRow, w int) []string {
	gutter := ui.Muted.Render("│") + "   "
	preset := b.Preset
	if preset == "" {
		preset = standardPreset
	}
	sum := fmt.Sprintf("started %s · seed %x · preset %s · %s · banked %s",
		b.Started.Format("2006-01-02 15:04"), b.Seed, preset, prestigeText(b.Prestige, l.nameOf), units.Bytes(b.Banked))
	if b.Dumped > 0 {
		sum += " · dumped " + units.Bytes(b.Dumped)
	}
	if b.Problem != "" && !b.Cleared {
		sum += " · on " + b.Problem
	}
	var out []string
	for _, line := range strings.Split(ansi.Wordwrap(sum, max(w-4, 10), ""), "\n") {
		out = append(out, gutter+ui.Muted.Render(line))
	}
	if len(b.Attempts) == 0 {
		return append(out, gutter+ui.Muted.Render("no doors opened"))
	}
	lay := narrowAttempt
	if w >= 100 {
		lay = wideAttempt
	}
	for _, a := range b.Attempts {
		out = append(out, gutter+attemptLine(a, lay))
	}
	return out
}

// attemptLayout sizes the columns of an attempt line. Narrow terminals drop
// the door's timer: elapsed time alone still says how close it was.
type attemptLayout struct {
	title, topic int
	timer        bool
}

var (
	narrowAttempt = attemptLayout{title: 18, topic: 19}
	wideAttempt   = attemptLayout{title: 28, topic: 21, timer: true}
)

func attemptLine(a AttemptRow, lay attemptLayout) string {
	mark := ui.Danger.Render(ui.G.Fail)
	switch a.Outcome {
	case game.Cleared:
		mark = ui.OK.Render(ui.G.Pass)
	case game.Retreated:
		mark = ui.Warn.Render(ui.G.Retreat)
	case "": // still open
		mark = ui.Focus.Render(ui.G.Here)
	}
	// The door's difficulty follows its topic (D27). When both do not fit
	// the column, the topic is cut, never the numeral (§2.6).
	topic := a.Topic
	if a.Rating > 0 {
		r := ui.Rating(a.Rating)
		topic = ui.Clip(a.Topic, lay.topic-1-ansi.StringWidth(r)) + " " + r
	}
	t := ui.AlignRight(units.Clock(a.Elapsed), 5)
	if lay.timer {
		tm := ""
		if a.Timer > 0 {
			tm = "/" + units.Clock(a.Timer)
		}
		t = ui.AlignRight(units.Clock(a.Elapsed)+tm, 11)
	}
	subs := fmt.Sprintf("%d sub", a.Submits)
	if a.Failed > 0 {
		subs += fmt.Sprintf(" %d%s", a.Failed, ui.G.Fail)
	}
	end := string(a.Kind)
	if a.Outcome != game.Cleared {
		end = ui.Danger.Render(causeText(a.Outcome))
	}
	var tags []string
	if a.Boss != "" {
		tags = append(tags, "boss "+a.Boss)
	}
	for _, f := range []struct {
		on   bool
		name string
	}{{a.Cursed, "cursed"}, {a.Mystery, "mystery"}, {a.Blind && !a.Mystery, "blind"}, {a.Regress, "regression"}} {
		if f.on {
			tags = append(tags, f.name)
		}
	}
	if len(tags) > 0 {
		end += ui.Muted.Render(" [" + strings.Join(tags, ", ") + "]")
	}
	return fmt.Sprintf("%s %s %s %s %s %s%d %s", mark, ui.Fit(a.Title, lay.title), ui.Fit(topic, lay.topic),
		t, ui.Fit(subs, 8), ui.G.Duck, a.Quacks, end)
}

// markNightlies tags the nightly builds among boots (newest first): a
// boot on its own day's NightlySeed, the first played on a seed marked
// First; the rest on that seed are practice (retention RT15). Boots older
// than the list are not seen, so the oldest listed counts as first.
func markNightlies(boots []BootRow) {
	seen := map[uint64]bool{}
	for i := len(boots) - 1; i >= 0; i-- {
		b := &boots[i]
		if b.Seed == 0 || b.Seed != NightlySeed(b.Started) {
			continue
		}
		b.Nightly, b.First = true, !seen[b.Seed]
		seen[b.Seed] = true
	}
}

// seedTag marks a nightly in its row: " nightly · first" for the first
// play of the day's seed, " nightly" for a replay.
func seedTag(b BootRow) string {
	switch {
	case !b.Nightly:
		return ""
	case b.First:
		return " nightly" + ui.G.Sep + "first"
	}
	return " nightly"
}
