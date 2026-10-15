package anim

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var lines = []string{
	`Down, down, down. Would the fall never come to an end?`,
	`"Oh dear! Oh dear! I shall be late!"`,
	``,
	`hello, world`,
}

// clips is one of each primitive, and the compositions, for the
// properties every clip shares.
func clips() map[string]Clip {
	italic := lipgloss.NewStyle().Italic(true)
	screenA := []string{"the old screen", "", "row three"}
	screenB := []string{"the new screen"}
	return map[string]Clip{
		"typewriter":    Typewriter{Lines: lines, Style: italic},
		"reveal":        Reveal{Lines: lines, Total: 600 * time.Millisecond},
		"fade in":       Fade{Lines: lines, From: Gone, To: Normal, D: 400 * time.Millisecond},
		"fade out":      Fade{Lines: lines, From: Bold, To: Gone, D: 400 * time.Millisecond, Salt: 3},
		"cheshire in":   Cheshire{Appear: true, D: time.Second},
		"cheshire out":  Cheshire{D: time.Second},
		"cheshire dump": Cheshire{D: time.Second, KeepGrin: true},
		"card": Card{Label: "⚔ BOSS", Name: "Heisenbug", W: 60, D: 1200 * time.Millisecond,
			Paras: []Para{{Text: "Fails on the judge; passes the moment you add a printf."},
				{Text: "The Sheep's shop: look hard at any shelf and it's quite empty.", Style: italic}}},
		"fall": Fall{From: screenA, To: screenB, W: 50, H: 12, D: 900 * time.Millisecond, Label: "The Cache"},
		"fire": testFire(24, 8).Clip(),
		"then": Then(Fade{Lines: lines[:1], From: Gone, To: Normal, D: 200 * time.Millisecond},
			Typewriter{Lines: lines[1:]}, Hold{Lines: []string{"press any key"}}),
		"seq": Seq(Cheshire{Appear: true, D: 500 * time.Millisecond}, Hold{Lines: nil, D: 0},
			Cheshire{D: 500 * time.Millisecond, KeepGrin: true}),
		"stack": Stack(Reveal{Lines: lines, Total: 300 * time.Millisecond},
			Delay(100*time.Millisecond, Fade{Lines: lines, From: Gone, To: Faint, D: 300 * time.Millisecond})),
		"wipe": Wipe{Lines: []string{italic.Render("styled ") + "and plain", "short"}, D: 360 * time.Millisecond},
	}
}

func TestWipe(t *testing.T) {
	bold := lipgloss.NewStyle().Bold(true)
	w := Wipe{Lines: []string{bold.Render("abcd") + "efgh", "ab"}, D: 900 * time.Millisecond}
	// Eight columns over 900 ms: one more every 100 ms.
	if got := plain(w.Frame(450 * time.Millisecond)); !slices.Equal(got, []string{"abcd", "ab"}) {
		t.Errorf("half way: %q", got)
	}
	if f := w.Frame(250 * time.Millisecond)[0]; ansi.Strip(f) != "ab      " || !strings.Contains(f, "\x1b[1m") {
		t.Errorf("a cut lost its style or its width: %q", f)
	}
}

// samples are times across c: its change points, either side of each, and
// past the end.
func samples(c Clip) []time.Duration {
	ts := []time.Duration{0, 1, c.Len() / 2}
	for t := time.Duration(0); t < c.Len(); t = c.Next(t) {
		ts = append(ts, t, t+1, max(t-1, 0))
	}
	return append(ts, c.Len(), c.Len()+time.Hour)
}

// TestFrameKeepsItsShape: at every time, a clip has its last frame's line
// count and each line's width, so a screen's layout never jumps.
func TestFrameKeepsItsShape(t *testing.T) {
	for name, c := range clips() {
		last := c.Frame(c.Len())
		if len(last) == 0 {
			t.Errorf("%s: empty last frame", name)
		}
		for _, at := range samples(c) {
			f := c.Frame(at)
			if len(f) != len(last) {
				t.Errorf("%s at %v: %d lines, last frame has %d", name, at, len(f), len(last))
				continue
			}
			for i := range f {
				if w, want := ansi.StringWidth(f[i]), ansi.StringWidth(last[i]); w != want {
					t.Errorf("%s at %v, line %d: width %d, want %d: %q", name, at, i, w, want, f[i])
				}
			}
		}
		if !slices.Equal(c.Frame(c.Len()+time.Hour), last) {
			t.Errorf("%s: a frame past the end is not the last frame", name)
		}
	}
}

// TestNextNeverSkipsAChange: the frame holds still from t until Next(t),
// so a Player that sleeps until Next misses nothing; and Next moves on.
func TestNextNeverSkipsAChange(t *testing.T) {
	for name, c := range clips() {
		steps := 0
		for at := time.Duration(0); at < c.Len(); {
			next := c.Next(at)
			if next <= at {
				t.Fatalf("%s: Next(%v) = %v does not move on", name, at, next)
			}
			f := c.Frame(at)
			for _, s := range []time.Duration{at + 1, at + (next-at)/2, next - 1} {
				if s < next && !slices.Equal(c.Frame(s), f) {
					t.Errorf("%s: the frame changed at %v, before Next(%v) = %v", name, s, at, next)
					break
				}
			}
			at = next
			if steps++; steps > 10000 {
				t.Fatalf("%s: more than 10000 changes", name)
			}
		}
	}
}

func plain(f []string) []string {
	out := make([]string, len(f))
	for i, l := range f {
		out[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	return out
}

func TestTypewriter(t *testing.T) {
	w := Typewriter{Lines: lines, CPS: 100}
	if got := strings.Join(plain(w.Frame(0)), ""); got != "" {
		t.Errorf("Frame(0) shows %q", got)
	}
	if got := plain(w.Frame(w.Len())); !slices.Equal(got, lines) {
		t.Errorf("last frame %q", got)
	}
	// One rune per 10 ms, so the first word is out at 40 ms.
	if got := plain(w.Frame(40 * time.Millisecond))[0]; got != "Down" {
		t.Errorf("at 40ms: %q, want %q", got, "Down")
	}
	// A comma holds the next rune for three extra beats.
	at5 := w.Next(40*time.Millisecond) - 40*time.Millisecond // the comma
	if at5 != 10*time.Millisecond {
		t.Errorf("the comma took %v", at5)
	}
	comma := w.Next(40 * time.Millisecond)
	if gap := w.Next(comma) - comma; gap != 40*time.Millisecond {
		t.Errorf("the beat after a comma is %v, want 40ms", gap)
	}
	prev := 0
	ts := samples(w)
	slices.Sort(ts)
	for _, at := range ts {
		n := len(strings.Join(plain(w.Frame(at)), ""))
		if at <= w.Len() && n < prev {
			t.Errorf("text shrank at %v", at)
		}
		prev = n
	}
}

func TestReveal(t *testing.T) {
	r := Reveal{Lines: lines, Total: 400 * time.Millisecond}
	for i := range lines {
		at := r.Total * time.Duration(i+1) / time.Duration(len(lines))
		if got := r.Shown(at); got != i+1 {
			t.Errorf("Shown(%v) = %d, want %d", at, got, i+1)
		}
		if got := r.Shown(at - 1); got != i {
			t.Errorf("Shown(%v) = %d, want %d", at-1, got, i)
		}
	}
	f := plain(r.Frame(r.Total / 2))
	if !slices.Equal(f, []string{lines[0], lines[1], "", ""}) {
		t.Errorf("half way: %q", f)
	}
}

func TestFadeSteps(t *testing.T) {
	f := Fade{Lines: lines, From: Gone, To: Normal, D: 300 * time.Millisecond}
	want := []Level{Gone, Dust, Faint, Normal}
	for i, l := range want {
		if got := f.LevelAt(100 * time.Millisecond * time.Duration(i)); got != l {
			t.Errorf("LevelAt(%dms) = %d, want %d", 100*i, got, l)
		}
	}
	// Dust keeps some cells and blanks the rest, the same way every time.
	d := plain(f.Frame(150 * time.Millisecond))[0]
	if d == "" || d == lines[0] || d != plain(f.Frame(150 * time.Millisecond))[0] {
		t.Errorf("dust %q", d)
	}
	out := Fade{Lines: lines, From: Normal, To: Gone, D: 300 * time.Millisecond}
	if got := strings.Join(plain(out.Frame(out.D)), ""); got != "" {
		t.Errorf("faded out, still shows %q", got)
	}
	// Faint and bold are attributes, not different text.
	for _, l := range []Level{Faint, Normal, Bold} {
		if got := ansi.Strip(Shade("abc", l, lipgloss.NewStyle(), 0)); got != "abc" {
			t.Errorf("level %d draws %q", l, got)
		}
	}
}

// cells is the non-blank cells of a frame.
func cells(f []string) string {
	return strings.Join(strings.Fields(strings.Join(plain(f), " ")), "")
}

func TestCheshire(t *testing.T) {
	smile := `\_________________/\_|_|_|_|_|_|_|_/`
	c := Cheshire{D: time.Second, KeepGrin: true}
	if got := cells(c.Frame(c.Len())); got != smile {
		t.Errorf("a kept grin leaves %q, want just the grin", got)
	}
	full := cells(c.Frame(0))
	if !strings.Contains(full, "(O)") || !strings.Contains(full, "))") {
		t.Errorf("the cat at t=0 is %q", full)
	}
	// The tail goes first: by the end of the first slot it is gone and
	// the grin is whole.
	_, tailEnd, _ := c.window(tail)
	f := plain(c.Frame(tailEnd))
	if strings.Contains(strings.Join(f, ""), "( )") || !strings.Contains(strings.Join(f, ""), `\_|_|_|_|_|_|_|_/`) {
		t.Errorf("after the tail's slot:\n%s", strings.Join(f, "\n"))
	}
	if got := cells(Cheshire{D: time.Second}.Frame(time.Second)); got != "" {
		t.Errorf("a full vanish leaves %q", got)
	}
	// Appearing goes grin first.
	in := Cheshire{Appear: true, D: time.Second}
	_, grinIn, _ := in.window(grin)
	if got := cells(in.Frame(grinIn)); got != smile {
		t.Errorf("appearing, the grin's slot ends on %q", got)
	}
	if got := cells(in.Frame(in.Len())); got != full {
		t.Errorf("appeared %q, want %q", got, full)
	}
}

func TestCard(t *testing.T) {
	c := clips()["card"].(Card)
	last := plain(c.Frame(c.Len()))
	text := strings.Join(last, "\n")
	for _, want := range []string{"⚔ BOSS", "H E I S E N B U G", "add a printf", "empty."} {
		if !strings.Contains(text, want) {
			t.Errorf("last frame lacks %q:\n%s", want, text)
		}
	}
	if !strings.HasPrefix(last[0], "╔══ ⚔ BOSS ") || !strings.HasPrefix(last[len(last)-1], "╚") {
		t.Errorf("borders: %q / %q", last[0], last[len(last)-1])
	}
	// The card opens from the middle: just after the start only the
	// middle rows are drawn, and there is no name yet.
	first := plain(c.Frame(1))
	drawn := 0
	for _, l := range first {
		if l != "" {
			drawn++
		}
	}
	if drawn == 0 || drawn > 2 || strings.Contains(strings.Join(first, ""), "H E") {
		t.Errorf("at the start:\n%s", strings.Join(first, "\n"))
	}
	// Half way through the stamp, half the name is in.
	mid := c.at((cardOpen + cardStamp) / 2)
	if got := strings.Join(plain(c.Frame(mid)), ""); !strings.Contains(got, "H E I S") ||
		strings.Contains(got, "B U G") || strings.Contains(got, "printf") {
		t.Errorf("mid-stamp:\n%s", strings.Join(plain(c.Frame(mid)), "\n"))
	}
	// A name too wide to space is stamped plain.
	long := Card{Name: "The Halting Problem", W: 30, D: time.Second}
	if got := string(long.name()); got != "THE HALTING PROBLEM" {
		t.Errorf("a long name stamps as %q", got)
	}
}

func TestFall(t *testing.T) {
	f := clips()["fall"].(Fall)
	if got := plain(f.Frame(0)); got[0] != "the old screen" || got[2] != "row three" {
		t.Errorf("Frame(0) = %q", got)
	}
	if got := plain(f.Frame(f.Len())); got[0] != "the new screen" || len(got) != f.H {
		t.Errorf("last frame = %q", got)
	}
	// Somewhere in the middle you are in the shaft, and pass the label.
	sawShaft, sawLabel := false, false
	for _, at := range samples(f) {
		s := strings.Join(plain(f.Frame(at)), "\n")
		sawShaft = sawShaft || strings.Contains(s, "|")
		sawLabel = sawLabel || strings.Contains(s, "· The Cache ·")
	}
	if !sawShaft || !sawLabel {
		t.Errorf("the fall never showed the shaft (%v) or the label (%v)", sawShaft, sawLabel)
	}
	for _, l := range shaft(80, 30, "a label far too long for a narrow shaft, longer than forty-four", 1) {
		if w := ansi.StringWidth(l); w > 80 {
			t.Errorf("shaft row %d cells wide: %q", w, l)
		}
	}
}

// run drives p to the end the way tea.Tick would, without sleeping: each
// tick is the message its command would have sent. It returns the steps.
func run(t *testing.T, p *Player, cmd tea.Cmd) []time.Duration {
	t.Helper()
	var steps []time.Duration
	for cmd != nil {
		if len(steps) > 1000 {
			t.Fatal("more than 1000 ticks")
		}
		steps = append(steps, p.wait)
		var used bool
		cmd, used = p.Update(frameMsg{id: p.id, gen: p.gen})
		if !used {
			t.Fatal("the player's own tick was not used")
		}
	}
	return steps
}

func TestPlayerCapsTheFrameRate(t *testing.T) {
	c := Typewriter{Lines: lines, CPS: 1000} // a rune a millisecond
	p := New(c, Motion{FPS: 20})
	steps := run(t, p, p.Start())
	if !p.Done() || !slices.Equal(p.Frame(), c.Frame(c.Len())) {
		t.Fatal("the player did not end on the last frame")
	}
	for i, s := range steps {
		if s < 50*time.Millisecond {
			t.Errorf("tick %d after %v, faster than 20 fps", i, s)
		}
	}
	if max := int(c.Len()/(50*time.Millisecond)) + 1; len(steps) > max {
		t.Errorf("%d ticks for a %v clip at 20 fps, want at most %d", len(steps), c.Len(), max)
	}
}

// TestPlayerIdlesBetweenChanges: a clip that holds still asks for no
// frames in between, and a finished one for none at all.
func TestPlayerIdlesBetweenChanges(t *testing.T) {
	c := Then(Hold{Lines: []string{"wait"}, D: 2 * time.Second},
		Reveal{Lines: []string{"a", "b"}, Total: 200 * time.Millisecond})
	p := New(c, Motion{})
	steps := run(t, p, p.Start())
	if want := []time.Duration{2100 * time.Millisecond, 100 * time.Millisecond}; !slices.Equal(steps, want) {
		t.Errorf("ticks %v, want %v", steps, want)
	}
	if cmd, _ := p.Update(frameMsg{id: p.id, gen: p.gen}); cmd != nil {
		t.Error("a finished player ticks again")
	}
}

func TestPlayerReducedMotion(t *testing.T) {
	c := Cheshire{D: time.Second, KeepGrin: true}
	p := New(c, Motion{Reduced: true})
	if cmd := p.Start(); cmd != nil {
		t.Error("reduced motion still ticks")
	}
	if !p.Done() || !slices.Equal(p.Frame(), c.Frame(c.Len())) {
		t.Error("reduced motion does not show the last frame at once")
	}
	SetMotion(Motion{Reduced: true})
	defer SetMotion(Motion{})
	if !CurrentMotion().Reduced {
		t.Error("CurrentMotion ignores SetMotion")
	}
}

func TestPlayerInput(t *testing.T) {
	c := Typewriter{Lines: lines}
	p := New(c, Motion{})
	if p.Start() == nil {
		t.Fatal("no first tick")
	}
	key := tea.KeyPressMsg{Code: 'x', Text: "x"}
	if _, used := p.Update(key); !used || !p.Done() {
		t.Fatal("a key while typing does not complete the text")
	}
	if _, used := p.Update(key); used {
		t.Error("a key after the end is still the player's")
	}
	// The tick that was in flight lands after the skip and does nothing.
	if cmd, used := p.Update(frameMsg{id: p.id, gen: p.gen}); !used || cmd != nil {
		t.Errorf("a stale tick: used %v, cmd %v", used, cmd != nil)
	}
	// A restart drops the old run's ticks, and other players' ticks are
	// not ours.
	old := p.gen
	p.Start()
	if cmd, used := p.Update(frameMsg{id: p.id, gen: old}); !used || cmd != nil || p.at != 0 {
		t.Error("a tick from before the restart moved the clip")
	}
	q := New(c, Motion{})
	if _, used := p.Update(frameMsg{id: q.id, gen: p.gen}); used {
		t.Error("a player took another player's tick")
	}
	p.Seek(c.Len() / 2)
	if p.Done() || p.at != c.Len()/2 {
		t.Errorf("Seek: at %v", p.at)
	}
}

func TestParseMotion(t *testing.T) {
	for _, c := range []struct {
		motion string
		fps    int
		want   Motion
	}{
		{"", 0, Motion{}},
		{"full", 60, Motion{FPS: 60}},
		{"reduced", 0, Motion{Reduced: true}},
		{"reduced", 12, Motion{Reduced: true, FPS: 12}},
	} {
		if got, err := ParseMotion(c.motion, c.fps); err != nil || got != c.want {
			t.Errorf("ParseMotion(%q, %d) = %+v, %v; want %+v", c.motion, c.fps, got, err, c.want)
		}
	}
	for _, bad := range []struct {
		motion string
		fps    int
	}{{"none", 0}, {"Reduced", 0}, {"full", -1}} {
		if _, err := ParseMotion(bad.motion, bad.fps); err == nil {
			t.Errorf("ParseMotion(%q, %d) took it", bad.motion, bad.fps)
		}
	}
}

// TestPlayerSwapKeepsAFinishedClipFinished: a screen rebuilds its clip on
// a resize, and a narrower pane rewraps text into more lines, so the new
// clip runs longer. A clip that had finished must stay finished: nothing
// is in flight to play the rest, so it would freeze part-typed.
func TestPlayerSwapKeepsAFinishedClipFinished(t *testing.T) {
	short := Typewriter{Lines: lines[:1]}
	long := Typewriter{Lines: lines}
	p := New(short, Motion{})
	run(t, p, p.Start())
	p.Swap(long)
	if !p.Done() || !slices.Equal(p.Frame(), long.Frame(long.Len())) {
		t.Errorf("after the swap: done %v, frame %q", p.Done(), plain(p.Frame()))
	}
	// Mid-play, the swap keeps the time and the tick in flight plays on.
	q := New(short, Motion{})
	cmd := q.Start()
	q.Swap(long)
	if q.Done() || cmd == nil {
		t.Fatal("a swap mid-play ended the clip")
	}
	run(t, q, cmd)
	if !q.Done() || !slices.Equal(q.Frame(), long.Frame(long.Len())) {
		t.Errorf("mid-play swap ends on %q", plain(q.Frame()))
	}
}
