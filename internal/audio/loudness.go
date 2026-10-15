package audio

import "math"

// Loudness follows ITU-R BS.1770-4: K-weighting (a high shelf for the
// head's acoustics, then a high-pass), mean square per channel over 400 ms
// blocks that overlap by 75%, an absolute gate at -70 LUFS and a relative
// gate 10 LU under the ungated mean. The filter coefficients come from the
// analog prototypes through the bilinear transform, as libebur128 derives
// them, so any sample rate works.

type biquad struct {
	b0, b1, b2, a1, a2 float64
	z1, z2             float64
}

func (f *biquad) step(x float64) float64 {
	y := f.b0*x + f.z1
	f.z1 = f.b1*x - f.a1*y + f.z2
	f.z2 = f.b2*x - f.a2*y
	return y
}

func kWeighting(rate int) (shelf, highpass biquad) {
	fs := float64(rate)
	{
		const f0, g, q = 1681.974450955533, 3.999843853973347, 0.7071752369554196
		k := math.Tan(math.Pi * f0 / fs)
		vh := math.Pow(10, g/20)
		vb := math.Pow(vh, 0.4996667741545416)
		a0 := 1 + k/q + k*k
		shelf = biquad{
			b0: (vh + vb*k/q + k*k) / a0,
			b1: 2 * (k*k - vh) / a0,
			b2: (vh - vb*k/q + k*k) / a0,
			a1: 2 * (k*k - 1) / a0,
			a2: (1 - k/q + k*k) / a0,
		}
	}
	{
		const f0, q = 38.13547087602444, 0.5003270373238773
		k := math.Tan(math.Pi * f0 / fs)
		a0 := 1 + k/q + k*k
		highpass = biquad{b0: 1, b1: -2, b2: 1, a1: 2 * (k*k - 1) / a0, a2: (1 - k/q + k*k) / a0}
	}
	return shelf, highpass
}

// Meter measures interleaved stereo (or mono) float frames as they stream
// past, so a whole track never needs to sit in memory.
type Meter struct {
	ch      int
	filters [][2]biquad
	sub     int       // frames per 100 ms sub-block
	n       int       // frames in the current sub-block
	acc     float64   // weighted sum of squares in it
	subs    []float64 // finished sub-block mean squares
	peak    float64
	frames  int
}

func NewMeter(rate, channels int) *Meter {
	m := &Meter{ch: channels, sub: rate / 10}
	for range channels {
		s, h := kWeighting(rate)
		m.filters = append(m.filters, [2]biquad{s, h})
	}
	return m
}

// Add measures interleaved frames.
func (m *Meter) Add(x []float32) {
	for i := 0; i+m.ch <= len(x); i += m.ch {
		for c := range m.ch {
			v := float64(x[i+c])
			if a := math.Abs(v); a > m.peak {
				m.peak = a
			}
			f := &m.filters[c]
			y := f[1].step(f[0].step(v))
			m.acc += y * y
		}
		m.n++
		m.frames++
		if m.n == m.sub {
			m.subs = append(m.subs, m.acc/float64(m.sub))
			m.n, m.acc = 0, 0
		}
	}
}

func lufs(z float64) float64 {
	if z <= 0 {
		return math.Inf(-1)
	}
	return -0.691 + 10*math.Log10(z)
}

// blocks are the 400 ms gating blocks' mean squares (hop 100 ms). A clip
// shorter than one block is one block of its whole length, which is how
// loud a short effect sounds.
func (m *Meter) blocks() []float64 {
	if len(m.subs) < 4 {
		if m.frames == 0 {
			return nil
		}
		var sum float64
		for _, s := range m.subs {
			sum += s * float64(m.sub)
		}
		sum += m.acc
		return []float64{sum / float64(m.frames)}
	}
	out := make([]float64, 0, len(m.subs)-3)
	for i := 3; i < len(m.subs); i++ {
		out = append(out, (m.subs[i-3]+m.subs[i-2]+m.subs[i-1]+m.subs[i])/4)
	}
	return out
}

// Integrated is the gated loudness of everything added, in LUFS; -Inf for
// silence.
func (m *Meter) Integrated() float64 {
	bs := m.blocks()
	const abs = -70.0
	var sum float64
	var n int
	for _, z := range bs {
		if lufs(z) > abs {
			sum += z
			n++
		}
	}
	if n == 0 {
		return math.Inf(-1)
	}
	rel := lufs(sum/float64(n)) - 10
	sum, n = 0, 0
	for _, z := range bs {
		if l := lufs(z); l > abs && l > rel {
			sum += z
			n++
		}
	}
	if n == 0 {
		return math.Inf(-1)
	}
	return lufs(sum / float64(n))
}

// MaxMomentary is the loudest 400 ms block, in LUFS: how loud an effect
// feels at its peak.
func (m *Meter) MaxMomentary() float64 {
	best := math.Inf(-1)
	for _, z := range m.blocks() {
		best = math.Max(best, lufs(z))
	}
	return best
}

// Peak is the largest sample magnitude seen (sample peak, not true peak).
func (m *Meter) Peak() float64 { return m.peak }

// Frames is how many frames were added.
func (m *Meter) Frames() int { return m.frames }

// RMS is the unweighted root mean square over every sample, for reports.
func RMS(x []float32) float64 {
	if len(x) == 0 {
		return 0
	}
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(len(x)))
}

// DB converts a linear amplitude to dBFS.
func DB(a float64) float64 {
	if a <= 0 {
		return math.Inf(-1)
	}
	return 20 * math.Log10(a)
}
