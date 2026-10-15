package audio

import (
	"math/rand/v2"
	"slices"
)

// bag deals a pool's tracks in shuffled rounds: every track once per
// round, and a round never opens with the track that closed the last, so
// the same track never plays twice in a row unless it is alone.
type bag struct {
	ids   []string // the set the round was dealt from, sorted
	order []string
	pos   int
	last  string
}

// next deals the next track of ids (sorted). A changed set starts a new
// round.
func (b *bag) next(ids []string, rng *rand.Rand) string {
	if len(ids) == 0 {
		return ""
	}
	if !slices.Equal(ids, b.ids) {
		b.ids = slices.Clone(ids)
		b.order, b.pos = nil, 0
	}
	if b.pos >= len(b.order) {
		b.order = slices.Clone(ids)
		rng.Shuffle(len(b.order), func(i, j int) { b.order[i], b.order[j] = b.order[j], b.order[i] })
		if len(b.order) > 1 && b.order[0] == b.last {
			k := 1 + rng.IntN(len(b.order)-1)
			b.order[0], b.order[k] = b.order[k], b.order[0]
		}
		b.pos = 0
	}
	id := b.order[b.pos]
	b.pos++
	b.last = id
	return id
}

// avoid keeps id, just played from elsewhere (a lead), from being dealt
// next.
func (b *bag) avoid(id string, rng *rand.Rand) {
	b.last = id
	if rest := len(b.order) - b.pos; rest > 1 && b.order[b.pos] == id {
		k := b.pos + 1 + rng.IntN(rest-1)
		b.order[b.pos], b.order[k] = b.order[k], b.order[b.pos]
	}
}
