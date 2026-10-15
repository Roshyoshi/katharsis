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
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
)

type Catalog struct {
	root   string // <cache>/katharsis/usaco
	base   string // site; a test server in tests
	client *politeClient

	mu      sync.RWMutex // guards entries (fetches refine them)
	saveMu  sync.Mutex
	entries map[catalog.ID]*entry
	order   []catalog.ID // cpid order

	flightMu sync.Mutex
	flights  map[catalog.ID]*flight
}

var _ catalog.Catalog = (*Catalog)(nil)

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
	cleanDebris(root, time.Now().Add(-debrisAge))
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
		flights: map[catalog.ID]*flight{},
	}
	for i := range idx.Entries {
		e := &idx.Entries[i]
		c.entries[e.ID] = e
		c.order = append(c.order, e.ID)
	}
	return c, nil
}

// debrisAge is how old a .tmp-* entry must be before Open deletes it:
// older than any fetch still running, so another process's work in
// progress is left alone.
const debrisAge = 6 * time.Hour

// cleanDebris removes what a killed process leaves behind: materialize's
// temp dirs (a half-downloaded zip and its tests can be hundreds of MB)
// and writeAtomic's temp files. The game quits without waiting for door
// prefetches, so this is the normal case, not just a crash.
func cleanDebris(root string, before time.Time) {
	des, _ := os.ReadDir(root) // a missing root has nothing to clean
	for _, d := range des {
		if !strings.HasPrefix(d.Name(), ".tmp-") {
			continue
		}
		if fi, err := d.Info(); err == nil && fi.ModTime().Before(before) {
			_ = os.RemoveAll(filepath.Join(root, d.Name()))
		}
	}
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

// flight is one in-progress Fetch that later callers for the same id wait
// on instead of downloading again.
type flight struct {
	done   chan struct{}
	assets catalog.Assets
	err    error
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
	for {
		c.flightMu.Lock()
		f, waiting := c.flights[id]
		if !waiting {
			f = &flight{done: make(chan struct{})}
			c.flights[id] = f
		}
		c.flightMu.Unlock()

		if !waiting {
			f.assets, f.err = c.fetch(ctx, e)
			c.flightMu.Lock()
			delete(c.flights, id)
			c.flightMu.Unlock()
			close(f.done)
			return f.assets, f.err
		}
		select {
		case <-ctx.Done():
			return catalog.Assets{}, ctx.Err()
		case <-f.done:
		}
		// The leader's context may have been cancelled while ours is
		// fine; take over instead of failing with someone else's error.
		if f.err != nil && isCtxErr(f.err) && ctx.Err() == nil {
			continue
		}
		return f.assets, f.err
	}
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

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
