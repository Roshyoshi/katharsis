// Package codeforces serves Codeforces problems from the Hugging Face
// dataset open-r1/codeforces as a catalog.Catalog: about 10k problems with
// Codeforces ratings, statements, editorials (60%) and test data, judged
// locally like every other source.
//
// Open downloads the problem table once, only the columns it needs (17 MB
// of the 2.7 GB; see remote.go), and caches an index plus the statement
// text, so All and Get never touch the network and a Fetch needs it only
// for tests:
//
//	<cache>/katharsis/codeforces/
//	  index.json, text.bin                problem table; statement text
//	  <contest><index>/statement.md       e.g. 1899C/
//	  <contest><index>/editorial.md       when the dataset has one
//	  <contest><index>/samples/N.in, N.out
//	  <contest><index>/tests/N.in, N.out  official tests, then generated
//	  <contest><index>/meta.json          written last; marks the dir done
//
// Tests come from two places. A problem whose official tests are all in
// the dataset (19% of the judgeable ones) gets them from its
// verifiable_tests file, a few ranged reads. Every other judgeable
// problem has generated tests, stored one file per contest (median 32 MB,
// up to 1.75 GB); Fetch downloads the contest's file, keeps this
// problem's rows and deletes the rest, and adds the official tests the
// dataset does have (inputs under 400 characters, Codeforces' truncation
// limit) in front of them.
package codeforces

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/web"
)

type Catalog struct {
	root   string // <cache>/katharsis/codeforces
	base   string // "https://huggingface.co"; a test server in tests
	lay    layout
	client *web.Client

	mu      sync.RWMutex // guards entries (a fetch can mark one unjudgeable)
	saveMu  sync.Mutex
	idx     index // Key and text.bin's size and hash, for persist
	entries map[catalog.ID]*entry
	order   []catalog.ID // contest, then problem index

	flightMu sync.Mutex
	flights  map[catalog.ID]*flight
}

var _ catalog.Catalog = (*Catalog)(nil)

// client is the process's one client for Hugging Face, which rate-limits
// anonymous resolves (3000 per 5 minutes when written).
var client = &web.Client{
	HTTP:    &http.Client{Timeout: 15 * time.Minute},
	Gap:     100 * time.Millisecond,
	Retries: 4,
	Backoff: time.Second,
}

// Open loads the cached index (about 0.1 s), or builds it from the dataset
// and caches it: about 17 MB in 38 requests, 20 s when measured.
// Afterwards All and Get need no network.
func Open(ctx context.Context) (*Catalog, error) {
	c, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return open(ctx, filepath.Join(c, "katharsis", "codeforces"), "https://huggingface.co", pinned, client)
}

func open(ctx context.Context, root, base string, l layout, pc *web.Client) (*Catalog, error) {
	c := &Catalog{
		root: root, base: base, lay: l, client: pc,
		entries: map[catalog.ID]*entry{},
		flights: map[catalog.ID]*flight{},
	}
	// A contest download (.dl-) can be hundreds of MB.
	web.CleanDebris(root, web.TempPrefix, ".dl-")
	idx, ok := loadIndex(root, l)
	if !ok {
		var err error
		if idx, err = c.build(ctx); err != nil {
			return nil, err
		}
	}
	c.idx = *idx
	c.idx.Entries = nil
	for i := range idx.Entries {
		e := &idx.Entries[i]
		c.entries[e.ID] = e
		c.order = append(c.order, e.ID)
	}
	return c, nil
}

// build downloads the table shards and the generated-tests listing and
// writes the cache.
func (c *Catalog) build(ctx context.Context) (*index, error) {
	gen, err := c.genSizes(ctx)
	if err != nil {
		return nil, err
	}
	// A split's rows in shard order are its table order, which the
	// verifiable_tests positions are counted in.
	var splits []split
	for _, s := range c.lay.Shards {
		rows, err := c.readShard(ctx, s)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(splits, func(sp split) bool { return sp.name == s.Split })
		if i < 0 {
			i = len(splits)
			splits = append(splits, split{name: s.Split})
		}
		splits[i].rows = append(splits[i].rows, rows...)
	}
	idx, blob, err := buildIndex(c.lay, splits, gen)
	if err != nil {
		return nil, err
	}
	if err := saveIndex(c.root, idx, blob); err != nil {
		return nil, err
	}
	return idx, nil
}

// All lists every problem in contest order, judgeable or not.
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

// markMissing takes id out of door play after a fetch found its tests
// absent from the files the index pointed at, and persists that, so the
// engine stops offering it.
func (c *Catalog) markMissing(id catalog.ID) error {
	c.mu.Lock()
	e, ok := c.entries[id]
	if !ok || e.Skip == skipTestsMissing {
		c.mu.Unlock()
		return nil
	}
	e.Skip, e.Judgeable = skipTestsMissing, false
	c.mu.Unlock()
	return c.persist()
}

// persist rewrites index.json from memory; text.bin does not change.
// saveMu orders the writers and each snapshots under it, so the last
// rename carries every update so far.
func (c *Catalog) persist() error {
	c.saveMu.Lock()
	defer c.saveMu.Unlock()
	c.mu.RLock()
	idx := c.idx
	idx.Entries = make([]entry, 0, len(c.order))
	for _, id := range c.order {
		idx.Entries = append(idx.Entries, *c.entries[id])
	}
	c.mu.RUnlock()
	b, err := json.Marshal(&idx)
	if err != nil {
		return err
	}
	return web.WriteFile(filepath.Join(c.root, "index.json"), b)
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
// parallel but still go through the per-host limiter. Tests are fetched
// only for judgeable problems; the others get statement, samples and
// editorial.
func (c *Catalog) Fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	e, ok := c.entry(id)
	if !ok {
		return catalog.Assets{}, errors.New(string(id) + ": not a Codeforces problem in the catalog")
	}
	dir := filepath.Join(c.root, dirName(e))
	for {
		if a, ok := loadAssets(dir); usable(a, ok, e) {
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

// dirName is "1899C": the id without "cf:". newEntry admits only digits
// and an index of capital letters and digits, so it is a safe path.
func dirName(e entry) string { return e.ID.Native() }

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
