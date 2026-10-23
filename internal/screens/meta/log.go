package meta

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Log is git log: one line per boot and per upsolve, newest first, in the
// shape of `git log --graph --oneline`. Enter expands the highlighted line
// into a boot's attempts or an upsolve's verdicts; f cycles a filter. Its
// calendar, when it has one, heads the list (calendar.go); c moves the keys
// into it.
type Log struct {
	all    []BootRow    // newest first
	ups    []UpsolveRow // newest first
	rows   []logRow     // all and ups by time, newest first
	filter int          // index into logFilters
	shown  []int        // indexes into rows that pass the filter
	sel    int          // index into shown
	open   *logRow      // the expanded line, nil for none
	top    int          // first visible line
	cal    *calendar    // nil without one
	nameOf func(int) string
	h      int // list rows at the last View, for paging
}

// logRow is one line of git log: a boot or an upsolve.
type logRow struct {
	boot *BootRow
	up   *UpsolveRow
}

// at is when the line's boot started or its upsolve opened.
func (r logRow) at() time.Time {
	if r.boot != nil {
		return r.boot.Started
	}
	return r.up.Opened
}

type logFilter struct {
	name string
	keep func(logRow) bool
}

// bootsWhere keeps the boots keep does, and no upsolve.
func bootsWhere(keep func(BootRow) bool) func(logRow) bool {
	return func(r logRow) bool { return r.boot != nil && keep(*r.boot) }
}

var logFilters = []logFilter{
	{"all", func(logRow) bool { return true }},
	{"cleared", bootsWhere(func(b BootRow) bool { return b.Cleared })},
	{"crashed", bootsWhere(func(b BootRow) bool { return !b.Cleared && b.Cause != "" })},
	{"timer", bootsWhere(func(b BootRow) bool { return !b.Cleared && b.Cause == game.TimedOut })},
	{"out of cycles", bootsWhere(func(b BootRow) bool { return !b.Cleared && b.Cause == game.OutOfLives })},
	{":q!", bootsWhere(func(b BootRow) bool { return !b.Cleared && b.Cause == game.GaveUp })},
	// Last, so F from all is one key from either.
	{"boots", bootsWhere(func(BootRow) bool { return true })},
	{"upsolves", func(r logRow) bool { return r.up != nil }},
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
	l.merge()
	l.apply()
	return l
}

// WithUpsolves lists ups, in any order, among the boots by time.
func (l *Log) WithUpsolves(ups []UpsolveRow) *Log {
	l.ups = slices.Clone(ups)
	slices.SortStableFunc(l.ups, func(a, b UpsolveRow) int { return b.Opened.Compare(a.Opened) })
	l.merge()
	l.apply()
	return l
}

// WithCalendar heads git log with c.
func (l *Log) WithCalendar(c Calendar) *Log {
	l.cal = newCalendar(c)
	return l
}

// merge lays the boots and upsolves out by time, newest first. Boots keep
// their order by number whatever their clocks say; an upsolve goes above
// the newest boot that started before it opened.
func (l *Log) merge() {
	l.rows, l.open = l.rows[:0], nil
	i, j := 0, 0
	for i < len(l.all) || j < len(l.ups) {
		if j == len(l.ups) || i < len(l.all) && !l.ups[j].Opened.After(l.all[i].Started) {
			l.rows = append(l.rows, logRow{boot: &l.all[i]})
			i++
		} else {
			l.rows = append(l.rows, logRow{up: &l.ups[j]})
			j++
		}
	}
}

func (l *Log) apply() {
	l.shown = l.shown[:0]
	for i, r := range l.rows {
		if logFilters[l.filter].keep(r) {
			l.shown = append(l.shown, i)
		}
	}
	l.sel, l.top = 0, 0
}

func (l *Log) selected() *logRow {
	if len(l.shown) == 0 {
		return nil
	}
	return &l.rows[l.shown[l.sel]]
}

func (l *Log) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return l, nil
	}
	if l.cal != nil && l.cal.focus {
		switch s := k.String(); s {
		case "q", "r", "f", "F": // the list's, as ever
		case "enter":
			l.showDay()
			return l, nil
		default:
			l.cal.key(s)
			return l, nil
		}
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
		if r := l.selected(); r != nil {
			if l.open == r {
				l.open = nil
			} else {
				l.open = r
			}
		}
	case "r":
		// Any boot's seed plays again, a past nightly's too: nothing
		// here expires (retention RT15). An upsolve has no seed.
		if r := l.selected(); r != nil && r.boot != nil {
			b := r.boot
			return l, screens.Emit(screens.StartBoot{Prestige: b.Prestige, Preset: cmp.Or(b.Preset, standardPreset), Seed: b.Seed})
		}
	case "f":
		l.filter = (l.filter + 1) % len(logFilters)
		l.apply()
	case "F":
		l.filter = (l.filter + len(logFilters) - 1) % len(logFilters)
		l.apply()
	case "c":
		if l.cal != nil {
			var at time.Time
			if r := l.selected(); r != nil {
				at = r.at()
			}
			l.cal.enter(at)
		}
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
	title := ui.Strong.Render("git log --graph --oneline")
	counts := fmt.Sprintf("  %s%s%d cleared%s%d crashed", units.Plural(len(l.all), "boot"), ui.G.Sep, cleared, ui.G.Sep, crashed)
	filter := "filter: " + ui.Focus.Render(logFilters[l.filter].name)
	// The upsolves' count goes first when the filter would not fit.
	if more := counts + ui.G.Sep + units.Plural(len(l.ups), "upsolve"); len(l.ups) > 0 &&
		ansi.StringWidth(title+more)+2+ansi.StringWidth(filter) <= w {
		counts = more
	}
	head := ui.Spread(title+ui.Muted.Render(counts), filter, w)
	keys := ui.Footer(w, "j/k", "move", "enter", "expand", "r", "replay seed", "f", "filter", "g/G", "ends", "q", "back")
	var cal []string
	if l.cal != nil {
		cal = l.cal.lines(w, h-2)
		keys = ui.Footer(w, "j/k", "move", "enter", "expand", "r", "replay seed", "f", "filter", "c", "days", "g/G", "ends", "q", "back")
		if l.cal.focus {
			keys = ui.Footer(w, "h/l", "week", "j/k", "day", "g/G", "ends", "enter", "that day", "c", "list", "q", "back")
		}
	}

	rows := max(h-2-len(cal), 0)
	l.h = rows
	var body []string
	switch {
	case len(l.rows) == 0:
		body = []string{"fatal: your current branch 'main' does not have any commits yet"}
	case len(l.shown) == 0 && logFilters[l.filter].name == "upsolves":
		body = []string{ui.Muted.Render("no upsolves yet")}
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

	out := append(append([]string{head}, cal...), body...)
	return ui.Layout(w, h, out, keys)
}

// showDay gives the keys back to the list on the newest line shown from
// the calendar cursor's day; with none, the cursor stays.
func (l *Log) showDay() {
	for i, idx := range l.shown {
		if dateKey(l.rows[idx].at().In(l.cal.Today.Location())) == dateKey(l.cal.at) {
			l.sel, l.cal.focus = i, false
			return
		}
	}
}

// lines renders every shown line, with the expansion under the open one.
// selAt and selEnd are the first and last line of the selected one.
func (l *Log) lines(w int) (out []string, selAt, selEnd int) {
	numW := 2 // "#1", and an upsolve's "up"
	if len(l.all) > 0 {
		numW = len(fmt.Sprint(l.all[0].Boot)) + 1
	}
	wide := w >= 100
	for i, idx := range l.shown {
		r := &l.rows[idx]
		if i == l.sel {
			selAt = len(out)
		}
		var line string
		if r.boot != nil {
			line = l.bootLine(*r.boot, w, numW, wide)
		} else {
			line = upsolveLine(*r.up, w, numW, wide)
		}
		if i == l.sel {
			line = ui.Select.Render(ui.Fit(ansi.Strip(line), w))
		}
		out = append(out, line)
		switch {
		case r != l.open:
		case r.boot != nil:
			out = append(out, l.details(*r.boot, w)...)
		default:
			out = append(out, upsolveDetails(*r.up, w)...)
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

// bytesW is the bytes column: "+1.2 KiB", "+312 KiB"; a line banking 10-99
// KiB runs one over.
const bytesW = 8

// shortHash stands in for a commit hash: 7 hex digits mixed from the seed
// and boot number, so boots with small or similar seeds still get
// distinct, stable hashes. The full seed is in the expansion.
func shortHash(b BootRow) string { return hash7(b.Seed ^ uint64(b.Boot)*0x9e3779b97f4a7c15) }

// upsolveHash is an upsolve's commit hash, from its problem and the moment
// it opened.
func upsolveHash(u UpsolveRow) string {
	h := fnv.New64a()
	h.Write([]byte(u.Problem))
	return hash7(h.Sum64() ^ uint64(u.Opened.UnixNano()))
}

// hash7 is 7 hex digits of z mixed by splitmix64's finalizer.
func hash7(z uint64) string {
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	z ^= z >> 31
	return fmt.Sprintf("%07x", z>>36)
}

// outcomeText says how a boot ended in at most room cells. The biome name
// gives way first: the cause is the part worth reading.
func outcomeText(b BootRow, room int) string {
	switch {
	case b.Cleared && b.Ending == game.EndingWake:
		return "awake (the Court)" // achievements.md (f): never "woke"
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
	sum := fmt.Sprintf("started %s"+ui.G.Sep+"seed %x"+ui.G.Sep+"preset %s"+ui.G.Sep+"%s"+ui.G.Sep+"banked %s",
		b.Started.Format("2006-01-02 15:04"), b.Seed, preset, prestigeText(b.Prestige, l.nameOf), units.Bytes(b.Banked))
	if b.Dumped > 0 {
		sum += ui.G.Sep + "dumped " + units.Bytes(b.Dumped)
	}
	if b.Problem != "" && !b.Cleared {
		sum += ui.G.Sep + "on " + b.Problem
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

// upsolveLine is one upsolve in at most w cells, in a boot's columns: "up"
// for its number, no prestige, took for an uptime, and the bytes a core
// dump it recovered banked. The star is green for a solve, yellow for one
// left.
func upsolveLine(u UpsolveRow, w, numW int, wide bool) string {
	star := ui.Warn.Render("*")
	if u.Solved != "" {
		star = ui.OK.Render("*")
	}
	date := u.Opened.Format("Jan 02")
	if wide {
		date = u.Opened.Format("2006-01-02 15:04")
	}
	left := fmt.Sprintf("%s %s %s %s    %7s ", star, ui.Warn.Render(upsolveHash(u)),
		ui.Fit("up", numW), ui.Muted.Render(date), units.Clock(took(u)))
	bytes := strings.Repeat(" ", bytesW)
	if u.Recovered > 0 {
		bytes = ui.OK.Render(ui.AlignRight("+"+units.Bytes(u.Recovered), bytesW))
	}
	room := w - ansi.StringWidth(left) - bytesW - 1
	return left + ui.Fit(upsolveText(u, room), room) + " " + bytes
}

// took is an upsolve's time on its line: to its first AC, which a solved
// upsolve left open after does not stretch, else the time it was open.
func took(u UpsolveRow) time.Duration {
	for _, v := range u.Verdicts {
		if v.Verdict == "AC" {
			return v.At
		}
	}
	return u.Elapsed
}

// upsolveText says how an upsolve ended in at most room cells: solved, an
// editorial-assisted solve marked, or left at its last verdict. The title
// gives way first, as a crash's biome does.
func upsolveText(u UpsolveRow, room int) string {
	verb, tail := "left ", ""
	switch {
	case u.Solved == game.Editorial:
		verb, tail = "solved ", " (editorial)"
	case u.Solved != "":
		verb = "solved "
	case len(u.Verdicts) > 0:
		tail = " (" + verdictText(u.Verdicts[len(u.Verdicts)-1]) + ")"
	}
	title := u.Title
	if n := len(verb) + ansi.StringWidth(title) + len(tail); n > room {
		title = ui.Clip(title, max(ansi.StringWidth(title)-(n-room), 1))
	}
	return verb + title + tail
}

// verdictText is a verdict with the tests it passed, "WA 23/29"; a
// compile error passes none worth counting.
func verdictText(v VerdictRow) string {
	if v.Total == 0 || v.Verdict == "CE" {
		return v.Verdict
	}
	return fmt.Sprintf("%s %d/%d", v.Verdict, v.Passed, v.Total)
}

// upsolveDetails is an upsolve's expansion: a summary line, then one line
// per full-test verdict, when it came and whether the editorial had been
// read by then.
func upsolveDetails(u UpsolveRow, w int) []string {
	gutter := ui.Muted.Render("│") + "   "
	sum := "opened " + u.Opened.Format("2006-01-02 15:04") + " for " + units.Clock(u.Elapsed) + ui.G.Sep + string(u.Problem)
	if u.Topic != "" {
		sum += ui.G.Sep + u.Topic
	}
	if u.Rating > 0 && catalog.ShowsRating(u.Problem) {
		sum += ui.G.Sep + ui.G.Rating + strconv.Itoa(u.Rating)
	}
	if u.Editorial {
		sum += ui.G.Sep + "editorial read"
	}
	if u.Recovered > 0 {
		sum += ui.G.Sep + "recovered " + units.Bytes(u.Recovered)
	}
	var out []string
	for _, line := range strings.Split(ansi.Wordwrap(sum, max(w-4, 10), ""), "\n") {
		out = append(out, gutter+ui.Muted.Render(line))
	}
	if len(u.Verdicts) == 0 {
		return append(out, gutter+ui.Muted.Render("nothing submitted"))
	}
	for _, v := range u.Verdicts {
		mark := ui.Danger.Render(ui.G.Fail)
		if v.Verdict == "AC" {
			mark = ui.OK.Render(ui.G.Pass)
		}
		line := fmt.Sprintf("%s %s %s", mark, ui.Fit(verdictText(v), 9), ui.AlignRight(units.Clock(v.At), 7))
		if v.Editorial {
			line += ui.Muted.Render(" [editorial read]")
		}
		out = append(out, gutter+line)
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
	narrowAttempt = attemptLayout{title: 18, topic: 13}
	wideAttempt   = attemptLayout{title: 28, topic: 15, timer: true}
)

// ratingW is the rating column of an attempt line: "◆1720".
const ratingW = 5

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
	// The door's difficulty (D27) has a column of its own after the topic,
	// right-aligned as numerals are, so a long topic is cut, never the
	// rating (§2.6).
	rating := ""
	if a.Rating > 0 && catalog.ShowsRating(a.Problem) {
		rating = ui.Rating(a.Rating)
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
	return fmt.Sprintf("%s %s %s %s %s %s %s%d %s", mark, ui.Fit(a.Title, lay.title), ui.Fit(a.Topic, lay.topic),
		ui.AlignRight(rating, ratingW), t, ui.Fit(subs, 8), ui.G.Duck, a.Quacks, end)
}

// markNightlies tags the nightly builds among boots (newest first): a
// boot on its own day's game.NightlySeed, the first played on a seed marked
// First; the rest on that seed are practice (retention RT15). Boots older
// than the list are not seen, so the oldest listed counts as first.
func markNightlies(boots []BootRow) {
	seen := map[uint64]bool{}
	for i := len(boots) - 1; i >= 0; i-- {
		b := &boots[i]
		if b.Seed == 0 || b.Seed != game.NightlySeed(b.Started) {
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
