package flavor

import (
	"strings"

	"katharsis/internal/game"
)

// The story's one-time beats and when they show (docs/story/story-bible.md,
// "How the story unfolds", paced by docs/research/retention.md RT6). Each
// beat shows once and is recorded with game.MarkSeen; game.Meta.Seen, in
// order, is the reread log.
//
//	scene      shown by its screen at its moment (first merge, first boss);
//	           the app checks game.HasSeen and marks it
//	triggered  the boot end that earned it (first crash, a prestige reveal)
//	milestone  a boot end after a dry spell, once a cumulative learning act
//	           earned it (the 5th upsolve, 100 rated doors, a capability)
//
// At most one beat shows per boot end; a triggered beat outranks a
// milestone, which waits. No trigger reads a clock, a date or bytes
// (retention GR2): beats follow what the player did, not when.

// DefaultBeatGap is how many boot ends in a row may pass without a new beat
// before a milestone fills the next one (RT6: no run of more than 8).
const DefaultBeatGap = 8

// NextBeat is the one beat to show at the end of the boot st (crashed or
// cleared), given m as the engine left it after that end, or ok false. gap
// is how many boot ends in a row must pass without a beat before a
// milestone may fill this one (DefaultBeatGap); a negative gap never shows
// milestones. NextBeat is pure: the caller shows the beat and records it
// with game.MarkSeen, and from then on NextBeat gives nothing more for
// this boot.
func NextBeat(st *game.State, m *game.Meta, gap int) (id string, ok bool) {
	if st == nil || st.Phase != game.PhaseCrashed && st.Phase != game.PhaseCleared {
		return "", false
	}
	last := 0 // the boot of the newest beat of any kind
	for _, s := range m.Seen {
		if s.Boot == st.Boot && endBeat(s.ID) {
			return "", false // this boot end already had its beat
		}
		last = max(last, s.Boot)
	}
	for _, b := range triggered {
		if !game.HasSeen(m, b.id) && b.due(st, m) {
			return b.id, true
		}
	}
	if gap < 0 || st.Boot-last-1 < gap {
		return "", false
	}
	for _, b := range milestones {
		if b.id == capabilityBeat {
			for _, c := range m.Capabilities {
				if id := capabilityBeat + ":" + c; !game.HasSeen(m, id) {
					return id, true
				}
			}
			continue
		}
		if !game.HasSeen(m, b.id) && b.due(st, m) {
			return b.id, true
		}
	}
	return "", false
}

// BeatLines is a beat's text, ready to draw: a block in order, or for a
// family (an id of hook:arg) the entry its argument picks, filled with it.
// An unknown id gives nil.
func BeatLines(id string) []string {
	hook, arg, family := strings.Cut(id, ":")
	switch {
	case family && hook == capabilityBeat:
		return strings.Split(Fill(Story(hook, hash64(arg)), "cap", arg), "\n")
	case family:
		return nil
	}
	if ls := StoryBlock(hook); ls != nil {
		return ls
	}
	if s := Story(hook, 0); s != "" {
		return strings.Split(s, "\n")
	}
	return nil
}

// LogEntry is one beat in the reread log.
type LogEntry struct {
	ID, Title string
	Boot      int // the boot it was shown in; 0 before the first boot
	Lines     []string
}

// StoryLog is every beat m has seen, in the order shown, with its text, so
// nothing the story says is missable (RT6's /proc/story). Seen ids that are
// not beats are left out.
func StoryLog(m *game.Meta) []LogEntry {
	var out []LogEntry
	for _, s := range m.Seen {
		t := beatTitle(s.ID)
		if t == "" {
			continue
		}
		out = append(out, LogEntry{ID: s.ID, Title: t, Boot: s.Boot, Lines: BeatLines(s.ID)})
	}
	return out
}

type beat struct {
	id    string
	title string // the reread log's heading
	due   func(st *game.State, m *game.Meta) bool
}

// triggered beats show at the boot end that earned them, in this order
// when two are due. Each also needs a real boot, not a trial: a trial
// unlocks no prestige and leaves no core dump to talk about.
var triggered = []beat{
	// The Cat's speech is about the bytes left on the problem, so it waits
	// for a crash that left some (tone rule 5: never lie about mechanics).
	{"story.first-crash", "the Cat keeps your grin", func(st *game.State, _ *game.Meta) bool {
		return st.Phase == game.PhaseCrashed && st.Trial == nil && st.Crash != nil && st.Crash.Lost > 0
	}},
	// Reveals are win cues, so never on a crash (GR3). A save that already
	// holds a prestige gets each one at its next clears, lowest first.
	{"reveal.prestige.1", "through the looking-glass", reveal(1)},
	{"reveal.prestige.2", "the wood where things have no names", reveal(2)},
	{"reveal.prestige.3", "the Court of Hearts", reveal(3)},
	{"reveal.prestige.4", "the Red King's dream", reveal(4)},
	{"reveal.final-clear", "which dreamed it", func(st *game.State, _ *game.Meta) bool {
		return cleared(st) && st.Prestige == game.MaxPrestige
	}},
}

func reveal(p int) func(*game.State, *game.Meta) bool {
	return func(st *game.State, m *game.Meta) bool { return cleared(st) && m.Prestige >= p }
}

func cleared(st *game.State) bool { return st.Phase == game.PhaseCleared && st.Trial == nil }

// capabilityBeat is a family: one beat per capability earned, its id
// capabilityBeat + ":" + the capability, in the order they were earned.
const capabilityBeat = "milestone.capability"

// milestones fill the dry spells, in this order when several are due. Each
// counts a learning act (a rated door, an upsolve, a lib, a capability) and
// stays due once earned, so a milestone that waits for a gap still holds.
// The rated-door thread is Turing's note, which the prologue mentions and
// the final clear answers.
var milestones = []beat{
	{"milestone.doors.10", "a note under the mat", doors(10)},
	{"milestone.upsolve.5", "the Cat's ledger", upsolves(5)},
	{"milestone.doors.25", "A. M. T., 1936", doors(25)},
	{"milestone.gate.half", "the glass goes cloudy", gateHalf},
	{"milestone.doors.50", "a tape and a head", doors(50)},
	{"milestone.gate.open", "the glass goes soft", gateOpen},
	{"milestone.lib.1", "a word on the wall", func(_ *game.State, m *game.Meta) bool { return len(m.Libs) > 0 }},
	{capabilityBeat, "the Gryphon's certificate", nil},
	{"milestone.doors.75", "the second fold", doors(75)},
	{"milestone.doors.100", "nobody can say", doors(100)},
	{"milestone.upsolve.25", "the ledger, eaten", upsolves(25)},
	{"milestone.doors.150", "one by one", doors(150)},
	{"milestone.doors.200", "snoring in 4/4", doors(200)},
	{"milestone.doors.250", "the last fold", doors(250)},
	{"milestone.doors.300", "older than the Wizards", doors(300)},
	{"milestone.doors.400", "four-fifteen", doors(400)},
	{"milestone.doors.500", "the size of a door", doors(500)},
}

// doors is due at n rated doors, counted as the prestige gate counts them
// (game.RatedDoors): every rated attempt, cleared or not, so a run of
// crashes still moves it.
func doors(n int) func(*game.State, *game.Meta) bool {
	return func(_ *game.State, m *game.Meta) bool { return game.RatedDoors(m) >= n }
}

// upsolves is due at n upsolves of any kind: re-solving the problem that
// crashed you is the learning act, editorial or not.
func upsolves(n int) func(*game.State, *game.Meta) bool {
	return func(_ *game.State, m *game.Meta) bool {
		k := 0
		for _, s := range m.Solves {
			if s.Upsolve {
				k++
			}
		}
		return k >= n
	}
}

// The gate beats follow the first prestige gate: the rated doors
// (Rules.CalibAfter) a clear needs before the glass lets you through.
// Past the first unlock the gate is behind you, and a preset without one
// has nothing to tell. Halfway says "half", so it is due only while the
// gate is still shut: a player who went past it before the beat found a
// gap gets the open one instead.
func gateHalf(st *game.State, m *game.Meta) bool {
	need := st.Rules.CalibAfter
	n := game.RatedDoors(m)
	return m.Prestige == 0 && need > 0 && 2*n >= need && n < need
}

func gateOpen(st *game.State, m *game.Meta) bool {
	need := st.Rules.CalibAfter
	return m.Prestige == 0 && need > 0 && game.RatedDoors(m) >= need
}

// beatTitle is id's heading in the reread log, "" when id is no beat.
func beatTitle(id string) string {
	hook, arg, family := strings.Cut(id, ":")
	for _, list := range [][]beat{scenes, triggered, milestones} {
		for _, b := range list {
			if b.id != hook {
				continue
			}
			if family {
				return b.title + ": " + arg
			}
			return b.title
		}
	}
	return ""
}

// endBeat reports whether id is a beat shown at a boot end (triggered or a
// milestone), not a scene.
func endBeat(id string) bool {
	hook, _, _ := strings.Cut(id, ":")
	for _, list := range [][]beat{triggered, milestones} {
		for _, b := range list {
			if b.id == hook {
				return true
			}
		}
	}
	return false
}

// scenes are the beats a screen shows at its moment, in story order. The
// app decides the moment, checks game.HasSeen and marks them; they are
// listed here for the reread log, and because they count for the gap.
var scenes = []beat{
	{"story.prologue", "down the rabbit-hole", nil},        // before boot 1's first route
	{"door.first.inscription", "hello, world", nil},        // boot 1, room 1
	{"story.first-merge", "the merge window", nil},         // the first merge window
	{"daemon.intro", "the White Knight's inventions", nil}, // the first daemon offer
	{"story.first-boss", "begin at the beginning", nil},    // the first boss door
	{"story.first-regression", "the White Queen", nil},     // the first regression door
	{"story.first-rtfm", "the Mock Turtle's school", nil},  // the first RTFM visit
	{"reveal.final-boss", "the Jabberwock", nil},           // the first halting-problem door
}
