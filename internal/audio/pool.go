package audio

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"os"
	"path/filepath"
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
// next. When it is all the round has left, its turn there is the one it
// just had, and the next round opens with another.
func (b *bag) avoid(id string, rng *rand.Rand) {
	b.last = id
	if b.pos >= len(b.order) || b.order[b.pos] != id {
		return
	}
	if rest := len(b.order) - b.pos; rest > 1 {
		k := b.pos + 1 + rng.IntN(rest-1)
		b.order[b.pos], b.order[k] = b.order[k], b.order[b.pos]
	} else {
		b.pos++
	}
}

// roundKey names the bag a set of tracks is dealt from: every cue that
// plays the same set (each topic biome keys its cue, with only the pool's
// tracks) goes on with the same round.
func roundKey(pool string, ids []string) string {
	h := fnv.New64a()
	for _, id := range ids {
		h.Write([]byte(id))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%s#%016x", pool, h.Sum64())
}

// dealt is a bag as rotation.json keeps it, so a round goes on across
// launches: a pool plays through before anything repeats, however short
// the sessions it is heard in.
type dealt struct {
	Order []string `json:"order"`
	Pos   int      `json:"pos"`
	Last  string   `json:"last"`
}

func rotationPath(cache string) string { return filepath.Join(cache, "rotation.json") }

// loadRotation reads the bags rotation.json kept; none when it is missing
// or unreadable, as for a first launch.
func loadRotation(cache string) map[string]*bag {
	bags := map[string]*bag{}
	b, err := os.ReadFile(rotationPath(cache))
	if cache == "" || err != nil {
		return bags
	}
	var kept map[string]dealt
	if json.Unmarshal(b, &kept) != nil {
		return bags
	}
	for k, d := range kept {
		if len(d.Order) == 0 || d.Pos < 0 || d.Pos > len(d.Order) {
			continue
		}
		ids := slices.Sorted(slices.Values(d.Order))
		bags[k] = &bag{ids: ids, order: d.Order, pos: d.Pos, last: d.Last}
	}
	return bags
}

// saveRotation keeps bags in rotation.json, those that have dealt.
func saveRotation(cache string, bags map[string]*bag) error {
	if cache == "" {
		return nil
	}
	kept := map[string]dealt{}
	for k, b := range bags {
		if len(b.order) > 0 {
			kept[k] = dealt{Order: b.order, Pos: b.pos, Last: b.last}
		}
	}
	out, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	return writeAtomic(rotationPath(cache), out)
}
