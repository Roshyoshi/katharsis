package audio

import (
	"encoding/binary"
	"math"
	"slices"
	"sync"
)

// Mixer renders the music and effect buses into interleaved stereo
// float32 at Rate. The device pulls Read from its own goroutine; the
// engine changes voices under mu. It does no I/O and allocates nothing in
// steady state, so it renders the same in tests (by hand) as on the
// device.
type Mixer struct {
	mu sync.Mutex

	music   []*musicVoice // the current track last; earlier ones fade out
	ui      []*sfxVoice   // overlapping effects
	jingles []*sfxVoice   // queued; only the first plays

	musicGain, sfxGain float64 // bus gains from the volumes

	// The duck: music multiplier, its level while ducked, and per-frame
	// steps down (attack) and back up (release).
	duck, duckLevel        float64
	duckAttack, duckReturn float64
	ducked                 bool

	tmp, bus []float32
	idle     int // frames rendered since anything last played

	// ending is told, once per voice, when a music voice that does not
	// loop forever comes within its fade of the end, so the engine can
	// crossfade the next track in on time. The render never blocks on it.
	ending chan<- uint64
}

type musicVoice struct {
	id     uint64
	st     *stream
	gain   float64 // loudness normalization
	fade   fade
	fadeAt int64 // output frames before the end when ending fires
	told   bool
}

type sfxVoice struct {
	pcm []float32
	at  int
}

// fade is an equal-power fade: gain sin(pos·π/2), pos moving by step per
// frame towards 1 (up) or 0.
type fade struct {
	pos, step float64
	up        bool
}

func (f *fade) gain() float64 { return math.Sin(f.pos * math.Pi / 2) }

func (f *fade) advance() {
	if f.up {
		f.pos = math.Min(1, f.pos+f.step)
	} else {
		f.pos = math.Max(0, f.pos-f.step)
	}
}

// fadeStep is the per-frame step of a fade lasting seconds.
func fadeStep(seconds float64) float64 {
	if seconds <= 0 {
		return 1
	}
	return 1 / (seconds * Rate)
}

// NewMixer starts both buses at the settings' volumes.
func NewMixer(s Settings, ending chan<- uint64) *Mixer {
	return &Mixer{
		musicGain:  gainOf(s.MusicVolume),
		sfxGain:    gainOf(s.SFXVolume),
		duck:       1,
		duckLevel:  s.DuckGain,
		duckAttack: (1 - s.DuckGain) / math.Max(1, s.DuckAttack.Seconds()*Rate),
		duckReturn: (1 - s.DuckGain) / math.Max(1, s.DuckRelease.Seconds()*Rate),
		ending:     ending,
	}
}

// Read implements io.Reader for the device: float32 little endian. It
// always fills p, with silence when nothing plays.
func (m *Mixer) Read(p []byte) (int, error) {
	frames := len(p) / 8
	out := grow(&m.bus, frames*2) // bus is free outside Render
	m.Render(out)
	for i, v := range out {
		binary.LittleEndian.PutUint32(p[4*i:], math.Float32bits(v))
	}
	return frames * 8, nil
}

func grow(b *[]float32, n int) []float32 {
	if cap(*b) < n {
		*b = make([]float32, n)
	}
	return (*b)[:n]
}

// Render mixes the next len(out)/2 frames into out.
func (m *Mixer) Render(out []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(out)
	frames := len(out) / 2
	if len(m.music) == 0 && len(m.ui) == 0 && len(m.jingles) == 0 {
		m.idle += frames
		if !m.ducked {
			m.duck = 1
		}
		return
	}
	m.idle = 0
	m.renderMusic(out, frames)
	m.renderSFX(out)
	for i, v := range out {
		out[i] = limit(v)
	}
}

func (m *Mixer) renderMusic(out []float32, frames int) {
	if len(m.music) == 0 {
		return
	}
	// The duck moves once per frame, whatever the number of voices.
	bus := grow(&m.tmp, frames*3)[frames*2:]
	for i := range bus {
		if m.ducked {
			m.duck = math.Max(m.duckLevel, m.duck-m.duckAttack)
		} else {
			m.duck = math.Min(1, m.duck+m.duckReturn)
		}
		bus[i] = float32(m.duck * m.musicGain)
	}
	buf := m.tmp[:frames*2]
	keep := m.music[:0]
	for _, v := range m.music {
		n := v.st.Read(buf)
		for i := range n {
			g := float32(v.gain*v.fade.gain()) * bus[i]
			v.fade.advance()
			out[2*i] += buf[2*i] * g
			out[2*i+1] += buf[2*i+1] * g
		}
		if !v.told && (n < frames || v.st.passes > 0 && v.st.Remaining() <= v.fadeAt) {
			v.told = true
			select {
			case m.ending <- v.id:
			default:
			}
		}
		if n < frames || !v.fade.up && v.fade.pos == 0 {
			continue // finished or faded out
		}
		keep = append(keep, v)
	}
	clear(m.music[len(keep):])
	m.music = keep
}

func (m *Mixer) renderSFX(out []float32) {
	frames := len(out) / 2
	g := float32(m.sfxGain)
	mix := func(v *sfxVoice, from int) int {
		n := min(frames-from, len(v.pcm)-v.at)
		for i := range n {
			s := v.pcm[v.at+i] * g
			out[2*(from+i)] += s
			out[2*(from+i)+1] += s
		}
		v.at += n
		return n
	}
	keep := m.ui[:0]
	for _, v := range m.ui {
		mix(v, 0)
		if v.at < len(v.pcm) {
			keep = append(keep, v)
		}
	}
	clear(m.ui[len(keep):])
	m.ui = keep
	// A jingle that ends mid-buffer hands the rest of it to the next.
	for from := 0; from < frames && len(m.jingles) > 0; {
		from += mix(m.jingles[0], from)
		if m.jingles[0].at >= len(m.jingles[0].pcm) {
			m.jingles[0] = nil
			m.jingles = m.jingles[1:]
		}
	}
}

// limit is a soft clip: linear to ±0.9, then a tanh knee towards ±0.98
// (-0.2 dBFS) that never reaches it, so a pile-up of effects over loud
// music saturates gently instead of wrapping or clicking.
func limit(x float32) float32 {
	const knee, ceiling = 0.9, 0.98
	a := math.Abs(float64(x))
	if a <= knee {
		return x
	}
	y := knee + (ceiling-knee)*math.Tanh((a-knee)/(ceiling-knee))
	return float32(math.Copysign(y, float64(x)))
}

// ---- what the engine calls, under no lock of its own ------------------------

// playSFX starts a rendered effect on its channel. A jingle queue longer
// than maxJingles drops the newest, so a burst of events cannot back the
// fanfares up for seconds.
func (m *Mixer) playSFX(s Sound) {
	const maxJingles = 3
	m.mu.Lock()
	defer m.mu.Unlock()
	v := &sfxVoice{pcm: s.PCM}
	if s.Channel == "jingle" {
		if len(m.jingles) < maxJingles {
			m.jingles = append(m.jingles, v)
		}
		return
	}
	m.ui = append(m.ui, v)
}

// startMusic fades every playing voice out over out seconds and v in over
// in seconds.
func (m *Mixer) startMusic(v *musicVoice, in, out float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fadeOutLocked(out)
	v.fade = fade{step: fadeStep(in), up: true}
	if in <= 0 {
		v.fade.pos = 1
	}
	m.music = append(m.music, v)
}

// stopMusic fades every music voice out over seconds.
func (m *Mixer) stopMusic(seconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fadeOutLocked(seconds)
}

func (m *Mixer) fadeOutLocked(seconds float64) {
	for _, o := range m.music {
		if o.fade.up {
			o.fade.up = false
			// From wherever its fade-in got to, so a cue changed twice
			// quickly does not jump.
			o.fade.step = fadeStep(seconds)
		}
	}
}

func (m *Mixer) setDuck(on bool) {
	m.mu.Lock()
	m.ducked = on
	m.mu.Unlock()
}

// idleFrames is how long nothing has played.
func (m *Mixer) idleFrames() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.idle
}

// hasMusic reports whether the music voice id is still playing.
func (m *Mixer) hasMusic(id uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.ContainsFunc(m.music, func(v *musicVoice) bool { return v.id == id })
}

// stopAll drops every voice at once, for an output that will never play
// them.
func (m *Mixer) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.music, m.ui, m.jingles = nil, nil, nil
}

// busy reports whether anything is playing or queued.
func (m *Mixer) busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.music)+len(m.ui)+len(m.jingles) > 0
}
