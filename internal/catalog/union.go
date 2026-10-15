package catalog

import (
	"context"
	"fmt"
	"sync"
)

// Union serves several sources as one catalog. IDs carry their source
// ("usaco:1141", "cf:1899C"), so a lookup goes straight to its owner. A
// source can join after the union is made (Add); a caller that took All
// before keeps what it took, so a boot drawing doors from that pool is not
// changed under it.
type Union struct {
	mu    sync.RWMutex // Add writes while the game reads
	order []Source     // as added
	bySrc map[Source]Catalog
}

// NewUnion adds each of cats in turn.
func NewUnion(cats ...Catalog) *Union {
	u := &Union{bySrc: map[Source]Catalog{}}
	for _, c := range cats {
		u.Add(c)
	}
	return u
}

// Add serves c's problems from now on, keyed by the source of its first
// problem. One with no problems has nothing to serve and is left out; one
// whose source is already served takes its place.
func (u *Union) Add(c Catalog) {
	ps := c.All()
	if len(ps) == 0 {
		return
	}
	src := ps[0].Source
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, ok := u.bySrc[src]; !ok {
		u.order = append(u.order, src)
	}
	u.bySrc[src] = c
}

// All asks every source again rather than keeping a copy from NewUnion, so
// it agrees with Get after a fetch has refined a problem.
func (u *Union) All() []Problem {
	u.mu.RLock()
	cats := make([]Catalog, 0, len(u.order))
	for _, s := range u.order {
		cats = append(cats, u.bySrc[s])
	}
	u.mu.RUnlock()
	var out []Problem
	for _, c := range cats {
		out = append(out, c.All()...)
	}
	return out
}

func (u *Union) Get(id ID) (Problem, bool) {
	if c, ok := u.owner(id); ok {
		return c.Get(id)
	}
	return Problem{}, false
}

func (u *Union) Fetch(ctx context.Context, id ID) (Assets, error) {
	c, ok := u.owner(id)
	if !ok {
		return Assets{}, fmt.Errorf("no source serves %s", id)
	}
	return c.Fetch(ctx, id)
}

func (u *Union) owner(id ID) (Catalog, bool) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	c, ok := u.bySrc[id.Source()]
	return c, ok
}
