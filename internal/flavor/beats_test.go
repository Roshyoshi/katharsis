package flavor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

func TestBeatsHaveText(t *testing.T) {
	titles := map[string]string{}
	for i, list := range [][]beat{scenes, triggered, milestones} {
		atEnd := i > 0 // triggered and milestones show at a boot end
		for _, b := range list {
			if b.title == "" {
				t.Errorf("%s has no title", b.id)
			}
			if other, dup := titles[b.title]; dup {
				t.Errorf("%s and %s share the title %q", b.id, other, b.title)
			}
			titles[b.title] = b.id
			id := b.id
			if id == capabilityBeat {
				id += ":CAP_DP"
			} else if atEnd && b.due == nil {
				t.Errorf("%s fires at a boot end but has no trigger", b.id)
			}
			ls := BeatLines(id)
			if len(ls) == 0 || slices.Contains(ls, "") {
				t.Errorf("BeatLines(%q) = %q", id, ls)
			}
			for _, l := range ls {
				if strings.ContainsAny(l, "{}") {
					t.Errorf("BeatLines(%q) leaves a placeholder: %q", id, l)
				}
			}
		}
	}
	if BeatLines("no.such.beat") != nil || BeatLines("story.prologue:x") != nil {
		t.Error("an unknown beat has text")
	}
	// A family keys its pick on the argument, so two capabilities differ.
	if slices.Equal(BeatLines(capabilityBeat+":CAP_DP"), BeatLines(capabilityBeat+":CAP_GRAPH")) &&
		slices.Equal(BeatLines(capabilityBeat+":CAP_DP"), BeatLines(capabilityBeat+":CAP_DSU")) {
		t.Error("capability beats do not vary with the capability")
	}
}

// crashed and clearedAt build boot ends for the pacing tests.
func crashed(boot int, lost int64) *game.State {
	return &game.State{Boot: boot, Phase: game.PhaseCrashed, Rules: rules.Standard(),
		Crash: &game.Crash{Cause: game.TimedOut, Lost: lost}}
}

func clearedAt(boot, prestige int) *game.State {
	return &game.State{Boot: boot, Phase: game.PhaseCleared, Prestige: prestige, Rules: rules.Standard()}
}

// seenAt is a Meta whose story so far is ids, each shown at its boot.
func seenAt(boots int, ids map[string]int) *game.Meta {
	m := &game.Meta{Boots: boots}
	for id, b := range ids {
		m.Seen = append(m.Seen, game.SeenScene{ID: id, Boot: b})
	}
	slices.SortFunc(m.Seen, func(a, b game.SeenScene) int { return a.Boot - b.Boot })
	return m
}

func next(st *game.State, m *game.Meta, gap int) string {
	id, ok := NextBeat(st, m, gap)
	if ok != (id != "") {
		panic("NextBeat: ok disagrees with the id")
	}
	return id
}

func TestNextBeatTriggered(t *testing.T) {
	trial := crashed(3, 50)
	trial.Trial = &game.TrialState{}
	trialClear := clearedAt(9, 0)
	trialClear.Trial = &game.TrialState{}
	tests := []struct {
		name string
		st   *game.State
		m    *game.Meta
		want string
	}{
		{"first crash", crashed(1, 40), seenAt(1, nil), "story.first-crash"},
		// The Cat says you left bytes on the problem; with none it waits.
		{"first crash with nothing lost", crashed(1, 0), seenAt(1, nil), ""},
		{"a trial crash leaves no dump", trial, seenAt(3, nil), ""},
		{"second crash", crashed(2, 40), seenAt(2, map[string]int{"story.first-crash": 1}), ""},
		{"no boot end", &game.State{Boot: 1, Phase: game.PhaseDoors}, seenAt(1, nil), ""},
		{"nil state", nil, seenAt(1, nil), ""},
		{"the clear that unlocks Silver", clearedAt(9, 0), &game.Meta{Boots: 9, Prestige: 1}, "reveal.prestige.1"},
		{"a clear without an unlock", clearedAt(9, 0), &game.Meta{Boots: 9}, ""},
		{"a trial clear", trialClear, &game.Meta{Boots: 9, Prestige: 1}, ""},
		// A save from before the story catches up one reveal per clear,
		// in ring order.
		{"catch-up, lowest first", clearedAt(40, 3), &game.Meta{Boots: 40, Prestige: 3}, "reveal.prestige.1"},
		{"catch-up, next", clearedAt(41, 3), seenAt(41, map[string]int{"reveal.prestige.1": 40}), "reveal.prestige.2"},
		// No win cue on a crash (retention GR3), even one that is owed.
		{"reveal never on a crash", crashed(12, 40),
			&game.Meta{Boots: 12, Prestige: 2, Seen: []game.SeenScene{{ID: "story.first-crash", Boot: 1}}}, ""},
		{"final clear", clearedAt(80, game.MaxPrestige), seenAt(80, map[string]int{
			"reveal.prestige.1": 10, "reveal.prestige.2": 20, "reveal.prestige.3": 30, "reveal.prestige.4": 40,
		}), "reveal.final-clear"},
	}
	tests[10].m.Prestige = 3
	tests[12].m.Prestige = game.MaxPrestige
	for _, tc := range tests {
		// A huge gap keeps milestones out, so only triggered beats show.
		if got := next(tc.st, tc.m, 1<<20); got != tc.want {
			t.Errorf("%s: NextBeat = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestNextBeatOnePerBootEnd: once a beat is shown at a boot end, nothing
// else shows there, however much is owed.
func TestNextBeatOnePerBootEnd(t *testing.T) {
	m := &game.Meta{Boots: 30, Prestige: 1, Rated: 500, Capabilities: []string{"CAP_DP"}}
	st := clearedAt(30, 0)
	for gap := range 3 {
		mm := *m
		mm.Seen = nil
		id := next(st, &mm, gap)
		if id != "reveal.prestige.1" {
			t.Fatalf("gap %d: first beat %q, want the reveal", gap, id)
		}
		game.MarkSeen(&mm, id)
		if again := next(st, &mm, gap); again != "" {
			t.Errorf("gap %d: a second beat %q at the same boot end", gap, again)
		}
		// A scene earlier in the same boot is not a boot-end beat.
		mm.Seen = []game.SeenScene{{ID: "story.first-merge", Boot: 30}}
		if id := next(st, &mm, 0); id != "reveal.prestige.1" {
			t.Errorf("gap %d: a scene in the boot blocked the boot end: %q", gap, id)
		}
	}
}

// TestNextBeatGap: a milestone fills the boot end after gap boot ends in a
// row without a beat, and not before; the gap is a setting.
func TestNextBeatGap(t *testing.T) {
	for _, gap := range []int{0, 1, 3, 8, 13} {
		const last = 20 // the boot of the newest beat
		m := seenAt(last, map[string]int{"story.prologue": 1, "story.first-crash": 2, "story.first-boss": last})
		m.Rated = 12 // earns milestone.doors.10
		for boot := last + 1; boot <= last+gap+1; boot++ {
			m.Boots = boot
			got := next(crashed(boot, 0), m, gap)
			if dry := boot - last - 1; dry < gap && got != "" {
				t.Errorf("gap %d: boot %d, %d dry boot ends: %q, want nothing yet", gap, boot, dry, got)
			} else if dry == gap && got != "milestone.doors.10" {
				t.Errorf("gap %d: boot %d fills the gap with %q, want milestone.doors.10", gap, boot, got)
			}
		}
		if got := next(crashed(last+gap+1, 0), &game.Meta{Boots: last + gap + 1, Seen: m.Seen}, gap); got != "" {
			t.Errorf("gap %d: a milestone nobody earned: %q", gap, got)
		}
	}
	m := &game.Meta{Boots: 90, Rated: 500}
	if got := next(crashed(90, 0), m, -1); got != "" {
		t.Errorf("a negative gap still shows milestone %q", got)
	}
}

func TestNextBeatMilestoneOrder(t *testing.T) {
	m := &game.Meta{Boots: 50, Rated: 120, Capabilities: []string{"CAP_GRAPH", "CAP_DP"},
		Libs: []game.Lib{{Name: "dsu"}}}
	for range 10 {
		m.Solves = append(m.Solves, game.Solve{Upsolve: true})
	}
	var got []string
	for boot := 50; boot < 70; boot++ {
		m.Boots = boot
		if id := next(crashed(boot, 0), m, 0); id != "" {
			got = append(got, id)
			game.MarkSeen(m, id)
		}
	}
	// Table order, the capabilities in the order earned; the gate beats
	// are past (rated doors beyond the gate count as open, not half).
	want := []string{"milestone.doors.10", "milestone.upsolve.5", "milestone.doors.25",
		"milestone.doors.50", "milestone.gate.open", "milestone.lib.1",
		"milestone.capability:CAP_GRAPH", "milestone.capability:CAP_DP",
		"milestone.doors.75", "milestone.doors.100"}
	if !slices.Equal(got, want) {
		t.Errorf("milestones in order:\n got %q\nwant %q", got, want)
	}
	// The gate beats are about prestige 0; past the first unlock, none.
	past := &game.Meta{Boots: 5, Rated: 100, Prestige: 1}
	for _, b := range []func(*game.State, *game.Meta) bool{gateHalf, gateOpen} {
		if b(crashed(5, 0), past) {
			t.Error("a gate beat is due after the gate opened")
		}
	}
	if !gateHalf(crashed(5, 0), &game.Meta{Rated: 30}) || gateHalf(crashed(5, 0), &game.Meta{Rated: 29}) {
		t.Error("gate.half is not due at exactly half of CalibAfter 60")
	}
	// The gate beats count doors the way the gate does: an older save
	// whose per-topic attempts outran Meta.Rated is through the gate.
	old := &game.Meta{Rated: 10, TopicAttempts: map[string]int{"dp": 40, "graphs": 25}}
	if !gateOpen(crashed(5, 0), old) || gateHalf(crashed(5, 0), old) {
		t.Error("the gate beats disagree with the gate's own count")
	}
}

// TestEveryBeatReachable plays a long, steady career with a beat allowed at
// every boot end and checks that each triggered beat and milestone shows
// exactly once: no trigger is dead, and none repeats.
func TestEveryBeatReachable(t *testing.T) {
	m := &game.Meta{}
	caps := []string{"CAP_DP", "CAP_GRAPH", "CAP_DSU"}
	shown := map[string]int{}
	for boot := 1; boot <= 700; boot++ {
		m.Boots = boot
		m.Rated++
		if boot%3 == 0 {
			m.Solves = append(m.Solves, game.Solve{Upsolve: true})
		}
		if boot == 40 {
			m.Libs = append(m.Libs, game.Lib{Name: "dsu"})
		}
		if boot == 60 {
			m.TopicAttempts = map[string]int{"dp": rules.Standard().MasteryAttempts}
			m.TopicRating = map[string]float64{"dp": 1500}
		}
		if boot == 80 {
			m.Retired++ // the first regression re-solved at its last stage
		}
		if boot%50 == 0 && len(m.Capabilities) < len(caps) {
			m.Capabilities = append(m.Capabilities, caps[len(m.Capabilities)])
		}
		st := crashed(boot, 30)
		if boot%7 == 0 { // every seventh boot clears; past the gate, a clear unlocks
			st = clearedAt(boot, m.Prestige)
			if m.Rated >= st.Rules.CalibAfter && m.Prestige < game.MaxPrestige {
				m.Prestige++
			}
		}
		if id := next(st, m, 0); id != "" {
			shown[id]++
			game.MarkSeen(m, id)
		}
	}
	var want []string
	for _, b := range append(slices.Clone(triggered), milestones...) {
		if b.id == capabilityBeat {
			for _, c := range caps {
				want = append(want, capabilityBeat+":"+c)
			}
			continue
		}
		want = append(want, b.id)
	}
	for _, id := range want {
		if shown[id] != 1 {
			t.Errorf("%s shown %d times, want once", id, shown[id])
		}
	}
	if len(shown) != len(want) {
		t.Errorf("shown %d distinct beats, want %d: %v", len(shown), len(want), shown)
	}
}

// TestBeatTriggersIgnoreClocks is retention guardrail GR2 (no story keyed
// to a date) and RT3/RT6 (no story for bytes), checked on the source: the
// triggers never read a time or a balance.
func TestBeatTriggersIgnoreClocks(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "beats.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, im := range f.Imports {
		if p, _ := strconv.Unquote(im.Path.Value); p == "time" {
			t.Error("beats.go imports time")
		}
	}
	banned := map[string]bool{
		"At": true, "Due": true, "Started": true, "Opened": true, "Closed": true, "Deadline": true,
		"Now": true, "Weekday": true, "Date": true, "Bank": true, "LifetimeBytes": true, "Committed": true,
		"Uncommitted": true, "Seed": true,
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && banned[s.Sel.Name] {
			t.Errorf("beats.go reads .%s in a trigger", s.Sel.Name)
		}
		return true
	})
}

func TestStoryLog(t *testing.T) {
	m := &game.Meta{}
	for _, s := range []struct {
		boot int
		id   string
	}{{0, "story.prologue"}, {1, "story.first-crash"}, {1, "not.a.beat"}, {9, "milestone.capability:CAP_DP"}} {
		m.Boots = s.boot
		game.MarkSeen(m, s.id)
	}
	log := StoryLog(m)
	var ids []string
	for _, e := range log {
		ids = append(ids, e.ID)
		if e.Title == "" || len(e.Lines) == 0 {
			t.Errorf("log entry %+v", e)
		}
	}
	if want := []string{"story.prologue", "story.first-crash", "milestone.capability:CAP_DP"}; !slices.Equal(ids, want) {
		t.Errorf("StoryLog ids %q, want %q", ids, want)
	}
	if l := log[2]; l.Boot != 9 || !strings.Contains(l.Title, "CAP_DP") || !strings.Contains(strings.Join(l.Lines, " "), "CAP_DP") {
		t.Errorf("capability entry %+v", l)
	}
}

// TestPacingInTheSim is RT6's acceptance check on the game's own balance
// sim: from an empty Meta, for a weaker and a stronger player, no boot end
// shows two beats, and through boot 100 no run of more than DefaultBeatGap
// boots passes without a new one. The screens' moments (prologue, first
// merge, first boss...) are modelled from the phases the sim plays.
func TestPacingInTheSim(t *testing.T) {
	if testing.Short() {
		t.Skip("plays 200 boots")
	}
	const boots = 100
	pool := sim.SyntheticPool(20000, 1)
	for _, rating := range []float64{1000, 1600} {
		m := &game.Meta{}
		ended := map[int]bool{}
		sim.Run(sim.Config{
			Rules: rules.Standard(), Pool: pool, Player: sim.DefaultPlayer(rating),
			Boots: boots, Seed: 7, Prestige: -1, Meta: m,
			Check: func(s *game.State, m *game.Meta, _ game.Action, _ []game.Effect, err error) {
				if err != nil || s == nil {
					return
				}
				scenesAt(s, m)
				if (s.Phase == game.PhaseCrashed || s.Phase == game.PhaseCleared) && !ended[s.Boot] {
					ended[s.Boot] = true
					if id, ok := NextBeat(s, m, DefaultBeatGap); ok {
						game.MarkSeen(m, id)
					}
				}
			},
		})
		perBoot := map[int]int{} // boot-end beats per boot
		beatIn := map[int]bool{}
		for _, s := range m.Seen {
			beatIn[s.Boot] = true
			if endBeat(s.ID) {
				perBoot[s.Boot]++
			}
		}
		for b, n := range perBoot {
			if n > 1 {
				t.Errorf("rating %.0f: boot %d ended with %d beats", rating, b, n)
			}
		}
		run, worst, at := 0, 0, 0
		for b := 1; b <= boots; b++ {
			if beatIn[b] {
				run = 0
				continue
			}
			if run++; run > worst {
				worst, at = run, b
			}
		}
		t.Logf("rating %.0f: %d beats through boot %d, longest dry run %d (to boot %d)",
			rating, len(m.Seen), boots, worst, at)
		if worst > DefaultBeatGap {
			t.Errorf("rating %.0f: %d boots in a row without a beat, ending at boot %d; seen %+v",
				rating, worst, at, m.Seen)
		}
	}
}

// scenesAt marks the scenes a player would meet in st, as the app will
// when it shows them.
func scenesAt(st *game.State, m *game.Meta) {
	mark := func(id string) { game.MarkSeen(m, id) }
	if st.Boot == 1 {
		mark("story.prologue")
	}
	switch st.Phase {
	case game.PhaseDoors:
		if st.Boot == 1 && st.Room == 0 {
			mark("door.first.inscription")
		}
		for _, d := range st.Doors {
			switch {
			case d.Boss == game.FinalBoss:
				mark("reveal.final-boss")
			case d.Boss != "":
				mark("story.first-boss")
			case d.Regression:
				mark("story.first-regression")
			}
		}
	case game.PhaseMerge:
		mark("story.first-merge")
		if len(st.DaemonOffers) > 0 {
			mark("daemon.intro")
		}
	}
}

// TestTopicMastered: the mastery milestone follows the engine's mastery
// (D24): enough rated attempts on a topic with a rating of its own, never
// mixed, and nothing to tell under a preset with mastery off.
func TestTopicMastered(t *testing.T) {
	st := crashed(20, 0)
	need := st.Rules.MasteryAttempts
	if need <= 0 {
		t.Fatal("the standard rules have mastery off")
	}
	rated := map[string]float64{"dp": 1500, game.Mixed: 1500}
	for _, c := range []struct {
		name string
		m    game.Meta
		want bool
	}{
		{"short of it", game.Meta{TopicAttempts: map[string]int{"dp": need - 1}, TopicRating: rated}, false},
		{"mastered", game.Meta{TopicAttempts: map[string]int{"dp": need}, TopicRating: rated}, true},
		{"mixed", game.Meta{TopicAttempts: map[string]int{game.Mixed: need * 3}, TopicRating: rated}, false},
		{"no rating", game.Meta{TopicAttempts: map[string]int{"graphs": need}, TopicRating: rated}, false},
	} {
		if got := topicMastered(st, &c.m); got != c.want {
			t.Errorf("%s: due %v, want %v", c.name, got, c.want)
		}
	}
	off := crashed(20, 0)
	off.Rules.MasteryAttempts = 0
	if topicMastered(off, &game.Meta{TopicAttempts: map[string]int{"dp": 99}, TopicRating: rated}) {
		t.Error("due with mastery off")
	}
}

// The first regression retired (Meta.Retired, RT6) is a milestone: due
// from the first, once shown never again.
func TestRegressionRetiredBeat(t *testing.T) {
	const id = "milestone.retired.1"
	var due func(*game.State, *game.Meta) bool
	for _, b := range milestones {
		if b.id == id {
			due = b.due
		}
	}
	if due == nil {
		t.Fatalf("no %s milestone", id)
	}
	st := crashed(30, 0)
	if due(st, &game.Meta{}) || !due(st, &game.Meta{Retired: 1}) {
		t.Errorf("%s: due with none retired, or not with one", id)
	}
	if ls := BeatLines(id); len(ls) != 2 || !strings.HasSuffix(ls[1], "?") {
		t.Errorf("%s lines %q, want two ending on a question", id, ls)
	}
}

// teaserWidth is the room the end screens give a beat's first line at 80
// columns: the kept block's teaser is 80 less its label column and
// " · b read on" (screens/boot teaser, crash_end_80x24.golden).
const teaserWidth = 59

// TestMilestoneLines holds the milestones to text.md's rules for them: the
// first line is the teaser, so it fits whole at 80 columns, and the last
// ends on a small question a later beat answers (retention RT6).
func TestMilestoneLines(t *testing.T) {
	for _, b := range milestones {
		h, ok := story[b.id]
		if !ok {
			t.Errorf("%s has no text", b.id)
			continue
		}
		passages := [][]string{h.entries} // a block is one passage
		if h.kind != block {
			passages = nil
			for _, e := range h.entries {
				passages = append(passages, strings.Split(e, "\n"))
			}
		}
		for _, ls := range passages {
			if w := ansi.StringWidth(widest(t, b.id, ls[0])); w > teaserWidth {
				t.Errorf("%s: teaser %q is %d columns filled, want <= %d", b.id, ls[0], w, teaserWidth)
			}
			if last := ls[len(ls)-1]; !strings.HasSuffix(last, "?") {
				t.Errorf("%s: %q does not end on a question", b.id, last)
			}
		}
	}
}

// TestRevealTeasers: a triggered beat's first line is the end screens'
// teaser too, so it fits whole at 80 columns. The Cat's first crash is the
// one exception: the app plays it by itself (screens/boot beat).
func TestRevealTeasers(t *testing.T) {
	for _, b := range triggered {
		if b.id == "story.first-crash" {
			continue
		}
		ls := BeatLines(b.id)
		if len(ls) == 0 {
			t.Errorf("%s has no text", b.id)
			continue
		}
		if w := ansi.StringWidth(widest(t, b.id, ls[0])); w > teaserWidth {
			t.Errorf("%s: teaser %q is %d columns, want <= %d", b.id, ls[0], w, teaserWidth)
		}
	}
}
