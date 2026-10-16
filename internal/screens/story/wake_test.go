package story

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/anim/animtest"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
)

// trialPane is the boss pane the Court's door was won on, as the game
// draws it: the header, the pool at its cap, a frame, the HP bar.
var trialPane = []string{
	"Boot #57 · Bronze · ⚔ The Court                               ⏱ 08:12 left",
	"♥ 50/50 cycles  🦆0  412 B uncommitted                       daemons strace",
	"╔══════════════════════════════════════════════════════════════════════════════╗",
	"║                           T H E   C O U R T                                  ║",
	"║   The King judges, the Queen sentences, the jury writes it on slates.        ║",
	"╚══════════════════════════════════════════════════════════════════════════════╝",
	"HP ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 12/12 · AC",
	"────────────────────────────────────────────────────────────────────────────────",
}

func wake(first bool) *Scene {
	return NewWake(Wake{From: trialPane, AC: "AC · 12/12 tests · 27:48 / 36:00",
		Examples: []string{"Bovine Shuffle", "Cow Poetry", "Haybale Feast", "Shell Game"}, First: first, Seed: 57})
}

// wakeText is every line the wake owns, the first one or a later one.
func wakeText(first bool) []string {
	var out []string
	from := 1
	if !first {
		from = 4
	}
	for n := from; n <= 6; n++ {
		key := fmt.Sprintf("ending.wake.%d", n)
		out = append(out, flavor.StoryBlock(key)...)
		if s := flavor.Story(key, 0); s != "" {
			out = append(out, s)
		}
	}
	out = append(out, "EXAMPLES", "Bovine Shuffle", "Shell Game", "rabbit(6)")
	if first {
		out = append(out, "AC · 12/12 tests · 27:48 / 36:00")
	}
	return out
}

// The first wake plays six beats and is ending.wake; esc does not leave
// it, and its footer offers no skip. A later wake plays the last three,
// records nothing and leaves on esc.
func TestWakeOnceInFull(t *testing.T) {
	s := wake(true)
	if s.Pages() != 6 || s.ID() != "ending.wake" {
		t.Fatalf("first wake: %d pages, id %q", s.Pages(), s.ID())
	}
	s.Init()
	if v := ansi.Strip(s.View(80, 24)); strings.Contains(v, "skip") {
		t.Errorf("the first wake offers a skip:\n%s", v)
	}
	for range 20 {
		if d, ok := emitted(press(s, "esc")).(Done); ok {
			if d != (Done{Scene: "ending.wake"}) {
				t.Errorf("the first wake left with %#v", d)
			}
			break
		}
	}
	if !s.Left() || s.i != 5 {
		t.Errorf("esc skipped the first wake: page %d, left %v", s.i, s.Left())
	}
	l := wake(false)
	if l.Pages() != 3 || l.ID() != "" {
		t.Fatalf("a later wake: %d pages, id %q", l.Pages(), l.ID())
	}
	l.Init()
	if msg := emitted(press(l, "esc")); msg != (Done{Skipped: true}) {
		t.Errorf("esc on a later wake emits %#v", msg)
	}
}

// Every frame fits, and by its last page the wake has said every line:
// the beats, the AC, this boot's doors as the man page's EXAMPLES.
func TestWakeFitsAndSays(t *testing.T) {
	words := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	for _, first := range []bool{true, false} {
		for _, size := range [][2]int{{80, 24}, {60, 18}, {140, 40}} {
			for _, f := range frames(wake(first), size[0], size[1]) {
				rows := strings.Split(f, "\n")
				if len(rows) != size[1] {
					t.Errorf("first %v at %v: %d rows", first, size, len(rows))
				}
				for i, r := range rows {
					if w := ansi.StringWidth(r); w > size[0] {
						t.Errorf("first %v at %v, row %d: %d cells", first, size, i, w)
					}
				}
			}
			s := wake(first)
			s.View(size[0], size[1])
			var shown strings.Builder
			for i := range s.Pages() {
				s.Seek(i, s.PageLen(i))
				shown.WriteString(ansi.Strip(s.View(size[0], size[1])) + "\n")
			}
			for _, l := range wakeText(first) {
				if !strings.Contains(words(shown.String()), words(l)) {
					t.Errorf("first %v at %v never shows %q", first, size, l)
				}
			}
		}
	}
}

// The man page's EXAMPLES counts what does not fit.
func TestWakeManPageCounts(t *testing.T) {
	var many []string
	for i := range 30 {
		many = append(many, fmt.Sprintf("Problem %d", i))
	}
	got := strings.Join(manPage(many, 80, 14), "\n")
	if !strings.Contains(got, "Problem 0") || strings.Contains(got, "Problem 29") || !strings.Contains(got, "more") {
		t.Errorf("man page:\n%s", got)
	}
	if n := len(manPage(many, 80, 14)); n > 14 {
		t.Errorf("%d rows, want 14 at most", n)
	}
}

// Reduced motion shows each page's last frame at once: the cards' page
// is a cut to black, its words under it.
func TestWakeReducedMotion(t *testing.T) {
	anim.SetMotion(anim.Motion{Reduced: true})
	defer anim.SetMotion(anim.Motion{})
	s := wake(true)
	if s.Init() != nil {
		t.Error("reduced motion still ticks")
	}
	press(s, "x")
	press(s, "x") // page 3, the cards
	v := ansi.Strip(s.View(80, 24))
	if s.i != 2 || strings.ContainsAny(v, "═║▓─") || !strings.Contains(v, "the whole pack rose up") {
		t.Errorf("page %d under reduced motion:\n%s", s.i, v)
	}
}

// TestWakeGoldens holds the ending's frames at 80x24 and 140x40: the
// court, the cards lifting and gone, the bank, the man page, the loop.
func TestWakeGoldens(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {140, 40}} {
		s := wake(true)
		s.View(size[0], size[1])
		var b strings.Builder
		for _, at := range []struct {
			page int
			t    time.Duration
		}{{0, time.Hour}, {1, time.Hour}, {2, packTime / 3}, {2, time.Hour}, {3, time.Hour}, {4, time.Hour}, {5, time.Hour}} {
			s.Seek(at.page, min(at.t, s.PageLen(at.page)))
			fmt.Fprintf(&b, "=== page %d at %v\n%s\n", at.page+1, min(at.t, s.PageLen(at.page)), ansi.Strip(s.View(size[0], size[1])))
		}
		animtest.Golden(t, fmt.Sprintf("wake_%dx%d", size[0], size[1]), b.String())
	}
}

// The Court's card says it is cursed, under its one-shot price, whole.
func TestTrialBossCard(t *testing.T) {
	br := BossRules{Price: "one shot: a fail or :q! ends the boot", OneShot: true, Cursed: true}
	for _, size := range [][2]int{{80, 24}, {60, 18}} {
		fs := frames(NewBossCard(game.TrialBoss, 1, false, br), size[0], size[1])
		last := ansi.Strip(fs[len(fs)-1])
		flat := strings.Join(strings.Fields(strings.ReplaceAll(last, "║", " ")), " ")
		for _, w := range []string{"T H E C O U R T", CursedRule, br.Price} {
			if !strings.Contains(flat, w) {
				t.Errorf("at %v the card misses %q:\n%s", size, w, last)
			}
		}
	}
}
