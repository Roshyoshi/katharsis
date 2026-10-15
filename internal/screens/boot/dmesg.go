package boot

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// The dmesg strip (UX J5): the last few things that changed the player's
// lives, ducks, bytes or build, in the kernel log's idiom, under the
// header's status line on every boot screen. It is never modal: a line
// stays until newer ones push it out. The app keeps one Tracker per game,
// shows it the state after every update, and hands its events back
// through Env.Events.
//
//	Boot #1 · Bronze · merge window                           uptime 02:46
//	♥♥♥♡♡  🦆1  83 B uncommitted · 0 B committed
//	[  166.2] judge: ✓ AC "Matrix Rotation" in 00:42 of 17:00 · +41 B
//	[  166.2] merge window: ducks refilled 🦆0 → 🦆1
//	          "Did you say pig, or fig?" The Cat is back on its branch.
//	──────────────────────────────────────────────────────────────────────
//
// An event's first line is the mechanic, in the game's own numbers: that
// line is the truth (story bible, tone rule 5). Its flavor line, from
// docs/story/text.md, goes under it while it is the newest.

// Event is one entry of the strip.
type Event struct {
	At   time.Time
	Kind string // what happened, for tests and the app: "verdict", "ac", ...
	Text string // the mechanic line, styled
	// Hook names the flavor line (flavor.Story), "" for none; Fill holds
	// its name/value pairs and Seed picks a pool's entry.
	Hook string
	Fill []string
	Seed uint64
	// Seen is the one-time id the app records with game.MarkSeen once the
	// event is made: onboarding lines ("tip.*") show the first time only.
	Seen string
}

// Event kinds.
const (
	EvVerdict    = "verdict"    // a failed submit and its cost
	EvFree       = "free"       // a failed submit that cost nothing (a CE)
	EvAC         = "ac"         // a door cleared
	EvRegression = "regression" // a regression door cleared
	EvRetreat    = "retreat"    // a door swapped, for a life or kexec
	EvTimeout    = "timeout"    // a door's timer ran out and cost a life (D44)
	EvShuf       = "shuf"       // a room re-rolled with a shuf
	EvQuack      = "quack"      // a duck spent
	EvDuckRefuse = "duck-refused"
	EvUpgrade    = "upgrade"
	EvDaemon     = "daemon"
	EvSpend      = "spend"
	EvRefill     = "refill" // ducks refilled at a merge window
	EvCommit     = "commit"
	EvLeave      = "leave" // left a merge window
	EvLib        = "lib"   // a // lib: block saved from an AC
	EvDoor       = "door"  // a door with modifiers opened
	EvTimer      = "timer" // 5:00 or 1:00 left on the open door
	EvLives      = "lives" // lives changed for a reason the strip has no line for
	EvTip        = "tip"   // a one-time onboarding line
	EvEase       = "ease"  // an easing the engine made (RT8), announced, or declined
)

// keepEvents is how many events a Tracker keeps; the strip shows at most
// three rows.
const keepEvents = 16

// Tracker turns a boot's changes into strip events by comparing the state
// after each update with the one before. The first look only takes a
// baseline, so a resumed boot does not replay its past, and a new boot
// starts an empty strip.
type Tracker struct {
	// Lookup names the problems in the lines; without it a problem is its
	// id.
	Lookup screens.Lookup
	// Hold keeps the onboarding lines back while something covers the boot
	// screens (a story scene): a line nobody saw must not count as seen.
	Hold bool

	prev   snapshot
	ok     bool
	events []Event
	// judged is the last full-test result the app reported (Judged), for
	// the verdict line's passed count, which the state does not keep.
	judged [2]int
}

// snapshot is what the events are diffed from.
type snapshot struct {
	boot                          int
	phase                         game.Phase
	lives, ducks                  int
	hist                          int
	door                          catalog.ID // the open attempt
	opened                        time.Time
	submits, quacks               int
	uncommitted, committed, spent int64
	upgrades, daemons, rerolls    int
	at                            time.Duration
	kexec, declined               bool
	libs                          map[string]string
	biome, room                   int
	offered                       string
	warned                        int // timer marks passed on the open door: 1 at 5:00, 2 at 1:00
}

func snap(st *game.State, m *game.Meta, now time.Time) snapshot {
	s := snapshot{
		boot: st.Boot, phase: st.Phase, lives: st.Lives, ducks: st.Ducks, hist: len(st.History),
		uncommitted: st.Uncommitted, committed: st.Committed, spent: st.Spent,
		upgrades: len(st.Upgrades), daemons: len(st.Daemons), rerolls: st.Rerolls, at: st.AtTime,
		kexec: st.KexecUsed, declined: st.Declined, biome: st.Biome, room: st.Room, libs: map[string]string{},
	}
	if c := st.Current; c != nil {
		s.door, s.opened, s.submits, s.quacks = c.Door.Problem, c.Opened, c.Submits, c.Quacks
		s.warned = timerMark(st, now)
	}
	for _, l := range m.Libs {
		s.libs[l.Name] = l.Source
	}
	ids := make([]string, len(st.Doors))
	for i, d := range st.Doors {
		ids[i] = string(d.Problem)
	}
	s.offered = strings.Join(ids, ",")
	return s
}

// timerMark is how many warnings the open door's timer has passed: 1 at
// 5:00 left, 2 at 1:00, each only on a door whose timer is longer. A soft
// timer (a trial) only goes overtime, so it is never warned.
func timerMark(st *game.State, now time.Time) int {
	c := st.Current
	if c == nil || st.SoftTimer() || st.Phase != game.PhaseProblem {
		return 0
	}
	left := c.Deadline.Sub(now)
	switch {
	case left <= 0:
		return 0
	case c.Door.Timer > time.Minute && left <= time.Minute:
		return 2
	case c.Door.Timer > 5*time.Minute && left <= 5*time.Minute:
		return 1
	}
	return 0
}

// Events is the strip as it stands, oldest first.
func (t *Tracker) Events() []Event { return slices.Clone(t.events) }

// Judged tells the tracker the passed and total of the full-test verdict
// about to be stepped, which the state keeps only as a best.
func (t *Tracker) Judged(passed, total int) { t.judged = [2]int{passed, total} }

// Add puts an event the state cannot show on the strip, such as a duck
// the engine refused.
func (t *Tracker) Add(ev Event) {
	t.events = append(t.events, ev)
	if n := len(t.events); n > keepEvents {
		t.events = slices.Delete(t.events, 0, n-keepEvents)
	}
}

// Observe compares st and m with the last look and returns the events
// the change makes, which it also keeps for the strip. m is read only;
// the caller marks each returned Seen.
func (t *Tracker) Observe(st *game.State, m *game.Meta, now time.Time) []Event {
	if st == nil {
		t.ok, t.events = false, nil
		return nil
	}
	n := snap(st, m, now)
	p, ok := t.prev, t.ok
	t.prev, t.ok = n, true
	if !ok || p.boot != n.boot {
		t.events, ok = nil, false
	}
	var out []Event
	if ok {
		out = t.diff(p, n, st, m, now)
	} else {
		// A boot's easing is said as the strip starts (RT8, notice.go).
		lines, _ := easing(st)
		for _, l := range lines {
			out = append(out, Event{At: now, Kind: EvEase, Text: l})
		}
	}
	if !t.Hold {
		out = append(out, tips(st, m, now, out)...)
	}
	for _, ev := range out {
		t.Add(ev)
	}
	return out
}

// diff is every event between snapshots p and n of the same boot.
func (t *Tracker) diff(p, n snapshot, st *game.State, m *game.Meta, now time.Time) []Event {
	// Judged is called in the update that steps the verdict, so this look
	// is the one it belongs to.
	judged := t.judged
	t.judged = [2]int{}
	var out []Event
	add := func(kind, text, hook string, fill ...string) *Event {
		out = append(out, Event{At: now, Kind: kind, Text: text, Hook: hook, Fill: fill, Seed: st.Seed ^ uint64(now.Unix())})
		return &out[len(out)-1]
	}
	livesSeen := false // a line below accounts for a change in lives

	// Libs come from accepted code and are saved before the clear is
	// stepped, so their lines go first.
	for _, name := range slices.Sorted(maps.Keys(n.libs)) {
		if src, had := p.libs[name]; !had || src != n.libs[name] {
			add(EvLib, "lib: "+ui.Strong.Render(name)+" saved"+ui.Muted.Render(ui.G.Sep+"link it when a boot starts"),
				"lib.saved", "name", ui.Clip(name, 16))
		}
	}

	// A failed submit on the door that is still open.
	if n.door != "" && n.door == p.door && n.opened.Equal(p.opened) && n.submits > p.submits && n.phase == game.PhaseProblem {
		v := string(st.Current.LastVerdict)
		text := "judge: " + ui.Verdict(v).Render(v)
		if judged[1] > 0 {
			text += fmt.Sprintf(" %d/%d", judged[0], judged[1])
		}
		kind := EvVerdict
		if lost := p.lives - n.lives; lost > 0 {
			text += ui.Muted.Render(ui.G.Sep) + livesLost(lost, n.lives)
			livesSeen = true
		} else {
			kind, text = EvFree, text+ui.Muted.Render(ui.G.Sep+"no charge")
		}
		// The flavor line keys on the fold, an OLE on the MLE line.
		add(kind, text, "verdict."+flavor.NormVerdict(v))
	}

	// Doors that closed since: a clear, a swap, or a timeout.
	for _, a := range st.History[min(p.hist, len(st.History)):] {
		switch a.Outcome {
		case game.Cleared:
			title := strconv.Quote(ui.Clip(t.problem(a.Door.Problem).Title, 24))
			text := "judge: " + ui.OK.Render(ui.G.Pass) + " AC " + title +
				ui.Muted.Render(fmt.Sprintf(" in %s of %s", units.Clock(elapsed(a, now)), units.Clock(a.Door.Timer)))
			hook := "verdict.AC"
			// -O3 rides on the clear's line, as J5 draws it, with the Red
			// Queen's nod for its flavor. The last clear ends the boot with no
			// pick to earn.
			if a.O3 && st.Phase != game.PhaseCleared {
				text += ui.Muted.Render(ui.G.Sep) + ui.OK.Render("-O3")
				hook = "o3.bonus"
			}
			add(EvAC, text+ui.Muted.Render(ui.G.Sep)+ui.OK.Render("+"+units.Bytes(st.Paid(a))), hook)
			if a.Door.Regression {
				add(EvRegression, regressionCleared(m, a.Door.Problem, now), "regression.cleared")
			}
		case game.Retreated:
			text := ui.Warn.Render(ui.G.Retreat) + " retreat: "
			if n.kexec && !p.kexec && n.lives >= p.lives {
				text += "kexec swapped it free" + ui.Muted.Render(ui.G.Sep+"used for this biome")
			} else {
				text += livesLost(p.lives-n.lives, n.lives)
				livesSeen = true
			}
			add(EvRetreat, text+ui.Muted.Render(ui.G.Sep+"new doors"), "")
		case game.TimedOut:
			// A timeout that ended the boot is the panic's to say; one that
			// cost a life (D44) left the boot running, on new doors.
			if n.phase != game.PhaseCrashed {
				text := ui.Warn.Render(ui.G.Timer+"timer") + " ran out: " + livesLost(p.lives-n.lives, n.lives)
				add(EvTimeout, text+ui.Muted.Render(ui.G.Sep+"new doors"), "")
				livesSeen = true
			}
		}
	}

	// The easing turned down before the first door (RT8): what it took
	// back, and the room drawn again without it.
	if n.declined && !p.declined {
		text := "easing declined"
		if lost := p.lives - n.lives; lost > 0 {
			text += ui.Muted.Render(ui.G.Sep) + livesLost(lost, n.lives)
			livesSeen = true
		}
		if n.phase == game.PhaseDoors {
			text += ui.Muted.Render(ui.G.Sep + "new doors")
		}
		add(EvEase, text, "")
	}

	// A shuf spent on the doors screen: the same room, other doors.
	if n.rerolls < p.rerolls && n.phase == game.PhaseDoors && n.biome == p.biome && n.room == p.room && n.offered != p.offered {
		add(EvShuf, "shuf: new doors"+ui.Muted.Render(ui.G.Sep+units.Plural(n.rerolls, "shuf")+" left"), "")
	}

	// A duck spent on the open door.
	if n.door != "" && n.door == p.door && n.quacks > p.quacks && st.Current != nil {
		c := st.Current
		prob := t.problem(c.Door.Problem)
		what, hook, fill := "tags shown", "duck.quack.1", []string{"tags", ui.Clip(strings.Join(prob.Tags, ", "), 24)}
		if c.Hint >= 2 {
			what, hook, fill = "technique shown", "duck.quack.2", []string{"technique", ui.Clip(prob.Topic, 15)}
		}
		add(EvQuack, "duck: "+what+ui.Muted.Render(ui.G.Sep)+fmt.Sprintf("%s%d left", ui.G.Duck, n.ducks), hook, fill...)
	}

	// Picks.
	for _, u := range st.Upgrades[min(p.upgrades, len(st.Upgrades)):] {
		name, desc := upgradeText(u, st.Rules)
		text := "upgrade: " + ui.Strong.Render(name)
		if d := strings.TrimSuffix(desc, "."); d != "" {
			text += ui.Muted.Render(ui.G.Sep + d)
		}
		ev := add(EvUpgrade, text, "upgrade."+string(u))
		if u == game.Overclock {
			ev.Fill = []string{"pct", pct(st.Rules.OverclockFrac)}
		}
		if u == game.ECC {
			livesSeen = true
		}
	}
	for _, d := range st.Daemons[min(p.daemons, len(st.Daemons)):] {
		name, desc := daemonText(d, st)
		text := "daemon: " + ui.Strong.Render(name) + " started"
		if d := strings.TrimSuffix(desc, "."); d != "" {
			text += ui.Muted.Render(ui.G.Sep + d)
		}
		add(EvDaemon, text, "daemon."+string(d))
	}

	// A purchase: the bytes went to Spent, and what changed says on what.
	if n.spent > p.spent {
		var it game.Item
		switch {
		case n.lives > p.lives:
			it, livesSeen = game.Fsck, true
		case n.ducks > p.ducks:
			it = game.Dup
		case n.rerolls > p.rerolls:
			it = game.Shuf
		case n.at > p.at:
			it = game.At
		}
		text := "spend: " + units.Bytes(n.spent-p.spent)
		if it != "" {
			name, desc := itemText(it, st.Rules)
			text = "spend: " + ui.Strong.Render(name) + ui.Muted.Render(ui.G.Sep+desc+ui.G.Sep) + units.Bytes(n.spent-p.spent)
		}
		add(EvSpend, text, "")
	}

	// The merge window: ducks refilled on the way in, a commit, leaving.
	if n.phase == game.PhaseMerge && p.phase != game.PhaseMerge && n.ducks > p.ducks {
		add(EvRefill, fmt.Sprintf("merge window: ducks refilled %s%d %s %s%d", ui.G.Duck, p.ducks, ui.G.Arrow, ui.G.Duck, n.ducks),
			"duck.refill", "n", strconv.Itoa(n.ducks))
	}
	if n.committed > p.committed {
		add(EvCommit, "commit: "+units.Bytes(n.committed-p.committed)+" banked"+
			ui.Muted.Render(ui.G.Sep+"bank ")+units.Bytes(m.Bank), "merge.commit")
	}
	if p.phase == game.PhaseMerge && n.phase != game.PhaseMerge && n.phase != game.PhaseCrashed {
		if n.uncommitted > 0 {
			add(EvLeave, "merge window closed"+ui.Muted.Render(ui.G.Sep)+units.Bytes(n.uncommitted)+
				ui.Muted.Render(" still uncommitted"), "merge.uncommitted", "bytes", units.Bytes(n.uncommitted))
		} else {
			add(EvLeave, "merge window closed", "merge.leave")
		}
	}

	// A door with modifiers just opened.
	if n.door != "" && (n.door != p.door || !n.opened.Equal(p.opened)) && st.Current != nil {
		if text, hook := doorMods(st.Current.Door); text != "" {
			add(EvDoor, fmt.Sprintf("door_%d: ", len(st.History)+1)+text, hook)
		}
	}

	// The open door's timer passed 5:00 or 1:00 left.
	if n.door != "" && n.door == p.door && n.opened.Equal(p.opened) && n.warned > p.warned {
		left := st.Current.Deadline.Sub(now)
		tone, hook := ui.Warn, "timer.warn.5m"
		if n.warned == 2 {
			tone, hook = ui.Danger, "timer.warn.1m"
		}
		add(EvTimer, tone.Render(ui.G.Timer+units.Countdown(left))+" left on this door", hook)
	}

	// Lives that changed for a reason no line above names.
	if !livesSeen && n.lives != p.lives && n.phase != game.PhaseCrashed && n.phase != game.PhaseCleared {
		add(EvLives, "lives: "+ui.Hearts(n.lives, st.MaxLives), "")
	}
	return out
}

// livesLost is the cost of a failed submit or a retreat: "-1 ♥, 3 left".
func livesLost(lost, left int) string {
	return fmt.Sprintf("-%d ", lost) + ui.Life.Render(ui.G.Life) + fmt.Sprintf(", %d left", left)
}

// doorMods is the line for a door's modifiers as it opens, the most
// dangerous first, and the flavor hook of that one; "" for a plain door.
func doorMods(d game.Door) (text, hook string) {
	var parts []string
	if d.Cursed {
		parts = append(parts, ui.Modifier(ui.ModCursed, "cursed: no ducks, 1 WA = crash"))
		hook = "door.cursed"
	}
	switch {
	case d.Mystery:
		parts = append(parts, ui.Modifier(ui.ModMystery, "mystery: title and tags hidden"))
		hook = cmp.Or(hook, "door.mystery")
	case d.Blind:
		parts = append(parts, ui.Modifier(ui.ModBlind, "blind: tags hidden"))
		hook = cmp.Or(hook, "door.blind")
	}
	return strings.Join(parts, ui.Muted.Render(ui.G.Sep)), hook
}

// regressionCleared says when a cleared regression comes back, or that it
// is retired: clearing one moves it to its next stage (game
// advanceRegression).
func regressionCleared(m *game.Meta, id catalog.ID, now time.Time) string {
	head := ui.Warn.Render(ui.G.Regression) + " regression cleared"
	for _, g := range m.Regressions {
		if g.Problem == id && g.Due.After(now) {
			days := int(math.Ceil(g.Due.Sub(now).Hours() / 24))
			return head + ui.Muted.Render(ui.G.Sep+"back in "+units.Plural(days, "day"))
		}
	}
	return head + ui.Muted.Render(ui.G.Sep+"retired")
}

// pct is a fraction as the flavor text writes it: 0.15 is "15%".
func pct(f float64) string {
	return strconv.FormatFloat(math.Round(f*1000)/10, 'f', -1, 64) + "%"
}

func (t *Tracker) problem(id catalog.ID) catalog.Problem {
	return Env{Lookup: t.Lookup}.problem(id)
}

// DuckRefused is the event for a quack the engine refuses on the open
// door: on a cursed door, where the Cat will not come, or with no ducks
// left. ok is false for any other refusal.
func DuckRefused(st *game.State, now time.Time) (ev Event, ok bool) {
	c := st.Current
	ev = Event{At: now, Kind: EvDuckRefuse, Seed: st.Seed ^ uint64(now.Unix())}
	switch {
	case c == nil || st.Phase != game.PhaseProblem:
		return ev, false
	case c.Door.Cursed:
		ev.Text, ev.Hook = "duck: none on a cursed door", "duck.cursed"
	case st.Ducks <= 0:
		ev.Text, ev.Hook = "duck: none left", "duck.empty"
	default:
		return ev, false
	}
	return ev, true
}

// ---- the strip -------------------------------------------------------------

// stripRows is how many rows the strip takes on a screen height rows tall:
// none before the boot's first event, then a fixed number, so the screen
// under it moves once and not with every event.
func (e Env) stripRows(height int) int {
	switch {
	case len(e.Events) == 0:
		return 0
	case height >= 34:
		return 3
	case height >= 20:
		return 2
	}
	return 1
}

// dstamp is an event's time in the boot as dmesg prints it, to a tenth of
// a second: "[  166.2]". One decimal, not the panic's three, leaves a
// 70-column flavor line room at 80 columns.
func dstamp(d time.Duration) string {
	return fmt.Sprintf("[%7.1f]", max(d, 0).Seconds())
}

// strip is the dmesg strip in rows lines of width cells, the newest at the
// bottom. The newest moment's events show first (all of them, as rows
// allow), then the newest event's flavor line under it, then older events,
// one row each.
func (e Env) strip(width, rows int) []string {
	evs := e.Events
	if rows <= 0 || len(evs) == 0 {
		return nil
	}
	pad := strings.Repeat(" ", len(dstamp(0))+1)
	line := func(ev Event, all bool) []string {
		at := ev.At.Sub(e.State.Started)
		if e.State.Started.IsZero() {
			at = 0
		}
		ls := ui.WrapN(ev.Text, max(width-len(pad), 10), 0)
		if !all {
			ls = ls[:1]
		}
		for i := range ls {
			pre := pad
			if i == 0 {
				pre = ui.Muted.Render(dstamp(at)) + " "
			}
			ls[i] = ui.Clip(pre+ls[i], width)
		}
		return ls
	}
	last := evs[len(evs)-1]
	cut := len(evs) - 1
	for cut > 0 && evs[cut-1].At.Equal(last.At) {
		cut--
	}
	var newest []string // the newest moment, oldest first
	for i := cut; i < len(evs); i++ {
		newest = append(newest, line(evs[i], true)...)
	}
	if len(newest) > rows {
		newest = newest[len(newest)-rows:]
	}
	// The flavor line is the newest event's that has one: a tip has none.
	var voice []string
	speaker := last
	for i := len(evs) - 1; i >= cut; i-- {
		if evs[i].Hook != "" {
			speaker = evs[i]
			break
		}
	}
	for _, v := range e.voice(speaker) {
		voice = append(voice, ui.Clip(pad+ui.Voice.Render(ui.Muted.Render(v)), width))
	}
	voice = voice[:min(len(voice), rows-len(newest))]
	var older []string
	for i := cut - 1; i >= 0 && len(older)+len(newest)+len(voice) < rows; i-- {
		older = append(line(evs[i], false), older...)
	}
	out := slices.Concat(older, newest, voice)
	return append(make([]string, rows-len(out)), out...)
}

// voice is an event's flavor line, filled; nil when it has none.
func (e Env) voice(ev Event) []string {
	if ev.Hook == "" {
		return nil
	}
	s := fl.Story(ev.Hook, ev.Seed)
	if s == "" {
		return nil
	}
	return strings.Split(flavor.Fill(s, ev.Fill...), "\n")
}

// ---- onboarding --------------------------------------------------------------

// tips is the one-time onboarding line this look shows, if any (UX "The
// first 10 minutes"): each comes the first time its mechanic does, as a
// dmesg line and never a dialog, and the app records its Seen id so it
// does not come back. One idea at a time, as Dead Cells teaches: a look
// shows at most one, and only beside at most one other event (out), so
// the strip's two rows hold both. A tip that has to wait comes the next
// time its moment does. Each is one line at 80 columns. A trial is learn
// mode, with rules of its own, so it teaches nothing here.
func tips(st *game.State, m *game.Meta, now time.Time, out []Event) []Event {
	if st.Trial != nil || len(out) > 1 {
		return nil
	}
	has := func(kind string) bool {
		return slices.ContainsFunc(out, func(e Event) bool { return e.Kind == kind })
	}
	for _, t := range []struct {
		id, text string
		due      bool
	}{
		{"tip.doors", "doors: aimed at your rating, and the aim adapts as you play",
			st.Phase == game.PhaseDoors && !st.PendingUpgrade},
		{"tip.door", ":w judges samples" + ui.G.Sep + ":Submit all tests, a fail costs " + ui.Life.Render(ui.G.Life) + ui.G.Sep + ":q! gives up",
			st.Phase == game.PhaseProblem && st.Current != nil},
		{"tip.upgrade", "upgrades last the boot" + ui.G.Sep + "AC under half the timer (-O3): a second pick",
			st.PendingUpgrade},
		{"tip.bytes", "bytes: a crash loses the uncommitted" + ui.G.Sep + "commit them at a merge window",
			st.Phase == game.PhaseDoors && !st.PendingUpgrade && st.Uncommitted > 0},
		{"tip.fail", "a failed submit costs " + ui.Life.Render(ui.G.Life) + ", a CE nothing" + ui.G.Sep + "red samples need :Submit!",
			has(EvVerdict)},
		{"tip.duck", "ducks: the 1st quack shows hidden tags, the 2nd the technique",
			has(EvQuack)},
		{"tip.regression", ui.Warn.Render(ui.G.Regression) + " a door that crashed you returns; each clear sends it further out",
			st.Phase == game.PhaseDoors && slices.ContainsFunc(st.Doors, func(d game.Door) bool { return d.Regression })},
	} {
		if t.due && !game.HasSeen(m, t.id) {
			return []Event{{At: now, Kind: EvTip, Text: t.text, Seen: t.id}}
		}
	}
	return nil
}

// DoorLines is the strip in the problem pane, rows lines of width cells
// at most, wrapped and with no stamp, which the narrow pane has no room
// for: the newest moment since the open door opened, its events, then its
// one-time line, then the flavor line. Nil when the door has had none.
func (e Env) DoorLines(width, rows int) []string {
	c := e.State.Current
	if c == nil || rows <= 0 || len(e.Events) == 0 {
		return nil
	}
	last := e.Events[len(e.Events)-1]
	if last.At.Before(c.Opened) {
		return nil
	}
	var real, tips, voice []string
	speaker := Event{}
	for _, ev := range e.Events {
		switch {
		case !ev.At.Equal(last.At):
		case ev.Kind == EvTip:
			tips = append(tips, ui.WrapN(ev.Text, width, 0)...)
		default:
			real = append(real, ui.WrapN(ev.Text, width, 0)...)
			if ev.Hook != "" {
				speaker = ev
			}
		}
	}
	for _, v := range e.voice(speaker) {
		for _, l := range ui.WrapN(v, width, 0) {
			voice = append(voice, ui.Voice.Render(ui.Muted.Render(l)))
		}
	}
	out := slices.Concat(real, tips, voice)
	return out[:min(len(out), rows)]
}
