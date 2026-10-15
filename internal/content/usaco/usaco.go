// Package usaco serves USACO contest problems as a catalog.Catalog.
//
// The index (titles, divisions, tags, ratings, limits) is built once from
// the pinned USACO Guide snapshot plus facts.json, the statement facts
// scraped for that snapshot's problems, and cached as JSON, so All and Get
// never touch the network. Fetch downloads a problem's statement, test
// data and analysis from usaco.org into the cache on first use:
//
//	<cache>/katharsis/usaco/
//	  index.json
//	  <cpid>/statement.html, statement.md   the page and its markdown
//	  <cpid>/samples/1.in, 1.out, ...       from the statement
//	  <cpid>/tests/1.in, 1.out, ...         from data/<stem>.zip
//	  <cpid>/editorial.html, editorial.md   from data/sol_<stem>.html
//	  <cpid>/meta.json                      written last; marks the dir done
package usaco

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
	"katharsis/internal/content/web"
)

type Catalog struct {
	root   string // <cache>/katharsis/usaco
	base   string // site; a test server in tests
	client *web.Client

	mu      sync.RWMutex // guards entries (fetches refine them)
	saveMu  sync.Mutex
	entries map[catalog.ID]*entry
	order   []catalog.ID // cpid order

	flights web.Flights[catalog.ID, catalog.Assets]
}

var _ catalog.Catalog = (*Catalog)(nil)

// client is the process's one client for usaco.org, a volunteer-run site,
// so requests are half a second apart.
var client = &web.Client{
	HTTP:    &http.Client{Timeout: 5 * time.Minute},
	Gap:     500 * time.Millisecond,
	Retries: 4,
	Backoff: time.Second,
}

// Open loads the cached index, or builds it from the guide snapshot
// (downloading the snapshot on first run) and caches it.
func Open(ctx context.Context) (*Catalog, error) {
	c, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return open(ctx, filepath.Join(c, "katharsis", "usaco"), guide.Snapshot)
}

func open(ctx context.Context, root string, snapshot func(context.Context) (string, error)) (*Catalog, error) {
	// A temp dir from materialize can hold a half-downloaded zip and its
	// tests, hundreds of MB.
	web.CleanDebris(root, web.TempPrefix)
	idxPath := filepath.Join(root, "index.json")
	idx, ok := loadIndex(idxPath)
	if !ok {
		dir, err := snapshot(ctx)
		if err != nil {
			return nil, err
		}
		table, err := loadFactsTable()
		if err != nil {
			return nil, err
		}
		if idx, err = buildIndex(dir, table); err != nil {
			return nil, err
		}
		if err := saveIndex(idxPath, idx); err != nil {
			return nil, err
		}
	}
	c := &Catalog{
		root:    root,
		base:    site,
		client:  client,
		entries: map[catalog.ID]*entry{},
	}
	for i := range idx.Entries {
		e := &idx.Entries[i]
		c.entries[e.ID] = e
		c.order = append(c.order, e.ID)
	}
	return c, nil
}

// All lists every problem in cpid order, judgeable or not.
func (c *Catalog) All() []catalog.Problem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]catalog.Problem, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.entries[id].problem())
	}
	return out
}

func (c *Catalog) Get(id catalog.ID) (catalog.Problem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[id]
	if !ok {
		return catalog.Problem{}, false
	}
	return e.problem(), true
}

// problem copies the entry so callers cannot alias the shared Tags slice.
func (e *entry) problem() catalog.Problem {
	p := e.Problem
	p.Tags = append([]string(nil), p.Tags...)
	return p
}

func (c *Catalog) entry(id catalog.ID) (entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[id]
	if !ok {
		return entry{}, false
	}
	return *e, true
}

// update applies what a fetch read from the statement and persists the
// index when anything changed, so the next Open (and the engine's next
// Get) sees, say, that a problem turned out to need a checker.
func (c *Catalog) update(id catalog.ID, m meta) error {
	c.mu.Lock()
	e := c.entries[id]
	before := *e // apply never touches Tags, so sharing them is fine
	e.HasZip, e.Verified = m.HasZip, true
	e.apply(m.Facts)
	e.HasEditorial = m.HasEditorial
	changed := !reflect.DeepEqual(before, *e)
	c.mu.Unlock()
	if !changed {
		return nil
	}
	return c.persist()
}

// persist writes the in-memory index. saveMu orders the writers and each
// snapshots under it, so the last rename always carries every update
// applied so far.
func (c *Catalog) persist() error {
	c.saveMu.Lock()
	defer c.saveMu.Unlock()
	c.mu.RLock()
	idx := &index{Key: indexKey()}
	for _, id := range c.order {
		idx.Entries = append(idx.Entries, *c.entries[id])
	}
	c.mu.RUnlock()
	return saveIndex(filepath.Join(c.root, "index.json"), idx)
}

// Fetch materializes id under the cache and returns its asset paths. A
// finished problem dir is reused without the network. Concurrent calls for
// one id share a single download; calls for different ids proceed in
// parallel but still go through the per-host limiter.
func (c *Catalog) Fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	e, ok := c.entry(id)
	if !ok {
		return catalog.Assets{}, errors.New(string(id) + ": not a USACO problem in the catalog")
	}
	return c.flights.Do(ctx, id, func() (catalog.Assets, error) { return c.fetch(ctx, e) })
}

// fetch runs as the only flight for e, so its cache check cannot miss a
// download of the same problem that finishes a moment later and start a
// second one.
func (c *Catalog) fetch(ctx context.Context, e entry) (catalog.Assets, error) {
	dir := filepath.Join(c.root, strconv.Itoa(e.CPID))
	a, m, ok := loadAssets(dir)
	if !ok {
		return c.materialize(ctx, e, dir)
	}
	// A dir rendered by an older renderer is re-rendered in place. If the
	// saved pages cannot be read the old markdown stays: it is still the
	// statement, only less well drawn.
	if m.Render < renderVersion {
		_ = rerender(dir, e.CPID, &m)
	}
	// An index rebuilt since that fetch (new facts.json, new guide commit)
	// is back on guess-work for a problem facts.json lacks: default
	// limits, and judgeable even if its zip turned out not to exist.
	if !e.Verified {
		if err := c.update(e.ID, m); err != nil {
			return catalog.Assets{}, err
		}
	}
	return a, nil
}
