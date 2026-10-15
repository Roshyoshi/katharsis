package audio

import (
	"fmt"
	"math"
	"os"
	"slices"
	"time"
)

// Analysis is what can be checked about a track without listening to it.
type Analysis struct {
	Rate                     int
	Frames                   int64
	LUFS, PeakDB, RMSDB      float64
	Clipped                  int     // samples at or past ±0.999
	LeadSilence, TailSilence float64 // seconds under -60 dBFS at each end
	// SeamJump is the step from the loop's last frame to its first, in
	// multiples of the track's median step: near 1 is seamless, above
	// ~10 an audible click.
	SeamJump float64
	// SeamStepDB is the level of the loop's last 100 ms against its first
	// 100 ms: near 0 for a loop, very negative for a fade-out ending,
	// positive for a quiet intro.
	SeamStepDB float64
	Trimmed    int64 // mp3 gapless frames dropped (encoder delay + padding)
	// DecodeSpeed is how many seconds of audio decode per second of CPU
	// wall time, single-threaded.
	DecodeSpeed float64
}

// Analyze decodes a whole file at its own rate and measures it, with the
// loop bounds a manifest entry would give it (0, 0: the whole file).
func Analyze(path string, loopStart, loopEnd float64) (Analysis, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Analysis{}, err
	}
	t0 := time.Now()
	src, err := decode(path, data)
	if err != nil {
		return Analysis{}, err
	}
	var pcm []float32
	buf := make([]float32, 16384)
	for {
		n, err := src.Read(buf)
		if n == 0 || err != nil {
			break
		}
		pcm = append(pcm, buf[:2*n]...)
	}
	took := time.Since(t0)
	a := Analysis{Rate: src.Rate(), Frames: int64(len(pcm) / 2)}
	if mp3, ok := src.(*mp3Source); ok {
		a.Trimmed = mp3.d.Length()/4 - mp3.n
	}
	if a.Frames == 0 {
		return a, fmt.Errorf("%s: no audio", path)
	}
	secs := float64(a.Frames) / float64(a.Rate)
	a.DecodeSpeed = secs / math.Max(took.Seconds(), 1e-9)
	m := NewMeter(a.Rate, 2)
	m.Add(pcm)
	a.LUFS, a.PeakDB, a.RMSDB = m.Integrated(), DB(m.Peak()), DB(RMS(pcm))
	for _, v := range pcm {
		if math.Abs(float64(v)) >= 0.999 {
			a.Clipped++
		}
	}
	quiet := dbToGain(-60)
	loud := func(i int64) bool {
		return math.Abs(float64(pcm[2*i])) > quiet || math.Abs(float64(pcm[2*i+1])) > quiet
	}
	var lead, tail int64
	for lead < a.Frames && !loud(lead) {
		lead++
	}
	for tail < a.Frames && !loud(a.Frames-1-tail) {
		tail++
	}
	a.LeadSilence, a.TailSilence = float64(lead)/float64(a.Rate), float64(tail)/float64(a.Rate)

	ls, le := int64(loopStart*float64(a.Rate)), a.Frames
	if loopEnd > 0 {
		le = min(a.Frames, int64(loopEnd*float64(a.Rate)))
	}
	w := int64(a.Rate / 10)
	// What plays after the loop's last frame: its first frames, or for a
	// VGS song the decoder's own continuation from its loop mark.
	var after []float32
	if l, ok := src.(looper); ok && loopStart == 0 && loopEnd == 0 {
		if s, e, ok := l.loop(); ok && src.SeekFrame(s) == nil {
			ls, le = s, e
			after = make([]float32, 2*w)
			n, _ := src.Read(after)
			after = after[:2*n]
		}
	}
	if ls >= le-1 {
		return a, nil
	}
	if after == nil {
		after = pcm[2*ls : 2*min(ls+w, le)]
	}
	if len(after) < 2 {
		return a, nil
	}
	steps := make([]float64, 0, a.Frames)
	for i := ls + 1; i < le; i++ {
		steps = append(steps, math.Abs(float64(pcm[2*i]-pcm[2*i-2]))+math.Abs(float64(pcm[2*i+1]-pcm[2*i-1])))
	}
	slices.Sort(steps)
	med := math.Max(steps[len(steps)/2], 1e-4)
	jump := math.Abs(float64(after[0]-pcm[2*(le-1)])) + math.Abs(float64(after[1]-pcm[2*(le-1)+1]))
	a.SeamJump = jump / med
	if le-ls > 2*w {
		head := RMS(after)
		end := RMS(pcm[2*(le-w) : 2*le])
		a.SeamStepDB = DB(math.Max(end, 1e-6)) - DB(math.Max(head, 1e-6))
	}
	return a, nil
}
