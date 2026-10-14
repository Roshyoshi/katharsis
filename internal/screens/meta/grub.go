package meta

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
)

// GrubConfig is what the start screen shows.
type GrubConfig struct {
	Meta game.Meta
	// Presets are custom rules preset names; "standard" is always offered.
	Presets []string
	// Title resolves problem titles for the core dump and regressions; nil
	// shows catalog IDs.
	Title func(catalog.ID) string
	// Now decides which regressions are due; zero means time.Now().
	Now time.Time

	// PrestigeName and Banner default to the flavor package. Tests pin them
	// so goldens do not churn whenever flavor text changes.
	PrestigeName func(int) string
	Banner       func(kind string) string
}

// Menu is a GRUB 2 style text menu: a boxed list with one highlighted
// entry, submenus, and an `e` editor for boot entries. NewGrub builds the
// start screen; NewUpsolve builds the upsolve picker in the same chrome.
type Menu struct {
	stack   []*page // stack[0] is the root; submenus push
	edit    *editor
	escBack bool // esc at the root emits screens.Back (the picker)
	presets []string
	maxP    int
	nameOf  func(int) string
	banner  string
	status  string
	lastH   int // box rows at the last View, for page up/down
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
}

const standardPreset = "standard"

// NewGrub builds the start screen. The highest unlocked prestige is listed
// first and highlighted, as GRUB lists the newest kernel first.
func NewGrub(c GrubConfig) *Menu {
	if c.PrestigeName == nil {
		c.PrestigeName = flavor.PrestigeName
	}
	if c.Banner == nil {
		c.Banner = flavor.Banner
	}
	now := c.Now
	if now.IsZero() {
		now = time.Now()
	}
	m := &Menu{
		maxP:   min(max(c.Meta.Prestige, 0), game.MaxPrestige),
		nameOf: c.PrestigeName,
		banner: strings.TrimRight(c.Banner("grub"), "\n"),
	}
	m.presets = []string{standardPreset}
	for _, p := range c.Presets {
		if p != "" && p != standardPreset {
			m.presets = append(m.presets, p)
		}
	}
	m.status = fmt.Sprintf("bank %s · rating %.0f · next boot #%d",
		Bytes(c.Meta.Bank), c.Meta.Rating, c.Meta.Boots+1)

	var es []entry
	for p := m.maxP; p >= 0; p-- {
		es = append(es, m.bootEntry(p, standardPreset))
	}
	es = append(es, entry{label: "katharsis (RTFM — learn mode)", msg: screens.OpenLearn{}})
	if len(m.presets) > 1 {
		sub := &page{}
		for _, name := range m.presets[1:] {
			sub.entries = append(sub.entries, m.bootEntry(m.maxP, name))
		}
		es = append(es, entry{label: "katharsis (custom preset…)", sub: sub})
	}
	ups := DueUpsolves(c.Meta, c.Title, now)
	regs := 0
	for _, u := range ups {
		if u.CoreDump {
			es = append(es, entry{
				label: fmt.Sprintf("upsolve core dump: %s (%s)", u.Title, Bytes(u.Bytes)),
				msg:   screens.Upsolve{Problem: string(u.Problem)},
			})
		} else {
			regs++
		}
	}
	if regs > 0 {
		es = append(es, entry{
			label: fmt.Sprintf("regressions due: %d", regs),
			sub:   upsolvePage(ups, now),
		})
	}
	es = append(es,
		entry{label: "git log", msg: screens.OpenLog{}},
		entry{label: "top", msg: screens.OpenTop{}},
		entry{label: "poweroff", msg: screens.Quit{}},
	)
	m.stack = []*page{{entries: es}}
	return m
}

// NewUpsolve builds the upsolve picker: the core dump and due regressions
// (see DueUpsolves). Enter emits screens.Upsolve; esc emits screens.Back.
func NewUpsolve(items []UpsolveItem, now time.Time) *Menu {
	if now.IsZero() {
		now = time.Now()
	}
	return &Menu{stack: []*page{upsolvePage(items, now)}, escBack: true}
}

func upsolvePage(items []UpsolveItem, now time.Time) *page {
	p := &page{pick: true}
	for _, u := range items {
		label := fmt.Sprintf("regression: %s (due %s, stage %d)", u.Title, ago(u.Due, now), u.Stage+1)
		if u.CoreDump {
			label = fmt.Sprintf("core dump: %s (%s, boot #%d)", u.Title, Bytes(u.Bytes), u.Boot)
		}
		p.entries = append(p.entries, entry{label: label, msg: screens.Upsolve{Problem: string(u.Problem)}})
	}
	if len(p.entries) == 0 {
		p.entries = []entry{{label: "nothing to upsolve"}}
	}
	return p
}

func (m *Menu) bootEntry(prestige int, preset string) entry {
	return entry{
		label: m.bootLabel(prestige, preset),
		boot:  &screens.StartBoot{Prestige: prestige, Preset: preset},
	}
}

func (m *Menu) bootLabel(prestige int, preset string) string {
	s := fmt.Sprintf("katharsis, prestige %d", prestige)
	if m.nameOf != nil {
		if n := m.nameOf(prestige); n != "" {
			s += " (" + n + ")"
		}
	}
	if preset != standardPreset {
		s += ", preset " + preset
	}
	return s
}

func (m *Menu) cur() *page { return m.stack[len(m.stack)-1] }

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
		case e.boot != nil:
			return m, screens.Emit(*e.boot)
		case e.msg != nil:
			return m, screens.Emit(e.msg)
		}
	case "e":
		if e := p.entries[p.sel]; e.boot != nil {
			m.edit = m.newEditor(e)
		}
	case "esc", "q", "left", "backspace":
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		} else if m.escBack {
			return m, screens.Emit(screens.Back{})
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
	val := func(field int, s string) string {
		if ed.field == field {
			return reverse.Render(s) + "█"
		}
		return s
	}
	note := func(field int, s string) string {
		if ed.field == field {
			return "   " + faint.Render(s)
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
		"Press enter to boot the selected entry, `e' to edit the commands " +
		"before booting."
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
	if m.status != "" {
		foot = append(foot, "", center(faint.Render(m.status), w))
	}

	var head []string
	head = append(head, center(grubVersion, w), "")

	// The box takes what is left. Drop the help, then the header, when the
	// terminal is too short to hold them and a few entries.
	rows := len(m.cur().entries)
	if m.edit != nil {
		rows = 6
	}
	box := h - len(head) - 2 - 1 - len(foot)
	if box < min(rows, 3) {
		foot = nil
		box = h - len(head) - 2
	}
	if box < min(rows, 3) {
		head = nil
		box = h - 2
	}
	box = max(box, 1)
	if head != nil && m.banner != "" {
		// The banner is art: centre it as a block so its lines stay
		// aligned, and let it replace the version line when it carries
		// one (flavor's does).
		art := strings.Split(m.banner, "\n")
		artW := 0
		for _, l := range art {
			artW = max(artW, ansi.StringWidth(l))
		}
		var b []string
		for _, l := range art {
			b = append(b, strings.Repeat(" ", max((w-artW)/2, 0))+l)
		}
		b = append(b, "")
		if !strings.Contains(m.banner, "GRUB") {
			b = append(b, head...)
		}
		if extra := len(b) - len(head); box-extra >= rows && artW <= w {
			head, box = b, box-extra
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
		out = append(out, " │"+pad(l, inner)+"│")
	}
	out = append(out, " └"+strings.Repeat("─", max(inner-1, 0))+down+"┘")
	if foot != nil {
		out = append(out, "")
		out = append(out, foot...)
	}
	return frame(out, w, h)
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
			out = append(out, reverse.Render(pad("*"+e.label, inner)))
			continue
		}
		if e.msg == nil && e.boot == nil && e.sub == nil {
			out = append(out, faint.Render(" "+e.label))
			continue
		}
		out = append(out, " "+e.label)
	}
	return out
}
