package catalog

import (
	"context"
	"fmt"
	"strings"
)

// Union serves several sources as one catalog. IDs carry their source
// ("usaco:1141", "cf:1899C"), so a lookup goes straight to its owner.
type Union struct {
	all   []Problem
	bySrc map[Source]Catalog
}

func NewUnion(cats ...Catalog) *Union {
	u := &Union{bySrc: map[Source]Catalog{}}
	for _, c := range cats {
		ps := c.All()
		u.all = append(u.all, ps...)
		if len(ps) > 0 {
			u.bySrc[ps[0].Source] = c
		}
	}
	return u
}

func (u *Union) All() []Problem { return u.all }

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
	src, _, ok := strings.Cut(string(id), ":")
	if !ok {
		return nil, false
	}
	c, ok := u.bySrc[Source(src)]
	return c, ok
}
