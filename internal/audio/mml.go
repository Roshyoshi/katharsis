package audio

import (
	"fmt"
	"io"

	"katharsis/internal/audio/vgs"
)

// mmlSource renders VGS MML (SUZUKI PLAN's format) as it plays: a song is
// a few kilobytes of text and costs a few percent of one core's
// microseconds per buffer, so nothing is decoded ahead.
//
// Frames is the first pass (intro and one loop). The song's loop is the
// decoder's own: at the end of the pass it carries on from the loop
// point, so the stream's seek back there is free and the seam is exactly
// the arranger's.
type mmlSource struct {
	song    *vgs.Song
	dec     *vgs.Decoder
	pos     int64 // frames into the first pass
	scratch []int16
}

func newMML(data []byte) (*mmlSource, error) {
	song, err := vgs.Compile(data)
	if err != nil {
		return nil, err
	}
	if song.Frames() == 0 {
		return nil, fmt.Errorf("mml: no notes")
	}
	return &mmlSource{song: song, dec: vgs.NewDecoder(song)}, nil
}

func (s *mmlSource) Rate() int     { return vgs.Rate }
func (s *mmlSource) Frames() int64 { return s.song.Frames() }

// loop is the song's own loop, in frames.
func (s *mmlSource) loop() (start, end int64, ok bool) {
	return s.song.IntroFrames(), s.song.Frames(), s.song.Loops()
}

func (s *mmlSource) SeekFrame(f int64) error {
	switch {
	case f == s.pos:
		return nil
	case s.song.Loops() && s.pos == s.song.Frames() && f == s.song.IntroFrames():
		s.pos = f // the decoder has just jumped there itself
		return nil
	}
	// Anywhere else: from the top, rendering the way there so every
	// envelope and phase is where it would be.
	s.dec.Reset()
	s.dec.Skip(f)
	s.pos = f
	return nil
}

func (s *mmlSource) Read(dst []float32) (int, error) {
	want := min(int64(len(dst)/2), s.song.Frames()-s.pos)
	if want <= 0 {
		return 0, io.EOF
	}
	if cap(s.scratch) < int(want) {
		s.scratch = make([]int16, want)
	}
	b := s.scratch[:want]
	n := s.dec.Render(b)
	for i, v := range b[:n] {
		f := float32(v) / 32768
		dst[2*i], dst[2*i+1] = f, f
	}
	s.pos += int64(n)
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}
