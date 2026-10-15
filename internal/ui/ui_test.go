package ui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

// golden compares got with testdata/<name>.golden.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden (go test -update to accept):\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

// withGlyphs runs f with glyph set g and palette p, then restores the
// defaults.
func withGlyphs(t *testing.T, g Glyphs, p Palette, f func()) {
	t.Helper()
	SetGlyphs(g)
	SetPalette(p)
	defer func() {
		SetGlyphs(Unicode)
		SetPalette(ANSI)
	}()
	f()
}

var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// tags spells out SGR sequences as the tags ui-system.md uses, so a golden
// shows color and attributes as text: «G» green, «b» bold, «/» reset.
func tags(s string) string {
	return sgr.ReplaceAllStringFunc(s, func(seq string) string {
		params := strings.Split(sgr.FindStringSubmatch(seq)[1], ";")
		var out strings.Builder
		for i := 0; i < len(params); i++ {
			p := params[i]
			n, _ := strconv.Atoi(p)
			switch {
			case p == "" || p == "0":
				out.WriteString("«/»")
			case n == 1:
				out.WriteString("«b»")
			case n == 2:
				out.WriteString("«d»")
			case n == 3:
				out.WriteString("«i»")
			case n == 4:
				out.WriteString("«u»")
			case n == 5:
				out.WriteString("«k»")
			case n == 7:
				out.WriteString("«v»")
			case n == 9:
				out.WriteString("«s»")
			case n >= 30 && n <= 37:
				out.WriteString("«" + string("KRGYBMCW"[n-30]) + "»")
			case n >= 40 && n <= 47:
				out.WriteString("«" + string("KRGYBMCW"[n-40]) + "-bg»")
			case (n == 38 || n == 48) && i+2 < len(params) && params[i+1] == "5":
				bg := map[int]string{38: "", 48: "-bg"}[n]
				out.WriteString("«256" + bg + ":" + params[i+2] + "»")
				i += 2
			default:
				out.WriteString("«?" + p + "»")
			}
		}
		return out.String()
	})
}

// catalogue renders every component in the states screens use, one per
// line, as the visual contract the goldens hold.
func catalogue() string {
	var b strings.Builder
	section := func(name string) { fmt.Fprintf(&b, "\n-- %s --\n", name) }
	row := func(label, s string) { fmt.Fprintf(&b, "%-22s %s\n", label, tags(s)) }

	section("tokens")
	for _, c := range []struct {
		name string
		t    Token
	}{
		{"FG", FG}, {"Strong", Strong}, {"Muted", Muted}, {"Focus", Focus}, {"OK", OK},
		{"Warn", Warn}, {"Danger", Danger}, {"Special", Special}, {"Life", Life},
		{"Select", Select}, {"Field", Field}, {"Hit", Hit}, {"HitCurrent", HitCurrent},
		{"Voice", Voice}, {"Alert", Alert},
	} {
		row(c.name, c.t.Render("text"))
	}

	section("ratings")
	for _, r := range []int{800, 1199, 1200, 1399, 1400, 1599, 1600, 1899, 1900, 2099, 2100, 2399, 2400, 3500} {
		row(strconv.Itoa(r), Rating(r))
	}
	row("band 850-1050", RatingBand(850, 1050))
	row("band 1350-1650", RatingBand(1350, 1650))

	section("timer")
	m := time.Minute
	for _, c := range []struct {
		name        string
		left, total time.Duration
	}{
		{"calm 23:41 of 30", 23*m + 41*time.Second, 30 * m},
		{"warn 12:05 of 30", 12*m + 5*time.Second, 30 * m},
		{"low 05:12 of 30", 5*m + 12*time.Second, 30 * m},
		{"crit 01:40 of 30", 100 * time.Second, 30 * m},
		{"crit 00:00 of 30", 0, 30 * m},
		{"no total 17:00", 17 * m, 0},
	} {
		row(c.name, Timer(c.left, c.total))
		row("  border", BorderTimer(c.left, c.total))
	}

	section("hearts")
	for _, c := range [][2]int{{3, 3}, {2, 5}, {0, 3}, {7, 12}} {
		row(fmt.Sprintf("%d of %d", c[0], c[1]), Hearts(c[0], c[1]))
		row("  border", BorderHearts(c[0], c[1]))
	}

	section("modifiers")
	for _, md := range []Mod{ModRegression, ModMystery, ModBlind, ModCursed, ModBoss} {
		row(md.Word()+" "+md.Taint(), Modifier(md, ""))
	}
	row("boss Heisenbug", Modifier(ModBoss, "Heisenbug"))

	section("verdicts")
	for _, v := range []string{"AC", "WA", "RE", "CE", "TLE", "MLE", "OLE", "judging"} {
		row(v, Verdict(v).Render(v))
	}

	section("hp")
	for _, c := range []struct {
		w, passed, total int
		note             string
	}{
		{41, 0, 0, ""},
		{41, 14, 30, Muted.Render("judging 22/30…")},
		{41, 16, 30, Verdict("WA").Render("WA") + " #17" + Muted.Render(" · best 20/30")},
		{41, 30, 30, Verdict("AC").Render("AC")},
		{24, 16, 30, Verdict("WA").Render("WA") + " #17 · line 1"},
	} {
		row(fmt.Sprintf("w%d %d/%d", c.w, c.passed, c.total), HP(c.w, c.passed, c.total, c.note))
	}

	section("footer")
	keys := []string{"←/→", "select", "1-3", "jump", "enter", "open door", "m", "map", "q", "quit"}
	for _, w := range []int{80, 40, 20, 8} {
		row(fmt.Sprintf("w%d", w), Footer(w, keys...))
	}

	section("status bar")
	for _, w := range []int{80, 50, 30, 10} {
		row(fmt.Sprintf("w%d", w), StatusBar(w, "Manual page rtfm(7) line 1/30", "enter", "open", "/", "filter", "q", "quit"))
	}

	section("frames")
	body := []string{Strong.Render("Favorite Sequence"), "implementation · two pointers", "", Rating(800) + "  ⏱ 17:00  " + OK.Render("+41 B")}
	for _, k := range []struct {
		name string
		k    FrameKind
		tag  string
	}{
		{"idle", FrameIdle, ""}, {"selected", FrameSelected, Modifier(ModRegression, "")},
		{"cursed", FrameCursed, Modifier(ModCursed, "")}, {"cursed selected", FrameCursedSelected, Modifier(ModCursed, "")},
		{"boss", FrameBoss, Modifier(ModBoss, "Heisenbug")}, {"boss selected", FrameBossSelected, ""},
		{"done", FrameDone, ""}, {"here", FrameHere, ""},
	} {
		fmt.Fprintf(&b, "%s:\n", k.name)
		for _, l := range Frame(k.k, 40, G.Cursor+" 1 · CF Round 690 (Div. 3) · A", k.tag, body) {
			b.WriteString(tags(l) + "\n")
		}
	}
	row("title selected", FrameSelected.Render(G.Cursor+" 1"))

	section("text")
	row("clip 8", Clip("Codeforces Round 172", 8))
	row("fit 8", Fit("abc", 8)+"|")
	row("align right 8", AlignRight("1300", 8)+"|")
	row("spread 30", Spread("Boot #42", "uptime 47:12", 30)+"|")
	row("spread 15", Spread("Boot #42", "uptime 47:12", 15)+"|")
	row("center 20", Center("CLEARED", 20)+"|")
	row("wrap 12x2", strings.Join(WrapN("constructive algorithms · greedy · strings", 12, 2), " / "))
	row("rule 12", Rule(12))
	row("crumb", strings.Join([]string{Strong.Render("RTFM"), "Silver", "Prefix Sums"}, Muted.Render(G.Crumb)))
	row("caps", OK.Render(G.Cap+" CAP_DP")+"  "+Muted.Render(G.CapLocked+" CAP_PREFIX"))
	return b.String()
}

func TestGoldenCatalogue(t *testing.T) {
	golden(t, "catalogue", catalogue())
	withGlyphs(t, ASCII, ANSI, func() { golden(t, "catalogue_ascii", catalogue()) })
	withGlyphs(t, Unicode, Mono, func() { golden(t, "catalogue_mono", catalogue()) })
}

// glyphWidths is the width every Unicode glyph must take, by field. A
// field missing here fails the test, so a new glyph gets checked.
var glyphWidths = map[string]int{
	"Timer": 2, "Duck": 2, "Crumb": 3, "Sep": 3,
}

func TestGlyphWidths(t *testing.T) {
	check := func(set string, g Glyphs, want func(name, s string) int) {
		v := reflect.ValueOf(g)
		for i := range v.NumField() {
			f := v.Type().Field(i)
			switch val := v.Field(i).Interface().(type) {
			case string:
				if w := want(f.Name, val); ansi.StringWidth(val) != w {
					t.Errorf("%s.%s = %q is %d cells, want %d", set, f.Name, val, ansi.StringWidth(val), w)
				}
				if val == "" {
					t.Errorf("%s.%s is empty", set, f.Name)
				}
			case lipgloss.Border:
				for _, s := range []string{val.Top, val.Bottom, val.Left, val.Right, val.TopLeft, val.TopRight, val.BottomLeft, val.BottomRight} {
					if ansi.StringWidth(s) != 1 {
						t.Errorf("%s.%s border glyph %q is %d cells, want 1", set, f.Name, s, ansi.StringWidth(s))
					}
				}
			default:
				t.Errorf("%s.%s: unchecked glyph type %T", set, f.Name, val)
			}
		}
	}
	check("Unicode", Unicode, func(name, _ string) int {
		if w, ok := glyphWidths[name]; ok {
			return w
		}
		return 1
	})
	check("ASCII", ASCII, func(name, s string) int {
		for _, r := range s {
			if r > 0x7e {
				t.Errorf("ASCII.%s = %q is not ASCII", name, s)
			}
		}
		return len(s)
	})
}

// TestOneMeaning checks no two meanings share a Unicode glyph, except the
// two the design gives one: ☠ is lethal, so it marks both a crash and a
// cursed door, whose one WA is a crash.
func TestOneMeaning(t *testing.T) {
	shared := map[string]bool{"Crash/Cursed": true}
	seen := map[string]string{}
	v := reflect.ValueOf(Unicode)
	for i := range v.NumField() {
		s, ok := v.Field(i).Interface().(string)
		if !ok {
			continue
		}
		name := v.Type().Field(i).Name
		if prev, dup := seen[s]; dup && !shared[prev+"/"+name] {
			t.Errorf("%s and %s are both %q", prev, name, s)
		}
		seen[s] = name
	}
}

// TestFits renders the width-taking components at every width from 0 to
// 60: none may be wider than asked, and Fit, AlignRight, StatusBar and
// Frame are exactly as wide.
func TestFits(t *testing.T) {
	for _, g := range []Glyphs{Unicode, ASCII} {
		withGlyphs(t, g, ANSI, func() {
			long := "Codeforces Round 172 (Div. 2) · Word Capitalization"
			for w := 0; w <= 60; w++ {
				exact := map[string]string{
					"Fit":        Fit(long, w),
					"AlignRight": AlignRight(long, w),
					"StatusBar":  StatusBar(w, long, "enter", "open", "q", "quit"),
				}
				for name, s := range exact {
					if got := ansi.StringWidth(s); got != w {
						t.Errorf("%s at %d is %d wide: %q", name, w, got, ansi.Strip(s))
					}
				}
				most := map[string]string{
					"Clip":   Clip(long, w),
					"Spread": Spread(long, "uptime 47:12", w),
					"Footer": Footer(w, "enter", "open door", "q", "quit"),
					"HP":     HP(w, 16, 30, "WA #17 · best 20/30"),
					"Center": Center("CLEARED", w),
					"Layout": Layout(w, 3, []string{long, long + "\n" + long}, long),
				}
				for _, l := range WrapN(long, w, 2) {
					most["WrapN"] = l
				}
				for name, s := range most {
					for _, l := range strings.Split(s, "\n") {
						if got := ansi.StringWidth(l); got > max(w, 0) {
							t.Errorf("%s at %d is %d wide: %q", name, w, got, ansi.Strip(l))
						}
					}
				}
				if w >= 4 {
					for _, l := range Frame(FrameSelected, w, long, Modifier(ModCursed, ""), []string{long}) {
						if got := ansi.StringWidth(l); got != w {
							t.Errorf("Frame at %d: line is %d wide: %q", w, got, ansi.Strip(l))
						}
					}
				}
			}
		})
	}
}

func TestLayoutPinsFooter(t *testing.T) {
	got := Layout(10, 4, []string{"a", "b\nc", "d", "e"}, "keys")
	if want := "a\nb\nc\nkeys"; got != want {
		t.Errorf("Layout = %q, want %q", got, want)
	}
	if got := Layout(10, 3, []string{"a"}, ""); got != "a\n\n" {
		t.Errorf("Layout pads to the height: %q", got)
	}
}

func TestTimerTones(t *testing.T) {
	m := time.Minute
	for _, c := range []struct {
		left, total time.Duration
		want        Token
	}{
		{30 * m, 30 * m, OK}, {15*m + time.Second, 30 * m, OK}, {15 * m, 30 * m, Warn},
		{6*m + time.Second, 30 * m, Warn}, {6 * m, 30 * m, timerLow},
		{2 * m, 30 * m, timerLow}, {2*m - time.Millisecond, 30 * m, timerCrit},
		{-time.Second, 30 * m, timerCrit}, {10 * m, 0, OK},
		// Under two minutes is critical even when that is most of the timer.
		{90 * time.Second, 2 * m, timerCrit},
	} {
		if got := timerTone(c.left, c.total); got != c.want {
			t.Errorf("timerTone(%v, %v) = %d, want %d", c.left, c.total, got, c.want)
		}
	}
}

func TestTmux(t *testing.T) {
	if got, want := BorderTimer(100*time.Second, 30*time.Minute), "#[fg=colour1,reverse]⏱ 01:40#[default]"; got != want {
		t.Errorf("BorderTimer = %q, want %q", got, want)
	}
	if got, want := BorderHearts(2, 5), "#[fg=colour1]♥♥#[default]#[dim]♡♡♡#[default]"; got != want {
		t.Errorf("BorderHearts = %q, want %q", got, want)
	}
	if got, want := Strong.Tmux("#17 #[x]"), "#[bold]##17 ##[x]#[default]"; got != want {
		t.Errorf("Tmux escapes the text: %q, want %q", got, want)
	}
	if got := FG.Tmux("a#b"); got != "a##b" {
		t.Errorf("FG.Tmux = %q", got)
	}
	withGlyphs(t, Unicode, Mono, func() {
		if got, want := Field.Tmux("x"), "#[reverse]x#[default]"; got != want {
			t.Errorf("Mono Field.Tmux = %q, want %q", got, want)
		}
	})
}

func TestAutoGlyphs(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{}, "Unicode"},
		{map[string]string{"LANG": "en_US.UTF-8"}, "Unicode"},
		{map[string]string{"LC_CTYPE": "UTF-8", "LANG": "C"}, "Unicode"},
		{map[string]string{"LANG": "en_US.utf8"}, "Unicode"},
		{map[string]string{"LANG": "C"}, "ASCII"},
		{map[string]string{"LC_ALL": "POSIX", "LANG": "en_US.UTF-8"}, "ASCII"},
		{map[string]string{"TERM": "linux", "LANG": "en_US.UTF-8"}, "ASCII"},
	} {
		got := "Unicode"
		if reflect.DeepEqual(AutoGlyphs(func(k string) string { return c.env[k] }), ASCII) {
			got = "ASCII"
		}
		if got != c.want {
			t.Errorf("AutoGlyphs(%v) = %s, want %s", c.env, got, c.want)
		}
	}
}

// TestStyleConfigFollowsTheTheme renders markdown with every element the
// reader meets through Glamour with StyleConfig: every color in the output
// must be one of the terminal's ANSI 1-6, and nothing may set a
// background. Glamour's "dark" style fails this with fixed 256 colors.
func TestStyleConfigFollowsTheTheme(t *testing.T) {
	md := "# Title\n\n## Section\n\nBody with *emphasis*, **strong**, `code` and a [link](https://usaco.guide).\n\n" +
		"> a quote\n\n- item\n- item\n\n1. one\n\n---\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n" +
		"```cpp\n#include <bits/stdc++.h>\n// comment\nint main() { long long x = 42; cout << \"hi\\n\"; return 0; }\n```\n"
	render := func() string {
		r, err := glamour.NewTermRenderer(glamour.WithStyles(StyleConfig()), glamour.WithWordWrap(80))
		if err != nil {
			t.Fatal(err)
		}
		out, err := r.Render(md)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	allowed := regexp.MustCompile(`^(|0|1|2|3|4|7|9|22|23|24|27|29|39|3[1-6]|38;5;[1-6])$`)
	colors := map[string]bool{}
	for _, m := range sgr.FindAllStringSubmatch(render(), -1) {
		for _, p := range splitSGR(m[1]) {
			if !allowed.MatchString(p) {
				t.Errorf("SGR %q is not a theme color or a plain attribute", p)
			}
			if p != "" && p != "0" && (strings.HasPrefix(p, "3") || strings.HasPrefix(p, "38")) && p != "39" {
				colors[p] = true
			}
		}
	}
	// The code block is highlighted: keywords, types, strings, numbers
	// and the preprocessor line take five different hues.
	if len(colors) < 5 {
		t.Errorf("code highlighting used %d colors (%v), want at least 5", len(colors), colors)
	}
}

// splitSGR splits SGR parameters, keeping 38;5;n together.
func splitSGR(s string) []string {
	ps := strings.Split(s, ";")
	var out []string
	for i := 0; i < len(ps); i++ {
		if (ps[i] == "38" || ps[i] == "48") && i+2 < len(ps) {
			out = append(out, strings.Join(ps[i:i+3], ";"))
			i += 2
			continue
		}
		out = append(out, ps[i])
	}
	return out
}

func TestPlainUnderMono(t *testing.T) {
	withGlyphs(t, Unicode, Mono, func() {
		for _, s := range []string{Rating(2400), Timer(time.Minute, time.Hour), Hearts(1, 3), Modifier(ModCursed, "")} {
			for _, m := range sgr.FindAllStringSubmatch(s, -1) {
				for _, p := range splitSGR(m[1]) {
					if strings.HasPrefix(p, "3") && p != "3" || strings.HasPrefix(p, "4") && p != "4" {
						t.Errorf("Mono painted a hue: SGR %q in %q", p, s)
					}
				}
			}
		}
	})
}
