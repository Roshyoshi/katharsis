package ui

import (
	"strconv"
	"strings"

	"katharsis/internal/units"
)

// Hearts is lives as hearts, units.Hearts painted: the lives left Life,
// the lost ones Muted. "♥♥♡"
func Hearts(lives, maxLives int) string { return hearts(lives, maxLives, Token.Render) }

// BorderHearts is Hearts for a tmux format, such as the editor pane's
// border.
func BorderHearts(lives, maxLives int) string { return hearts(lives, maxLives, Token.Tmux) }

// hearts splits units.Hearts, which owns when to draw and when to count,
// into the lives left and lost, in the glyphs in use.
func hearts(lives, maxLives int, paint func(Token, string) string) string {
	h := units.Hearts(lives, maxLives)
	left := strings.TrimRight(h, "♡")
	glyph := strings.NewReplacer("♥", G.Life, "♡", G.LifeLost)
	return paint(Life, glyph.Replace(left)) + paint(Muted, glyph.Replace(h[len(left):]))
}

// Rating is a problem's difficulty, "◆1300", in its Codeforces rank color
// (D27). A player's own rating is a bare numeral painted with Rank.
func Rating(r int) string {
	return Rank(float64(r)).Render(G.Rating + strconv.Itoa(r))
}

// RatingBand is a span of difficulties, "◆850–1050", in the rank color of
// its middle.
func RatingBand(lo, hi int) string {
	return Rank(float64(lo+hi) / 2).Render(G.Rating + strconv.Itoa(lo) + G.Range + strconv.Itoa(hi))
}

// Verdict is the token for a judge verdict: AC is OK; WA, RE and CE are
// Danger; TLE, MLE and OLE are Warn (the solution was right as far as it
// got); anything else, such as a test still running, is Muted.
func Verdict(v string) Token {
	switch v {
	case "AC":
		return OK
	case "WA", "RE", "CE":
		return Danger
	case "TLE", "MLE", "OLE":
		return Warn
	}
	return Muted
}

// Mod is a door modifier, in the kernel's taint-flag order.
type Mod uint8

const (
	ModRegression Mod = iota
	ModMystery
	ModBlind
	ModCursed
	ModBoss
)

var mods = [...]struct {
	taint, word string
	paint       Token
	wholeToken  bool // paint the word too, not just the glyph
}{
	ModRegression: {"R", "regression", Warn, false},
	ModMystery:    {"M", "mystery", Special, true},
	ModBlind:      {"B", "blind", Muted, true},
	ModCursed:     {"C", "cursed", Danger, true},
	ModBoss:       {"X", "boss", Special, true},
}

// Glyph is m's glyph in the set in use: ☠ for cursed, or C in ASCII.
func (m Mod) Glyph() string {
	switch m {
	case ModRegression:
		return G.Regression
	case ModMystery:
		return G.Mystery
	case ModBlind:
		return G.Blind
	case ModCursed:
		return G.Cursed
	}
	return G.Boss
}

// Taint is m's kernel taint letter, the same in every glyph set: the crash
// screen's "Tainted: C (cursed)".
func (m Mod) Taint() string { return mods[m].taint }

// Word is m's name: "cursed".
func (m Mod) Word() string { return mods[m].word }

// Token is the token m is painted with.
func (m Mod) Token() Token { return mods[m].paint }

// Modifier is a door modifier's tag, its glyph and text: "☠ cursed",
// "⚔ Heisenbug". text "" is the modifier's word. Cursed, boss, mystery and
// blind paint the whole tag; a regression paints its glyph only, since it
// warns rather than threatens.
func Modifier(m Mod, text string) string {
	if text == "" {
		text = m.Word()
	}
	d := mods[m]
	if d.wholeToken {
		return d.paint.Render(m.Glyph() + " " + text)
	}
	return d.paint.Render(m.Glyph()) + " " + text
}
