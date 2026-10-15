package app

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/audio"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/screens/reader"
)

// recSink records what the app asks of the sound system; it plays
// nothing.
type recSink struct {
	mu      sync.Mutex
	played  []string
	cues    []audio.Cue // each change
	ducks   []bool      // each change
	musicOn bool
	track   audio.Track
	serial  uint64
}

func (r *recSink) Play(s string) { r.mu.Lock(); r.played = append(r.played, s); r.mu.Unlock() }
func (r *recSink) Cue(c audio.Cue) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.cues) == 0 || r.cues[len(r.cues)-1] != c {
		r.cues = append(r.cues, c)
	}
}
func (r *recSink) Duck(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.ducks) == 0 || r.ducks[len(r.ducks)-1] != on {
		r.ducks = append(r.ducks, on)
	}
}
func (r *recSink) ToggleMusic() bool                 { r.musicOn = !r.musicOn; return r.musicOn }
func (r *recSink) NowPlaying() (audio.Track, uint64) { return r.track, r.serial }

func (r *recSink) pools() []string {
	var ps []string
	for _, c := range r.cues {
		if len(ps) == 0 || ps[len(ps)-1] != c.Pool {
			ps = append(ps, c.Pool)
		}
	}
	return ps
}

func withAudio(h *harness) *recSink {
	r := &recSink{musicOn: true}
	h.m.d.Audio = r
	h.m.d.AudioSettings = audio.DefaultSettings()
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30}) // an Update, so the first cue lands
	return r
}

func TestAudioFollowsAFullClear(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		r := withAudio(h)
		h.m.meta.Rated = 100 // calibrated, so the clear unlocks prestige 1
		h.send(screens.StartBoot{Prestige: 0, Preset: "standard"})
		if got := h.play(); got != game.PhaseCleared {
			t.Fatalf("boot ended %q", got)
		}
		want := []string{audio.PoolGrub, audio.PoolUserland, audio.PoolMerge, audio.PoolMid, audio.PoolMerge, audio.PoolBoss, audio.PoolCleared}
		if got := r.pools(); !slices.Equal(got, want) {
			t.Errorf("music went %v, want %v", got, want)
		}
		for _, c := range r.cues {
			if c.Pool == audio.PoolBoss && c.Key == "" {
				t.Errorf("the boss cue names no boss: %+v", c)
			}
		}
		for _, sfx := range []string{"door", "submit", "ac", "pick", "commit", "boss", "cleared", "prestige"} {
			if !slices.Contains(r.played, sfx) {
				t.Errorf("never played %q; played %v", sfx, r.played)
			}
		}
		// Every door opened once and every submit sounded once.
		doors, submits := count(r.played, "door"), count(r.played, "submit")
		if doors != h.runner.opened {
			t.Errorf("%d door sounds for %d doors opened", doors, h.runner.opened)
		}
		// Every submit was accepted; the last boss's sounds "cleared", not "ac".
		if acs := count(r.played, "ac") + count(r.played, "cleared"); submits == 0 || submits != acs {
			t.Errorf("%d submit sounds for %d accepted submits: %v", submits, acs, r.played)
		}
		if !slices.Equal(r.ducks[:2], []bool{false, true}) || r.ducks[len(r.ducks)-1] {
			t.Errorf("duck went %v; want it on while judging and off after", r.ducks)
		}
		if i, j := slices.Index(r.played, "cleared"), slices.Index(r.played, "prestige"); i < 0 || j < i {
			t.Errorf("prestige (%d) did not follow cleared (%d): %v", j, i, r.played)
		}
	})
}

func count(xs []string, x string) int {
	n := 0
	for _, v := range xs {
		if v == x {
			n++
		}
	}
	return n
}

func TestAudioCrash(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		r := withAudio(h)
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(game.Quack{})
		h.verdict = "WA"
		lives := h.m.state.Cycles
		for h.m.state.Phase == game.PhaseProblem {
			h.send(submitMsg{})
		}
		if got := count(r.played, "life"); got != lives-1 {
			t.Errorf("%d life sounds for %d lives lost before the crash", got, lives-1)
		}
		for _, sfx := range []string{"quack", "wa", "crash"} {
			if !slices.Contains(r.played, sfx) {
				t.Errorf("never played %q; played %v", sfx, r.played)
			}
		}
		if c := r.cues[len(r.cues)-1]; c.Pool != audio.PoolPanic {
			t.Errorf("the crash cued %+v, want the panic pool", c)
		}
		h.send(screens.Quit{})
	})
}

func TestAudioTimerWarning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		r := withAudio(h)
		h.send(screens.StartBoot{Preset: "standard"})
		a := h.enterDoor()
		h.clock.t = a.Deadline.Add(-61 * time.Second)
		h.send(tickMsg(h.clock.t))
		if count(r.played, "warn") != 0 {
			t.Fatal("warned with more than a minute left")
		}
		h.clock.t = a.Deadline.Add(-59 * time.Second)
		h.send(tickMsg(h.clock.t))
		h.clock.t = a.Deadline.Add(-30 * time.Second)
		h.send(tickMsg(h.clock.t))
		if got := count(r.played, "warn"); got != 1 {
			t.Errorf("%d warnings in the last minute, want 1", got)
		}
	})
}

func TestMusicKeyAndToast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		r := withAudio(h)
		h.send(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
		if r.musicOn {
			t.Fatal("ctrl+t did not turn the music off")
		}
		if v := h.m.View().Content; !strings.Contains(v, "music off") {
			t.Errorf("no toast for the toggle:\n%s", v)
		}
		r.track = audio.Track{Title: "Bad Apple!!", Artist: "someone", Original: "ZUN, Touhou 4", License: "CC BY 4.0"}
		r.serial = 1
		h.send(tickMsg(h.clock.t))
		if v := h.m.View().Content; !strings.Contains(v, "♪ Bad Apple!!, arr. by someone (orig. ZUN, Touhou 4), CC BY 4.0") {
			t.Errorf("no credit toast:\n%s", v)
		}
		h.clock.t = h.clock.t.Add(7 * time.Second)
		if v := h.m.View().Content; strings.Contains(v, "Bad Apple") {
			t.Error("the toast outlived its 6 s")
		}
	})
}

func TestCreditsFromGrub(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		withAudio(h)
		h.m.d.Credits = func() string { return "# Credits\n\nZUN notice here\n" }
		h.send(screens.OpenCredits{})
		if _, ok := h.m.top().(*reader.Model); !ok {
			t.Fatalf("credits opened %T, want the reader", h.m.top())
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "ZUN notice here") {
			t.Errorf("credits page:\n%s", v)
		}
	})
}
