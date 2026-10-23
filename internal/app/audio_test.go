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
	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/reader"
	"katharsis/internal/screens/rtfm"
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
		want := []string{audio.PoolGrub, audio.PoolUserland, audio.PoolMerge, audio.PoolAct1, audio.PoolMerge, audio.PoolBoss1, audio.PoolCleared}
		if got := r.pools(); !slices.Equal(got, want) {
			t.Errorf("music went %v, want %v", got, want)
		}
		for _, c := range r.cues {
			if slices.Contains(audio.BossPools, c.Pool) && c.Key == "" {
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

// On the map each act's topic biomes and boss rooms draw their own pools,
// as a Touhou run climbs by stage; the Court, a Boss II exit, opens Boss
// II's.
func TestAudioClimbsByAct(t *testing.T) {
	st := &game.State{Phase: game.PhaseDoors}
	for _, b := range rules.MapBiomes() {
		bs := game.BiomeState{Kind: b.Kind, Topic: "dp"}
		if b.Kind == rules.Boss {
			bs.Boss = "off-by-one"
		}
		st.Biomes = append(st.Biomes, bs)
	}
	st.Biomes[rules.TierBossII].Boss = game.TrialBoss
	st.Biomes[rules.TierFinal].Boss = game.FinalBoss
	want := []string{audio.PoolUserland, audio.PoolAct1, audio.PoolBoss1, audio.PoolAct2, audio.PoolBoss2,
		audio.PoolAct3, audio.PoolFinal}
	for i := range st.Biomes {
		st.Biome = i
		if got := (&Model{state: st}).audioCue(); got.Pool != want[i] {
			t.Errorf("tier %d plays %q, want %q", i, got.Pool, want[i])
		}
	}
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

// Leaving a boot in flight for GRUB or RTFM leaves its music too: the
// hub keeps the biome's, GRUB over the paused boot plays GRUB's, RTFM
// from the hub RTFM's, and resuming brings the biome's back.
func TestAudioLeavesAPausedBoot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		r := withAudio(h)
		h.send(screens.StartBoot{Preset: "standard"})
		biome := r.cues[len(r.cues)-1].Pool
		steps := []struct {
			msg  tea.Msg
			want string
		}{
			{key("m"), biome},
			{key("g"), audio.PoolGrub},
			{screens.Continue{}, biome},
			{key("m"), biome},
			{key("t"), audio.PoolGrub},
		}
		for _, s := range steps {
			h.send(s.msg)
			if got := r.cues[len(r.cues)-1].Pool; got != s.want {
				t.Fatalf("after %v (top %T): music %q, want %q", s.msg, h.m.top(), got, s.want)
			}
		}
	})
}

// A practice problem opened from RTFM shuffles every track, and solving it
// takes the music home with the map.
func TestAudioRTFMPractice(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		r := withAudio(h)
		h.send(screens.StartBoot{Preset: "standard"})
		var id catalog.ID
		for _, p := range h.m.d.Catalog.All() {
			if !h.m.onOffer(p.ID) {
				id = p.ID
				break
			}
		}
		music := func() string { return r.cues[len(r.cues)-1].Pool }
		h.send(key("m"))
		h.send(key("t"))
		if _, ok := h.m.top().(*rtfm.Map); !ok || music() != audio.PoolGrub {
			t.Fatalf("t: top %T, music %q", h.m.top(), music())
		}
		h.send(screens.Upsolve{Problem: id}) // a level's enter on a practice row
		if h.m.open == nil || !h.m.open.Upsolve || music() != audio.PoolAll {
			t.Fatalf("practice: open %v, music %q, want %q", h.m.open != nil, music(), audio.PoolAll)
		}
		h.verdict = "AC"
		h.send(submitMsg{})
		if _, ok := h.m.top().(*rtfm.Map); !ok || music() != audio.PoolGrub {
			t.Fatalf("after the solve: top %T, music %q, want %q", h.m.top(), music(), audio.PoolGrub)
		}
	})
}

// Where each kind of screen sits: a stage's track inside its doors, the
// map between stages (the merge window and the route, one track for
// both), a trial's own pool, and home for anything outside a boot,
// whatever boot is under it: an upsolve, or git log over a crash.
func TestAudioPlaces(t *testing.T) {
	stage := func(p game.Phase) *game.State {
		return &game.State{Boot: 3, Phase: p, Biomes: []game.BiomeState{{Kind: rules.Fixed, Topic: "dp"}}}
	}
	trial := stage(game.PhaseDoors)
	trial.Trial = &game.TrialState{}
	crashed := stage(game.PhaseCrashed)
	tests := []struct {
		name string
		m    *Model
		want audio.Cue
	}{
		{"a door", &Model{state: stage(game.PhaseProblem)}, audio.Cue{Pool: audio.PoolUserland, Key: "dp", Scene: 300}},
		{"between doors", &Model{state: stage(game.PhaseDoors)}, audio.Cue{Pool: audio.PoolUserland, Key: "dp", Scene: 300}},
		{"the merge window", &Model{state: stage(game.PhaseMerge)}, audio.Cue{Pool: audio.PoolMerge, Scene: 300}},
		{"the route", &Model{state: stage(game.PhaseRoute)}, audio.Cue{Pool: audio.PoolMerge, Scene: 300}},
		{"a trial", &Model{state: trial}, audio.Cue{Pool: audio.PoolTrial, Scene: 300}},
		{"the crash", &Model{state: crashed}, audio.Cue{Pool: audio.PoolPanic, Scene: 300}},
		{"an upsolve from the crash", &Model{state: crashed, open: &Door{Upsolve: true}}, audio.Cue{Pool: audio.PoolGrub}},
		{"an upsolve from GRUB", &Model{open: &Door{Upsolve: true},
			stack: []screens.Screen{(*meta.Menu)(nil), newUpsolveScreen(nil, game.Door{})}}, audio.Cue{Pool: audio.PoolGrub}},
		{"RTFM practice", &Model{open: &Door{Upsolve: true},
			stack: []screens.Screen{(*rtfm.Map)(nil), newUpsolveScreen(nil, game.Door{})}}, audio.Cue{Pool: audio.PoolAll}},
		{"RTFM practice, its statement open over it", &Model{state: crashed, open: &Door{Upsolve: true},
			stack: []screens.Screen{nil, (*meta.Menu)(nil), (*rtfm.Map)(nil), newUpsolveScreen(nil, game.Door{}), nil}},
			audio.Cue{Pool: audio.PoolAll}},
		{"git log over the crash", &Model{state: crashed, stack: []screens.Screen{nil, meta.NewLog(nil, nil)}}, audio.Cue{Pool: audio.PoolGrub}},
		{"no boot", &Model{}, audio.Cue{Pool: audio.PoolGrub}},
	}
	for _, tt := range tests {
		if got := tt.m.audioCue(); got != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.name, got, tt.want)
		}
	}
}
