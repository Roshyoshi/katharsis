package anim

import "time"

// Stack plays clips at once, their lines one under another.
func Stack(clips ...Clip) Clip { return stack(clips) }

// Then plays clips one after another, their lines one under another: each
// starts when the one above it ends, and shows its first frame until then.
func Then(clips ...Clip) Clip {
	out := make([]Clip, len(clips))
	var at time.Duration
	for i, c := range clips {
		out[i] = Delay(at, c)
		at += c.Len()
	}
	return stack(out)
}

type stack []Clip

func (s stack) Len() time.Duration {
	var d time.Duration
	for _, c := range s {
		d = max(d, c.Len())
	}
	return d
}

func (s stack) Frame(t time.Duration) []string {
	var out []string
	for _, c := range s {
		out = append(out, c.Frame(t)...)
	}
	return out
}

func (s stack) Next(t time.Duration) time.Duration {
	next := Never
	for _, c := range s {
		next = min(next, c.Next(t))
	}
	return next
}

// Delay holds c's first frame for d, then plays it.
func Delay(d time.Duration, c Clip) Clip {
	if d <= 0 {
		return c
	}
	return delay{d, c}
}

type delay struct {
	d time.Duration
	c Clip
}

func (x delay) Len() time.Duration             { return x.d + x.c.Len() }
func (x delay) Frame(t time.Duration) []string { return x.c.Frame(max(t-x.d, 0)) }
func (x delay) Next(t time.Duration) time.Duration {
	n := x.c.Next(max(t-x.d, 0))
	if n == Never {
		return Never
	}
	return x.d + n
}

// Seq plays clips one after another in the same place: the frame is the
// current clip's.
func Seq(clips ...Clip) Clip { return seq(clips) }

type seq []Clip

func (s seq) Len() time.Duration {
	var d time.Duration
	for _, c := range s {
		d += c.Len()
	}
	return d
}

// at is the index of the clip playing at t, when it started, and t's
// offset into it; past the end, the last clip.
func (s seq) at(t time.Duration) (i int, start, off time.Duration) {
	for i, c := range s {
		if t < start+c.Len() || i == len(s)-1 {
			return i, start, t - start
		}
		start += c.Len()
	}
	return -1, 0, 0
}

func (s seq) Frame(t time.Duration) []string {
	i, _, off := s.at(t)
	if i < 0 {
		return nil
	}
	return s[i].Frame(off)
}

// Next is the current clip's next change, or the switch to the clip after
// it.
func (s seq) Next(t time.Duration) time.Duration {
	i, start, off := s.at(t)
	if i < 0 {
		return Never
	}
	if n := s[i].Next(off); n != Never {
		return start + n
	}
	if i == len(s)-1 {
		return Never
	}
	return start + s[i].Len()
}
