// Package story holds the story's own screens (docs/story/story-bible.md,
// "How the story unfolds"): the prologue, the boot splash, boss intro
// cards, the Cheshire Cat's beat on the first crash, biome transitions, the
// first door's inscription, the first merge window, the ACCEPTED beat, and
// a page for any one-time beat. Each is a screens.Screen that plays
// internal/anim clips and leaves with Done.
//
// The screens decide nothing about when they show. The app picks the
// moment, checks game.HasSeen for one-time scenes, and records Done.Scene
// with game.MarkSeen.
package story

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
)

// Done is the intent a story screen leaves with. Scene is the seen id to
// record (a story hook), or "" for a scene that shows every time. Skipped
// is set when esc left it: the player wants out of the story for now, so
// the app drops the scenes queued behind it.
type Done struct {
	Scene   string
	Skipped bool
}

// Read asks the app to show the one-time beat ID as a page (NewBeat) over
// the screen that emits it: the key under a boot end's teaser line, or the
// reread log. It is shown between doors only.
type Read struct{ ID string }

// page is one step of a scene: a clip built for the body's size.
type page func(w, h int) anim.Clip

// Scene plays its pages in order. A key while a page plays shows all of
// it; a key on a finished page goes on to the next, and after the last
// one the scene emits Done. esc leaves at once. With auto set, the scene
// is one page that leaves on its own that long after it is drawn, and any
// key leaves it at once: no key is spent waiting. A scene emits Done once
// and takes no messages after.
type Scene struct {
	id    string
	pages []page
	auto  time.Duration
	// motion is the setting the scene was built under (anim.CurrentMotion).
	motion anim.Motion
	// bare draws the page over the whole screen, with no footer and as
	// its clip draws it, which must be exactly the screen's size: the
	// title, whose art says its own "press any key", and whose fire
	// fills the screen at every frame.
	bare bool
	// noSkip leaves esc to do what any key does: a scene that plays once in
	// full (the Trial's first wake) shows every page.
	noSkip bool

	i    int // the page showing
	p    *anim.Player
	size [2]int // the body size p's clip was built for
	gen  int    // bumps per page, so a stale auto-leave timer is dropped
	left bool   // Done was emitted
}

// defaultSize is the body size before the first View: 80x24 less the
// footer.
var defaultSize = [2]int{80, 23}

func newScene(id string, auto time.Duration, pages ...page) *Scene {
	s := &Scene{id: id, pages: pages, auto: auto, motion: anim.CurrentMotion(), size: defaultSize}
	// Built fully drawn, like the crash screen: Init plays it.
	s.i = len(pages) - 1
	s.p = anim.New(s.pages[s.i](s.size[0], s.size[1]), s.motion)
	s.p.Skip()
	return s
}

// ID is the seen id the scene records, "" for an every-time scene.
func (s *Scene) ID() string { return s.id }

// Init plays the scene from its first page.
func (s *Scene) Init() tea.Cmd { return s.open(0) }

func (s *Scene) open(i int) tea.Cmd {
	s.i = i
	s.gen++
	s.p = anim.New(s.pages[i](s.size[0], s.size[1]), s.motion)
	return tea.Batch(s.p.Start(), s.afterward())
}

// leaveMsg ends a scene that leaves on its own.
type leaveMsg struct {
	s   *Scene
	gen int
}

// afterward is the auto-leave timer, once the last page is done. It is a
// dwell, not motion, so reduced motion keeps it.
func (s *Scene) afterward() tea.Cmd {
	if s.auto <= 0 || s.i != len(s.pages)-1 || !s.p.Done() {
		return nil
	}
	sc, gen := s, s.gen
	return tea.Tick(s.auto, func(time.Time) tea.Msg { return leaveMsg{sc, gen} })
}

func (s *Scene) leave(skipped bool) tea.Cmd {
	s.left = true
	return screens.Emit(Done{Scene: s.id, Skipped: skipped})
}

// Left reports whether the scene has emitted Done.
func (s *Scene) Left() bool { return s.left }

func (s *Scene) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	if s.left {
		return s, nil
	}
	if m, ok := msg.(leaveMsg); ok {
		if m.s == s && m.gen == s.gen {
			return s, s.leave(false)
		}
		return s, nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case k.String() == "esc" && !s.noSkip:
			return s, s.leave(true)
		case s.auto > 0:
			return s, s.leave(false)
		}
	}
	wasDone := s.p.Done()
	if cmd, used := s.p.Update(msg); used {
		if !wasDone && s.p.Done() {
			return s, tea.Batch(cmd, s.afterward())
		}
		return s, cmd
	}
	if _, ok := msg.(tea.KeyPressMsg); ok {
		if s.i+1 < len(s.pages) {
			return s, s.open(s.i + 1)
		}
		return s, s.leave(false)
	}
	return s, nil
}

func (s *Scene) View(width, height int) string {
	bw, bh := width, max(height-1, 1)
	if s.bare {
		bh = max(height, 1)
	}
	if s.size != [2]int{bw, bh} {
		s.size = [2]int{bw, bh}
		s.p.Swap(s.pages[s.i](bw, bh))
	}
	if s.bare {
		return strings.Join(s.p.Frame(), "\n")
	}
	var f string
	switch {
	case s.auto > 0:
		f = ui.Footer(width, "any key", "continue")
	case !s.p.Done() && s.noSkip:
		f = ui.Footer(width, "any key", "show all")
	case !s.p.Done():
		f = ui.Footer(width, "any key", "show all", "esc", "skip")
	case s.i+1 < len(s.pages) && s.noSkip:
		f = ui.Footer(width, "any key", "next")
	case s.i+1 < len(s.pages):
		f = ui.Footer(width, "any key", "next", "esc", "skip")
	default:
		f = ui.Footer(width, "any key", "continue")
	}
	return strings.Join(place(s.p.Frame(), bw, bh), "\n") + "\n" + f
}

// place centers a block in w×h: the block keeps its own left edge, so
// narration stays ragged-right. Rows past h drop from the top, like a
// console; columns past w are cut.
func place(block []string, w, h int) []string {
	if len(block) > h {
		block = block[len(block)-h:]
	}
	bw := 0
	for _, l := range block {
		bw = max(bw, ansi.StringWidth(l))
	}
	lead := strings.Repeat(" ", max((w-bw)/2, 0))
	top := max((h-len(block))/2, 0)
	out := make([]string, h)
	for i := range out {
		if j := i - top; j >= 0 && j < len(block) {
			out[i] = ansi.Truncate(lead+block[j], w, "")
		}
	}
	return out
}

// indent shifts a clip's lines n cells right.
func indent(n int, c anim.Clip) anim.Clip { return shifted{n, c} }

type shifted struct {
	n int
	c anim.Clip
}

func (s shifted) Len() time.Duration                 { return s.c.Len() }
func (s shifted) Next(t time.Duration) time.Duration { return s.c.Next(t) }
func (s shifted) Frame(t time.Duration) []string {
	f := s.c.Frame(t)
	out := make([]string, len(f))
	for i, l := range f {
		out[i] = strings.Repeat(" ", max(s.n, 0)) + l
	}
	return out
}

// Pages is how many pages the scene has.
func (s *Scene) Pages() int { return len(s.pages) }

// Seek shows page i at t into it, without starting its ticks, for tests
// and demos that look at a frame at a time. Seek(i, PageLen(i)) is the
// page's last frame.
func (s *Scene) Seek(i int, t time.Duration) {
	s.i = min(max(i, 0), len(s.pages)-1)
	s.gen++
	s.p = anim.New(s.pages[s.i](s.size[0], s.size[1]), s.motion)
	s.p.Seek(t)
}

// PageLen is how long page i plays at the current size.
func (s *Scene) PageLen(i int) time.Duration {
	return s.pages[min(max(i, 0), len(s.pages)-1)](s.size[0], s.size[1]).Len()
}
