package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Glyphs is the set of symbols screens draw with. Each glyph has one
// meaning (ui-system.md §2.4): ▸ is only the cursor, ↺ only a regression,
// ◆ only a difficulty. A non-ASCII glyph is followed by a space, a digit or
// the end of its token, never by another glyph or a border.
//
// Prefix glyphs (Timer, Stopwatch, Duck, Rating, Times, Approx) are
// written straight before their digits and carry any space they need:
// "⏱ 23:41", "🦆1", "◆1300"; in ASCII "T-23:41", "duck 1", "*1300".
type Glyphs struct {
	Timer      string // countdown prefix: "⏱ 23:41"; never sent to vim, which draws ⏱ two cells wide
	Stopwatch  string // count-up prefix, a practice clock: "⏱ 12:34", "T+12:34"; Timer's ⏱, as for vim
	Life       string // a life left
	LifeLost   string // a life lost
	Duck       string // duck count prefix: "🦆1"; the one true emoji, two cells everywhere
	Pass       string // pass, AC, cleared
	Fail       string // fail
	Crash      string // crash, lethal
	Boss       string // boss door
	Mystery    string // mystery door
	Blind      string // blind door
	Cursed     string // cursed door: "this kills you"
	Regression string // a past crash, due again
	Retreat    string // retreat (NetHack's up-stairs)
	Here       string // you are here
	Ahead      string // a place ahead
	Rating     string // difficulty prefix: "◆1300" (D27)
	AtMost     string // a ceiling, after Rating: "◆≤800", a problem rated at its source's floor
	Cap        string // capability held
	CapLocked  string // capability not yet held
	Cursor     string // the cursor
	FoldOpen   string // an open fold
	FoldShut   string // a shut fold
	Crumb      string // between breadcrumb parts, spaces included: " › "
	Sep        string // between parts of a line, spaces included: " · "
	Fill       string // a bar's filled cell
	Empty      string // a bar's empty cell
	Star       string // achievement; a frequency level reached
	StarOff    string // a frequency level not reached
	Ellipsis   string // marks a cut
	Mask       string // one character of a password, masked
	Times      string // multiplier prefix: "×1.5"
	Approx     string // estimate prefix: "≈74%"
	Range      string // between a range's ends: "850–1050"
	Arrow      string // leads to
	Rule       string // a horizontal rule's cell
	// The RTFM map's: a reading-only page, a shortcut to a later world (Super
	// Mario World's Star Road), and an optional path's cells, across and
	// down. Its paths are Rule's cells and Thin's sides and corners.
	Page, Warp, Dotted, DottedV string

	// Heat is a heat map's cell by level: none, then least to most.
	Heat [5]string

	// Frames: Thin for idle cards and regions, Thick for the selected
	// card, Double for the boss. Selection changes the glyphs as well as
	// the color, so it reads on a monochrome terminal.
	Thin, Thick, Double lipgloss.Border
}

// Unicode is the default glyph set.
var Unicode = Glyphs{
	Timer: "⏱ ", Stopwatch: "⏱ ", Life: "♥", LifeLost: "♡", Duck: "🦆",
	Pass: "✓", Fail: "✗", Crash: "☠",
	Boss: "⚔", Mystery: "?", Blind: "◌", Cursed: "☠", Regression: "↺", Retreat: "↩",
	Here: "●", Ahead: "○", Rating: "◆", AtMost: "≤", Cap: "■", CapLocked: "□",
	Cursor: "▸", FoldOpen: "▾", FoldShut: "▹", Crumb: " › ", Sep: " · ",
	Fill: "▓", Empty: "░", Star: "★", StarOff: "☆", Ellipsis: "…", Mask: "•",
	Times: "×", Approx: "≈", Range: "–", Arrow: "→", Rule: "─",
	Page: "▪", Warp: "↑", Dotted: "┄", DottedV: "┆",
	Heat: [5]string{"·", "░", "▒", "▓", "█"},
	Thin: lipgloss.RoundedBorder(), Thick: lipgloss.ThickBorder(), Double: lipgloss.DoubleBorder(),
}

// ASCII is the fallback for terminals without the Unicode glyphs. Door
// modifiers fall back to the kernel taint letters the crash screen prints
// (R M B C X), so a letter means the same modifier on every screen.
var ASCII = Glyphs{
	Timer: "T-", Stopwatch: "T+", Life: "#", LifeLost: "-", Duck: "duck ",
	Pass: "+", Fail: "x", Crash: "!",
	Boss: "X", Mystery: "M", Blind: "B", Cursed: "C", Regression: "R", Retreat: "<",
	Here: "@", Ahead: ".", Rating: "*", AtMost: "<=", Cap: "[x]", CapLocked: "[ ]",
	Cursor: ">", FoldOpen: "-", FoldShut: "+", Crumb: " / ", Sep: " | ",
	Fill: "#", Empty: ".", Star: "*", StarOff: ".", Ellipsis: "...", Mask: "*",
	Times: "x", Approx: "~", Range: "-", Arrow: "->", Rule: "-",
	Page: "=", Warp: "^", Dotted: ":", DottedV: ":",
	Heat:   [5]string{" ", ".", ":", "*", "#"},
	Thin:   lipgloss.ASCIIBorder(),
	Thick:  asciiBorder("#", "=", "#"),
	Double: asciiBorder("+", "=", "|"),
}

func asciiBorder(corner, horizontal, vertical string) lipgloss.Border {
	return lipgloss.Border{
		Top: horizontal, Bottom: horizontal, Left: vertical, Right: vertical,
		TopLeft: corner, TopRight: corner, BottomLeft: corner, BottomRight: corner,
		MiddleLeft: corner, MiddleRight: corner, Middle: corner, MiddleTop: corner, MiddleBottom: corner,
	}
}

// G is the glyph set in use. Read it at render time.
var G = Unicode

// SetGlyphs picks the glyph set every screen draws with.
func SetGlyphs(g Glyphs) { G = g }

// AutoGlyphs picks the glyph set for an environment: ASCII on the Linux
// console (TERM=linux) or under a locale that names a charset other than
// UTF-8, Unicode otherwise. No locale at all counts as UTF-8, which is what
// macOS terminals use whether or not they export LANG.
func AutoGlyphs(getenv func(string) string) Glyphs {
	if getenv("TERM") == "linux" {
		return ASCII
	}
	for _, v := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		loc := getenv(v)
		if loc == "" {
			continue
		}
		up := strings.ToUpper(loc)
		if strings.Contains(up, "UTF-8") || strings.Contains(up, "UTF8") {
			return Unicode
		}
		return ASCII
	}
	return Unicode
}
