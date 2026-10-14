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
	"strconv"
	"sync"

	"katharsis/internal/catalog"
	"katharsis/internal/content/guide"
)

type Catalog struct {
	root   string // <cache>/katharsis/usaco
	base   string // "https://usaco.org"; a test server in tests
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
		base:    "https://usaco.org",
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

// update replaces the statement-derived fields of id after a fetch and
// persists the index when anything changed, so the next Open (and the
// engine's next Get) sees, say, that a problem turned out to need a
// checker.
func (c *Catalog) update(id catalog.ID, f facts, hasZip, hasEditorial bool) error {
	c.mu.Lock()
	e, ok := c.entries[id]
	if !ok {
		c.mu.Unlock()
		return nil
	}
	before, beforeZip, beforeVerified := e.problem(), e.HasZip, e.Verified
	e.HasZip, e.Verified = hasZip, true
	e.apply(f)
	e.HasEditorial = hasEditorial
	changed := e.HasZip != beforeZip || e.Verified != beforeVerified || !sameProblem(before, e.Problem)
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

func sameProblem(a, b catalog.Problem) bool {
	return a.Title == b.Title && a.Contest == b.Contest && a.Division == b.Division &&
		a.Position == b.Position && a.TimeLimit == b.TimeLimit && a.MemLimit == b.MemLimit &&
		a.FileIO == b.FileIO && a.Interactive == b.Interactive &&
		a.NeedsChecker == b.NeedsChecker && a.FloatTolerance == b.FloatTolerance &&
		a.Judgeable == b.Judgeable && a.HasEditorial == b.HasEditorial
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
	dir := filepath.Join(c.root, strconv.Itoa(e.CPID))
	for {
		if a, ok := loadAssets(dir); ok {
			return a, nil
		}
		c.flightMu.Lock()
		f, waiting := c.flights[id]
		if !waiting {
			f = &flight{done: make(chan struct{})}
			c.flights[id] = f
		}
		c.flightMu.Unlock()

		if !waiting {
			f.assets, f.err = c.materialize(ctx, e, dir)
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

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
