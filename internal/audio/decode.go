package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
)

// looper is a source that knows its own loop, in frames.
type looper interface {
	loop() (start, end int64, ok bool)
}

// source is decoded audio at its own rate, as interleaved stereo float32.
type source interface {
	Rate() int
	// Read fills dst with whole frames and returns how many; 0 with io.EOF
	// at the end.
	Read(dst []float32) (int, error)
	SeekFrame(f int64) error
	Frames() int64 // total frames, after any gapless trim
}

// decode opens data as the format its name or magic says.
func decode(name string, data []byte) (source, error) {
	switch {
	case strings.EqualFold(filepath.Ext(name), ".mml"):
		return newMML(data)
	case bytes.HasPrefix(data, []byte("OggS")):
		return newOgg(data)
	case bytes.HasPrefix(data, []byte("RIFF")) && len(data) > 12 && string(data[8:12]) == "WAVE":
		return newWAV(data)
	case bytes.HasPrefix(data, []byte("ID3")) || len(data) > 2 && data[0] == 0xFF && data[1]&0xE0 == 0xE0:
		return newMP3(data)
	}
	return nil, fmt.Errorf("%s: not mp3, ogg vorbis, wav or VGS mml", filepath.Base(name))
}

// IsAudioName reports whether a file name has an extension decode reads.
func IsAudioName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp3", ".ogg", ".oga", ".wav", ".mml":
		return true
	}
	return false
}

// ---- mp3 ---------------------------------------------------------------------

type mp3Source struct {
	d       *mp3.Decoder
	skip, n int64 // gapless: frames to drop at the start, frames kept
	pos     int64 // frame position within the kept frames
	scratch []byte
}

// MP3 encoders pad the start (the encoder delay) and the end; LAME records
// both in its Info tag, and without trimming them a loop has a gap of
// tens of milliseconds at the seam. decoderDelay is the 529 samples of
// the standard MDCT/filterbank decoder delay that LAME's numbers assume.
const decoderDelay = 529

func newMP3(data []byte) (*mp3Source, error) {
	d, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("mp3: %w", err)
	}
	total := d.Length() / 4
	if total <= 0 {
		return nil, errors.New("mp3: no frames")
	}
	s := &mp3Source{d: d, n: total}
	if g, ok := lameGapless(data); ok {
		skip := int64(g.delay + decoderDelay)
		if g.infoFrame {
			skip += int64(g.frameSamples) // go-mp3 decodes the Info frame as silence
		}
		trim := int64(g.padding - decoderDelay)
		if trim < 0 {
			trim = 0
		}
		if skip+trim < total {
			s.skip, s.n = skip, total-skip-trim
		}
	}
	if err := s.SeekFrame(0); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *mp3Source) Rate() int     { return s.d.SampleRate() }
func (s *mp3Source) Frames() int64 { return s.n }

func (s *mp3Source) SeekFrame(f int64) error {
	if _, err := s.d.Seek((s.skip+f)*4, io.SeekStart); err != nil {
		return fmt.Errorf("mp3 seek: %w", err)
	}
	s.pos = f
	return nil
}

func (s *mp3Source) Read(dst []float32) (int, error) {
	want := min(int64(len(dst)/2), s.n-s.pos)
	if want <= 0 {
		return 0, io.EOF
	}
	if cap(s.scratch) < int(want)*4 {
		s.scratch = make([]byte, want*4)
	}
	b := s.scratch[:want*4]
	got, err := io.ReadFull(s.d, b)
	frames := got / 4
	for i := range frames * 2 {
		dst[i] = float32(int16(binary.LittleEndian.Uint16(b[2*i:]))) / 32768
	}
	s.pos += int64(frames)
	if frames == 0 && err != nil {
		return 0, io.EOF
	}
	return frames, nil
}

type gapless struct {
	delay, padding int
	infoFrame      bool
	frameSamples   int
}

// lameGapless reads the encoder delay and padding from a LAME Info/Xing
// tag in the first frame.
func lameGapless(data []byte) (gapless, bool) {
	p := 0
	if len(data) > 10 && string(data[:3]) == "ID3" {
		size := int(data[6]&0x7f)<<21 | int(data[7]&0x7f)<<14 | int(data[8]&0x7f)<<7 | int(data[9]&0x7f)
		p = 10 + size
		if data[5]&0x10 != 0 {
			p += 10 // footer
		}
	}
	for p+4 < len(data) && (data[p] != 0xFF || data[p+1]&0xE0 != 0xE0) {
		p++
	}
	if p+40 > len(data) {
		return gapless{}, false
	}
	h := data[p : p+4]
	version := (h[1] >> 3) & 3 // 3: MPEG-1, 2: MPEG-2, 0: MPEG-2.5
	mono := (h[3] >> 6) == 3
	var side int
	switch {
	case version == 3 && !mono:
		side = 32
	case version == 3:
		side = 17
	case !mono:
		side = 17
	default:
		side = 9
	}
	samples := 1152
	if version != 3 {
		samples = 576
	}
	x := p + 4 + side
	if h[1]&1 == 0 {
		x += 2 // CRC
	}
	if x+120+24 > len(data) {
		return gapless{}, false
	}
	tag := string(data[x : x+4])
	if tag != "Xing" && tag != "Info" {
		return gapless{}, false
	}
	flags := binary.BigEndian.Uint32(data[x+4:])
	l := x + 8
	if flags&1 != 0 {
		l += 4
	}
	if flags&2 != 0 {
		l += 4
	}
	if flags&4 != 0 {
		l += 100
	}
	if flags&8 != 0 {
		l += 4
	}
	if l+24 > len(data) || !strings.HasPrefix(string(data[l:l+4]), "LAME") && !strings.HasPrefix(string(data[l:l+4]), "Lavc") && !strings.HasPrefix(string(data[l:l+4]), "Lavf") {
		return gapless{}, false
	}
	b := data[l+21 : l+24]
	g := gapless{
		delay:        int(b[0])<<4 | int(b[1])>>4,
		padding:      int(b[1]&0x0f)<<8 | int(b[2]),
		infoFrame:    true,
		frameSamples: samples,
	}
	return g, true
}

// ---- ogg vorbis ----------------------------------------------------------------

type oggSource struct {
	r       *oggvorbis.Reader
	ch      int
	scratch []float32
}

func newOgg(data []byte) (*oggSource, error) {
	r, err := oggvorbis.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ogg: %w", err)
	}
	if ch := r.Channels(); ch != 1 && ch != 2 {
		return nil, fmt.Errorf("ogg: %d channels", ch)
	}
	return &oggSource{r: r, ch: r.Channels()}, nil
}

func (s *oggSource) Rate() int     { return s.r.SampleRate() }
func (s *oggSource) Frames() int64 { return s.r.Length() }

func (s *oggSource) SeekFrame(f int64) error {
	if err := s.r.SetPosition(f); err != nil {
		return fmt.Errorf("ogg seek: %w", err)
	}
	return nil
}

func (s *oggSource) Read(dst []float32) (int, error) {
	frames := len(dst) / 2
	if s.ch == 2 {
		n, err := s.r.Read(dst[:frames*2])
		if n == 0 && err != nil {
			return 0, io.EOF
		}
		return n / 2, nil
	}
	if cap(s.scratch) < frames {
		s.scratch = make([]float32, frames)
	}
	n, err := s.r.Read(s.scratch[:frames])
	for i := range n {
		dst[2*i], dst[2*i+1] = s.scratch[i], s.scratch[i]
	}
	if n == 0 && err != nil {
		return 0, io.EOF
	}
	return n, nil
}

// ---- wav -----------------------------------------------------------------------

type wavSource struct {
	data     []byte // the data chunk
	rate, ch int
	bits     int
	float    bool
	frame    int // bytes per frame
	pos      int64
}

func newWAV(b []byte) (*wavSource, error) {
	s := &wavSource{}
	var haveFmt bool
	for p := 12; p+8 <= len(b); {
		id, size := string(b[p:p+4]), int(binary.LittleEndian.Uint32(b[p+4:]))
		body := b[p+8 : min(len(b), p+8+size)]
		switch id {
		case "fmt ":
			if len(body) < 16 {
				return nil, errors.New("wav: short fmt chunk")
			}
			format := binary.LittleEndian.Uint16(body)
			s.ch = int(binary.LittleEndian.Uint16(body[2:]))
			s.rate = int(binary.LittleEndian.Uint32(body[4:]))
			s.bits = int(binary.LittleEndian.Uint16(body[14:]))
			if format == 0xFFFE && len(body) >= 26 {
				format = binary.LittleEndian.Uint16(body[24:])
			}
			s.float = format == 3
			if format != 1 && format != 3 {
				return nil, fmt.Errorf("wav: format %d is not PCM", format)
			}
			haveFmt = true
		case "data":
			s.data = body
		}
		p += 8 + size + size&1
	}
	if !haveFmt || s.data == nil {
		return nil, errors.New("wav: missing fmt or data chunk")
	}
	if s.ch != 1 && s.ch != 2 {
		return nil, fmt.Errorf("wav: %d channels", s.ch)
	}
	if s.rate == 0 {
		return nil, errors.New("wav: 0 Hz") // the resampler would never step
	}
	switch {
	case !s.float && (s.bits == 8 || s.bits == 16 || s.bits == 24 || s.bits == 32):
	case s.float && s.bits == 32:
	default:
		return nil, fmt.Errorf("wav: %d-bit samples", s.bits)
	}
	s.frame = s.ch * s.bits / 8
	return s, nil
}

func (s *wavSource) Rate() int     { return s.rate }
func (s *wavSource) Frames() int64 { return int64(len(s.data) / s.frame) }

func (s *wavSource) SeekFrame(f int64) error {
	if f < 0 || f > s.Frames() {
		return fmt.Errorf("wav seek: frame %d of %d", f, s.Frames())
	}
	s.pos = f
	return nil
}

func (s *wavSource) sample(off int) float32 {
	b := s.data[off:]
	switch {
	case s.float:
		return math.Float32frombits(binary.LittleEndian.Uint32(b))
	case s.bits == 8:
		return (float32(b[0]) - 128) / 128
	case s.bits == 16:
		return float32(int16(binary.LittleEndian.Uint16(b))) / 32768
	case s.bits == 24:
		v := int32(uint32(b[0])<<8|uint32(b[1])<<16|uint32(b[2])<<24) >> 8
		return float32(v) / 8388608
	default:
		return float32(int32(binary.LittleEndian.Uint32(b))) / 2147483648
	}
}

func (s *wavSource) Read(dst []float32) (int, error) {
	n := min(int64(len(dst)/2), s.Frames()-s.pos)
	if n <= 0 {
		return 0, io.EOF
	}
	bps := s.bits / 8
	for i := range n {
		off := int(s.pos+i) * s.frame
		l := s.sample(off)
		r := l
		if s.ch == 2 {
			r = s.sample(off + bps)
		}
		dst[2*i], dst[2*i+1] = l, r
	}
	s.pos += n
	return int(n), nil
}
