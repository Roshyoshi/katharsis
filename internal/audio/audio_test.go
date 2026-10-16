package audio

import (
	"encoding/binary"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// No test here opens an output: engines are built with newEngine, which
// has none, and the mixer is rendered into a slice.

// wav encodes interleaved stereo float frames as 16-bit PCM.
func wav(rate int, frames []float32) []byte {
	data := make([]byte, 2*len(frames))
	for i, v := range frames {
		binary.LittleEndian.PutUint16(data[2*i:], uint16(int16(math.Round(float64(v)*32767))))
	}
	b := []byte("RIFF\x00\x00\x00\x00WAVEfmt ")
	b = binary.LittleEndian.AppendUint32(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 1)
	b = binary.LittleEndian.AppendUint16(b, 2)
	b = binary.LittleEndian.AppendUint32(b, uint32(rate))
	b = binary.LittleEndian.AppendUint32(b, uint32(rate*4))
	b = binary.LittleEndian.AppendUint16(b, 4)
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(data)))
	b = append(b, data...)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	return b
}

// sine is seconds of a stereo sine at amp, at rate.
func sine(rate int, hz, amp, seconds float64) []float32 {
	n := int(seconds * float64(rate))
	out := make([]float32, 2*n)
	for i := range n {
		v := float32(amp * math.Sin(2*math.Pi*hz*float64(i)/float64(rate)))
		out[2*i], out[2*i+1] = v, v
	}
	return out
}

// EBU Tech 3341 case 1: a stereo 1 kHz sine at -23 dBFS reads -23 LUFS.
func TestLoudnessReference(t *testing.T) {
	for _, rate := range []int{44100, 48000} {
		m := NewMeter(rate, 2)
		m.Add(sine(rate, 1000, dbToGain(-23), 20))
		if got := m.Integrated(); math.Abs(got+23) > 0.1 {
			t.Errorf("%d Hz: integrated %.2f LUFS, want -23.0 ± 0.1", rate, got)
		}
	}
	// EBU Tech 3341 case 3: -36 dBFS for 10 s, -23 for 60 s, -36 for 10
	// s reads -23 LUFS: the relative gate drops the quiet parts.
	m := NewMeter(48000, 2)
	m.Add(sine(48000, 1000, dbToGain(-36), 10))
	m.Add(sine(48000, 1000, dbToGain(-23), 60))
	m.Add(sine(48000, 1000, dbToGain(-36), 10))
	if got := m.Integrated(); math.Abs(got+23) > 0.1 {
		t.Errorf("gated: integrated %.2f LUFS, want -23.0 ± 0.1", got)
	}
}

func TestEffects(t *testing.T) {
	fx := DefaultEffects()
	for _, want := range []string{"ac", "wa", "crash", "commit", "boss", "cleared",
		"door", "submit", "life", "warn", "buy", "reroll", "pick", "prestige", "quack"} {
		if _, ok := fx[want]; !ok {
			t.Errorf("sfx.toml has no %q", want)
		}
	}
	set := DefaultSettings()
	sounds := Render(fx, set.SFXLUFS)
	for _, name := range Names(sounds) {
		s, e := sounds[name], fx[name]
		dur := float64(len(s.PCM)) / Rate
		m := NewMeter(Rate, 2)
		st := make([]float32, 2*len(s.PCM))
		var dc float64
		for i, v := range s.PCM {
			st[2*i], st[2*i+1] = v, v
			dc += float64(v)
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("%s: sample %d is %v", name, i, v)
			}
		}
		m.Add(st)
		loud, peak := m.MaxMomentary(), m.Peak()
		t.Logf("%-8s %5.2f s  peak %6.1f dBFS  rms %6.1f dBFS  loudest 400 ms %6.1f LUFS (target %5.1f)",
			name, dur, DB(peak), DB(RMS(s.PCM)), loud, set.SFXLUFS+e.Level)
		if dur > 1.3 {
			t.Errorf("%s lasts %.2f s; effects stay under 1.3 s", name, dur)
		}
		if peak > peakCap+1e-6 {
			t.Errorf("%s peaks at %.2f dBFS, over -1", name, DB(peak))
		}
		// Normalized to its target unless the peak cap held it under.
		if d := loud - (set.SFXLUFS + e.Level); d > 0.2 || d < -0.2 && peak < peakCap-1e-3 {
			t.Errorf("%s is %.1f LU off its target", name, d)
		}
		// A click is a jump at either end.
		if a, z := math.Abs(float64(s.PCM[0])), math.Abs(float64(s.PCM[len(s.PCM)-1])); a > 0.01 || z > 0.01 {
			t.Errorf("%s starts at %.3f and ends at %.3f; want silence at both ends", name, a, z)
		}
		if dc /= float64(len(s.PCM)); math.Abs(dc) > 0.01 {
			t.Errorf("%s has DC offset %.4f", name, dc)
		}
	}
}

func TestEffectsRejectBadData(t *testing.T) {
	for _, bad := range []string{
		"[x]\nchannel = \"ui\"\n",
		"[x]\nchannel = \"loud\"\n[[x.tone]]\nwave = \"sine\"\nfreq = 440\ndur = 0.1\n",
		"[x]\nchannel = \"ui\"\n[[x.tone]]\nwave = \"fm\"\nfreq = 440\ndur = 0.1\n",
		"[x]\nchannel = \"ui\"\n[[x.tone]]\nwave = \"sine\"\nfreq = 30000\ndur = 0.1\n",
		"[x]\nchannel = \"ui\"\n[[x.tone]]\nwave = \"sine\"\nfreq = 440\ndur = 0.1\nattack = 0.2\n",
		"[x]\nchannel = \"ui\"\n[[x.tone]]\nwave = \"sine\"\nfreq = 440\ndur = 0.1\narp = [0, 4]\n",
	} {
		if _, err := LoadEffects(bad); err == nil {
			t.Errorf("LoadEffects accepted:\n%s", bad)
		}
	}
}

func TestLimit(t *testing.T) {
	for _, x := range []float32{0, 0.5, 0.9, 1, 2, 10, 1e6} {
		for _, s := range []float32{1, -1} {
			y := limit(s * x)
			if math.Abs(float64(y)) >= 1 || (x <= 0.9 && y != s*x) || y*s < 0 {
				t.Errorf("limit(%v) = %v", s*x, y)
			}
		}
	}
	// Monotonic, so it never folds a louder input under a quieter one.
	prev := float32(-2)
	for x := float32(-3); x < 3; x += 0.001 {
		if y := limit(x); y < prev {
			t.Fatalf("limit is not monotonic at %v", x)
		} else {
			prev = y
		}
	}
}

func TestBag(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	ids := []string{"a", "b", "c", "d", "e"}
	var b bag
	last := ""
	for round := range 200 {
		seen := map[string]bool{}
		for range ids {
			id := b.next(ids, rng)
			if seen[id] {
				t.Fatalf("round %d dealt %s twice", round, id)
			}
			if id == last {
				t.Fatalf("round %d repeated %s back to back", round, id)
			}
			seen[id], last = true, id
		}
	}
	if got := b.next([]string{"z"}, rng); got != "z" {
		t.Errorf("a changed set dealt %q, want z", got)
	}
	if got := b.next([]string{"z"}, rng); got != "z" {
		t.Errorf("a pool of one dealt %q", got)
	}
	if got := b.next(nil, rng); got != "" {
		t.Errorf("an empty pool dealt %q", got)
	}
}

// A lead that is the last track of the pool's round is not dealt again
// right after it played.
func TestBagAvoidsLeadAtRoundEnd(t *testing.T) {
	ids := []string{"a", "b", "c"}
	for seed := range uint64(50) {
		rng := rand.New(rand.NewPCG(seed, 2))
		var b bag
		b.next(ids, rng)
		b.next(ids, rng)
		lead := b.order[2]
		b.avoid(lead, rng)
		if got := b.next(ids, rng); got == lead {
			t.Fatalf("seed %d: %s played as the lead, then again from the pool", seed, lead)
		}
	}
}

func TestStreamLoops(t *testing.T) {
	// 0.5 s at 44.1 kHz, played 3 times: exactly 1.5 s out.
	src, err := newWAV(wav(Rate, sine(Rate, 441, 0.5, 0.5)))
	if err != nil {
		t.Fatal(err)
	}
	s := newStream(src, 0, 0, 3)
	if got, want := s.Remaining(), int64(1.5*Rate); got != want {
		t.Errorf("Remaining before play = %d, want %d", got, want)
	}
	total := 0
	buf := make([]float32, 2*1000)
	var prev float32
	maxJump := 0.0
	for {
		n := s.Read(buf)
		for i := range n {
			maxJump = math.Max(maxJump, math.Abs(float64(buf[2*i]-prev)))
			prev = buf[2*i]
		}
		total += n
		if n < 1000 {
			break
		}
	}
	if total != int(1.5*Rate) {
		t.Errorf("3 passes of 0.5 s gave %d frames, want %d", total, int(1.5*Rate))
	}
	// 441 Hz fits 220.5 cycles in 0.5 s, so this loop is not seamless by
	// design; a sine's steepest step is 2π·441/44100·0.5 ≈ 0.031. The
	// seam jumps from the last sample back to 0, still small.
	if maxJump > 0.04 {
		t.Errorf("largest step %.3f; the seam added a click", maxJump)
	}
}

// A 48 kHz file comes out at 44.1 kHz with its length and pitch kept.
func TestStreamResamples(t *testing.T) {
	src, err := newWAV(wav(48000, sine(48000, 1000, 0.5, 1)))
	if err != nil {
		t.Fatal(err)
	}
	s := newStream(src, 0, 0, 1)
	var out []float32
	buf := make([]float32, 2*4096)
	for {
		n := s.Read(buf)
		out = append(out, buf[:2*n]...)
		if n < 4096 {
			break
		}
	}
	if n := len(out) / 2; n < Rate-2 || n > Rate+2 {
		t.Errorf("1 s at 48 kHz became %d frames, want %d ± 2", n, Rate)
	}
	// Zero crossings over the middle 0.5 s: 1000 Hz has 1000 of them.
	crossings := 0
	for i := Rate / 4; i < 3*Rate/4; i++ {
		if (out[2*i] < 0) != (out[2*i+2] < 0) {
			crossings++
		}
	}
	if crossings < 995 || crossings > 1005 {
		t.Errorf("%d zero crossings in 0.5 s, want 1000 for 1 kHz", crossings)
	}
}

// A WAV header with no sample rate is a broken file, not a stream that
// holds its first sample forever.
func TestWAVRejectsZeroRate(t *testing.T) {
	if _, err := newWAV(wav(0, sine(Rate, 440, 0.5, 0.1))); err == nil {
		t.Error("a 0 Hz WAV decoded")
	}
}

func TestStreamLoopPoints(t *testing.T) {
	// An intro of 0.25 s, then a 0.5 s loop body played twice.
	src, err := newWAV(wav(Rate, sine(Rate, 400, 0.5, 0.75)))
	if err != nil {
		t.Fatal(err)
	}
	s := newStream(src, 0.25, 0.75, 2)
	n := 0
	buf := make([]float32, 2*512)
	for {
		k := s.Read(buf)
		n += k
		if k < 512 {
			break
		}
	}
	if want := int(0.25*Rate) + 2*int(0.5*Rate); n != want {
		t.Errorf("intro + 2 loops = %d frames, want %d", n, want)
	}
}

func TestMixerCrossfadeAndDuck(t *testing.T) {
	set := DefaultSettings()
	set.MusicVolume, set.DuckGain = 1, 0.5
	ending := make(chan uint64, 4)
	m := NewMixer(set, ending)
	voice := func(id uint64, amp float64) *musicVoice {
		src, err := newWAV(wav(Rate, constant(amp, 4)))
		if err != nil {
			t.Fatal(err)
		}
		return &musicVoice{id: id, st: newStream(src, 0, 0, 0), gain: 1}
	}
	out := make([]float32, 2*Rate/10)
	m.startMusic(voice(1, 0.5), 0, 0)
	m.Render(out)
	if got := out[len(out)-2]; math.Abs(float64(got)-0.5) > 1e-3 {
		t.Fatalf("one voice at full gain renders %v, want 0.5", got)
	}
	// An equal-power crossfade of two equal, correlated signals peaks at
	// sin+cos = √2 at the middle: the curve is right when it does.
	m.startMusic(voice(2, 0.5), 1, 1)
	var top float64
	for range 12 {
		m.Render(out)
		for i := 0; i < len(out); i += 2 {
			top = math.Max(top, float64(out[i]))
		}
	}
	if math.Abs(top-0.5*math.Sqrt2) > 0.01 {
		t.Errorf("crossfade peak %.3f, want %.3f (equal power)", top, 0.5*math.Sqrt2)
	}
	if len(m.music) != 1 || m.music[0].id != 2 {
		t.Errorf("after the fade %d voices play; want only voice 2", len(m.music))
	}
	m.setDuck(true)
	for range 5 { // 0.5 s, past the 150 ms attack
		m.Render(out)
	}
	if got := out[len(out)-2]; math.Abs(float64(got)-0.25) > 1e-3 {
		t.Errorf("ducked to %v, want 0.25 (0.5 × duck 0.5)", got)
	}
	m.setDuck(false)
	for range 10 { // 1 s, past the 800 ms release
		m.Render(out)
	}
	if got := out[len(out)-2]; math.Abs(float64(got)-0.5) > 1e-3 {
		t.Errorf("released to %v, want 0.5", got)
	}
}

func constant(amp, seconds float64) []float32 {
	out := make([]float32, 2*int(seconds*Rate))
	for i := range out {
		out[i] = float32(amp)
	}
	return out
}

func TestMixerEndingSignal(t *testing.T) {
	set := DefaultSettings()
	ending := make(chan uint64, 4)
	m := NewMixer(set, ending)
	src, err := newWAV(wav(Rate, constant(0.1, 3)))
	if err != nil {
		t.Fatal(err)
	}
	fadeAt := int64(1.5 * Rate)
	m.startMusic(&musicVoice{id: 7, st: newStream(src, 0, 0, 1), gain: 1, fadeAt: fadeAt}, 0, 0)
	out := make([]float32, 2*Rate/10)
	frames := 0
	for frames < 3*Rate {
		m.Render(out)
		frames += len(out) / 2
		select {
		case id := <-ending:
			if id != 7 {
				t.Fatalf("ending for %d", id)
			}
			// Told within one buffer after 1.5 s was left.
			if left := 3*Rate - frames; left > int(fadeAt) || left < int(fadeAt)-len(out)/2 {
				t.Errorf("told with %d frames left, want about %d", left, fadeAt)
			}
			return
		default:
		}
	}
	t.Fatal("never told the voice was ending")
}

func TestMixerJinglesQueue(t *testing.T) {
	m := NewMixer(DefaultSettings(), make(chan uint64, 1))
	m.sfxGain = 1
	a := Sound{Channel: "jingle", PCM: constant(0.1, 0.05)[:2205]}
	b := Sound{Channel: "jingle", PCM: constant(0.2, 0.05)[:2205]}
	ui := Sound{Channel: "ui", PCM: constant(0.3, 0.05)[:100]}
	m.playSFX(a)
	m.playSFX(b)
	m.playSFX(ui)
	out := make([]float32, 2*4410)
	m.Render(out)
	// ui overlaps the first jingle; the second follows the first.
	if got := out[0]; math.Abs(float64(got)-0.4) > 1e-6 {
		t.Errorf("frame 0 = %v, want jingle a + ui = 0.4", got)
	}
	if got := out[2*500]; math.Abs(float64(got)-0.1) > 1e-6 {
		t.Errorf("frame 500 = %v, want jingle a alone", got)
	}
	if got := out[2*3000]; math.Abs(float64(got)-0.2) > 1e-6 {
		t.Errorf("frame 3000 = %v, want jingle b after a", got)
	}
	if m.busy() {
		t.Error("voices left after they finished")
	}
	m.Render(out)
	if m.idleFrames() != 4410 {
		t.Errorf("idle %d frames, want 4410", m.idleFrames())
	}
}

// Every effect at once over loud music at full volumes stays under full
// scale: the limiter holds the mix.
func TestMixerNeverClips(t *testing.T) {
	set := DefaultSettings()
	set.MusicVolume, set.SFXVolume = 1, 1
	m := NewMixer(set, make(chan uint64, 1))
	src, err := newWAV(wav(Rate, sine(Rate, 220, 0.99, 2)))
	if err != nil {
		t.Fatal(err)
	}
	m.startMusic(&musicVoice{id: 1, st: newStream(src, 0, 0, 0), gain: 2}, 0, 0)
	for _, s := range Render(DefaultEffects(), set.SFXLUFS) {
		s.Channel = "ui"
		m.playSFX(s)
	}
	out := make([]float32, 2*Rate)
	m.Render(out)
	for i, v := range out {
		if math.Abs(float64(v)) >= 1 {
			t.Fatalf("sample %d = %v reaches full scale", i, v)
		}
	}
}

func TestSettings(t *testing.T) {
	if err := DefaultSettings().Validate(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	s, err := LoadSettings(filepath.Join(dir, "missing.toml"))
	if err != nil || s != DefaultSettings() {
		t.Errorf("a missing file = %+v, %v; want the defaults", s, err)
	}
	p := filepath.Join(dir, "audio.toml")
	write(t, p, "music_volume = 0.3\ncrossfade = \"5s\"\n")
	s, err = LoadSettings(p)
	if err != nil || s.MusicVolume != 0.3 || s.Crossfade.Duration != 5*time.Second || s.SFXVolume != DefaultSettings().SFXVolume {
		t.Errorf("LoadSettings = %+v, %v", s, err)
	}
	for _, bad := range []string{"music_volum = 0.3\n", "music_volume = 2\n", "panic = \"loud\"\n", "user_music = \"merge\"\n", "repeats = 0\n"} {
		write(t, p, bad)
		if _, err := LoadSettings(p); err == nil {
			t.Errorf("LoadSettings accepted %q", bad)
		}
	}
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManifest(t *testing.T) {
	ts := Manifest()
	urls := map[string]bool{}
	for _, tr := range ts {
		if urls[tr.URL] {
			t.Errorf("%s: URL listed twice", tr.ID)
		}
		urls[tr.URL] = true
		if strings.Contains(tr.License, "ND") {
			t.Errorf("%s: an ND license", tr.ID)
		}
		if c := tr.Credit(); !strings.HasPrefix(c, "♪ "+tr.Title) || !strings.Contains(c, tr.License) {
			t.Errorf("%s: credit %q", tr.ID, c)
		}
		if tr.Touhou && !strings.Contains(tr.Credit(), "ZUN") {
			t.Errorf("%s: a Touhou arrangement's credit must name ZUN: %q", tr.ID, tr.Credit())
		}
	}
	bad := `[[track]]
id = "x"
title = "X"
artist = "Y"
license = "CC BY-ND 4.0"
license_url = "https://creativecommons.org/licenses/by-nd/4.0/"
license_quote = "q"
page = "https://example.org/"
url = "https://example.org/x.mp3"
sha256 = "0000000000000000000000000000000000000000000000000000000000000000"
size = 1
format = "mp3"
pools = ["grub"]
`
	if _, err := LoadManifest(bad); err == nil {
		t.Error("LoadManifest accepted an ND license")
	}
	if _, err := LoadManifest(strings.Replace(strings.Replace(bad, "BY-ND", "BY", 1), "by-nd", "by", 1)); err != nil {
		t.Errorf("LoadManifest rejected a good entry: %v", err)
	}
}

func TestCredit(t *testing.T) {
	tr := Track{Title: "Necrofantasia", Artist: "Someone", Original: "ZUN, Touhou 7", License: "CC BY 4.0"}
	if got, want := tr.Credit(), "♪ Necrofantasia, arr. by Someone (orig. ZUN, Touhou 7), CC BY 4.0"; got != want {
		t.Errorf("Credit = %q, want %q", got, want)
	}
	tr = Track{Title: "Loop", Artist: "Someone", License: "CC0 1.0"}
	if got, want := tr.Credit(), "♪ Loop, by Someone, CC0 1.0"; got != want {
		t.Errorf("Credit = %q, want %q", got, want)
	}
}

// library builds a library over temp dirs with WAV tracks in it.
func library(t *testing.T, manifest map[string][]string, user map[string][]string) *Library {
	t.Helper()
	dir := t.TempDir()
	l := &Library{CacheDir: filepath.Join(dir, "cache"), UserDir: filepath.Join(dir, "music"), Mode: "override"}
	ids := make([]string, 0, len(manifest))
	for id := range manifest {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for i, id := range ids {
		data := wav(Rate, sine(Rate, 200+50*float64(i), 0.3, 1))
		tr := Track{ID: id, Title: id, Artist: "a", Format: "wav", Size: int64(len(data)), Pools: manifest[id],
			SHA256: strings.Repeat("ab", 32)}
		l.Tracks = append(l.Tracks, tr)
		writeBytes(t, l.Path(tr), data)
	}
	for folder, files := range user {
		for i, f := range files {
			writeBytes(t, filepath.Join(l.UserDir, folder, f), wav(Rate, sine(Rate, 500+50*float64(i), 0.3, 1)))
		}
	}
	return l
}

func writeBytes(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func ids(ts []Track) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

func TestLibraryResolve(t *testing.T) {
	l := library(t,
		map[string][]string{"g1": {PoolGrub}, "m1": {PoolAct1, PoolUserland}, "m2": {PoolAct1}},
		map[string][]string{"act1": {"mine.wav", "notes.txt"}, "dp": {"cache.wav"}})
	if got := ids(l.Resolve(PoolGrub, "")); !slices.Equal(got, []string{"g1"}) {
		t.Errorf("grub = %v", got)
	}
	if got := ids(l.Resolve(PoolAct1, "")); !slices.Equal(got, []string{"user:act1/mine.wav"}) {
		t.Errorf("act1 with a user folder (override) = %v", got)
	}
	if got := ids(l.Resolve(PoolAct1, "dp")); !slices.Equal(got, []string{"user:dp/cache.wav"}) {
		t.Errorf("act1/dp = %v; the key folder comes first", got)
	}
	if got := ids(l.Resolve(PoolAct1, "graphs")); !slices.Equal(got, []string{"user:act1/mine.wav"}) {
		t.Errorf("act1/graphs = %v; an empty key folder falls back to the pool's", got)
	}
	l.Mode = "extend"
	if got := ids(l.Resolve(PoolAct1, "")); !slices.Equal(got, []string{"m1", "m2", "user:act1/mine.wav"}) {
		t.Errorf("act1 extended = %v", got)
	}
	if got := ids(l.Resolve(PoolBoss1, "")); len(got) != 0 {
		t.Errorf("boss = %v, want nothing", got)
	}
	// A partial download (wrong size) is not there.
	if err := os.Truncate(l.Path(l.Tracks[0]), 10); err != nil {
		t.Fatal(err)
	}
	if got := l.Resolve(PoolGrub, ""); len(got) != 0 {
		t.Errorf("a truncated file resolved: %v", ids(got))
	}
	if got := l.Resolve(PoolGrub, "../.."); len(got) != 0 {
		t.Errorf("a key with a path resolved: %v", ids(got))
	}
}

// eventually polls cond for up to 2 s.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); !cond(); {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestEngineCues(t *testing.T) {
	l := library(t, map[string][]string{"u1": {PoolUserland}, "u2": {PoolUserland}, "b1": {PoolBoss1}}, nil)
	set := DefaultSettings()
	e := newEngine(set, l, DefaultEffects())
	defer e.Close()

	e.Cue(Cue{Pool: PoolUserland, Scene: 0})
	eventually(t, "a userland track", func() bool { tr, s := e.NowPlaying(); return s == 1 && (tr.ID == "u1" || tr.ID == "u2") })
	first, _ := e.NowPlaying()

	// The same cue again, or the key moving inside the stretch, carries on.
	e.Cue(Cue{Pool: PoolUserland, Key: "mixed", Scene: 0})
	time.Sleep(50 * time.Millisecond)
	if tr, s := e.NowPlaying(); s != 1 || tr.ID != first.ID {
		t.Errorf("a key change restarted the music: %s #%d", tr.ID, s)
	}

	e.Cue(Cue{Pool: PoolBoss1, Key: "heisenbug", Scene: 2})
	eventually(t, "the boss track", func() bool { tr, _ := e.NowPlaying(); return tr.ID == "b1" })

	// The kernel panic is silent by default: the music fades out.
	e.Cue(Cue{Pool: PoolPanic, Scene: 2})
	eventually(t, "silence", func() bool {
		e.mix.Render(make([]float32, 2*Rate))
		return !e.mix.busy()
	})

	if e.ToggleMusic() == set.Music {
		t.Fatal("ToggleMusic did not flip")
	}
	e.Cue(Cue{Pool: PoolUserland, Scene: 3})
	time.Sleep(50 * time.Millisecond)
	if _, s := e.NowPlaying(); s != 2 {
		t.Errorf("music off still started a track (serial %d)", s)
	}
	e.ToggleMusic()
	eventually(t, "music back on", func() bool { _, s := e.NowPlaying(); return s == 3 })
}

func TestEngineRotatesPool(t *testing.T) {
	l := library(t, map[string][]string{"a": {PoolAct1}, "b": {PoolAct1}, "c": {PoolAct1}}, nil)
	set := DefaultSettings()
	set.Repeats = 1
	set.TrackFade = Duration{200 * time.Millisecond}
	e := newEngine(set, l, DefaultEffects())
	defer e.Close()
	e.Cue(Cue{Pool: PoolAct1, Scene: 1})
	var played []string
	buf := make([]float32, 2*Rate/20)
	for len(played) < 6 {
		tr, s := e.NowPlaying()
		if int(s) > len(played) {
			played = append(played, tr.ID)
		}
		e.mix.Render(buf)
		time.Sleep(time.Millisecond)
	}
	for i := 1; i < len(played); i++ {
		if played[i] == played[i-1] {
			t.Errorf("played %v: a repeat back to back", played)
		}
	}
	first := slices.Clone(played[:3])
	slices.Sort(first)
	if !slices.Equal(first, []string{"a", "b", "c"}) {
		t.Errorf("first round %v is not every track once", played[:3])
	}
}

func TestEngineEffects(t *testing.T) {
	set := DefaultSettings()
	e := newEngine(set, nil, DefaultEffects())
	defer e.Close()
	e.Play("ac")
	e.Play("no-such-effect")
	if !e.mix.busy() {
		t.Error("Play(ac) queued nothing")
	}
	set.SFX = false
	quiet := newEngine(set, nil, DefaultEffects())
	defer quiet.Close()
	quiet.Play("ac")
	if quiet.mix.busy() {
		t.Error("effects off still played")
	}
}

func TestEngineSkipsBadFiles(t *testing.T) {
	l := library(t, map[string][]string{"good": {PoolGrub}}, map[string][]string{"grub": {"broken.wav"}})
	l.Mode = "extend"
	writeBytes(t, filepath.Join(l.UserDir, "grub", "broken.wav"), []byte("RIFF....WAVEjunk"))
	e := newEngine(DefaultSettings(), l, DefaultEffects())
	defer e.Close()
	for i := range 4 {
		e.Cue(Cue{Pool: PoolGrub, Scene: i + 1})
		eventually(t, "the good track", func() bool { tr, _ := e.NowPlaying(); return tr.ID == "good" })
	}
}

// A cue can start a track while the device opens. When the device then
// fails, the engine stays silent: housekeeping does not reopen the track
// the failure dropped, nor announce it as playing.
func TestEngineStaysSilentWithoutDevice(t *testing.T) {
	l := library(t, map[string][]string{"g1": {PoolGrub}}, nil)
	e := newEngine(DefaultSettings(), l, DefaultEffects())
	defer e.Close()
	e.Cue(Cue{Pool: PoolGrub})
	eventually(t, "a track", func() bool { _, s := e.NowPlaying(); return s == 1 })
	e.noDevice()
	done := make(chan struct{})
	e.cmds <- func() { e.housekeep(); close(done) }
	<-done
	if _, s := e.NowPlaying(); s != 1 || e.mix.busy() {
		t.Errorf("after the device failed: track serial %d (want 1), mixer busy %v", s, e.mix.busy())
	}
}

func TestNull(t *testing.T) {
	var s Sink = Null{}
	s.Play("ac")
	s.Cue(Cue{Pool: PoolGrub})
	s.Duck(true)
	if s.ToggleMusic() {
		t.Error("Null music is on")
	}
	if _, n := s.NowPlaying(); n != 0 {
		t.Error("Null is playing")
	}
}
