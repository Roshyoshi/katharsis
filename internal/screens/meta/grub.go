package meta

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/story"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// GrubConfig is what the start screen shows.
type GrubConfig struct {
	Meta game.Meta
	// Presets are the preset names past "standard", which is always
	// offered, in the order listed: the built-ins short and patch first.
	Presets []string
	// Lookup resolves problem titles for the core dump and regressions; nil
	// shows catalog IDs.
	Lookup screens.Lookup
	// Now decides which regressions are due and, by its date in its own
	// location, the nightly build; zero means time.Now().
	Now time.Time
	// Gate is what unlocking the next prestige still needs (D31), from
	// game.NextPrestige; its zero value shows nothing.
	Gate game.PrestigeGate
	// Packages are apt's, in shelf order; GRUB lists apt with the next
	// one to buy (Goal) when there are any.
	Packages []Package
	// Load is the load average on the status line (retention.md RT9), the
	// share of practice days over 7, 30 and 90 days; nil shows none.
	Load *[3]float64
	// Sources, when set, is a note on content sources not serving yet
	// ("Codeforces indexing… 40%"), added to the status line at every draw.
	Sources func() string
	// Nightly is how today's nightly has been played (retention RT15),
	// shown on its entry; nil for not yet.
	Nightly *NightlyBest
	// Today is the play so far today and Budget the session budget
	// (RT12, 0 for none): past it the status line says so.
	Today, Budget time.Duration
	// Away is the days since the player last played when that makes the
	// next boot a comeback (game.Comeback, retention RT8), 0 otherwise:
	// the status line says "first boot in N days", as the boot will.
	Away int

	// PrestigeName and Banner default to the flavor package. Tests pin them
	// so goldens do not churn whenever flavor text changes.
	PrestigeName func(int) string
	Banner       func(kind string) string
	// Story is the story's text; it defaults to flavor.Story.
	Story func(hook string, seed uint64) string
}

// Menu is a GRUB 2 style text menu: a boxed list with one highlighted
// entry, submenus, and an `e` editor for boot entries.
type Menu struct {
	stack   []*page // stack[0] is the root; submenus push
	edit    *editor
	presets []string
	maxP    int
	nameOf  func(int) string
	banner  string
	status  string
	gate    string // the prestige gate line under the status, "" for none
	motd    string // boot.motd, on the status line when it fits
	// firstRegression is set until the regressions submenu has opened
	// with story.first-regression unseen: the first open plays it.
	firstRegression bool
	lastH           int // box rows at the last View, for page up/down
	sources         func() string
}

type page struct {
	entries  []entry
	pick     bool // entries upsolve rather than boot, for the help text
	sel, top int
}

type entry struct {
	label string
	msg   tea.Msg            // emitted on enter
	boot  *screens.StartBoot // boot entries: also editable with e
	sub   *page              // submenus
	note  string             // the subtitle shown under the box while selected
}

const standardPreset = "standard"

// NewGrub builds the start screen. The highest unlocked prestige is listed
// first and highlighted, as GRUB lists the newest kernel first. Under the
// prestiges is the nightly build, the daily run: the highest prestige,
// standard rules and the day's NightlySeed.
func NewGrub(c GrubConfig) *Menu {
	if c.PrestigeName == nil {
		c.PrestigeName = flavor.PrestigeName
	}
	if c.Banner == nil {
		c.Banner = flavor.Banner
	}
	if c.Story == nil {
		c.Story = flavor.Story
	}
	now := c.Now
	if now.IsZero() {
		now = time.Now()
	}
	title := func(id catalog.ID) string {
		if c.Lookup != nil {
			if p, ok := c.Lookup(id); ok && p.Title != "" {
				return p.Title
			}
		}
		return string(id)
	}
	m := &Menu{
		maxP:   min(max(c.Meta.Prestige, 0), game.MaxPrestige),
		nameOf: c.PrestigeName,
		banner: strings.TrimRight(c.Banner("grub"), "\n"),
	}
	if c.Meta.Theme != "" {
		if art := c.Banner("grub:" + c.Meta.Theme); art != "" {
			m.banner = strings.TrimRight(art, "\n")
		}
	}
	m.sources = c.Sources
	m.presets = []string{standardPreset}
	for _, p := range c.Presets {
		if p != "" && p != standardPreset {
			m.presets = append(m.presets, p)
		}
	}
	m.status = fmt.Sprintf("bank %s"+ui.G.Sep+"rating %.0f"+ui.G.Sep+"next boot #%d",
		units.Bytes(c.Meta.Bank), c.Meta.Rating, c.Meta.Boots+1)
	// The days away go next to the boot they make a comeback, before what
	// a narrow line clips. A fact, never a guilt line (GR8).
	if c.Away > 0 {
		m.status += ui.G.Sep + "first boot in " + units.Plural(c.Away, "day")
	}
	if c.Load != nil {
		m.status += ui.G.Sep + LoadText(*c.Load)
	}
	// Past the session budget the status line says so, and nothing more
	// (retention RT12: a nudge, never a lock).
	if c.Budget > 0 && c.Today >= c.Budget {
		m.status += fmt.Sprintf(ui.G.Sep+"today %s of %s", units.HM(c.Today), units.HM(c.Budget))
	}
	m.motd = c.Story("boot.motd", uint64(max(c.Meta.Boots, 0)))
	m.gate = screens.GateText(c.Gate, m.maxP, func(p int) string {
		return cmp.Or(m.nameOf(p), fmt.Sprintf("prestige %d", p))
	})
	sub := func(hook string, pairs ...string) string {
		return flavor.Fill(c.Story("grub.subtitle."+hook, 0), pairs...)
	}

	// The default boot, then the core dump right under it (RT4): the
	// cliffhanger is the second thing GRUB offers. Once a level past the
	// divisions is unlocked, the divisions fold into a submenu, as GRUB
	// keeps older kernels under one entry: nine levels and the rest would
	// not fit beside the art at 80x24.
	var es []entry
	var divisions *page
	for p := m.maxP; p >= 0; p-- {
		e := m.bootEntry(p, standardPreset)
		e.note = prestigeNote(p, sub)
		if m.maxP > foldAbove && p <= foldAbove {
			if divisions == nil {
				divisions = &page{}
				es = append(es, entry{label: fmt.Sprintf("katharsis (prestige %d to 0…)", foldAbove), sub: divisions})
			}
			divisions.entries = append(divisions.entries, e)
			continue
		}
		es = append(es, e)
		if d := c.Meta.CoreDump; d != nil && p == m.maxP {
			es = append(es, entry{
				label: fmt.Sprintf("upsolve core dump: %s (%s)", title(d.Problem), units.Bytes(d.Bytes)),
				msg:   screens.Upsolve{Problem: d.Problem},
				note:  sub("coredump", "bytes", units.Bytes(d.Bytes), "title", ui.Clip(title(d.Problem), 24)),
			})
		}
	}
	es = append(es, entry{
		label: "katharsis (nightly build " + now.Format(time.DateOnly) + ")" + nightlyBest(c.Nightly),
		boot:  &screens.StartBoot{Prestige: m.maxP, Preset: standardPreset, Seed: NightlySeed(now)},
		note:  sub("nightly"),
	})
	es = append(es, entry{label: "katharsis (RTFM — learn mode)", msg: screens.OpenLearn{}, note: sub("rtfm")})
	if len(m.presets) > 1 {
		sub := &page{}
		for _, name := range m.presets[1:] {
			sub.entries = append(sub.entries, m.bootEntry(m.maxP, name))
		}
		es = append(es, entry{label: "katharsis (other presets…)", sub: sub, note: c.Story("grub.subtitle.custom", 0)})
	}
	if regs := dueRegressions(c.Meta.Regressions, now); len(regs) > 0 {
		// At most three on screen, and no count past them (retention
		// GR10): the rest wait, and none of it is a debt.
		regs = regs[:min(len(regs), maxDue)]
		page := &page{pick: true}
		for _, r := range regs {
			page.entries = append(page.entries, entry{
				label: fmt.Sprintf("regression: %s (due %s, stage %d)", title(r.Problem), ago(r.Due, now), r.Stage+1),
				msg:   screens.Upsolve{Problem: r.Problem},
				note:  regressionNote(r, c.Story),
			})
		}
		label := fmt.Sprintf("regressions due: %d", len(regs))
		if len(regs) == maxDue && len(dueRegressions(c.Meta.Regressions, now)) > maxDue {
			label = fmt.Sprintf("regressions due: %d+", maxDue)
		}
		es = append(es, entry{label: label, sub: page, note: sub("regressions")})
		// The White Queen's beat, the first time the player opens it.
		m.firstRegression = !game.HasSeen(&c.Meta, firstRegression)
	}
	if len(c.Packages) > 0 {
		es = append(es, entry{label: aptLabel(c.Packages, c.Meta.Bank), msg: screens.OpenShop{}})
	}
	es = append(es,
		entry{label: "git log", msg: screens.OpenLog{}, note: sub("gitlog")},
		entry{label: "top", msg: screens.OpenTop{}, note: sub("top")},
		entry{label: "poweroff", msg: screens.Quit{}, note: sub("poweroff")},
	)
	m.stack = []*page{{entries: es}}
	return m
}

// maxDue is how many due regressions GRUB lists (retention GR10).
const maxDue = 3

// foldAbove is the last division (P4, Advanced). The levels past it are
// rings −2 and −3; once one is unlocked, the divisions fold into one
// submenu.
const foldAbove = 4

// prestigeNote is a prestige entry's subtitle (grub.subtitle.prestige.N):
// the ring it boots into.
func prestigeNote(p int, sub func(string, ...string) string) string {
	if s := sub(fmt.Sprintf("prestige.%d", p)); s != "" {
		return s
	}
	ring, _ := flavor.Ring(p)
	return sub("prestige.beyond", "ring", strconv.Itoa(ring))
}

// NightlyBest is how today's nightly has been played (retention RT15): a
// personal best, with nothing to keep up. Played 0 is not yet.
type NightlyBest struct {
	Played  int // boots on the seed
	Cleared bool
	Uptime  time.Duration // the fastest clear, else the longest crash
	Doors   int           // doors cleared on that boot
}

// nightlyBest is the nightly entry's personal best, "" before a play.
func nightlyBest(b *NightlyBest) string {
	switch {
	case b == nil || b.Played == 0:
		return ""
	case b.Cleared:
		return ui.G.Sep + "best: cleared in " + units.Clock(b.Uptime)
	}
	return ui.G.Sep + "best: " + units.Plural(b.Doors, "door") + " cleared"
}

// NightlySeed is the nightly build's seed for the day t falls on in t's own
// location (hand it local time): the date's decimal digits read as hex, so
// 2026-10-14 is seed 0x20261014. Each day has one seed, git log shows it as
// the date, and it is never 0 (which StartBoot takes as random). The engine
// mixes its seed through splitmix64, so neighbouring days still roll
// unrelated boots. The seed fixes the engine's draws, but doors are still
// aimed at the player's rating and history, so a replay can meet other
// doors.
func NightlySeed(t time.Time) uint64 {
	y, mo, d := t.Date()
	seed, _ := strconv.ParseUint(fmt.Sprintf("%04d%02d%02d", y, int(mo), d), 16, 64)
	return seed
}

// dueRegressions is the regressions due at or before now, oldest due first.
func dueRegressions(rs []game.Regression, now time.Time) []game.Regression {
	var due []game.Regression
	for _, r := range rs {
		if !r.Due.After(now) {
			due = append(due, r)
		}
	}
	slices.SortStableFunc(due, func(a, b game.Regression) int { return a.Due.Compare(b.Due) })
	return due
}

func (m *Menu) bootEntry(prestige int, preset string) entry {
	label := "katharsis, " + prestigeText(prestige, m.nameOf)
	if preset != standardPreset {
		label += ", preset " + preset
	}
	return entry{label: label, boot: &screens.StartBoot{Prestige: prestige, Preset: preset}}
}

func (m *Menu) cur() *page { return m.stack[len(m.stack)-1] }

// Follow puts the cursor where prev had it, for an app that rebuilds the
// menu under a screen (an apt install changes the bank and apt's own
// line): on the same entry when the new menu still has it, matched by what
// it does, not its label, since apt's names the next package to buy.
func (m *Menu) Follow(prev *Menu) {
	if prev == nil || len(prev.stack) == 0 {
		return
	}
	old, root := prev.stack[0], m.stack[0]
	if old.sel < 0 || old.sel >= len(old.entries) || len(root.entries) == 0 {
		return
	}
	root.sel, root.top = min(old.sel, len(root.entries)-1), old.top
	want := old.entries[old.sel].key()
	for i, e := range root.entries {
		if e.key() == want {
			root.sel = i
			return
		}
	}
}

// key names what an entry does: the boot it starts, the screen it opens,
// or, for a submenu, its label without the count it ends with.
func (e entry) key() string {
	switch {
	case e.boot != nil:
		return fmt.Sprintf("boot %+v", *e.boot)
	case e.sub != nil:
		return "menu " + strings.TrimRight(e.label, "0123456789 ")
	}
	return fmt.Sprintf("%T %+v", e.msg, e.msg)
}

// Editing reports whether the `e` editor is open. The editor wants tab and
// hex digits for itself, so an app with global keys should pass them through
// while it is.
func (m *Menu) Editing() bool { return m.edit != nil }

func (m *Menu) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.edit != nil {
		return m, m.editKey(k)
	}
	p := m.cur()
	switch k.String() {
	case "up", "k", "ctrl+p":
		p.sel = max(p.sel-1, 0)
	case "down", "j", "ctrl+n":
		p.sel = min(p.sel+1, len(p.entries)-1)
	case "home", "g":
		p.sel = 0
	case "end", "G":
		p.sel = len(p.entries) - 1
	case "pgup":
		p.sel = max(p.sel-max(m.lastH-1, 1), 0)
	case "pgdown":
		p.sel = min(p.sel+max(m.lastH-1, 1), len(p.entries)-1)
	case "enter", "ctrl+x", "f10":
		e := p.entries[p.sel]
		switch {
		case e.sub != nil:
			e.sub.sel, e.sub.top = 0, 0
			m.stack = append(m.stack, e.sub)
			if e.sub.pick && m.firstRegression {
				m.firstRegression = false
				return m, screens.Emit(story.Read{ID: firstRegression})
			}
		case e.boot != nil:
			return m, screens.Emit(*e.boot)
		case e.msg != nil:
			return m, screens.Emit(e.msg)
		}
	case "e":
		if e := p.entries[p.sel]; e.boot != nil {
			m.edit = m.newEditor(e)
		}
	case "c":
		// Where real GRUB opens its command line, the credits: a key, not an
		// entry, so the menu keeps its row for the banner at 80x24.
		return m, screens.Emit(screens.OpenCredits{})
	case "esc", "q", "left", "backspace":
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
	}
	return m, nil
}

// ---- the `e` editor ------------------------------------------------------------

// editor is GRUB's entry editor cut down to three settings. Each is a line
// of a pseudo grub.cfg so the screen reads like the real thing.
type editor struct {
	title    string
	prestige int
	preset   int    // index into Menu.presets
	seed     string // hex digits; "" and "0" both mean random
	field    int    // 0 prestige, 1 preset, 2 seed
}

const maxSeedDigits = 16

func (m *Menu) newEditor(e entry) *editor {
	ed := &editor{title: e.label, prestige: e.boot.Prestige}
	for i, p := range m.presets {
		if p == e.boot.Preset {
			ed.preset = i
		}
	}
	if e.boot.Seed != 0 {
		ed.seed = strconv.FormatUint(e.boot.Seed, 16)
	}
	return ed
}

func (m *Menu) editKey(k tea.KeyPressMsg) tea.Cmd {
	ed := m.edit
	switch s := k.String(); s {
	case "esc":
		m.edit = nil
	case "ctrl+x", "f10", "enter":
		seed, _ := strconv.ParseUint(ed.seedText(), 16, 64)
		m.edit = nil
		return screens.Emit(screens.StartBoot{
			Prestige: ed.prestige, Preset: m.presets[ed.preset], Seed: seed,
		})
	case "up", "ctrl+p", "shift+tab":
		ed.field = max(ed.field-1, 0)
	case "down", "ctrl+n":
		ed.field = min(ed.field+1, 2)
	case "tab", "right", "left":
		d := 1
		if s == "left" {
			d = -1
		}
		switch ed.field {
		case 0:
			ed.prestige = (ed.prestige + d + m.maxP + 1) % (m.maxP + 1)
		case 1:
			ed.preset = (ed.preset + d + len(m.presets)) % len(m.presets)
		case 2:
			if s == "tab" {
				ed.field = 0 // nothing to complete in a seed; wrap round
			}
		}
	case "backspace", "ctrl+h":
		if ed.field == 2 && ed.seed != "" {
			ed.seed = ed.seed[:len(ed.seed)-1]
		}
	case "ctrl+u", "ctrl+k":
		if ed.field == 2 {
			ed.seed = ""
		}
	default:
		if ed.field == 2 && len(k.Text) == 1 && strings.Contains("0123456789abcdefABCDEF", k.Text) {
			if ed.seed == "0" {
				ed.seed = ""
			}
			if len(ed.seed) < maxSeedDigits {
				ed.seed += strings.ToLower(k.Text)
			}
		}
	}
	return nil
}

func (ed *editor) seedText() string {
	if ed.seed == "" {
		return "0"
	}
	return ed.seed
}

func (m *Menu) editLines() []string {
	ed := m.edit
	// One cursor: the cell after the value under edit, in reverse, the way
	// a terminal draws its own.
	val := func(field int, s string) string {
		if ed.field == field {
			return s + ui.Select.Render(" ")
		}
		return s
	}
	note := func(field int, s string) string {
		if ed.field == field {
			return "   " + ui.Muted.Render(s)
		}
		return ""
	}
	return []string{
		fmt.Sprintf("setparams '%s'", ed.title),
		"",
		"        set prestige=" + val(0, strconv.Itoa(ed.prestige)) +
			note(0, fmt.Sprintf("# 0–%d unlocked", m.maxP)),
		"        set preset=" + val(1, m.presets[ed.preset]) +
			note(1, "# TAB: "+strings.Join(m.presets, ", ")),
		"        set seed=" + val(2, ed.seedText()) + note(2, "# hex; 0 is random"),
		"        boot",
	}
}

// ---- view ------------------------------------------------------------------------

const grubVersion = "GNU GRUB  version 2.12"

var (
	helpMenu = "Use the ↑ and ↓ keys to select which entry is highlighted. " +
		"Press enter to boot the selected entry, `e' to edit the commands, " +
		"`c' for credits."
	helpSub  = helpMenu + " ESC to return previous menu."
	helpPick = "Use the ↑ and ↓ keys to select which entry is highlighted. " +
		"Press enter to upsolve the selected problem, or ESC to return " +
		"previous menu."
	helpEdit = "Minimum Emacs-like screen editing is supported. ↑ and ↓ pick " +
		"a line, TAB lists completions, and the seed takes hex digits. Press " +
		"Ctrl-x or F10 to boot, or ESC to discard edits and return to the " +
		"GRUB menu."
)

func (m *Menu) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	help := helpMenu
	switch {
	case m.edit != nil:
		help = helpEdit
	case m.cur().pick:
		help = helpPick
	case len(m.stack) > 1:
		help = helpSub
	}
	helpW := min(w-4, 70)
	foot := strings.Split(ansi.Wordwrap(help, max(helpW, 10), ""), "\n")
	for i := range foot {
		foot[i] = strings.Repeat(" ", max((w-helpW)/2, 1)) + foot[i]
	}
	status := m.status
	if m.sources != nil {
		if note := m.sources(); note != "" {
			status += ui.G.Sep + note
		}
	}
	// The motd rides along where the line has room for it (boot.motd).
	if m.motd != "" && ansi.StringWidth(status+ui.G.Sep+m.motd) <= w-2 {
		status += ui.G.Sep + m.motd
	}
	gap := len(foot) // the blank row above the status line
	foot = append(foot, "", ui.Center(ui.Muted.Render(status), w))
	if m.gate != "" {
		foot = append(foot, ui.Center(ui.Muted.Render(m.gate), w))
	}
	head := []string{ui.Center(grubVersion, w), ""}

	// The box takes what is left: the screen less its borders and, with
	// the foot, the subtitle row. Drop the help, then the header, when the
	// terminal is too short to hold them and a few entries.
	rows := len(m.cur().entries)
	if m.edit != nil {
		rows = 6
	}
	boxRows := func(head, foot []string) int {
		if foot == nil {
			return h - len(head) - 2
		}
		return h - len(head) - 2 - 1 - len(foot)
	}
	box := boxRows(head, foot)
	if box < min(rows, 3) {
		foot = nil
		box = boxRows(head, foot)
	}
	if box < min(rows, 3) {
		head = nil
		box = boxRows(head, foot)
	}
	box = max(box, 1)
	// The art replaces the header when every entry still fits beside it.
	// Short of rows, it gives up its blank row, then the one above the
	// status line, before the plain header comes back.
	if art := m.art(w); head != nil && foot != nil && art != nil {
		tight := slices.Delete(slices.Clone(foot), gap, gap+1)
		for _, try := range [][2][]string{{append(art, ""), foot}, {art, foot}, {art, tight}} {
			if n := boxRows(try[0], try[1]); n >= rows {
				head, foot, box = try[0], try[1], n
				break
			}
		}
	}
	m.lastH = box

	inner := max(w-4, 1) // " │" + inner + "│ "
	var lines []string
	if m.edit != nil {
		lines = m.editLines()
	} else {
		lines = m.menuLines(inner, box)
	}

	out := append([]string{}, head...)
	p := m.cur()
	up, down := "─", "─"
	if m.edit == nil && p.top > 0 {
		up = "↑"
	}
	if m.edit == nil && p.top+box < len(p.entries) {
		down = "↓"
	}
	out = append(out, " ┌"+strings.Repeat("─", max(inner-1, 0))+up+"┐")
	for i := range box {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out = append(out, " │"+ui.Fit(l, inner)+"│")
	}
	out = append(out, " └"+strings.Repeat("─", max(inner-1, 0))+down+"┘")
	if foot != nil {
		// The selected entry's subtitle (grub.subtitle.*) takes the row
		// between the box and the help, so it costs the menu nothing.
		out = append(out, m.subtitle(w))
		out = append(out, foot...)
	}
	return ui.Layout(w, h, out, "")
}

// art is the banner as the header: centred as one block, so its lines stay
// aligned, with GRUB's version line under it unless it carries its own
// (flavor's do). nil when there is no banner or it is wider than w.
func (m *Menu) art(w int) []string {
	if m.banner == "" {
		return nil
	}
	lines := strings.Split(m.banner, "\n")
	artW := 0
	for _, l := range lines {
		artW = max(artW, ansi.StringWidth(l))
	}
	if artW > w {
		return nil
	}
	for i, l := range lines {
		lines[i] = strings.Repeat(" ", (w-artW)/2) + l
	}
	if !strings.Contains(m.banner, "GRUB") {
		lines = append(lines, ui.Center(grubVersion, w))
	}
	return lines
}

// menuLines renders the visible slice of the current page, keeping the
// selection on screen.
func (m *Menu) menuLines(inner, rows int) []string {
	p := m.cur()
	if p.sel < p.top {
		p.top = p.sel
	}
	if p.sel >= p.top+rows {
		p.top = p.sel - rows + 1
	}
	p.top = max(min(p.top, len(p.entries)-rows), 0)
	var out []string
	for i := p.top; i < len(p.entries) && i < p.top+rows; i++ {
		e := p.entries[i]
		if i == p.sel {
			out = append(out, ui.Select.Render(ui.Fit("*"+e.label, inner)))
			continue
		}
		out = append(out, " "+e.label)
	}
	return out
}

// aptLabel is GRUB's apt entry, with the next package to buy and what it
// still costs: "apt install procps (400 B, 88 B to go)".
func aptLabel(ps []Package, bank int64) string {
	p, short, ok := Goal(ps, bank)
	switch {
	case !ok:
		return "apt (every package installed)"
	case short > 0:
		return fmt.Sprintf("apt install %s (%s, %s to go)", p.Name, units.Bytes(p.Price), units.Bytes(short))
	}
	return fmt.Sprintf("apt install %s (%s, affordable)", p.Name, units.Bytes(p.Price))
}

// subtitle is the selected entry's story line, centered in the flavor
// voice, "" when it has none or the editor is open.
func (m *Menu) subtitle(w int) string {
	p := m.cur()
	if m.edit != nil || p.sel < 0 || p.sel >= len(p.entries) || p.entries[p.sel].note == "" {
		return ""
	}
	return ui.Center(ui.Voice.Render(ui.Muted.Render(ui.Clip(p.entries[p.sel].note, max(w-4, 1)))), w)
}

// firstRegression is the White Queen's beat: the first regression back
// (docs/story/text.md).
const firstRegression = "story.first-regression"

// regressionNote is a due regression's subtitle: the line for its first or
// second stage (regression.stage.1, .2), with {days} that stage's delay
// from the regression's own schedule, read as game.advanceRegression reads
// it. A preset with more stages has no line for the rest, so they get the
// submenu's own.
func regressionNote(r game.Regression, story func(string, uint64) string) string {
	days := r.Days
	if len(days) == 0 {
		days = rules.Standard().RegressionDays // scheduled before Days existed
	}
	if r.Stage < 2 && r.Stage < len(days) {
		if s := story(fmt.Sprintf("regression.stage.%d", r.Stage+1), 0); s != "" {
			return flavor.Fill(s, "days", strconv.Itoa(days[r.Stage]))
		}
	}
	return story("grub.subtitle.regressions", 0)
}
