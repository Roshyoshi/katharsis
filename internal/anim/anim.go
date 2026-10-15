// Package anim is katharsis's motion: small, deterministic animations for
// Bubble Tea v2 screens (D33). The crash reveal (600 ms, line by line, any
// key skips; docs/design/ui-system.md §1.7) is the reference, and the rest
// follow its rules:
//
//   - A Clip is a pure function of time: Frame(t) is its look t into the
//     animation, and every t at or past Len gives the last frame. Tests ask
//     for the frame at a time; nothing sleeps.
//   - A Player runs a clip on tea.Tick. It ticks only while the clip is
//     changing (a clip says when it next changes), never faster than the
//     frame-rate cap, and not at all once the clip is done.
//   - Input is never blocked: a key during a clip skips it to the last
//     frame at once.
//   - Motion.Reduced shows every clip's last frame at once, with no ticks.
//
// The text clips keep their last frame's shape at every t: the same lines,
// each the same width, with blanks for what is not shown yet. A screen
// lays a clip out once and the layout never jumps while it plays.
package anim

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Clip is an animation as a function of time.
type Clip interface {
	// Len is when the last frame is reached.
	Len() time.Duration
	// Frame is the clip's lines at t; t <= 0 is the first frame and t >=
	// Len the last.
	Frame(t time.Duration) []string
	// Next is the first time after t at which the frame may change, or
	// Never when it no longer does. The Player sleeps until then.
	Next(t time.Duration) time.Duration
}

// Motion is the motion setting (D2: a tunable setting with a default).
type Motion struct {
	// Reduced shows each clip's last frame at once.
	Reduced bool
	// FPS caps the frame rate; 0 is DefaultFPS.
	FPS int
}

// Never is Next's answer when a clip's frame will not change again.
const Never = time.Duration(math.MaxInt64)

// DefaultFPS is the frame-rate cap when Motion.FPS is 0: smooth enough for
// a typewriter, and no more redraws than a terminal in tmux needs.
const DefaultFPS = 30

// step is the shortest time between two frames.
func (m Motion) step() time.Duration {
	fps := m.FPS
	if fps <= 0 {
		fps = DefaultFPS
	}
	return time.Second / time.Duration(fps)
}

// ParseMotion is the motion setting from the settings ui.motion and
// ui.fps: motion is "full" (or "") or "reduced", and fps the frame-rate
// cap, 0 for DefaultFPS.
func ParseMotion(motion string, fps int) (Motion, error) {
	if fps < 0 {
		return Motion{}, fmt.Errorf("fps %d: want 0 (the default, %d) or more", fps, DefaultFPS)
	}
	switch motion {
	case "", "full":
		return Motion{FPS: fps}, nil
	case "reduced":
		return Motion{Reduced: true, FPS: fps}, nil
	}
	return Motion{}, fmt.Errorf("motion %q: want full or reduced", motion)
}

var motion atomic.Value // Motion

// SetMotion sets the motion every new Player starts with. The app sets it
// once from the settings, before the program starts, like ui.SetPalette.
func SetMotion(m Motion) { motion.Store(m) }

// CurrentMotion is the motion SetMotion last set; full motion at
// DefaultFPS until then.
func CurrentMotion() Motion {
	m, _ := motion.Load().(Motion)
	return m
}

var ids atomic.Uint64

// frameMsg advances the Player it was scheduled for, if that Player is
// still on the run (gen) that scheduled it.
type frameMsg struct{ id, gen uint64 }

// Player runs one clip. A screen holds one per animation, passes it every
// message (Update says whether the message was the player's), and draws
// Frame.
type Player struct {
	clip Clip
	m    Motion
	id   uint64
	gen  uint64
	at   time.Duration // time into the clip, the sum of the ticks so far
	wait time.Duration // the tick in flight, added to at when it lands
}

// New is a Player for c with the current motion setting. It shows the
// first frame until Start.
func New(c Clip) *Player { return NewWith(c, CurrentMotion()) }

// NewWith is New with an explicit motion setting.
func NewWith(c Clip, m Motion) *Player {
	return &Player{clip: c, m: m, id: ids.Add(1)}
}

// Start plays the clip from its first frame. Under reduced motion it jumps
// to the last frame and returns nil. Ticks from an earlier run are
// dropped.
func (p *Player) Start() tea.Cmd {
	p.gen++
	p.at = 0
	if p.m.Reduced {
		p.at = p.clip.Len()
	}
	return p.schedule()
}

// schedule is the tick to the clip's next change, at least one frame-cap
// step away, or nil once the clip is done.
func (p *Player) schedule() tea.Cmd {
	end := p.clip.Len()
	if p.at >= end {
		p.wait = 0
		return nil
	}
	next := min(p.clip.Next(p.at), end)
	p.wait = max(next-p.at, p.m.step())
	id, gen := p.id, p.gen
	return tea.Tick(p.wait, func(time.Time) tea.Msg { return frameMsg{id: id, gen: gen} })
}

// Update takes the player's own ticks and, while the clip runs, a key,
// which skips to the last frame. used reports that msg was the player's,
// so the screen should not act on it too.
func (p *Player) Update(msg tea.Msg) (cmd tea.Cmd, used bool) {
	switch msg := msg.(type) {
	case frameMsg:
		if msg.id != p.id {
			return nil, false
		}
		if msg.gen != p.gen || p.wait == 0 {
			return nil, true // from a run that was restarted or skipped
		}
		p.at = min(p.at+p.wait, p.clip.Len())
		return p.schedule(), true
	case tea.KeyPressMsg:
		if p.Done() {
			return nil, false
		}
		p.Skip()
		return nil, true
	}
	return nil, false
}

// Skip jumps to the last frame; the tick in flight is dropped when it
// lands.
func (p *Player) Skip() {
	p.at = p.clip.Len()
	p.wait = 0
}

// Seek jumps to t into the clip and, unless that is the end, returns the
// tick that plays on from there. Tests use it to look at a frame; a screen
// can use it to keep its place across a rebuilt clip.
func (p *Player) Seek(t time.Duration) tea.Cmd {
	p.gen++
	p.at = min(max(t, 0), p.clip.Len())
	return p.schedule()
}

// Swap replaces the clip and keeps the time, for a screen that rebuilt its
// clip at a new size. The tick in flight keeps running. A finished clip
// stays finished even when the new one is longer: no tick is in flight to
// play the rest.
func (p *Player) Swap(c Clip) {
	done := p.Done()
	p.clip = c
	if done || p.at >= c.Len() {
		p.Skip()
	}
}

// Done reports whether the clip is on its last frame.
func (p *Player) Done() bool { return p.at >= p.clip.Len() }

// At is the time into the clip.
func (p *Player) At() time.Duration { return p.at }

// Frame is the clip's lines now.
func (p *Player) Frame() []string { return p.clip.Frame(p.at) }
