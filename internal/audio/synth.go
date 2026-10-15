package audio

import (
	_ "embed"
	"fmt"
	"math"
	"sort"

	"github.com/BurntSushi/toml"
)

// The effects are synthesized, sfxr style: each is a few tones of an
// NES-like voice (pulse, triangle, saw, sine, noise) with a pitch glide,
// arpeggio, vibrato and a percussive envelope, described as data in
// sfx.toml. Nothing is sampled, so no effect carries a license.

//go:embed sfx.toml
var sfxTOML string

// Tone is one voice of an effect.
type Tone struct {
	At   float64 `toml:"at"`   // start, in seconds into the effect
	Wave string  `toml:"wave"` // square, triangle, saw, sine, noise
	Freq float64 `toml:"freq"` // Hz at the start
	To   float64 `toml:"to"`   // Hz at the end, an exponential glide; 0 holds Freq
	Dur  float64 `toml:"dur"`  // seconds, release included
	// The envelope: a linear attack, then exponential decay with time
	// constant Decay (0 holds full level), then a linear release to 0
	// over the last Release seconds.
	Attack  float64 `toml:"attack"`
	Decay   float64 `toml:"decay"`
	Release float64 `toml:"release"`
	Duty    float64 `toml:"duty"`    // square: 0.125, 0.25, 0.5 like the NES; 0 is 0.5
	DutyTo  float64 `toml:"duty_to"` // duty at the end; 0 holds Duty
	Vibrato float64 `toml:"vibrato"` // depth in semitones
	VibRate float64 `toml:"vib_rate"`
	// Arp steps the pitch by these semitones, one every ArpStep seconds,
	// cycling: the chiptune chord.
	Arp     []float64 `toml:"arp"`
	ArpStep float64   `toml:"arp_step"`
	Vol     float64   `toml:"vol"` // relative level; 0 is 1
	// Formants are band-pass filters (Hz, Q, gain) the wave is heard
	// through instead of directly: a vowel, such as a duck's.
	Formants [][3]float64 `toml:"formants"`
	Lowpass  float64      `toml:"lowpass"`  // one-pole cutoff in Hz; 0 is none
	NoiseHz  float64      `toml:"noise_hz"` // noise: sample-and-hold rate; 0 follows the pitch
}

// Effect is one named sound.
type Effect struct {
	// Level is dB relative to the SFX loudness target: fanfares a little
	// over, interface blips under.
	Level float64 `toml:"level"`
	// Channel "ui" sounds at once over anything; "jingle" waits for the
	// jingle before it, so a crash or a fanfare is never stepped on.
	Channel string `toml:"channel"`
	Tones   []Tone `toml:"tone"`
	Note    string `toml:"note"` // what it is meant to sound like
}

// Sound is a rendered effect: mono samples at Rate.
type Sound struct {
	Name    string
	Channel string
	PCM     []float32
}

// LoadEffects parses an effects table (sfx.toml's format).
func LoadEffects(data string) (map[string]Effect, error) {
	var fx map[string]Effect
	if _, err := toml.Decode(data, &fx); err != nil {
		return nil, fmt.Errorf("sfx: %w", err)
	}
	for name, e := range fx {
		if e.Channel != "ui" && e.Channel != "jingle" {
			return nil, fmt.Errorf("sfx %s: channel %q is not ui or jingle", name, e.Channel)
		}
		if len(e.Tones) == 0 {
			return nil, fmt.Errorf("sfx %s: no tones", name)
		}
		for i, t := range e.Tones {
			if err := t.check(); err != nil {
				return nil, fmt.Errorf("sfx %s tone %d: %w", name, i, err)
			}
		}
	}
	return fx, nil
}

// DefaultEffects is sfx.toml.
func DefaultEffects() map[string]Effect {
	fx, err := LoadEffects(sfxTOML)
	if err != nil {
		panic(err) // the embedded table is tested
	}
	return fx
}

func (t Tone) check() error {
	switch t.Wave {
	case "square", "triangle", "saw", "sine", "noise":
	default:
		return fmt.Errorf("wave %q", t.Wave)
	}
	if t.Freq <= 0 || t.Freq >= Rate/2 || t.To < 0 || t.To >= Rate/2 {
		return fmt.Errorf("frequency %v→%v outside 0..%d Hz", t.Freq, t.To, Rate/2)
	}
	if t.Dur <= 0 || t.Dur > 4 || t.At < 0 || t.At > 4 {
		return fmt.Errorf("timing at %v dur %v outside 0..4 s", t.At, t.Dur)
	}
	if t.Attack < 0 || t.Decay < 0 || t.Release < 0 || t.Attack+t.Release > t.Dur {
		return fmt.Errorf("envelope %v/%v/%v does not fit in %v s", t.Attack, t.Decay, t.Release, t.Dur)
	}
	if t.Duty < 0 || t.Duty >= 1 || t.DutyTo < 0 || t.DutyTo >= 1 {
		return fmt.Errorf("duty %v→%v outside 0..1", t.Duty, t.DutyTo)
	}
	if len(t.Arp) > 0 && t.ArpStep <= 0 {
		return fmt.Errorf("arp without arp_step")
	}
	return nil
}

// Render synthesizes every effect, each normalized so its loudest 400 ms
// sits at lufs+Level LUFS (as a centred stereo sound) and its peak at or
// under -1 dBFS.
func Render(fx map[string]Effect, lufs float64) map[string]Sound {
	out := make(map[string]Sound, len(fx))
	for name, e := range fx {
		out[name] = Sound{Name: name, Channel: e.Channel, PCM: e.render(lufs)}
	}
	return out
}

// Names lists effects in a stable order.
func Names[V any](fx map[string]V) []string {
	var ns []string
	for n := range fx {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

// peakCap is -1 dBFS: headroom for the mix and for inter-sample peaks.
const peakCap = 0.891

func (e Effect) render(lufs float64) []float32 {
	var end float64
	for _, t := range e.Tones {
		end = math.Max(end, t.At+t.Dur)
	}
	buf := make([]float64, int(math.Ceil(end*Rate)))
	for i, t := range e.Tones {
		t.render(buf, uint64(i)+1)
	}
	pcm := make([]float32, len(buf))
	for i, v := range buf {
		pcm[i] = float32(v)
	}
	// A mono effect is played to both channels, which BS.1770 hears 3 dB
	// louder than the mono measurement, so it is measured as stereo.
	m := NewMeter(Rate, 2)
	st := make([]float32, 2*len(pcm))
	for i, v := range pcm {
		st[2*i], st[2*i+1] = v, v
	}
	m.Add(st)
	loud := m.MaxMomentary()
	if math.IsInf(loud, -1) {
		return pcm
	}
	g := dbToGain(lufs + e.Level - loud)
	if p := m.Peak() * g; p > peakCap {
		g *= peakCap / p
	}
	for i := range pcm {
		pcm[i] *= float32(g)
	}
	return pcm
}

// polyBLEP smooths a waveform's step so its harmonics do not fold back
// past Nyquist: aliasing is most of what makes naive square waves grate.
func polyBLEP(t, dt float64) float64 {
	switch {
	case t < dt:
		t /= dt
		return t + t - t*t - 1
	case t > 1-dt:
		t = (t - 1) / dt
		return t*t + t + t + 1
	}
	return 0
}

// bandpass is an RBJ band-pass biquad (0 dB peak gain).
func bandpass(f0, q float64) biquad {
	w := 2 * math.Pi * f0 / Rate
	alpha := math.Sin(w) / (2 * q)
	a0 := 1 + alpha
	return biquad{
		b0: alpha / a0, b1: 0, b2: -alpha / a0,
		a1: -2 * math.Cos(w) / a0, a2: (1 - alpha) / a0,
	}
}

func (t Tone) render(buf []float64, seed uint64) {
	start := int(t.At * Rate)
	n := int(t.Dur * Rate)
	vol := t.Vol
	if vol == 0 {
		vol = 1
	}
	duty := t.Duty
	if duty == 0 {
		duty = 0.5
	}
	dutyTo := t.DutyTo
	if dutyTo == 0 {
		dutyTo = duty
	}
	var formants []biquad
	var fgain []float64
	for _, f := range t.Formants {
		formants = append(formants, bandpass(f[0], f[1]))
		fgain = append(fgain, f[2])
	}
	lpA := 0.0
	if t.Lowpass > 0 {
		lpA = 1 - math.Exp(-2*math.Pi*t.Lowpass/Rate)
	}
	var phase, lp, hold, holdPhase float64
	rng := seed*0x9E3779B97F4A7C15 | 1
	for i := 0; i < n && start+i < len(buf); i++ {
		tt := float64(i) / Rate
		frac := tt / t.Dur
		f := t.Freq
		if t.To > 0 {
			f *= math.Pow(t.To/t.Freq, frac)
		}
		if len(t.Arp) > 0 {
			f *= math.Pow(2, t.Arp[int(tt/t.ArpStep)%len(t.Arp)]/12)
		}
		if t.Vibrato > 0 {
			f *= math.Pow(2, t.Vibrato*math.Sin(2*math.Pi*t.VibRate*tt)/12)
		}
		dt := math.Min(f/Rate, 0.5)
		var v float64
		switch t.Wave {
		case "square":
			d := duty + (dutyTo-duty)*frac
			if phase < d {
				v = 1
			} else {
				v = -1
			}
			v += polyBLEP(phase, dt)
			v -= polyBLEP(math.Mod(phase-d+1, 1), dt)
			v -= 2*d - 1 // the duty's DC offset
		case "saw":
			v = 2*phase - 1 - polyBLEP(phase, dt)
		case "triangle":
			v = 4*math.Abs(phase-0.5) - 1
		case "sine":
			v = math.Sin(2 * math.Pi * phase)
		case "noise":
			rate := t.NoiseHz
			if rate == 0 {
				rate = f
			}
			holdPhase += rate / Rate
			if holdPhase >= 1 || i == 0 {
				holdPhase = math.Mod(holdPhase, 1)
				rng ^= rng << 13
				rng ^= rng >> 7
				rng ^= rng << 17
				hold = float64(rng>>11)/float64(1<<53)*2 - 1
			}
			v = hold
		}
		phase += dt
		if phase >= 1 {
			phase--
		}
		if len(formants) > 0 {
			var s float64
			for k := range formants {
				s += fgain[k] * formants[k].step(v)
			}
			v = s
		}
		if lpA > 0 {
			lp += lpA * (v - lp)
			v = lp
		}
		buf[start+i] += vol * v * t.env(tt)
	}
}

func (t Tone) env(tt float64) float64 {
	a := 1.0
	switch {
	case t.Attack > 0 && tt < t.Attack:
		a = tt / t.Attack
	case t.Decay > 0:
		a = math.Exp(-(tt - t.Attack) / t.Decay)
	}
	if r := t.Dur - tt; t.Release > 0 && r < t.Release {
		a *= math.Max(r, 0) / t.Release
	}
	// 1 ms ramps at both ends, so even an envelope without attack or
	// release starts and stops without a click.
	const ramp = 0.001
	if tt < ramp {
		a *= tt / ramp
	}
	if r := t.Dur - tt; r < ramp {
		a *= math.Max(r, 0) / ramp
	}
	return a
}
