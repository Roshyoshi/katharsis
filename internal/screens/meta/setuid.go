package meta

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/setuid"
	"katharsis/internal/ui"
)

// Setuid is the setuid bits' screen, `find / -perm -4000`
// (docs/design/achievements.md (g)): every bit as an ls -l line under its
// kind's directory, the selected one's barrier under the list, and at
// 140x40 a pane beside it with the barrier, the completion breakdown and
// the bits set last.
//
//	MODE        SIZE   SET           PATH
//	-rwsr-xr-x     -   Oct 14 21:07  /bin/hello-world      set
//	-rwxr-xr-x  34/60                /bin/through-the-glass a counter
//	-?????????     ?   ?             /home/you/.???        a secret
//
// A secret stays -????????? until it is set; ? shows the selected one's
// hint for this visit, s cycles the spoilers this visit shows (the
// setuid_spoilers setting is where it starts), n turns the boot's notices
// on or off. The completion percentage shows once a three-act boot was
// cleared (setuid.ShowsPercent). The bits open nothing: the screen is the
// whole of them (§(j)).
type Setuid struct {
	c       SetuidConfig
	rows    []setuidRow
	sel     int // index into rows, always a bit's
	top     int
	h       int             // list rows at the last View, for paging
	shown   map[string]bool // secrets ? showed the hint of, this visit
	spoil   string          // the spoilers this visit shows
	notices bool
}

// SetuidConfig is what the setuid screen shows.
type SetuidConfig struct {
	Meta game.Meta
	// Now dates the bits ("Oct 14 21:07", as ls -l does, with the year
	// past six months) in its own location; zero is time.Now().
	Now time.Time
	// Spoilers is the setuid_spoilers setting: SpoilersNone (the default,
	// for ""), SpoilersHints or SpoilersAll.
	Spoilers string
	// Notices is whether the boot screens say a bit set (setuid_notices).
	Notices bool
}

// The spoiler levels (setuid_spoilers): none draws a secret not set as
// -?????????, hints adds every secret's hint, all its name and barrier.
const (
	SpoilersNone  = "none"
	SpoilersHints = "hints"
	SpoilersAll   = "all"
)

// Spoilers are the levels in s's order.
var Spoilers = []string{SpoilersNone, SpoilersHints, SpoilersAll}

// SetuidHint is the intent ? emits on a secret not set: its hint is on
// screen (the app records setuid_hint).
type SetuidHint struct{ ID string }

// SetuidNotices is the intent n emits: the boot screens' setuid notices
// on or off.
type SetuidNotices struct{ On bool }

// setuidRow is a row of the list: a directory's header (def nil) or a bit.
type setuidRow struct {
	kind setuid.Kind
	def  *setuid.Def
}

// NewSetuid builds the screen: each kind's directory in setuid.Kinds's
// order, its bits in the table's, a secret set before those not set.
func NewSetuid(c SetuidConfig) *Setuid {
	if c.Now.IsZero() {
		c.Now = time.Now()
	}
	s := &Setuid{c: c, shown: map[string]bool{}, notices: c.Notices}
	s.spoil = SpoilersNone
	if slices.Contains(Spoilers, c.Spoilers) {
		s.spoil = c.Spoilers
	}
	for _, k := range setuid.Kinds {
		var defs []*setuid.Def
		for i := range setuid.Table {
			if d := &setuid.Table[i]; d.Kind == k {
				defs = append(defs, d)
			}
		}
		if k == setuid.KindSecret {
			slices.SortStableFunc(defs, func(a, b *setuid.Def) int {
				return cmpBool(!s.set(a), !s.set(b))
			})
		}
		s.rows = append(s.rows, setuidRow{kind: k})
		for _, d := range defs {
			s.rows = append(s.rows, setuidRow{kind: k, def: d})
		}
	}
	s.sel = s.next(0, 1)
	return s
}

func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

func (s *Setuid) set(d *setuid.Def) bool { return setuid.IsSet(&s.c.Meta, d.ID) }

// visible reports whether d's row shows what it is: every bit but a
// secret not set, unless the spoilers say all.
func (s *Setuid) visible(d *setuid.Def) bool {
	return !d.Secret() || s.set(d) || s.spoil == SpoilersAll
}

// hint is d's hint while the row shows it, "" otherwise: a hidden secret's,
// under ? or the hints spoilers.
func (s *Setuid) hint(d *setuid.Def) string {
	if !d.Secret() || s.visible(d) || s.spoil == SpoilersNone && !s.shown[d.ID] {
		return ""
	}
	return cmp.Or(flavor.SetuidHint(d.ID), d.Hint)
}

// Set is how many bits of the table m has set: the header's "23 of 61".
func setCount(m *game.Meta, kind setuid.Kind) (set, of int) {
	for _, d := range setuid.Table {
		if kind == "" || d.Kind == kind {
			of++
			if setuid.IsSet(m, d.ID) {
				set++
			}
		}
	}
	return set, of
}

// next is the first bit's row from i on in direction d (+1 or -1), i if
// none.
func (s *Setuid) next(i, d int) int {
	for j := i; j >= 0 && j < len(s.rows); j += d {
		if s.rows[j].def != nil {
			return j
		}
	}
	return i
}

func (s *Setuid) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	last := s.next(len(s.rows)-1, -1)
	switch k.String() {
	case "q", "esc":
		return s, screens.Emit(screens.Back{})
	case "up", "k":
		if j := s.next(s.sel-1, -1); j < s.sel && s.rows[j].def != nil {
			s.sel = j
		}
	case "down", "j":
		if j := s.next(s.sel+1, 1); j > s.sel && s.rows[j].def != nil {
			s.sel = j
		}
	case "home", "g":
		s.sel = s.next(0, 1)
	case "end", "G":
		s.sel = last
	case "pgup":
		s.sel = s.next(max(s.sel-max(s.h-1, 1), 0), 1)
	case "pgdown":
		s.sel = s.next(min(s.sel+max(s.h-1, 1), last), -1)
	case "tab", "shift+tab":
		s.sel = s.dirJump(k.String() == "tab")
	case "?":
		if d := s.rows[s.sel].def; d.Secret() && !s.visible(d) && s.spoil == SpoilersNone && !s.shown[d.ID] {
			s.shown[d.ID] = true
			return s, screens.Emit(SetuidHint{ID: d.ID})
		}
	case "s":
		s.spoil = Spoilers[(slices.Index(Spoilers, s.spoil)+1)%len(Spoilers)]
	case "n":
		s.notices = !s.notices
		return s, screens.Emit(SetuidNotices{On: s.notices})
	}
	return s, nil
}

// dirJump is the first bit of the next directory (fwd) or the previous
// one, round the ends.
func (s *Setuid) dirJump(fwd bool) int {
	ks := setuid.Kinds
	i := slices.Index(ks, s.rows[s.sel].kind)
	if fwd {
		i = (i + 1) % len(ks)
	} else {
		i = (i - 1 + len(ks)) % len(ks)
	}
	h := slices.IndexFunc(s.rows, func(r setuidRow) bool { return r.def == nil && r.kind == ks[i] })
	return s.next(h, 1)
}

// ---- view ----------------------------------------------------------------------

// setuidPane is the widest the list grows beside the pane: a hidden
// secret's row with its longest hint, and setuidWide the size from which
// the screen has the pane beside it, which keeps at least setuidRight.
const (
	setuidPane  = 84
	setuidRight = 49
	setuidWideW = 120
	setuidWideH = 30
)

// setuidNote is the wide layout's footnote: the bits are a record.
const setuidNote = "setuid bits open nothing. They record what you did, and when."

func (s *Setuid) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	wide := w >= setuidWideW && h >= setuidWideH
	footer := ui.Footer(w, "j/k", "move", "tab", "next dir", "?", "hint", "s", "spoilers", "n", "notices", "q", "back")
	if wide {
		footer = ui.Footer(w, "j/k", "move", "tab", "next dir", "?", "hint for this row", "s", "spoilers", "n", "notices", "q", "back")
	}
	out := []string{s.header(w)}
	if !wide {
		n := max(h-5, 2)
		out = append(out, s.list(w, n)...)
		out = append(out, ui.Rule(w))
		for _, l := range ui.WrapN(s.detail(), max(w-2, 10), 2) {
			out = append(out, " "+l)
		}
		return ui.Layout(w, h, out, footer)
	}
	lw := min(w-setuidRight-1, setuidPane)
	rw := w - lw - 1
	n := h - 4
	left, right := s.list(lw-1, n), s.pane(rw, n)
	bar := ui.Muted.Render(ui.G.Thin.Left)
	for i := range n {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		out = append(out, ui.Fit(l, lw)+bar+r)
	}
	out = append(out, ui.Muted.Render(strings.Repeat(ui.G.Rule, lw)+ui.G.Thin.MiddleBottom+strings.Repeat(ui.G.Rule, max(rw, 0))))
	out = append(out, " "+ui.Muted.Render(setuidNote))
	return ui.Layout(w, h, out, footer)
}

// header is the first line: the bits set, the percentage once it shows,
// and the settings at the right, notices too where they fit.
func (s *Setuid) header(w int) string {
	m := &s.c.Meta
	set, of := setCount(m, "")
	left := ui.Strong.Render("find / -perm -4000 -ls") + ui.Muted.Render(ui.G.Sep) + fmt.Sprintf("%d of %d set", set, of)
	if setuid.ShowsPercent(m) {
		pct, _ := setuid.Completion(m)
		left += ui.Muted.Render(ui.G.Sep) + ui.Strong.Render(fmt.Sprintf("%d%%", pct)) + " complete"
	}
	on := "off"
	if s.notices {
		on = "on"
	}
	right := ui.Muted.Render("spoilers: " + s.spoil + ui.G.Sep + "notices: " + on)
	if ansi.StringWidth(left)+2+ansi.StringWidth(right) > w {
		right = ui.Muted.Render("spoilers: " + s.spoil)
	}
	return ui.Spread(left, right, w)
}

// list is the column header and n rows of the list, w cells wide, the
// selection on screen; its last row says there is more where there is.
func (s *Setuid) list(w, n int) []string {
	out := []string{ui.Muted.Render(lsCols("MODE", "SIZE", "SET", "PATH"))}
	n = max(n-1, 1)
	from, to, more := s.window(n)
	for i := from; i < to; i++ {
		out = append(out, s.row(i, w))
	}
	if more != "" {
		out = append(out, ui.Muted.Render(" "+more))
	}
	return out
}

// window is the rows [from, to) n rows show with the selection among
// them, and "↓ more" (or ↑, or both) when the list does not fit, which
// takes a row of the n.
func (s *Setuid) window(n int) (from, to int, more string) {
	s.h = n
	if len(s.rows) <= n {
		return 0, len(s.rows), ""
	}
	n = max(n-1, 1)
	if s.sel < s.top {
		s.top = s.sel
	}
	if s.sel >= s.top+n {
		s.top = s.sel - n + 1
	}
	// A directory's first bit brings its header along.
	if s.top == s.sel && s.sel > 0 && s.rows[s.sel-1].def == nil && n > 1 {
		s.top--
	}
	s.top = max(min(s.top, len(s.rows)-n), 0)
	up, down := s.top > 0, s.top+n < len(s.rows)
	switch {
	case up && down:
		more = arrow(true) + arrow(false) + " more"
	case down:
		more = arrow(false) + " more"
	case up:
		more = arrow(true) + " more"
	}
	return s.top, s.top + n, more
}

// arrow is ↑ or ↓, or ^ and v under the ASCII glyphs.
func arrow(up bool) string {
	switch {
	case ui.G.Fill != ui.Unicode.Fill && up:
		return "^"
	case ui.G.Fill != ui.Unicode.Fill:
		return "v"
	case up:
		return "↑"
	}
	return "↓"
}

// lsCols lays out an ls -l row after its cursor cell: mode, size right
// aligned, the date and the path, so the header and every row agree.
func lsCols(mode, size, at, path string) string {
	return " " + ui.Fit(mode, 10) + " " + ui.AlignRight(size, 7) + "  " + ui.Fit(at, 12) + "  " + path
}

// row is row i of the list, w cells wide: a directory's header, or a bit
// as ls -l prints it.
func (s *Setuid) row(i, w int) string {
	r := s.rows[i]
	if r.def == nil {
		set, of := setCount(&s.c.Meta, r.kind)
		return ui.Spread(" "+ui.Strong.Render(r.kind.Dir())+ui.Muted.Render(ui.G.Sep+string(r.kind)),
			ui.Muted.Render(fmt.Sprintf("%d of %d", set, of)), w)
	}
	d, m := r.def, &s.c.Meta
	b, set := setuid.Bit(m, d.ID)
	mode, size, at, path := "-rwxr-xr-x", "-", "", d.Path()
	note := ""
	switch {
	case set:
		mode, at = "-rwsr-xr-x", lsDate(b.At, s.c.Now)
		if b.Backfill {
			at = ui.Muted.Render(at)
		}
		path = ui.Strong.Render(path)
	case !s.visible(d):
		mode, size, at, path = "-?????????", "?", "?", d.Kind.Dir()+"/.???"
		if h := s.hint(d); h != "" {
			note = ui.Voice.Render(ui.Muted.Render("# " + h))
		}
	case setuid.FreshStore(m, d.ID):
		note = ui.Muted.Render("for a fresh store")
	}
	if have, need := d.Progress(m); need > 0 && s.visible(d) && (setuid.ShowsPercent(m) || !percentBits[d.ID]) {
		size = fmt.Sprintf("%d/%d", have, need)
	}
	line := lsCols(mode, size, at, path)
	if !s.visible(d) {
		line = ui.Muted.Render(line)
	}
	if note != "" {
		line += "  " + note
	}
	if i == s.sel {
		return ui.Select.Render(ui.Fit(ui.G.Cursor+ansi.Strip(line)[1:], w))
	}
	return ui.Clip(line, w)
}

// lsDate is when a bit was set as ls -l dates a file: "Oct 14 21:07", or
// "Oct 14  2025" past six months or in the future, in now's location.
func lsDate(t, now time.Time) string {
	t = t.In(now.Location())
	if d := now.Sub(t); d > 182*24*time.Hour || d < -time.Hour {
		return t.Format("Jan _2  2006")
	}
	return t.Format("Jan _2 15:04")
}

// ---- the selected bit ----------------------------------------------------------

// detail is the selected bit's line under the list at 80x24: its name and
// barrier, where it waits, and whether it is set. A hidden secret says it
// is one, and its hint once ? showed it.
func (s *Setuid) detail() string {
	d := s.rows[s.sel].def
	if !s.visible(d) {
		if h := s.hint(d); h != "" {
			return ui.Strong.Render("A secret.") + " Hint: " + ui.Voice.Render(h)
		}
		return ui.Strong.Render("A secret.") + " Its name shows once it is set; ? shows a hint."
	}
	text := ui.Strong.Render(d.Name) + ui.Muted.Render(ui.G.Sep) + lowerFirst(d.Text)
	if act, behind, ok := bossWhere(bossOf[d.ID]); ok {
		if len(behind) == 0 {
			text += " It waits at the end of " + act + "."
		} else {
			text += " It waits behind " + orList(behind) + " in " + act + "."
		}
	}
	return text + " " + s.status(d, true)
}

// status says whether d is set: "Set Oct 14 21:07." (from git log for a
// backfilled bit), "Not set.", or for a speed completion past its bound
// "Not set: for a fresh store." (GR8: never "missed"). sentence starts it
// with a capital and ends it with a stop.
func (s *Setuid) status(d *setuid.Def, sentence bool) string {
	m := &s.c.Meta
	b, set := setuid.Bit(m, d.ID)
	var out string
	switch {
	case set && b.Backfill:
		out = "set " + lsDate(b.At, s.c.Now) + ", from git log"
	case set:
		out = "set " + lsDate(b.At, s.c.Now)
		if b.Boot > 0 {
			out += fmt.Sprintf(" in boot #%d", b.Boot)
		}
	case setuid.FreshStore(m, d.ID):
		out = "not set: for a fresh store"
	default:
		out = "not set"
	}
	if sentence {
		out = upperFirst(out) + "."
	}
	if set {
		return ui.OK.Render(out)
	}
	return ui.Muted.Render(out)
}

// pane is the 140x40 right pane, w cells wide and at most n rows: the
// selected bit, the completion breakdown once the percentage shows, and
// the bits set last, as many as fit.
func (s *Setuid) pane(w, n int) []string {
	d, m := s.rows[s.sel].def, &s.c.Meta
	wrap := func(text string, indent int) []string {
		var out []string
		for _, l := range ui.WrapN(text, max(w-indent-1, 10), 3) {
			out = append(out, strings.Repeat(" ", indent)+l)
		}
		return out
	}
	var out []string
	if s.visible(d) {
		out = append(out, " "+ui.Strong.Render(d.Path()), "", " "+ui.Strong.Render(d.Name))
		out = append(out, wrap(d.Text, 3)...)
		if act, behind, ok := bossWhere(bossOf[d.ID]); ok {
			where := act + " boss" + ui.G.Sep + "behind " + orList(behind)
			if len(behind) == 0 {
				where = "the final" + ui.G.Sep + "at the end of " + act
			}
			out = append(out, wrap(ui.Muted.Render(where), 3)...)
		}
		out = append(out, "", "   "+s.status(d, false))
	} else {
		out = append(out, " "+ui.Strong.Render(d.Kind.Dir()+"/.???"), "", " "+ui.Strong.Render("a secret"))
		out = append(out, wrap("Its name and barrier show once it is set.", 3)...)
		if h := s.hint(d); h != "" {
			out = append(out, "", "   hint: "+ui.Voice.Render(h))
		} else if s.spoil == SpoilersNone {
			out = append(out, "", ui.Muted.Render("   ? shows its hint, for this visit"))
		}
	}
	section := func(title string) string {
		head := ui.G.Rule + " " + title + " "
		return " " + ui.Muted.Render(head+strings.Repeat(ui.G.Rule, max(w-2-ansi.StringWidth(head), 0)))
	}
	if setuid.ShowsPercent(m) {
		pct, parts := setuid.Completion(m)
		out = append(out, "", section(fmt.Sprintf("completion%s%d%%", ui.G.Sep, pct)))
		for _, p := range parts {
			out = append(out, "   "+ui.Fit(p.Name, 14)+partBar(p.Have, p.Of, 18)+"  "+
				ui.Fit(fmt.Sprintf("%d/%d", p.Have, p.Of), 7)+ui.Muted.Render(ui.AlignRight(fmt.Sprintf("%d of %d", p.Points, p.Max), 9)))
		}
	}
	if recent := s.recent(n - len(out) - 2); len(recent) > 0 {
		out = append(out, "", section("recent"))
		out = append(out, recent...)
	}
	return out
}

// partBar is have of of as a bar w cells wide: ▓ for the share done, ░ for
// the rest (# and . under the ASCII glyphs).
func partBar(have, of, w int) string {
	full := 0
	if of > 0 {
		full = min(have*w/of, w)
	}
	return ui.OK.Render(strings.Repeat(ui.G.Fill, full)) + ui.Muted.Render(strings.Repeat(ui.G.Empty, w-full))
}

// recent is the bits set last, newest first, at most n: when, which, and
// the boot that set it (or the history a backfill read it from).
func (s *Setuid) recent(n int) []string {
	bits := s.c.Meta.Setuid
	var out []string
	for i := len(bits) - 1; i >= 0 && len(out) < n; i-- {
		b := bits[i]
		if _, ok := setuid.Lookup(b.ID); !ok {
			continue
		}
		from := "outside a boot"
		switch {
		case b.Backfill:
			from = "from git log"
		case b.Boot > 0:
			from = fmt.Sprintf("boot #%d", b.Boot)
		}
		out = append(out, "   "+ui.Muted.Render(lsDate(b.At, s.c.Now))+"  "+ui.Fit(b.ID, 24)+ui.Muted.Render(from))
	}
	return out
}

// bossOf is the boss each boss bit is about, for the line that says where
// it waits: its kill, its twist mastered, its secrets.
var bossOf = map[string]string{
	"two-twins": "off-by-one", "ditto-ditto": "off-by-one", ".counted-again": "off-by-one",
	"there-goes-bill": "oom-killer", "rss-modest": "oom-killer",
	"shelf-stays-full": "heisenbug", "stopped-looking": "heisenbug",
	"caucus-race": "race-condition", "every-thread": "race-condition",
	"handsome-pig": "undefined-behavior", "bounds-checked": "undefined-behavior",
	"snicker-snack": game.FinalBoss, "decided-this-one": game.FinalBoss,
	".then-it-ends": game.FinalBoss, ".galumphing-back": game.FinalBoss,
}

// bossWhere is where boss key waits on the built-in map: its act and the
// nodes whose exits lead to it, none for the final, which every act III
// node leads to. ok is false for no boss.
func bossWhere(key string) (act string, behind []string, ok bool) {
	if key == "" {
		return "", nil, false
	}
	nodes := rules.MapNodes()
	i := slices.IndexFunc(nodes, func(n rules.Biome) bool { return n.Boss == key && !n.Hidden })
	if i < 0 {
		return "", nil, false
	}
	switch nodes[i].Tier {
	case rules.TierBossI:
		act = "act I"
	case rules.TierBossII:
		act = "act II"
	default:
		return "act III", nil, true
	}
	for _, n := range nodes {
		if !n.Hidden && slices.Contains(n.Next, key) {
			behind = append(behind, n.Name)
		}
	}
	return act, behind, true
}

// orList is names as prose: "A", "A or B", "A, B or C".
func orList(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// percentBits are the counters of the percentage itself, which stays
// hidden, their SIZE too, until setuid.ShowsPercent.
var percentBits = map[string]bool{"hundred": true, "hundred-twelve": true}
