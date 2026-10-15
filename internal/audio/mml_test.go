package audio

import (
	"os"
	"path/filepath"
	"testing"

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
