// Package setuid is the game's achievements, setuid bits
// (docs/design/achievements.md): 61 bits in four kinds, each a pure
// function of Meta, one boot's State and Meta.Record, the fold of the boots
// already finished. They pay nothing and steer nothing: internal/game never
// imports this package (GR7), and a bit set is only a line in Meta.Setuid.
//
//	app.Update → game.Step → effects
//	└─ outside a door                      a safe point (GR6)
//	   ├─ new := setuid.Check(&m, s, now)  folds an ended boot once, then
//	   │                                   appends what holds to m.Setuid
//	   └─ store.Checkpoint(m, s)           the bits land with the boot
//	outside a boot: Check(&m, nil, now)    an upsolve, a trial, an install
//	launch: !m.Record.Backfilled → store.BackfillSetuid, then SaveMeta
//
// Only the built-in presets' boots count (Counts), and easing never counts
// against a bit: it is the doors' state, not the player's (R8, GR9).
package setuid

import (
	"slices"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// Kind is a bit's kind, which is its directory on the find screen.
type Kind string

const (
	KindProgression Kind = "progression" // /bin: playing on
	KindSkill       Kind = "skill"       // /sbin: a run set out to do
	KindCompletion  Kind = "completion"  // /usr/bin: counters, 100% and 112%
	KindSecret      Kind = "secret"      // /home/you: dotfiles, -????????? until set
)

// Kinds are the kinds in the screen's order.
var Kinds = []Kind{KindProgression, KindSkill, KindCompletion, KindSecret}

// Dir is the kind's directory.
func (k Kind) Dir() string {
	switch k {
	case KindProgression:
		return "/bin"
	case KindSkill:
		return "/sbin"
	case KindCompletion:
		return "/usr/bin"
	case KindSecret:
		return "/home/you"
	}
	return ""
}

// Def is one bit. Name, Text and Hint are mechanic text: Text names the
// barrier (a boss, a count, a time) and may name numbers (tone rule 5);
// the story's unlock line is the flavor hook "setuid.<ID>", the storyteller's.
type Def struct {
	// ID is the bit's file name: "hello-world", or for a secret a dotfile,
	// ".dull-reality".
	ID   string
	Kind Kind
	Name string
	// Text is the barrier the row states; a secret's shows once set.
	Text string
	// Hint is a secret's line under `?`, "" for the rest.
	Hint string

	// holds reports whether the bit's condition holds in v.
	holds func(v *view) bool
	// current marks a condition that reads Meta as it stands, beyond the
	// boots' fold (capabilities, libs, ratings, completion, other bits): a
	// backfill checks it once, at its end, since history cannot date it.
	current bool
	// progress is a counter's have and need, nil for no counter.
	progress func(m *game.Meta) (have, need int)
}

// Path is the bit's path on the find screen: "/bin/hello-world".
func (d Def) Path() string { return d.Kind.Dir() + "/" + d.ID }

// Secret reports whether the bit is a secret: a -????????? row until set.
func (d Def) Secret() bool { return d.Kind == KindSecret }

// Progress is a counter's progress, have of need (have at most need); need
// 0 for a bit without one. A ring bit shows the prestige gate's rated
// doors while its level is the next to unlock.
func (d Def) Progress(m *game.Meta) (have, need int) {
	if d.progress == nil || m == nil {
		return 0, 0
	}
	have, need = d.progress(m)
	return min(have, need), need
}

// Lookup finds the bit id in Table.
func Lookup(id string) (Def, bool) {
	i := slices.IndexFunc(Table, func(d Def) bool { return d.ID == id })
	if i < 0 {
		return Def{}, false
	}
	return Table[i], true
}

// Bit is the bit id as m set it, ok false while it is not set.
func Bit(m *game.Meta, id string) (b game.SetuidBit, ok bool) {
	i := slices.IndexFunc(m.Setuid, func(b game.SetuidBit) bool { return b.ID == id })
	if i < 0 {
		return game.SetuidBit{}, false
	}
	return m.Setuid[i], true
}

// IsSet reports whether m has bit id set.
func IsSet(m *game.Meta, id string) bool {
	_, ok := Bit(m, id)
	return ok
}

// Counts reports whether boot s counts toward the bits: a boot, not a
// learn-mode trial, of a built-in preset (rules.BuiltIns: their names are
// reserved, so a stored name always means those rules). A custom preset
// can set timer_mult to 10, and its clears set nothing.
func Counts(s *game.State) bool {
	if s == nil || s.Trial != nil {
		return false
	}
	return slices.Contains(builtIns, s.Rules.Name)
}

// builtIns are the built-in presets' names.
var builtIns = func() []string {
	var names []string
	for _, r := range rules.BuiltIns() {
		names = append(names, r.Name)
	}
	return names
}()

// view is what one check reads: Meta, the boot checked when it counts (s,
// nil otherwise and outside a boot), the boot whatever it is (boot), when,
// and in a backfill the events table's sample runs.
type view struct {
	m        *game.Meta
	s        *game.State
	boot     *game.State
	now      time.Time
	backfill *Samples
}

// Check sets every bit that holds for m and boot s (nil outside a boot)
// and is not set yet, at now, and returns those it set, in Table's order.
// A boot that has ended is folded into m.Record first (Fold), once. Check
// sets nothing while a door is open (GR6), never clears a bit, and twice is
// once: it reads only m, s and now, and now only for the timestamps.
//
// A boot's own bits hold as soon as it shows them (a boss beaten, the pool
// at its cap); those of the Record wait for the boot's end, when it is
// folded. An abandoned boot keeps what it did before; its end, a crash,
// sets nothing a clear would.
func Check(m *game.Meta, s *game.State, now time.Time) []game.SetuidBit {
	if m == nil || s != nil && (s.Phase == game.PhaseProblem || s.Current != nil) {
		return nil
	}
	Fold(m, s)
	return check(&view{m: m, boot: s, now: now}, func(Def) bool { return true })
}

// check sets the bits of Table that which picks and that hold in v,
// stamped v.now (and Backfill in a backfill), and returns them.
func check(v *view, which func(Def) bool) []game.SetuidBit {
	if Counts(v.boot) {
		v.s = v.boot
	}
	var set []game.SetuidBit
	for _, d := range Table {
		if d.holds == nil || !which(d) || IsSet(v.m, d.ID) || !d.holds(v) {
			continue
		}
		b := game.SetuidBit{ID: d.ID, At: v.now, Backfill: v.backfill != nil}
		if v.boot != nil {
			b.Boot = v.boot.Boot
		}
		v.m.Setuid = append(v.m.Setuid, b) // later bits of the table read it
		set = append(set, b)
	}
	return set
}

// ended reports whether s is a boot (or trial) that has ended.
func ended(s *game.State) bool {
	return s != nil && (s.Phase == game.PhaseCrashed || s.Phase == game.PhaseCleared)
}

// Fold folds the ended boot s into m.Record, once (Record.Folded): what it
// reached that the bits count across boots. A boot still running, and one
// already folded, change nothing; a trial is only marked folded, and a boot
// of a custom preset adds only its boot time.
func Fold(m *game.Meta, s *game.State) {
	if m == nil || !ended(s) || s.Boot <= m.Record.Folded {
		return
	}
	m.Record.Folded = s.Boot
	if s.Trial != nil {
		return
	}
	m.Record.BootTime += s.BootTime(s.Started)
	if Counts(s) {
		fold(&m.Record, s)
	}
}

// FreshStore reports whether speed completion id (speed-completion,
// speed-completion-112) can no longer be set on this store: its
// percentage was reached, or its boot time passed, without it. The screen
// says "for a fresh store", never "missed" (GR8).
func FreshStore(m *game.Meta, id string) bool {
	pct, bound, ok := speedOf(id)
	if !ok || IsSet(m, id) {
		return false
	}
	got, _ := Completion(m)
	return got >= pct || m.Record.BootTime >= bound
}

// speedOf is speed completion id's percentage and boot-time bound.
func speedOf(id string) (pct int, bound time.Duration, ok bool) {
	b := rules.Setuid()
	switch id {
	case "speed-completion":
		return Full, b.SpeedCompletion, true
	case "speed-completion-112":
		return Pure, b.SpeedCompletion112, true
	}
	return 0, 0, false
}
