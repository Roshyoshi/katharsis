package app

import (
	"cmp"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens/meta"
	"katharsis/internal/store"
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
		Dumped:   b.Dumped,
	}
	if b.LastDoor != "" {
		row.Problem = m.title(b.LastDoor)
	}
	// Attempts come newest first; the screen lists them in play order.
	for i := len(atts) - 1; i >= 0; i-- {
		row.Attempts = append(row.Attempts, m.attemptRow(atts[i].Attempt))
	}
	if len(atts) > 0 && !row.Cleared {
		row.Biome = m.lastBiome(b.Rules, atts)
	}
	return row
}

// lastBiome names the biome a boot ended in, from its last attempt (atts
// come newest first): the boss of a boss room, else the preset's name for
// the biome the door opened in, else the flavor name of its topic. A door's
// topic names a topic biome, but not Userland, whose doors each keep their
// own topic, so the engine's first fixed biome is named as mixed.
func (m *Model) lastBiome(preset string, atts []store.AttemptRecord) string {
	last := atts[0]
	if last.Door.Boss != "" {
		return flavor.BossName(last.Door.Boss)
	}
	topic := last.Door.Topic
	r, ok := m.d.Presets[preset]
	if !ok || last.Biome >= len(r.Biomes) {
		return flavor.BiomeName(topic)
	}
	rb := r.Biomes[last.Biome]
	if rb.Kind == rules.Fixed && last.Biome == 0 {
		topic = game.Mixed
	}
	return cmp.Or(rb.Name, flavor.BiomeName(topic))
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

// stats maps store totals and the rating history onto the top screen, which
// sorts the topics itself.
func (m *Model) stats(st store.Stats, hist []store.RatingPoint) meta.Stats {
	s := meta.Stats{
		Rating:        m.meta.Rating,
		Solves:        st.Solves,
		Boots:         st.Boots,
		FullClears:    st.FullClears,
		Crashes:       st.Crashes,
		TimePlus:      st.TimePlus,
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
			Time:     t.Time,
		})
	}
	return s
}
