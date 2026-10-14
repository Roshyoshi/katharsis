package app

import (
	"slices"

	"deadcases/internal/flavor"
	"deadcases/internal/game"
	"deadcases/internal/screens/meta"
	"deadcases/internal/store"
)

// bootRow maps a stored boot and its attempts onto the git log row. Display
// strings (titles, biome names) are resolved here so the screen stays a
// pure view.
func (m *Model) bootRow(b store.BootSummary, atts []store.AttemptRecord) meta.BootRow {
	row := meta.BootRow{
		Boot:     b.Boot,
		Seed:     b.Seed,
		Preset:   b.Rules,
		Prestige: b.Prestige,
		Started:  b.Started.Local(),
		Uptime:   b.Uptime,
		Cleared:  b.Outcome == store.BootCleared,
		Cause:    b.Cause,
		Banked:   b.Committed,
	}
	if b.LastDoor != "" {
		row.Problem = m.title(b.LastDoor)
	}
	// Attempts come newest first; the screen lists them in play order.
	for i := len(atts) - 1; i >= 0; i-- {
		a := atts[i]
		row.Attempts = append(row.Attempts, m.attemptRow(a.Attempt))
	}
	if n := len(atts); n > 0 && !row.Cleared {
		last := atts[0].Attempt
		row.Biome = flavor.BiomeName(last.Door.Topic)
	}
	return row
}

func (m *Model) attemptRow(a game.Attempt) meta.AttemptRow {
	d := a.Door
	r := meta.AttemptRow{
		Problem: d.Problem,
		Title:   m.title(d.Problem),
		Topic:   d.Topic,
		Rating:  d.Rating,
		Outcome: a.Outcome,
		Timer:   d.Timer,
		Submits: a.Submits,
		Failed:  a.Failed,
		Quacks:  a.Quacks,
		Boss:    d.Boss,
		Blind:   d.Blind,
		Mystery: d.Mystery,
		Cursed:  d.Cursed,
		Regress: d.Regression,
	}
	if !a.Closed.IsZero() {
		r.Elapsed = a.Closed.Sub(a.Opened)
	}
	if a.Outcome == game.Cleared {
		r.Kind = game.Independent
		if a.Quacks > 0 {
			r.Kind = game.Hinted
		}
	}
	return r
}

// stats maps store totals and the rating history onto the top screen.
func (m *Model) stats(st store.Stats, hist []store.RatingPoint) meta.Stats {
	s := meta.Stats{
		Rating:        m.meta.Rating,
		Solves:        st.Solves,
		Boots:         st.Boots,
		FullClears:    st.FullClears,
		Crashes:       st.Crashes,
		Uptime:        st.Uptime,
		LifetimeBytes: m.meta.LifetimeBytes,
		Bank:          m.meta.Bank,
	}
	for _, p := range hist {
		s.RatingHistory = append(s.RatingHistory, meta.RatingPoint{At: p.At.Local(), Rating: p.Rating})
	}
	for topic, t := range st.Topics {
		s.Topics = append(s.Topics, meta.TopicStat{
			Topic:    topic,
			Attempts: t.Attempts,
			Clears:   t.Clears,
			Rating:   m.meta.TopicRating[topic],
		})
	}
	slices.SortFunc(s.Topics, func(a, b meta.TopicStat) int { return b.Attempts - a.Attempts })
	return s
}
