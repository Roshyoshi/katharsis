package rtfm

import (
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/anim"
)

// The switch palace (.agents/work/rtfm-map-plan.md §3.3): when the map
// shows again after a trial granted a capability, the world's marks for it
// fill in along the path, □ to ■, one after another, as Super Mario
// World's blocks fill in after a switch palace. The clear screen already
// named the capability and its biome; this is where it shows. Any key
// skips to the end, and reduced motion shows the end at once.

// flipStep is the time between two marks filling in.
const flipStep = 90 * time.Millisecond

// flipClip fills n marks in turn. Its one line is how many are in.
type flipClip struct{ n int }

func (c flipClip) Len() time.Duration { return time.Duration(c.n) * flipStep }

func (c flipClip) Frame(t time.Duration) []string {
	return []string{strconv.Itoa(min(max(int(t/flipStep), 0), c.n))}
}

func (c flipClip) Next(t time.Duration) time.Duration {
	if t >= c.Len() {
		return anim.Never
	}
	return t - t%flipStep + flipStep
}

// Flip plays the switch palace for caps, the capabilities granted since
// the player last saw the map, on the world the map is at when it is
// next shown (Init). It follows Open.
func (p *Map) Flip(caps []string) {
	p.flipped = map[string]bool{}
	for _, c := range caps {
		p.flipped[c] = true
	}
	n := 0
	if p.world >= 0 {
		for _, s := range p.lv[p.world] {
			if p.flipped[s.Capability] {
				n++
			}
		}
	}
	p.flip = anim.New(flipClip{n}, anim.CurrentMotion())
}

// Init starts the switch palace Flip set up, if any.
func (p *Map) Init() tea.Cmd {
	if p.flip == nil {
		return nil
	}
	return p.flip.Start()
}

// unflipped reports whether level i of the world shown still waits for the
// switch palace: its mark is one being filled in, not yet reached.
func (p *Map) unflipped(i int) bool {
	s := p.lv[p.world]
	if p.flip == nil || p.flip.Done() || !p.flipped[s[i].Capability] {
		return false
	}
	in, _ := strconv.Atoi(p.flip.Frame()[0])
	rank := 0
	for _, x := range s[:i] {
		if p.flipped[x.Capability] {
			rank++
		}
	}
	return rank >= in
}
