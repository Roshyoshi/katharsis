package audio

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/audio/vgs"
)

const testSong = "# a test song\nCh0 t200 @2 v10 o4 l8 c e g | >c<g e c d f a f\nCh1 @0 v12 o2 c4 | c4 g4 f4 g4\n"

func TestMMLLoopsSeamlessly(t *testing.T) {
	src, err := decode("song.mml", []byte(testSong))
	if err != nil {
		t.Fatal(err)
	}
	song, _ := vgs.Compile([]byte(testSong))
	intro, total := song.IntroFrames(), song.Frames()
	if intro == 0 || total <= intro || src.Frames() != total {
		t.Fatalf("intro %d, total %d, source %d", intro, total, src.Frames())
	}
	// Three times round the loop through the stream are the decoder's
	// own uninterrupted output: the seek back is the decoder's jump.
	st := newStream(src, 0, 0, 3)
	if st.loopStart != intro || st.loopEnd != total {
		t.Fatalf("stream loop %d..%d, song %d..%d", st.loopStart, st.loopEnd, intro, total)
	}
	n := int(total + 2*(total-intro))
	want := make([]int16, n)
	vgs.NewDecoder(song).Render(want)
	// newStream has already pulled three frames for the resampler.
	for i := 3; i < n; i++ {
		f, ok := st.next()
		if !ok {
			t.Fatalf("stream ended at frame %d of %d", i, n)
		}
		if w := float32(want[i]) / 32768; f[0] != w || f[1] != w {
			t.Fatalf("frame %d: %v, decoder %v", i, f, w)
		}
	}
	if _, ok := st.next(); ok {
		t.Error("three passes played on")
	}
	// Seeking anywhere else renders the way there.
	if err := src.SeekFrame(intro + 100); err != nil {
		t.Fatal(err)
	}
	buf := make([]float32, 20)
	if n, _ := src.Read(buf); n != 10 || buf[0] != float32(want[intro+100])/32768 {
		t.Errorf("after a seek: %d frames, %v want %v", n, buf[0], float32(want[intro+100])/32768)
	}
}

func TestMMLInLibrary(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "boss"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "boss", "theme.mml"), []byte(testSong), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Library{UserDir: dir, CacheDir: t.TempDir()}
	ts := l.Resolve(PoolBoss, "")
	if len(ts) != 1 || ts[0].Title != "theme" {
		t.Fatalf("a player's .mml file is a track: %+v", ts)
	}
	m, err := l.Measure(ts[0])
	if err != nil || m.LUFS > -3 || m.LUFS < -40 {
		t.Errorf("measured %+v, %v", m, err)
	}
	if _, err := decode("bad.mml", []byte("Ch0 x\n")); err == nil {
		t.Error("bad MML decoded")
	}
}

func TestEngineLeadsAndOnce(t *testing.T) {
	l := library(t, map[string][]string{"b1": {PoolBoss}, "b2": {PoolBoss}, "b3": {PoolBoss}, "cirno": {PoolBoss}}, nil)
	for i := range l.Tracks {
		switch l.Tracks[i].ID {
		case "cirno":
			l.Tracks[i].Keys = []string{"off-by-one"}
		case "b1":
			l.Tracks[i].Once = true
		}
	}
	e := newEngine(DefaultSettings(), l, DefaultEffects())
	defer e.Close()
	// Every boss of that key opens with its lead, whatever the shuffle.
	for scene := range 6 {
		e.Cue(Cue{Pool: PoolBoss, Key: "off-by-one", Scene: scene})
		eventually(t, "the lead", func() bool { tr, _ := e.NowPlaying(); return tr.ID == "cirno" })
	}
	// Another boss shuffles the pool as before.
	seen := map[string]bool{}
	for scene := 10; scene < 40 && len(seen) < 2; scene++ {
		e.Cue(Cue{Pool: PoolBoss, Key: "heisenbug", Scene: scene})
		eventually(t, "a new track", func() bool { _, s := e.NowPlaying(); return s > 0 })
		time.Sleep(5 * time.Millisecond)
		tr, _ := e.NowPlaying()
		seen[tr.ID] = true
	}
	if len(seen) < 2 {
		t.Errorf("a key with no lead always played %v", seen)
	}
	// A song with an ending plays one pass; others Settings.Repeats.
	b1 := l.Tracks[slices.IndexFunc(l.Tracks, func(t Track) bool { return t.ID == "b1" })]
	b1.Path = l.Path(b1)
	e.cmds <- func() {
		e.tracks, e.cue = []Track{b1, b1}, Cue{Pool: PoolBoss, Key: "x"} // two, so it is not looped alone
		e.advance(0)
	}
	eventually(t, "b1", func() bool { tr, _ := e.NowPlaying(); return tr.ID == "b1" })
	done := make(chan int)
	e.cmds <- func() { done <- e.playing.st.passes }
	if p := <-done; p != 1 {
		t.Errorf("a song with an ending plays %d passes", p)
	}
}

func TestVGSManifest(t *testing.T) {
	ts := Manifest()
	per := map[string]int{}
	leads := map[string][]string{}
	for _, tr := range ts {
		for _, p := range tr.Pools {
			per[p]++
		}
		for _, k := range tr.Keys {
			leads[k] = append(leads[k], tr.ID)
			if !slices.Contains(tr.Pools, PoolBoss) && !slices.Contains(tr.Pools, PoolFinal) {
				t.Errorf("%s leads for %s but plays in no boss pool", tr.ID, k)
			}
		}
		if tr.Source != "tohovgs" {
			continue
		}
		if !tr.Touhou || tr.Format != "mml" || tr.License != "permission" || tr.From == nil ||
			!strings.HasPrefix(tr.URL, "https://raw.githubusercontent.com/suzukiplan/tohovgs-cli/cbb13ada5dd1d9244148a265946310bdd192d4b6/mml/") {
			t.Errorf("%s did not take its source's terms: %+v", tr.ID, tr)
		}
		if c := tr.Credit(); !strings.Contains(c, "arr. by SUZUKI PLAN (orig. ZUN") || !strings.HasSuffix(c, "used by permission") {
			t.Errorf("%s: toast %q", tr.ID, c)
		}
	}
	for _, p := range []string{PoolGrub, PoolUserland, PoolMid, PoolBoss, PoolFinal, PoolMerge, PoolCleared, PoolRTFM} {
		if per[p] < 5 {
			t.Errorf("pool %s has %d tracks", p, per[p])
		}
	}
	// Every named bug and the final boss open with a theme of their own.
	for _, k := range []string{"off-by-one", "heisenbug", "oom-killer", "race-condition", "undefined-behavior", "halting-problem"} {
		if len(leads[k]) == 0 {
			t.Errorf("no lead for %s", k)
		}
	}
	// The credits state the permission once, list every track, and carry
	// the ZUN notice and the decoder's license.
	page := Credits(&Library{Tracks: ts, CacheDir: t.TempDir()}, DefaultSettings())
	if n := strings.Count(page, "本楽曲の東方Projectの二次創作物への利用は"); n != 1 {
		t.Errorf("the permission is quoted %d times", n)
	}
	for _, want := range []string{TouhouNotice, "Redistributions in binary form", "ほおずきみたいに紅い魂", "SwapXFO", "CC BY 4.0"} {
		if !strings.Contains(page, want) {
			t.Errorf("credits lack %q", want)
		}
	}
	if _, err := LoadManifest("[[track]]\nid = \"x\"\nsource = \"nope\"\n"); err == nil {
		t.Error("a track with an unknown source loaded")
	}
}
