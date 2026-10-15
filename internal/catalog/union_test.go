package catalog_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"katharsis/internal/catalog"
)

// fake is a source whose problems a test can refine after NewUnion, the way
// a USACO fetch does.
type fake struct{ ps []catalog.Problem }

func (f *fake) All() []catalog.Problem { return slices.Clone(f.ps) }

func (f *fake) Get(id catalog.ID) (catalog.Problem, bool) {
	i := slices.IndexFunc(f.ps, func(p catalog.Problem) bool { return p.ID == id })
	if i < 0 {
		return catalog.Problem{}, false
	}
	return f.ps[i], true
}

func (f *fake) Fetch(_ context.Context, id catalog.ID) (catalog.Assets, error) {
	if _, ok := f.Get(id); !ok {
		return catalog.Assets{}, errors.New("unknown")
	}
	return catalog.Assets{Statement: string(id) + ".md"}, nil
}

func TestUnion(t *testing.T) {
	t.Parallel()
	us := &fake{ps: []catalog.Problem{{ID: "usaco:1", Source: catalog.USACO, Judgeable: true}}}
	cf := &fake{ps: []catalog.Problem{{ID: "cf:1A", Source: catalog.CF}, {ID: "cf:2B", Source: catalog.CF}}}
	u := catalog.NewUnion(us, &fake{}, cf)

	var ids []catalog.ID
	for _, p := range u.All() {
		ids = append(ids, p.ID)
	}
	if want := []catalog.ID{"usaco:1", "cf:1A", "cf:2B"}; !slices.Equal(ids, want) {
		t.Errorf("All ids = %v, want %v", ids, want)
	}
	if p, ok := u.Get("cf:2B"); !ok || p.ID != "cf:2B" {
		t.Errorf("Get(cf:2B) = %+v, %v", p, ok)
	}
	for _, id := range []catalog.ID{"cses:1", "usaco", "nosuch"} {
		if _, ok := u.Get(id); ok {
			t.Errorf("Get(%s) found a problem", id)
		}
		if _, err := u.Fetch(context.Background(), id); err == nil {
			t.Errorf("Fetch(%s) succeeded", id)
		}
	}
	if a, err := u.Fetch(context.Background(), "usaco:1"); err != nil || a.Statement != "usaco:1.md" {
		t.Errorf("Fetch(usaco:1) = %+v, %v", a, err)
	}

	// A fetch that learns a problem cannot be judged must show in All, as
	// it already does in Get.
	us.ps[0].Judgeable = false
	if p := u.All()[0]; p.Judgeable {
		t.Errorf("All()[0] = %+v after the source refined it", p)
	}
}
