package setuid

import (
	"slices"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// The conditions' helpers (achievements.md §(d)): B is v.s, the boot
// checked when it counts (Counts), nil otherwise.
//
//	mapped   B is a three-act boot (its rules have Nodes)
//	clear    B was cleared, not to an ending: Phase cleared, Ending ""
//	full     mapped and clear
//	beat k   B's cleared door of boss k (door)
//	burns a  a's failed hidden-test submits

// mapped reports whether the boot checked is a three-act boot.
func (v *view) mapped() bool { return v.s != nil && len(v.s.Rules.Nodes) > 0 }

// clear reports whether the boot checked was cleared to its last boss.
func (v *view) clear() bool {
	return v.s != nil && v.s.Phase == game.PhaseCleared && v.s.Ending == ""
}

// full reports whether the boot checked is a three-act boot cleared.
func (v *view) full() bool { return v.mapped() && v.clear() }

// door is the boot's cleared door of boss k and its index in History, ok
// false when it has none.
func (v *view) door(k string) (a game.Attempt, i int, ok bool) {
	if v.s == nil {
		return game.Attempt{}, 0, false
	}
	for i, a := range v.s.History {
		if a.Outcome == game.Cleared && a.Door.Boss == k {
			return a, i, true
		}
	}
	return game.Attempt{}, 0, false
}

// beat reports whether the boot checked defeated boss k.
func (v *view) beat(k string) bool {
	_, _, ok := v.door(k)
	return ok
}

// every reports whether ok holds for every door of the boot checked.
func (v *view) every(ok func(a game.Attempt) bool) bool {
	return v.s != nil && !slices.ContainsFunc(v.s.History, func(a game.Attempt) bool { return !ok(a) })
}

// burns counts a's failed hidden-test submits under r: the submits judged
// neither AC, nor a free verdict (CE), nor failed on a sample. Under ECC
// one costs clock, not cycles, and still counts; Off-by-One's double burn
// counts once. An attempt saved before its submits were logged (Events)
// counts every failed submit, CE included, so it can only miss a bit.
func burns(r *rules.Rules, a game.Attempt) int {
	if len(a.Events) == 0 {
		return a.Failed
	}
	n := 0
	for _, e := range a.Events {
		if e.Kind == "submit" && e.Verdict != game.Accepted && !e.SampleFailed && !slices.Contains(r.FreeVerdicts, string(e.Verdict)) {
			n++
		}
	}
	return n
}

// clean reports whether no door of the boot checked burned a submit.
func (v *view) clean() bool {
	return v.every(func(a game.Attempt) bool { return burns(&v.s.Rules, a) == 0 })
}

// ducks counts the ducks the boot checked used.
func (v *view) ducks() int {
	n := 0
	for _, a := range v.s.History {
		n += a.Quacks
	}
	return n
}

// node is the map node the door a played in, ok false off the map.
func (v *view) node(a game.Attempt) (rules.Biome, bool) {
	if a.Biome < 0 || a.Biome >= len(v.s.Biomes) {
		return rules.Biome{}, false
	}
	return v.s.Rules.Node(v.s.Biomes[a.Biome].Node)
}

// took reports whether the boot checked played a node pick allows.
func (v *view) took(pick func(n rules.Biome) bool) bool {
	if !v.mapped() {
		return false
	}
	for _, b := range v.s.Biomes {
		if n, ok := v.s.Rules.Node(b.Node); ok && pick(n) {
			return true
		}
	}
	return false
}

// samples is how many sample runs door i (a) of the boot checked made, ok
// false when nobody counted: a boot saved before Attempt.Samples, whose
// door a backfill finds no events for.
func (v *view) samples(i int, a game.Attempt) (n int, ok bool) {
	switch {
	case v.s.CountsSamples:
		return a.Samples, true
	case v.backfill == nil || v.backfill.Since.IsZero() || a.Opened.Before(v.backfill.Since):
		return 0, false
	}
	return v.backfill.Runs[Door{v.s.Boot, i}], true
}

// capped reports whether the boot checked held a full cycle pool between
// doors: the engine's mark, or for a boot saved before it, a door opened
// at the cap.
func (v *view) capped() bool {
	if !v.mapped() {
		return false
	}
	return v.s.Capped || slices.ContainsFunc(v.s.History, func(a game.Attempt) bool { return a.CyclesAtOpen >= v.s.MaxCycles })
}

// speed reports whether completion reaches pct for the first time (bit
// first, the percentage's own, not set yet: the check that finds it is the
// first) under bound of boot time, the boot in play included.
func (v *view) speed(first string, pct int, bound time.Duration) bool {
	if IsSet(v.m, first) {
		return false
	}
	if got, _ := Completion(v.m); got < pct {
		return false
	}
	t := v.m.Record.BootTime
	if b := v.boot; b != nil && b.Trial == nil && !ended(b) && b.Boot > v.m.Record.Folded {
		t += b.Uptime(v.now)
	}
	return t < bound
}

// lastMinute reports whether a cleared its door inside the last d of its
// own clock, not in overtime.
func lastMinute(a game.Attempt, d time.Duration) bool {
	left := a.Deadline.Sub(a.Closed)
	return !a.Overtime && left >= 0 && left < d
}

// fold folds the ended, counted boot s into r (Fold's work).
func fold(r *game.Record, s *game.State) {
	mapped := len(s.Rules.Nodes) > 0
	for _, a := range s.History {
		for _, e := range a.Events {
			if e.Kind == "submit" && e.Verdict != "" {
				add(&r.Verdicts, e.Verdict)
			}
		}
		if a.LastVerdict != "" {
			add(&r.Verdicts, a.LastVerdict)
		}
		if a.Outcome != game.Cleared {
			continue
		}
		if k := a.Door.Boss; k != "" && k != game.TrialBoss {
			add(&r.Bosses, k)
		}
		if mapped && a.Biome >= 0 && a.Biome < len(s.Biomes) {
			if b := s.Biomes[a.Biome]; b.Node != "" && b.Kind != rules.Boss {
				add(&r.Nodes, b.Node)
			}
		}
	}
	cleared := s.Phase == game.PhaseCleared
	clear := cleared && s.Ending == ""
	if clear {
		for _, d := range s.Daemons {
			add(&r.Daemons, d)
		}
	}
	if s.Unlocked > 0 {
		addSorted(&r.Unlocked, s.Unlocked)
	}
	if cleared && s.Ending != "" && mapped {
		add(&r.Endings, s.Ending)
	}
	if clear && mapped {
		addSorted(&r.Cleared, s.Prestige)
		if slices.ContainsFunc(s.Biomes, func(b game.BiomeState) bool {
			n, ok := s.Rules.Node(b.Node)
			return ok && n.Skip
		}) {
			r.Skips++
		}
		if s.Nightly() {
			r.Nightly++
		}
	}
}

// add appends x to xs unless it is there.
func add[T comparable](xs *[]T, x T) {
	if !slices.Contains(*xs, x) {
		*xs = append(*xs, x)
	}
}

// addSorted inserts x into the ascending xs unless it is there.
func addSorted(xs *[]int, x int) {
	if i, found := slices.BinarySearch(*xs, x); !found {
		*xs = slices.Insert(*xs, i, x)
	}
}
