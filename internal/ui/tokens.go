// Package ui is katharsis's visual system (docs/design/ui-system.md §2):
// color tokens and the palette behind them, the glyph set, and the
// components screens draw with: frames, the pinned layout, footers and
// status bars, the HP bar, the timer, and the small tokens for hearts,
// ratings, modifiers and verdicts. Numbers are formatted by internal/units;
// this package paints them.
//
// A token names a meaning, not a color: OK is success and gain wherever it
// appears. The palette maps each meaning onto the terminal's own 16 ANSI
// colors, so every screen follows the player's theme (D26: dark terminals
// only) and nothing fixes an RGB value.
//
// The palette and the glyph set are process-wide. The app picks them once,
// before the program starts; nothing changes them while screens render.
package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Token is a meaning a piece of text can carry. Render paints text with it.
type Token uint8

const (
	FG          Token = iota // body text: the terminal's own foreground
	Strong                   // names, titles, values, key caps: bold
	Muted                    // secondary text, labels, idle frames, hidden info: faint
	Focus                    // cursor, selected frame, "you are here", active filter
	OK                       // success and gain: AC, ✓, cleared, HP fill, +bytes
	Warn                     // time pressure, soft caution: TLE/MLE/OLE, regression, retreat
	Danger                   // failure, loss, lethal: WA/RE/CE, crash, cursed, a timer's last fifth
	Special                  // boss and mystery doors
	Life                     // ♥; a lost life is Muted
	Select                   // a parody list's selected row (GRUB, git log, man): reverse
	Field                    // htop's cyan bar, top only
	FieldHeader              // htop's green column header, top only
	Hit                      // a search hit
	HitCurrent               // the search hit the cursor is on
	Voice                    // the flavor voice: route blurbs, boss lines, quotes, "tags hidden"
	Alert                    // Danger in reverse, a banner that must stop the eye: the :q! confirm, an error banner

	// The countdown's last two tones (calm and warn are OK and Warn) and
	// the Codeforces rank ramp (newbie is Muted). Callers reach them
	// through Timer and Rank.
	timerLow
	timerCrit
	rankPupil
	rankSpecialist
	rankExpert
	rankCandidate
	rankMaster
	rankGrandmaster

	numTokens
)

// hue is an ANSI color, or none for the terminal's own. Only 1 to 6 paint
// text: 0 and 7 are a theme's background or foreground on some themes, and
// 8 to 15 vary too much (Solarized maps 8 to its background and 12 to its
// body text).
type hue uint8

const (
	none hue = iota
	black
	red
	green
	yellow
	blue
	magenta
	cyan
)

// index is h's ANSI palette index, as a string: black is "0".
func (h hue) index() string { return strconv.Itoa(int(h) - 1) }

// look is how a token is drawn: its colors and attributes.
type look struct {
	fg, bg                                  hue
	bold, faint, italic, underline, reverse bool
}

var looks = [numTokens]look{
	FG:          {},
	Strong:      {bold: true},
	Muted:       {faint: true},
	Focus:       {fg: cyan},
	OK:          {fg: green},
	Warn:        {fg: yellow},
	Danger:      {fg: red},
	Special:     {fg: magenta},
	Life:        {fg: red},
	Select:      {reverse: true},
	Field:       {fg: black, bg: cyan},
	FieldHeader: {fg: black, bg: green},
	Hit:         {underline: true},
	HitCurrent:  {bold: true, reverse: true},
	Voice:       {italic: true},
	Alert:       {fg: red, reverse: true},

	// Low and critical carry a shape cue as well as red, so the timer's
	// urgency reads without color: bold, then a reverse block. Never
	// blink: the HUD sits in the corner of the eye while the player types.
	timerLow:  {fg: red, bold: true},
	timerCrit: {fg: red, reverse: true},

	rankPupil:       {fg: green},
	rankSpecialist:  {fg: cyan},
	rankExpert:      {fg: blue},
	rankCandidate:   {fg: magenta},
	rankMaster:      {fg: yellow},
	rankGrandmaster: {fg: red},
}

// Palette is how tokens reach the terminal.
type Palette uint8

const (
	// ANSI paints with the terminal's 16-color theme (the default).
	ANSI Palette = iota
	// Mono drops every hue and keeps the attributes, for NO_COLOR. Bubble
	// Tea already strips colors from the screen under NO_COLOR; Mono is for
	// what it does not draw, such as the tmux border, and keeps a colored
	// background readable as reverse.
	Mono
)

var (
	styles [numTokens]lipgloss.Style
	tmux   [numTokens]string // tmux style directives, "" for none
	mono   bool
)

func init() { SetPalette(ANSI) }

// SetPalette picks the palette every token renders with.
func SetPalette(p Palette) {
	mono = p == Mono
	for t, l := range looks {
		if mono {
			if l.bg != none {
				l.reverse = true
			}
			l.fg, l.bg = none, none
		}
		styles[t] = l.style()
		tmux[t] = l.tmux()
	}
}

// Hued reports whether the palette paints hues: false under Mono. Art
// that paints colors of its own, such as the title's fire, draws its
// hue-free form when it is false.
func Hued() bool { return !mono }

func (l look) style() lipgloss.Style {
	s := lipgloss.NewStyle()
	if l.fg != none {
		s = s.Foreground(lipgloss.Color(l.fg.index()))
	}
	if l.bg != none {
		s = s.Background(lipgloss.Color(l.bg.index()))
	}
	return s.Bold(l.bold).Faint(l.faint).Italic(l.italic).
		Underline(l.underline).Reverse(l.reverse)
}

// tmux spells l as a tmux style: "fg=colour2,bold". colourN is the
// terminal's palette entry N, so the border follows the theme too.
func (l look) tmux() string {
	var parts []string
	if l.fg != none {
		parts = append(parts, "fg=colour"+l.fg.index())
	}
	if l.bg != none {
		parts = append(parts, "bg=colour"+l.bg.index())
	}
	for _, a := range []struct {
		on   bool
		name string
	}{{l.bold, "bold"}, {l.faint, "dim"}, {l.italic, "italics"}, {l.underline, "underscore"}, {l.reverse, "reverse"}} {
		if a.on {
			parts = append(parts, a.name)
		}
	}
	return strings.Join(parts, ",")
}

// Render paints s with t.
func (t Token) Render(s string) string {
	if t == FG || s == "" {
		return s
	}
	return styles[t].Render(s)
}

// Style is t as a Lip Gloss style, for APIs that take one (StyleRanges).
func (t Token) Style() lipgloss.Style { return styles[t] }

// Tmux paints s with t for a tmux format, such as the editor pane's border
// text: "#[fg=colour2]⏱ 23:41#[default]". A # in s is escaped, so s is
// never read as a directive.
func (t Token) Tmux(s string) string {
	if tmux[t] == "" || s == "" {
		return TmuxEscape(s)
	}
	return "#[" + tmux[t] + "]" + TmuxEscape(s) + "#[default]"
}

// TmuxEscape makes s print as itself in a tmux format: # becomes ##.
func TmuxEscape(s string) string { return strings.ReplaceAll(s, "#", "##") }

// Rank is the Codeforces rank color of a rating, the scale every rating in
// the game is on: Muted newbie (not ANSI 8, which some themes paint as
// their background) through red grandmaster. Rank colors paint rating
// numerals and nothing else.
func Rank(r float64) Token {
	switch {
	case r < 1200:
		return Muted
	case r < 1400:
		return rankPupil
	case r < 1600:
		return rankSpecialist
	case r < 1900:
		return rankExpert
	case r < 2100:
		return rankCandidate
	case r < 2400:
		return rankMaster
	}
	return rankGrandmaster
}
