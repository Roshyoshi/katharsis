package audio

import (
	"errors"
	"io"
)

// stream plays a source at Rate, looping its [loopStart, loopEnd) part
// passes times (0: forever), then ends. Rates other than Rate go through a
// 4-point cubic Hermite resampler, which is clean enough for the 48 kHz
// files a pool may hold. The loop is followed below the resampler, so the
// interpolation runs straight across the seam.
type stream struct {
	src                source
	loopStart, loopEnd int64 // source frames
	passes             int   // 0 loops forever
	pass               int   // passes finished
	pos                int64 // source frame the next src.Read starts at
	done               bool  // the source is finished for good
	buf                []float32
	bufAt              int // samples of buf already consumed
	err                error

	ratio float64       // source frames per output frame
	frac  float64       // position between hist[1] and hist[2]
	hist  [4][2]float32 // x[-1], x[0], x[1], x[2]
	tail  int           // silent frames pushed after the end
}

func newStream(src source, loopStart, loopEnd float64, passes int) *stream {
	n := src.Frames()
	r := float64(src.Rate())
	s := &stream{src: src, passes: passes, loopEnd: n, ratio: r / Rate}
	if le := int64(loopEnd * r); loopEnd > 0 && le < n {
		s.loopEnd = le
	}
	if ls := int64(loopStart * r); loopStart > 0 && ls < s.loopEnd {
		s.loopStart = ls
	}
	// A VGS song carries its own loop point; the manifest need not.
	if l, ok := src.(looper); ok && loopStart == 0 && loopEnd == 0 {
		if a, b, ok := l.loop(); ok {
			s.loopStart, s.loopEnd = a, b
		}
	}
	if s.ratio != 1 {
		f0, _ := s.next()
		f1, _ := s.next()
		f2, _ := s.next()
		s.hist = [4][2]float32{f0, f0, f1, f2}
	}
	return s
}

// next is the next source frame, following the loop.
func (s *stream) next() ([2]float32, bool) {
	if s.bufAt >= len(s.buf) && !s.refill() {
		return [2]float32{}, false
	}
	f := [2]float32{s.buf[s.bufAt], s.buf[s.bufAt+1]}
	s.bufAt += 2
	return f, true
}

func (s *stream) refill() bool {
	if cap(s.buf) < 2048 {
		s.buf = make([]float32, 0, 2048)
	}
	for !s.done {
		if s.pos >= s.loopEnd {
			s.pass++
			if s.passes > 0 && s.pass >= s.passes {
				s.done = true
				break
			}
			if err := s.src.SeekFrame(s.loopStart); err != nil {
				s.err, s.done = err, true
				break
			}
			s.pos = s.loopStart
		}
		want := min(int64(1024), s.loopEnd-s.pos)
		n, err := s.src.Read(s.buf[:want*2])
		if n == 0 {
			if err != nil && !errors.Is(err, io.EOF) {
				s.err = err
			}
			// The decoder ended before its stated length: that is the end
			// of the loop, then.
			s.loopEnd = s.pos
			if s.loopEnd <= s.loopStart {
				s.done = true
			}
			continue
		}
		s.buf, s.bufAt = s.buf[:n*2], 0
		s.pos += int64(n)
		return true
	}
	s.buf, s.bufAt = s.buf[:0], 0
	return false
}

// Read fills dst (interleaved stereo at Rate) and returns frames written;
// fewer than len(dst)/2 means the stream ended.
func (s *stream) Read(dst []float32) int {
	frames := len(dst) / 2
	if s.ratio == 1 {
		for i := range frames {
			f, ok := s.next()
			if !ok {
				return i
			}
			dst[2*i], dst[2*i+1] = f[0], f[1]
		}
		return frames
	}
	for i := range frames {
		for s.frac >= 1 {
			f, ok := s.next()
			if !ok {
				// Two silent frames carry the last real one through the
				// interpolator; a third would play past the end.
				if s.tail++; s.tail > 2 {
					return i
				}
			}
			s.hist[0], s.hist[1], s.hist[2], s.hist[3] = s.hist[1], s.hist[2], s.hist[3], f
			s.frac--
		}
		t := float32(s.frac)
		for c := range 2 {
			dst[2*i+c] = hermite(s.hist[0][c], s.hist[1][c], s.hist[2][c], s.hist[3][c], t)
		}
		s.frac += s.ratio
	}
	return frames
}

// Remaining is how many output frames are left, or -1 when the stream
// loops forever.
func (s *stream) Remaining() int64 {
	if s.passes == 0 {
		return -1
	}
	left := int64(len(s.buf)-s.bufAt) / 2
	if !s.done {
		left += s.loopEnd - s.pos
		left += int64(max(s.passes-s.pass-1, 0)) * (s.loopEnd - s.loopStart)
	}
	return int64(float64(left) / s.ratio)
}

func hermite(xm1, x0, x1, x2, t float32) float32 {
	c1 := 0.5 * (x1 - xm1)
	c2 := xm1 - 2.5*x0 + 2*x1 - 0.5*x2
	c3 := 0.5*(x2-xm1) + 1.5*(x0-x1)
	return ((c3*t+c2)*t+c1)*t + x0
}

// seek moves to source frame f inside the loop (before playing).
func (s *stream) seek(f int64) error {
	f = min(f, s.loopEnd-1)
	if err := s.src.SeekFrame(f); err != nil {
		return err
	}
	s.pos, s.buf, s.bufAt = f, s.buf[:0], 0
	if s.ratio != 1 {
		f0, _ := s.next()
		f1, _ := s.next()
		f2, _ := s.next()
		s.hist, s.frac = [4][2]float32{f0, f0, f1, f2}, 0
	}
	return nil
}
