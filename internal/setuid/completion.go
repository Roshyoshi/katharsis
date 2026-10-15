package setuid

import (
	"maps"
	"slices"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

// Completion (achievements.md §(e)): Hollow Knight's split on katharsis's
// content. 100% is the divisions done, Bronze to Advanced; the 12% beyond
// is rings -2 and -3 and the secret ending, so one point of 112% sits
// behind a ???. It counts no bytes and no purchases.
const (
	Full = 100
	Pure = 112
)

// Part is one line of the breakdown: Have of Of things, worth Points of
// Max.
type Part struct {
	Name        string // "bosses", "map nodes", "capabilities", "rings", "firsts", "beyond"
	Have, Of    int
	Points, Max int
}

// Completion is m's percentage, 0 to Pure, and its parts in the screen's
// order. It reads Meta and its Record, never the bits.
func Completion(m *game.Meta) (pct int, parts []Part) {
	b, r := rules.Setuid(), &m.Record
	part := func(name string, have, of, each int) Part {
		return Part{Name: name, Have: have, Of: of, Points: have * each, Max: of * each}
	}
	caps := slices.Sorted(maps.Values(game.TopicCapabilities))

	rings := 0
	for k := 1; k <= b.RingTop; k++ {
		if slices.Contains(r.Unlocked, k) {
			rings++
		}
	}
	if slices.ContainsFunc(r.Cleared, func(p int) bool { return p >= b.RingTop }) {
		rings++
	}
	// The firsts: a dump recovered, a regression retired, a lib saved, a
	// topic mastered, a skip clear, a nightly clear.
	first := []bool{r.Recovered > 0, m.Retired > 0, len(m.Libs) > 0, mastered(m) > 0, r.Skips > 0, r.Nightly > 0}
	firsts := 0
	for _, ok := range first {
		if ok {
			firsts++
		}
	}
	beyond := 0
	for k := b.RingTop + 1; k <= b.BeyondTop; k++ {
		if slices.Contains(r.Unlocked, k) {
			beyond++
		}
	}
	if slices.ContainsFunc(r.Cleared, func(p int) bool { return p >= b.BeyondTop }) {
		beyond++
	}
	if slices.Contains(r.Endings, game.EndingWake) {
		beyond++
	}

	parts = []Part{
		part("bosses", have(r.Bosses, Bosses), len(Bosses), b.BossPoints),
		part("map nodes", have(r.Nodes, Nodes), len(Nodes), b.NodePoints),
		part("capabilities", have(m.Capabilities, caps), len(caps), b.CapPoints),
		part("rings", rings, b.RingTop+1, b.RingPoints),
		part("firsts", firsts, len(first), b.FirstPoints),
		part("beyond", beyond, b.BeyondTop-b.RingTop+2, b.BeyondPoints),
	}
	for _, p := range parts {
		pct += p.Points
	}
	return pct, parts
}

// ShowsPercent reports whether the screen shows the percentage: once a
// three-act boot was cleared (Silksong's "not until you go beyond Act 2").
// Before that its header counts bits set.
func ShowsPercent(m *game.Meta) bool { return len(m.Record.Cleared) > 0 }

// Bosses are the six bosses the bits count: BossKeys and the final, the
// Trial aside.
var Bosses = append(slices.Clone(game.BossKeys), game.FinalBoss)

// Nodes are the map nodes a room can be cleared in, as the cartographer
// counts them: every node of the built-in map but the bosses, the skip and
// the hidden ones (17).
var Nodes = func() []string {
	var ids []string
	for _, n := range rules.MapNodes() {
		if n.Boss == "" && !n.Skip && !n.Hidden {
			ids = append(ids, n.ID)
		}
	}
	return ids
}()

// Topics are the topics a mastery counts: every catalog topic but mixed.
var Topics = slices.DeleteFunc(slices.Clone(catalog.Topics), func(t string) bool { return t == catalog.Mixed })

// mastered counts the topics m has mastered: rules.MasteryAttempts rated
// attempts on each, as milestone.mastered.1 counts them.
func mastered(m *game.Meta) int {
	need := rules.Standard().MasteryAttempts
	n := 0
	for _, t := range Topics {
		if m.TopicAttempts[t] >= need {
			n++
		}
	}
	return n
}
