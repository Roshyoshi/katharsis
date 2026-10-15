package catalog

import (
	"context"
	"fmt"
)

// Union serves several sources as one catalog. IDs carry their source
// ("usaco:1141", "cf:1899C"), so a lookup goes straight to its owner.
type Union struct {
	cats  []Catalog
	bySrc map[Source]Catalog
}

// NewUnion keys each catalog by the source of its first problem; one with
// no problems has nothing to serve and is left out.
func NewUnion(cats ...Catalog) *Union {
	u := &Union{bySrc: map[Source]Catalog{}}
	for _, c := range cats {
		if ps := c.All(); len(ps) > 0 {
			u.cats = append(u.cats, c)
			u.bySrc[ps[0].Source] = c
		}
	}
	return u
}

// All asks every source again rather than keeping a copy from NewUnion, so
// it agrees with Get after a fetch has refined a problem.
func (u *Union) All() []Problem {
	var out []Problem
	for _, c := range u.cats {
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
	c, ok := u.bySrc[id.Source()]
	return c, ok
}
